package validator

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	emptypb "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/go-zond/common"
	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/v4/beacon-chain/blockchain"
	"github.com/theQRL/qrysm/v4/beacon-chain/builder"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/feed"
	blockfeed "github.com/theQRL/qrysm/v4/beacon-chain/core/feed/block"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	"github.com/theQRL/qrysm/v4/beacon-chain/db/kv"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/features"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/time/slots"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// zond1DataNotification is a latch to stop flooding logs with the same warning.
var zond1DataNotification bool

const (
	// CouldNotDecodeBlock means that a signed beacon block couldn't be created from the block present in the request.
	CouldNotDecodeBlock = "Could not decode block"
	zond1dataTimeout    = 2 * time.Second
)

// GetBeaconBlock is called by a proposer during its assigned slot to request a block to sign
// by passing in the slot and the signed randao reveal of the slot.
func (vs *Server) GetBeaconBlock(ctx context.Context, req *zondpb.BlockRequest) (*zondpb.GenericBeaconBlock, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.GetBeaconBlock")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("slot", int64(req.Slot)))

	t, err := slots.ToTime(uint64(vs.TimeFetcher.GenesisTime().Unix()), req.Slot)
	if err != nil {
		log.WithError(err).Error("Could not convert slot to time")
	}
	log.WithFields(logrus.Fields{
		"slot":               req.Slot,
		"sinceSlotStartTime": time.Since(t),
	}).Info("Begin building block")

	// A syncing validator should not produce a block.
	if vs.SyncChecker.Syncing() {
		return nil, status.Error(codes.Unavailable, "Syncing to latest head, not ready to respond")
	}

	// process attestations and update head in forkchoice
	vs.ForkchoiceFetcher.UpdateHead(ctx, vs.TimeFetcher.CurrentSlot())
	headRoot := vs.ForkchoiceFetcher.CachedHeadRoot()
	parentRoot := vs.ForkchoiceFetcher.GetProposerHead()
	if parentRoot != headRoot {
		blockchain.LateBlockAttemptedReorgCount.Inc()
	}

	// An optimistic validator MUST NOT produce a block (i.e., sign across the DOMAIN_BEACON_PROPOSER domain).
	if err := vs.optimisticStatus(ctx); err != nil {
		return nil, status.Errorf(codes.Unavailable, "Validator is not ready to propose: %v", err)
	}

	sBlk, err := getEmptyBlock(req.Slot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not prepare block: %v", err)
	}
	head, err := vs.HeadFetcher.HeadState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state: %v", err)
	}
	head, err = transition.ProcessSlotsUsingNextSlotCache(ctx, head, parentRoot[:], req.Slot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not process slots up to %d: %v", req.Slot, err)
	}

	// Set slot, graffiti, randao reveal, and parent root.
	sBlk.SetSlot(req.Slot)
	sBlk.SetGraffiti(req.Graffiti)
	sBlk.SetRandaoReveal(req.RandaoReveal)
	sBlk.SetParentRoot(parentRoot[:])

	// Set proposer index.
	idx, err := helpers.BeaconProposerIndex(ctx, head)
	if err != nil {
		return nil, fmt.Errorf("could not calculate proposer index %v", err)
	}
	sBlk.SetProposerIndex(idx)

	if features.Get().BuildBlockParallel {
		if err := vs.BuildBlockParallel(ctx, sBlk, head); err != nil {
			return nil, errors.Wrap(err, "could not build block in parallel")
		}
	} else {
		// Set zond1 data.
		zond1Data, err := vs.zond1DataMajorityVote(ctx, head)
		if err != nil {
			zond1Data = &zondpb.Zond1Data{DepositRoot: params.BeaconConfig().ZeroHash[:], BlockHash: params.BeaconConfig().ZeroHash[:]}
			log.WithError(err).Error("Could not get zond1data")
		}
		sBlk.SetZond1Data(zond1Data)

		// Set deposit and attestation.
		deposits, atts, err := vs.packDepositsAndAttestations(ctx, head, zond1Data) // TODO: split attestations and deposits
		if err != nil {
			sBlk.SetDeposits([]*zondpb.Deposit{})
			sBlk.SetAttestations([]*zondpb.Attestation{})
			log.WithError(err).Error("Could not pack deposits and attestations")
		} else {
			sBlk.SetDeposits(deposits)
			sBlk.SetAttestations(atts)
		}

		// Set slashings.
		validProposerSlashings, validAttSlashings := vs.getSlashings(ctx, head)
		sBlk.SetProposerSlashings(validProposerSlashings)
		sBlk.SetAttesterSlashings(validAttSlashings)

		// Set exits.
		sBlk.SetVoluntaryExits(vs.getExits(head, req.Slot))

		// Set sync aggregate. New in Altair.
		vs.setSyncAggregate(ctx, sBlk)

		// Get local and builder (if enabled) payloads. Set execution data. New in Bellatrix.
		localPayload, err := vs.getLocalPayload(ctx, sBlk.Block(), head)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not get local payload: %v", err)
		}
		builderPayload, err := vs.getBuilderPayload(ctx, sBlk.Block().Slot(), sBlk.Block().ProposerIndex())
		if err != nil {
			builderGetPayloadMissCount.Inc()
			log.WithError(err).Error("Could not get builder payload")
		}
		if err := setExecutionData(ctx, sBlk, localPayload, builderPayload); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not set execution data: %v", err)
		}

		// Set dilithium to execution change. New in Capella.
		vs.setDilithiumToExecData(sBlk, head)
	}

	sr, err := vs.computeStateRoot(ctx, sBlk)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not compute state root: %v", err)
	}
	sBlk.SetStateRoot(sr)

	log.WithFields(logrus.Fields{
		"slot":               req.Slot,
		"sinceSlotStartTime": time.Since(t),
		"validator":          sBlk.Block().ProposerIndex(),
	}).Info("Finished building block")

	pb, err := sBlk.Block().Proto()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not convert block to proto: %v", err)
	}

	if sBlk.IsBlinded() {
		return &zondpb.GenericBeaconBlock{Block: &zondpb.GenericBeaconBlock_BlindedCapella{BlindedCapella: pb.(*zondpb.BlindedBeaconBlock)}}, nil
	}
	return &zondpb.GenericBeaconBlock{Block: &zondpb.GenericBeaconBlock_Capella{Capella: pb.(*zondpb.BeaconBlock)}}, nil
}

