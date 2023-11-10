package validator

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	fastssz "github.com/prysmaticlabs/fastssz"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/features"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/hash"
	"github.com/theQRL/qrysm/v4/crypto/rand"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/time/slots"
)

// zond1DataMajorityVote determines the appropriate zond1data for a block proposal using
// an algorithm called Voting with the Majority. The algorithm works as follows:
//   - Determine the timestamp for the start slot for the zond1 voting period.
//   - Determine the earliest and latest timestamps that a valid block can have.
//   - Determine the first block not before the earliest timestamp. This block is the lower bound.
//   - Determine the last block not after the latest timestamp. This block is the upper bound.
//   - If the last block is too early, use current zond1data from the beacon state.
//   - Filter out votes on unknown blocks and blocks which are outside of the range determined by the lower and upper bounds.
//   - If no blocks are left after filtering votes, use zond1data from the latest valid block.
//   - Otherwise:
//   - Determine the vote with the highest count. Prefer the vote with the highest zond1 block height in the event of a tie.
//   - This vote's block is the zond1 block to use for the block proposal.
func (vs *Server) zond1DataMajorityVote(ctx context.Context, beaconState state.BeaconState) (*zondpb.Zond1Data, error) {
	ctx, cancel := context.WithTimeout(ctx, zond1dataTimeout)
	defer cancel()

	slot := beaconState.Slot()
	votingPeriodStartTime := vs.slotStartTime(slot)

	if vs.MockZond1Votes {
		return vs.mockETH1DataVote(ctx, slot)
	}
	if !vs.Zond1InfoFetcher.ExecutionClientConnected() {
		return vs.randomETH1DataVote(ctx)
	}
	zond1DataNotification = false

	genesisTime, _ := vs.Zond1InfoFetcher.GenesisExecutionChainInfo()
	followDistanceSeconds := params.BeaconConfig().Zond1FollowDistance * params.BeaconConfig().SecondsPerETH1Block
	latestValidTime := votingPeriodStartTime - followDistanceSeconds
	earliestValidTime := votingPeriodStartTime - 2*followDistanceSeconds

	// Special case for starting from a pre-mined genesis: the zond1 vote should be genesis until the chain has advanced
	// by ETH1_FOLLOW_DISTANCE. The head state should maintain the same ETH1Data until this condition has passed, so
	// trust the existing head for the right zond1 vote until we can get a meaningful value from the deposit contract.
	if latestValidTime < genesisTime+followDistanceSeconds {
		log.WithField("genesisTime", genesisTime).WithField("latestValidTime", latestValidTime).Warn("voting period before genesis + follow distance, using zond1data from head")
		return vs.HeadFetcher.HeadETH1Data(), nil
	}

	lastBlockByLatestValidTime, err := vs.Zond1BlockFetcher.BlockByTimestamp(ctx, latestValidTime)
	if err != nil {
		log.WithError(err).Error("Could not get last block by latest valid time")
		return vs.randomETH1DataVote(ctx)
	}
	if lastBlockByLatestValidTime.Time < earliestValidTime {
		return vs.HeadFetcher.HeadETH1Data(), nil
	}

	lastBlockDepositCount, lastBlockDepositRoot := vs.DepositFetcher.DepositsNumberAndRootAtHeight(ctx, lastBlockByLatestValidTime.Number)
	if lastBlockDepositCount == 0 {
		return vs.ChainStartFetcher.ChainStartZond1Data(), nil
	}

	if lastBlockDepositCount >= vs.HeadFetcher.HeadETH1Data().DepositCount {
		h, err := vs.Zond1BlockFetcher.BlockHashByHeight(ctx, lastBlockByLatestValidTime.Number)
		if err != nil {
			log.WithError(err).Error("Could not get hash of last block by latest valid time")
			return vs.randomETH1DataVote(ctx)
		}
		return &zondpb.Zond1Data{
			BlockHash:    h.Bytes(),
			DepositCount: lastBlockDepositCount,
			DepositRoot:  lastBlockDepositRoot[:],
		}, nil
	}
	return vs.HeadFetcher.HeadETH1Data(), nil
}

