package flags

import (
	"testing"

	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/testing/assert"
)

// The circuit-breaker flags are only applied when set, so their advertised
// defaults must be the config values that apply when they are not.
func TestBuilderCircuitBreakerFlagDefaults(t *testing.T) {
	cfg := params.MainnetConfig()
	assert.Equal(t, int(cfg.MaxBuilderConsecutiveMissedSlots), MaxBuilderConsecutiveMissedSlots.Value)
	assert.Equal(t, int(cfg.MaxBuilderEpochMissedSlots), MaxBuilderEpochMissedSlots.Value)
}
