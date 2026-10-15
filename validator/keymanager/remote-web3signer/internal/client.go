package internal

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/common/hexutil"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/monitoring/tracing"
	"go.opencensus.io/trace"
)

const (
	apiNamespace     = "/api/v1/consensus/"
	signPath         = apiNamespace + "sign/"
	randaoRevealPath = apiNamespace + "randao_reveal/"
	capabilitiesPath = apiNamespace + "capabilities"
	upcheckPath      = "/upcheck"

	// DefaultTimeout bounds a single request to the remote signer when the
	// caller does not configure one.
	DefaultTimeout = 10 * time.Second
	// publicKeysTimeout bounds the public keys request, whose response is
	// about 5 KB per key (21 MB for 4,096 keys).
	publicKeysTimeout = 2 * time.Minute

	// Response body limits. A signature response is about 9.3 KB, a public
	// key list about 5.2 KB per key.
	maxSignatureResponseBytes  = 16 << 10
	maxSmallResponseBytes      = 16 << 10
	maxPublicKeysResponseBytes = 64 << 20
	maxResponseHeaderBytes     = 64 << 10

	maxIdleConnsPerHost = 160
	idleConnTimeout     = 90 * time.Second
	dialTimeout         = 5 * time.Second
	tlsHandshakeTimeout = 10 * time.Second
)

// SignRequestJson is the JSON body of a sign request.
type SignRequestJson []byte

// RandaoRevealRequestJson is the JSON body of a RANDAO reveal request.
type RandaoRevealRequestJson []byte

// SignatureResponse is the JSON body of a successful sign response.
type SignatureResponse struct {
	Signature hexutil.Bytes `json:"signature"`
}

// RandaoRevealResponse is the JSON body of a successful RANDAO reveal response.
type RandaoRevealResponse struct {
	RandaoReveal hexutil.Bytes `json:"randao_reveal"`
}

// Capabilities is the remote signer's startup handshake response.
type Capabilities struct {
	APIVersion         string              `json:"api_version"`
	Types              []string            `json:"types"`
	RandaoReveal       bool                `json:"randao_reveal"`
	SelectionProofMode string              `json:"selection_proof_mode"`
	SlashingProtection string              `json:"slashing_protection"`
	Network            CapabilitiesNetwork `json:"network"`
}

// CapabilitiesNetwork is the network binding reported by the remote signer.
// GenesisValidatorsRoot is nil while the signer's trust-on-first-use pin is
// pending, and GenesisTime is nil when the signer does not know it.
type CapabilitiesNetwork struct {
	ConfigName            string         `json:"config_name"`
	GenesisForkVersion    hexutil.Bytes  `json:"genesis_fork_version"`
	GenesisValidatorsRoot *hexutil.Bytes `json:"genesis_validators_root"`
	GenesisTime           *string        `json:"genesis_time"`
	SecondsPerSlot        string         `json:"seconds_per_slot"`
	SlotsPerEpoch         string         `json:"slots_per_epoch"`
}

// HttpSignerClient defines the interface for interacting with a remote signer.
type HttpSignerClient interface {
	Sign(ctx context.Context, pubKey string, request SignRequestJson) (ml_dsa_87.Signature, error)
	RandaoReveal(ctx context.Context, pubKey string, request RandaoRevealRequestJson) ([field_params.RandaoRevealLength]byte, error)
	GetPublicKeys(ctx context.Context, url string) ([][field_params.MLDSA87PubkeyLength]byte, error)
	Capabilities(ctx context.Context) (*Capabilities, error)
	GetServerStatus(ctx context.Context) (string, error)
}

