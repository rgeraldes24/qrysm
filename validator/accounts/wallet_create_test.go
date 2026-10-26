package accounts

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager"
)

func TestWalletCreate_RejectsKindsWithoutPersistentWallet(t *testing.T) {
	tests := []struct {
		name    string
		kind    keymanager.Kind
		wantErr string
	}{
		{
			name:    "web3signer",
			kind:    keymanager.Web3Signer,
			wantErr: "web3signer keymanager does not require persistent wallets",
		},
		{
			name:    "unknown kind",
			kind:    keymanager.Kind(99),
			wantErr: "keymanager kind not supported: 99",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			walletDir := filepath.Join(t.TempDir(), "wallet")
			acm, err := NewCLIManager(
				WithWalletDir(walletDir),
				WithKeymanagerType(tt.kind),
				WithWalletPassword("Passwordz0320$"),
			)
			require.NoError(t, err)
			w, err := acm.WalletCreate(context.Background())
			require.ErrorContains(t, tt.wantErr, err)
			assert.Equal(t, true, w == nil)
			_, statErr := os.Stat(walletDir)
			assert.Equal(t, true, os.IsNotExist(statErr), "wallet directory must not be created")
		})
	}
}
