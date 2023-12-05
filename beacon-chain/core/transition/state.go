package transition

import (
	"context"

	"github.com/pkg/errors"
	dilithium2 "github.com/theQRL/go-qrllib/dilithium"
	b "github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	state_native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stateutil"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/container/trie"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// GenesisBeaconState gets called when MinGenesisActiveValidatorCount count of
// full deposits were made to the deposit contract and the ChainStart log gets emitted.
func GenesisBeaconState(ctx context.Context, deposits []*zondpb.Deposit, genesisTime uint64, zond1Data *zondpb.Zond1Data) (state.BeaconState, error) {
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

	return OptimizedGenesisBeaconState(genesisTime, st, st.Zond1Data())
}

// PreminedGenesisBeaconState works almost exactly like GenesisBeaconState, except that it assumes that genesis deposits
// are not represented in the deposit contract and are only found in the genesis state validator registry. In order
// to ensure the deposit root and count match the empty deposit contract deployed in a testnet genesis block, the root
// of an empty deposit trie is computed and used as Zond1Data.deposit_root, and the deposit count is set to 0.
func PreminedGenesisBeaconState(ctx context.Context, deposits []*zondpb.Deposit, genesisTime uint64, zond1Data *zondpb.Zond1Data) (state.BeaconState, error) {
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

	t, err := trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
	if err != nil {
		return nil, err
	}
	dr, err := t.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	if err := st.SetZond1Data(&zondpb.Zond1Data{DepositRoot: dr[:], BlockHash: zond1Data.BlockHash}); err != nil {
		return nil, err
	}
	if err := st.SetZond1DepositIndex(0); err != nil {
		return nil, err
	}
	return OptimizedGenesisBeaconState(genesisTime, st, st.Zond1Data())
}

// OptimizedGenesisBeaconState is used to create a state that has already processed deposits. This is to efficiently
// create a mainnet state at chainstart.
func OptimizedGenesisBeaconState(genesisTime uint64, preState state.BeaconState, zond1Data *zondpb.Zond1Data) (state.BeaconState, error) {
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

		HistoricalRoots:            [][]byte{},
		BlockRoots:                 blockRoots,
		StateRoots:                 stateRoots,
		Slashings:                  slashings,
		CurrentEpochParticipation:  []byte{},
		PreviousEpochParticipation: []byte{},

		// Zond1 data.
		Zond1Data:         zond1Data,
		Zond1DataVotes:    []*zondpb.Zond1Data{},
		Zond1DepositIndex: preState.Zond1DepositIndex(),
	}

	bodyRoot, err := (&zondpb.BeaconBlockBody{
		RandaoReveal: make([]byte, dilithium2.CryptoBytes),
		Zond1Data: &zondpb.Zond1Data{
			DepositRoot: make([]byte, 32),
			BlockHash:   make([]byte, 32),
		},
		Graffiti: make([]byte, 32),
	}).HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "could not hash tree root empty block body")
	}

	st.LatestBlockHeader = &zondpb.BeaconBlockHeader{
		ParentRoot: zeroHash,
		StateRoot:  zeroHash,
		BodyRoot:   bodyRoot[:],
	}

	return state_native.InitializeFromProtoCapella(st)
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

		JustificationBits:          []byte{0},
		HistoricalRoots:            [][]byte{},
		CurrentEpochParticipation:  []byte{},
		PreviousEpochParticipation: []byte{},

		// Zond1 data.
		Zond1Data:         &zondpb.Zond1Data{},
		Zond1DataVotes:    []*zondpb.Zond1Data{},
		Zond1DepositIndex: 0,
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
