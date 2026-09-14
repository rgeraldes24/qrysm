package params_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/testing/require"
)

func TestMainnetSlashingWindow(t *testing.T) {
	cfg := params.MainnetConfig()
	require.Equal(t, uint64(512), uint64(cfg.EpochsPerSlashingsVector))
	window := time.Duration(uint64(cfg.EpochsPerSlashingsVector)*uint64(cfg.SlotsPerEpoch)*cfg.SecondsPerSlot) * time.Second
	require.Equal(t, 45*24*time.Hour+12*time.Hour+16*time.Minute, window)
	require.Equal(t, 22*24*time.Hour+18*time.Hour+8*time.Minute, window/2)
	// Shortening the window does not change the initial or correlation penalty rates.
	require.Equal(t, uint64(32), cfg.MinSlashingPenaltyQuotient)
	require.Equal(t, uint64(3), cfg.ProportionalSlashingMultiplier)
}

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

func TestValidate_ForkVersionLength(t *testing.T) {
	for _, base := range []*params.BeaconChainConfig{params.MainnetConfig(), params.MinimalSpecConfig()} {
		t.Run(base.ConfigName, func(t *testing.T) {
			for _, version := range [][]byte{nil, {}, {1}, {1, 2}, {1, 2, 3}, {1, 2, 3, 4}, {1, 2, 3, 4, 5}, make([]byte, 8)} {
				t.Run(fmt.Sprintf("length_%d", len(version)), func(t *testing.T) {
					cfg := base.Copy()
					cfg.GenesisForkVersion = version
					err := cfg.Validate()
					if len(version) == fieldparams.VersionLength {
						require.NoError(t, err)
						return
					}
					require.ErrorContains(t, fmt.Sprintf("GENESIS_FORK_VERSION must be exactly %d bytes, got %d", fieldparams.VersionLength, len(version)), err)
				})
			}
		})
	}
}

func TestMaxActiveValidators(t *testing.T) {
	maxActiveValidators, err := params.MainnetConfig().MaxActiveValidators()
	require.NoError(t, err)
	require.Equal(t, uint64(4096), maxActiveValidators)
}

