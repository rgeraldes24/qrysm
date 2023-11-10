package epoch_processing

import (
	"testing"

	"github.com/theQRL/qrysm/v4/testing/spectest/shared/altair/epoch_processing"
)

func TestMainnet_Altair_EpochProcessing_Zond1DataReset(t *testing.T) {
	epoch_processing.RunZond1DataResetTests(t, "mainnet")
}
