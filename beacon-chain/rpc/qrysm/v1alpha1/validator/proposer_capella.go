package validator

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	dilithium2 "github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/api/client/builder"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/encoding/ssz"
	"github.com/theQRL/qrysm/v4/monitoring/tracing"
	"github.com/theQRL/qrysm/v4/network/forks"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	synccontribution "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation/aggregation/sync_contribution"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"github.com/theQRL/qrysm/v4/time/slots"
	"go.opencensus.io/trace"
)

// builderGetPayloadMissCount tracks the number of misses when validator tries to get a payload from builder
var builderGetPayloadMissCount = promauto.NewCounter(prometheus.CounterOpts{
	Name: "builder_get_payload_miss_count",
	Help: "The number of get payload misses for validator requests to builder",
})

// emptyTransactionsRoot represents the returned value of ssz.TransactionsRoot([][]byte{}) and
// can be used as a constant to avoid recomputing this value in every call.
var emptyTransactionsRoot = [32]byte{127, 254, 36, 30, 166, 1, 135, 253, 176, 24, 123, 250, 34, 222, 53, 209, 249, 190, 215, 171, 6, 29, 148, 1, 253, 71, 227, 74, 84, 251, 237, 225}

// blockBuilderTimeout is the maximum amount of time allowed for a block builder to respond to a
// block request. This value is known as `BUILDER_PROPOSAL_DELAY_TOLERANCE` in builder spec.
const blockBuilderTimeout = 1 * time.Second

// Sets the execution data for the block. Execution data can come from local EL client or remote builder depends on validator registration and circuit breaker conditions.
func setExecutionData(ctx context.Context, blk interfaces.SignedBeaconBlock, localPayload, builderPayload interfaces.ExecutionData) error {
	_, span := trace.StartSpan(ctx, "ProposerServer.setExecutionData")
	defer span.End()

	if localPayload == nil {
		return errors.New("local payload is nil")
	}

	// Use local payload if builder payload is nil.
	if builderPayload == nil {
		return blk.SetExecution(localPayload)
	}

	switch {
	case blk.Version() >= version.Capella:
		// Compare payload values between local and builder. Default to the local value if it is higher.
		localValueGwei, err := localPayload.ValueInGwei()
		if err != nil {
			return errors.Wrap(err, "failed to get local payload value")
		}
		builderValueGwei, err := builderPayload.ValueInGwei()
		if err != nil {
			log.WithError(err).Warn("Proposer: failed to get builder payload value") // Default to local if can't get builder value.
			return blk.SetExecution(localPayload)
		}

		withdrawalsMatched, err := matchingWithdrawalsRoot(localPayload, builderPayload)
		if err != nil {
			tracing.AnnotateError(span, err)
			log.WithError(err).Warn("Proposer: failed to match withdrawals root")
			return blk.SetExecution(localPayload)
		}

		// Use builder payload if the following in true:
		// builder_bid_value * 100 > local_block_value * (local-block-value-boost + 100)
		boost := params.BeaconConfig().LocalBlockValueBoost
		higherValueBuilder := builderValueGwei*100 > localValueGwei*(100+boost)

		// If we can't get the builder value, just use local block.
		if higherValueBuilder && withdrawalsMatched { // Builder value is higher and withdrawals match.
			blk.SetBlinded(true)
			if err := blk.SetExecution(builderPayload); err != nil {
				log.WithError(err).Warn("Proposer: failed to set builder payload")
				blk.SetBlinded(false)
				return blk.SetExecution(localPayload)
			} else {
				return nil
			}
		}
		if !higherValueBuilder {
			log.WithFields(logrus.Fields{
				"localGweiValue":       localValueGwei,
				"localBoostPercentage": boost,
				"builderGweiValue":     builderValueGwei,
			}).Warn("Proposer: using local execution payload because higher value")
		}
		span.AddAttributes(
			trace.BoolAttribute("higherValueBuilder", higherValueBuilder),
			trace.Int64Attribute("localGweiValue", int64(localValueGwei)),     // lint:ignore uintcast -- This is OK for tracing.
			trace.Int64Attribute("localBoostPercentage", int64(boost)),        // lint:ignore uintcast -- This is OK for tracing.
			trace.Int64Attribute("builderGweiValue", int64(builderValueGwei)), // lint:ignore uintcast -- This is OK for tracing.
		)
		return blk.SetExecution(localPayload)
	default: // Bellatrix case.
		blk.SetBlinded(true)
		if err := blk.SetExecution(builderPayload); err != nil {
			log.WithError(err).Warn("Proposer: failed to set builder payload")
			blk.SetBlinded(false)
			return blk.SetExecution(localPayload)
		} else {
			return nil
		}
	}
}

