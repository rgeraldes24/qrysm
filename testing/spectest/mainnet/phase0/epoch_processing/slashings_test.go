package epoch_processing

import (
	"testing"

	"github.com/theQRL/qrysm/v4/testing/spectest/shared/phase0/epoch_processing"
)

func TestMainnet_Phase0_EpochProcessing_Slashings(t *testing.T) {
	epoch_processing.RunSlashingsTests(t, "mainnet")
}