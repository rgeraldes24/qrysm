package util

import (
	"context"
	"fmt"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/go-zond/common/hexutil"
	b "github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/db/iface"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	state_native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/require"
)

// FillRootsNaturalOpt is meant to be used as an option when calling NewBeaconState.
// It fills state and block roots with hex representations of natural numbers starting with 0.
// Example: 16 becomes 0x00...0f.
func FillRootsNaturalOpt(state *zondpb.BeaconState) error {
	roots, err := PrepareRoots(int(params.BeaconConfig().SlotsPerHistoricalRoot))
	if err != nil {
		return err
	}
	state.StateRoots = roots
	state.BlockRoots = roots
	return nil
}

type NewBeaconStateOption func(state *zondpb.BeaconState) error

// NewBeaconState creates a beacon state with minimum marshalable fields.
func NewBeaconState(options ...func(state *zondpb.BeaconState) error) (state.BeaconState, error) {
	pubkeys := make([][]byte, 0, params.BeaconConfig().SyncCommitteeSize)
	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSize; i++ {
		pubkeys = append(pubkeys, make([]byte, 2592))
	}

	seed := &zondpb.BeaconState{
		BlockRoots:                 filledByteSlice2D(uint64(params.BeaconConfig().SlotsPerHistoricalRoot), 32),
		StateRoots:                 filledByteSlice2D(uint64(params.BeaconConfig().SlotsPerHistoricalRoot), 32),
		Slashings:                  make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
		RandaoMixes:                filledByteSlice2D(uint64(params.BeaconConfig().EpochsPerHistoricalVector), 32),
		Validators:                 make([]*zondpb.Validator, 0),
		CurrentJustifiedCheckpoint: &zondpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
		Zond1Data: &zondpb.Zond1Data{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, 32),
		},
		Fork: &zondpb.Fork{
			PreviousVersion: make([]byte, 4),
			CurrentVersion:  make([]byte, 4),
		},
		Zond1DataVotes:              make([]*zondpb.Zond1Data, 0),
		HistoricalSummaries:         make([]*zondpb.HistoricalSummary, 0),
		JustificationBits:           bitfield.Bitvector4{0x0},
		FinalizedCheckpoint:         &zondpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
		LatestBlockHeader:           HydrateBeaconHeader(&zondpb.BeaconBlockHeader{}),
		PreviousJustifiedCheckpoint: &zondpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
		PreviousEpochParticipation:  make([]byte, 0),
		CurrentEpochParticipation:   make([]byte, 0),
		CurrentSyncCommittee: &zondpb.SyncCommittee{
			Pubkeys: pubkeys,
		},
		NextSyncCommittee: &zondpb.SyncCommittee{
			Pubkeys: pubkeys,
		},
		LatestExecutionPayloadHeader: &enginev1.ExecutionPayloadHeader{
			ParentHash:       make([]byte, 32),
			FeeRecipient:     make([]byte, 20),
			StateRoot:        make([]byte, 32),
			ReceiptsRoot:     make([]byte, 32),
			LogsBloom:        make([]byte, 256),
			PrevRandao:       make([]byte, 32),
			ExtraData:        make([]byte, 0),
			BaseFeePerGas:    make([]byte, 32),
			BlockHash:        make([]byte, 32),
			TransactionsRoot: make([]byte, 32),
			WithdrawalsRoot:  make([]byte, 32),
		},
	}

	for _, opt := range options {
		err := opt(seed)
		if err != nil {
			return nil, err
		}
	}

	var st, err = state_native.InitializeFromProtoUnsafeCapella(seed)
	if err != nil {
		return nil, err
	}

	return st.Copy(), nil
}

// SSZ will fill 2D byte slices with their respective values, so we must fill these in too for round
// trip testing.
func filledByteSlice2D(length, innerLen uint64) [][]byte {
	b := make([][]byte, length)
	for i := uint64(0); i < length; i++ {
		b[i] = make([]byte, innerLen)
	}
	return b
}

// PrepareRoots returns a list of roots with hex representations of natural numbers starting with 0.
// Example: 16 becomes 0x00...0f.
func PrepareRoots(size int) ([][]byte, error) {
	roots := make([][]byte, size)
	for i := 0; i < size; i++ {
		roots[i] = make([]byte, fieldparams.RootLength)
	}
	for j := 0; j < len(roots); j++ {
		// Remove '0x' prefix and left-pad '0' to have 64 chars in total.
		s := fmt.Sprintf("%064s", hexutil.EncodeUint64(uint64(j))[2:])
		h, err := hexutil.Decode("0x" + s)
		if err != nil {
			return nil, err
		}
		roots[j] = h
	}
	return roots, nil
}

// DeterministicGenesisStateWithGenesisBlock creates a genesis state, saves the genesis block,
// genesis state and head block root. It returns the genesis state, genesis block's root and
// validator private keys.
func DeterministicGenesisStateWithGenesisBlock(
	t *testing.T,
	ctx context.Context,
	db iface.HeadAccessDatabase,
	numValidators uint64,
) (state.BeaconState, [32]byte, []dilithium.DilithiumKey) {
	genesisState, privateKeys := DeterministicGenesisState(t, numValidators)
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
