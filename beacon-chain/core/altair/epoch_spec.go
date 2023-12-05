package altair

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
)

// ProcessSyncCommitteeUpdates  processes sync client committee updates for the beacon state.
func ProcessSyncCommitteeUpdates(ctx context.Context, beaconState state.BeaconState) (state.BeaconState, error) {
	nextEpoch := time.NextEpoch(beaconState)
	if nextEpoch%params.BeaconConfig().EpochsPerSyncCommitteePeriod == 0 {
		nextSyncCommittee, err := beaconState.NextSyncCommittee()
		if err != nil {
			return nil, err
		}
		if err := beaconState.SetCurrentSyncCommittee(nextSyncCommittee); err != nil {
			return nil, err
		}
		nextSyncCommittee, err = NextSyncCommittee(ctx, beaconState)
		if err != nil {
			return nil, err
		}
		if err := beaconState.SetNextSyncCommittee(nextSyncCommittee); err != nil {
			return nil, err
		}
		if err := helpers.UpdateSyncCommitteeCache(beaconState); err != nil {
			log.WithError(err).Error("Could not update sync committee cache")
		}
	}
	return beaconState, nil
}

// ProcessParticipationFlagUpdates processes participation flag updates by rotating current to previous.
func ProcessParticipationFlagUpdates(beaconState state.BeaconState) (state.BeaconState, error) {
	c, err := beaconState.CurrentEpochParticipation()
	if err != nil {
		return nil, err
	}
	if err := beaconState.SetPreviousParticipationBits(c); err != nil {
		return nil, err
	}
	if err := beaconState.SetCurrentParticipationBits(make([]byte, beaconState.NumValidators())); err != nil {
		return nil, err
	}
	return beaconState, nil
}
