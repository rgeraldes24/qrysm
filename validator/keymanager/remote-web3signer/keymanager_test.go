package remote_web3signer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-qrl/common/hexutil"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	qrlpbservice "github.com/theQRL/qrysm/proto/qrl/service"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/internal"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1/mock"
)

var _ keymanager.IKeymanager = (*Keymanager)(nil)

const testGenesisTime = 1760000000

func testGVR() []byte {
	return bytes.Repeat([]byte{0x7a}, field_params.RootLength)
}

// setup binds the beacon config to the e2e network and returns a fake signer
// with numKeys keys and a keymanager talking to it.
func setup(t *testing.T, numKeys int, mutate ...func(*SetupConfig)) (*fakeSigner, *Keymanager) {
	t.Helper()
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.E2ETestConfig())
	signer := newFakeSigner(t, testGVR(), numKeys)
	cfg := &SetupConfig{
		BaseEndpoint:          signer.srv.URL,
		GenesisValidatorsRoot: testGVR(),
		AuthTokenFile:         signer.tokenFile(),
	}
	for _, m := range mutate {
		m(cfg)
	}
	km, err := NewKeymanager(context.Background(), cfg)
	require.NoError(t, err)
	return signer, km
}

func randomRoot(t *testing.T) []byte {
	root := make([]byte, field_params.RootLength)
	_, err := rand.Read(root)
	require.NoError(t, err)
	return root
}

