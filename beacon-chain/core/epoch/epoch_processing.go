// Package epoch contains epoch processing libraries according to spec, able to
// process new balance for validators, justify and finalize new
// check points, and shuffle validators to different slots and
// shards.
package epoch

import (
	"context"
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/validators"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stateutil"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/math"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation"
)

// sortableIndices implements the Sort interface to sort newly activated validator indices
// by activation epoch and by index number.
type sortableIndices struct {
	indices    []primitives.ValidatorIndex
	validators []*zondpb.Validator
}

// Len is the number of elements in the collection.
func (s sortableIndices) Len() int { return len(s.indices) }

// Swap swaps the elements with indexes i and j.
func (s sortableIndices) Swap(i, j int) { s.indices[i], s.indices[j] = s.indices[j], s.indices[i] }

// Less reports whether the element with index i must sort before the element with index j.
func (s sortableIndices) Less(i, j int) bool {
	if s.validators[s.indices[i]].ActivationEligibilityEpoch == s.validators[s.indices[j]].ActivationEligibilityEpoch {
		return s.indices[i] < s.indices[j]
	}
	return s.validators[s.indices[i]].ActivationEligibilityEpoch < s.validators[s.indices[j]].ActivationEligibilityEpoch
}

// AttestingBalance returns the total balance from all the attesting indices.
//
// WARNING: This method allocates a new copy of the attesting validator indices set and is
// considered to be very memory expensive. Avoid using this unless you really
// need to get attesting balance from attestations.
func AttestingBalance(ctx context.Context, state state.ReadOnlyBeaconState, atts []*zondpb.PendingAttestation) (uint64, error) {
	indices, err := UnslashedAttestingIndices(ctx, state, atts)
	if err != nil {
		return 0, errors.Wrap(err, "could not get attesting indices")
	}
	return helpers.TotalBalance(state, indices), nil
}

// ProcessRegistryUpdates rotates validators in and out of active pool.
// the amount to rotate is determined churn limit.
func ProcessRegistryUpdates(ctx context.Context, state state.BeaconState) (state.BeaconState, error) {
	currentEpoch := time.CurrentEpoch(state)
	vals := state.Validators()
	var err error
	ejectionBal := params.BeaconConfig().EjectionBalance
	activationEligibilityEpoch := time.CurrentEpoch(state) + 1
	for idx, validator := range vals {
		// Process the validators for activation eligibility.
		if helpers.IsEligibleForActivationQueue(validator) {
			validator.ActivationEligibilityEpoch = activationEligibilityEpoch
			if err := state.UpdateValidatorAtIndex(primitives.ValidatorIndex(idx), validator); err != nil {
				return nil, err
			}
		}

		// Process the validators for ejection.
		isActive := helpers.IsActiveValidator(validator, currentEpoch)
		belowEjectionBalance := validator.EffectiveBalance <= ejectionBal
		if isActive && belowEjectionBalance {
			state, err = validators.InitiateValidatorExit(ctx, state, primitives.ValidatorIndex(idx))
			if err != nil {
				return nil, errors.Wrapf(err, "could not initiate exit for validator %d", idx)
			}
		}
	}

	// Queue validators eligible for activation and not yet dequeued for activation.
	var activationQ []primitives.ValidatorIndex
	for idx, validator := range vals {
		if helpers.IsEligibleForActivation(state, validator) {
			activationQ = append(activationQ, primitives.ValidatorIndex(idx))
		}
	}

	sort.Sort(sortableIndices{indices: activationQ, validators: vals})

	// Only activate just enough validators according to the activation churn limit.
	limit := uint64(len(activationQ))
	activeValidatorCount, err := helpers.ActiveValidatorCount(ctx, state, currentEpoch)
	if err != nil {
		return nil, errors.Wrap(err, "could not get active validator count")
	}

	churnLimit, err := helpers.ValidatorChurnLimit(activeValidatorCount)
	if err != nil {
		return nil, errors.Wrap(err, "could not get churn limit")
	}

	// Prevent churn limit cause index out of bound.
	if churnLimit < limit {
		limit = churnLimit
	}

	activationExitEpoch := helpers.ActivationExitEpoch(currentEpoch)
	for _, index := range activationQ[:limit] {
		validator, err := state.ValidatorAtIndex(index)
		if err != nil {
			return nil, err
		}
		validator.ActivationEpoch = activationExitEpoch
		if err := state.UpdateValidatorAtIndex(index, validator); err != nil {
			return nil, err
		}
	}
	return state, nil
}

