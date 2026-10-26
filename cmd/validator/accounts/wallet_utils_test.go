package accounts

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/accounts"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/local"
	"github.com/theQRL/qrysm/validator/node"
	"github.com/urfave/cli/v2"
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

func TestWalletWithKeymanager_web3signer(t *testing.T) {
	key, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	pubKey := hexutil.Encode(key.PublicKey().Marshal())
	app := cli.App{}
	set := flag.NewFlagSet("test", 0)
	set.String(flags.Web3SignerURLFlag.Name, "http://localhost:12345", "web3signer")
	c := &cli.StringSliceFlag{
		Name: "validators-external-signer-public-keys",
	}
	err = c.Apply(set)
	require.NoError(t, err)
	require.NoError(t, set.Set(flags.Web3SignerURLFlag.Name, "http://localhost:12345"))
	require.NoError(t, set.Set(flags.Web3SignerPublicValidatorKeysFlag.Name, pubKey))
	ctx := cli.NewContext(&app, set, nil)
	bytes, err := hexutil.Decode("0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2")
	require.NoError(t, err)
	config, err := node.Web3SignerConfig(ctx)
	require.NoError(t, err)
	config.GenesisValidatorsRoot = bytes
	w, k, err := walletWithWeb3SignerKeymanager(ctx, config)
	require.NoError(t, err)
	keys, err := k.FetchValidatingPublicKeys(ctx.Context)
	require.NoError(t, err)
	require.Equal(t, len(keys), 1)
	assert.Equal(t, pubKey, hexutil.Encode(keys[0][:]))
	require.Equal(t, w.KeymanagerKind(), keymanager.Web3Signer)
}
