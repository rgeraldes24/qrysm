package beacon

import (
	"context"
	"time"

	"github.com/theQRL/qrysm/api/grpc"
	"github.com/theQRL/qrysm/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/beacon-chain/core/feed"
	"github.com/theQRL/qrysm/beacon-chain/core/feed/operation"
	"github.com/theQRL/qrysm/beacon-chain/core/transition"
	"github.com/theQRL/qrysm/beacon-chain/rpc/qrl/helpers"
	"github.com/theQRL/qrysm/config/features"
	"github.com/theQRL/qrysm/proto/migration"
	qrlpb "github.com/theQRL/qrysm/proto/qrl/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const broadcastMLDSA87ChangesRateLimit = 128

// ListPoolAttesterSlashings retrieves attester slashings known by the node but
// not necessarily incorporated into any block.
func (bs *Server) ListPoolAttesterSlashings(ctx context.Context, _ *emptypb.Empty) (*qrlpb.AttesterSlashingsPoolResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.ListPoolAttesterSlashings")
	defer span.End()

	headState, err := bs.ChainInfoFetcher.HeadStateReadOnly(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state: %v", err)
	}
	sourceSlashings := bs.SlashingsPool.PendingAttesterSlashings(ctx, headState, true /* return unlimited slashings */)

	slashings := make([]*qrlpb.AttesterSlashing, len(sourceSlashings))
	for i, s := range sourceSlashings {
		slashings[i] = migration.V1Alpha1AttSlashingToV1(s)
	}

	return &qrlpb.AttesterSlashingsPoolResponse{
		Data: slashings,
	}, nil
}

// SubmitAttesterSlashing submits AttesterSlashing object to node's pool and
// if passes validation node MUST broadcast it to network.
func (bs *Server) SubmitAttesterSlashing(ctx context.Context, req *qrlpb.AttesterSlashing) (*emptypb.Empty, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.SubmitAttesterSlashing")
	defer span.End()

	headState, err := bs.ChainInfoFetcher.HeadState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state: %v", err)
	}
	headState, err = transition.ProcessSlotsIfPossible(ctx, headState, req.Attestation_1.Data.Slot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not process slots: %v", err)
	}

	alphaSlashing := migration.V1AttSlashingToV1Alpha1(req)
	err = blocks.VerifyAttesterSlashing(ctx, headState, alphaSlashing)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid attester slashing: %v", err)
	}

	err = bs.SlashingsPool.InsertAttesterSlashing(ctx, headState, alphaSlashing)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not insert attester slashing into pool: %v", err)
	}
	if !features.Get().DisableBroadcastSlashings {
		if err := bs.Broadcaster.Broadcast(ctx, alphaSlashing); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not broadcast slashing object: %v", err)
		}
	}

	return &emptypb.Empty{}, nil
}

// ListPoolProposerSlashings retrieves proposer slashings known by the node
// but not necessarily incorporated into any block.
func (bs *Server) ListPoolProposerSlashings(ctx context.Context, _ *emptypb.Empty) (*qrlpb.ProposerSlashingPoolResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.ListPoolProposerSlashings")
	defer span.End()

	headState, err := bs.ChainInfoFetcher.HeadStateReadOnly(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state: %v", err)
	}
	sourceSlashings := bs.SlashingsPool.PendingProposerSlashings(ctx, headState, true /* return unlimited slashings */)

	slashings := make([]*qrlpb.ProposerSlashing, len(sourceSlashings))
	for i, s := range sourceSlashings {
		slashings[i] = migration.V1Alpha1ProposerSlashingToV1(s)
	}

	return &qrlpb.ProposerSlashingPoolResponse{
		Data: slashings,
	}, nil
}

// SubmitProposerSlashing submits AttesterSlashing object to node's pool and if
// passes validation node MUST broadcast it to network.
func (bs *Server) SubmitProposerSlashing(ctx context.Context, req *qrlpb.ProposerSlashing) (*emptypb.Empty, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.SubmitProposerSlashing")
	defer span.End()

	headState, err := bs.ChainInfoFetcher.HeadState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state: %v", err)
	}
	headState, err = transition.ProcessSlotsIfPossible(ctx, headState, req.SignedHeader_1.Message.Slot)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not process slots: %v", err)
	}

	alphaSlashing := migration.V1ProposerSlashingToV1Alpha1(req)
	err = blocks.VerifyProposerSlashing(headState, alphaSlashing)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid proposer slashing: %v", err)
	}

	err = bs.SlashingsPool.InsertProposerSlashing(ctx, headState, alphaSlashing)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not insert proposer slashing into pool: %v", err)
	}
	if !features.Get().DisableBroadcastSlashings {
		if err := bs.Broadcaster.Broadcast(ctx, alphaSlashing); err != nil {
			return nil, status.Errorf(codes.Internal, "Could not broadcast slashing object: %v", err)
		}
	}

	return &emptypb.Empty{}, nil
}

