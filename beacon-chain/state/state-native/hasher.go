package state_native

import (
	"context"
	"encoding/binary"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/state-native/types"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stateutil"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/encoding/ssz"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"go.opencensus.io/trace"
)

// ComputeFieldRootsWithHasher hashes the provided state and returns its respective field roots.
func ComputeFieldRootsWithHasher(ctx context.Context, state *BeaconState) ([][]byte, error) {
	ctx, span := trace.StartSpan(ctx, "ComputeFieldRootsWithHasher")
	defer span.End()

	if state == nil {
		return nil, errors.New("nil state")
	}
	var fieldRoots [][]byte
	switch state.version {
	case version.Capella:
		fieldRoots = make([][]byte, params.BeaconConfig().BeaconStateFieldCount)
	}

	// Genesis time root.
	genesisRoot := ssz.Uint64Root(state.genesisTime)
	fieldRoots[types.GenesisTime.RealPosition()] = genesisRoot[:]

	// Genesis validators root.
	var r [32]byte
	copy(r[:], state.genesisValidatorsRoot[:])
	fieldRoots[types.GenesisValidatorsRoot.RealPosition()] = r[:]

	// Slot root.
	slotRoot := ssz.Uint64Root(uint64(state.slot))
	fieldRoots[types.Slot.RealPosition()] = slotRoot[:]

	// Fork data structure root.
	forkHashTreeRoot, err := ssz.ForkRoot(state.fork)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute fork merkleization")
	}
	fieldRoots[types.Fork.RealPosition()] = forkHashTreeRoot[:]

	// BeaconBlockHeader data structure root.
	headerHashTreeRoot, err := stateutil.BlockHeaderRoot(state.latestBlockHeader)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute block header merkleization")
	}
	fieldRoots[types.LatestBlockHeader.RealPosition()] = headerHashTreeRoot[:]

	// BlockRoots array root.
	bRoots := make([][]byte, len(state.blockRoots))
	for i := range bRoots {
		bRoots[i] = state.blockRoots[i][:]
	}
	blockRootsRoot, err := stateutil.ArraysRoot(bRoots, fieldparams.BlockRootsLength)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute block roots merkleization")
	}
	fieldRoots[types.BlockRoots.RealPosition()] = blockRootsRoot[:]

	// StateRoots array root.
	sRoots := make([][]byte, len(state.stateRoots))
	for i := range sRoots {
		sRoots[i] = state.stateRoots[i][:]
	}
	stateRootsRoot, err := stateutil.ArraysRoot(sRoots, fieldparams.StateRootsLength)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute state roots merkleization")
	}
	fieldRoots[types.StateRoots.RealPosition()] = stateRootsRoot[:]

	// HistoricalRoots slice root.
	hRoots := make([][]byte, len(state.historicalRoots))
	for i := range hRoots {
		hRoots[i] = state.historicalRoots[i][:]
	}
	historicalRootsRt, err := ssz.ByteArrayRootWithLimit(hRoots, fieldparams.HistoricalRootsLength)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute historical roots merkleization")
	}
	fieldRoots[types.HistoricalRoots.RealPosition()] = historicalRootsRt[:]

	// Zond1Data data structure root.
	zond1HashTreeRoot, err := stateutil.Zond1Root(state.zond1Data)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute zond1data merkleization")
	}
	fieldRoots[types.Zond1Data.RealPosition()] = zond1HashTreeRoot[:]

	// Zond1DataVotes slice root.
	zond1VotesRoot, err := stateutil.Zond1DataVotesRoot(state.zond1DataVotes)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute zond1data votes merkleization")
	}
	fieldRoots[types.Zond1DataVotes.RealPosition()] = zond1VotesRoot[:]

	// Zond1DepositIndex root.
	zond1DepositIndexBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(zond1DepositIndexBuf, state.zond1DepositIndex)
	zond1DepositBuf := bytesutil.ToBytes32(zond1DepositIndexBuf)
	fieldRoots[types.Zond1DepositIndex.RealPosition()] = zond1DepositBuf[:]

	// Validators slice root.
	validatorsRoot, err := stateutil.ValidatorRegistryRoot(state.validators)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute validator registry merkleization")
	}
	fieldRoots[types.Validators.RealPosition()] = validatorsRoot[:]

	// Balances slice root.
	balancesRoot, err := stateutil.Uint64ListRootWithRegistryLimit(state.balances)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute validator balances merkleization")
	}
	fieldRoots[types.Balances.RealPosition()] = balancesRoot[:]

	// RandaoMixes array root.
	mixes := make([][]byte, len(state.randaoMixes))
	for i := range mixes {
		mixes[i] = state.randaoMixes[i][:]
	}
	randaoRootsRoot, err := stateutil.ArraysRoot(mixes, fieldparams.RandaoMixesLength)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute randao roots merkleization")
	}
	fieldRoots[types.RandaoMixes.RealPosition()] = randaoRootsRoot[:]

	// Slashings array root.
	slashingsRootsRoot, err := ssz.SlashingsRoot(state.slashings)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute slashings merkleization")
	}
	fieldRoots[types.Slashings.RealPosition()] = slashingsRootsRoot[:]

	// JustificationBits root.
	justifiedBitsRoot := bytesutil.ToBytes32(state.justificationBits)
	fieldRoots[types.JustificationBits.RealPosition()] = justifiedBitsRoot[:]

	// PreviousJustifiedCheckpoint data structure root.
	prevCheckRoot, err := ssz.CheckpointRoot(state.previousJustifiedCheckpoint)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute previous justified checkpoint merkleization")
	}
	fieldRoots[types.PreviousJustifiedCheckpoint.RealPosition()] = prevCheckRoot[:]

	// CurrentJustifiedCheckpoint data structure root.
	currJustRoot, err := ssz.CheckpointRoot(state.currentJustifiedCheckpoint)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute current justified checkpoint merkleization")
	}
	fieldRoots[types.CurrentJustifiedCheckpoint.RealPosition()] = currJustRoot[:]

	// FinalizedCheckpoint data structure root.
	finalRoot, err := ssz.CheckpointRoot(state.finalizedCheckpoint)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute finalized checkpoint merkleization")
	}
	fieldRoots[types.FinalizedCheckpoint.RealPosition()] = finalRoot[:]

	// Inactivity scores root.
	inactivityScoresRoot, err := stateutil.Uint64ListRootWithRegistryLimit(state.inactivityScores)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute inactivityScoreRoot")
	}
	fieldRoots[types.InactivityScores.RealPosition()] = inactivityScoresRoot[:]

	// Current sync committee root.
	currentSyncCommitteeRoot, err := stateutil.SyncCommitteeRoot(state.currentSyncCommittee)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute sync committee merkleization")
	}
	fieldRoots[types.CurrentSyncCommittee.RealPosition()] = currentSyncCommitteeRoot[:]

	// Next sync committee root.
	nextSyncCommitteeRoot, err := stateutil.SyncCommitteeRoot(state.nextSyncCommittee)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute sync committee merkleization")
	}
	fieldRoots[types.NextSyncCommittee.RealPosition()] = nextSyncCommitteeRoot[:]

	// Execution payload root.
	executionPayloadRoot, err := state.latestExecutionPayloadHeader.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	fieldRoots[types.LatestExecutionPayloadHeader.RealPosition()] = executionPayloadRoot[:]

	// Next withdrawal index root.
	nextWithdrawalIndexRoot := make([]byte, 32)
	binary.LittleEndian.PutUint64(nextWithdrawalIndexRoot, state.nextWithdrawalIndex)
	fieldRoots[types.NextWithdrawalIndex.RealPosition()] = nextWithdrawalIndexRoot

	// Next partial withdrawal validator index root.
	nextWithdrawalValidatorIndexRoot := make([]byte, 32)
	binary.LittleEndian.PutUint64(nextWithdrawalValidatorIndexRoot, uint64(state.nextWithdrawalValidatorIndex))
	fieldRoots[types.NextWithdrawalValidatorIndex.RealPosition()] = nextWithdrawalValidatorIndexRoot

	// Historical summary root.
	historicalSummaryRoot, err := stateutil.HistoricalSummariesRoot(state.historicalSummaries)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute historical summary merkleization")
	}
	fieldRoots[types.HistoricalSummaries.RealPosition()] = historicalSummaryRoot[:]

	return fieldRoots, nil
}