func TestNewKeymanager(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *SetupConfig
		wantErr string
	}{
		{name: "ok", cfg: &SetupConfig{BaseEndpoint: "http://localhost:9000", GenesisValidatorsRoot: testGVR()}},
		{name: "no endpoint", cfg: &SetupConfig{GenesisValidatorsRoot: testGVR()}, wantErr: "remote signer base endpoint is empty"},
		{name: "short gvr", cfg: &SetupConfig{BaseEndpoint: "http://user:secret@localhost:9000", GenesisValidatorsRoot: []byte{1}}, wantErr: "genesis validators root is invalid (1 bytes)"},
		{name: "bad url", cfg: &SetupConfig{BaseEndpoint: "localhost:9000", GenesisValidatorsRoot: testGVR()}, wantErr: "could not create remote signer client"},
		{
			name:    "missing token file",
			cfg:     &SetupConfig{BaseEndpoint: "http://localhost:9000", GenesisValidatorsRoot: testGVR(), AuthTokenFile: "/nonexistent/token"},
			wantErr: "could not read remote signer auth token file",
		},
		{
			name: "https url with an https public keys url",
			cfg: &SetupConfig{
				BaseEndpoint: "https://localhost:9000", GenesisValidatorsRoot: testGVR(),
				PublicKeysURL: "https://localhost:9000/api/v1/consensus/publicKeys",
			},
		},
		{
			name: "https url with an http public keys url",
			cfg: &SetupConfig{
				BaseEndpoint: "https://localhost:9000", GenesisValidatorsRoot: testGVR(),
				PublicKeysURL: "http://user:secret@localhost:9000/api/v1/consensus/publicKeys",
			},
			wantErr: "it must use https, as the remote signer url does",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeymanager(context.Background(), tt.cfg)
			if tt.wantErr != "" {
				require.ErrorContains(t, tt.wantErr, err)
				assert.Equal(t, false, strings.Contains(err.Error(), "secret"), "the error must not print the url")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestKeymanager_Sign(t *testing.T) {
	signer, km := setup(t, 1)
	key := signer.anyKey()
	pubKey := key.PublicKey().Marshal()

	for _, typ := range mock.SignRequestTypes {
		t.Run(typ, func(t *testing.T) {
			req := mock.GetMockSignRequest(typ)
			req.PublicKey = pubKey
			req.SigningRoot = randomRoot(t)
			if reg := req.GetRegistration(); reg != nil {
				reg.Pubkey = pubKey
			}
			sig, err := km.Sign(context.Background(), req)
			require.NoError(t, err)
			ok, err := ml_dsa_87.VerifySignature(sig.Marshal(), [32]byte(req.SigningRoot), key.PublicKey())
			require.NoError(t, err)
			assert.Equal(t, true, ok)
			assert.Equal(t, strings.TrimPrefix(typ, "BLINDED_"), signer.signTypes[len(signer.signTypes)-1])
		})
	}
}

func TestKeymanager_Sign_RejectsSignatureThatDoesNotVerify(t *testing.T) {
	signer, km := setup(t, 1)
	other, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	signer.signWith = other

	req := mock.GetMockSignRequest(mock.Attestation)
	req.PublicKey = signer.anyKey().PublicKey().Marshal()
	req.SigningRoot = randomRoot(t)
	_, err = km.Sign(context.Background(), req)
	require.Equal(t, true, errors.Is(err, ErrInvalidSignature), "got %v", err)
	require.ErrorContains(t, "ATTESTATION", err)
}

func TestKeymanager_Sign_Refusals(t *testing.T) {
	tests := []struct {
		refusal refusal
		want    error
	}{
		{refusal: refusal{status: 412, reason: "SLASHING_DOUBLE_VOTE"}, want: internal.ErrSlashingProtection},
		{refusal: refusal{status: 412, reason: "CLOCK_BOUND"}, want: internal.ErrSlashingProtection},
		{refusal: refusal{status: 403, reason: "ARTIFACT_TYPE_DISABLED"}, want: internal.ErrForbidden},
		{refusal: refusal{status: 503, reason: "OVERLOADED"}, want: internal.ErrSignerUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.refusal.reason, func(t *testing.T) {
			signer, km := setup(t, 1)
			signer.refusal = &tt.refusal
			req := mock.GetMockSignRequest(mock.Attestation)
			req.PublicKey = signer.anyKey().PublicKey().Marshal()
			_, err := km.Sign(context.Background(), req)
			require.Equal(t, true, errors.Is(err, tt.want), "got %v", err)
			assert.Equal(t, tt.refusal.status == 412, errors.Is(err, keymanager.ErrSigningRefused))
			var r keymanager.Reasoner
			require.Equal(t, true, errors.As(err, &r))
			assert.Equal(t, tt.refusal.reason, r.Reason())
			// Sign requests are never retried by the keymanager.
			assert.Equal(t, 1, len(signer.signTypes))
		})
	}
}

// A signer without --voluntary-exit-signing-enabled refuses exits with 403
// ARTIFACT_TYPE_DISABLED. Only that refusal of an exit names the flag: the same
// reason on a registration is a fee recipient pin, and another 403 reason on an
// exit is an access problem.
func TestKeymanager_Sign_VoluntaryExitSigningDisabled(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		refusal  refusal
		disabled bool
	}{
		{name: "exit refused by type policy", typ: mock.VoluntaryExit, refusal: refusal{status: 403, reason: "ARTIFACT_TYPE_DISABLED"}, disabled: true},
		{name: "exit refused for scope", typ: mock.VoluntaryExit, refusal: refusal{status: 403, reason: "FORBIDDEN_SCOPE"}},
		{name: "exit refused by protection", typ: mock.VoluntaryExit, refusal: refusal{status: 412, reason: "CLOCK_BOUND"}},
		{name: "registration refused by fee recipient pin", typ: mock.ValidatorRegistration, refusal: refusal{status: 403, reason: "ARTIFACT_TYPE_DISABLED"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, km := setup(t, 1)
			signer.refusal = &tt.refusal
			pubKey := signer.anyKey().PublicKey().Marshal()
			req := mock.GetMockSignRequest(tt.typ)
			req.PublicKey = pubKey
			if reg := req.GetRegistration(); reg != nil {
				reg.Pubkey = pubKey
			}
			_, err := km.Sign(context.Background(), req)
			require.NotNil(t, err)
			assert.Equal(t, tt.disabled, errors.Is(err, keymanager.ErrVoluntaryExitSigningDisabled), "got %v", err)
			assert.Equal(t, tt.disabled, strings.Contains(err.Error(), "--voluntary-exit-signing-enabled"), "got %v", err)
			// The signer's refusal stays reachable for callers that log its reason.
			var r keymanager.Reasoner
			require.Equal(t, true, errors.As(err, &r))
			assert.Equal(t, tt.refusal.reason, r.Reason())
			assert.Equal(t, tt.refusal.status == 403, errors.Is(err, internal.ErrForbidden))
		})
	}
}

// A signer on another network than the exit command rejects its exits with
// 400 FORK_INFO_MISMATCH (the command's chain config differs) or
// SIGNING_ROOT_MISMATCH (the beacon node's domain differs). Only those
// rejections of an exit match ErrVoluntaryExitNetworkMismatch and name the
// chain config.
func TestKeymanager_Sign_VoluntaryExitNetworkMismatch(t *testing.T) {
	tests := []struct {
		name     string
		typ      string
		refusal  refusal
		mismatch bool
	}{
		{name: "exit with another fork_info", typ: mock.VoluntaryExit, refusal: refusal{status: 400, reason: "FORK_INFO_MISMATCH"}, mismatch: true},
		{name: "exit with another signing root", typ: mock.VoluntaryExit, refusal: refusal{status: 400, reason: "SIGNING_ROOT_MISMATCH"}, mismatch: true},
		{name: "exit with a bad field", typ: mock.VoluntaryExit, refusal: refusal{status: 400, reason: "BAD_FIELD"}},
		{name: "attestation with another fork_info", typ: mock.Attestation, refusal: refusal{status: 400, reason: "FORK_INFO_MISMATCH"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, km := setup(t, 1)
			signer.refusal = &tt.refusal
			req := mock.GetMockSignRequest(tt.typ)
			req.PublicKey = signer.anyKey().PublicKey().Marshal()
			_, err := km.Sign(context.Background(), req)
			require.NotNil(t, err)
			assert.Equal(t, tt.mismatch, errors.Is(err, keymanager.ErrVoluntaryExitNetworkMismatch), "got %v", err)
			assert.Equal(t, tt.mismatch, strings.Contains(err.Error(), "--chain-config-file"), "got %v", err)
			assert.Equal(t, false, errors.Is(err, keymanager.ErrVoluntaryExitSigningDisabled))
			var r keymanager.Reasoner
			require.Equal(t, true, errors.As(err, &r))
			assert.Equal(t, tt.refusal.reason, r.Reason())
			assert.Equal(t, true, errors.Is(err, internal.ErrBadRequest))
		})
	}
}

func TestKeymanager_Sign_UnknownKey(t *testing.T) {
	_, km := setup(t, 1)
	other, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	req := mock.GetMockSignRequest(mock.AggregationSlot)
	req.PublicKey = other.PublicKey().Marshal()
	_, err = km.Sign(context.Background(), req)
	require.Equal(t, true, errors.Is(err, internal.ErrPublicKeyNotFound), "got %v", err)
}

func TestKeymanager_Sign_LocalErrors(t *testing.T) {
	signer, km := setup(t, 1)
	pubKey := signer.anyKey().PublicKey().Marshal()

	epoch := mock.GetMockSignRequest(mock.RandaoReveal)
	epoch.PublicKey = pubKey
	badKey := mock.GetMockSignRequest(mock.AggregationSlot)
	badKey.PublicKey = []byte{1, 2, 3}

	tests := []struct {
		name    string
		req     *validatorpb.SignRequest
		wantErr string
	}{
		{name: "epoch oneof", req: epoch, wantErr: "RANDAO is a hash-onion reveal"},
		{name: "bad public key", req: badKey, wantErr: "invalid public key in sign request"},
		{name: "nil request", req: nil, wantErr: "nil sign request provided"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := km.Sign(context.Background(), tt.req)
			require.ErrorContains(t, tt.wantErr, err)
		})
	}
	assert.Equal(t, 0, len(signer.signTypes), "local errors must not reach the signer")
}

func TestKeymanager_RandaoReveal(t *testing.T) {
	signer, km := setup(t, 1)
	reveal := bytes.Repeat([]byte{0x99}, 32)
	signer.randaoResponses = []randaoResponse{{reveal: reveal}}
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], signer.anyKey().PublicKey().Marshal())
	commitment := [32]byte(bytes.Repeat([]byte{0x33}, 32))

	got, err := km.RandaoReveal(context.Background(), pubKey, commitment, 62)
	require.NoError(t, err)
	assert.DeepEqual(t, [32]byte(reveal), got)

	require.Equal(t, 1, len(signer.randaoBodies))
	var body map[string]any
	require.NoError(t, json.Unmarshal(signer.randaoBodies[0], &body))
	assert.Equal(t, "62", body["slot"])
	assert.Equal(t, hexutil.Encode(commitment[:]), body["randao_commitment"])
}

