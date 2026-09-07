package params

import (
	"fmt"
	"math/bits"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
)

// Validate checks the arithmetic invariants that the consensus code assumes a
// configuration satisfies. The epoch transition divides by, reduces modulo and
// multiplies these values without checking them (process_slashings,
// slash_validator, process_rewards_and_penalties, process_registry_updates,
// process_effective_balance_updates, sync committee rewards), so a value that
// breaks an invariant either panics the node at the next epoch boundary or
// silently miscomputes balances. Built-in presets are covered by tests; a
// user-supplied chain config file is checked when it is loaded.
func (b *BeaconChainConfig) Validate() error {
	nonZero := []struct {
		name  string
		value uint64
	}{
		// Balances and slashing penalties.
		{"EFFECTIVE_BALANCE_INCREMENT", b.EffectiveBalanceIncrement},
		{"MIN_SLASHING_PENALTY_QUOTIENT", b.MinSlashingPenaltyQuotient},
		{"PROPORTIONAL_SLASHING_MULTIPLIER", b.ProportionalSlashingMultiplier},
		{"WHISTLEBLOWER_REWARD_QUOTIENT", b.WhistleBlowerRewardQuotient},
		{"PROPOSER_REWARD_QUOTIENT", b.ProposerRewardQuotient},
		{"EPOCHS_PER_SLASHINGS_VECTOR", uint64(b.EpochsPerSlashingsVector)},
		{"HYSTERESIS_QUOTIENT", b.HysteresisQuotient},
		// Rewards, penalties and registry updates.
		{"INACTIVITY_PENALTY_QUOTIENT", b.InactivityPenaltyQuotient},
		{"CHURN_LIMIT_QUOTIENT", b.ChurnLimitQuotient},
		{"MAX_PER_EPOCH_ACTIVATION_CHURN_LIMIT", b.MaxPerEpochActivationChurnLimit},
		{"WEIGHT_DENOMINATOR", b.WeightDenominator},
		{"PROPOSER_WEIGHT", b.ProposerWeight},
		{"SLOTS_PER_EPOCH", uint64(b.SlotsPerEpoch)},
		{"EPOCHS_PER_HISTORICAL_VECTOR", uint64(b.EpochsPerHistoricalVector)},
		{"SLOTS_PER_HISTORICAL_ROOT", uint64(b.SlotsPerHistoricalRoot)},
		{"SYNC_COMMITTEE_SIZE", b.SyncCommitteeSize},
		{"SYNC_COMMITTEE_SUBNET_COUNT", b.SyncCommitteeSubnetCount},
		{"EPOCHS_PER_SYNC_COMMITTEE_PERIOD", uint64(b.EpochsPerSyncCommitteePeriod)},
	}
	for _, c := range nonZero {
		if c.value == 0 {
			return fmt.Errorf("%s must be non-zero", c.name)
		}
	}

	// Effective balances are rounded down to a multiple of the increment and
	// capped at MAX_EFFECTIVE_BALANCE; the slashing and reward code then works
	// in whole increments (effective_balance / EFFECTIVE_BALANCE_INCREMENT).
	if b.MaxEffectiveBalance < b.EffectiveBalanceIncrement || b.MaxEffectiveBalance%b.EffectiveBalanceIncrement != 0 {
		return fmt.Errorf("MAX_EFFECTIVE_BALANCE (%d) must be a positive multiple of EFFECTIVE_BALANCE_INCREMENT (%d)",
			b.MaxEffectiveBalance, b.EffectiveBalanceIncrement)
	}
	// process_effective_balance_updates: hysteresis thresholds are
	// hysteresis_increment * multiplier in uint64.
	hysteresisIncrement := b.EffectiveBalanceIncrement / b.HysteresisQuotient
	if hi, _ := bits.Mul64(hysteresisIncrement, b.HysteresisDownwardMultiplier); hi != 0 {
		return fmt.Errorf("EFFECTIVE_BALANCE_INCREMENT / HYSTERESIS_QUOTIENT * HYSTERESIS_DOWNWARD_MULTIPLIER overflows uint64")
	}
	if hi, _ := bits.Mul64(hysteresisIncrement, b.HysteresisUpwardMultiplier); hi != 0 {
		return fmt.Errorf("EFFECTIVE_BALANCE_INCREMENT / HYSTERESIS_QUOTIENT * HYSTERESIS_UPWARD_MULTIPLIER overflows uint64")
	}

	// get_base_reward_per_increment computes
	// EFFECTIVE_BALANCE_INCREMENT * BASE_REWARD_FACTOR in uint64.
	if hi, _ := bits.Mul64(b.EffectiveBalanceIncrement, b.BaseRewardFactor); hi != 0 {
		return fmt.Errorf("EFFECTIVE_BALANCE_INCREMENT (%d) * BASE_REWARD_FACTOR (%d) overflows uint64",
			b.EffectiveBalanceIncrement, b.BaseRewardFactor)
	}

	// The participation flag weights plus the sync and proposer weights must
	// sum to WEIGHT_DENOMINATOR (spec invariant: rewards are distributed as
	// weight / WEIGHT_DENOMINATOR shares of the base reward). The attestation
	// proposer reward additionally divides by PROPOSER_WEIGHT and computes
	// (WEIGHT_DENOMINATOR - PROPOSER_WEIGHT) * WEIGHT_DENOMINATOR.
	weightSum := b.TimelySourceWeight + b.TimelyTargetWeight + b.TimelyHeadWeight + b.SyncRewardWeight + b.ProposerWeight
	if weightSum != b.WeightDenominator {
		return fmt.Errorf("TIMELY_SOURCE_WEIGHT + TIMELY_TARGET_WEIGHT + TIMELY_HEAD_WEIGHT + SYNC_REWARD_WEIGHT + PROPOSER_WEIGHT (%d) must equal WEIGHT_DENOMINATOR (%d)",
			weightSum, b.WeightDenominator)
	}
	if hi, _ := bits.Mul64(b.WeightDenominator-b.ProposerWeight, b.WeightDenominator); hi != 0 {
		return fmt.Errorf("(WEIGHT_DENOMINATOR - PROPOSER_WEIGHT) * WEIGHT_DENOMINATOR overflows uint64")
	}

	// Sync committee subnets partition the committee.
	if b.SyncCommitteeSize%b.SyncCommitteeSubnetCount != 0 {
		return fmt.Errorf("SYNC_COMMITTEE_SIZE (%d) must be a multiple of SYNC_COMMITTEE_SUBNET_COUNT (%d)",
			b.SyncCommitteeSize, b.SyncCommitteeSubnetCount)
	}
	return nil
}

