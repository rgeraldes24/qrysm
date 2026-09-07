package state_native_test

import (
	"runtime"
	"testing"

	"github.com/theQRL/qrysm/config/params"
)

// Runs first in this package (file name order). CI has computed the
// execution-data votes root with a list limit of 512 where mainnet's is
// 2048; the limit is EpochsPerExecutionVotingPeriod x SlotsPerEpoch from
// the active config, so print what the active config is at start-up.
func TestAAAConfigReport(t *testing.T) {
	reportConfig(t, "package start")
}

func reportConfig(t *testing.T, when string) {
	c := params.BeaconConfig()
	t.Logf("config report (%s): name=%s slotsPerEpoch=%d epochsPerExecutionVotingPeriod=%d votesLimit=%d syncCommitteeSize=%d go=%s %s/%s",
		when, c.ConfigName, c.SlotsPerEpoch, c.EpochsPerExecutionVotingPeriod, c.ExecutionDataVotesLength(), c.SyncCommitteeSize, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