func TestKeymanager_RandaoReveal_Errors(t *testing.T) {
	saved := defaultRetryDelay
	defaultRetryDelay = 10 * time.Millisecond
	t.Cleanup(func() { defaultRetryDelay = saved })

	ok := randaoResponse{reveal: bytes.Repeat([]byte{0x99}, 32)}
	refuse := func(status int, reason, retryAfter string) randaoResponse {
		return randaoResponse{refusal: &refusal{status: status, reason: reason, retryAfter: retryAfter}}
	}
	tests := []struct {
		name      string
		responses []randaoResponse
		timeout   time.Duration
		want      error
		wantCalls int
	}{
		{
			name:      "unsupported key maps to ErrRandaoRevealUnsupported",
			responses: []randaoResponse{refuse(422, "RANDAO_UNSUPPORTED_FOR_KEY", "")},
			want:      keymanager.ErrRandaoRevealUnsupported,
			wantCalls: 1,
		},
		{
			name:      "unknown commitment",
			responses: []randaoResponse{refuse(422, "RANDAO_UNKNOWN_COMMITMENT", "")},
			want:      internal.ErrRandaoRevealRefused,
			wantCalls: 1,
		},
		{
			name:      "new-layer rate limit is a protection refusal, not retried",
			responses: []randaoResponse{refuse(412, "RANDAO_NEW_LAYER_RATE_LIMITED", "")},
			want:      internal.ErrSlashingProtection,
			wantCalls: 1,
		},
		{
			name:      "429 retried once and succeeds",
			responses: []randaoResponse{refuse(429, "RANDAO_SCAN_RATE_LIMITED", ""), ok},
			wantCalls: 2,
		},
		{
			name:      "503 retried once only",
			responses: []randaoResponse{refuse(503, "OVERLOADED", "")},
			want:      internal.ErrSignerUnavailable,
			wantCalls: 2,
		},
		{
			name:      "no retry when Retry-After outlasts the slot",
			responses: []randaoResponse{refuse(429, "RANDAO_SCAN_RATE_LIMITED", "30"), ok},
			timeout:   time.Second,
			want:      internal.ErrRateLimited,
			wantCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, km := setup(t, 1)
			signer.randaoResponses = tt.responses
			var pubKey [field_params.MLDSA87PubkeyLength]byte
			copy(pubKey[:], signer.anyKey().PublicKey().Marshal())
			ctx := context.Background()
			if tt.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
				defer cancel()
			}
			_, err := km.RandaoReveal(ctx, pubKey, [32]byte{1}, 62)
			if tt.want == nil {
				require.NoError(t, err)
			} else {
				require.Equal(t, true, errors.Is(err, tt.want), "got %v", err)
			}
			assert.Equal(t, tt.wantCalls, len(signer.randaoBodies))
		})
	}
}

