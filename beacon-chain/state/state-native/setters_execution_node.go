package state_native

import (
	"github.com/theQRL/qrysm/beacon-chain/state/state-native/types"
	"github.com/theQRL/qrysm/beacon-chain/state/stateutil"
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// SetExecutionNodeData for the beacon state.
func (b *BeaconState) SetExecutionNodeData(val *zondpb.ExecutionNodeData) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.executionNodeData = val
	b.markFieldAsDirty(types.ExecutionNodeData)
	return nil
}

// SetExecutionNodeDataVotes for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetExecutionNodeDataVotes(val []*zondpb.ExecutionNodeData) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[types.ExecutionNodeDataVotes].MinusRef()
	b.sharedFieldReferences[types.ExecutionNodeDataVotes] = stateutil.NewRef(1)

	b.executionNodeDataVotes = val
	b.markFieldAsDirty(types.ExecutionNodeDataVotes)
	b.rebuildTrie[types.ExecutionNodeDataVotes] = true
	return nil
}

// SetEth1DepositIndex for the beacon state.
func (b *BeaconState) SetEth1DepositIndex(val uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.eth1DepositIndex = val
	b.markFieldAsDirty(types.Eth1DepositIndex)
	return nil
}

// AppendExecutionNodeDataVotes for the beacon state. Appends the new value
// to the end of list.
func (b *BeaconState) AppendExecutionNodeDataVotes(val *zondpb.ExecutionNodeData) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	votes := b.executionNodeDataVotes
	if b.sharedFieldReferences[types.ExecutionNodeDataVotes].Refs() > 1 {
		// Copy elements in underlying array by reference.
		votes = make([]*zondpb.ExecutionNodeData, 0, len(b.executionNodeDataVotes)+1)
		votes = append(votes, b.executionNodeDataVotes...)
		b.sharedFieldReferences[types.ExecutionNodeDataVotes].MinusRef()
		b.sharedFieldReferences[types.ExecutionNodeDataVotes] = stateutil.NewRef(1)
	}

	b.executionNodeDataVotes = append(votes, val)
	b.markFieldAsDirty(types.ExecutionNodeDataVotes)
	b.addDirtyIndices(types.ExecutionNodeDataVotes, []uint64{uint64(len(b.executionNodeDataVotes) - 1)})
	return nil
}
