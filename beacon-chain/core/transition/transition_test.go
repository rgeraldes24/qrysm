package transition_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/altair"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	state_native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	consensusblocks "github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func init() {
	transition.SkipSlotCache.Disable()
}

func TestExecuteStateTransition_IncorrectSlot(t *testing.T) {
	base := &zondpb.BeaconState{
		Slot: 5,
	}
	beaconState, err := state_native.InitializeFromProtoCapella(base)
	require.NoError(t, err)
	block := &zondpb.SignedBeaconBlock{
		Block: &zondpb.BeaconBlock{
			Slot: 4,
			Body: &zondpb.BeaconBlockBody{},
		},
	}
	want := "expected state.slot"
	wsb, err := consensusblocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	_, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	assert.ErrorContains(t, want, err)
}

func TestExecuteStateTransition_FullProcess(t *testing.T) {
	beaconState, privKeys := util.DeterministicGenesisState(t, 100)

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

	oldMix, err := beaconState.RandaoMixAtIndex(1)
	require.NoError(t, err)

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

	wsb, err := consensusblocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	stateRoot, err := transition.CalculateStateRoot(context.Background(), beaconState, wsb)
	require.NoError(t, err)

	block.Block.StateRoot = stateRoot[:]

	sig, err := util.BlockSignature(beaconState, block.Block, privKeys)
	require.NoError(t, err)
	block.Signature = sig.Marshal()

	wsb, err = consensusblocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	beaconState, err = transition.ExecuteStateTransition(context.Background(), beaconState, wsb)
	require.NoError(t, err)

	assert.Equal(t, params.BeaconConfig().SlotsPerEpoch, beaconState.Slot(), "Unexpected Slot number")

	mix, err := beaconState.RandaoMixAtIndex(1)
	require.NoError(t, err)
	assert.DeepNotEqual(t, oldMix, mix, "Did not expect new and old randao mix to equal")
}

func TestProcessBlock_IncorrectProcessExits(t *testing.T) {
	beaconState, _ := util.DeterministicGenesisState(t, 100)

	proposerSlashings := []*zondpb.ProposerSlashing{
		{
			Header_1: util.HydrateSignedBeaconHeader(&zondpb.SignedBeaconBlockHeader{
				Header: &zondpb.BeaconBlockHeader{
					ProposerIndex: 3,
					Slot:          1,
				},
				Signature: bytesutil.PadTo([]byte("A"), 4595),
			}),
			Header_2: util.HydrateSignedBeaconHeader(&zondpb.SignedBeaconBlockHeader{
				Header: &zondpb.BeaconBlockHeader{
					ProposerIndex: 3,
					Slot:          1,
				},
				Signature: bytesutil.PadTo([]byte("B"), 4595),
			}),
		},
	}
	attesterSlashings := []*zondpb.AttesterSlashing{
		{
			Attestation_1: &zondpb.IndexedAttestation{
				Data:             util.HydrateAttestationData(&zondpb.AttestationData{}),
				AttestingIndices: []uint64{0, 1},
				Signatures:       [][]byte{},
			},
			Attestation_2: &zondpb.IndexedAttestation{
				Data:             util.HydrateAttestationData(&zondpb.AttestationData{}),
				AttestingIndices: []uint64{0, 1},
				Signatures:       [][]byte{},
			},
		},
	}
	var blockRoots [][]byte
	for i := uint64(0); i < uint64(params.BeaconConfig().SlotsPerHistoricalRoot); i++ {
		blockRoots = append(blockRoots, []byte{byte(i)})
	}
	require.NoError(t, beaconState.SetBlockRoots(blockRoots))
	blockAtt := util.HydrateAttestation(&zondpb.Attestation{
		Data: &zondpb.AttestationData{
			Target: &zondpb.Checkpoint{Root: bytesutil.PadTo([]byte("hello-world"), 32)},
		},
		ParticipationBits: bitfield.Bitlist{0xC0, 0xC0, 0xC0, 0xC0, 0x01},
	})
	attestations := []*zondpb.Attestation{blockAtt}
	var exits []*zondpb.SignedVoluntaryExit
	for i := uint64(0); i < params.BeaconConfig().MaxVoluntaryExits+1; i++ {
		exits = append(exits, &zondpb.SignedVoluntaryExit{})
	}
	genesisBlock := blocks.NewGenesisBlock([]byte{})
	bodyRoot, err := genesisBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	err = beaconState.SetLatestBlockHeader(util.HydrateBeaconHeader(&zondpb.BeaconBlockHeader{
		Slot:       genesisBlock.Block.Slot,
		ParentRoot: genesisBlock.Block.ParentRoot,
		BodyRoot:   bodyRoot[:],
	}))
	require.NoError(t, err)
	parentRoot, err := beaconState.LatestBlockHeader().HashTreeRoot()
	require.NoError(t, err)
	block := util.NewBeaconBlock()
	block.Block.Slot = 1
	block.Block.ParentRoot = parentRoot[:]
	block.Block.Body.ProposerSlashings = proposerSlashings
	block.Block.Body.Attestations = attestations
	block.Block.Body.AttesterSlashings = attesterSlashings
	block.Block.Body.VoluntaryExits = exits
	block.Block.Body.Zond1Data.DepositRoot = bytesutil.PadTo([]byte{2}, 32)
	block.Block.Body.Zond1Data.BlockHash = bytesutil.PadTo([]byte{3}, 32)
	err = beaconState.SetSlot(beaconState.Slot() + params.BeaconConfig().MinAttestationInclusionDelay)
	require.NoError(t, err)
	cp := beaconState.CurrentJustifiedCheckpoint()
	cp.Root = []byte("hello-world")
	require.NoError(t, beaconState.SetCurrentJustifiedCheckpoint(cp))
	//require.NoError(t, beaconState.AppendCurrentEpochAttestations(&zondpb.PendingAttestation{}))
	wsb, err := consensusblocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	_, err = transition.VerifyOperationLengths(context.Background(), beaconState, wsb)
	wanted := "number of voluntary exits (17) in block body exceeds allowed threshold of 16"
	assert.ErrorContains(t, wanted, err)
}

