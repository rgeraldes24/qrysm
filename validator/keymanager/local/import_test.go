package local

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/uuid"
	logTest "github.com/sirupsen/logrus/hooks/test"
	keystorev1 "github.com/theQRL/go-zond-wallet-encryptor-keystore"
	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/crypto/dilithium"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	zondpbservice "github.com/theQRL/qrysm/proto/zond/service"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	mock "github.com/theQRL/qrysm/validator/accounts/testing"
	"github.com/theQRL/qrysm/validator/keymanager"
)

const password = "secretPassw0rd$1999"

func createRandomKeystore(t testing.TB, password string) *keymanager.Keystore {
	encryptor := keystorev1.New()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	validatingKey, err := dilithium.RandKey()
	require.NoError(t, err)
	pubKey := validatingKey.PublicKey().Marshal()
	cryptoFields, err := encryptor.Encrypt(validatingKey.Marshal(), password)
	require.NoError(t, err)
	return &keymanager.Keystore{
		Crypto:      cryptoFields,
		Pubkey:      fmt.Sprintf("%x", pubKey),
		ID:          id.String(),
		Version:     encryptor.Version(),
		Description: encryptor.Name(),
	}
}

func TestLocalKeymanager_NoDuplicates(t *testing.T) {
	numKeys := 50
	pubKeys := make([][]byte, numKeys)
	seeds := make([][]byte, numKeys)
	for i := 0; i < numKeys; i++ {
		priv, err := dilithium.RandKey()
		require.NoError(t, err)
		seeds[i] = priv.Marshal()
		pubKeys[i] = priv.PublicKey().Marshal()
	}
	wallet := &mock.Wallet{
		WalletPassword: "Passwordz0202$",
	}
	dr := &Keymanager{
		wallet: wallet,
	}
	ctx := context.Background()
	_, err := dr.CreateAccountsKeystore(ctx, seeds, pubKeys)
	require.NoError(t, err)

	// We expect the 50 keys in the account store to match.
	require.NotNil(t, dr.accountsStore)
	require.Equal(t, len(dr.accountsStore.PublicKeys), len(dr.accountsStore.Seeds))
	require.Equal(t, len(dr.accountsStore.PublicKeys), numKeys)
	for i := 0; i < len(dr.accountsStore.Seeds); i++ {
		assert.DeepEqual(t, dr.accountsStore.Seeds[i], seeds[i])
		assert.DeepEqual(t, dr.accountsStore.PublicKeys[i], pubKeys[i])
	}

	// Re-run the create accounts keystore function with the same pubkeys.
	_, err = dr.CreateAccountsKeystore(ctx, seeds, pubKeys)
	require.NoError(t, err)

	// We expect nothing to change.
	require.NotNil(t, dr.accountsStore)
	require.Equal(t, len(dr.accountsStore.PublicKeys), len(dr.accountsStore.Seeds))
	require.Equal(t, len(dr.accountsStore.PublicKeys), numKeys)
	for i := 0; i < len(dr.accountsStore.Seeds); i++ {
		assert.DeepEqual(t, dr.accountsStore.Seeds[i], seeds[i])
		assert.DeepEqual(t, dr.accountsStore.PublicKeys[i], pubKeys[i])
	}

	// Now, we run the function again but with a new priv and pubkey and this
	// time, we do expect a change.
	privKey, err := dilithium.RandKey()
	require.NoError(t, err)
	seeds = append(seeds, privKey.Marshal())
	pubKeys = append(pubKeys, privKey.PublicKey().Marshal())

	_, err = dr.CreateAccountsKeystore(ctx, seeds, pubKeys)
	require.NoError(t, err)
	require.Equal(t, len(dr.accountsStore.PublicKeys), len(dr.accountsStore.Seeds))

	// We should have 1 more new key in the store.
	require.Equal(t, numKeys+1, len(dr.accountsStore.Seeds))
}