// Config configures the HTTP client of the remote signer.
type Config struct {
	// BaseEndpoint is the signer's base URL, http(s)://host:port.
	BaseEndpoint string
	// AuthTokenFile holds the sign-scope bearer token. Optional.
	AuthTokenFile string
	// TLSCAFile is a PEM bundle of CAs that replace the system roots. Optional.
	TLSCAFile string
	// TLSClientCertFile and TLSClientKeyFile hold a PEM client certificate and
	// key for mutual TLS. Both or neither must be set.
	TLSClientCertFile string
	TLSClientKeyFile  string
	// Timeout bounds every request except the public keys list. Zero means
	// DefaultTimeout.
	Timeout time.Duration
}

// ApiClient is the HTTP client of a remote signer implementing the
// Web3Signer-style consensus signing API.
type ApiClient struct {
	baseURL    *url.URL
	restClient *http.Client
	authToken  string
	timeout    time.Duration
}

var _ HttpSignerClient = (*ApiClient)(nil)

// NewApiClient builds a client from cfg.
func NewApiClient(cfg Config) (*ApiClient, error) {
	u, err := url.ParseRequestURI(cfg.BaseEndpoint)
	if err != nil {
		return nil, errors.New("invalid remote signer url: it must be in the format http(s)://host:port")
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("invalid remote signer url: it must be in the format http(s)://host:port")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")

	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil && u.Scheme != "https" {
		return nil, errors.New("remote signer TLS files are set, but the url does not use https")
	}

	var token string
	if cfg.AuthTokenFile != "" {
		token, err = readAuthToken(cfg.AuthTokenFile)
		if err != nil {
			return nil, err
		}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           maxIdleConnsPerHost,
		MaxIdleConnsPerHost:    maxIdleConnsPerHost,
		IdleConnTimeout:        idleConnTimeout,
		TLSHandshakeTimeout:    tlsHandshakeTimeout,
		ResponseHeaderTimeout:  timeout,
		MaxResponseHeaderBytes: maxResponseHeaderBytes,
		ExpectContinueTimeout:  time.Second,
		TLSClientConfig:        tlsConfig,
	}
	return &ApiClient{
		baseURL:    u,
		restClient: &http.Client{Transport: transport},
		authToken:  token,
		timeout:    timeout,
	}, nil
}

func buildTLSConfig(cfg Config) (*tls.Config, error) {
	if cfg.TLSCAFile == "" && cfg.TLSClientCertFile == "" && cfg.TLSClientKeyFile == "" {
		return nil, nil
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS13}
	if cfg.TLSCAFile != "" {
		pem, err := os.ReadFile(cfg.TLSCAFile) // #nosec G304 -- operator-supplied path
		if err != nil {
			return nil, errors.Wrap(err, "could not read remote signer TLS CA file")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("remote signer TLS CA file contains no PEM certificate")
		}
		tlsConfig.RootCAs = pool
	}
	if (cfg.TLSClientCertFile == "") != (cfg.TLSClientKeyFile == "") {
		return nil, errors.New("remote signer TLS client certificate and key must be set together")
	}
	if cfg.TLSClientCertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSClientCertFile, cfg.TLSClientKeyFile)
		if err != nil {
			return nil, errors.Wrap(err, "could not load remote signer TLS client certificate")
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	return tlsConfig, nil
}

func readAuthToken(path string) (string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- operator-supplied path
	if err != nil {
		return "", errors.Wrap(err, "could not read remote signer auth token file")
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", errors.New("remote signer auth token file is empty")
	}
	if strings.ContainsAny(token, " \t\r\n") {
		return "", errors.New("remote signer auth token file must hold a single token")
	}
	return token, nil
}

// Sign requests a signature from the remote signer.
func (c *ApiClient) Sign(ctx context.Context, pubKey string, request SignRequestJson) (ml_dsa_87.Signature, error) {
	resp, err := c.doRequest(ctx, requestSpec{
		route:     "sign",
		method:    http.MethodPost,
		url:       c.endpoint(signPath + pubKey),
		body:      request,
		limit:     maxSignatureResponseBytes,
		sendToken: true,
	})
	if err != nil {
		return nil, err
	}
	var sigResp SignatureResponse
	if err := decodeJSON(resp, &sigResp); err != nil {
		return nil, err
	}
	if len(sigResp.Signature) != field_params.MLDSA87SignatureLength {
		return nil, fmt.Errorf("remote signer returned a %d-byte signature, want %d bytes", len(sigResp.Signature), field_params.MLDSA87SignatureLength)
	}
	return ml_dsa_87.SignatureFromBytes(sigResp.Signature)
}

// RandaoReveal requests the RANDAO hash-onion pre-image of a commitment.
func (c *ApiClient) RandaoReveal(
	ctx context.Context, pubKey string, request RandaoRevealRequestJson,
) ([field_params.RandaoRevealLength]byte, error) {
	var reveal [field_params.RandaoRevealLength]byte
	resp, err := c.doRequest(ctx, requestSpec{
		route:     "randao_reveal",
		method:    http.MethodPost,
		url:       c.endpoint(randaoRevealPath + pubKey),
		body:      request,
		limit:     maxSmallResponseBytes,
		sendToken: true,
	})
	if err != nil {
		return reveal, err
	}
	var revealResp RandaoRevealResponse
	if err := decodeJSON(resp, &revealResp); err != nil {
		return reveal, err
	}
	if len(revealResp.RandaoReveal) != field_params.RandaoRevealLength {
		return reveal, fmt.Errorf("remote signer returned a %d-byte randao reveal, want %d bytes", len(revealResp.RandaoReveal), field_params.RandaoRevealLength)
	}
	copy(reveal[:], revealResp.RandaoReveal)
	return reveal, nil
}

// GetPublicKeys fetches the list of public keys from the given URL, normally
// the signer's /api/v1/consensus/publicKeys. The bearer token is only sent
// when the URL has the same origin as the signer's base URL.
func (c *ApiClient) GetPublicKeys(ctx context.Context, rawURL string) ([][field_params.MLDSA87PubkeyLength]byte, error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("invalid remote signer public keys url: it must be in the format http(s)://host:port/path")
	}
	resp, err := c.doRequest(ctx, requestSpec{
		route:     "public_keys",
		method:    http.MethodGet,
		url:       u.String(),
		limit:     maxPublicKeysResponseBytes,
		timeout:   max(c.timeout, publicKeysTimeout),
		sendToken: sameOrigin(u, c.baseURL),
	})
	if err != nil {
		return nil, err
	}
	var publicKeys []string
	if err := decodeJSON(resp, &publicKeys); err != nil {
		return nil, err
	}
	decodedKeys := make([][field_params.MLDSA87PubkeyLength]byte, len(publicKeys))
	var badPositions []string
	for i, value := range publicKeys {
		decoded, err := hexutil.Decode(value)
		if err != nil || len(decoded) != field_params.MLDSA87PubkeyLength {
			badPositions = append(badPositions, strconv.Itoa(i))
			continue
		}
		copy(decodedKeys[i][:], decoded)
	}
	if len(badPositions) != 0 {
		return nil, fmt.Errorf("remote signer returned malformed public keys at positions: %s", strings.Join(badPositions, ", "))
	}
	return decodedKeys, nil
}

