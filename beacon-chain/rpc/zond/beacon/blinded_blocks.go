package beacon

import (
	"context"
	"strings"

	"google.golang.org/grpc"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/api"
	"github.com/theQRL/qrysm/v4/beacon-chain/rpc/qrysm/v1alpha1/validator"
	rpchelpers "github.com/theQRL/qrysm/v4/beacon-chain/rpc/zond/helpers"
	"github.com/theQRL/qrysm/v4/config/params"
	consensus_types "github.com/theQRL/qrysm/v4/consensus-types"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/encoding/ssz/detect"
	"github.com/theQRL/qrysm/v4/network/forks"
	"github.com/theQRL/qrysm/v4/proto/migration"
	zond "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"go.opencensus.io/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetBlindedBlock retrieves blinded block for given block id.
func (bs *Server) GetBlindedBlock(ctx context.Context, req *zondpbv1.BlockRequest) (*zondpbv1.BlindedBlockResponse, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.GetBlindedBlock")
	defer span.End()

	blk, err := bs.Blocker.Block(ctx, req.BlockId)
	err = handleGetBlockError(blk, err)
	if err != nil {
		return nil, err
	}
	blkRoot, err := blk.Block().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrapf(err, "could not get block root")
	}
	if err := grpc.SetHeader(ctx, metadata.Pairs(api.VersionHeader, version.String(blk.Version()))); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not set "+api.VersionHeader+" header: %v", err)
	}

	result, err := getBlindedBlockPhase0(blk)
	if result != nil {
		result.Finalized = bs.FinalizationFetcher.IsFinalized(ctx, blkRoot)
		return result, nil
	}
	// ErrUnsupportedField means that we have another block type
	if !errors.Is(err, consensus_types.ErrUnsupportedField) {
		return nil, status.Errorf(codes.Internal, "Could not get blinded block: %v", err)
	}
	result, err = getBlindedBlockAltair(blk)
	if result != nil {
		result.Finalized = bs.FinalizationFetcher.IsFinalized(ctx, blkRoot)
		return result, nil
	}
	// ErrUnsupportedField means that we have another block type
	if !errors.Is(err, consensus_types.ErrUnsupportedField) {
		return nil, status.Errorf(codes.Internal, "Could not get blinded block: %v", err)
	}
	result, err = bs.getBlindedBlockBellatrix(ctx, blk)
	if result != nil {
		result.Finalized = bs.FinalizationFetcher.IsFinalized(ctx, blkRoot)
		return result, nil
	}
	// ErrUnsupportedField means that we have another block type
	if !errors.Is(err, consensus_types.ErrUnsupportedField) {
		return nil, status.Errorf(codes.Internal, "Could not get blinded block: %v", err)
	}
	result, err = bs.getBlindedBlockCapella(ctx, blk)
	if result != nil {
		result.Finalized = bs.FinalizationFetcher.IsFinalized(ctx, blkRoot)
		return result, nil
	}
	// ErrUnsupportedField means that we have another block type
	if !errors.Is(err, consensus_types.ErrUnsupportedField) {
		return nil, status.Errorf(codes.Internal, "Could not get blinded block: %v", err)
	}

	return nil, status.Errorf(codes.Internal, "Unknown block type %T", blk)
}

// GetBlindedBlockSSZ returns the SSZ-serialized version of the blinded beacon block for given block id.
func (bs *Server) GetBlindedBlockSSZ(ctx context.Context, req *zondpbv1.BlockRequest) (*zondpbv1.SSZContainer, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.GetBlindedBlockSSZ")
	defer span.End()

	blk, err := bs.Blocker.Block(ctx, req.BlockId)
	err = handleGetBlockError(blk, err)
	if err != nil {
		return nil, err
	}
	blkRoot, err := blk.Block().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrapf(err, "could not get block root")
	}

	result, err := getSSZBlockPhase0(blk)
	if result != nil {
		result.Finalized = bs.FinalizationFetcher.IsFinalized(ctx, blkRoot)
		return result, nil
	}
	// ErrUnsupportedField means that we have another block type
	if !errors.Is(err, consensus_types.ErrUnsupportedField) {
		return nil, status.Errorf(codes.Internal, "Could not get signed beacon block: %v", err)
	}
	result, err = getSSZBlockAltair(blk)
	if result != nil {
		result.Finalized = bs.FinalizationFetcher.IsFinalized(ctx, blkRoot)
		return result, nil
	}
	// ErrUnsupportedField means that we have another block type
	if !errors.Is(err, consensus_types.ErrUnsupportedField) {
		return nil, status.Errorf(codes.Internal, "Could not get signed beacon block: %v", err)
	}
	result, err = bs.getBlindedSSZBlockBellatrix(ctx, blk)
	if result != nil {
		result.Finalized = bs.FinalizationFetcher.IsFinalized(ctx, blkRoot)
		return result, nil
	}
	// ErrUnsupportedField means that we have another block type
	if !errors.Is(err, consensus_types.ErrUnsupportedField) {
		return nil, status.Errorf(codes.Internal, "Could not get signed beacon block: %v", err)
	}
	result, err = bs.getBlindedSSZBlockCapella(ctx, blk)
	if result != nil {
		result.Finalized = bs.FinalizationFetcher.IsFinalized(ctx, blkRoot)
		return result, nil
	}
	// ErrUnsupportedField means that we have another block type
	if !errors.Is(err, consensus_types.ErrUnsupportedField) {
		return nil, status.Errorf(codes.Internal, "Could not get signed beacon block: %v", err)
	}

	return nil, status.Errorf(codes.Internal, "Unknown block type %T", blk)
}