func (vs *Server) BuildBlockParallel(ctx context.Context, sBlk interfaces.SignedBeaconBlock, head state.BeaconState) error {
	// Build consensus fields in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Set zond1 data.
		zond1Data, err := vs.zond1DataMajorityVote(ctx, head)
		if err != nil {
			zond1Data = &zondpb.Zond1Data{DepositRoot: params.BeaconConfig().ZeroHash[:], BlockHash: params.BeaconConfig().ZeroHash[:]}
			log.WithError(err).Error("Could not get zond1data")
		}
		sBlk.SetZond1Data(zond1Data)

		// Set deposit and attestation.
		deposits, atts, err := vs.packDepositsAndAttestations(ctx, head, zond1Data) // TODO: split attestations and deposits
		if err != nil {
			sBlk.SetDeposits([]*zondpb.Deposit{})
			sBlk.SetAttestations([]*zondpb.Attestation{})
			log.WithError(err).Error("Could not pack deposits and attestations")
		} else {
			sBlk.SetDeposits(deposits)
			sBlk.SetAttestations(atts)
		}

		// Set slashings.
		validProposerSlashings, validAttSlashings := vs.getSlashings(ctx, head)
		sBlk.SetProposerSlashings(validProposerSlashings)
		sBlk.SetAttesterSlashings(validAttSlashings)

		// Set exits.
		sBlk.SetVoluntaryExits(vs.getExits(head, sBlk.Block().Slot()))

		// Set sync aggregate. New in Altair.
		vs.setSyncAggregate(ctx, sBlk)

		// Set dilithium to execution change. New in Capella.
		vs.setDilithiumToExecData(sBlk, head)
	}()

	localPayload, err := vs.getLocalPayload(ctx, sBlk.Block(), head)
	if err != nil {
		return status.Errorf(codes.Internal, "Could not get local payload: %v", err)
	}

	builderPayload, err := vs.getBuilderPayload(ctx, sBlk.Block().Slot(), sBlk.Block().ProposerIndex())
	if err != nil {
		builderGetPayloadMissCount.Inc()
		log.WithError(err).Error("Could not get builder payload")
	}

	if err := setExecutionData(ctx, sBlk, localPayload, builderPayload); err != nil {
		return status.Errorf(codes.Internal, "Could not set execution data: %v", err)
	}

	wg.Wait() // Wait until block is built via consensus and execution fields.

	return nil
}

