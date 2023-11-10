package epoch_processing

import (
	"path"
	"testing"

	"github.com/theQRL/qrysm/v4/beacon-chain/core/epoch"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/spectest/utils"
)

// RunZond1DataResetTests executes "epoch_processing/zond1_data_reset" tests.
func RunZond1DataResetTests(t *testing.T, config string) {
	require.NoError(t, utils.SetConfig(t, config))

	testFolders, testsFolderPath := utils.TestFolders(t, config, "phase0", "epoch_processing/zond1_data_reset/pyspec_tests")
	if len(testFolders) == 0 {
		t.Fatalf("No test folders found for %s/%s/%s", config, "phase0", "epoch_processing/zond1_data_reset/pyspec_tests")
	}
	for _, folder := range testFolders {
		t.Run(folder.Name(), func(t *testing.T) {
			folderPath := path.Join(testsFolderPath, folder.Name())
			RunEpochOperationTest(t, folderPath, processZond1DataResetWrapper)
		})
	}
}

func processZond1DataResetWrapper(t *testing.T, st state.BeaconState) (state.BeaconState, error) {
	st, err := epoch.ProcessZond1DataReset(st)
	require.NoError(t, err, "Could not process final updates")
	return st, nil
}
