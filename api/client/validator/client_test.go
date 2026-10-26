package validator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/theQRL/qrysm/api/client"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/rpc/apimiddleware"
)

// keymanagerServer serves the local and remote key lists of a keymanager API.
// A nil remote list is answered with remoteStatus instead.
func keymanagerServer(t *testing.T, local []string, remote []string, remoteStatus int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var resp any
		switch r.URL.Path {
		case localKeysPath:
			list := &apimiddleware.ListKeystoresResponseJson{Keystores: []*apimiddleware.KeystoreJson{}}
			for _, key := range local {
				list.Keystores = append(list.Keystores, &apimiddleware.KeystoreJson{ValidatingPubkey: key})
			}
			resp = list
		case remoteKeysPath:
			if remote == nil {
				w.WriteHeader(remoteStatus)
				return
			}
			list := &apimiddleware.ListRemoteKeysResponseJson{Keystores: []*apimiddleware.RemoteKeysListJson{}}
			for _, key := range remote {
				list.Keystores = append(list.Keystores, &apimiddleware.RemoteKeysListJson{Pubkey: key, Url: "http://signer:9000", Readonly: true})
			}
			resp = list
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_GetValidatorPubKeys(t *testing.T) {
	tests := []struct {
		name         string
		local        []string
		remote       []string
		remoteStatus int
		want         []string
		wantErr      string
	}{
		{
			name:   "local and remote keys",
			local:  []string{"0x01", "0x02"},
			remote: []string{"0x02", "0x03"},
			want:   []string{"0x01", "0x02", "0x03"},
		},
		{
			name:   "remote keys only",
			remote: []string{"0x03"},
			want:   []string{"0x03"},
		},
		{
			name:         "validator client without the remote keys api",
			local:        []string{"0x01"},
			remoteStatus: http.StatusNotFound,
			want:         []string{"0x01"},
		},
		{
			name:         "remote keys api fails",
			local:        []string{"0x01"},
			remoteStatus: http.StatusInternalServerError,
			wantErr:      "did not receive 2xx response from API",
		},
		{
			name:    "no keys",
			remote:  []string{},
			wantErr: "there are no local keys or remote keys on the validator",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := keymanagerServer(t, tt.local, tt.remote, tt.remoteStatus)
			c, err := NewClient(srv.URL, client.WithAuthenticationToken("token"))
			require.NoError(t, err)
			got, err := c.GetValidatorPubKeys(context.Background())
			if tt.wantErr != "" {
				require.ErrorContains(t, tt.wantErr, err)
				return
			}
			require.NoError(t, err)
			slices.Sort(got)
			require.DeepEqual(t, tt.want, got)
		})
	}
}
