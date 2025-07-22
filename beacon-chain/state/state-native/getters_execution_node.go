package state_native

import (
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// ExecutionNodeData corresponding to the proof-of-work chain information stored in the beacon state.
func (b *BeaconState) ExecutionNodeData() *zondpb.ExecutionNodeData {
	if b.executionNodeData == nil {
		return nil
	}

	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.executionNodeDataVal()
}

// executionNodeDataVal corresponding to the proof-of-work chain information stored in the beacon state.
// This assumes that a lock is already held on BeaconState.
func (b *BeaconState) executionNodeDataVal() *zondpb.ExecutionNodeData {
	if b.executionNodeData == nil {
		return nil
	}

	return zondpb.CopyExecutionNodeData(b.executionNodeData)
}

// ExecutionNodeDataVotes corresponds to votes from Ethereum on the canonical proof-of-work chain
// data retrieved from eth1.
func (b *BeaconState) ExecutionNodeDataVotes() []*zondpb.ExecutionNodeData {
	if b.executionNodeDataVotes == nil {
		return nil
	}

	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.executionNodeDataVotesVal()
}

// executionNodeDataVotesVal corresponds to votes from Ethereum on the canonical proof-of-work chain
// data retrieved from eth1.
// This assumes that a lock is already held on BeaconState.
func (b *BeaconState) executionNodeDataVotesVal() []*zondpb.ExecutionNodeData {
	if b.executionNodeDataVotes == nil {
		return nil
	}

	res := make([]*zondpb.ExecutionNodeData, len(b.executionNodeDataVotes))
	for i := 0; i < len(res); i++ {
		res[i] = zondpb.CopyExecutionNodeData(b.executionNodeDataVotes[i])
	}
	return res
}

// Eth1DepositIndex corresponds to the index of the deposit made to the
// validator deposit contract at the time of this state's eth1 data.
func (b *BeaconState) Eth1DepositIndex() uint64 {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.eth1DepositIndex
}
