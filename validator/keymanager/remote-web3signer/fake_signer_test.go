package remote_web3signer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1/mock"
)

const fakeToken = "0x00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

// fakeSigner is an httptest stand-in for the remote signer. It enforces the
// contract the way the signer does (bearer token, content negotiation, strict
// request decoding, fork_info bound to the e2e network) and signs signing
// roots with real ML-DSA-87 keys.
type fakeSigner struct {
	t   *testing.T
	srv *httptest.Server
	gvr []byte

	mu   sync.Mutex
	keys map[string]ml_dsa_87.MLDSA87Key // canonical hex pubkey -> key
	// signWith, when set, signs every request with this key instead.
	signWith ml_dsa_87.MLDSA87Key
	// refusal, when set, answers every sign request with this status and reason.
	refusal *refusal
	// randaoResponses are served in order to RANDAO reveal requests; the last
	// one repeats.
	randaoResponses []randaoResponse
	capabilities    map[string]any

	signTypes      []string
	randaoBodies   [][]byte
	publicKeyCalls int
}

type refusal struct {
	status     int
	reason     string
	retryAfter string
}

type randaoResponse struct {
	reveal  []byte
	refusal *refusal
}

func newFakeSigner(t *testing.T, gvr []byte, numKeys int) *fakeSigner {
	f := &fakeSigner{t: t, gvr: gvr, keys: map[string]ml_dsa_87.MLDSA87Key{}}
	for range numKeys {
		key, err := ml_dsa_87.RandKey()
		if err != nil {
			t.Fatal(err)
		}
		f.keys[hexutil.Encode(key.PublicKey().Marshal())] = key
	}
	f.capabilities = e2eCapabilities(gvr)
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// e2eCapabilities is the contract's capabilities example for the e2e network.
func e2eCapabilities(gvr []byte) map[string]any {
	cfg := params.BeaconConfig()
	return map[string]any{
		"api_version":          "1.0.0",
		"types":                []string{"BLOCK_V2", "ATTESTATION"},
		"randao_reveal":        true,
		"selection_proof_mode": "deterministic",
		"slashing_protection":  "bolt",
		"network": map[string]any{
			"config_name":             cfg.ConfigName,
			"genesis_fork_version":    hexutil.Encode(cfg.GenesisForkVersion),
			"genesis_validators_root": hexutil.Encode(gvr),
			"genesis_time":            "1760000000",
			"seconds_per_slot":        fmt.Sprint(cfg.SecondsPerSlot),
			"slots_per_epoch":         fmt.Sprint(cfg.SlotsPerEpoch),
		},
	}
}

func (f *fakeSigner) tokenFile() string {
	path := filepath.Join(f.t.TempDir(), "auth-token")
	if err := os.WriteFile(path, []byte(fakeToken+"\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return path
}

func (f *fakeSigner) anyKey() ml_dsa_87.MLDSA87Key {
	for _, k := range f.keys {
		return k
	}
	return nil
}

func writeError(w http.ResponseWriter, r refusal) {
	if r.retryAfter != "" {
		w.Header().Set("Retry-After", r.retryAfter)
	}
	writeJSON(w, r.status, map[string]any{"code": r.status, "message": "refused", "reason": r.reason})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeSigner) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+fakeToken {
		writeError(w, refusal{status: http.StatusUnauthorized, reason: "UNAUTHENTICATED"})
		return
	}
	if r.Header.Get("Accept") != "application/json" {
		writeError(w, refusal{status: http.StatusBadRequest, reason: "MALFORMED_JSON"})
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/consensus/sign/"):
		f.sign(w, r, strings.TrimPrefix(r.URL.Path, "/api/v1/consensus/sign/"))
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/consensus/randao_reveal/"):
		f.randaoReveal(w, r, strings.TrimPrefix(r.URL.Path, "/api/v1/consensus/randao_reveal/"))
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/consensus/publicKeys":
		f.publicKeyCalls++
		keys := make([]string, 0, len(f.keys))
		for k := range f.keys {
			keys = append(keys, k)
		}
		writeJSON(w, http.StatusOK, keys)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/consensus/capabilities":
		writeJSON(w, http.StatusOK, f.capabilities)
	default:
		// A path that matches no route gets the signer's 404 without a reason.
		writeJSON(w, http.StatusNotFound, map[string]any{"code": http.StatusNotFound, "message": "Not Found"})
	}
}

// readRequest checks the content type and identifier and returns the body.
func (f *fakeSigner) readRequest(w http.ResponseWriter, r *http.Request, identifier string) ([]byte, ml_dsa_87.MLDSA87Key, bool) {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, refusal{status: http.StatusUnsupportedMediaType, reason: "UNSUPPORTED_MEDIA_TYPE"})
		return nil, nil, false
	}
	if len(identifier) != 2+2*2592 || !strings.HasPrefix(identifier, "0x") {
		writeError(w, refusal{status: http.StatusBadRequest, reason: "BAD_IDENTIFIER"})
		return nil, nil, false
	}
	key, ok := f.keys[strings.ToLower(identifier)]
	if !ok {
		writeError(w, refusal{status: http.StatusNotFound, reason: "UNKNOWN_KEY"})
		return nil, nil, false
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, refusal{status: http.StatusBadRequest, reason: "MALFORMED_JSON"})
		return nil, nil, false
	}
	return body, key, true
}

