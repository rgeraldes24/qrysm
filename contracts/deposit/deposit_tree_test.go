package deposit_test

import (
	"strconv"
	"testing"

	"github.com/theQRL/go-zond/accounts/abi/bind"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/container/trie"
	depositcontract "github.com/theQRL/qrysm/contracts/deposit/mock"
	"github.com/theQRL/qrysm/runtime/interop"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

func TestDepositTrieRoot_OK(t *testing.T) {
	testAcc, err := depositcontract.Setup()
	require.NoError(t, err)

	localTrie, err := trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
	require.NoError(t, err)

	depRoot, err := testAcc.Contract.GetDepositRoot(&bind.CallOpts{})
	require.NoError(t, err)

	localRoot, err := localTrie.HashTreeRoot()
	require.NoError(t, err)
	assert.Equal(t, depRoot, localRoot, "Local deposit trie root and contract deposit trie root are not equal")

	privKeys, pubKeys, err := interop.DeterministicallyGenerateKeys(0 /*startIndex*/, 101)
	require.NoError(t, err)
	depositDataItems, depositDataRoots, err := interop.DepositDataFromKeys(privKeys, pubKeys)
	require.NoError(t, err)

	testAcc.TxOpts.Value = depositcontract.Amount40000Zond()

	for i := 0; i < 100; i++ {
		data := depositDataItems[i]
		var dataRoot [32]byte
		copy(dataRoot[:], depositDataRoots[i])

		_, err := testAcc.Contract.Deposit(testAcc.TxOpts, data.PublicKey, data.WithdrawalCredentials, data.Signature, dataRoot)
		require.NoError(t, err, "Could not deposit to deposit contract")

		testAcc.Backend.Commit()
		item, err := data.HashTreeRoot()
		require.NoError(t, err)

		assert.NoError(t, localTrie.Insert(item[:], i))
		depRoot, err = testAcc.Contract.GetDepositRoot(&bind.CallOpts{})
		require.NoError(t, err)
		localRoot, err := localTrie.HashTreeRoot()
		require.NoError(t, err)
		assert.Equal(t, depRoot, localRoot, "Local deposit trie root and contract deposit trie root are not equal for index %d", i)
	}
}

func TestDepositTrieRoot_Fail(t *testing.T) {
	testAcc, err := depositcontract.Setup()
	require.NoError(t, err)

	localTrie, err := trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
	require.NoError(t, err)

	depRoot, err := testAcc.Contract.GetDepositRoot(&bind.CallOpts{})
	require.NoError(t, err)

	localRoot, err := localTrie.HashTreeRoot()
	require.NoError(t, err)
	assert.Equal(t, depRoot, localRoot, "Local deposit trie root and contract deposit trie root are not equal")

	privKeys, pubKeys, err := interop.DeterministicallyGenerateKeys(0 /*startIndex*/, 101)
	require.NoError(t, err)
	depositDataItems, depositDataRoots, err := interop.DepositDataFromKeys(privKeys, pubKeys)
	require.NoError(t, err)
	testAcc.TxOpts.Value = depositcontract.Amount40000Zond()

	for i := 0; i < 100; i++ {
		data := depositDataItems[i]
		var dataRoot [32]byte
		copy(dataRoot[:], depositDataRoots[i])

		_, err := testAcc.Contract.Deposit(testAcc.TxOpts, data.PublicKey, data.WithdrawalCredentials, data.Signature, dataRoot)
		require.NoError(t, err, "Could not deposit to deposit contract")

		// Change an element in the data when storing locally
		copy(data.PublicKey, strconv.Itoa(i+10))

		testAcc.Backend.Commit()
		item, err := data.HashTreeRoot()
		require.NoError(t, err)

		assert.NoError(t, localTrie.Insert(item[:], i))

		depRoot, err = testAcc.Contract.GetDepositRoot(&bind.CallOpts{})
		require.NoError(t, err)

		localRoot, err := localTrie.HashTreeRoot()
		require.NoError(t, err)
		assert.NotEqual(t, depRoot, localRoot, "Local deposit trie root and contract deposit trie root are equal for index %d", i)
	}
}
