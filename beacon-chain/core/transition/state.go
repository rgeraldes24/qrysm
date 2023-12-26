package transition

import (
	"context"

	"github.com/pkg/errors"
	"github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/altair"
	b "github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	state_native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stateutil"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// GenesisBeaconState gets called when MinGenesisActiveValidatorCount count of
// full deposits were made to the deposit contract and the ChainStart log gets emitted.
func GenesisBeaconState(ctx context.Context, deposits []*zondpb.Deposit, genesisTime uint64, zond1Data *zondpb.Zond1Data, ep *enginev1.ExecutionPayload) (state.BeaconState, error) {
	st, err := EmptyGenesisState()
	if err != nil {
		return nil, err
	}

	// Process initial deposits.
	st, err = helpers.UpdateGenesisZond1Data(st, deposits, zond1Data)
	if err != nil {
		return nil, err
	}

	st, err = b.ProcessPreGenesisDeposits(ctx, st, deposits)
	if err != nil {
		return nil, errors.Wrap(err, "could not process validator deposits")
	}

	// After deposits have been processed, overwrite zond1data to what is passed in. This allows us to "pre-mine" validators
	// without the deposit root and count mismatching the real deposit contract.
	if err := st.SetZond1Data(zond1Data); err != nil {
		return nil, err
	}
	if err := st.SetZond1DepositIndex(zond1Data.DepositCount); err != nil {
		return nil, err
	}

	return OptimizedGenesisBeaconState(genesisTime, st, st.Zond1Data(), ep)
}

// OptimizedGenesisBeaconState is used to create a state that has already processed deposits. This is to efficiently
// create a mainnet state at chainstart.
func OptimizedGenesisBeaconState(genesisTime uint64, preState state.BeaconState, zond1Data *zondpb.Zond1Data, ep *enginev1.ExecutionPayload) (state.BeaconState, error) {
	if zond1Data == nil {
		return nil, errors.New("no zond1data provided for genesis state")
	}

	randaoMixes := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(randaoMixes); i++ {
		h := make([]byte, 32)
		copy(h, zond1Data.BlockHash)
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

	scores, err := preState.InactivityScores()
	if err != nil {
		return nil, err
	}
	scoresMissing := len(preState.Validators()) - len(scores)
	if scoresMissing > 0 {
		for i := 0; i < scoresMissing; i++ {
			scores = append(scores, 0)
		}
	}

	// TODO(rgeraldes24) - review value
	wep, err := blocks.WrappedExecutionPayload(ep, 0)
	if err != nil {
		return nil, err
	}
	eph, err := blocks.PayloadToHeader(wep)
	if err != nil {
		return nil, err
	}
	st := &zondpb.BeaconState{
		// Misc fields.
		Slot:                  0,
		GenesisTime:           genesisTime,
		GenesisValidatorsRoot: genesisValidatorsRoot[:],

		Fork: &zondpb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},

		// Validator registry fields.
		Validators: preState.Validators(),
		Balances:   preState.Balances(),

		// Randomness and committees.
		RandaoMixes: randaoMixes,

		// Finality.
		PreviousJustifiedCheckpoint: &zondpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		CurrentJustifiedCheckpoint: &zondpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},
		JustificationBits: []byte{0},
		FinalizedCheckpoint: &zondpb.Checkpoint{
			Epoch: 0,
			Root:  params.BeaconConfig().ZeroHash[:],
		},

		HistoricalRoots: [][]byte{},
		BlockRoots:      blockRoots,
		StateRoots:      stateRoots,
		Slashings:       slashings,

		// Zond1 data.
		Zond1Data:                    zond1Data,
		Zond1DataVotes:               []*zondpb.Zond1Data{},
		Zond1DepositIndex:            preState.Zond1DepositIndex(),
		LatestExecutionPayloadHeader: eph,
		InactivityScores:             scores,
	}

	bodyRoot, err := (&zondpb.BeaconBlockBody{
		RandaoReveal: make([]byte, dilithium.CryptoBytes),
		Zond1Data: &zondpb.Zond1Data{
			DepositRoot: make([]byte, 32),
			BlockHash:   make([]byte, 32),
		},
		Graffiti: make([]byte, 32),
		SyncAggregate: &zondpb.SyncAggregate{
			SyncCommitteeBits:       make([]byte, fieldparams.SyncCommitteeLength/8),
			SyncCommitteeSignatures: make([][]byte, 0),
		},
		ExecutionPayload: &enginev1.ExecutionPayload{
			ParentHash:    make([]byte, 32),
			FeeRecipient:  make([]byte, 20),
			StateRoot:     make([]byte, 32),
			ReceiptsRoot:  make([]byte, 32),
			LogsBloom:     make([]byte, 256),
			PrevRandao:    make([]byte, 32),
			BaseFeePerGas: make([]byte, 32),
			BlockHash:     make([]byte, 32),
			Transactions:  make([][]byte, 0),
			Withdrawals:   make([]*enginev1.Withdrawal, 0),
		},
		DilithiumToExecutionChanges: make([]*zondpb.SignedDilithiumToExecutionChange, 0),
	}).HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "could not hash tree root empty block body")
	}

	st.LatestBlockHeader = &zondpb.BeaconBlockHeader{
		ParentRoot: zeroHash,
		StateRoot:  zeroHash,
		BodyRoot:   bodyRoot[:],
	}

	ist, err := state_native.InitializeFromProtoCapella(st)
	if err != nil {
		return nil, err
	}
	sc, err := altair.NextSyncCommittee(context.Background(), ist)
	if err != nil {
		return nil, err
	}
	if err := ist.SetNextSyncCommittee(sc); err != nil {
		return nil, err
	}
	if err := ist.SetCurrentSyncCommittee(sc); err != nil {
		return nil, err
	}
	return ist, nil
}

// EmptyGenesisState returns an empty beacon state object.
func EmptyGenesisState() (state.BeaconState, error) {
	st := &zondpb.BeaconState{
		// Misc fields.
		Slot: 0,
		Fork: &zondpb.Fork{
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			Epoch:           0,
		},
		// Validator registry fields.
		Validators: []*zondpb.Validator{},
		Balances:   []uint64{},

		JustificationBits: []byte{0},
		HistoricalRoots:   [][]byte{},

		// Zond1 data.
		Zond1Data:         &zondpb.Zond1Data{},
		Zond1DataVotes:    []*zondpb.Zond1Data{},
		Zond1DepositIndex: 0,
		LatestExecutionPayloadHeader: &enginev1.ExecutionPayloadHeader{
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
		},
	}

	return state_native.InitializeFromProtoCapella(st)
}

// IsValidGenesisState gets called whenever there's a deposit event,
// it checks whether there's enough effective balance to trigger and
// if the minimum genesis time arrived already.
func IsValidGenesisState(chainStartDepositCount, currentTime uint64) bool {
	if currentTime < params.BeaconConfig().MinGenesisTime {
		return false
	}
	if chainStartDepositCount < params.BeaconConfig().MinGenesisActiveValidatorCount {
		return false
	}
	return true
}
