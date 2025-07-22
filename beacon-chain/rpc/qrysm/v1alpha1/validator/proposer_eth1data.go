package validator

import (
	"context"
	"math/big"

	"github.com/pkg/errors"
	fastssz "github.com/prysmaticlabs/fastssz"
	"github.com/theQRL/qrysm/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/config/features"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/hash"
	"github.com/theQRL/qrysm/crypto/rand"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/time/slots"
)

// executionNodeDataMajorityVote determines the appropriate executionNodeData for a block proposal using
// an algorithm called Voting with the Majority. The algorithm works as follows:
//   - Determine the timestamp for the start slot for the eth1 voting period.
//   - Determine the earliest and latest timestamps that a valid block can have.
//   - Determine the first block not before the earliest timestamp. This block is the lower bound.
//   - Determine the last block not after the latest timestamp. This block is the upper bound.
//   - If the last block is too early, use current executionNodeData from the beacon state.
//   - Filter out votes on unknown blocks and blocks which are outside of the range determined by the lower and upper bounds.
//   - If no blocks are left after filtering votes, use executionNodeData from the latest valid block.
//   - Otherwise:
//   - Determine the vote with the highest count. Prefer the vote with the highest eth1 block height in the event of a tie.
//   - This vote's block is the eth1 block to use for the block proposal.
func (vs *Server) executionNodeDataMajorityVote(ctx context.Context, beaconState state.BeaconState) (*qrysmpb.ExecutionNodeData, error) {
	ctx, cancel := context.WithTimeout(ctx, executionNodeDataTimeout)
	defer cancel()

	slot := beaconState.Slot()
	votingPeriodStartTime := vs.slotStartTime(slot)

	if vs.MockExecutionNodeVotes {
		return vs.mockExecutionNodeDataVote(ctx, slot)
	}
	if !vs.ExecutionNodeInfoFetcher.ExecutionClientConnected() {
		return vs.randomExecutionNodeDataVote(ctx)
	}
	executionNodeDataNotification = false

	genesisTime, _ := vs.ExecutionNodeInfoFetcher.GenesisExecutionChainInfo()
	followDistanceSeconds := params.BeaconConfig().Eth1FollowDistance * params.BeaconConfig().SecondsPerETH1Block
	latestValidTime := votingPeriodStartTime - followDistanceSeconds
	earliestValidTime := votingPeriodStartTime - 2*followDistanceSeconds

	// Special case for starting from a pre-mined genesis: the eth1 vote should be genesis until the chain has advanced
	// by ETH1_FOLLOW_DISTANCE. The head state should maintain the same ExecutionNodeData until this condition has passed, so
	// trust the existing head for the right eth1 vote until we can get a meaningful value from the deposit contract.
	if latestValidTime < genesisTime+followDistanceSeconds {
		log.WithField("genesisTime", genesisTime).WithField("latestValidTime", latestValidTime).Warn("voting period before genesis + follow distance, using executionNodeData from head")
		return vs.HeadFetcher.HeadExecutionNodeData(), nil
	}

	lastBlockByLatestValidTime, err := vs.ExecutionNodeBlockFetcher.BlockByTimestamp(ctx, latestValidTime)
	if err != nil {
		log.WithError(err).Error("Could not get last block by latest valid time")
		return vs.randomExecutionNodeDataVote(ctx)
	}
	if lastBlockByLatestValidTime.Time < earliestValidTime {
		return vs.HeadFetcher.HeadExecutionNodeData(), nil
	}

	lastBlockDepositCount, lastBlockDepositRoot := vs.DepositFetcher.DepositsNumberAndRootAtHeight(ctx, lastBlockByLatestValidTime.Number)
	if lastBlockDepositCount == 0 {
		return vs.ChainStartFetcher.ChainStartExecutionNodeData(), nil
	}

	if lastBlockDepositCount >= vs.HeadFetcher.HeadExecutionNodeData().DepositCount {
		h, err := vs.ExecutionNodeBlockFetcher.BlockHashByHeight(ctx, lastBlockByLatestValidTime.Number)
		if err != nil {
			log.WithError(err).Error("Could not get hash of last block by latest valid time")
			return vs.randomExecutionNodeDataVote(ctx)
		}
		return &qrysmpb.ExecutionNodeData{
			BlockHash:    h.Bytes(),
			DepositCount: lastBlockDepositCount,
			DepositRoot:  lastBlockDepositRoot[:],
		}, nil
	}
	return vs.HeadFetcher.HeadExecutionNodeData(), nil
}

