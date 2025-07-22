package util

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/core/altair"
	"github.com/theQRL/qrysm/beacon-chain/core/blocks"
	b "github.com/theQRL/qrysm/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/beacon-chain/db/iface"
	"github.com/theQRL/qrysm/beacon-chain/state"
	state_native "github.com/theQRL/qrysm/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/beacon-chain/state/stateutil"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/crypto/dilithium"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
)

// DeterministicGenesisStateCapellaWithGenesisBlock creates a genesis state, saves the genesis block,
// genesis state and head block root. It returns the genesis state, genesis block's root and
// validator private keys.
func DeterministicGenesisStateCapellaWithGenesisBlock(
	t *testing.T,
	ctx context.Context,
	db iface.HeadAccessDatabase,
	numValidators uint64,
) (state.BeaconState, [32]byte, []dilithium.DilithiumKey) {
	genesisState, privateKeys := DeterministicGenesisStateCapella(t, numValidators)
	stateRoot, err := genesisState.HashTreeRoot(ctx)
	require.NoError(t, err, "Could not hash genesis state")

	genesis := b.NewGenesisBlock(stateRoot[:])
	SaveBlock(t, ctx, db, genesis)

	parentRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err, "Could not get signing root")
	require.NoError(t, db.SaveState(ctx, genesisState, parentRoot), "Could not save genesis state")
	require.NoError(t, db.SaveHeadBlockRoot(ctx, parentRoot), "Could not save genesis state")

	return genesisState, parentRoot, privateKeys
}

// DeterministicGenesisStateCapella returns a genesis state in Capella format made using the deterministic deposits.
func DeterministicGenesisStateCapella(t testing.TB, numValidators uint64) (state.BeaconState, []dilithium.DilithiumKey) {
	deposits, privKeys, err := DeterministicDepositsAndKeys(numValidators)
	if err != nil {
		t.Fatal(errors.Wrapf(err, "failed to get %d deposits", numValidators))
	}
	executionNodeData, err := DeterministicExecutionNodeData(len(deposits))
	if err != nil {
		t.Fatal(errors.Wrapf(err, "failed to get executionNodeData for %d deposits", numValidators))
	}
	beaconState, err := GenesisBeaconStateCapella(context.Background(), deposits, uint64(0), executionNodeData)
	if err != nil {
		t.Fatal(errors.Wrapf(err, "failed to get genesis beacon state of %d validators", numValidators))
	}
	resetCache()
	return beaconState, privKeys
}

// GenesisBeaconStateCapella returns the genesis beacon state.
func GenesisBeaconStateCapella(ctx context.Context, deposits []*qrysmpb.Deposit, genesisTime uint64, executionNodeData *qrysmpb.ExecutionNodeData) (state.BeaconState, error) {
	st, err := emptyGenesisStateCapella()
	if err != nil {
		return nil, err
	}

	// Process initial deposits.
	st, err = helpers.UpdateGenesisExecutionNodeData(st, deposits, executionNodeData)
	if err != nil {
		return nil, err
	}

	st, err = processPreGenesisDeposits(ctx, st, deposits)
	if err != nil {
		return nil, errors.Wrap(err, "could not process validator deposits")
	}

	return buildGenesisBeaconStateCapella(genesisTime, st, st.ExecutionNodeData())
}

// emptyGenesisStateCapella returns an empty genesis state in Capella format.
func emptyGenesisStateCapella() (state.BeaconState, error) {
	st := &qrysmpb.BeaconStateCapella{
		// Misc fields.
		Slot: 0,
		Fork: &qrysmpb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},
		// Validator registry fields.
		Validators:       []*qrysmpb.Validator{},
		Balances:         []uint64{},
		InactivityScores: []uint64{},

		JustificationBits:          []byte{0},
		HistoricalRoots:            [][]byte{},
		CurrentEpochParticipation:  []byte{},
		PreviousEpochParticipation: []byte{},

		// Eth1 data.
		ExecutionNodeData:      &qrysmpb.ExecutionNodeData{},
		ExecutionNodeDataVotes: []*qrysmpb.ExecutionNodeData{},
		Eth1DepositIndex:       0,

		LatestExecutionPayloadHeader: &enginev1.ExecutionPayloadHeaderCapella{},
	}
	return state_native.InitializeFromProtoCapella(st)
}