// Capabilities fetches the remote signer's capabilities and network binding.
func (c *ApiClient) Capabilities(ctx context.Context) (*Capabilities, error) {
	resp, err := c.doRequest(ctx, requestSpec{
		route:     "capabilities",
		method:    http.MethodGet,
		url:       c.endpoint(capabilitiesPath),
		limit:     maxSmallResponseBytes,
		sendToken: true,
	})
	if err != nil {
		return nil, err
	}
	var caps Capabilities
	if err := decodeJSON(resp, &caps); err != nil {
		return nil, err
	}
	return &caps, nil
}

// GetServerStatus calls the signer's /upcheck, which answers a plain-text
// "OK" once the signer has loaded its keys.
func (c *ApiClient) GetServerStatus(ctx context.Context) (string, error) {
	resp, err := c.doRequest(ctx, requestSpec{
		route:  "upcheck",
		method: http.MethodGet,
		url:    c.endpoint(upcheckPath),
		limit:  maxSmallResponseBytes,
		accept: "text/plain",
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(resp.body)), nil
}

func (c *ApiClient) endpoint(path string) string {
	return c.baseURL.String() + path
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

type requestSpec struct {
	route   string // metric, log and trace name; never the full URL
	method  string
	url     string
	body    []byte
	limit   int64
	timeout time.Duration
	accept  string
	// sendToken attaches the bearer token: on every signer route except the
	// unauthenticated /upcheck, and on a public keys URL of the same origin.
	sendToken bool
}

type response struct {
	header http.Header
	body   []byte
}

// doRequest sends one request and reads the whole response body, at most
// spec.limit bytes. A non-2xx status is returned as a *SignerError.
func (c *ApiClient) doRequest(ctx context.Context, spec requestSpec) (*response, error) {
	ctx, span := trace.StartSpan(ctx, "remote_web3signer.Client.doRequest")
	defer span.End()
	span.AddAttributes(
		trace.StringAttribute("httpMethod", spec.method),
		trace.StringAttribute("route", spec.route),
	)

	timeout := spec.timeout
	if timeout <= 0 {
		timeout = c.timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var body io.Reader
	if spec.body != nil {
		body = bytes.NewReader(spec.body)
	}
	req, err := http.NewRequestWithContext(ctx, spec.method, spec.url, body)
	if err != nil {
		return nil, errors.Wrap(err, "could not create remote signer request")
	}
	accept := spec.accept
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	if spec.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.authToken != "" && spec.sendToken {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	start := time.Now()
	resp, err := c.restClient.Do(req)
	if err != nil {
		requestDurationSeconds.WithLabelValues(spec.method, "error").Observe(time.Since(start).Seconds())
		// The url.Error wrapping the cause names the full URL, which carries
		// the public key; keep only the cause.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		err = errors.Wrapf(err, "remote signer %s request failed", spec.route)
		tracing.AnnotateError(span, err)
		return nil, err
	}
	defer closeBody(resp.Body)
	requestDurationSeconds.WithLabelValues(spec.method, strconv.Itoa(resp.StatusCode)).Observe(time.Since(start).Seconds())

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxSmallResponseBytes))
		signerErr := newSignerError(resp.StatusCode, resp.Header, raw)
		signerErr.log(spec.route)
		tracing.AnnotateError(span, signerErr)
		return nil, signerErr
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, spec.limit+1))
	if err != nil {
		err = errors.Wrapf(err, "could not read remote signer %s response", spec.route)
		tracing.AnnotateError(span, err)
		return nil, err
	}
	if int64(len(raw)) > spec.limit {
		err = fmt.Errorf("remote signer %s response exceeds %d bytes", spec.route, spec.limit)
		tracing.AnnotateError(span, err)
		return nil, err
	}
	return &response{header: resp.Header, body: raw}, nil
}

// decodeJSON decodes a JSON response strictly: the content type must be
// application/json, unknown fields are rejected and nothing may follow the
// value.
func decodeJSON(resp *response, out any) error {
	mediaType, _, err := mime.ParseMediaType(resp.header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("remote signer answered with content type %q, want application/json", resp.header.Get("Content-Type"))
	}
	dec := json.NewDecoder(bytes.NewReader(resp.body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return errors.Wrap(err, "could not decode remote signer response")
	}
	if dec.More() {
		return errors.New("could not decode remote signer response: trailing data after the JSON value")
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("could not decode remote signer response: trailing data after the JSON value")
	}
	return nil
}

func closeBody(body io.Closer) {
	if err := body.Close(); err != nil {
		log.WithError(err).Debug("Could not close remote signer response body")
	}
}