func (vs *Server) slotStartTime(slot primitives.Slot) uint64 {
	startTime, _ := vs.Zond1InfoFetcher.GenesisExecutionChainInfo()
	return slots.VotingPeriodStartTime(startTime, slot)
}

// canonicalZond1Data determines the canonical zond1data and zond1 block height to use for determining deposits.
func (vs *Server) canonicalZond1Data(
	ctx context.Context,
	beaconState state.BeaconState,
	currentVote *zondpb.Zond1Data) (*zondpb.Zond1Data, *big.Int, error) {
	var zond1BlockHash [32]byte

	// Add in current vote, to get accurate vote tally
	if err := beaconState.AppendZond1DataVotes(currentVote); err != nil {
		return nil, nil, errors.Wrap(err, "could not append zond1 data votes to state")
	}
	hasSupport, err := blocks.Zond1DataHasEnoughSupport(beaconState, currentVote)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not determine if current zond1data vote has enough support")
	}
	var canonicalZond1Data *zondpb.Zond1Data
	if hasSupport {
		canonicalZond1Data = currentVote
		zond1BlockHash = bytesutil.ToBytes32(currentVote.BlockHash)
	} else {
		canonicalZond1Data = beaconState.Zond1Data()
		zond1BlockHash = bytesutil.ToBytes32(beaconState.Zond1Data().BlockHash)
	}
	if features.Get().DisableStakinContractCheck && zond1BlockHash == [32]byte{} {
		return canonicalZond1Data, new(big.Int).SetInt64(0), nil
	}
	_, canonicalZond1DataHeight, err := vs.Zond1BlockFetcher.BlockExists(ctx, zond1BlockHash)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not fetch zond1data height")
	}
	return canonicalZond1Data, canonicalZond1DataHeight, nil
}

func (vs *Server) mockETH1DataVote(ctx context.Context, slot primitives.Slot) (*zondpb.Zond1Data, error) {
	if !zond1DataNotification {
		log.Warn("Beacon Node is no longer connected to an ETH1 chain, so ETH1 data votes are now mocked.")
		zond1DataNotification = true
	}
	// If a mock zond1 data votes is specified, we use the following for the
	// zond1data we provide to every proposer based on https://github.com/ethereum/eth2.0-pm/issues/62:
	//
	// slot_in_voting_period = current_slot % SLOTS_PER_ETH1_VOTING_PERIOD
	// Zond1Data(
	//   DepositRoot = hash(current_epoch + slot_in_voting_period),
	//   DepositCount = state.zond1_deposit_index,
	//   BlockHash = hash(hash(current_epoch + slot_in_voting_period)),
	// )
	slotInVotingPeriod := slot.ModSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerZond1VotingPeriod)))
	headState, err := vs.HeadFetcher.HeadStateReadOnly(ctx)
	if err != nil {
		return nil, err
	}
	var enc []byte
	enc = fastssz.MarshalUint64(enc, uint64(slots.ToEpoch(slot))+uint64(slotInVotingPeriod))
	depRoot := hash.Hash(enc)
	blockHash := hash.Hash(depRoot[:])
	return &zondpb.Zond1Data{
		DepositRoot:  depRoot[:],
		DepositCount: headState.Zond1DepositIndex(),
		BlockHash:    blockHash[:],
	}, nil
}

func (vs *Server) randomETH1DataVote(ctx context.Context) (*zondpb.Zond1Data, error) {
	if !zond1DataNotification {
		log.Warn("Beacon Node is no longer connected to an ETH1 chain, so ETH1 data votes are now random.")
		zond1DataNotification = true
	}
	headState, err := vs.HeadFetcher.HeadStateReadOnly(ctx)
	if err != nil {
		return nil, err
	}

	// set random roots and block hashes to prevent a majority from being
	// built if the zond1 node is offline
	randGen := rand.NewGenerator()
	depRoot := hash.Hash(bytesutil.Bytes32(randGen.Uint64()))
	blockHash := hash.Hash(bytesutil.Bytes32(randGen.Uint64()))
	return &zondpb.Zond1Data{
		DepositRoot:  depRoot[:],
		DepositCount: headState.Zond1DepositIndex(),
		BlockHash:    blockHash[:],
	}, nil
}