// ProcessSlashings processes the slashed validators during epoch processing,
func ProcessSlashings(state state.BeaconState, slashingMultiplier uint64) (state.BeaconState, error) {
	currentEpoch := time.CurrentEpoch(state)
	totalBalance, err := helpers.TotalActiveBalance(state)
	if err != nil {
		return nil, errors.Wrap(err, "could not get total active balance")
	}

	// Compute slashed balances in the current epoch
	exitLength := params.BeaconConfig().EpochsPerSlashingsVector

	// Compute the sum of state slashings
	slashings := state.Slashings()
	totalSlashing := uint64(0)
	for _, slashing := range slashings {
		totalSlashing, err = math.Add64(totalSlashing, slashing)
		if err != nil {
			return nil, err
		}
	}

	// a callback is used here to apply the following actions to all validators
	// below equally.
	increment := params.BeaconConfig().EffectiveBalanceIncrement
	minSlashing := math.Min(totalSlashing*slashingMultiplier, totalBalance)
	err = state.ApplyToEveryValidator(func(idx int, val *zondpb.Validator) (bool, *zondpb.Validator, error) {
		correctEpoch := (currentEpoch + exitLength/2) == val.WithdrawableEpoch
		if val.Slashed && correctEpoch {
			penaltyNumerator := val.EffectiveBalance / increment * minSlashing
			penalty := penaltyNumerator / totalBalance * increment
			if err := helpers.DecreaseBalance(state, primitives.ValidatorIndex(idx), penalty); err != nil {
				return false, val, err
			}
			return true, val, nil
		}
		return false, val, nil
	})
	return state, err
}

// ProcessZond1DataReset processes updates to ZOND1 data votes during epoch processing.
func ProcessZond1DataReset(state state.BeaconState) (state.BeaconState, error) {
	currentEpoch := time.CurrentEpoch(state)
	nextEpoch := currentEpoch + 1

	// Reset ZOND1 data votes.
	if nextEpoch%params.BeaconConfig().EpochsPerZond1VotingPeriod == 0 {
		if err := state.SetZond1DataVotes([]*zondpb.Zond1Data{}); err != nil {
			return nil, err
		}
	}

	return state, nil
}

// ProcessEffectiveBalanceUpdates processes effective balance updates during epoch processing.
func ProcessEffectiveBalanceUpdates(state state.BeaconState) (state.BeaconState, error) {
	effBalanceInc := params.BeaconConfig().EffectiveBalanceIncrement
	maxEffBalance := params.BeaconConfig().MaxEffectiveBalance
	hysteresisInc := effBalanceInc / params.BeaconConfig().HysteresisQuotient
	downwardThreshold := hysteresisInc * params.BeaconConfig().HysteresisDownwardMultiplier
	upwardThreshold := hysteresisInc * params.BeaconConfig().HysteresisUpwardMultiplier

	bals := state.Balances()

	// Update effective balances with hysteresis.
	validatorFunc := func(idx int, val *zondpb.Validator) (bool, *zondpb.Validator, error) {
		if val == nil {
			return false, nil, fmt.Errorf("validator %d is nil in state", idx)
		}
		if idx >= len(bals) {
			return false, nil, fmt.Errorf("validator index exceeds validator length in state %d >= %d", idx, len(state.Balances()))
		}
		balance := bals[idx]

		if balance+downwardThreshold < val.EffectiveBalance || val.EffectiveBalance+upwardThreshold < balance {
			effectiveBal := maxEffBalance
			if effectiveBal > balance-balance%effBalanceInc {
				effectiveBal = balance - balance%effBalanceInc
			}
			if effectiveBal != val.EffectiveBalance {
				newVal := zondpb.CopyValidator(val)
				newVal.EffectiveBalance = effectiveBal
				return true, newVal, nil
			}
			return false, val, nil
		}
		return false, val, nil
	}

	if err := state.ApplyToEveryValidator(validatorFunc); err != nil {
		return nil, err
	}

	return state, nil
}

