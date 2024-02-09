package util

import (
	"context"
	"testing"

	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func TestDeterministicGenesisState_100Validators(t *testing.T) {
	validatorCount := uint64(100)
	beaconState, privKeys := DeterministicGenesisStateCapella(t, validatorCount)
	activeValidators, err := helpers.ActiveValidatorCount(context.Background(), beaconState, 0)
	require.NoError(t, err)

	// lint:ignore uintcast -- test code
	if len(privKeys) != int(validatorCount) {
		t.Fatalf("expected amount of private keys %d to match requested amount of validators %d", len(privKeys), validatorCount)
	}
	if activeValidators != validatorCount {
		t.Fatalf("expected validators in state %d to match requested amount %d", activeValidators, validatorCount)
	}
}
