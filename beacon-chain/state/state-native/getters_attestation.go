package state_native

// previousEpochAttestationsVal corresponding to blocks on the beacon chain.
// This assumes that a lock is already held on BeaconState.
/*
func (b *BeaconState) previousEpochAttestationsVal() []*zondpb.PendingAttestation {
	return zondpb.CopyPendingAttestationSlice(b.previousEpochAttestations)
}

// currentEpochAttestations corresponding to blocks on the beacon chain.
// This assumes that a lock is already held on BeaconState.
func (b *BeaconState) currentEpochAttestationsVal() []*zondpb.PendingAttestation {
	return zondpb.CopyPendingAttestationSlice(b.currentEpochAttestations)
}
*/