func TestKeymanager_FetchValidatingPublicKeys_HappyPath_WithKeyList(t *testing.T) {
	keys := [][field_params.MLDSA87PubkeyLength]byte{{1}, {2}}
	signer, km := setup(t, 1, func(c *SetupConfig) { c.ProvidedPublicKeys = keys })
	got, err := km.FetchValidatingPublicKeys(context.Background())
	require.NoError(t, err)
	assert.DeepEqual(t, keys, got)
	assert.Equal(t, 0, signer.publicKeyCalls)
}

func TestKeymanager_FetchValidatingPublicKeys_HappyPath_WithExternalURL(t *testing.T) {
	signer, km := setup(t, 3, func(c *SetupConfig) { c.PublicKeysURL = c.BaseEndpoint + "/api/v1/consensus/publicKeys" })
	for range 2 {
		got, err := km.FetchValidatingPublicKeys(context.Background())
		require.NoError(t, err)
		require.Equal(t, 3, len(got))
		for _, k := range got {
			_, ok := signer.keys[hexutil.Encode(k[:])]
			assert.Equal(t, true, ok)
		}
	}
	assert.Equal(t, 1, signer.publicKeyCalls, "the public keys URL is fetched once")
}

func TestKeymanager_FetchValidatingPublicKeys_WithExternalURL_ThrowsError(t *testing.T) {
	_, km := setup(t, 1, func(c *SetupConfig) {
		c.PublicKeysURL = c.BaseEndpoint + "/api/v1/consensus/publicKeys"
		c.AuthTokenFile = ""
	})
	_, err := km.FetchValidatingPublicKeys(context.Background())
	require.ErrorContains(t, "could not get public keys from the remote signer public keys url", err)
	require.Equal(t, true, errors.Is(err, internal.ErrUnauthenticated), "got %v", err)
}

// newKeyFileKeymanager returns a keymanager whose keys come from a key file
// holding keys, and the file's path.
func newKeyFileKeymanager(t *testing.T, listen bool, keys ...string) (*Keymanager, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "remote-keys.txt")
	writeKeyFile(t, path, keys...)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	km, err := NewKeymanager(ctx, &SetupConfig{
		BaseEndpoint:          "http://localhost:9000",
		GenesisValidatorsRoot: testGVR(),
		KeyFile:               path,
		ListenForChanges:      listen,
	})
	require.NoError(t, err)
	t.Cleanup(km.Close)
	return km, path
}

func requireKeys(t *testing.T, km *Keymanager, path string, want [][field_params.MLDSA87PubkeyLength]byte) {
	t.Helper()
	got, err := km.FetchValidatingPublicKeys(context.Background())
	require.NoError(t, err)
	require.DeepEqual(t, want, got)
	inFile, err := ReadKeyFile(path)
	require.NoError(t, err)
	require.DeepEqual(t, want, inFile)
}

