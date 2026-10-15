package remote_web3signer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-qrl/common/hexutil"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
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
		{name: "no endpoint", cfg: &SetupConfig{GenesisValidatorsRoot: testGVR()}, wantErr: "invalid setup config"},
		{name: "short gvr", cfg: &SetupConfig{BaseEndpoint: "http://localhost:9000", GenesisValidatorsRoot: []byte{1}}, wantErr: "invalid setup config"},
		{name: "bad url", cfg: &SetupConfig{BaseEndpoint: "localhost:9000", GenesisValidatorsRoot: testGVR()}, wantErr: "could not create remote signer client"},
		{
			name:    "missing token file",
			cfg:     &SetupConfig{BaseEndpoint: "http://localhost:9000", GenesisValidatorsRoot: testGVR(), AuthTokenFile: "/nonexistent/token"},
			wantErr: "could not read remote signer auth token file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewKeymanager(context.Background(), tt.cfg)
			if tt.wantErr != "" {
				require.ErrorContains(t, tt.wantErr, err)
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
			var r keymanager.Reasoner
			require.Equal(t, true, errors.As(err, &r))
			assert.Equal(t, tt.refusal.reason, r.Reason())
			// Sign requests are never retried by the keymanager.
			assert.Equal(t, 1, len(signer.signTypes))
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

func TestKeymanager_CheckCapabilities(t *testing.T) {
	set := func(path string, v any) func(map[string]any) {
		return func(caps map[string]any) {
			if net, ok := strings.CutPrefix(path, "network."); ok {
				caps["network"].(map[string]any)[net] = v
				return
			}
			caps[path] = v
		}
	}
	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantField string
		wantWarn  bool
	}{
		{name: "match"},
		{name: "gvr pin pending", mutate: set("network.genesis_validators_root", nil), wantWarn: true},
		{name: "wrong fork version", mutate: set("network.genesis_fork_version", "0x00000001"), wantField: "genesis_fork_version"},
		{name: "wrong gvr", mutate: set("network.genesis_validators_root", hexutil.Encode(make([]byte, 32))), wantField: "genesis_validators_root"},
		{name: "wrong genesis time", mutate: set("network.genesis_time", "1760000010"), wantField: "genesis_time"},
		{name: "unknown genesis time", mutate: set("network.genesis_time", nil), wantField: "genesis_time: unknown"},
		{name: "wrong seconds per slot", mutate: set("network.seconds_per_slot", "12"), wantField: "seconds_per_slot"},
		{name: "wrong slots per epoch", mutate: set("network.slots_per_epoch", "32"), wantField: "slots_per_epoch"},
		{name: "randao disabled", mutate: set("randao_reveal", false), wantField: "randao_reveal"},
		{name: "unsupported major", mutate: set("api_version", "2.0.0"), wantField: "api_version"},
		{name: "malformed version", mutate: set("api_version", "1.0"), wantField: "api_version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logTest.NewGlobal()
			defer hook.Reset()
			signer, km := setup(t, 1)
			if tt.mutate != nil {
				tt.mutate(signer.capabilities)
			}
			err := km.CheckCapabilities(context.Background(), testGVR(), testGenesisTime)
			if tt.wantField == "" {
				require.NoError(t, err)
			} else {
				require.Equal(t, true, errors.Is(err, ErrCapabilitiesMismatch), "got %v", err)
				require.ErrorContains(t, tt.wantField, err)
			}
			if tt.wantWarn {
				require.LogsContain(t, hook, "has not pinned a genesis validators root")
			}
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
