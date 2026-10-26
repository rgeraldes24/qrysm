package accounts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/network/forks"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1/mock"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// exitTestGVR is the genesis validators root of the chain the exit tests run
// on.
var exitTestGVR = bytes.Repeat([]byte{0x5a}, 32)

const exitTestValidatorIndex = 7

// fakeBeaconNode serves over gRPC the beacon node calls a voluntary exit
// makes. It computes domains as a beacon node does and accepts an exit only
// when its signature verifies against the validator's public key.
type fakeBeaconNode struct {
	addr        string
	genesisTime time.Time
	pubKey      []byte

	mu    sync.Mutex
	exits []*qrysmpb.SignedVoluntaryExit
}

type fakeNodeServer struct {
	qrysmpb.UnimplementedNodeServer
	b *fakeBeaconNode
}

func (s *fakeNodeServer) GetGenesis(context.Context, *emptypb.Empty) (*qrysmpb.Genesis, error) {
	return &qrysmpb.Genesis{
		GenesisTime:           timestamppb.New(s.b.genesisTime),
		GenesisValidatorsRoot: exitTestGVR,
	}, nil
}

func (*fakeNodeServer) GetSyncStatus(context.Context, *emptypb.Empty) (*qrysmpb.SyncStatus, error) {
	return &qrysmpb.SyncStatus{Syncing: false}, nil
}

type fakeValidatorServer struct {
	qrysmpb.UnimplementedBeaconNodeValidatorServer
	b *fakeBeaconNode
}

func (s *fakeValidatorServer) ValidatorIndex(_ context.Context, req *qrysmpb.ValidatorIndexRequest) (*qrysmpb.ValidatorIndexResponse, error) {
	if !bytes.Equal(req.PublicKey, s.b.pubKey) {
		return nil, status.Error(codes.NotFound, "unknown validator")
	}
	return &qrysmpb.ValidatorIndexResponse{Index: exitTestValidatorIndex}, nil
}

func (*fakeValidatorServer) DomainData(_ context.Context, req *qrysmpb.DomainRequest) (*qrysmpb.DomainResponse, error) {
	domain, err := exitTestDomain(req.Epoch, req.Domain)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &qrysmpb.DomainResponse{SignatureDomain: domain}, nil
}

func (s *fakeValidatorServer) ProposeExit(_ context.Context, exit *qrysmpb.SignedVoluntaryExit) (*qrysmpb.ProposeExitResponse, error) {
	if exit.Exit.ValidatorIndex != exitTestValidatorIndex {
		return nil, status.Errorf(codes.InvalidArgument, "unknown validator index %d", exit.Exit.ValidatorIndex)
	}
	domain, err := exitTestDomain(exit.Exit.Epoch, params.BeaconConfig().DomainVoluntaryExit[:])
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

func exitTestDomain(epoch primitives.Epoch, domainType []byte) ([]byte, error) {
	fork, err := forks.Fork(epoch)
	if err != nil {
		return nil, err
	}
	return signing.Domain(fork, epoch, bytesutil.ToBytes4(domainType), exitTestGVR)
}

func startFakeBeaconNode(t *testing.T, pubKey []byte, genesisTime time.Time) *fakeBeaconNode {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	b := &fakeBeaconNode{addr: lis.Addr().String(), genesisTime: genesisTime, pubKey: pubKey}
	srv := grpc.NewServer()
	qrysmpb.RegisterNodeServer(srv, &fakeNodeServer{b: b})
	qrysmpb.RegisterBeaconNodeValidatorServer(srv, &fakeValidatorServer{b: b})
	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(srv.Stop)
	return b
}

func (b *fakeBeaconNode) proposedExits() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.exits)
}

// runRemoteSignerExit runs AccountsExit with the flags of accounts
// voluntary-exit, exiting the beacon node's validator through the remote
// signer at signerURL.
func runRemoteSignerExit(t *testing.T, beacon *fakeBeaconNode, signerURL, tokenFile string) error {
	t.Helper()
	var command *cli.Command
	for _, c := range Commands.Subcommands {
		if c.Name == "voluntary-exit" {
			command = c
		}
	}
	require.NotNil(t, command)
	set := flag.NewFlagSet(command.Name, flag.ContinueOnError)
	for _, f := range command.Flags {
		require.NoError(t, f.Apply(set))
	}
	pubKey := hexutil.Encode(beacon.pubKey)
	require.NoError(t, set.Parse([]string{
		"--" + flags.Web3SignerURLFlag.Name, signerURL,
		"--" + flags.Web3SignerPublicValidatorKeysFlag.Name, pubKey,
		"--" + flags.Web3SignerAuthTokenFileFlag.Name, tokenFile,
		"--" + flags.BeaconRPCProviderFlag.Name, beacon.addr,
		"--" + flags.VoluntaryExitPublicKeysFlag.Name, pubKey,
		"--" + flags.ForceExitFlag.Name,
	}))
	return AccountsExit(cli.NewContext(&cli.App{}, set, nil), strings.NewReader(""))
}

// assertRemoteSignerExit checks the outcome of runRemoteSignerExit: one exit
// with a valid signature reached the beacon node when the signer signs exits,
// and otherwise the command failed naming the signer option and nothing was
// submitted.
func assertRemoteSignerExit(t *testing.T, beacon *fakeBeaconNode, err error, exitsEnabled bool) {
	t.Helper()
	if exitsEnabled {
		require.NoError(t, err)
		require.Equal(t, 1, beacon.proposedExits())
		return
	}
	t.Logf("exit refused: %v", err)
	require.ErrorContains(t, "remote signer does not sign voluntary exits", err)
	require.ErrorContains(t, "--voluntary-exit-signing-enabled", err)
	require.ErrorContains(t, "ARTIFACT_TYPE_DISABLED", err)
	require.Equal(t, true, errors.Is(err, keymanager.ErrVoluntaryExitSigningDisabled))
	require.Equal(t, 0, beacon.proposedExits())
}

const exitTestToken = "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// startFakeRemoteSigner serves the remote signer's sign route for key: it
// checks each request against the contract, then signs voluntary exits or,
// as the signer does without --voluntary-exit-signing-enabled, refuses them
// with 403 ARTIFACT_TYPE_DISABLED. It returns the signer URL and a token file.
func startFakeRemoteSigner(t *testing.T, key ml_dsa_87.MLDSA87Key, exitsEnabled bool) (string, string) {
	t.Helper()
	signPath := "/api/v1/consensus/sign/" + hexutil.Encode(key.PublicKey().Marshal())
	reply := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	refuse := func(w http.ResponseWriter, status int, reason string) {
		reply(w, status, map[string]any{"code": status, "message": "refused", "reason": reason})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+exitTestToken {
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
			SigningRoot hexutil.Bytes `json:"signing_root"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			refuse(w, http.StatusBadRequest, "MALFORMED_JSON")
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
	require.NoError(t, os.WriteFile(tokenFile, []byte(exitTestToken+"\n"), 0o600))
	return srv.URL, tokenFile
}

func TestAccountsExit_RemoteSigner(t *testing.T) {
	key, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	for _, exitsEnabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("exit signing enabled %v", exitsEnabled), func(t *testing.T) {
			beacon := startFakeBeaconNode(t, key.PublicKey().Marshal(), time.Now().Add(-time.Hour))
			signerURL, tokenFile := startFakeRemoteSigner(t, key, exitsEnabled)
			err := runRemoteSignerExit(t, beacon, signerURL, tokenFile)
			assertRemoteSignerExit(t, beacon, err, exitsEnabled)
		})
	}
}