// ProposeBeaconBlock is called by a proposer during its assigned slot to create a block in an attempt
// to get it processed by the beacon node as the canonical head.
func (vs *Server) ProposeBeaconBlock(ctx context.Context, req *zondpb.GenericSignedBeaconBlock) (*zondpb.ProposeResponse, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.ProposeBeaconBlock")
	defer span.End()
	blk, err := blocks.NewSignedBeaconBlock(req.Block)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s: %v", CouldNotDecodeBlock, err)
	}
	return vs.proposeGenericBeaconBlock(ctx, blk)
}

// PrepareBeaconProposer caches and updates the fee recipient for the given proposer.
func (vs *Server) PrepareBeaconProposer(
	ctx context.Context, request *zondpb.PrepareBeaconProposerRequest,
) (*emptypb.Empty, error) {
	ctx, span := trace.StartSpan(ctx, "validator.PrepareBeaconProposer")
	defer span.End()
	var feeRecipients []common.Address
	var validatorIndices []primitives.ValidatorIndex

	newRecipients := make([]*zondpb.PrepareBeaconProposerRequest_FeeRecipientContainer, 0, len(request.Recipients))
	for _, r := range request.Recipients {
		f, err := vs.BeaconDB.FeeRecipientByValidatorID(ctx, r.ValidatorIndex)
		switch {
		case errors.Is(err, kv.ErrNotFoundFeeRecipient):
			newRecipients = append(newRecipients, r)
		case err != nil:
			return nil, status.Errorf(codes.Internal, "Could not get fee recipient by validator index: %v", err)
		default:
			if common.BytesToAddress(r.FeeRecipient) != f {
				newRecipients = append(newRecipients, r)
			}
		}
	}
	if len(newRecipients) == 0 {
		return &emptypb.Empty{}, nil
	}

	for _, recipientContainer := range newRecipients {
		recipient := hexutil.Encode(recipientContainer.FeeRecipient)
		if !common.IsHexAddress(recipient) {
			return nil, status.Errorf(codes.InvalidArgument, fmt.Sprintf("Invalid fee recipient address: %v", recipient))
		}
		feeRecipients = append(feeRecipients, common.BytesToAddress(recipientContainer.FeeRecipient))
		validatorIndices = append(validatorIndices, recipientContainer.ValidatorIndex)
	}
	if err := vs.BeaconDB.SaveFeeRecipientsByValidatorIDs(ctx, validatorIndices, feeRecipients); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not save fee recipients: %v", err)
	}
	log.WithFields(logrus.Fields{
		"validatorIndices": validatorIndices,
	}).Info("Updated fee recipient addresses for validator indices")
	return &emptypb.Empty{}, nil
}

