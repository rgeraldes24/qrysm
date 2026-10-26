package internal_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/internal"
)

var (
	testPubKey    = "0x" + strings.Repeat("a5", 2592)
	testSignature = "0x" + strings.Repeat("5a", 4627)
	testToken     = "0x" + strings.Repeat("ab", 32)
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func newClient(t *testing.T, url string, mutate ...func(*internal.Config)) *internal.ApiClient {
	t.Helper()
	cfg := internal.Config{BaseEndpoint: url, AuthTokenFile: writeFile(t, "token", testToken+"\n")}
	for _, m := range mutate {
		m(&cfg)
	}
	c, err := internal.NewApiClient(cfg)
	require.NoError(t, err)
	return c
}

func jsonResponse(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func TestNewApiClient(t *testing.T) {
	tokenFile := writeFile(t, "token", testToken)
	tests := []struct {
		name    string
		cfg     internal.Config
		wantErr string
	}{
		{name: "http", cfg: internal.Config{BaseEndpoint: "http://localhost:9000"}},
		{name: "https with token", cfg: internal.Config{BaseEndpoint: "https://signer:9000/", AuthTokenFile: tokenFile}},
		{name: "no scheme", cfg: internal.Config{BaseEndpoint: "localhost:9000"}, wantErr: "invalid remote signer url"},
		{name: "wrong scheme", cfg: internal.Config{BaseEndpoint: "ftp://localhost:9000"}, wantErr: "invalid remote signer url"},
		{name: "empty", cfg: internal.Config{}, wantErr: "invalid remote signer url"},
		{
			name:    "missing token file",
			cfg:     internal.Config{BaseEndpoint: "http://localhost:9000", AuthTokenFile: filepath.Join(t.TempDir(), "absent")},
			wantErr: "could not read remote signer auth token file",
		},
		{
			name:    "empty token file",
			cfg:     internal.Config{BaseEndpoint: "http://localhost:9000", AuthTokenFile: writeFile(t, "empty", " \n")},
			wantErr: "remote signer auth token file is empty",
		},
		{
			name:    "two tokens",
			cfg:     internal.Config{BaseEndpoint: "http://localhost:9000", AuthTokenFile: writeFile(t, "two", "a b")},
			wantErr: "must hold a single token",
		},
		{
			name:    "tls files with http",
			cfg:     internal.Config{BaseEndpoint: "http://localhost:9000", TLSCAFile: writeFile(t, "ca.pem", selfSignedPEM(t))},
			wantErr: "the url does not use https",
		},
		{
			name:    "client cert without key",
			cfg:     internal.Config{BaseEndpoint: "https://localhost:9000", TLSClientCertFile: tokenFile},
			wantErr: "certificate and key must be set together",
		},
		{
			name:    "ca file without certificates",
			cfg:     internal.Config{BaseEndpoint: "https://localhost:9000", TLSCAFile: tokenFile},
			wantErr: "contains no PEM certificate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := internal.NewApiClient(tt.cfg)
			if tt.wantErr != "" {
				require.ErrorContains(t, tt.wantErr, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, c)
		})
	}
}

func TestClient_Sign_HappyPath(t *testing.T) {
	requestBody := []byte(`{"type":"AGGREGATION_SLOT"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/consensus/sign/"+testPubKey, r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.DeepEqual(t, requestBody, body)
		jsonResponse(w, http.StatusOK, fmt.Sprintf(`{"signature":"%s"}`, testSignature))
	}))
	defer srv.Close()

	sig, err := newClient(t, srv.URL).Sign(context.Background(), testPubKey, requestBody)
	require.NoError(t, err)
	assert.Equal(t, testSignature, hexutil.Encode(sig.Marshal()))
}

func TestClient_Sign_NoTokenWithoutTokenFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "", r.Header.Get("Authorization"))
		jsonResponse(w, http.StatusOK, fmt.Sprintf(`{"signature":"%s"}`, testSignature))
	}))
	defer srv.Close()
	c, err := internal.NewApiClient(internal.Config{BaseEndpoint: srv.URL})
	require.NoError(t, err)
	_, err = c.Sign(context.Background(), testPubKey, []byte(`{}`))
	require.NoError(t, err)
}

func TestClient_Sign_StrictResponse(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     string
	}{
		{
			name:        "text/plain signature",
			contentType: "text/plain",
			body:        testSignature,
			wantErr:     "want application/json",
		},
		{
			name:        "unknown field",
			contentType: "application/json",
			body:        fmt.Sprintf(`{"signature":"%s","extra":1}`, testSignature),
			wantErr:     "unknown field",
		},
		{
			name:        "trailing data",
			contentType: "application/json",
			body:        fmt.Sprintf(`{"signature":"%s"}{}`, testSignature),
			wantErr:     "trailing data",
		},
		{
			name:        "short signature",
			contentType: "application/json",
			body:        `{"signature":"0x0102"}`,
			wantErr:     "2-byte signature",
		},
		{
			name:        "oversized body",
			contentType: "application/json",
			body:        `{"signature":"` + strings.Repeat("0", 20<<10) + `"}`,
			wantErr:     "response exceeds",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()
			_, err := newClient(t, srv.URL).Sign(context.Background(), testPubKey, []byte(`{}`))
			require.ErrorContains(t, tt.wantErr, err)
		})
	}
}

// TestClient_ErrorMapping covers every status code of the contract's error
// catalogue and its mapping to a client error.
func TestClient_ErrorMapping(t *testing.T) {
	tests := []struct {
		status     int
		reason     string
		retryAfter string
		want       error
		wantRetry  time.Duration
	}{
		{status: 400, reason: "SIGNING_ROOT_MISMATCH", want: internal.ErrBadRequest},
		{status: 400, reason: "UNKNOWN_FIELD", want: internal.ErrBadRequest},
		{status: 401, reason: "UNAUTHENTICATED", want: internal.ErrUnauthenticated},
		{status: 403, reason: "ARTIFACT_TYPE_DISABLED", want: internal.ErrForbidden},
		{status: 404, reason: "UNKNOWN_KEY", want: internal.ErrPublicKeyNotFound},
		{status: 404, reason: "PROTECTION_DISABLED", want: internal.ErrUnexpectedResponse},
		{status: 404, want: internal.ErrRouteNotFound},
		{status: 412, reason: "SLASHING_DOUBLE_VOTE", want: internal.ErrSlashingProtection},
		{status: 412, reason: "CLOCK_BOUND", want: internal.ErrSlashingProtection},
		{status: 412, reason: "RANDAO_SLOT_NOT_INCREASING", want: internal.ErrSlashingProtection},
		{status: 412, reason: "RANDAO_NEW_LAYER_RATE_LIMITED", want: internal.ErrSlashingProtection},
		{status: 413, reason: "BODY_TOO_LARGE", want: internal.ErrRequestTooLarge},
		{status: 415, reason: "UNSUPPORTED_MEDIA_TYPE", want: internal.ErrUnsupportedMediaType},
		{status: 422, reason: "RANDAO_UNSUPPORTED_FOR_KEY", want: keymanager.ErrRandaoRevealUnsupported},
		{status: 422, reason: "RANDAO_UNKNOWN_COMMITMENT", want: internal.ErrRandaoRevealRefused},
		{status: 422, reason: "RANDAO_ONION_EXHAUSTED", want: internal.ErrRandaoRevealRefused},
		{status: 429, reason: "RANDAO_SCAN_RATE_LIMITED", retryAfter: "3", want: internal.ErrRateLimited, wantRetry: 3 * time.Second},
		{status: 500, reason: "SIGNATURE_FAULT", want: internal.ErrSignerInternal},
		{status: 503, reason: "OVERLOADED", retryAfter: "1", want: internal.ErrSignerUnavailable, wantRetry: time.Second},
		{status: 503, reason: "KEY_QUARANTINED", want: internal.ErrSignerUnavailable},
		{status: 409, reason: "RELOAD_IN_PROGRESS", want: internal.ErrUnexpectedResponse},
		{status: 502, want: internal.ErrUnexpectedResponse},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d %s", tt.status, tt.reason), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
				if tt.reason == "" {
					w.WriteHeader(tt.status)
					return
				}
				jsonResponse(w, tt.status, fmt.Sprintf(`{"code":%d,"message":"fixed message","reason":"%s"}`, tt.status, tt.reason))
			}))
			defer srv.Close()
			c := newClient(t, srv.URL)

			_, err := c.Sign(context.Background(), testPubKey, []byte(`{}`))
			require.Equal(t, true, errors.Is(err, tt.want), "got %v", err)
			var signerErr *internal.SignerError
			require.Equal(t, true, errors.As(err, &signerErr))
			assert.Equal(t, tt.status, signerErr.StatusCode)
			assert.Equal(t, tt.reason, signerErr.Reason())
			assert.Equal(t, tt.wantRetry, signerErr.RetryAfter)
			var reasoner keymanager.Reasoner
			require.Equal(t, true, errors.As(err, &reasoner))
			assert.Equal(t, tt.status == http.StatusPreconditionFailed, errors.Is(err, keymanager.ErrSigningRefused),
				"only a 412 is a protection refusal")
			assert.Equal(t, false, strings.Contains(err.Error(), testPubKey[:20]), "error must not name the URL")

			// The RANDAO route maps the same way.
			_, err = c.RandaoReveal(context.Background(), testPubKey, []byte(`{}`))
			require.Equal(t, true, errors.Is(err, tt.want), "got %v", err)
		})
	}
}

// TestClient_RouteNotFound checks that a 404 without a reason, the answer of
// the signer or of a proxy to a path that matches no route, is not reported as
// a missing key.
func TestClient_RouteNotFound(t *testing.T) {
	bodies := map[string]func(w http.ResponseWriter){
		"signer": func(w http.ResponseWriter) {
			jsonResponse(w, http.StatusNotFound, `{"code":404,"message":"Not Found"}`)
		},
		"proxy": func(w http.ResponseWriter) {
			http.Error(w, "404 page not found", http.StatusNotFound)
		},
	}
	for name, write := range bodies {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { write(w) }))
			defer srv.Close()
			_, err := newClient(t, srv.URL).Capabilities(context.Background())
			require.Equal(t, true, errors.Is(err, internal.ErrRouteNotFound), "got %v", err)
			assert.Equal(t, false, errors.Is(err, internal.ErrPublicKeyNotFound))
			require.ErrorContains(t, "check the remote signer url", err)
		})
	}
}

// TestClient_RefusalLogging checks that a non-2xx response is logged once:
// at DEBUG on the sign and randao_reveal routes, whose callers log the error
// with its reason, and otherwise at WARN for a 412 or 429 and at ERROR for
// anything else. No log line carries the token, the key or the body.
func TestClient_RefusalLogging(t *testing.T) {
	type call struct {
		route string
		do    func(c *internal.ApiClient) error
	}
	sign := call{"sign", func(c *internal.ApiClient) error {
		_, err := c.Sign(context.Background(), testPubKey, []byte(`{"secret":"body"}`))
		return err
	}}
	reveal := call{"randao_reveal", func(c *internal.ApiClient) error {
		_, err := c.RandaoReveal(context.Background(), testPubKey, []byte(`{"secret":"body"}`))
		return err
	}}
	capabilities := call{"capabilities", func(c *internal.ApiClient) error {
		_, err := c.Capabilities(context.Background())
		return err
	}}
	tests := []struct {
		call   call
		status int
		reason string
		level  logrus.Level
	}{
		{call: sign, status: 412, reason: "SLASHING_DOUBLE_PROPOSAL", level: logrus.DebugLevel},
		{call: sign, status: 400, reason: "SIGNING_ROOT_MISMATCH", level: logrus.DebugLevel},
		{call: sign, status: 500, reason: "INTERNAL", level: logrus.DebugLevel},
		{call: reveal, status: 412, reason: "RANDAO_NEW_LAYER_RATE_LIMITED", level: logrus.DebugLevel},
		{call: reveal, status: 429, reason: "RANDAO_SCAN_RATE_LIMITED", level: logrus.DebugLevel},
		{call: capabilities, status: 429, reason: "RANDAO_SCAN_RATE_LIMITED", level: logrus.WarnLevel},
		{call: capabilities, status: 401, reason: "UNAUTHENTICATED", level: logrus.ErrorLevel},
		{call: capabilities, status: 503, reason: "NOT_READY", level: logrus.ErrorLevel},
	}
	level := logrus.GetLevel()
	logrus.SetLevel(logrus.DebugLevel)
	defer logrus.SetLevel(level)
	for _, tt := range tests {
		t.Run(tt.call.route+" "+tt.reason, func(t *testing.T) {
			hook := logTest.NewGlobal()
			defer hook.Reset()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, tt.status, fmt.Sprintf(`{"code":%d,"message":"m","reason":"%s"}`, tt.status, tt.reason))
			}))
			defer srv.Close()
			require.NotNil(t, tt.call.do(newClient(t, srv.URL)))

			entries := hook.AllEntries()
			require.Equal(t, 1, len(entries))
			e := entries[0]
			assert.Equal(t, tt.level, e.Level)
			assert.Equal(t, tt.reason, e.Data["reason"])
			assert.Equal(t, tt.call.route, e.Data["route"])
			line, err := e.String()
			require.NoError(t, err)
			for _, leaked := range []string{testToken, testPubKey[:40], "secret", "Authorization"} {
				assert.Equal(t, false, strings.Contains(line, leaked), "log line leaks %q", leaked)
			}
		})
	}
}

func TestClient_TransportErrorDoesNotLeakURL(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	_, err := newClient(t, url).Sign(context.Background(), testPubKey, []byte(`{}`))
	require.ErrorContains(t, "remote signer sign request failed", err)
	assert.Equal(t, false, strings.Contains(err.Error(), testPubKey[:20]))
}

func TestClient_Timeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)
	c := newClient(t, srv.URL, func(cfg *internal.Config) { cfg.Timeout = 50 * time.Millisecond })
	_, err := c.Sign(context.Background(), testPubKey, []byte(`{}`))
	require.NotNil(t, err)
}

func TestClient_RandaoReveal(t *testing.T) {
	reveal := "0x" + strings.Repeat("ee", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/consensus/randao_reveal/"+testPubKey, r.URL.Path)
		assert.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
		jsonResponse(w, http.StatusOK, fmt.Sprintf(`{"randao_reveal":"%s"}`, reveal))
	}))
	defer srv.Close()
	got, err := newClient(t, srv.URL).RandaoReveal(context.Background(), testPubKey, []byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, reveal, hexutil.Encode(got[:]))

	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, `{"randao_reveal":"0x01"}`)
	}))
	defer short.Close()
	_, err = newClient(t, short.URL).RandaoReveal(context.Background(), testPubKey, []byte(`{}`))
	require.ErrorContains(t, "1-byte randao reveal", err)
}

func TestClient_GetPublicKeys_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/consensus/publicKeys", r.URL.Path)
		assert.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
		jsonResponse(w, http.StatusOK, fmt.Sprintf(`["%s","%s"]`, testPubKey, "0x"+strings.ToUpper(testPubKey[2:])))
	}))
	defer srv.Close()
	keys, err := newClient(t, srv.URL).GetPublicKeys(context.Background(), srv.URL+"/api/v1/consensus/publicKeys")
	require.NoError(t, err)
	require.Equal(t, 2, len(keys))
	assert.Equal(t, testPubKey, hexutil.Encode(keys[0][:]))
	assert.Equal(t, testPubKey, hexutil.Encode(keys[1][:]))
}

func TestClient_GetPublicKeys_EncodingError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No 0x prefix, too short, not hex.
		jsonResponse(w, http.StatusOK, fmt.Sprintf(`["%s","0xfb6b","0x%s","%s"]`, testPubKey[2:], strings.Repeat("z", 5184), testPubKey))
	}))
	defer srv.Close()
	_, err := newClient(t, srv.URL).GetPublicKeys(context.Background(), srv.URL+"/api/v1/consensus/publicKeys")
	require.ErrorContains(t, "malformed public keys at positions: 0, 1, 2", err)
}

func TestClient_GetPublicKeys_SendsToken(t *testing.T) {
	var gotAuth atomic.Value
	keysSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		jsonResponse(w, http.StatusOK, `[]`)
	}))
	defer keysSrv.Close()
	// The public keys URL is operator-supplied and may spell the signer's
	// host differently from the base URL; it still gets the sign token.
	c := newClient(t, "http://signer.invalid:9000")
	keys, err := c.GetPublicKeys(context.Background(), keysSrv.URL+"/api/v1/consensus/publicKeys")
	require.NoError(t, err)
	assert.Equal(t, 0, len(keys))
	assert.Equal(t, "Bearer "+testToken, gotAuth.Load())

	_, err = c.GetPublicKeys(context.Background(), "not a url")
	require.ErrorContains(t, "invalid remote signer public keys url", err)
}

// capabilitiesResponses returns the signer's capabilities response of every
// case of the golden capabilities vectors, by case name.
func capabilitiesResponses(t *testing.T) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile("../testdata/vectors/capabilities.json")
	require.NoError(t, err)
	var vectors struct {
		Cases []struct {
			Name     string          `json:"name"`
			Response json.RawMessage `json:"response"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &vectors))
	responses := make(map[string][]byte, len(vectors.Cases))
	for _, c := range vectors.Cases {
		responses[c.Name] = c.Response
	}
	return responses
}

func TestClient_Capabilities(t *testing.T) {
	responses := capabilitiesResponses(t)
	require.Equal(t, 26, len(responses))
	for name, body := range responses {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/v1/consensus/capabilities", r.URL.Path)
				assert.Equal(t, "Bearer "+testToken, r.Header.Get("Authorization"))
				jsonResponse(w, http.StatusOK, string(body))
			}))
			defer srv.Close()
			caps, err := newClient(t, srv.URL).Capabilities(context.Background())
			require.NoError(t, err)

			// The decoded response is the vector's response.
			var want, got map[string]any
			require.NoError(t, json.Unmarshal(body, &want))
			encoded, err := json.Marshal(caps)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(encoded, &got))
			assert.DeepEqual(t, want, got)
		})
	}

	caps, err := func() (*internal.Capabilities, error) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, http.StatusOK, string(responses["minimal/matching"]))
		}))
		defer srv.Close()
		return newClient(t, srv.URL).Capabilities(context.Background())
	}()
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", caps.APIVersion)
	assert.Equal(t, true, caps.RandaoReveal)
	assert.Equal(t, 8, len(caps.Types))
	assert.Equal(t, "end-to-end", caps.Network.ConfigName)
	assert.Equal(t, "0x000000fd", hexutil.Encode(caps.Network.GenesisForkVersion))
	require.NotNil(t, caps.Network.GenesisValidatorsRoot)
	assert.Equal(t, "0x9471e8fb19e0faae953e9d46889f207192bcec9b14293d7ba418ae4dc5e3ac8f", hexutil.Encode(*caps.Network.GenesisValidatorsRoot))
	require.NotNil(t, caps.Network.GenesisTime)
	assert.Equal(t, "1760000000", *caps.Network.GenesisTime)
	assert.Equal(t, "10", caps.Network.SecondsPerSlot)
	assert.Equal(t, "6", caps.Network.SlotsPerEpoch)

	strict := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		require.NoError(t, json.Unmarshal(responses["minimal/matching"], &m))
		m["new_field"] = true
		b, err := json.Marshal(m)
		require.NoError(t, err)
		jsonResponse(w, http.StatusOK, string(b))
	}))
	defer strict.Close()
	_, err = newClient(t, strict.URL).Capabilities(context.Background())
	require.ErrorContains(t, "unknown field", err)
}