// This function retrieves the payload header given the slot number and the validator index.
// It's a no-op if the latest head block is not versioned bellatrix.
func (vs *Server) getPayloadHeaderFromBuilder(ctx context.Context, slot primitives.Slot, idx primitives.ValidatorIndex) (interfaces.ExecutionData, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.getPayloadHeaderFromBuilder")
	defer span.End()

	b, err := vs.HeadFetcher.HeadBlock(ctx)
	if err != nil {
		return nil, err
	}

	h, err := b.Block().Body().Execution()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get execution header")
	}
	pk, err := vs.HeadFetcher.HeadValidatorIndexToPublicKey(ctx, idx)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, blockBuilderTimeout)
	defer cancel()

	signedBid, err := vs.BlockBuilder.GetHeader(ctx, slot, bytesutil.ToBytes32(h.BlockHash()), pk)
	if err != nil {
		return nil, err
	}
	if signedBid.IsNil() {
		return nil, errors.New("builder returned nil bid")
	}
	fork, err := forks.Fork(slots.ToEpoch(slot))
	if err != nil {
		return nil, errors.Wrap(err, "unable to get fork information")
	}
	forkName, ok := params.BeaconConfig().ForkVersionNames[bytesutil.ToBytes4(fork.CurrentVersion)]
	if !ok {
		return nil, errors.New("unable to find current fork in schedule")
	}
	if !strings.EqualFold(version.String(signedBid.Version()), forkName) {
		return nil, fmt.Errorf("builder bid response version: %d is different from head block version: %d for epoch %d", signedBid.Version(), b.Version(), slots.ToEpoch(slot))
	}

	bid, err := signedBid.Message()
	if err != nil {
		return nil, errors.Wrap(err, "could not get bid")
	}
	if bid.IsNil() {
		return nil, errors.New("builder returned nil bid")
	}

	v := bytesutil.LittleEndianBytesToBigInt(bid.Value())
	if v.String() == "0" {
		return nil, errors.New("builder returned header with 0 bid amount")
	}

	header, err := bid.Header()
	if err != nil {
		return nil, errors.Wrap(err, "could not get bid header")
	}
	txRoot, err := header.TransactionsRoot()
	if err != nil {
		return nil, errors.Wrap(err, "could not get transaction root")
	}
	if bytesutil.ToBytes32(txRoot) == emptyTransactionsRoot {
		return nil, errors.New("builder returned header with an empty tx root")
	}

	if !bytes.Equal(header.ParentHash(), h.BlockHash()) {
		return nil, fmt.Errorf("incorrect parent hash %#x != %#x", header.ParentHash(), h.BlockHash())
	}

	t, err := slots.ToTime(uint64(vs.TimeFetcher.GenesisTime().Unix()), slot)
	if err != nil {
		return nil, err
	}
	if header.Timestamp() != uint64(t.Unix()) {
		return nil, fmt.Errorf("incorrect timestamp %d != %d", header.Timestamp(), uint64(t.Unix()))
	}

	if err := validateBuilderSignature(signedBid); err != nil {
		return nil, errors.Wrap(err, "could not validate builder signature")
	}

	log.WithFields(logrus.Fields{
		"value":              v.String(),
		"builderPubKey":      fmt.Sprintf("%#x", bid.Pubkey()),
		"blockHash":          fmt.Sprintf("%#x", header.BlockHash()),
		"slot":               slot,
		"validator":          idx,
		"sinceSlotStartTime": time.Since(t),
	}).Info("Received header with bid")

	span.AddAttributes(
		trace.StringAttribute("value", v.String()),
		trace.StringAttribute("builderPubKey", fmt.Sprintf("%#x", bid.Pubkey())),
		trace.StringAttribute("blockHash", fmt.Sprintf("%#x", header.BlockHash())),
	)

	return header, nil
}

// Validates builder signature and returns an error if the signature is invalid.
func validateBuilderSignature(signedBid builder.SignedBid) error {
	d, err := signing.ComputeDomain(params.BeaconConfig().DomainApplicationBuilder,
		nil, /* fork version */
		nil /* genesis val root */)
	if err != nil {
		return err
	}
	if signedBid.IsNil() {
		return errors.New("nil builder bid")
	}
	bid, err := signedBid.Message()
	if err != nil {
		return errors.Wrap(err, "could not get bid")
	}
	if bid.IsNil() {
		return errors.New("builder returned nil bid")
	}
	return signing.VerifySigningRoot(bid, bid.Pubkey(), signedBid.Signature(), d)
}

