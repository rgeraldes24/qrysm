package altair

import (
	"context"

	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation"
)

// TranslateParticipation translates pending attestations into participation bits, then inserts the bits into beacon state.
// This is helper function to convert phase 0 beacon state(pending_attestations) to Altair beacon state(participation_bits).
//
// Spec code:
// def translate_participation(state: BeaconState, pending_attestations: Sequence[phase0.PendingAttestation]) -> None:
//
//	for attestation in pending_attestations:
//	    data = attestation.data
//	    inclusion_delay = attestation.inclusion_delay
//	    # Translate attestation inclusion info to flag indices
//	    participation_flag_indices = get_attestation_participation_flag_indices(state, data, inclusion_delay)
//
//	    # Apply flags to all attesting validators
//	    epoch_participation = state.previous_epoch_participation
//	    for index in get_attesting_indices(state, data, attestation.aggregation_bits):
//	        for flag_index in participation_flag_indices:
//	            epoch_participation[index] = add_flag(epoch_participation[index], flag_index)
func TranslateParticipation(ctx context.Context, state state.BeaconState, atts []*zondpb.PendingAttestation) (state.BeaconState, error) {
	epochParticipation, err := state.PreviousEpochParticipation()
	if err != nil {
		return nil, err
	}

	for _, att := range atts {
		participatedFlags, err := AttestationParticipationFlagIndices(state, att.Data, att.InclusionDelay)
		if err != nil {
			return nil, err
		}
		committee, err := helpers.BeaconCommitteeFromState(ctx, state, att.Data.Slot, att.Data.CommitteeIndex)
		if err != nil {
			return nil, err
		}
		indices, err := attestation.AttestingIndices(att.AggregationBits, committee)
		if err != nil {
			return nil, err
		}
		cfg := params.BeaconConfig()
		sourceFlagIndex := cfg.TimelySourceFlagIndex
		targetFlagIndex := cfg.TimelyTargetFlagIndex
		headFlagIndex := cfg.TimelyHeadFlagIndex
		for _, index := range indices {
			has, err := HasValidatorFlag(epochParticipation[index], sourceFlagIndex)
			if err != nil {
				return nil, err
			}
			if participatedFlags[sourceFlagIndex] && !has {
				epochParticipation[index], err = AddValidatorFlag(epochParticipation[index], sourceFlagIndex)
				if err != nil {
					return nil, err
				}
			}
			has, err = HasValidatorFlag(epochParticipation[index], targetFlagIndex)
			if err != nil {
				return nil, err
			}
			if participatedFlags[targetFlagIndex] && !has {
				epochParticipation[index], err = AddValidatorFlag(epochParticipation[index], targetFlagIndex)
				if err != nil {
					return nil, err
				}
			}
			has, err = HasValidatorFlag(epochParticipation[index], headFlagIndex)
			if err != nil {
				return nil, err
			}
			if participatedFlags[headFlagIndex] && !has {
				epochParticipation[index], err = AddValidatorFlag(epochParticipation[index], headFlagIndex)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if err := state.SetPreviousParticipationBits(epochParticipation); err != nil {
		return nil, err
	}

	return state, nil
}