func TestClient_GetServerStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/upcheck", r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "OK")
	}))
	defer srv.Close()
	status, err := newClient(t, srv.URL).GetServerStatus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "OK", status)

	loading := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "LOADING")
	}))
	defer loading.Close()
	_, err = newClient(t, loading.URL).GetServerStatus(context.Background())
	require.Equal(t, true, errors.Is(err, internal.ErrSignerUnavailable), "got %v", err)
}

func TestClient_TLS(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) == 0 {
			jsonResponse(w, http.StatusUnauthorized, `{"code":401,"message":"m","reason":"UNAUTHENTICATED"}`)
			return
		}
		jsonResponse(w, http.StatusOK, fmt.Sprintf(`{"signature":"%s"}`, testSignature))
	}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	srv.StartTLS()
	defer srv.Close()

	caFile := writeFile(t, "ca.pem", string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})))
	certFile, keyFile := clientCertFiles(t)

	// Without the CA the server certificate is untrusted.
	_, err := newClient(t, srv.URL).Sign(context.Background(), testPubKey, []byte(`{}`))
	require.NotNil(t, err)

	// With the CA but no client certificate the server refuses.
	_, err = newClient(t, srv.URL, func(c *internal.Config) { c.TLSCAFile = caFile }).
		Sign(context.Background(), testPubKey, []byte(`{}`))
	require.Equal(t, true, errors.Is(err, internal.ErrUnauthenticated), "got %v", err)

	// Mutual TLS.
	_, err = newClient(t, srv.URL, func(c *internal.Config) {
		c.TLSCAFile = caFile
		c.TLSClientCertFile = certFile
		c.TLSClientKeyFile = keyFile
	}).Sign(context.Background(), testPubKey, []byte(`{}`))
	require.NoError(t, err)
}

func TestClient_TLS_RefusesTLS12(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, http.StatusOK, fmt.Sprintf(`{"signature":"%s"}`, testSignature))
	}))
	srv.TLS = &tls.Config{MaxVersion: tls.VersionTLS12}
	srv.StartTLS()
	defer srv.Close()
	caFile := writeFile(t, "ca.pem", string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})))

	tests := []struct {
		name   string
		mutate func(*internal.Config)
	}{
		{name: "no tls files", mutate: func(*internal.Config) {}},
		{name: "ca file", mutate: func(c *internal.Config) { c.TLSCAFile = caFile }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newClient(t, srv.URL, tt.mutate).Sign(context.Background(), testPubKey, []byte(`{}`))
			require.ErrorContains(t, "protocol version", err)
		})
	}
}

func selfSignedPEM(t *testing.T) string {
	cert, _ := selfSigned(t)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}))
}

func clientCertFiles(t *testing.T) (string, string) {
	cert, key := selfSigned(t)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certFile := writeFile(t, "client.pem", string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})))
	keyFile := writeFile(t, "client.key", string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})))
	return certFile, keyFile
}

func selfSigned(t *testing.T) ([]byte, *ecdsa.PrivateKey) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "validator"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return der, key
}
