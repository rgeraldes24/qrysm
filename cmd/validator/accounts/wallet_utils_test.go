package accounts

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/accounts"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/local"
)

func TestWalletWithKeymanager(t *testing.T) {
	logHook := test.NewGlobal()
	walletDir, passwordsDir, passwordFilePath := setupWalletAndPasswordsDir(t)
	keysDir := filepath.Join(t.TempDir(), "keysDir")
	require.NoError(t, os.MkdirAll(keysDir, os.ModePerm))

	cliCtx := setupWalletCtx(t, &testWalletConfig{
		walletDir:           walletDir,
		passwordsDir:        passwordsDir,
		keysDir:             keysDir,
		keymanagerKind:      keymanager.Local,
		walletPasswordFile:  passwordFilePath,
		accountPasswordFile: passwordFilePath,
	})

	opts := []accounts.Option{
		accounts.WithWalletDir(walletDir),
		accounts.WithKeymanagerType(keymanager.Local),
		accounts.WithWalletPassword(password),
	}
	acc, err := accounts.NewCLIManager(opts...)
	require.NoError(t, err)
	w, err := acc.WalletCreate(cliCtx.Context)
	require.NoError(t, err)
	newKm, err := local.NewKeymanager(
		cliCtx.Context,
		&local.SetupConfig{
			Wallet:           w,
			ListenForChanges: false,
		},
	)
	require.NoError(t, err)

	// Make sure there are no accounts at the start.
	accNames, err := newKm.ValidatingAccountNames()
	require.NoError(t, err)
	require.Equal(t, len(accNames), 0)

	// Create 2 keys.
	createKeystore(t, keysDir)
	time.Sleep(time.Second)
	createKeystore(t, keysDir)
	require.NoError(t, accountsImport(cliCtx))

	w, k, err := walletWithKeymanager(cliCtx)
	require.NoError(t, err)
	keys, err := k.FetchValidatingPublicKeys(cliCtx.Context)
	require.NoError(t, err)
	require.Equal(t, len(keys), 2)
	require.Equal(t, w.KeymanagerKind(), keymanager.Local)

	assert.LogsContain(t, logHook, fmt.Sprintf("Imported accounts"))
	assert.LogsContain(t, logHook, hexutil.Encode(keys[0][:])[2:])
	assert.LogsContain(t, logHook, hexutil.Encode(keys[1][:])[2:])
}