// SubmitBlindedBlock instructs the beacon node to use the components of the `SignedBlindedBeaconBlock` to construct
// and publish a `ReadOnlySignedBeaconBlock` by swapping out the `transactions_root` for the corresponding full list of `transactions`.
// The beacon node should broadcast a newly constructed `ReadOnlySignedBeaconBlock` to the beacon network,
// to be included in the beacon chain. The beacon node is not required to validate the signed
// `ReadOnlyBeaconBlock`, and a successful response (20X) only indicates that the broadcast has been
// successful. The beacon node is expected to integrate the new block into its state, and
// therefore validate the block internally, however blocks which fail the validation are still
// broadcast but a different status code is returned (202).
func (bs *Server) SubmitBlindedBlock(ctx context.Context, req *zondpbv1.SignedBlindedBeaconBlockContainer) (*emptypb.Empty, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.SubmitBlindedBlock")
	defer span.End()

	if err := rpchelpers.ValidateSyncGRPC(ctx, bs.SyncChecker, bs.HeadFetcher, bs.TimeFetcher, bs.OptimisticModeFetcher); err != nil {
		// We simply return the error because it's already a gRPC error.
		return nil, err
	}

	switch blkContainer := req.Message.(type) {
	case *zondpbv1.SignedBlindedBeaconBlockContainer_CapellaBlock:
		if err := bs.submitBlindedCapellaBlock(ctx, blkContainer.CapellaBlock, req.Signature); err != nil {
			return nil, err
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "Unsupported block container type %T", blkContainer)
	}

	return &emptypb.Empty{}, nil
}

