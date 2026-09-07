package params_test

import (
	"math"
	"testing"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/testing/require"
)

func TestValidate_BuiltInConfigs(t *testing.T) {
	configs := map[string]*params.BeaconChainConfig{
		"mainnet":     params.MainnetConfig(),
		"minimal":     params.MinimalSpecConfig(),
		"e2e":         params.E2ETestConfig(),
		"e2e-mainnet": params.E2EMainnetTestConfig(),
	}
	for name, cfg := range configs {
		require.NoError(t, cfg.Validate(), "%s config fails validation", name)
	}
}

func TestValidate_RejectsBrokenArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(c *params.BeaconChainConfig)
		want   string
	}{
		{
			name:   "zero effective balance increment",
			mutate: func(c *params.BeaconChainConfig) { c.EffectiveBalanceIncrement = 0 },
			want:   "EFFECTIVE_BALANCE_INCREMENT must be non-zero",
		},
		{
			name:   "zero min slashing penalty quotient",
			mutate: func(c *params.BeaconChainConfig) { c.MinSlashingPenaltyQuotient = 0 },
			want:   "MIN_SLASHING_PENALTY_QUOTIENT must be non-zero",
		},
		{
			name:   "zero proportional slashing multiplier",
			mutate: func(c *params.BeaconChainConfig) { c.ProportionalSlashingMultiplier = 0 },
			want:   "PROPORTIONAL_SLASHING_MULTIPLIER must be non-zero",
		},
		{
			name:   "zero whistleblower reward quotient",
			mutate: func(c *params.BeaconChainConfig) { c.WhistleBlowerRewardQuotient = 0 },
			want:   "WHISTLEBLOWER_REWARD_QUOTIENT must be non-zero",
		},
		{
			name:   "zero proposer reward quotient",
			mutate: func(c *params.BeaconChainConfig) { c.ProposerRewardQuotient = 0 },
			want:   "PROPOSER_REWARD_QUOTIENT must be non-zero",
		},
		{
			name:   "zero epochs per slashings vector",
			mutate: func(c *params.BeaconChainConfig) { c.EpochsPerSlashingsVector = 0 },
			want:   "EPOCHS_PER_SLASHINGS_VECTOR must be non-zero",
		},
		{
			name:   "zero hysteresis quotient",
			mutate: func(c *params.BeaconChainConfig) { c.HysteresisQuotient = 0 },
			want:   "HYSTERESIS_QUOTIENT must be non-zero",
		},
		{
			name:   "zero inactivity penalty quotient",
			mutate: func(c *params.BeaconChainConfig) { c.InactivityPenaltyQuotient = 0 },
			want:   "INACTIVITY_PENALTY_QUOTIENT must be non-zero",
		},
		{
			name:   "zero churn limit quotient",
			mutate: func(c *params.BeaconChainConfig) { c.ChurnLimitQuotient = 0 },
			want:   "CHURN_LIMIT_QUOTIENT must be non-zero",
		},
		{
			name:   "zero max per epoch activation churn limit",
			mutate: func(c *params.BeaconChainConfig) { c.MaxPerEpochActivationChurnLimit = 0 },
			want:   "MAX_PER_EPOCH_ACTIVATION_CHURN_LIMIT must be non-zero",
		},
		{
			name:   "zero weight denominator",
			mutate: func(c *params.BeaconChainConfig) { c.WeightDenominator = 0 },
			want:   "WEIGHT_DENOMINATOR must be non-zero",
		},
		{
			name:   "zero sync committee subnet count",
			mutate: func(c *params.BeaconChainConfig) { c.SyncCommitteeSubnetCount = 0 },
			want:   "SYNC_COMMITTEE_SUBNET_COUNT must be non-zero",
		},
		{
			name: "max effective balance not a multiple of the increment",
			mutate: func(c *params.BeaconChainConfig) {
				c.MaxEffectiveBalance = 3*c.EffectiveBalanceIncrement + 1
			},
			want: "must be a positive multiple of EFFECTIVE_BALANCE_INCREMENT",
		},
		{
			name: "max effective balance below the increment",
			mutate: func(c *params.BeaconChainConfig) {
				c.MaxEffectiveBalance = c.EffectiveBalanceIncrement - 1
			},
			want: "must be a positive multiple of EFFECTIVE_BALANCE_INCREMENT",
		},
		{
			name:   "hysteresis threshold overflow",
			mutate: func(c *params.BeaconChainConfig) { c.HysteresisUpwardMultiplier = math.MaxUint64 },
			want:   "HYSTERESIS_UPWARD_MULTIPLIER overflows uint64",
		},
		{
			name:   "base reward per increment overflow",
			mutate: func(c *params.BeaconChainConfig) { c.BaseRewardFactor = math.MaxUint64 },
			want:   "BASE_REWARD_FACTOR",
		},
		{
			name:   "participation weights do not sum to the denominator",
			mutate: func(c *params.BeaconChainConfig) { c.TimelyHeadWeight++ },
			want:   "must equal WEIGHT_DENOMINATOR",
		},
		{
			name: "sync committee not divisible into subnets",
			mutate: func(c *params.BeaconChainConfig) {
				c.SyncCommitteeSubnetCount = c.SyncCommitteeSize + 1
			},
			want: "SYNC_COMMITTEE_SIZE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := params.MainnetConfig().Copy()
			tt.mutate(cfg)
			require.ErrorContains(t, tt.want, cfg.Validate())
		})
	}
}

func TestValidateStateLayout(t *testing.T) {
	// Pick the preset whose vectors match the SSZ layout this test binary was
	// compiled with (mainnet unless built with the minimal tag).
	cfg := params.MainnetConfig().Copy()
	if uint64(cfg.EpochsPerSlashingsVector) != fieldparams.SlashingsLength {
		cfg = params.MinimalSpecConfig().Copy()
	}
	require.NoError(t, cfg.ValidateStateLayout())

	broken := cfg.Copy()
	broken.EpochsPerSlashingsVector++
	require.ErrorContains(t, "EPOCHS_PER_SLASHINGS_VECTOR", broken.ValidateStateLayout())

	broken = cfg.Copy()
	broken.EpochsPerHistoricalVector++
	require.ErrorContains(t, "EPOCHS_PER_HISTORICAL_VECTOR", broken.ValidateStateLayout())

	broken = cfg.Copy()
	broken.SyncCommitteeSize *= 2
	require.ErrorContains(t, "SYNC_COMMITTEE_SIZE", broken.ValidateStateLayout())
}

func TestUnmarshalConfig_RejectsInvalidArithmetic(t *testing.T) {
	// A chain config file overriding a single value on top of the mainnet
	// defaults; the override breaks process_slashings.
	yaml := []byte("PROPORTIONAL_SLASHING_MULTIPLIER: 0\n")
	_, err := params.UnmarshalConfig(yaml, nil)
	require.ErrorContains(t, "PROPORTIONAL_SLASHING_MULTIPLIER must be non-zero", err)
}
