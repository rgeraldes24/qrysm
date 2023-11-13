package state_native

import (
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// Zond1Data corresponding to the proof-of-work chain information stored in the beacon state.
func (b *BeaconState) Zond1Data() *zondpb.Zond1Data {
	if b.zond1Data == nil {
		return nil
	}

	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.zond1DataVal()
}

// zond1DataVal corresponding to the proof-of-work chain information stored in the beacon state.
// This assumes that a lock is already held on BeaconState.
func (b *BeaconState) zond1DataVal() *zondpb.Zond1Data {
	if b.zond1Data == nil {
		return nil
	}

	return zondpb.CopyZOND1Data(b.zond1Data)
}

// Zond1DataVotes corresponds to votes from Ethereum on the canonical proof-of-work chain
// data retrieved from zond1.
func (b *BeaconState) Zond1DataVotes() []*zondpb.Zond1Data {
	if b.zond1DataVotes == nil {
		return nil
	}

	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.zond1DataVotesVal()
}

// zond1DataVotesVal corresponds to votes from Ethereum on the canonical proof-of-work chain
// data retrieved from zond1.
// This assumes that a lock is already held on BeaconState.
func (b *BeaconState) zond1DataVotesVal() []*zondpb.Zond1Data {
	if b.zond1DataVotes == nil {
		return nil
	}

	res := make([]*zondpb.Zond1Data, len(b.zond1DataVotes))
	for i := 0; i < len(res); i++ {
		res[i] = zondpb.CopyZOND1Data(b.zond1DataVotes[i])
	}
	return res
}

// Zond1DepositIndex corresponds to the index of the deposit made to the
// validator deposit contract at the time of this state's zond1 data.
func (b *BeaconState) Zond1DepositIndex() uint64 {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.zond1DepositIndex
}
