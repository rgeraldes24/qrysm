package state_native

import (
	"github.com/theQRL/qrysm/v4/beacon-chain/state/state-native/types"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stateutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// SetZond1Data for the beacon state.
func (b *BeaconState) SetZond1Data(val *zondpb.Zond1Data) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.zond1Data = val
	b.markFieldAsDirty(types.Zond1Data)
	return nil
}

// SetZond1DataVotes for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetZond1DataVotes(val []*zondpb.Zond1Data) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[types.Zond1DataVotes].MinusRef()
	b.sharedFieldReferences[types.Zond1DataVotes] = stateutil.NewRef(1)

	b.zond1DataVotes = val
	b.markFieldAsDirty(types.Zond1DataVotes)
	b.rebuildTrie[types.Zond1DataVotes] = true
	return nil
}

// SetZond1DepositIndex for the beacon state.
func (b *BeaconState) SetZond1DepositIndex(val uint64) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.zond1DepositIndex = val
	b.markFieldAsDirty(types.Zond1DepositIndex)
	return nil
}

// AppendZond1DataVotes for the beacon state. Appends the new value
// to the end of list.
func (b *BeaconState) AppendZond1DataVotes(val *zondpb.Zond1Data) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	votes := b.zond1DataVotes
	if b.sharedFieldReferences[types.Zond1DataVotes].Refs() > 1 {
		// Copy elements in underlying array by reference.
		votes = make([]*zondpb.Zond1Data, len(b.zond1DataVotes))
		copy(votes, b.zond1DataVotes)
		b.sharedFieldReferences[types.Zond1DataVotes].MinusRef()
		b.sharedFieldReferences[types.Zond1DataVotes] = stateutil.NewRef(1)
	}

	b.zond1DataVotes = append(votes, val)
	b.markFieldAsDirty(types.Zond1DataVotes)
	b.addDirtyIndices(types.Zond1DataVotes, []uint64{uint64(len(b.zond1DataVotes) - 1)})
	return nil
}
