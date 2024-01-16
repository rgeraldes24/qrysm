package deposit_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	zond "github.com/theQRL/go-zond"
	"github.com/theQRL/go-zond/common"
	depositcontract "github.com/theQRL/qrysm/v4/contracts/deposit"
	"github.com/theQRL/qrysm/v4/contracts/deposit/mock"
	"github.com/theQRL/qrysm/v4/runtime/interop"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func TestSetupRegistrationContract_OK(t *testing.T) {
	_, err := mock.Setup()
	assert.NoError(t, err, "Can not deploy validator registration contract")
}

// negative test case, deposit with less than 1 ETH which is less than the top off amount.
func TestRegister_Below1ETH(t *testing.T) {
	testAccount, err := mock.Setup()
	require.NoError(t, err)

	// Generate deposit data
	privKeys, pubKeys, err := interop.DeterministicallyGenerateKeys(0 /*startIndex*/, 1)
	require.NoError(t, err)
	depositDataItems, depositDataRoots, err := interop.DepositDataFromKeys(privKeys, pubKeys)
	require.NoError(t, err)

	var depositDataRoot [32]byte
	copy(depositDataRoot[:], depositDataRoots[0])
	testAccount.TxOpts.Value = mock.LessThan1Eth()
	_, err = testAccount.Contract.Deposit(testAccount.TxOpts, pubKeys[0].Marshal(), depositDataItems[0].WithdrawalCredentials, depositDataItems[0].Signature, depositDataRoot)
	assert.ErrorContains(t, "execution reverted", err, "Validator registration should have failed with insufficient deposit")
}

// normal test case, test depositing 32 ETH and verify HashChainValue event is correctly emitted.
func TestValidatorRegister_OK(t *testing.T) {
	testAccount, err := mock.Setup()
	require.NoError(t, err)
	testAccount.TxOpts.Value = mock.Amount40000Eth()

	// Generate deposit data
	privKeys, pubKeys, err := interop.DeterministicallyGenerateKeys(0 /*startIndex*/, 1)
	require.NoError(t, err)
	depositDataItems, depositDataRoots, err := interop.DepositDataFromKeys(privKeys, pubKeys)
	require.NoError(t, err)

	fmt.Println(pubKeys[0].Marshal())
	fmt.Println(depositDataItems[0].WithdrawalCredentials)
	fmt.Println(depositDataItems[0].Signature)
	fmt.Println("OVER")

	var depositDataRoot [32]byte
	copy(depositDataRoot[:], depositDataRoots[0])
	_, err = testAccount.Contract.Deposit(testAccount.TxOpts, pubKeys[0].Marshal(), depositDataItems[0].WithdrawalCredentials, depositDataItems[0].Signature, depositDataRoot)
	testAccount.Backend.Commit()
	require.NoError(t, err, "Validator registration failed")
	_, err = testAccount.Contract.Deposit(testAccount.TxOpts, pubKeys[0].Marshal(), depositDataItems[0].WithdrawalCredentials, depositDataItems[0].Signature, depositDataRoot)
	testAccount.Backend.Commit()
	assert.NoError(t, err, "Validator registration failed")
	_, err = testAccount.Contract.Deposit(testAccount.TxOpts, pubKeys[0].Marshal(), depositDataItems[0].WithdrawalCredentials, depositDataItems[0].Signature, depositDataRoot)
	testAccount.Backend.Commit()
	assert.NoError(t, err, "Validator registration failed")

	query := zond.FilterQuery{
		Addresses: []common.Address{
			testAccount.ContractAddr,
		},
	}

	logs, err := testAccount.Backend.FilterLogs(context.Background(), query)
	assert.NoError(t, err, "Unable to get logs of deposit contract")

	merkleTreeIndex := make([]uint64, 5)

	for i, log := range logs {
		_, _, _, _, idx, err := depositcontract.UnpackDepositLogData(log.Data)
		require.NoError(t, err, "Unable to unpack log data")
		merkleTreeIndex[i] = binary.LittleEndian.Uint64(idx)
	}

	assert.Equal(t, uint64(0), merkleTreeIndex[0], "Deposit event total deposit count miss matched")
	assert.Equal(t, uint64(1), merkleTreeIndex[1], "Deposit event total deposit count miss matched")
	assert.Equal(t, uint64(2), merkleTreeIndex[2], "Deposit event total deposit count miss matched")
}
