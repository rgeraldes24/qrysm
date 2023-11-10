package epoch_processing

import (
	"testing"

	"github.com/theQRL/qrysm/v4/testing/spectest/shared/bellatrix/epoch_processing"
)

func TestMinimal_Bellatrix_EpochProcessing_Zond1DataReset(t *testing.T) {
	epoch_processing.RunZond1DataResetTests(t, "minimal")
}
