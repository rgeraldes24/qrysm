package util

import (
	"context"
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/beacon-chain/core/time"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/time/slots"
)

func TestBlockSignature(t *testing.T) {
	beaconState, privKeys := DeterministicGenesisStateZond(t, 100)
	block, err := GenerateFullBlockZond(beaconState, privKeys, nil, 0)
	require.NoError(t, err)

	require.NoError(t, beaconState.SetSlot(beaconState.Slot()+1))
	proposerIdx, err := helpers.BeaconProposerIndex(context.Background(), beaconState)
	assert.NoError(t, err)

	assert.NoError(t, beaconState.SetSlot(beaconState.Slot()-1))
	epoch := slots.ToEpoch(block.Block.Slot)
	signature, err := BlockSignature(beaconState, block.Block, privKeys)
	assert.NoError(t, err)
	require.NoError(t, signing.ComputeDomainVerifySigningRoot(
		beaconState,
		proposerIdx,
		epoch,
		block.Block,
		params.BeaconConfig().DomainBeaconProposer,
		signature.Marshal(),
	))
}

func TestRandaoReveal(t *testing.T) {
	beaconState, privKeys := DeterministicGenesisStateZond(t, 100)

	epoch := time.CurrentEpoch(beaconState)
	randaoReveal, err := RandaoReveal(beaconState, epoch, privKeys)
	assert.NoError(t, err)

	proposerIdx, err := helpers.BeaconProposerIndex(context.Background(), beaconState)
	assert.NoError(t, err)
	sszUint := primitives.SSZUint64(epoch)
	require.NoError(t, signing.ComputeDomainVerifySigningRoot(
		beaconState,
		proposerIdx,
		epoch,
		&sszUint,
		params.BeaconConfig().DomainRandao,
		randaoReveal,
	))
}