// SubmitSignedMLDSA87ToExecutionChanges submits said object to the node's pool
// if it passes validation the node must broadcast it to the network.
func (bs *Server) SubmitSignedMLDSA87ToExecutionChanges(ctx context.Context, req *qrlpb.SubmitMLDSA87ToExecutionChangesRequest) (*emptypb.Empty, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.SubmitSignedMLDSA87ToExecutionChanges")
	defer span.End()
	st, err := bs.ChainInfoFetcher.HeadStateReadOnly(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get head state: %v", err)
	}
	var failures []*helpers.SingleIndexedVerificationFailure
	var toBroadcast []*qrysmpb.SignedMLDSA87ToExecutionChange

	for i, change := range req.GetChanges() {
		alphaChange := migration.V1SignedMLDSA87ToExecutionChangeToV1Alpha1(change)
		_, err = blocks.ValidateMLDSA87ToExecutionChange(st, alphaChange)
		if err != nil {
			failures = append(failures, &helpers.SingleIndexedVerificationFailure{
				Index:   i,
				Message: "Could not validate SignedMLDSA87ToExecutionChange: " + err.Error(),
			})
			continue
		}
		if err := blocks.VerifyMLDSA87ChangeSignature(st, change); err != nil {
			failures = append(failures, &helpers.SingleIndexedVerificationFailure{
				Index:   i,
				Message: "Could not validate signature: " + err.Error(),
			})
			continue
		}
		bs.OperationNotifier.OperationFeed().Send(&feed.Event{
			Type: operation.MLDSA87ToExecutionChangeReceived,
			Data: &operation.MLDSA87ToExecutionChangeReceivedData{
				Change: alphaChange,
			},
		})
		bs.MLDSA87ChangesPool.InsertMLDSA87ToExecChange(alphaChange)
		toBroadcast = append(toBroadcast, alphaChange)
	}
	go bs.broadcastMLDSA87Changes(ctx, toBroadcast)
	if len(failures) > 0 {
		failuresContainer := &helpers.IndexedVerificationFailure{Failures: failures}
		err := grpc.AppendCustomErrorHeader(ctx, failuresContainer)
		if err != nil {
			return nil, status.Errorf(
				codes.InvalidArgument,
				"One or more MLDSA87ToExecutionChange failed validation. Could not prepare MLDSA87ToExecutionChange failure information: %v",
				err,
			)
		}
		return nil, status.Errorf(codes.InvalidArgument, "One or more MLDSA87ToExecutionChange failed validation")
	}
	return &emptypb.Empty{}, nil
}

// broadcastMLDSA87Batch broadcasts the first `broadcastMLDSA87ChangesRateLimit` messages from the slice pointed to by ptr.
// It validates the messages again because they could have been invalidated by being included in blocks since the last validation.
// It removes the messages from the slice and modifies it in place.
func (bs *Server) broadcastMLDSA87Batch(ctx context.Context, ptr *[]*qrysmpb.SignedMLDSA87ToExecutionChange) {
	limit := broadcastMLDSA87ChangesRateLimit
	if len(*ptr) < broadcastMLDSA87ChangesRateLimit {
		limit = len(*ptr)
	}
	st, err := bs.ChainInfoFetcher.HeadStateReadOnly(ctx)
	if err != nil {
		log.WithError(err).Error("could not get head state")
		return
	}
	for _, ch := range (*ptr)[:limit] {
		if ch != nil {
			_, err := blocks.ValidateMLDSA87ToExecutionChange(st, ch)
			if err != nil {
				log.WithError(err).Error("could not validate ML-DSA-87 to execution change")
				continue
			}
			if err := bs.Broadcaster.Broadcast(ctx, ch); err != nil {
				log.WithError(err).Error("could not broadcast ML-DSA-87 to execution changes.")
			}
		}
	}
	*ptr = (*ptr)[limit:]
}

func (bs *Server) broadcastMLDSA87Changes(ctx context.Context, changes []*qrysmpb.SignedMLDSA87ToExecutionChange) {
	bs.broadcastMLDSA87Batch(ctx, &changes)
	if len(changes) == 0 {
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bs.broadcastMLDSA87Batch(ctx, &changes)
			if len(changes) == 0 {
				return
			}
		}
	}
}

// ListMLDSA87ToExecutionChanges retrieves ML-DSA-87 to execution changes known by the node but not necessarily incorporated into any block
func (bs *Server) ListMLDSA87ToExecutionChanges(ctx context.Context, _ *emptypb.Empty) (*qrlpb.MLDSA87ToExecutionChangesPoolResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.ListMLDSA87ToExecutionChanges")
	defer span.End()

	sourceChanges, err := bs.MLDSA87ChangesPool.PendingMLDSA87ToExecChanges()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not get ML-DSA-87 to execution changes: %v", err)
	}

	changes := make([]*qrlpb.SignedMLDSA87ToExecutionChange, len(sourceChanges))
	for i, ch := range sourceChanges {
		changes[i] = migration.V1Alpha1SignedMLDSA87ToExecChangeToV1(ch)
	}

	return &qrlpb.MLDSA87ToExecutionChangesPoolResponse{
		Data: changes,
	}, nil
}
