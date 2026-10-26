// Package exittest provides a fake beacon node and a fake remote signer for
// tests of the voluntary exit commands.
package exittest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GenesisValidatorsRoot is the genesis validators root of the chain the fakes
// run on.
var GenesisValidatorsRoot = bytes.Repeat([]byte{0x5a}, 32)

// ValidatorIndex is the index of the fake beacon node's one validator.
const ValidatorIndex = 7

// BeaconNode serves over gRPC the beacon node calls a voluntary exit makes.
// It computes domains as a beacon node of its network does, from its own
// genesis fork version rather than the process's beacon config, and accepts
// an exit only when its signature verifies against the validator's public key.
type BeaconNode struct {
	Addr        string
	genesisTime time.Time
	forkVersion []byte
	pubKey      []byte

	mu    sync.Mutex
	exits []*qrysmpb.SignedVoluntaryExit
}

type nodeServer struct {
	qrysmpb.UnimplementedNodeServer
	b *BeaconNode
}

func (s *nodeServer) GetGenesis(context.Context, *emptypb.Empty) (*qrysmpb.Genesis, error) {
	return &qrysmpb.Genesis{
		GenesisTime:           timestamppb.New(s.b.genesisTime),
		GenesisValidatorsRoot: GenesisValidatorsRoot,
	}, nil
}

func (*nodeServer) GetSyncStatus(context.Context, *emptypb.Empty) (*qrysmpb.SyncStatus, error) {
	return &qrysmpb.SyncStatus{Syncing: false}, nil
}

type validatorServer struct {
	qrysmpb.UnimplementedBeaconNodeValidatorServer
	b *BeaconNode
}

func (s *validatorServer) ValidatorIndex(_ context.Context, req *qrysmpb.ValidatorIndexRequest) (*qrysmpb.ValidatorIndexResponse, error) {
	if !bytes.Equal(req.PublicKey, s.b.pubKey) {
		return nil, status.Error(codes.NotFound, "unknown validator")
	}
	return &qrysmpb.ValidatorIndexResponse{Index: ValidatorIndex}, nil
}

func (s *validatorServer) DomainData(_ context.Context, req *qrysmpb.DomainRequest) (*qrysmpb.DomainResponse, error) {
	domain, err := s.b.domain(req.Epoch, req.Domain)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &qrysmpb.DomainResponse{SignatureDomain: domain}, nil
}

func (s *validatorServer) ProposeExit(_ context.Context, exit *qrysmpb.SignedVoluntaryExit) (*qrysmpb.ProposeExitResponse, error) {
	if exit.Exit.ValidatorIndex != ValidatorIndex {
		return nil, status.Errorf(codes.InvalidArgument, "unknown validator index %d", exit.Exit.ValidatorIndex)
	}
	domain, err := s.b.domain(exit.Exit.Epoch, params.BeaconConfig().DomainVoluntaryExit[:])
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := signing.VerifySigningRoot(exit.Exit, s.b.pubKey, exit.Signature, domain); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid exit signature: %v", err)
	}
	root, err := exit.Exit.HashTreeRoot()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	s.b.exits = append(s.b.exits, exit)
	return &qrysmpb.ProposeExitResponse{ExitRoot: root[:]}, nil
}

// domain computes a domain on the beacon node's network, which has a single
// fork: its genesis fork version from epoch 0.
func (b *BeaconNode) domain(epoch primitives.Epoch, domainType []byte) ([]byte, error) {
	fork := &qrysmpb.Fork{PreviousVersion: b.forkVersion, CurrentVersion: b.forkVersion, Epoch: 0}
	return signing.Domain(fork, epoch, bytesutil.ToBytes4(domainType), GenesisValidatorsRoot)
}