func TestKeymanager_AddPublicKeys(t *testing.T) {
	keys, hexKeys := testKeys(t, 3)
	km, path := newKeyFileKeymanager(t, false, "# node 0", hexKeys[0])
	changes := make(chan [][field_params.MLDSA87PubkeyLength]byte, 1)
	sub := km.SubscribeAccountChanges(changes)
	defer sub.Unsubscribe()

	statuses, err := km.AddPublicKeys(context.Background(), [][field_params.MLDSA87PubkeyLength]byte{keys[1], keys[0], keys[2], keys[1]})
	require.NoError(t, err)
	wantStatuses := []qrlpbservice.ImportedRemoteKeysStatus_Status{
		qrlpbservice.ImportedRemoteKeysStatus_IMPORTED,
		qrlpbservice.ImportedRemoteKeysStatus_DUPLICATE,
		qrlpbservice.ImportedRemoteKeysStatus_IMPORTED,
		qrlpbservice.ImportedRemoteKeysStatus_DUPLICATE,
	}
	require.Equal(t, len(wantStatuses), len(statuses))
	for i, want := range wantStatuses {
		assert.Equal(t, want, statuses[i].Status, "status %d", i)
	}
	assert.Equal(t, "Successfully added pubkey: "+hexKeys[1], statuses[0].Message)
	assert.Equal(t, "Duplicate pubkey: "+hexKeys[0]+", already in use", statuses[1].Message)

	want := [][field_params.MLDSA87PubkeyLength]byte{keys[0], keys[1], keys[2]}
	requireKeys(t, km, path, want)
	select {
	case got := <-changes:
		require.DeepEqual(t, want, got)
	case <-time.After(5 * time.Second):
		t.Fatal("subscribers were not notified")
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, true, strings.HasPrefix(string(data), "# node 0\n"), "comments are kept")

	statuses, err = km.AddPublicKeys(context.Background(), keys[:1])
	require.NoError(t, err)
	assert.Equal(t, qrlpbservice.ImportedRemoteKeysStatus_DUPLICATE, statuses[0].Status)
	select {
	case <-changes:
		t.Fatal("a duplicate notified subscribers")
	default:
	}
}

func TestKeymanager_DeletePublicKeys(t *testing.T) {
	keys, hexKeys := testKeys(t, 3)
	km, path := newKeyFileKeymanager(t, false, hexKeys[0], hexKeys[1]+" # second", hexKeys[2])
	changes := make(chan [][field_params.MLDSA87PubkeyLength]byte, 1)
	sub := km.SubscribeAccountChanges(changes)
	defer sub.Unsubscribe()

	other, _ := testKeys(t, 4)
	statuses, err := km.DeletePublicKeys(context.Background(), [][field_params.MLDSA87PubkeyLength]byte{keys[1], other[3], keys[1]})
	require.NoError(t, err)
	wantStatuses := []qrlpbservice.DeletedRemoteKeysStatus_Status{
		qrlpbservice.DeletedRemoteKeysStatus_DELETED,
		qrlpbservice.DeletedRemoteKeysStatus_NOT_FOUND,
		qrlpbservice.DeletedRemoteKeysStatus_NOT_FOUND,
	}
	require.Equal(t, len(wantStatuses), len(statuses))
	for i, want := range wantStatuses {
		assert.Equal(t, want, statuses[i].Status, "status %d", i)
	}
	assert.Equal(t, "Successfully deleted pubkey: "+hexKeys[1], statuses[0].Message)

	want := [][field_params.MLDSA87PubkeyLength]byte{keys[0], keys[2]}
	requireKeys(t, km, path, want)
	select {
	case got := <-changes:
		require.DeepEqual(t, want, got)
	case <-time.After(5 * time.Second):
		t.Fatal("subscribers were not notified")
	}

	statuses, err = km.DeletePublicKeys(context.Background(), keys)
	require.NoError(t, err)
	assert.Equal(t, qrlpbservice.DeletedRemoteKeysStatus_DELETED, statuses[0].Status)
	assert.Equal(t, qrlpbservice.DeletedRemoteKeysStatus_NOT_FOUND, statuses[1].Status)
	assert.Equal(t, qrlpbservice.DeletedRemoteKeysStatus_DELETED, statuses[2].Status)
	requireKeys(t, km, path, [][field_params.MLDSA87PubkeyLength]byte{})
}