// SubmitBlindedBlockSSZ instructs the beacon node to use the components of the `SignedBlindedBeaconBlock` to construct
// and publish a `ReadOnlySignedBeaconBlock` by swapping out the `transactions_root` for the corresponding full list of `transactions`.
// The beacon node should broadcast a newly constructed `ReadOnlySignedBeaconBlock` to the beacon network,
// to be included in the beacon chain. The beacon node is not required to validate the signed
// `ReadOnlyBeaconBlock`, and a successful response (20X) only indicates that the broadcast has been
// successful. The beacon node is expected to integrate the new block into its state, and
// therefore validate the block internally, however blocks which fail the validation are still
// broadcast but a different status code is returned (202).
//
// The provided block must be SSZ-serialized.
func (bs *Server) SubmitBlindedBlockSSZ(ctx context.Context, req *zondpbv1.SSZContainer) (*emptypb.Empty, error) {
	ctx, span := trace.StartSpan(ctx, "beacon.SubmitBlindedBlockSSZ")
	defer span.End()

	if err := rpchelpers.ValidateSyncGRPC(ctx, bs.SyncChecker, bs.HeadFetcher, bs.TimeFetcher, bs.OptimisticModeFetcher); err != nil {
		// We simply return the error because it's already a gRPC error.
		return nil, err
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not read"+api.VersionHeader+" header")
	}
	ver := md.Get(api.VersionHeader)
	if len(ver) == 0 {
		return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not read"+api.VersionHeader+" header")
	}
	schedule := forks.NewOrderedSchedule(params.BeaconConfig())
	forkVer, err := schedule.VersionForName(ver[0])
	if err != nil {
		return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not determine fork version: %v", err)
	}
	unmarshaler, err := detect.FromForkVersion(forkVer)
	if err != nil {
		return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not create unmarshaler: %v", err)
	}
	block, err := unmarshaler.UnmarshalBlindedBeaconBlock(req.Data)
	if err != nil {
		return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not unmarshal request data into block: %v", err)
	}

	switch forkVer {
	case bytesutil.ToBytes4(params.BeaconConfig().CapellaForkVersion):
		if !block.IsBlinded() {
			return nil, status.Error(codes.InvalidArgument, "Submitted block is not blinded")
		}
		b, err := block.PbBlindedCapellaBlock()
		if err != nil {
			return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not get proto block: %v", err)
		}
		_, err = bs.V1Alpha1ValidatorServer.ProposeBeaconBlock(ctx, &zond.GenericSignedBeaconBlock{
			Block: &zond.GenericSignedBeaconBlock_BlindedCapella{
				BlindedCapella: b,
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), validator.CouldNotDecodeBlock) {
				return &emptypb.Empty{}, status.Error(codes.InvalidArgument, err.Error())
			}
			return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not propose block: %v", err)
		}
		return &emptypb.Empty{}, nil
	case bytesutil.ToBytes4(params.BeaconConfig().BellatrixForkVersion):
		if !block.IsBlinded() {
			return nil, status.Error(codes.InvalidArgument, "Submitted block is not blinded")
		}
		b, err := block.PbBlindedBellatrixBlock()
		if err != nil {
			return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not get proto block: %v", err)
		}
		_, err = bs.V1Alpha1ValidatorServer.ProposeBeaconBlock(ctx, &zond.GenericSignedBeaconBlock{
			Block: &zond.GenericSignedBeaconBlock_BlindedBellatrix{
				BlindedBellatrix: b,
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), validator.CouldNotDecodeBlock) {
				return &emptypb.Empty{}, status.Error(codes.InvalidArgument, err.Error())
			}
			return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not propose block: %v", err)
		}
		return &emptypb.Empty{}, nil
	case bytesutil.ToBytes4(params.BeaconConfig().AltairForkVersion):
		b, err := block.PbAltairBlock()
		if err != nil {
			return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not get proto block: %v", err)
		}
		_, err = bs.V1Alpha1ValidatorServer.ProposeBeaconBlock(ctx, &zond.GenericSignedBeaconBlock{
			Block: &zond.GenericSignedBeaconBlock_Altair{
				Altair: b,
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), validator.CouldNotDecodeBlock) {
				return &emptypb.Empty{}, status.Error(codes.InvalidArgument, err.Error())
			}
			return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not propose block: %v", err)
		}
		return &emptypb.Empty{}, nil
	case bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion):
		b, err := block.PbPhase0Block()
		if err != nil {
			return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not get proto block: %v", err)
		}
		_, err = bs.V1Alpha1ValidatorServer.ProposeBeaconBlock(ctx, &zond.GenericSignedBeaconBlock{
			Block: &zond.GenericSignedBeaconBlock_Phase0{
				Phase0: b,
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), validator.CouldNotDecodeBlock) {
				return &emptypb.Empty{}, status.Error(codes.InvalidArgument, err.Error())
			}
			return &emptypb.Empty{}, status.Errorf(codes.Internal, "Could not propose block: %v", err)
		}
		return &emptypb.Empty{}, nil
	default:
		return &emptypb.Empty{}, status.Errorf(codes.InvalidArgument, "Unsupported fork %s", string(forkVer[:]))
	}
}

func (bs *Server) getBlindedBlockCapella(ctx context.Context, blk interfaces.ReadOnlySignedBeaconBlock) (*zondpbv1.BlindedBlockResponse, error) {
	capellaBlk, err := blk.PbCapellaBlock()
	if err != nil {
		// ErrUnsupportedField means that we have another block type
		if errors.Is(err, consensus_types.ErrUnsupportedField) {
			if blindedCapellaBlk, err := blk.PbBlindedCapellaBlock(); err == nil {
				if blindedCapellaBlk == nil {
					return nil, errNilBlock
				}
				v2Blk, err := migration.V1Alpha1ToV1BlindedBlock(blindedCapellaBlk.Block)
				if err != nil {
					return nil, errors.Wrapf(err, "Could not convert beacon block")
				}
				root, err := blk.Block().HashTreeRoot()
				if err != nil {
					return nil, errors.Wrapf(err, "could not get block root")
				}
				isOptimistic, err := bs.OptimisticModeFetcher.IsOptimisticForRoot(ctx, root)
				if err != nil {
					return nil, errors.Wrapf(err, "could not check if block is optimistic")
				}
				sig := blk.Signature()
				return &zondpbv1.BlindedBlockResponse{
					Version: zondpbv1.Version_CAPELLA,
					Data: &zondpbv1.SignedBlindedBeaconBlockContainer{
						Message:   &zondpbv1.SignedBlindedBeaconBlockContainer_CapellaBlock{CapellaBlock: v2Blk},
						Signature: sig[:],
					},
					ExecutionOptimistic: isOptimistic,
				}, nil
			}
			return nil, err
		}
		return nil, err
	}

	if capellaBlk == nil {
		return nil, errNilBlock
	}
	blindedBlkInterface, err := blk.ToBlinded()
	if err != nil {
		return nil, errors.Wrapf(err, "could not convert block to blinded block")
	}
	blindedCapellaBlock, err := blindedBlkInterface.PbBlindedCapellaBlock()
	if err != nil {
		return nil, errors.Wrapf(err, "could not get signed beacon block")
	}
	v2Blk, err := migration.V1Alpha1ToV1BlindedBlock(blindedCapellaBlock.Block)
	if err != nil {
		return nil, errors.Wrapf(err, "could not convert beacon block")
	}
	root, err := blk.Block().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrapf(err, "could not get block root")
	}
	isOptimistic, err := bs.OptimisticModeFetcher.IsOptimisticForRoot(ctx, root)
	if err != nil {
		return nil, errors.Wrapf(err, "could not check if block is optimistic")
	}
	sig := blk.Signature()
	return &zondpbv1.BlindedBlockResponse{
		Version: zondpbv1.Version_CAPELLA,
		Data: &zondpbv1.SignedBlindedBeaconBlockContainer{
			Message:   &zondpbv1.SignedBlindedBeaconBlockContainer_CapellaBlock{CapellaBlock: v2Blk},
			Signature: sig[:],
		},
		ExecutionOptimistic: isOptimistic,
	}, nil
}