// ProcessSlashingsReset processes the total slashing balances updates during epoch processing.
func ProcessSlashingsReset(state state.BeaconState) (state.BeaconState, error) {
	currentEpoch := time.CurrentEpoch(state)
	nextEpoch := currentEpoch + 1

	// Set total slashed balances.
	slashedExitLength := params.BeaconConfig().EpochsPerSlashingsVector
	slashedEpoch := nextEpoch % slashedExitLength
	slashings := state.Slashings()
	if uint64(len(slashings)) != uint64(slashedExitLength) {
		return nil, fmt.Errorf(
			"state slashing length %d different than EpochsPerHistoricalVector %d",
			len(slashings),
			slashedExitLength,
		)
	}
	if err := state.UpdateSlashingsAtIndex(uint64(slashedEpoch) /* index */, 0 /* value */); err != nil {
		return nil, err
	}

	return state, nil
}

// ProcessRandaoMixesReset processes the final updates to RANDAO mix during epoch processing.
func ProcessRandaoMixesReset(state state.BeaconState) (state.BeaconState, error) {
	currentEpoch := time.CurrentEpoch(state)
	nextEpoch := currentEpoch + 1

	// Set RANDAO mix.
	randaoMixLength := params.BeaconConfig().EpochsPerHistoricalVector
	if uint64(state.RandaoMixesLength()) != uint64(randaoMixLength) {
		return nil, fmt.Errorf(
			"state randao length %d different than EpochsPerHistoricalVector %d",
			state.RandaoMixesLength(),
			randaoMixLength,
		)
	}
	mix, err := helpers.RandaoMix(state, currentEpoch)
	if err != nil {
		return nil, err
	}
	if err := state.UpdateRandaoMixesAtIndex(uint64(nextEpoch%randaoMixLength), mix); err != nil {
		return nil, err
	}

	return state, nil
}

// ProcessHistoricalDataUpdate processes the updates to historical data during epoch processing.
// From Capella onward, per spec,state's historical summaries are updated instead of historical roots.
func ProcessHistoricalDataUpdate(state state.BeaconState) (state.BeaconState, error) {
	currentEpoch := time.CurrentEpoch(state)
	nextEpoch := currentEpoch + 1

	// Set historical root accumulator.
	epochsPerHistoricalRoot := params.BeaconConfig().SlotsPerHistoricalRoot.DivSlot(params.BeaconConfig().SlotsPerEpoch)
	if nextEpoch.Mod(uint64(epochsPerHistoricalRoot)) == 0 {
		br, err := stateutil.ArraysRoot(state.BlockRoots(), fieldparams.BlockRootsLength)
		if err != nil {
			return nil, err
		}
		sr, err := stateutil.ArraysRoot(state.StateRoots(), fieldparams.StateRootsLength)
		if err != nil {
			return nil, err
		}
		if err := state.AppendHistoricalSummaries(&zondpb.HistoricalSummary{BlockSummaryRoot: br[:], StateSummaryRoot: sr[:]}); err != nil {
			return nil, err
		}
	}

	return state, nil
}

// UnslashedAttestingIndices returns all the attesting indices from a list of attestations,
// it sorts the indices and filters out the slashed ones.
func UnslashedAttestingIndices(ctx context.Context, state state.ReadOnlyBeaconState, atts []*zondpb.PendingAttestation) ([]primitives.ValidatorIndex, error) {
	var setIndices []primitives.ValidatorIndex
	seen := make(map[uint64]bool)

	for _, att := range atts {
		committee, err := helpers.BeaconCommitteeFromState(ctx, state, att.Data.Slot, att.Data.CommitteeIndex)
		if err != nil {
			return nil, err
		}
		attestingIndices, err := attestation.AttestingIndices(att.ParticipationBits, committee)
		if err != nil {
			return nil, err
		}
		// Create a set for attesting indices
		for _, index := range attestingIndices {
			if !seen[index] {
				setIndices = append(setIndices, primitives.ValidatorIndex(index))
			}
			seen[index] = true
		}
	}
	// Sort the attesting set indices by increasing order.
	sort.Slice(setIndices, func(i, j int) bool { return setIndices[i] < setIndices[j] })
	// Remove the slashed validator indices.
	for i := 0; i < len(setIndices); i++ {
		v, err := state.ValidatorAtIndexReadOnly(setIndices[i])
		if err != nil {
			return nil, errors.Wrap(err, "failed to look up validator")
		}
		if !v.IsNil() && v.Slashed() {
			setIndices = append(setIndices[:i], setIndices[i+1:]...)
		}
	}

	return setIndices, nil
}