func TestValidate_RejectsBrokenArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(c *params.BeaconChainConfig)
		want   string
	}{
		{
			name:   "zero seconds per slot",
			mutate: func(c *params.BeaconChainConfig) { c.SecondsPerSlot = 0 },
			want:   "SECONDS_PER_SLOT must be non-zero",
		},
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
			name:   "zero max validators per committee",
			mutate: func(c *params.BeaconChainConfig) { c.MaxValidatorsPerCommittee = 0 },
			want:   "MAX_VALIDATORS_PER_COMMITTEE must be non-zero",
		},
		{
			name: "active validator capacity overflow",
			mutate: func(c *params.BeaconChainConfig) {
				c.MaxCommitteesPerSlot = math.MaxUint64
			},
			want: "MAX_COMMITTEES_PER_SLOT * SLOTS_PER_EPOCH overflows uint64",
		},
		{
			name: "genesis active validators exceed capacity",
			mutate: func(c *params.BeaconChainConfig) {
				maxActiveValidators, err := c.MaxActiveValidators()
				require.NoError(t, err)
				c.MinGenesisActiveValidatorCount = maxActiveValidators + 1
			},
			want: "MIN_GENESIS_ACTIVE_VALIDATOR_COUNT",
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

func TestValidateStateLayout_ExecutionVotingLimit(t *testing.T) {
	base := params.MainnetConfig()
	if fieldparams.Preset == params.MinimalName {
		base = params.MinimalSpecConfig()
	}
	const product = "EPOCHS_PER_EXECUTION_VOTING_PERIOD * SLOTS_PER_EPOCH"
	for _, tc := range []struct {
		name   string
		mutate func(*params.BeaconChainConfig)
		want   string
	}{
		{name: "preset", mutate: func(*params.BeaconChainConfig) {}},
		{name: "larger period", mutate: func(c *params.BeaconChainConfig) { c.EpochsPerExecutionVotingPeriod++ }, want: product},
		{name: "smaller period", mutate: func(c *params.BeaconChainConfig) { c.EpochsPerExecutionVotingPeriod-- }, want: product},
		{name: "zero period", mutate: func(c *params.BeaconChainConfig) { c.EpochsPerExecutionVotingPeriod = 0 }, want: product},
		{name: "shorter epoch", mutate: func(c *params.BeaconChainConfig) { c.SlotsPerEpoch /= 2 }, want: product},
		{name: "longer epoch", mutate: func(c *params.BeaconChainConfig) { c.SlotsPerEpoch *= 2 }, want: product},
		{name: "zero slots", mutate: func(c *params.BeaconChainConfig) { c.SlotsPerEpoch = 0 }, want: product},
		{name: "overflow", mutate: func(c *params.BeaconChainConfig) { c.EpochsPerExecutionVotingPeriod = math.MaxUint64 }, want: product + " overflows uint64"},
		{
			name: "overflow with matching low bits",
			mutate: func(c *params.BeaconChainConfig) {
				c.EpochsPerExecutionVotingPeriod += math.MaxUint64/fieldparams.SlotsPerEpoch + 1
			},
			want: product + " overflows uint64",
		},
		{
			name: "same capacity with different factors",
			mutate: func(c *params.BeaconChainConfig) {
				c.EpochsPerExecutionVotingPeriod *= 2
				c.SlotsPerEpoch /= 2
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base.Copy()
			tc.mutate(cfg)
			err := cfg.ValidateStateLayout()
			if tc.want != "" {
				require.ErrorContains(t, tc.want, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, uint64(fieldparams.ExecutionDataVotesLength), cfg.ExecutionDataVotesLength())
		})
	}
}

func TestValidate_CommitteeSizeBounds(t *testing.T) {
	for _, base := range []*params.BeaconChainConfig{params.MainnetConfig(), params.MinimalSpecConfig()} {
		t.Run(base.ConfigName, func(t *testing.T) {
			for _, size := range []uint64{0, 16, 31, 32, 33, 64, math.MaxUint64} {
				t.Run(fmt.Sprintf("committee_size_%d", size), func(t *testing.T) {
					cfg := base.Copy()
					cfg.MaxValidatorsPerCommittee = size
					capacity, err := cfg.MaxActiveValidators()
					if size == 0 || size > fieldparams.MaxValidatorsPerCommittee {
						require.ErrorContains(t, "MAX_VALIDATORS_PER_COMMITTEE", err)
						require.Equal(t, uint64(0), capacity)
						require.ErrorContains(t, "MAX_VALIDATORS_PER_COMMITTEE", cfg.Validate())
						require.ErrorContains(t, "MAX_VALIDATORS_PER_COMMITTEE", cfg.ValidateStateLayout())
						return
					}
					require.NoError(t, err)
					require.Equal(t, cfg.MaxCommitteesPerSlot*uint64(cfg.SlotsPerEpoch)*size, capacity)
					require.NoError(t, cfg.Validate())
					if cfg.PresetBase == fieldparams.Preset {
						require.NoError(t, cfg.ValidateStateLayout())
					}
				})
			}
		})
	}
}

func TestUnmarshalConfig_RejectsUnsafeOverrides(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "SECONDS_PER_SLOT: 0\n", want: "SECONDS_PER_SLOT must be non-zero"},
		{input: "MAX_VALIDATORS_PER_COMMITTEE: 33\n", want: "SSZ attestation limit (32)"},
		{input: "MAX_VALIDATORS_PER_COMMITTEE: 64\n", want: "SSZ attestation limit (32)"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			cfg, err := params.UnmarshalConfig([]byte("CONFIG_NAME: mainnet\n"+tc.input), nil)
			require.ErrorContains(t, "invalid chain config", err)
			require.ErrorContains(t, tc.want, err)
			require.Equal(t, true, cfg == nil)
		})
	}
}

func TestUnmarshalConfig_RejectsInvalidArithmetic(t *testing.T) {
	// A chain config file overriding a single value on top of the mainnet
	// defaults; the override breaks process_slashings.
	yaml := []byte("PROPORTIONAL_SLASHING_MULTIPLIER: 0\n")
	_, err := params.UnmarshalConfig(yaml, nil)
	require.ErrorContains(t, "PROPORTIONAL_SLASHING_MULTIPLIER must be non-zero", err)
}
