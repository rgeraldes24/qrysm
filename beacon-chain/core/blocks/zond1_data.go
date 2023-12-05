package blocks

import (
	"bytes"
	"context"
	"errors"

	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// ProcessZond1DataInBlock is an operation performed on each
// beacon block to ensure the ZOND1 data votes are processed
// into the beacon state.
func ProcessZond1DataInBlock(_ context.Context, beaconState state.BeaconState, zond1Data *zondpb.Zond1Data) (state.BeaconState, error) {
	if beaconState == nil || beaconState.IsNil() {
		return nil, errors.New("nil state")
	}
	if err := beaconState.AppendZond1DataVotes(zond1Data); err != nil {
		return nil, err
	}
	hasSupport, err := Zond1DataHasEnoughSupport(beaconState, zond1Data)
	if err != nil {
		return nil, err
	}
	if hasSupport {
		if err := beaconState.SetZond1Data(zond1Data); err != nil {
			return nil, err
		}
	}
	return beaconState, nil
}

// AreZond1DataEqual checks equality between two zond1 data objects.
func AreZond1DataEqual(a, b *zondpb.Zond1Data) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.DepositCount == b.DepositCount &&
		bytes.Equal(a.BlockHash, b.BlockHash) &&
		bytes.Equal(a.DepositRoot, b.DepositRoot)
}

// Zond1DataHasEnoughSupport returns true when the given zond1data has more than 50% votes in the
// zond1 voting period. A vote is cast by including zond1data in a block and part of state processing
// appends zond1data to the state in the Zond1DataVotes list. Iterating through this list checks the
// votes to see if they match the zond1data.
func Zond1DataHasEnoughSupport(beaconState state.ReadOnlyBeaconState, data *zondpb.Zond1Data) (bool, error) {
	voteCount := uint64(0)
	data = zondpb.CopyZOND1Data(data)

	for _, vote := range beaconState.Zond1DataVotes() {
		if AreZond1DataEqual(vote, data) {
			voteCount++
		}
	}

	// If 50+% majority converged on the same zond1data, then it has enough support to update the
	// state.
	support := params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerZond1VotingPeriod))
	return voteCount*2 > uint64(support), nil
}
