package epoch_processing

import (
	"path"
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/core/epoch"
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/spectest/utils"
)

// RunExecutionNodeDataResetTests executes "epoch_processing/eth1_data_reset" tests.
func RunExecutionNodeDataResetTests(t *testing.T, config string) {
	require.NoError(t, utils.SetConfig(t, config))

	testFolders, testsFolderPath := utils.TestFolders(t, config, "capella", "epoch_processing/eth1_data_reset/pyspec_tests")
	if len(testFolders) == 0 {
		t.Fatalf("No test folders found for %s/%s/%s", config, "capella", "epoch_processing/eth1_data_reset/pyspec_tests")
	}
	for _, folder := range testFolders {
		t.Run(folder.Name(), func(t *testing.T) {
			folderPath := path.Join(testsFolderPath, folder.Name())
			RunEpochOperationTest(t, folderPath, processExecutionNodeDataResetWrapper)
		})
	}
}

func processExecutionNodeDataResetWrapper(t *testing.T, st state.BeaconState) (state.BeaconState, error) {
	st, err := epoch.ProcessExecutionNodeDataReset(st)
	require.NoError(t, err, "Could not process final updates")
	return st, nil
}