// StartBeaconNode starts a fake beacon node on a loopback port for the
// validator with pubKey, on a network with the given genesis time and genesis
// fork version.
func StartBeaconNode(t testing.TB, pubKey []byte, genesisTime time.Time, forkVersion []byte) *BeaconNode {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	b := &BeaconNode{Addr: lis.Addr().String(), genesisTime: genesisTime, forkVersion: forkVersion, pubKey: pubKey}
	srv := grpc.NewServer()
	qrysmpb.RegisterNodeServer(srv, &nodeServer{b: b})
	qrysmpb.RegisterBeaconNodeValidatorServer(srv, &validatorServer{b: b})
	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(srv.Stop)
	return b
}

// ProposedExits returns the number of exits the beacon node accepted.
func (b *BeaconNode) ProposedExits() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.exits)
}

// Token is the sign-scope bearer token of the fake remote signer.
const Token = "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// StartRemoteSigner serves the remote signer's sign route for key, on the
// network with the given genesis fork version and GenesisValidatorsRoot. It
// checks each request against the contract and, as the signer does, refuses
// a voluntary exit with 403 ARTIFACT_TYPE_DISABLED unless exitsEnabled, and
// one whose fork_info is not its network's with 400 FORK_INFO_MISMATCH. It
// signs the others. It returns the signer URL and a file holding Token.
func StartRemoteSigner(t testing.TB, key ml_dsa_87.MLDSA87Key, forkVersion []byte, exitsEnabled bool) (string, string) {
	t.Helper()
	signPath := "/api/v1/consensus/sign/" + hexutil.Encode(key.PublicKey().Marshal())
	reply := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	refuse := func(w http.ResponseWriter, code int, reason string) {
		reply(w, code, map[string]any{"code": code, "message": "refused", "reason": reason})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+Token {
			refuse(w, http.StatusUnauthorized, "UNAUTHENTICATED")
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != signPath {
			refuse(w, http.StatusNotFound, "UNKNOWN_KEY")
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			refuse(w, http.StatusBadRequest, "MALFORMED_JSON")
			return
		}
		typ, err := mock.ValidateSignRequestJSON(body)
		if err != nil || typ != mock.VoluntaryExit {
			t.Errorf("unexpected sign request %s: %v", typ, err)
			refuse(w, http.StatusBadRequest, "BAD_FIELD")
			return
		}
		if !exitsEnabled {
			refuse(w, http.StatusForbidden, "ARTIFACT_TYPE_DISABLED")
			return
		}
		var req struct {
			ForkInfo struct {
				Fork struct {
					PreviousVersion hexutil.Bytes `json:"previous_version"`
					CurrentVersion  hexutil.Bytes `json:"current_version"`
					Epoch           string        `json:"epoch"`
				} `json:"fork"`
				GenesisValidatorsRoot hexutil.Bytes `json:"genesis_validators_root"`
			} `json:"fork_info"`
			SigningRoot hexutil.Bytes `json:"signing_root"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			refuse(w, http.StatusBadRequest, "MALFORMED_JSON")
			return
		}
		fork := req.ForkInfo.Fork
		if !bytes.Equal(fork.PreviousVersion, forkVersion) || !bytes.Equal(fork.CurrentVersion, forkVersion) ||
			fork.Epoch != "0" || !bytes.Equal(req.ForkInfo.GenesisValidatorsRoot, GenesisValidatorsRoot) {
			refuse(w, http.StatusBadRequest, "FORK_INFO_MISMATCH")
			return
		}
		sig, err := key.Sign(req.SigningRoot)
		if err != nil {
			refuse(w, http.StatusInternalServerError, "INTERNAL")
			return
		}
		reply(w, http.StatusOK, map[string]string{"signature": hexutil.Encode(sig.Marshal())})
	}))
	t.Cleanup(srv.Close)
	tokenFile := filepath.Join(t.TempDir(), "auth-token")
	require.NoError(t, os.WriteFile(tokenFile, []byte(Token+"\n"), 0o600))
	return srv.URL, tokenFile
}