func buildGenesisBeaconStateCapella(genesisTime uint64, preState state.BeaconState, executionNodeData *qrysmpb.ExecutionNodeData) (state.BeaconState, error) {
	if executionNodeData == nil {
		return nil, errors.New("no executionNodeData provided for genesis state")
	}

	randaoMixes := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(randaoMixes); i++ {
		h := make([]byte, 32)
		copy(h, executionNodeData.BlockHash)
		randaoMixes[i] = h
	}

	zeroHash := params.BeaconConfig().ZeroHash[:]

	activeIndexRoots := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(activeIndexRoots); i++ {
		activeIndexRoots[i] = zeroHash
	}

	blockRoots := make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	for i := 0; i < len(blockRoots); i++ {
		blockRoots[i] = zeroHash
	}

	stateRoots := make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	for i := 0; i < len(stateRoots); i++ {
		stateRoots[i] = zeroHash
	}

	slashings := make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector)

	genesisValidatorsRoot, err := stateutil.ValidatorRegistryRoot(preState.Validators())
	if err != nil {
		return nil, errors.Wrapf(err, "could not hash tree root genesis validators %v", err)
	}

	prevEpochParticipation, err := preState.PreviousEpochParticipation()
	if err != nil {
		return nil, err
	}
	currEpochParticipation, err := preState.CurrentEpochParticipation()
	if err != nil {
		return nil, err
	}
	scores, err := preState.InactivityScores()
	if err != nil {
		return nil, err
	}
	st := &qrysmpb.BeaconStateCapella{
		// Misc fields.
		Slot:                  0,
		GenesisTime:           genesisTime,
		GenesisValidatorsRoot: genesisValidatorsRoot[:],

		Fork: &qrysmpb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},

		// Validator registry fields.
		Validators:                 preState.Validators(),
		Balances:                   preState.Balances(),
		PreviousEpochParticipation: prevEpochParticipation,
		CurrentEpochParticipation:  currEpochParticipation,
		InactivityScores:           scores,

		// Randomness and committees.
		RandaoMixes: randaoMixes,

		// Finality.
		PreviousJustifiedCheckpoint: &qrysmpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		CurrentJustifiedCheckpoint: &qrysmpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		JustificationBits: []byte{0},
		FinalizedCheckpoint: &qrysmpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},

		HistoricalRoots: [][]byte{},
		BlockRoots:      blockRoots,
		StateRoots:      stateRoots,
		Slashings:       slashings,

		// Eth1 data.
		ExecutionNodeData:      executionNodeData,
		ExecutionNodeDataVotes: []*qrysmpb.ExecutionNodeData{},
		Eth1DepositIndex:       preState.Eth1DepositIndex(),
	}

	var scBits [fieldparams.SyncAggregateSyncCommitteeBytesLength]byte
	bodyRoot, err := (&qrysmpb.BeaconBlockBodyCapella{
		RandaoReveal: make([]byte, fieldparams.DilithiumSignatureLength),
		ExecutionNodeData: &qrysmpb.ExecutionNodeData{
			DepositRoot: make([]byte, 32),
			BlockHash:   make([]byte, 32),
		},
		Graffiti: make([]byte, 32),
		SyncAggregate: &qrysmpb.SyncAggregate{
			SyncCommitteeBits:       scBits[:],
			SyncCommitteeSignatures: [][]byte{},
		},
		ExecutionPayload: &enginev1.ExecutionPayloadCapella{
			ParentHash:    make([]byte, 32),
			FeeRecipient:  make([]byte, 20),
			StateRoot:     make([]byte, 32),
			ReceiptsRoot:  make([]byte, 32),
			LogsBloom:     make([]byte, 256),
			PrevRandao:    make([]byte, 32),
			BaseFeePerGas: make([]byte, 32),
			BlockHash:     make([]byte, 32),
		},
	}).HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "could not hash tree root empty block body")
	}

	st.LatestBlockHeader = &qrysmpb.BeaconBlockHeader{
		ParentRoot: zeroHash,
		StateRoot:  zeroHash,
		BodyRoot:   bodyRoot[:],
	}

	var pubKeys [][]byte
	vals := preState.Validators()
	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSize; i++ {
		j := i % uint64(len(vals))
		pubKeys = append(pubKeys, vals[j].PublicKey)
	}

	st.CurrentSyncCommittee = &qrysmpb.SyncCommittee{
		Pubkeys: pubKeys,
	}
	st.NextSyncCommittee = &qrysmpb.SyncCommittee{
		Pubkeys: pubKeys,
	}

	st.LatestExecutionPayloadHeader = &enginev1.ExecutionPayloadHeaderCapella{
		ParentHash:       make([]byte, 32),
		FeeRecipient:     make([]byte, 20),
		StateRoot:        make([]byte, 32),
		ReceiptsRoot:     make([]byte, 32),
		LogsBloom:        make([]byte, 256),
		PrevRandao:       make([]byte, 32),
		BaseFeePerGas:    make([]byte, 32),
		BlockHash:        make([]byte, 32),
		TransactionsRoot: make([]byte, 32),
		WithdrawalsRoot:  make([]byte, 32),
	}

	return state_native.InitializeFromProtoCapella(st)
}

// processPreGenesisDeposits processes a deposit for the beacon state Altair before chain start.
func processPreGenesisDeposits(
	ctx context.Context,
	beaconState state.BeaconState,
	deposits []*qrysmpb.Deposit,
) (state.BeaconState, error) {
	var err error
	beaconState, err = altair.ProcessDeposits(ctx, beaconState, deposits)
	if err != nil {
		return nil, errors.Wrap(err, "could not process deposit")
	}
	beaconState, err = blocks.ActivateValidatorWithEffectiveBalance(beaconState, deposits)
	if err != nil {
		return nil, err
	}
	return beaconState, nil
}
