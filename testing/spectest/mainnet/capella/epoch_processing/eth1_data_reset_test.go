package epoch_processing

import (
	"testing"

	"github.com/theQRL/qrysm/v4/testing/spectest/shared/capella/epoch_processing"
)

func TestMainnet_Capella_EpochProcessing_Zond1DataReset(t *testing.T) {
	epoch_processing.RunZond1DataResetTests(t, "mainnet")
}