// ValidateStateLayout checks that the configuration's vector sizes match the
// SSZ layout compiled into this binary (config/fieldparams, selected by the
// mainnet/minimal build tag). The beacon state's slashings, randao_mixes,
// block_roots, state_roots and historical_roots vectors, the validator
// registry limit and the sync committee size are fixed at build time, while
// the epoch transition indexes them with EPOCHS_PER_SLASHINGS_VECTOR,
// EPOCHS_PER_HISTORICAL_VECTOR and SLOTS_PER_HISTORICAL_ROOT from the runtime
// config: a mismatch ends in an out-of-range panic or in slashing / RANDAO
// lookups landing on the wrong epoch, not in an error.
func (b *BeaconChainConfig) ValidateStateLayout() error {
	checks := []struct {
		name string
		cfg  uint64
		ssz  uint64
	}{
		{"EPOCHS_PER_SLASHINGS_VECTOR", uint64(b.EpochsPerSlashingsVector), fieldparams.SlashingsLength},
		{"EPOCHS_PER_HISTORICAL_VECTOR", uint64(b.EpochsPerHistoricalVector), fieldparams.RandaoMixesLength},
		{"SLOTS_PER_HISTORICAL_ROOT", uint64(b.SlotsPerHistoricalRoot), fieldparams.BlockRootsLength},
		{"SLOTS_PER_HISTORICAL_ROOT", uint64(b.SlotsPerHistoricalRoot), fieldparams.StateRootsLength},
		{"HISTORICAL_ROOTS_LIMIT", b.HistoricalRootsLimit, fieldparams.HistoricalRootsLength},
		{"VALIDATOR_REGISTRY_LIMIT", b.ValidatorRegistryLimit, fieldparams.ValidatorRegistryLimit},
		{"SYNC_COMMITTEE_SIZE", b.SyncCommitteeSize, fieldparams.SyncCommitteeLength},
	}
	for _, c := range checks {
		if c.cfg != c.ssz {
			return fmt.Errorf("%s is %d but this binary's SSZ state layout is compiled for %d (mainnet/minimal build mismatch)",
				c.name, c.cfg, c.ssz)
		}
	}
	return nil
}