// checkForkInfo requires fork_info to be {G, G, "0"} and the bound GVR.
func (f *fakeSigner) checkForkInfo(forkInfo json.RawMessage) bool {
	var fi struct {
		Fork struct {
			PreviousVersion string `json:"previous_version"`
			CurrentVersion  string `json:"current_version"`
			Epoch           string `json:"epoch"`
		} `json:"fork"`
		GenesisValidatorsRoot string `json:"genesis_validators_root"`
	}
	if err := json.Unmarshal(forkInfo, &fi); err != nil {
		return false
	}
	genesis := hexutil.Encode(params.BeaconConfig().GenesisForkVersion)
	return fi.Fork.PreviousVersion == genesis && fi.Fork.CurrentVersion == genesis && fi.Fork.Epoch == "0" &&
		fi.GenesisValidatorsRoot == hexutil.Encode(f.gvr)
}

func (f *fakeSigner) sign(w http.ResponseWriter, r *http.Request, identifier string) {
	body, key, ok := f.readRequest(w, r, identifier)
	if !ok {
		return
	}
	typ, err := mock.ValidateSignRequestJSON(body)
	if err != nil {
		f.t.Errorf("sign request violates the contract: %v", err)
		writeError(w, refusal{status: http.StatusBadRequest, reason: "BAD_FIELD"})
		return
	}
	var envelope struct {
		ForkInfo    json.RawMessage `json:"fork_info"`
		SigningRoot string          `json:"signing_root"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		writeError(w, refusal{status: http.StatusBadRequest, reason: "MALFORMED_JSON"})
		return
	}
	if typ != "VALIDATOR_REGISTRATION" && !f.checkForkInfo(envelope.ForkInfo) {
		writeError(w, refusal{status: http.StatusBadRequest, reason: "FORK_INFO_MISMATCH"})
		return
	}
	f.signTypes = append(f.signTypes, typ)
	if f.refusal != nil {
		writeError(w, *f.refusal)
		return
	}
	root, err := hexutil.Decode(envelope.SigningRoot)
	if err != nil {
		writeError(w, refusal{status: http.StatusBadRequest, reason: "BAD_FIELD"})
		return
	}
	if f.signWith != nil {
		key = f.signWith
	}
	sig, err := key.Sign(root)
	if err != nil {
		writeError(w, refusal{status: http.StatusInternalServerError, reason: "INTERNAL"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"signature": hexutil.Encode(sig.Marshal())})
}

func (f *fakeSigner) randaoReveal(w http.ResponseWriter, r *http.Request, identifier string) {
	body, _, ok := f.readRequest(w, r, identifier)
	if !ok {
		return
	}
	if err := mock.ValidateRandaoRevealRequestJSON(body); err != nil {
		f.t.Errorf("randao reveal request violates the contract: %v", err)
		writeError(w, refusal{status: http.StatusBadRequest, reason: "BAD_FIELD"})
		return
	}
	var req struct {
		ForkInfo json.RawMessage `json:"fork_info"`
	}
	if err := json.Unmarshal(body, &req); err != nil || !f.checkForkInfo(req.ForkInfo) {
		writeError(w, refusal{status: http.StatusBadRequest, reason: "FORK_INFO_MISMATCH"})
		return
	}
	f.randaoBodies = append(f.randaoBodies, body)
	resp := randaoResponse{reveal: bytes.Repeat([]byte{0xee}, 32)}
	if n := len(f.randaoResponses); n > 0 {
		resp = f.randaoResponses[min(len(f.randaoBodies), n)-1]
	}
	if resp.refusal != nil {
		writeError(w, *resp.refusal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"randao_reveal": hexutil.Encode(resp.reveal)})
}
