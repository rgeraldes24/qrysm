package types

import (
	"github.com/theQRL/qrysm/v4/config/params"
)

func StartAt(v int, c *params.BeaconChainConfig) *params.BeaconChainConfig {
	c = c.Copy()
	//c.TerminalTotalDifficulty = "0"

	return c
}