// A key removed from the key file, through the API or by an edit, is no
// longer signed for, although the validator client keeps its duties until
// the next epoch: Sign and RandaoReveal refuse it without asking the signer.
func TestKeymanager_SignRefusesRemovedKey(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.E2ETestConfig())
	signer := newFakeSigner(t, testGVR(), 1)
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], signer.anyKey().PublicKey().Marshal())
	path := filepath.Join(t.TempDir(), "remote-keys.txt")
	writeKeyFile(t, path, hexutil.Encode(pubKey[:]))
	km, err := NewKeymanager(context.Background(), &SetupConfig{
		BaseEndpoint:          signer.srv.URL,
		GenesisValidatorsRoot: testGVR(),
		AuthTokenFile:         signer.tokenFile(),
		KeyFile:               path,
	})
	require.NoError(t, err)

	sign := func() error {
		req := mock.GetMockSignRequest(mock.Attestation)
		req.PublicKey = pubKey[:]
		req.SigningRoot = randomRoot(t)
		_, err := km.Sign(context.Background(), req)
		return err
	}
	reveal := func() error {
		_, err := km.RandaoReveal(context.Background(), pubKey, [32]byte{1}, 62)
		return err
	}
	requireRefused := func(signs, reveals int) {
		t.Helper()
		err := sign()
		require.Equal(t, true, errors.Is(err, ErrNotValidatingKey), "sign: got %v", err)
		err = reveal()
		require.Equal(t, true, errors.Is(err, ErrNotValidatingKey), "randao reveal: got %v", err)
		assert.Equal(t, signs, len(signer.signTypes), "a refused sign request reaches the signer")
		assert.Equal(t, reveals, len(signer.randaoBodies), "a refused randao reveal reaches the signer")
	}

	require.NoError(t, sign())
	require.NoError(t, reveal())

	statuses, err := km.DeletePublicKeys(context.Background(), [][field_params.MLDSA87PubkeyLength]byte{pubKey})
	require.NoError(t, err)
	require.Equal(t, qrlpbservice.DeletedRemoteKeysStatus_DELETED, statuses[0].Status)
	requireRefused(1, 1)

	statuses2, err := km.AddPublicKeys(context.Background(), [][field_params.MLDSA87PubkeyLength]byte{pubKey})
	require.NoError(t, err)
	require.Equal(t, qrlpbservice.ImportedRemoteKeysStatus_IMPORTED, statuses2[0].Status)
	require.NoError(t, sign())
	require.NoError(t, reveal())

	writeKeyFile(t, path, "# moved to another validator client")
	km.reloadKeyFile()
	requireRefused(2, 2)
}

// Without a key file the keys cannot be changed: a change in memory would be
// undone by a restart.
func TestKeymanager_AddDeletePublicKeys_RequireKeyFile(t *testing.T) {
	keys, _ := testKeys(t, 2)
	tests := []struct {
		name   string
		mutate func(*SetupConfig)
		want   [][field_params.MLDSA87PubkeyLength]byte
	}{
		{
			name:   "key list",
			mutate: func(c *SetupConfig) { c.ProvidedPublicKeys = keys[:1] },
			want:   keys[:1],
		},
		{
			name:   "public keys url",
			mutate: func(c *SetupConfig) { c.PublicKeysURL = c.BaseEndpoint + "/api/v1/consensus/publicKeys" },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, km := setup(t, 1, tt.mutate)
			assert.Equal(t, true, km.KeysReadOnly())
			before, err := km.FetchValidatingPublicKeys(context.Background())
			require.NoError(t, err)
			calls := signer.publicKeyCalls

			_, err = km.AddPublicKeys(context.Background(), keys[1:])
			require.Equal(t, true, errors.Is(err, ErrKeyFileRequired), "got %v", err)
			_, err = km.DeletePublicKeys(context.Background(), before)
			require.Equal(t, true, errors.Is(err, ErrKeyFileRequired), "got %v", err)

			after, err := km.FetchValidatingPublicKeys(context.Background())
			require.NoError(t, err)
			require.DeepEqual(t, before, after)
			if tt.want != nil {
				require.DeepEqual(t, tt.want, after)
			}
			assert.Equal(t, calls, signer.publicKeyCalls, "the public keys url is not fetched again")
		})
	}
}

// A failed key file write changes nothing: neither the file, nor the keys,
// nor subscribers.
func TestKeymanager_AddDeletePublicKeys_WriteFailure(t *testing.T) {
	keys, hexKeys := testKeys(t, 2)
	km, path := newKeyFileKeymanager(t, false, hexKeys[0])
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	changes := make(chan [][field_params.MLDSA87PubkeyLength]byte, 1)
	sub := km.SubscribeAccountChanges(changes)
	defer sub.Unsubscribe()
	renameFile = func(string, string) error { return errors.New("injected rename failure") }
	t.Cleanup(func() { renameFile = os.Rename })

	_, err = km.AddPublicKeys(context.Background(), keys[1:])
	require.ErrorContains(t, "could not write the remote signer key file: injected rename failure", err)
	_, err = km.DeletePublicKeys(context.Background(), keys[:1])
	require.ErrorContains(t, "could not write the remote signer key file: injected rename failure", err)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.DeepEqual(t, before, after)
	requireKeys(t, km, path, keys[:1])
	select {
	case <-changes:
		t.Fatal("a failed write notified subscribers")
	default:
	}
}