func TestLocalKeymanager_ImportKeystores(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	// Setup the keymanager.
	wallet := &mock.Wallet{
		Files:          make(map[string]map[string][]byte),
		WalletPassword: password,
	}
	dr := &Keymanager{
		wallet:        wallet,
		accountsStore: &accountStore{},
	}

	t.Run("same password used to decrypt all keystores", func(t *testing.T) {
		numKeystores := 5
		keystores := make([]*keymanager.Keystore, numKeystores)
		passwords := make([]string, numKeystores)
		for i := 0; i < numKeystores; i++ {
			keystores[i] = createRandomKeystore(t, password)
			passwords[i] = password
		}
		statuses, err := dr.ImportKeystores(
			ctx,
			keystores,
			passwords,
		)
		require.NoError(t, err)
		require.Equal(t, numKeystores, len(statuses))
		for _, status := range statuses {
			require.Equal(t, zondpbservice.ImportedKeystoreStatus_IMPORTED, status.Status)
		}
		require.LogsContain(t, hook, "Successfully imported validator key(s)")
	})
	t.Run("each imported keystore with a different password succeeds", func(t *testing.T) {
		numKeystores := 5
		keystores := make([]*keymanager.Keystore, numKeystores)
		passwords := make([]string, numKeystores)
		for i := 0; i < numKeystores; i++ {
			pass := password + strconv.Itoa(i)
			keystores[i] = createRandomKeystore(t, pass)
			passwords[i] = pass
		}
		statuses, err := dr.ImportKeystores(
			ctx,
			keystores,
			passwords,
		)
		require.NoError(t, err)
		require.Equal(t, numKeystores, len(statuses))
		for _, status := range statuses {
			require.Equal(t, zondpbservice.ImportedKeystoreStatus_IMPORTED, status.Status)
		}
	})
	t.Run("some succeed, some fail to decrypt, some duplicated", func(t *testing.T) {
		keystores := make([]*keymanager.Keystore, 0)
		passwords := make([]string, 0)

		// First keystore is normal.
		keystore1 := createRandomKeystore(t, password)
		keystores = append(keystores, keystore1)
		passwords = append(passwords, password)

		// Second keystore is a duplicate of the first.
		keystores = append(keystores, keystore1)
		passwords = append(passwords, password)

		// Third keystore has a wrong password.
		keystore3 := createRandomKeystore(t, password)
		keystores = append(keystores, keystore3)
		passwords = append(passwords, "foobar")

		statuses, err := dr.ImportKeystores(
			ctx,
			keystores,
			passwords,
		)
		require.NoError(t, err)
		require.Equal(t, len(keystores), len(statuses))
		require.Equal(
			t,
			zondpbservice.ImportedKeystoreStatus_IMPORTED,
			statuses[0].Status,
		)
		require.Equal(
			t,
			zondpbservice.ImportedKeystoreStatus_DUPLICATE,
			statuses[1].Status,
		)
		require.Equal(
			t,
			zondpbservice.ImportedKeystoreStatus_ERROR,
			statuses[2].Status,
		)
		require.StringContains(
			t,
			fmt.Sprintf("incorrect password for key %s", keystores[2].Pubkey[:12]),
			statuses[2].Message,
		)
		b, err := hexutil.Decode("0x" + keystore1.Pubkey)
		require.NoError(t, err)
		require.LogsContain(t, hook, fmt.Sprintf("%#x", bytesutil.Trunc(b)))
		require.LogsContain(t, hook, "Successfully imported validator key(s)")
	})
	t.Run("All fail or duplicated", func(t *testing.T) {
		// First keystore is normal.
		keystore1 := createRandomKeystore(t, password)
		// First Import successfully
		statuses, err := dr.ImportKeystores(
			ctx,
			[]*keymanager.Keystore{keystore1},
			[]string{password},
		)
		require.NoError(t, err)
		require.Equal(t, len(statuses), 1)

		keystores := make([]*keymanager.Keystore, 0)
		passwords := make([]string, 0)
		// Second keystore is a duplicate of the first.
		keystores = append(keystores, keystore1)
		passwords = append(passwords, password)

		// Third keystore has a wrong password.
		keystore3 := createRandomKeystore(t, password)
		keystores = append(keystores, keystore3)
		passwords = append(passwords, "foobar")

		statuses, err = dr.ImportKeystores(
			ctx,
			keystores,
			passwords,
		)
		require.NoError(t, err)
		require.Equal(t, len(keystores), len(statuses))
		require.Equal(
			t,
			zondpbservice.ImportedKeystoreStatus_DUPLICATE,
			statuses[0].Status,
		)
		require.Equal(
			t,
			zondpbservice.ImportedKeystoreStatus_ERROR,
			statuses[1].Status,
		)
		require.StringContains(
			t,
			fmt.Sprintf("incorrect password for key %s", keystores[1].Pubkey[:12]),
			statuses[1].Message,
		)
		require.LogsContain(t, hook, "no keys were imported")
	})
	t.Run("file write fails during import", func(t *testing.T) {
		wallet.HasWriteFileError = true
		copyStore := dr.accountsStore.Copy()
		keystore1 := createRandomKeystore(t, password)
		statuses, err := dr.ImportKeystores(
			ctx,
			[]*keymanager.Keystore{keystore1},
			[]string{password},
		)
		require.ErrorContains(t, "could not write keystore file for accounts", err)
		require.Equal(t, len(statuses), 0)
		// local copy did not update due to bad file write
		require.DeepEqual(t, dr.accountsStore, copyStore)
	})
}
