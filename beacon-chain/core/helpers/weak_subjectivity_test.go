package helpers_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/beacon-chain/state"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
)

func TestWeakSubjectivity_ComputeWeakSubjectivityPeriod(t *testing.T) {
	tests := []struct {
		valCount   uint64
		avgBalance uint64
		want       primitives.Epoch
		wantErr    string
	}{
		// These use the validator counts and balances from the reference table:
		// https://github.com/ethereum/consensus-specs/blob/master/specs/phase0/weak-subjectivity.md#calculating-the-weak-subjectivity-period
		// The expected periods reflect Qrysm's active consensus configuration.
		//
		// genState scales avgBalance by 1e12 (QRL balances are ~1000x Ethereum's
		// per validator), so rows whose total active balance exceeds uint64
		// (valCount * avgBalance * 1e12 >= 2^64) must fail with the checked-add
		// error from TotalActiveBalance rather than silently wrap.
		{valCount: 32768, avgBalance: 35, want: 512},
		{valCount: 65536, avgBalance: 35, want: 1008},
		{valCount: 131072, avgBalance: 35, want: 2001},
		{valCount: 262144, avgBalance: 35, want: 2001},
		{valCount: 524288, avgBalance: 35, want: 2001},
		{valCount: 1048576, avgBalance: 35, wantErr: "addition overflows"},
		{valCount: 32768, avgBalance: 40, want: 835},
		{valCount: 65536, avgBalance: 40, want: 1654},
		{valCount: 131072, avgBalance: 40, want: 3292},
		{valCount: 262144, avgBalance: 40, want: 3292},
		{valCount: 524288, avgBalance: 40, wantErr: "addition overflows"},
		{valCount: 1048576, avgBalance: 40, wantErr: "addition overflows"},
		// Additional test vectors, to check case when T*(200+3*D) >= t*(200+12*D)
		{valCount: 32768, avgBalance: 28, want: 21},
		{valCount: 65536, avgBalance: 28, want: 27},
		{valCount: 131072, avgBalance: 28, want: 38},
		{valCount: 262144, avgBalance: 28, want: 60},
		{valCount: 524288, avgBalance: 28, want: 105},
		{valCount: 1048576, avgBalance: 28, wantErr: "addition overflows"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("valCount: %d, avgBalance: %d", tt.valCount, tt.avgBalance), func(t *testing.T) {
			// Reset committee cache - as we need to recalculate active validator set for each test.
			helpers.ClearCache()
			got, err := helpers.ComputeWeakSubjectivityPeriod(context.Background(), genState(t, tt.valCount, tt.avgBalance), params.BeaconConfig())
			if tt.wantErr != "" {
				require.ErrorContains(t, tt.wantErr, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "valCount: %v, avgBalance: %v", tt.valCount, tt.avgBalance)
		})
	}
}

func TestWeakSubjectivity_ProposerHistoryRetentionAtValidatorCap(t *testing.T) {
	const maxActiveValidators = 4096
	state := genState(t, maxActiveValidators, 40)
	wsp, err := helpers.ComputeWeakSubjectivityPeriod(context.Background(), state, params.BeaconConfig())
	require.NoError(t, err)
	require.Equal(t, primitives.Epoch(118), wsp)

	// Proposal-history pruning deletes a record when its age equals this value,
	// so the retained period must be strictly greater than the dynamic WSP.
	require.Equal(t, true, params.BeaconConfig().WeakSubjectivityPeriod > wsp)
}

type mockWsCheckpoint func() (stateRoot [32]byte, blockRoot [32]byte, e primitives.Epoch)

func TestWeakSubjectivity_ParseWeakSubjectivityInputString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		checkpt   *qrysmpb.Checkpoint
		wantedErr string
	}{
		{
			name:      "No column in string",
			input:     "0x111111;123",
			wantedErr: "did not contain column",
		},
		{
			name:      "Too many columns in string",
			input:     "0x010203:123:456",
			wantedErr: "weak subjectivity checkpoint input should be in `block_root:epoch_number` format",
		},
		{
			name:      "Incorrect block root length",
			input:     "0x010203:987",
			wantedErr: "block root is not length of 32",
		},
		{
			name:  "Correct input",
			input: "0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:123456789",
			checkpt: &qrysmpb.Checkpoint{
				Root:  []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
				Epoch: primitives.Epoch(123456789),
			},
		},
		{
			name:  "Correct input without 0x",
			input: "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:123456789",
			checkpt: &qrysmpb.Checkpoint{
				Root:  []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
				Epoch: primitives.Epoch(123456789),
			},
		},
		{
			name:  "Correct input",
			input: "0xF0FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:123456789",
			checkpt: &qrysmpb.Checkpoint{
				Root:  []byte{0xf0, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
				Epoch: primitives.Epoch(123456789),
			},
		},
		{
			name:  "Correct input without 0x",
			input: "F0FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:123456789",
			checkpt: &qrysmpb.Checkpoint{
				Root:  []byte{0xf0, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
				Epoch: primitives.Epoch(123456789),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wsCheckpt, err := helpers.ParseWeakSubjectivityInputString(tt.input)
			if tt.wantedErr != "" {
				require.ErrorContains(t, tt.wantedErr, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, wsCheckpt)
			require.DeepEqual(t, tt.checkpt.Root, wsCheckpt.Root, "Roots do not match")
			require.Equal(t, tt.checkpt.Epoch, wsCheckpt.Epoch, "Epochs do not match")
		})
	}
}

func genState(t *testing.T, valCount, avgBalance uint64) state.BeaconState {
	beaconState, err := util.NewBeaconStateZond()
	require.NoError(t, err)

	validators := make([]*qrysmpb.Validator, valCount)
	balances := make([]uint64, len(validators))
	for i := range valCount {
		validators[i] = &qrysmpb.Validator{
			PublicKey:           make([]byte, field_params.MLDSA87PubkeyLength),
			WithdrawalRecipient: make([]byte, field_params.WithdrawalRecipientLength),
			EffectiveBalance:    avgBalance * 1e12,
			ExitEpoch:           params.BeaconConfig().FarFutureEpoch,
		}
		balances[i] = validators[i].EffectiveBalance
	}

	require.NoError(t, beaconState.SetValidators(validators))
	require.NoError(t, beaconState.SetBalances(balances))

	return beaconState
}