// An invalid key file, which the operator may be editing, is not overwritten.
func TestKeymanager_AddPublicKeys_InvalidKeyFile(t *testing.T) {
	keys, hexKeys := testKeys(t, 2)
	km, path := newKeyFileKeymanager(t, false, hexKeys[0])
	writeKeyFile(t, path, hexKeys[0], "0x1234")

	_, err := km.AddPublicKeys(context.Background(), keys[1:])
	require.ErrorContains(t, "invalid remote signer key file", err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, hexKeys[0]+"\n0x1234", string(data))
}

// The key file watcher picks up the API's writes without a second
// notification, and an edit it has not reloaded yet survives an API write.
func TestKeymanager_AddPublicKeys_WithKeyFileWatcher(t *testing.T) {
	old := keyFileReloadDebounce
	keyFileReloadDebounce = 50 * time.Millisecond
	t.Cleanup(func() { keyFileReloadDebounce = old })

	keys, hexKeys := testKeys(t, 3)
	km, path := newKeyFileKeymanager(t, true, hexKeys[0])
	changes := make(chan [][field_params.MLDSA87PubkeyLength]byte, 1)
	sub := km.SubscribeAccountChanges(changes)
	defer sub.Unsubscribe()
	expect := func(want [][field_params.MLDSA87PubkeyLength]byte) {
		t.Helper()
		select {
		case got := <-changes:
			require.DeepEqual(t, want, got)
		case <-time.After(5 * time.Second):
			t.Fatal("subscribers were not notified")
		}
	}

	_, err := km.AddPublicKeys(context.Background(), keys[1:2])
	require.NoError(t, err)
	expect(keys[:2])
	select {
	case got := <-changes:
		t.Fatalf("the watcher notified again with %d keys", len(got))
	case <-time.After(500 * time.Millisecond):
	}
	requireKeys(t, km, path, keys[:2])

	// The watcher still reloads the operator's edits.
	writeKeyFile(t, path, hexKeys[2])
	expect(keys[2:])
}

// An operator's edit that has not been reloaded yet is kept by an API write.
func TestKeymanager_AddPublicKeys_KeepsPendingEdit(t *testing.T) {
	keys, hexKeys := testKeys(t, 3)
	km, path := newKeyFileKeymanager(t, false, hexKeys[0])
	writeKeyFile(t, path, hexKeys[1])

	_, err := km.AddPublicKeys(context.Background(), keys[2:])
	require.NoError(t, err)
	requireKeys(t, km, path, keys[1:])
}

func TestKeymanager_SignerURL(t *testing.T) {
	_, hexKeys := testKeys(t, 1)
	km, _ := newKeyFileKeymanager(t, false, hexKeys[0])
	assert.Equal(t, "http://localhost:9000", km.SignerURL())
	assert.Equal(t, false, km.KeysReadOnly())

	km, err := NewKeymanager(context.Background(), &SetupConfig{
		BaseEndpoint:          "https://user:secret@signer.example:9000/base?token=abc",
		GenesisValidatorsRoot: testGVR(),
	})
	require.NoError(t, err)
	assert.Equal(t, "https://signer.example:9000/base", km.SignerURL())
}

// capabilitiesVector is a case of testdata/vectors/capabilities.json: what the
// client knows of its chain, the signer's response, and whether the client may
// start.
type capabilitiesVector struct {
	Name        string `json:"name"`
	Preset      string `json:"preset"`
	ClientChain struct {
		GenesisForkVersion    string `json:"genesis_fork_version"`
		GenesisValidatorsRoot string `json:"genesis_validators_root"`
		GenesisTime           string `json:"genesis_time"`
		SecondsPerSlot        string `json:"seconds_per_slot"`
		SlotsPerEpoch         string `json:"slots_per_epoch"`
	} `json:"client_chain"`
	Response map[string]any `json:"response"`
	Expect   struct {
		Accept bool   `json:"accept"`
		Check  string `json:"check"`
		Log    string `json:"log"`
	} `json:"expect"`
}

// capabilitiesFields are the fields CheckCapabilities names in a mismatch.
var capabilitiesFields = []string{
	"api_version", "genesis_fork_version", "genesis_validators_root", "genesis_time",
	"seconds_per_slot", "slots_per_epoch", "randao_reveal",
}

