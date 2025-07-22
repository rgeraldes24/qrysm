package blocks

import (
	"bytes"
	"context"
	"errors"

	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/config/params"
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// ProcessExecutionNodeDataInBlock is an operation performed on each
// beacon block to ensure the ETH1 data votes are processed
// into the beacon state.
//
// Official spec definition:
//
//	def process_eth1_data(state: BeaconState, body: BeaconBlockBody) -> None:
//	 state.eth1_data_votes.append(body.eth1_data)
//	 if state.eth1_data_votes.count(body.eth1_data) * 2 > EPOCHS_PER_ETH1_VOTING_PERIOD * SLOTS_PER_EPOCH:
//	     state.eth1_data = body.eth1_data
func ProcessExecutionNodeDataInBlock(_ context.Context, beaconState state.BeaconState, executionNodeData *zondpb.ExecutionNodeData) (state.BeaconState, error) {
	if beaconState == nil || beaconState.IsNil() {
		return nil, errors.New("nil state")
	}
	if err := beaconState.AppendExecutionNodeDataVotes(executionNodeData); err != nil {
		return nil, err
	}
	hasSupport, err := ExecutionNodeDataHasEnoughSupport(beaconState, executionNodeData)
	if err != nil {
		return nil, err
	}
	if hasSupport {
		if err := beaconState.SetExecutionNodeData(executionNodeData); err != nil {
			return nil, err
		}
	}
	return beaconState, nil
}

// AreExecutionNodeDataEqual checks equality between two eth1 data objects.
func AreExecutionNodeDataEqual(a, b *zondpb.ExecutionNodeData) bool {
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

// ExecutionNodeDataHasEnoughSupport returns true when the given executionNodeData has more than 50% votes in the
// eth1 voting period. A vote is cast by including executionNodeData in a block and part of state processing
// appends executionNodeData to the state in the ExecutionNodeDataVotes list. Iterating through this list checks the
// votes to see if they match the executionNodeData.
func ExecutionNodeDataHasEnoughSupport(beaconState state.ReadOnlyBeaconState, data *zondpb.ExecutionNodeData) (bool, error) {
	voteCount := uint64(0)
	data = zondpb.CopyExecutionNodeData(data)

	for _, vote := range beaconState.ExecutionNodeDataVotes() {
		if AreExecutionNodeDataEqual(vote, data) {
			voteCount++
		}
	}

	// If 50+% majority converged on the same executionNodeData, then it has enough support to update the
	// state.
	support := params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerEth1VotingPeriod))
	return voteCount*2 > uint64(support), nil
}