func createFullBlockWithOperations(t *testing.T) (state.BeaconState,
	*zondpb.SignedBeaconBlock) {
	beaconState, privKeys := util.DeterministicGenesisState(t, 32)
	sCom, err := altair.NextSyncCommittee(context.Background(), beaconState)
	assert.NoError(t, err)
	assert.NoError(t, beaconState.SetCurrentSyncCommittee(sCom))
	tState := beaconState.Copy()
	blk, err := util.GenerateFullBlock(tState, privKeys,
		&util.BlockGenConfig{NumAttestations: 1, NumVoluntaryExits: 0, NumDeposits: 0}, 1)
	require.NoError(t, err)

	blkCapella := &zondpb.SignedBeaconBlock{
		Block: &zondpb.BeaconBlock{
			Slot:          blk.Block.Slot,
			ProposerIndex: blk.Block.ProposerIndex,
			ParentRoot:    blk.Block.ParentRoot,
			StateRoot:     blk.Block.StateRoot,
			Body: &zondpb.BeaconBlockBody{
				RandaoReveal:      blk.Block.Body.RandaoReveal,
				Zond1Data:         blk.Block.Body.Zond1Data,
				Graffiti:          blk.Block.Body.Graffiti,
				ProposerSlashings: blk.Block.Body.ProposerSlashings,
				AttesterSlashings: blk.Block.Body.AttesterSlashings,
				Attestations:      blk.Block.Body.Attestations,
				Deposits:          blk.Block.Body.Deposits,
				VoluntaryExits:    blk.Block.Body.VoluntaryExits,
				SyncAggregate:     blk.Block.Body.SyncAggregate,
				ExecutionPayload: &enginev1.ExecutionPayload{
					ParentHash:    make([]byte, fieldparams.RootLength),
					FeeRecipient:  make([]byte, fieldparams.FeeRecipientLength),
					StateRoot:     make([]byte, fieldparams.RootLength),
					ReceiptsRoot:  make([]byte, fieldparams.RootLength),
					LogsBloom:     make([]byte, fieldparams.LogsBloomLength),
					PrevRandao:    make([]byte, fieldparams.RootLength),
					BaseFeePerGas: bytesutil.PadTo([]byte{1, 2, 3, 4}, fieldparams.RootLength),
					BlockHash:     make([]byte, fieldparams.RootLength),
					Transactions:  make([][]byte, 0),
					Withdrawals:   make([]*enginev1.Withdrawal, 0),
					ExtraData:     make([]byte, 0),
				},
			},
		},
		Signature: nil,
	}
	beaconStateCapella, _ := util.DeterministicGenesisState(t, 32)
	return beaconStateCapella, blkCapella
}
func TestProcessBlock_OverMaxProposerSlashings(t *testing.T) {
	maxSlashings := params.BeaconConfig().MaxProposerSlashings
	b := &zondpb.SignedBeaconBlock{
		Block: &zondpb.BeaconBlock{
			Body: &zondpb.BeaconBlockBody{
				ProposerSlashings: make([]*zondpb.ProposerSlashing, maxSlashings+1),
			},
		},
	}
	want := fmt.Sprintf("number of proposer slashings (%d) in block body exceeds allowed threshold of %d",
		len(b.Block.Body.ProposerSlashings), params.BeaconConfig().MaxProposerSlashings)
	s, err := state_native.InitializeFromProtoUnsafeCapella(&zondpb.BeaconState{})
	require.NoError(t, err)
	wsb, err := consensusblocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	_, err = transition.VerifyOperationLengths(context.Background(), s, wsb)
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_OverMaxAttesterSlashings(t *testing.T) {
	maxSlashings := params.BeaconConfig().MaxAttesterSlashings
	b := &zondpb.SignedBeaconBlock{
		Block: &zondpb.BeaconBlock{
			Body: &zondpb.BeaconBlockBody{
				AttesterSlashings: make([]*zondpb.AttesterSlashing, maxSlashings+1),
			},
		},
	}
	want := fmt.Sprintf("number of attester slashings (%d) in block body exceeds allowed threshold of %d",
		len(b.Block.Body.AttesterSlashings), params.BeaconConfig().MaxAttesterSlashings)
	s, err := state_native.InitializeFromProtoUnsafeCapella(&zondpb.BeaconState{})
	require.NoError(t, err)
	wsb, err := consensusblocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	_, err = transition.VerifyOperationLengths(context.Background(), s, wsb)
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_OverMaxAttestations(t *testing.T) {
	b := &zondpb.SignedBeaconBlock{
		Block: &zondpb.BeaconBlock{
			Body: &zondpb.BeaconBlockBody{
				Attestations: make([]*zondpb.Attestation, params.BeaconConfig().MaxAttestations+1),
			},
		},
	}
	want := fmt.Sprintf("number of attestations (%d) in block body exceeds allowed threshold of %d",
		len(b.Block.Body.Attestations), params.BeaconConfig().MaxAttestations)
	s, err := state_native.InitializeFromProtoUnsafeCapella(&zondpb.BeaconState{})
	require.NoError(t, err)
	wsb, err := consensusblocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	_, err = transition.VerifyOperationLengths(context.Background(), s, wsb)
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_OverMaxVoluntaryExits(t *testing.T) {
	maxExits := params.BeaconConfig().MaxVoluntaryExits
	b := &zondpb.SignedBeaconBlock{
		Block: &zondpb.BeaconBlock{
			Body: &zondpb.BeaconBlockBody{
				VoluntaryExits: make([]*zondpb.SignedVoluntaryExit, maxExits+1),
			},
		},
	}
	want := fmt.Sprintf("number of voluntary exits (%d) in block body exceeds allowed threshold of %d",
		len(b.Block.Body.VoluntaryExits), maxExits)
	s, err := state_native.InitializeFromProtoUnsafeCapella(&zondpb.BeaconState{})
	require.NoError(t, err)
	wsb, err := consensusblocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	_, err = transition.VerifyOperationLengths(context.Background(), s, wsb)
	assert.ErrorContains(t, want, err)
}

func TestProcessBlock_IncorrectDeposits(t *testing.T) {
	base := &zondpb.BeaconState{
		Zond1Data:         &zondpb.Zond1Data{DepositCount: 100},
		Zond1DepositIndex: 98,
	}
	s, err := state_native.InitializeFromProtoCapella(base)
	require.NoError(t, err)
	b := &zondpb.SignedBeaconBlock{
		Block: &zondpb.BeaconBlock{
			Body: &zondpb.BeaconBlockBody{
				Deposits: []*zondpb.Deposit{{}},
			},
		},
	}
	want := fmt.Sprintf("incorrect outstanding deposits in block body, wanted: %d, got: %d",
		s.Zond1Data().DepositCount-s.Zond1DepositIndex(), len(b.Block.Body.Deposits))
	wsb, err := consensusblocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	_, err = transition.VerifyOperationLengths(context.Background(), s, wsb)
	assert.ErrorContains(t, want, err)
}

func TestProcessSlots_SameSlotAsParentState(t *testing.T) {
	slot := primitives.Slot(2)
	parentState, err := state_native.InitializeFromProtoCapella(&zondpb.BeaconState{Slot: slot})
	require.NoError(t, err)

	_, err = transition.ProcessSlots(context.Background(), parentState, slot)
	assert.ErrorContains(t, "expected state.slot 2 < slot 2", err)
}

func TestProcessSlots_LowerSlotAsParentState(t *testing.T) {
	slot := primitives.Slot(2)
	parentState, err := state_native.InitializeFromProtoCapella(&zondpb.BeaconState{Slot: slot})
	require.NoError(t, err)

	_, err = transition.ProcessSlots(context.Background(), parentState, slot-1)
	assert.ErrorContains(t, "expected state.slot 2 < slot 1", err)
}

func TestProcessSlots_OnlyCapellaEpoch(t *testing.T) {
	transition.SkipSlotCache.Disable()
	params.SetupTestConfigCleanup(t)

	st, _ := util.DeterministicGenesisState(t, params.BeaconConfig().MaxValidatorsPerCommittee)
	require.NoError(t, st.SetSlot(params.BeaconConfig().SlotsPerEpoch*6))
	require.Equal(t, version.Capella, st.Version())
	st, err := transition.ProcessSlots(context.Background(), st, params.BeaconConfig().SlotsPerEpoch*10)
	require.NoError(t, err)
	require.Equal(t, version.Capella, st.Version())

	require.Equal(t, params.BeaconConfig().SlotsPerEpoch*10, st.Slot())

	s, err := st.InactivityScores()
	require.NoError(t, err)
	require.Equal(t, params.BeaconConfig().MaxValidatorsPerCommittee, uint64(len(s)))

	p, err := st.PreviousEpochParticipation()
	require.NoError(t, err)
	require.Equal(t, params.BeaconConfig().MaxValidatorsPerCommittee, uint64(len(p)))

	p, err = st.CurrentEpochParticipation()
	require.NoError(t, err)
	require.Equal(t, params.BeaconConfig().MaxValidatorsPerCommittee, uint64(len(p)))

	sc, err := st.CurrentSyncCommittee()
	require.NoError(t, err)
	require.Equal(t, params.BeaconConfig().SyncCommitteeSize, uint64(len(sc.Pubkeys)))

	sc, err = st.NextSyncCommittee()
	require.NoError(t, err)
	require.Equal(t, params.BeaconConfig().SyncCommitteeSize, uint64(len(sc.Pubkeys)))
}

func TestProcessSlots_ThroughCapellaEpoch(t *testing.T) {
	transition.SkipSlotCache.Disable()
	params.SetupTestConfigCleanup(t)

	st, _ := util.DeterministicGenesisState(t, params.BeaconConfig().MaxValidatorsPerCommittee)
	st, err := transition.ProcessSlots(context.Background(), st, params.BeaconConfig().SlotsPerEpoch*10)
	require.NoError(t, err)
	require.Equal(t, version.Capella, st.Version())

	require.Equal(t, params.BeaconConfig().SlotsPerEpoch*10, st.Slot())
}

func TestProcessSlotsUsingNextSlotCache(t *testing.T) {
	s, _ := util.DeterministicGenesisState(t, 1)
	r := []byte{'a'}
	s, err := transition.ProcessSlotsUsingNextSlotCache(context.Background(), s, r, 5)
	require.NoError(t, err)
	require.Equal(t, primitives.Slot(5), s.Slot())
}

func TestProcessSlotsConditionally(t *testing.T) {
	ctx := context.Background()
	s, _ := util.DeterministicGenesisState(t, 1)

	t.Run("target slot below current slot", func(t *testing.T) {
		require.NoError(t, s.SetSlot(5))
		s, err := transition.ProcessSlotsIfPossible(ctx, s, 4)
		require.NoError(t, err)
		assert.Equal(t, primitives.Slot(5), s.Slot())
	})

	t.Run("target slot equal current slot", func(t *testing.T) {
		require.NoError(t, s.SetSlot(5))
		s, err := transition.ProcessSlotsIfPossible(ctx, s, 5)
		require.NoError(t, err)
		assert.Equal(t, primitives.Slot(5), s.Slot())
	})

	t.Run("target slot above current slot", func(t *testing.T) {
		require.NoError(t, s.SetSlot(5))
		s, err := transition.ProcessSlotsIfPossible(ctx, s, 6)
		require.NoError(t, err)
		assert.Equal(t, primitives.Slot(6), s.Slot())
	})
}