// TestKeymanager_CheckCapabilities runs the golden capabilities handshakes:
// the check accepts or refuses each as the vector expects, a refusal names the
// deciding field and no other, and a pending GVR pin is logged at WARN.
func TestKeymanager_CheckCapabilities(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "vectors", "capabilities.json"))
	require.NoError(t, err)
	var vectors struct {
		Cases []capabilitiesVector `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &vectors))
	require.Equal(t, 26, len(vectors.Cases))
	configs := map[string]func() *params.BeaconChainConfig{
		"minimal": params.E2ETestConfig,
		"mainnet": params.E2EMainnetTestConfig,
	}

	for _, tc := range vectors.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			newConfig, ok := configs[tc.Preset]
			require.Equal(t, true, ok, "unknown preset %q", tc.Preset)
			params.SetupTestConfigCleanup(t)
			cfg := newConfig()
			params.OverrideBeaconConfig(cfg)
			// The client's chain is the preset's e2e network.
			chain := tc.ClientChain
			require.Equal(t, hexutil.Encode(cfg.GenesisForkVersion), chain.GenesisForkVersion)
			require.Equal(t, strconv.FormatUint(cfg.SecondsPerSlot, 10), chain.SecondsPerSlot)
			require.Equal(t, strconv.FormatUint(uint64(cfg.SlotsPerEpoch), 10), chain.SlotsPerEpoch)
			gvr, err := hexutil.Decode(chain.GenesisValidatorsRoot)
			require.NoError(t, err)
			genesisTime, err := strconv.ParseUint(chain.GenesisTime, 10, 64)
			require.NoError(t, err)

			signer := newFakeSigner(t, gvr, 0)
			signer.capabilities = tc.Response
			km, err := NewKeymanager(context.Background(), &SetupConfig{
				BaseEndpoint:          signer.srv.URL,
				GenesisValidatorsRoot: gvr,
				AuthTokenFile:         signer.tokenFile(),
			})
			require.NoError(t, err)

			hook := logTest.NewGlobal()
			defer hook.Reset()
			err = km.CheckCapabilities(context.Background(), gvr, genesisTime)
			if tc.Expect.Accept {
				require.NoError(t, err)
			} else {
				require.Equal(t, true, errors.Is(err, ErrCapabilitiesMismatch), "got %v", err)
				for _, field := range capabilitiesFields {
					assert.Equal(t, field == tc.Expect.Check, strings.Contains(err.Error(), field+":"),
						"field %s in %v", field, err)
				}
			}
			var warnings []string
			for _, e := range hook.AllEntries() {
				if e.Level == logrus.WarnLevel {
					warnings = append(warnings, e.Message)
				}
			}
			if tc.Expect.Log == "WARN" {
				require.Equal(t, 1, len(warnings), "warnings %v", warnings)
				require.Equal(t, true, strings.Contains(warnings[0], "has not pinned a genesis validators root"))
			} else {
				require.Equal(t, 0, len(warnings), "warnings %v", warnings)
			}
		})
	}
}

func TestKeymanager_CheckCapabilities_MalformedVersion(t *testing.T) {
	for _, version := range []string{"1.0", "01.0.0", "1.0.0-rc.1", "v1.0.0"} {
		t.Run(version, func(t *testing.T) {
			signer, km := setup(t, 1)
			signer.capabilities["api_version"] = version
			err := km.CheckCapabilities(context.Background(), testGVR(), testGenesisTime)
			require.Equal(t, true, errors.Is(err, ErrCapabilitiesMismatch), "got %v", err)
			require.ErrorContains(t, "not a semantic version", err)
		})
	}
}

func TestKeymanager_CheckCapabilities_ReportsEveryField(t *testing.T) {
	signer, km := setup(t, 1)
	signer.capabilities["randao_reveal"] = false
	signer.capabilities["network"].(map[string]any)["seconds_per_slot"] = "12"
	err := km.CheckCapabilities(context.Background(), testGVR(), testGenesisTime)
	require.ErrorContains(t, "seconds_per_slot: signer 12, validator 10", err)
	require.ErrorContains(t, "randao_reveal", err)

	err = km.CheckCapabilities(context.Background(), []byte{1}, testGenesisTime)
	require.ErrorContains(t, "invalid genesis validators root", err)
}

func TestKeymanager_CheckCapabilities_SignerError(t *testing.T) {
	signer, km := setup(t, 1)
	signer.srv.Close()
	err := km.CheckCapabilities(context.Background(), testGVR(), testGenesisTime)
	require.ErrorContains(t, "could not fetch remote signer capabilities", err)
}

func TestKeymanager_UnsupportedOperations(t *testing.T) {
	_, km := setup(t, 1)
	_, err := km.ExtractKeystores(context.Background(), nil, "")
	require.ErrorContains(t, "not supported", err)
	_, err = km.DeleteKeystores(context.Background(), nil)
	require.ErrorContains(t, "wrong wallet type", err)
}
