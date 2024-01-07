package transition_test

import (
	"context"
	"math"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/altair"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	p2pType "github.com/theQRL/qrysm/v4/beacon-chain/p2p/types"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestExecuteStateTransitionNoVerify_FullProcess(t *testing.T) {
	beaconState, privKeys := util.DeterministicGenesisState(t, 100)

	syncCommittee, err := altair.NextSyncCommittee(context.Background(), beaconState)
	require.NoError(t, err)
	require.NoError(t, beaconState.SetCurrentSyncCommittee(syncCommittee))

	zond1Data := &zondpb.Zond1Data{
		DepositCount: 100,
		DepositRoot:  bytesutil.PadTo([]byte{2}, 32),
		BlockHash:    make([]byte, 32),
	}
	require.NoError(t, beaconState.SetSlot(params.BeaconConfig().SlotsPerEpoch-1))
	e := beaconState.Zond1Data()
	e.DepositCount = 100
	require.NoError(t, beaconState.SetZond1Data(e))
	bh := beaconState.LatestBlockHeader()
	bh.Slot = beaconState.Slot()
	require.NoError(t, beaconState.SetLatestBlockHeader(bh))
	require.NoError(t, beaconState.SetZond1DataVotes([]*zondpb.Zond1Data{zond1Data}))

	require.NoError(t, beaconState.SetSlot(beaconState.Slot()+1))
	epoch := time.CurrentEpoch(beaconState)
	randaoReveal, err := util.RandaoReveal(beaconState, epoch, privKeys)
	require.NoError(t, err)
	require.NoError(t, beaconState.SetSlot(beaconState.Slot()-1))

	nextSlotState, err := transition.ProcessSlots(context.Background(), beaconState.Copy(), beaconState.Slot()+1)
	require.NoError(t, err)
	parentRoot, err := nextSlotState.LatestBlockHeader().HashTreeRoot()
	require.NoError(t, err)
	proposerIdx, err := helpers.BeaconProposerIndex(context.Background(), nextSlotState)
	require.NoError(t, err)
	block := util.NewBeaconBlock()
	block.Block.ProposerIndex = proposerIdx
	block.Block.Slot = beaconState.Slot() + 1
	block.Block.ParentRoot = parentRoot[:]
	block.Block.Body.RandaoReveal = randaoReveal
	block.Block.Body.Zond1Data = zond1Data

	syncBits := bitfield.NewBitvector512()
	for i := range syncBits {
		syncBits[i] = 0xff
	}
	indices, err := altair.NextSyncCommitteeIndices(context.Background(), beaconState)
	require.NoError(t, err)
	h := zondpb.CopyBeaconBlockHeader(beaconState.LatestBlockHeader())
	prevStateRoot, err := beaconState.HashTreeRoot(context.Background())
	require.NoError(t, err)
	h.StateRoot = prevStateRoot[:]
	pbr, err := h.HashTreeRoot()
	require.NoError(t, err)
	syncSigs := make([][]byte, len(indices))
	for i, indice := range indices {
		b := p2pType.SSZBytes(pbr[:])
		sb, err := signing.ComputeDomainAndSign(beaconState, time.CurrentEpoch(beaconState), &b, params.BeaconConfig().DomainSyncCommittee, privKeys[indice])
		require.NoError(t, err)
		syncSigs[i] = sb
	}
	syncAggregate := &zondpb.SyncAggregate{
		SyncCommitteeBits:       syncBits,
		SyncCommitteeSignatures: syncSigs,
	}
	block.Block.Body.SyncAggregate = syncAggregate
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	stateRoot, err := transition.CalculateStateRoot(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	block.Block.StateRoot = stateRoot[:]

	c := beaconState.Copy()
	sig, err := util.BlockSignature(c, block.Block, privKeys)
	require.NoError(t, err)
	block.Signature = sig.Marshal()

	wsb, err = blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	set, _, err := transition.ExecuteStateTransitionNoVerifyAnySig(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	verified, err := set.Verify()
	require.NoError(t, err)
	require.Equal(t, true, verified, "Could not verify signature set")
}

func TestExecuteStateTransitionNoVerifySignature_CouldNotVerifyStateRoot(t *testing.T) {
	beaconState, privKeys := util.DeterministicGenesisState(t, 100)

	syncCommittee, err := altair.NextSyncCommittee(context.Background(), beaconState)
	require.NoError(t, err)
	require.NoError(t, beaconState.SetCurrentSyncCommittee(syncCommittee))

	zond1Data := &zondpb.Zond1Data{
		DepositCount: 100,
		DepositRoot:  bytesutil.PadTo([]byte{2}, 32),
		BlockHash:    make([]byte, 32),
	}
	require.NoError(t, beaconState.SetSlot(params.BeaconConfig().SlotsPerEpoch-1))
	e := beaconState.Zond1Data()
	e.DepositCount = 100
	require.NoError(t, beaconState.SetZond1Data(e))
	bh := beaconState.LatestBlockHeader()
	bh.Slot = beaconState.Slot()
	require.NoError(t, beaconState.SetLatestBlockHeader(bh))
	require.NoError(t, beaconState.SetZond1DataVotes([]*zondpb.Zond1Data{zond1Data}))

	require.NoError(t, beaconState.SetSlot(beaconState.Slot()+1))
	epoch := time.CurrentEpoch(beaconState)
	randaoReveal, err := util.RandaoReveal(beaconState, epoch, privKeys)
	require.NoError(t, err)
	require.NoError(t, beaconState.SetSlot(beaconState.Slot()-1))

	nextSlotState, err := transition.ProcessSlots(context.Background(), beaconState.Copy(), beaconState.Slot()+1)
	require.NoError(t, err)
	parentRoot, err := nextSlotState.LatestBlockHeader().HashTreeRoot()
	require.NoError(t, err)
	proposerIdx, err := helpers.BeaconProposerIndex(context.Background(), nextSlotState)
	require.NoError(t, err)
	block := util.NewBeaconBlock()
	block.Block.ProposerIndex = proposerIdx
	block.Block.Slot = beaconState.Slot() + 1
	block.Block.ParentRoot = parentRoot[:]
	block.Block.Body.RandaoReveal = randaoReveal
	block.Block.Body.Zond1Data = zond1Data

	syncBits := bitfield.NewBitvector512()
	for i := range syncBits {
		syncBits[i] = 0xff
	}
	indices, err := altair.NextSyncCommitteeIndices(context.Background(), beaconState)
	require.NoError(t, err)
	h := zondpb.CopyBeaconBlockHeader(beaconState.LatestBlockHeader())
	prevStateRoot, err := beaconState.HashTreeRoot(context.Background())
	require.NoError(t, err)
	h.StateRoot = prevStateRoot[:]
	pbr, err := h.HashTreeRoot()
	require.NoError(t, err)
	syncSigs := make([][]byte, len(indices))
	for i, indice := range indices {
		b := p2pType.SSZBytes(pbr[:])
		sb, err := signing.ComputeDomainAndSign(beaconState, time.CurrentEpoch(beaconState), &b, params.BeaconConfig().DomainSyncCommittee, privKeys[indice])
		require.NoError(t, err)
		syncSigs[i] = sb
	}
	syncAggregate := &zondpb.SyncAggregate{
		SyncCommitteeBits:       syncBits,
		SyncCommitteeSignatures: syncSigs,
	}
	block.Block.Body.SyncAggregate = syncAggregate

	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	stateRoot, err := transition.CalculateStateRoot(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	block.Block.StateRoot = stateRoot[:]

	c := beaconState.Copy()
	sig, err := util.BlockSignature(c, block.Block, privKeys)
	require.NoError(t, err)
	block.Signature = sig.Marshal()

	block.Block.StateRoot = bytesutil.PadTo([]byte{'a'}, 32)
	wsb, err = blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	_, _, err = transition.ExecuteStateTransitionNoVerifyAnySig(context.Background(), beaconState, wsb)
	require.ErrorContains(t, "could not validate state root", err)
}

// FIX(rgeraldes24)
/*
func TestExecuteStateTransitionNoVerifyAnySig_PassesProcessingConditions(t *testing.T) {
	beaconState, block := createFullBlockWithOperations(t)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	set, _, err := transition.ExecuteStateTransitionNoVerifyAnySig(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	// Test Signature set verifies.
	verified, err := set.Verify()
	require.NoError(t, err)
	require.Equal(t, true, verified, "Could not verify signature set")
}
*/

// FIX(rgeraldes24)
/*
func TestProcessBlockNoVerify_PassesProcessingConditions(t *testing.T) {
	beaconState, block := createFullBlockWithOperations(t)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	set, _, err := transition.ProcessBlockNoVerifyAnySig(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	// Test Signature set verifies.
	verified, err := set.Verify()
	require.NoError(t, err)
	assert.Equal(t, true, verified, "Could not verify signature set.")
}
*/

// FIX(rgeraldes24)
/*
func TestProcessBlockNoVerifyAnySigCapella_OK(t *testing.T) {
	beaconState, block := createFullBlockWithOperations(t)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	beaconState, err = transition.ProcessSlots(context.Background(), beaconState, wsb.Block().Slot())
	require.NoError(t, err)
	set, _, err := transition.ProcessBlockNoVerifyAnySig(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	verified, err := set.Verify()
	require.NoError(t, err)
	require.Equal(t, true, verified, "Could not verify signature set")
}
*/

// FIX(rgeraldes24)
/*
func TestProcessBlockNoVerify_SigSetContainsDescriptions(t *testing.T) {
	beaconState, block := createFullBlockWithOperations(t)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	set, _, err := transition.ProcessBlockNoVerifyAnySig(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	assert.Equal(t, len(set.Signatures), len(set.Descriptions), "Signatures and descriptions do not match up")
	assert.Equal(t, "block signature", set.Descriptions[0])
	assert.Equal(t, "randao signature", set.Descriptions[1])
	assert.Equal(t, "attestation signature", set.Descriptions[2])
}
*/

func TestProcessOperationsNoVerifyAttsSigsCapella_OK(t *testing.T) {
	beaconState, block := createFullBlockWithOperations(t)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	beaconState, err = transition.ProcessSlots(context.Background(), beaconState, wsb.Block().Slot())
	require.NoError(t, err)
	_, err = transition.ProcessOperationsNoVerifyAttsSigs(context.Background(), beaconState, wsb)
	require.NoError(t, err)
}

// FIX
/*
func TestCalculateStateRootAltair_OK(t *testing.T) {
	beaconState, block := createFullBlockWithOperations(t)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	r, err := transition.CalculateStateRoot(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	require.DeepNotEqual(t, params.BeaconConfig().ZeroHash, r)
}
*/

// not valid for now since we dont have different version
/*
func TestProcessBlockDifferentVersion(t *testing.T) {
	beaconState, _ := util.DeterministicGenesisState(t, 64) // Phase 0 state
	_, block := createFullBlockWithOperations(t)
	wsb, err := blocks.NewSignedBeaconBlock(block) // Altair block
	require.NoError(t, err)
	_, _, err = transition.ProcessBlockNoVerifyAnySig(context.Background(), beaconState, wsb)
	require.ErrorContains(t, "state and block are different version. 0 != 1", err)
}
*/

func TestProcessEpoch_BadBalance(t *testing.T) {
	s, _ := util.DeterministicGenesisState(t, 100)
	assert.NoError(t, s.SetSlot(63))
	assert.NoError(t, s.UpdateBalancesAtIndex(0, math.MaxUint64))
	participation := byte(0)
	participation, err := altair.AddValidatorFlag(participation, params.BeaconConfig().TimelyHeadFlagIndex)
	require.NoError(t, err)
	participation, err = altair.AddValidatorFlag(participation, params.BeaconConfig().TimelySourceFlagIndex)
	require.NoError(t, err)
	participation, err = altair.AddValidatorFlag(participation, params.BeaconConfig().TimelyTargetFlagIndex)
	require.NoError(t, err)

	epochParticipation, err := s.CurrentEpochParticipation()
	assert.NoError(t, err)
	epochParticipation[0] = participation
	assert.NoError(t, s.SetCurrentParticipationBits(epochParticipation))
	assert.NoError(t, s.SetPreviousParticipationBits(epochParticipation))
	_, err = altair.ProcessEpoch(context.Background(), s)
	assert.ErrorContains(t, "addition overflows", err)
}
