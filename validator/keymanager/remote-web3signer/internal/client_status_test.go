package internal

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func mockClient(t *testing.T, body string) *ApiClient {
	t.Helper()
	base, err := url.Parse("http://signer.local")
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	return &ApiClient{
		BaseURL: base,
		RestClient: &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet {
					return nil, errors.New("unexpected method")
				}
				if r.URL.Path != "/upcheck" {
					return nil, errors.New("unexpected path")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}),
		},
	}
}

func TestGetServerStatus_JSONString(t *testing.T) {
	t.Parallel()

	cl := mockClient(t, `"OK"`)
	status, err := cl.GetServerStatus(t.Context())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status != "OK" {
		t.Fatalf("unexpected status: got %q want %q", status, "OK")
	}
}

func TestGetServerStatus_PlainText(t *testing.T) {
	t.Parallel()

	cl := mockClient(t, "OK")
	status, err := cl.GetServerStatus(t.Context())
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status != "OK" {
		t.Fatalf("unexpected status: got %q want %q", status, "OK")
	}
}
