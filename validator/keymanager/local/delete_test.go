package local

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	logTest "github.com/sirupsen/logrus/hooks/test"
	keystorev1 "github.com/theQRL/go-zond-wallet-encryptor-keystore"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	zondpbservice "github.com/theQRL/qrysm/proto/zond/service"
	"github.com/theQRL/qrysm/testing/require"
	mock "github.com/theQRL/qrysm/validator/accounts/testing"
	"github.com/theQRL/qrysm/validator/keymanager"
)

func TestLocalKeymanager_DeleteKeystores(t *testing.T) {
	hook := logTest.NewGlobal()
	wallet := &mock.Wallet{
		Files:          make(map[string]map[string][]byte),
		WalletPassword: password,
	}
	dr := &Keymanager{
		wallet:        wallet,
		accountsStore: &accountStore{},
	}
	numAccounts := 5
	ctx := context.Background()
	keystores := make([]*keymanager.Keystore, numAccounts)
	passwords := make([]string, numAccounts)
	for i := 0; i < numAccounts; i++ {
		keystores[i] = createRandomKeystore(t, password)
		passwords[i] = password
	}
	_, err := dr.ImportKeystores(ctx, keystores, passwords)
	require.NoError(t, err)
	accounts, err := dr.FetchValidatingPublicKeys(ctx)
	require.NoError(t, err)
	require.Equal(t, numAccounts, len(accounts))

	t.Run("keys not found", func(t *testing.T) {
		notFoundPubKey := [field_params.DilithiumPubkeyLength]byte{1, 2, 3}
		notFoundPubKey2 := [field_params.DilithiumPubkeyLength]byte{4, 5, 6}
		statuses, err := dr.DeleteKeystores(ctx, [][]byte{notFoundPubKey[:], notFoundPubKey2[:]})
		require.NoError(t, err)
		require.Equal(t, 2, len(statuses))
		require.Equal(t, zondpbservice.DeletedKeystoreStatus_NOT_FOUND, statuses[0].Status)
		require.Equal(t, zondpbservice.DeletedKeystoreStatus_NOT_FOUND, statuses[1].Status)
	})
	t.Run("file write errors should not lead to updated local keystore or cache", func(t *testing.T) {
		wallet.HasWriteFileError = true
		accountToRemove := uint64(2)
		accountPubKey := accounts[accountToRemove]
		require.NotEqual(t, len(dr.accountsStore.PublicKeys), 0)
		copyStore := dr.accountsStore.Copy()
		statuses, err := dr.DeleteKeystores(ctx, [][]byte{accountPubKey[:]})
		require.ErrorContains(t, "could not write keystore file for accounts", err)
		require.Equal(t, len(statuses), 0)
		require.DeepEqual(t, dr.accountsStore, copyStore)
	})
	t.Run("deletes properly", func(t *testing.T) {
		accountToRemove := uint64(2)
		accountPubKey := accounts[accountToRemove]
		statuses, err := dr.DeleteKeystores(ctx, [][]byte{accountPubKey[:]})
		require.NoError(t, err)

		require.Equal(t, 1, len(statuses))
		require.Equal(t, zondpbservice.DeletedKeystoreStatus_DELETED, statuses[0].Status)

		// Ensure the keystore file was written to the wallet
		// and ensure we can decrypt it using the EIP-2335 standard.
		var encodedKeystore []byte
		for k, v := range wallet.Files[AccountsPath] {
			if strings.Contains(k, "keystore") {
				encodedKeystore = v
			}
		}
		require.NotNil(t, encodedKeystore, "could not find keystore file")
		keystoreFile := &keymanager.Keystore{}
		require.NoError(t, json.Unmarshal(encodedKeystore, keystoreFile))

		// We extract the accounts from the keystore.
		decryptor := keystorev1.New()
		encodedAccounts, err := decryptor.Decrypt(keystoreFile.Crypto, password)
		require.NoError(t, err, "Could not decrypt validator accounts")
		store := &accountStore{}
		require.NoError(t, json.Unmarshal(encodedAccounts, store))

		require.Equal(t, numAccounts-1, len(store.PublicKeys))
		require.Equal(t, numAccounts-1, len(store.Seeds))
		require.LogsContain(t, hook, fmt.Sprintf("%#x", bytesutil.Trunc(accountPubKey[:])))
		require.LogsContain(t, hook, "Successfully deleted validator key(s)")
	})

	t.Run("returns NOT_ACTIVE status for duplicate public key in request", func(t *testing.T) {
		accountToRemove := uint64(3)
		accountPubKey := accounts[accountToRemove]
		statuses, err := dr.DeleteKeystores(ctx, [][]byte{
			accountPubKey[:],
			accountPubKey[:], // Add in the same key a few more times.
			accountPubKey[:],
			accountPubKey[:],
		})
		require.NoError(t, err)

		require.Equal(t, 4, len(statuses))
		for i, st := range statuses {
			if i == 0 {
				require.Equal(t, zondpbservice.DeletedKeystoreStatus_DELETED, st.Status)
			} else {
				require.Equal(t, zondpbservice.DeletedKeystoreStatus_NOT_ACTIVE, st.Status)
			}
		}

		// Ensure the keystore file was written to the wallet
		// and ensure we can decrypt it using the EIP-2335 standard.
		var encodedKeystore []byte
		for k, v := range wallet.Files[AccountsPath] {
			if strings.Contains(k, "keystore") {
				encodedKeystore = v
			}
		}
		require.NotNil(t, encodedKeystore, "could not find keystore file")
		keystoreFile := &keymanager.Keystore{}
		require.NoError(t, json.Unmarshal(encodedKeystore, keystoreFile))

		// We extract the accounts from the keystore.
		decryptor := keystorev1.New()
		encodedAccounts, err := decryptor.Decrypt(keystoreFile.Crypto, password)
		require.NoError(t, err, "Could not decrypt validator accounts")
		store := &accountStore{}
		require.NoError(t, json.Unmarshal(encodedAccounts, store))

		require.Equal(t, numAccounts-2, len(store.PublicKeys))
		require.Equal(t, numAccounts-2, len(store.Seeds))
		require.LogsContain(t, hook, fmt.Sprintf("%#x", bytesutil.Trunc(accountPubKey[:])))
		require.LogsContain(t, hook, "Successfully deleted validator key(s)")
	})

}
