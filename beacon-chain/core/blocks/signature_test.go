package blocks_test

import (
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/config/params"
	consensusblocks "github.com/theQRL/qrysm/consensus-types/blocks"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
)

func TestVerifyBlockSignatureUsingCurrentFork(t *testing.T) {
	params.SetupTestConfigCleanup(t)

	bState, keys := util.DeterministicGenesisStateZond(t, 100)
	blk := util.NewBeaconBlockZond()
	blk.Block.ProposerIndex = 0
	blk.Block.Slot = params.BeaconConfig().SlotsPerEpoch * 100
	fData := &qrysmpb.Fork{
		Epoch:           100,
		CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
		PreviousVersion: params.BeaconConfig().GenesisForkVersion,
	}
	domain, err := signing.Domain(fData, 100, params.BeaconConfig().DomainBeaconProposer, bState.GenesisValidatorsRoot())
	assert.NoError(t, err)
	blkRoot, err := blk.Block.HashTreeRoot()
	assert.NoError(t, err)
	rt, err := signing.ComputeSigningRoot(blk.Block, domain)
	assert.NoError(t, err)
	lsig2, err := keys[0].Sign(rt[:])
	require.NoError(t, err)
	sig := lsig2.Marshal()
	blk.Signature = sig
	wsb, err := consensusblocks.NewSignedBeaconBlock(blk)
	require.NoError(t, err)
	assert.NoError(t, blocks.VerifyBlockSignatureUsingCurrentFork(bState, wsb, blkRoot))
}

func TestVerifyBlockSignatureUsingCurrentFork_InvalidSignature(t *testing.T) {
	params.SetupTestConfigCleanup(t)

	bState, keys := util.DeterministicGenesisStateZond(t, 100)
	blk := util.NewBeaconBlockZond()
	blk.Block.ProposerIndex = 0
	blk.Block.Slot = params.BeaconConfig().SlotsPerEpoch * 100
	fData := &qrysmpb.Fork{
		Epoch:           100,
		CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
		PreviousVersion: params.BeaconConfig().GenesisForkVersion,
	}
	domain, err := signing.Domain(fData, 100, params.BeaconConfig().DomainBeaconProposer, bState.GenesisValidatorsRoot())
	assert.NoError(t, err)
	rt, err := signing.ComputeSigningRoot(blk.Block, domain)
	assert.NoError(t, err)

	// Sign with the wrong key (proposer index is 0, but using key 1).
	lsig3, err := keys[1].Sign(rt[:])
	require.NoError(t, err)
	blk.Signature = lsig3.Marshal()
	wsb, err := consensusblocks.NewSignedBeaconBlock(blk)
	require.NoError(t, err)
	blkRoot, err := blk.Block.HashTreeRoot()
	require.NoError(t, err)

	err = blocks.VerifyBlockSignatureUsingCurrentFork(bState, wsb, blkRoot)
	require.ErrorIs(t, err, blocks.ErrInvalidSignature, "Expected ErrInvalidSignature for invalid signature")
}

func TestVerifyBlockSignatureUsingCurrentFork_OutOfRangeProposerIndex(t *testing.T) {
	params.SetupTestConfigCleanup(t)

	bState, _ := util.DeterministicGenesisStateZond(t, 100)
	blk := util.NewBeaconBlockZond()
	blk.Block.ProposerIndex = 100 // first index outside the 100-validator registry
	blk.Block.Slot = params.BeaconConfig().SlotsPerEpoch * 100
	wsb, err := consensusblocks.NewSignedBeaconBlock(blk)
	require.NoError(t, err)
	blkRoot, err := blk.Block.HashTreeRoot()
	require.NoError(t, err)

	err = blocks.VerifyBlockSignatureUsingCurrentFork(bState, wsb, blkRoot)
	require.ErrorIs(t, err, blocks.ErrInvalidProposerIndex, "Expected ErrInvalidProposerIndex for out-of-range proposer index")
}
