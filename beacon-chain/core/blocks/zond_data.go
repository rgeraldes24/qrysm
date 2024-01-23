package blocks

import (
	"bytes"
	"context"
	"errors"

	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// ProcessZondDataInBlock is an operation performed on each
// beacon block to ensure the Zond data votes are processed
// into the beacon state.
//
// Official spec definition:
//
//	def process_eth1_data(state: BeaconState, body: BeaconBlockBody) -> None:
//	 state.eth1_data_votes.append(body.eth1_data)
//	 if state.eth1_data_votes.count(body.eth1_data) * 2 > EPOCHS_PER_ETH1_VOTING_PERIOD * SLOTS_PER_EPOCH:
//	     state.eth1_data = body.eth1_data
func ProcessZondDataInBlock(_ context.Context, beaconState state.BeaconState, zondData *zondpb.ZondData) (state.BeaconState, error) {
	if beaconState == nil || beaconState.IsNil() {
		return nil, errors.New("nil state")
	}
	if err := beaconState.AppendZondDataVotes(zondData); err != nil {
		return nil, err
	}
	hasSupport, err := ZondDataHasEnoughSupport(beaconState, zondData)
	if err != nil {
		return nil, err
	}
	if hasSupport {
		if err := beaconState.SetZondData(zondData); err != nil {
			return nil, err
		}
	}
	return beaconState, nil
}

// AreZondDataEqual checks equality between two eth1 data objects.
func AreZondDataEqual(a, b *zondpb.ZondData) bool {
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

// ZondDataHasEnoughSupport returns true when the given eth1data has more than 50% votes in the
// eth1 voting period. A vote is cast by including eth1data in a block and part of state processing
// appends eth1data to the state in the Eth1DataVotes list. Iterating through this list checks the
// votes to see if they match the eth1data.
func ZondDataHasEnoughSupport(beaconState state.ReadOnlyBeaconState, data *zondpb.ZondData) (bool, error) {
	voteCount := uint64(0)
	data = zondpb.CopyZondData(data)

	for _, vote := range beaconState.ZondDataVotes() {
		if AreZondDataEqual(vote, data) {
			voteCount++
		}
	}

	// If 50+% majority converged on the same eth1data, then it has enough support to update the
	// state.
	support := params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerZondVotingPeriod))
	return voteCount*2 > uint64(support), nil
}
