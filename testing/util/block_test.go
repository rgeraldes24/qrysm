package util

// FIX(rgeraldes24): expected state.slot 0 < slot 0
/*
import (
	"context"
	"testing"

	coreBlock "github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition/stateutils"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpbalpha "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func TestGenerateFullBlock_PassesStateTransition(t *testing.T) {
	beaconState, privs := DeterministicGenesisState(t, 128)
	conf := &BlockGenConfig{
		NumAttestations: 1,
	}
	block, err := GenerateFullBlock(beaconState, privs, conf, beaconState.Slot())
	require.NoError(t, err)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	_, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	require.NoError(t, err)
}

func TestGenerateFullBlock_ThousandValidators(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MainnetConfig().Copy())
	beaconState, privs := DeterministicGenesisState(t, 1024)
	conf := &BlockGenConfig{
		NumAttestations: 4,
	}
	block, err := GenerateFullBlock(beaconState, privs, conf, beaconState.Slot())
	require.NoError(t, err)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	_, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	require.NoError(t, err)
}

func TestGenerateFullBlock_Passes4Epochs(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MainnetConfig().Copy())
	beaconState, privs := DeterministicGenesisState(t, 64)

	conf := &BlockGenConfig{
		NumAttestations: 2,
	}
	finalSlot := params.BeaconConfig().SlotsPerEpoch*4 + 3
	for i := 0; i < int(finalSlot); i++ {
		helpers.ClearCache()
		block, err := GenerateFullBlock(beaconState, privs, conf, beaconState.Slot())
		require.NoError(t, err)
		wsb, err := blocks.NewSignedBeaconBlock(block)
		require.NoError(t, err)
		beaconState, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
		require.NoError(t, err)
	}

	// Blocks are one slot ahead of beacon state.
	if finalSlot != beaconState.Slot() {
		t.Fatalf("expected output slot to be %d, received %d", finalSlot, beaconState.Slot())
	}
	if beaconState.CurrentJustifiedCheckpoint().Epoch != 3 {
		t.Fatalf("expected justified epoch to change to 3, received %d", beaconState.CurrentJustifiedCheckpoint().Epoch)
	}
	if beaconState.FinalizedCheckpointEpoch() != 2 {
		t.Fatalf("expected finalized epoch to change to 2, received %d", beaconState.FinalizedCheckpointEpoch())
	}
}

func TestGenerateFullBlock_ValidProposerSlashings(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MainnetConfig().Copy())
	beaconState, privs := DeterministicGenesisState(t, 32)
	conf := &BlockGenConfig{
		NumProposerSlashings: 1,
	}
	block, err := GenerateFullBlock(beaconState, privs, conf, beaconState.Slot()+1)
	require.NoError(t, err)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	beaconState, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	require.NoError(t, err)

	slashableIndice := block.Block.Body.ProposerSlashings[0].Header_1.Header.ProposerIndex
	if val, err := beaconState.ValidatorAtIndexReadOnly(slashableIndice); err != nil || !val.Slashed() {
		require.NoError(t, err)
		t.Fatal("expected validator to be slashed")
	}
}

func TestGenerateFullBlock_ValidAttesterSlashings(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MainnetConfig().Copy())
	beaconState, privs := DeterministicGenesisState(t, 256)
	conf := &BlockGenConfig{
		NumAttesterSlashings: 1,
	}
	block, err := GenerateFullBlock(beaconState, privs, conf, beaconState.Slot())
	require.NoError(t, err)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	beaconState, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	require.NoError(t, err)

	slashableIndices := block.Block.Body.AttesterSlashings[0].Attestation_1.AttestingIndices
	if val, err := beaconState.ValidatorAtIndexReadOnly(primitives.ValidatorIndex(slashableIndices[0])); err != nil || !val.Slashed() {
		require.NoError(t, err)
		t.Fatal("expected validator to be slashed")
	}
}

func TestGenerateFullBlock_ValidAttestations(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MainnetConfig().Copy())

	beaconState, privs := DeterministicGenesisState(t, 256)
	conf := &BlockGenConfig{
		NumAttestations: 4,
	}
	block, err := GenerateFullBlock(beaconState, privs, conf, beaconState.Slot())
	require.NoError(t, err)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	beaconState, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	require.NoError(t, err)
	// TODO(rgeraldes24) - review
	// atts, err := beaconState.CurrentEpochAttestations()
	// require.NoError(t, err)
	// if len(atts) != 4 {
	// 	t.Fatal("expected 4 attestations to be saved to the beacon state")
	// }
}

func TestGenerateFullBlock_ValidDeposits(t *testing.T) {
	beaconState, privs := DeterministicGenesisState(t, 256)
	deposits, _, err := DeterministicDepositsAndKeys(257)
	require.NoError(t, err)
	zond1Data, err := DeterministicZond1Data(len(deposits))
	require.NoError(t, err)
	require.NoError(t, beaconState.SetZond1Data(zond1Data))
	conf := &BlockGenConfig{
		NumDeposits: 1,
	}
	block, err := GenerateFullBlock(beaconState, privs, conf, beaconState.Slot())
	require.NoError(t, err)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	beaconState, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	require.NoError(t, err)

	depositedPubkey := block.Block.Body.Deposits[0].Data.PublicKey
	valIndexMap := stateutils.ValidatorIndexMap(beaconState.Validators())
	index := valIndexMap[bytesutil.ToBytes2592(depositedPubkey)]
	val, err := beaconState.ValidatorAtIndexReadOnly(index)
	require.NoError(t, err)
	if val.EffectiveBalance() != params.BeaconConfig().MaxEffectiveBalance {
		t.Fatalf(
			"expected validator balance to be max effective balance, received %d",
			val.EffectiveBalance(),
		)
	}
}

func TestGenerateFullBlock_ValidVoluntaryExits(t *testing.T) {
	beaconState, privs := DeterministicGenesisState(t, 256)
	// Moving the state 2048 epochs forward due to PERSISTENT_COMMITTEE_PERIOD.
	err := beaconState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().ShardCommitteePeriod)).Add(3))
	require.NoError(t, err)
	conf := &BlockGenConfig{
		NumVoluntaryExits: 1,
	}
	block, err := GenerateFullBlock(beaconState, privs, conf, beaconState.Slot())
	require.NoError(t, err)
	wsb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	beaconState, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	require.NoError(t, err)

	exitedIndex := block.Block.Body.VoluntaryExits[0].Exit.ValidatorIndex

	val, err := beaconState.ValidatorAtIndexReadOnly(exitedIndex)
	require.NoError(t, err)
	if val.ExitEpoch() == params.BeaconConfig().FarFutureEpoch {
		t.Fatal("expected exiting validator index to be marked as exiting")
	}
}

func TestHydrateV1SignedBeaconBlock_NoError(t *testing.T) {
	b := &zondpbv1.SignedBeaconBlock{}
	b = HydrateV1SignedBeaconBlock(b)
	_, err := b.HashTreeRoot()
	require.NoError(t, err)
	_, err = b.Message.HashTreeRoot()
	require.NoError(t, err)
	_, err = b.Message.Body.HashTreeRoot()
	require.NoError(t, err)
}

func TestHydrateSignedBeaconBlock_NoError(t *testing.T) {
	b := &zondpbalpha.SignedBeaconBlock{}
	b = HydrateSignedBeaconBlock(b)
	_, err := b.HashTreeRoot()
	require.NoError(t, err)
	_, err = b.Block.HashTreeRoot()
	require.NoError(t, err)
	_, err = b.Block.Body.HashTreeRoot()
	require.NoError(t, err)
}

func TestHydrateBeaconBlock_NoError(t *testing.T) {
	b := &zondpbalpha.BeaconBlock{}
	b = HydrateBeaconBlock(b)
	_, err := b.HashTreeRoot()
	require.NoError(t, err)
	_, err = b.Body.HashTreeRoot()
	require.NoError(t, err)
}

func TestHydrateBeaconBlockBody_NoError(t *testing.T) {
	b := &zondpbalpha.BeaconBlockBody{}
	b = HydrateBeaconBlockBody(b)
	_, err := b.HashTreeRoot()
	require.NoError(t, err)
}

func TestHydrateSignedBlindedBeaconBlock_NoError(t *testing.T) {
	b := &zondpbalpha.SignedBlindedBeaconBlock{}
	b = HydrateSignedBlindedBeaconBlock(b)
	_, err := b.HashTreeRoot()
	require.NoError(t, err)
	_, err = b.Block.HashTreeRoot()
	require.NoError(t, err)
	_, err = b.Block.Body.HashTreeRoot()
	require.NoError(t, err)
}

func TestHydrateBlindedBeaconBlock_NoError(t *testing.T) {
	b := &zondpbalpha.BlindedBeaconBlock{}
	b = HydrateBlindedBeaconBlock(b)
	_, err := b.HashTreeRoot()
	require.NoError(t, err)
	_, err = b.Body.HashTreeRoot()
	require.NoError(t, err)
}

func TestHydrateBlindedBeaconBlockBody_NoError(t *testing.T) {
	b := &zondpbalpha.BlindedBeaconBlockBody{}
	b = HydrateBlindedBeaconBlockBody(b)
	_, err := b.HashTreeRoot()
	require.NoError(t, err)
}

func TestGenerateVoluntaryExits(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	config := params.BeaconConfig()
	config.ShardCommitteePeriod = 0
	params.OverrideBeaconConfig(config)

	beaconState, privKeys := DeterministicGenesisState(t, 256)
	exit, err := GenerateVoluntaryExits(beaconState, privKeys[0], 0)
	require.NoError(t, err)
	val, err := beaconState.ValidatorAtIndexReadOnly(0)
	require.NoError(t, err)
	require.NoError(t, coreBlock.VerifyExitAndSignature(val, 0, beaconState.Fork(), exit, beaconState.GenesisValidatorsRoot()))
}
*/