func matchingWithdrawalsRoot(local, builder interfaces.ExecutionData) (bool, error) {
	wds, err := local.Withdrawals()
	if err != nil {
		return false, errors.Wrap(err, "could not get local withdrawals")
	}
	br, err := builder.WithdrawalsRoot()
	if err != nil {
		return false, errors.Wrap(err, "could not get builder withdrawals root")
	}
	wr, err := ssz.WithdrawalSliceRoot(wds, fieldparams.MaxWithdrawalsPerPayload)
	if err != nil {
		return false, errors.Wrap(err, "could not compute local withdrawals root")
	}

	if !bytes.Equal(br, wr[:]) {
		log.WithFields(logrus.Fields{
			"local":   fmt.Sprintf("%#x", wr),
			"builder": fmt.Sprintf("%#x", br),
		}).Warn("Proposer: withdrawal roots don't match, using local block")
		return false, nil
	}
	return true, nil
}

// Sets the dilithium to exec data for a block.
func (vs *Server) setDilithiumToExecData(blk interfaces.SignedBeaconBlock, headState state.BeaconState) {
	if blk.Version() < version.Capella {
		return
	}
	if err := blk.SetDilithiumToExecutionChanges([]*zondpb.SignedDilithiumToExecutionChange{}); err != nil {
		log.WithError(err).Error("Could not set dilithium to execution data in block")
		return
	}
	changes, err := vs.DilithiumChangesPool.DilithiumToExecChangesForInclusion(headState)
	if err != nil {
		log.WithError(err).Error("Could not get dilithium to execution changes")
		return
	} else {
		if err := blk.SetDilithiumToExecutionChanges(changes); err != nil {
			log.WithError(err).Error("Could not set dilithium to execution changes")
			return
		}
	}
}

func (vs *Server) setSyncAggregate(ctx context.Context, blk interfaces.SignedBeaconBlock) {
	syncAggregate, err := vs.getSyncAggregate(ctx, blk.Block().Slot()-1, blk.Block().ParentRoot())
	if err != nil {
		log.WithError(err).Error("Could not get sync aggregate")
		emptySig := [dilithium2.CryptoBytes]byte{0xC0}
		emptyAggregate := &zondpb.SyncAggregate{
			SyncCommitteeBits:       make([]byte, params.BeaconConfig().SyncCommitteeSize/8),
			SyncCommitteeSignatures: [][]byte{emptySig[:]},
		}
		if err := blk.SetSyncAggregate(emptyAggregate); err != nil {
			log.WithError(err).Error("Could not set sync aggregate")
		}
		return
	}

	// Can not error. We already filter block versioning at the top. Phase 0 is impossible.
	if err := blk.SetSyncAggregate(syncAggregate); err != nil {
		log.WithError(err).Error("Could not set sync aggregate")
	}
}

// getSyncAggregate retrieves the sync contributions from the pool to construct the sync aggregate object.
// The contributions are filtered based on matching of the input root and slot then profitability.
func (vs *Server) getSyncAggregate(ctx context.Context, slot primitives.Slot, root [32]byte) (*zondpb.SyncAggregate, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.getSyncAggregate")
	defer span.End()

	if vs.SyncCommitteePool == nil {
		return nil, errors.New("sync committee pool is nil")
	}
	// Contributions have to match the input root
	contributions, err := vs.SyncCommitteePool.SyncCommitteeContributions(slot)
	if err != nil {
		return nil, err
	}
	proposerContributions := proposerSyncContributions(contributions).filterByBlockRoot(root)

	// Each sync subcommittee is 128 bits and the sync committee is 512 bits for mainnet.
	var bitsHolder [][]byte
	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
		bitsHolder = append(bitsHolder, zondpb.NewSyncCommitteeParticipationBits())
	}

	sigsHolder := make([][]byte, 0, params.BeaconConfig().SyncCommitteeSize)

	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
		cs := proposerContributions.filterBySubIndex(i)
		aggregates, err := synccontribution.Aggregate(cs)
		if err != nil {
			return nil, err
		}

		// Retrieve the most profitable contribution
		deduped, err := proposerSyncContributions(aggregates).dedup()
		if err != nil {
			return nil, err
		}
		c := deduped.mostProfitable()
		if c == nil {
			continue
		}
		bitsHolder[i] = c.ParticipationBits
		sigsHolder = append(sigsHolder, c.Signatures...)
	}

	// Aggregate all the contribution bits.
	var syncBits []byte
	for _, b := range bitsHolder {
		syncBits = append(syncBits, b...)
	}

	return &zondpb.SyncAggregate{
		SyncCommitteeBits:       syncBits,
		SyncCommitteeSignatures: sigsHolder,
	}, nil
}