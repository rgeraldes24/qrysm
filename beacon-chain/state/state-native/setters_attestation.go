package state_native

/*
import (
	"github.com/theQRL/qrysm/v4/beacon-chain/state/state-native/types"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stateutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// RotateAttestations sets the previous epoch attestations to the current epoch attestations and
// then clears the current epoch attestations.
func (b *BeaconState) RotateAttestations() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.setPreviousEpochAttestations(b.currentEpochAttestationsVal())
	b.setCurrentEpochAttestations([]*zondpb.PendingAttestation{})
	return nil
}

func (b *BeaconState) setPreviousEpochAttestations(val []*zondpb.PendingAttestation) {
	b.sharedFieldReferences[types.PreviousEpochAttestations].MinusRef()
	b.sharedFieldReferences[types.PreviousEpochAttestations] = stateutil.NewRef(1)

	b.previousEpochAttestations = val
	b.markFieldAsDirty(types.PreviousEpochAttestations)
	b.rebuildTrie[types.PreviousEpochAttestations] = true
}

func (b *BeaconState) setCurrentEpochAttestations(val []*zondpb.PendingAttestation) {
	b.sharedFieldReferences[types.CurrentEpochAttestations].MinusRef()
	b.sharedFieldReferences[types.CurrentEpochAttestations] = stateutil.NewRef(1)

	b.currentEpochAttestations = val
	b.markFieldAsDirty(types.CurrentEpochAttestations)
	b.rebuildTrie[types.CurrentEpochAttestations] = true
}
*/
