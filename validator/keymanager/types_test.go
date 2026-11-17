package keymanager_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/local"
	remoteweb3signer "github.com/theQRL/qrysm/validator/keymanager/remote-web3signer"
)

var (
	_ = keymanager.IKeymanager(&local.Keymanager{})
	_ = keymanager.IKeymanager(&remoteweb3signer.Keymanager{})

	// More granular assertions.
	_ = keymanager.KeysFetcher(&local.Keymanager{})
	_ = keymanager.Importer(&local.Keymanager{})
	_ = keymanager.Deleter(&local.Keymanager{})

	// _ = keymanager.PublicKeyAdder(&remoteweb3signer.Keymanager{})
	// _ = keymanager.PublicKeyDeleter(&remoteweb3signer.Keymanager{})
)

func TestKeystoreContainsPath(t *testing.T) {
	keystore := keymanager.Keystore{}
	encoded, err := json.Marshal(keystore)

	require.NoError(t, err, "Unexpected error marshalling keystore")
	assert.Equal(t, true, strings.Contains(string(encoded), "path"))
}

func TestKind_StringAndParse(t *testing.T) {
	tests := []struct {
		input string
		want  keymanager.Kind
		str   string
	}{
		{input: "direct", want: keymanager.Local, str: "direct"},
		{input: "imported", want: keymanager.Local, str: "direct"},
		{input: "web3signer", want: keymanager.Web3Signer, str: "web3signer"},
		{input: "Web3Signer", want: keymanager.Web3Signer, str: "web3signer"},
	}
	for _, tt := range tests {
		got, err := keymanager.ParseKind(tt.input)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
		assert.Equal(t, tt.str, got.String())
	}
	_, err := keymanager.ParseKind("derived")
	require.ErrorContains(t, "is not an allowed keymanager", err)
}