func (bs *Server) getBlindedSSZBlockCapella(ctx context.Context, blk interfaces.ReadOnlySignedBeaconBlock) (*zondpbv1.SSZContainer, error) {
	capellaBlk, err := blk.PbCapellaBlock()
	if err != nil {
		// ErrUnsupportedField means that we have another block type
		if errors.Is(err, consensus_types.ErrUnsupportedField) {
			if blindedCapellaBlk, err := blk.PbBlindedCapellaBlock(); err == nil {
				if blindedCapellaBlk == nil {
					return nil, errNilBlock
				}
				v2Blk, err := migration.V1Alpha1ToV1BlindedBlock(blindedCapellaBlk.Block)
				if err != nil {
					return nil, errors.Wrapf(err, "could not get signed beacon block")
				}
				root, err := blk.Block().HashTreeRoot()
				if err != nil {
					return nil, errors.Wrapf(err, "could not get block root")
				}
				isOptimistic, err := bs.OptimisticModeFetcher.IsOptimisticForRoot(ctx, root)
				if err != nil {
					return nil, errors.Wrapf(err, "could not check if block is optimistic")
				}
				sig := blk.Signature()
				data := &zondpbv1.SignedBlindedBeaconBlock{
					Message:   v2Blk,
					Signature: sig[:],
				}
				sszData, err := data.MarshalSSZ()
				if err != nil {
					return nil, errors.Wrapf(err, "could not marshal block into SSZ")
				}
				return &zondpbv1.SSZContainer{
					Version:             zondpbv1.Version_CAPELLA,
					ExecutionOptimistic: isOptimistic,
					Data:                sszData,
				}, nil
			}
			return nil, err
		}
	}

	if capellaBlk == nil {
		return nil, errNilBlock
	}
	blindedBlkInterface, err := blk.ToBlinded()
	if err != nil {
		return nil, errors.Wrapf(err, "could not convert block to blinded block")
	}
	blindedCapellaBlock, err := blindedBlkInterface.PbBlindedCapellaBlock()
	if err != nil {
		return nil, errors.Wrapf(err, "could not get signed beacon block")
	}
	v2Blk, err := migration.V1Alpha1ToV1BlindedBlock(blindedCapellaBlock.Block)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get signed beacon block")
	}
	root, err := blk.Block().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrapf(err, "could not get block root")
	}
	isOptimistic, err := bs.OptimisticModeFetcher.IsOptimisticForRoot(ctx, root)
	if err != nil {
		return nil, errors.Wrapf(err, "could not check if block is optimistic")
	}
	sig := blk.Signature()
	data := &zondpbv1.SignedBlindedBeaconBlock{
		Message:   v2Blk,
		Signature: sig[:],
	}
	sszData, err := data.MarshalSSZ()
	if err != nil {
		return nil, errors.Wrapf(err, "could not marshal block into SSZ")
	}
	return &zondpbv1.SSZContainer{Version: zondpbv1.Version_CAPELLA, ExecutionOptimistic: isOptimistic, Data: sszData}, nil
}

func (bs *Server) submitBlindedCapellaBlock(ctx context.Context, blindedCapellaBlk *zondpbv1.BlindedBeaconBlock, sig []byte) error {
	b, err := migration.V1ToV1Alpha1SignedBlindedBlock(&zondpbv1.SignedBlindedBeaconBlock{
		Message:   blindedCapellaBlk,
		Signature: sig,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "Could not convert block: %v", err)
	}
	_, err = bs.V1Alpha1ValidatorServer.ProposeBeaconBlock(ctx, &zond.GenericSignedBeaconBlock{
		Block: &zond.GenericSignedBeaconBlock_BlindedCapella{
			BlindedCapella: b,
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), validator.CouldNotDecodeBlock) {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		return status.Errorf(codes.Internal, "Could not propose blinded block: %v", err)
	}
	return nil
}
