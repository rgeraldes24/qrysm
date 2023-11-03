package testnet

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/theQRL/go-qrllib/common"
	cryptodilithium "github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/runtime/interop"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func Test_genesisStateFromJSONValidators(t *testing.T) {
	numKeys := 5
	jsonData, err := createGenesisDepositData(t, numKeys)
	require.NoError(t, err)
	jsonInput, err := json.Marshal(jsonData)
	require.NoError(t, err)
	_, dds, err := depositEntriesFromJSON(jsonInput)
	require.NoError(t, err)
	for i := range dds {
		assert.DeepEqual(t, fmt.Sprintf("%#x", dds[i].PublicKey), jsonData[i].PubKey)
	}
}

/*
var seed [common.SeedSize]uint8

_, err := rand.Read(seed[:])
if err != nil {
	return nil, fmt.Errorf("failed to generate random seed for Dilithium address: %v", err)
}
*/

func createGenesisDepositData(t *testing.T, numKeys int) ([]*depositDataJSON, error) {
	pubKeys := make([]cryptodilithium.PublicKey, numKeys)
	privKeys := make([]cryptodilithium.DilithiumKey, numKeys)
	for i := 0; i < numKeys; i++ {
		var seed [common.SeedSize]uint8
		_, err := rand.Read(seed[:])
		require.NoError(t, err)

		d, err := cryptodilithium.SecretKeyFromBytes(seed[:])
		require.NoError(t, err)

		privKeys[i] = d
		pubKeys[i] = d.PublicKey()
	}
	dataList, _, err := interop.DepositDataFromKeys(privKeys, pubKeys)
	require.NoError(t, err)
	jsonData := make([]*depositDataJSON, numKeys)
	for i := 0; i < numKeys; i++ {
		dataRoot, err := dataList[i].HashTreeRoot()
		require.NoError(t, err)
		jsonData[i] = &depositDataJSON{
			PubKey:                fmt.Sprintf("%#x", dataList[i].PublicKey),
			Amount:                dataList[i].Amount,
			WithdrawalCredentials: fmt.Sprintf("%#x", dataList[i].WithdrawalCredentials),
			DepositDataRoot:       fmt.Sprintf("%#x", dataRoot),
			Signature:             fmt.Sprintf("%#x", dataList[i].Signature),
		}
	}
	return jsonData, nil
}