func (vs *Server) slotStartTime(slot primitives.Slot) uint64 {
	startTime, _ := vs.ExecutionNodeInfoFetcher.GenesisExecutionChainInfo()
	return slots.VotingPeriodStartTime(startTime, slot)
}

// canonicalExecutionNodeData determines the canonical executionNodeData and eth1 block height to use for determining deposits.
func (vs *Server) canonicalExecutionNodeData(
	ctx context.Context,
	beaconState state.BeaconState,
	currentVote *qrysmpb.ExecutionNodeData) (*qrysmpb.ExecutionNodeData, *big.Int, error) {
	var eth1BlockHash [32]byte

	// Add in current vote, to get accurate vote tally
	if err := beaconState.AppendExecutionNodeDataVotes(currentVote); err != nil {
		return nil, nil, errors.Wrap(err, "could not append eth1 data votes to state")
	}
	hasSupport, err := blocks.ExecutionNodeDataHasEnoughSupport(beaconState, currentVote)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not determine if current executionNodeData vote has enough support")
	}
	var canonicalExecutionNodeData *qrysmpb.ExecutionNodeData
	if hasSupport {
		canonicalExecutionNodeData = currentVote
		eth1BlockHash = bytesutil.ToBytes32(currentVote.BlockHash)
	} else {
		canonicalExecutionNodeData = beaconState.ExecutionNodeData()
		eth1BlockHash = bytesutil.ToBytes32(beaconState.ExecutionNodeData().BlockHash)
	}
	if features.Get().DisableStakinContractCheck && eth1BlockHash == [32]byte{} {
		return canonicalExecutionNodeData, new(big.Int).SetInt64(0), nil
	}
	_, canonicalExecutionNodeDataHeight, err := vs.ExecutionNodeBlockFetcher.BlockExists(ctx, eth1BlockHash)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not fetch executionNodeData height")
	}
	return canonicalExecutionNodeData, canonicalExecutionNodeDataHeight, nil
}

func (vs *Server) mockExecutionNodeDataVote(ctx context.Context, slot primitives.Slot) (*qrysmpb.ExecutionNodeData, error) {
	if !executionNodeDataNotification {
		log.Warn("Beacon Node is no longer connected to an ETH1 chain, so ETH1 data votes are now mocked.")
		executionNodeDataNotification = true
	}
	// If a mock execution node data votes is specified, we use the following for the
	// executionNodeData we provide to every proposer based on https://github.com/ethereum/eth2.0-pm/issues/62:
	//
	// slot_in_voting_period = current_slot % SLOTS_PER_ETH1_VOTING_PERIOD
	// ExecutionNodeData(
	//   DepositRoot = hash(current_epoch + slot_in_voting_period),
	//   DepositCount = state.eth1_deposit_index,
	//   BlockHash = hash(hash(current_epoch + slot_in_voting_period)),
	// )
	slotInVotingPeriod := slot.ModSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerEth1VotingPeriod)))
	headState, err := vs.HeadFetcher.HeadStateReadOnly(ctx)
	if err != nil {
		return nil, err
	}
	var enc []byte
	enc = fastssz.MarshalUint64(enc, uint64(slots.ToEpoch(slot))+uint64(slotInVotingPeriod))
	depRoot := hash.Hash(enc)
	blockHash := hash.Hash(depRoot[:])
	return &qrysmpb.ExecutionNodeData{
		DepositRoot:  depRoot[:],
		DepositCount: headState.Eth1DepositIndex(),
		BlockHash:    blockHash[:],
	}, nil
}

func (vs *Server) randomExecutionNodeDataVote(ctx context.Context) (*qrysmpb.ExecutionNodeData, error) {
	if !executionNodeDataNotification {
		log.Warn("Beacon Node is no longer connected to an ETH1 chain, so ETH1 data votes are now random.")
		executionNodeDataNotification = true
	}
	headState, err := vs.HeadFetcher.HeadStateReadOnly(ctx)
	if err != nil {
		return nil, err
	}

	// set random roots and block hashes to prevent a majority from being
	// built if the eth1 node is offline
	randGen := rand.NewGenerator()
	depRoot := hash.Hash(bytesutil.Bytes32(randGen.Uint64()))
	blockHash := hash.Hash(bytesutil.Bytes32(randGen.Uint64()))
	return &qrysmpb.ExecutionNodeData{
		DepositRoot:  depRoot[:],
		DepositCount: headState.Eth1DepositIndex(),
		BlockHash:    blockHash[:],
	}, nil
}