// GetFeeRecipientByPubKey returns a fee recipient from the beacon node's settings or db based on a given public key
func (vs *Server) GetFeeRecipientByPubKey(ctx context.Context, request *zondpb.FeeRecipientByPubKeyRequest) (*zondpb.FeeRecipientByPubKeyResponse, error) {
	ctx, span := trace.StartSpan(ctx, "validator.GetFeeRecipientByPublicKey")
	defer span.End()
	if request == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request was empty")
	}

	resp, err := vs.ValidatorIndex(ctx, &zondpb.ValidatorIndexRequest{PublicKey: request.PublicKey})
	if err != nil {
		if strings.Contains(err.Error(), "Could not find validator index") {
			return &zondpb.FeeRecipientByPubKeyResponse{
				FeeRecipient: params.BeaconConfig().DefaultFeeRecipient.Bytes(),
			}, nil
		} else {
			log.WithError(err).Error("An error occurred while retrieving validator index")
			return nil, err
		}
	}
	address, err := vs.BeaconDB.FeeRecipientByValidatorID(ctx, resp.GetIndex())
	if err != nil {
		if errors.Is(err, kv.ErrNotFoundFeeRecipient) {
			return &zondpb.FeeRecipientByPubKeyResponse{
				FeeRecipient: params.BeaconConfig().DefaultFeeRecipient.Bytes(),
			}, nil
		} else {
			log.WithError(err).Error("An error occurred while retrieving fee recipient from db")
			return nil, status.Errorf(codes.Internal, err.Error())
		}
	}
	return &zondpb.FeeRecipientByPubKeyResponse{
		FeeRecipient: address.Bytes(),
	}, nil
}

func (vs *Server) proposeGenericBeaconBlock(ctx context.Context, blk interfaces.SignedBeaconBlock) (*zondpb.ProposeResponse, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.proposeGenericBeaconBlock")
	defer span.End()

	unblinder, err := newUnblinder(blk, vs.BlockBuilder)
	if err != nil {
		return nil, errors.Wrap(err, "could not create unblinder")
	}
	blk, err = unblinder.unblindBuilderBlock(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not unblind builder block")
	}

	// Broadcast the new block to the network.
	blkPb, err := blk.Proto()
	if err != nil {
		return nil, errors.Wrap(err, "could not get protobuf block")
	}
	if err := vs.P2P.Broadcast(ctx, blkPb); err != nil {
		return nil, fmt.Errorf("could not broadcast block: %v", err)
	}
	root, err := blk.Block().HashTreeRoot()
	if err != nil {
		return nil, fmt.Errorf("could not tree hash block: %v", err)
	}

	log.WithFields(logrus.Fields{
		"blockRoot": hex.EncodeToString(root[:]),
	}).Debug("Broadcasting block")

	if err := vs.BlockReceiver.ReceiveBlock(ctx, blk, root); err != nil {
		return nil, fmt.Errorf("could not process beacon block: %v", err)
	}

	log.WithField("slot", blk.Block().Slot()).Debugf(
		"Block proposal received via RPC")
	vs.BlockNotifier.BlockFeed().Send(&feed.Event{
		Type: blockfeed.ReceivedBlock,
		Data: &blockfeed.ReceivedBlockData{SignedBlock: blk},
	})

	return &zondpb.ProposeResponse{
		BlockRoot: root[:],
	}, nil
}

// computeStateRoot computes the state root after a block has been processed through a state transition and
// returns it to the validator client.
func (vs *Server) computeStateRoot(ctx context.Context, block interfaces.ReadOnlySignedBeaconBlock) ([]byte, error) {
	beaconState, err := vs.StateGen.StateByRoot(ctx, block.Block().ParentRoot())
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve beacon state")
	}
	root, err := transition.CalculateStateRoot(
		ctx,
		beaconState,
		block,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "could not calculate state root at slot %d", beaconState.Slot())
	}

	log.WithField("beaconStateRoot", fmt.Sprintf("%#x", root)).Debugf("Computed state root")
	return root[:], nil
}

// SubmitValidatorRegistrations submits validator registrations.
func (vs *Server) SubmitValidatorRegistrations(ctx context.Context, reg *zondpb.SignedValidatorRegistrationsV1) (*emptypb.Empty, error) {
	if vs.BlockBuilder == nil || !vs.BlockBuilder.Configured() {
		return &emptypb.Empty{}, status.Errorf(codes.InvalidArgument, "Could not register block builder: %v", builder.ErrNoBuilder)
	}

	if err := vs.BlockBuilder.RegisterValidator(ctx, reg.Messages); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Could not register block builder: %v", err)
	}

	return &emptypb.Empty{}, nil
}