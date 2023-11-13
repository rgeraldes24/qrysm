package blocks

import (
	"github.com/pkg/errors"
	consensus_types "github.com/theQRL/qrysm/v4/consensus-types"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zond "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"google.golang.org/protobuf/proto"
)

// Proto converts the signed beacon block to a protobuf object.
func (b *SignedBeaconBlock) Proto() (proto.Message, error) {
	if b == nil {
		return nil, errNilBlock
	}

	blockMessage, err := b.block.Proto()
	if err != nil {
		return nil, err
	}

	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			var block *zond.BlindedBeaconBlock
			if blockMessage != nil {
				var ok bool
				block, ok = blockMessage.(*zond.BlindedBeaconBlock)
				if !ok {
					return nil, errIncorrectBlockVersion
				}
			}
			return &zond.SignedBlindedBeaconBlock{
				Block:     block,
				Signature: b.signature[:],
			}, nil
		}
		var block *zond.BeaconBlock
		if blockMessage != nil {
			var ok bool
			block, ok = blockMessage.(*zond.BeaconBlock)
			if !ok {
				return nil, errIncorrectBlockVersion
			}
		}
		return &zond.SignedBeaconBlock{
			Block:     block,
			Signature: b.signature[:],
		}, nil
	default:
		return nil, errors.New("unsupported signed beacon block version")
	}
}

// Proto converts the beacon block to a protobuf object.
func (b *BeaconBlock) Proto() (proto.Message, error) {
	if b == nil {
		return nil, nil
	}

	bodyMessage, err := b.body.Proto()
	if err != nil {
		return nil, err
	}

	switch b.version {
	case version.Phase0:
		var body *zond.BeaconBlockBody
		if bodyMessage != nil {
			var ok bool
			body, ok = bodyMessage.(*zond.BeaconBlockBody)
			if !ok {
				return nil, errIncorrectBodyVersion
			}
		}
		return &zond.BeaconBlock{
			Slot:          b.slot,
			ProposerIndex: b.proposerIndex,
			ParentRoot:    b.parentRoot[:],
			StateRoot:     b.stateRoot[:],
			Body:          body,
		}, nil
	case version.Capella:
		if b.IsBlinded() {
			var body *zond.BlindedBeaconBlockBody
			if bodyMessage != nil {
				var ok bool
				body, ok = bodyMessage.(*zond.BlindedBeaconBlockBody)
				if !ok {
					return nil, errIncorrectBodyVersion
				}
			}
			return &zond.BlindedBeaconBlock{
				Slot:          b.slot,
				ProposerIndex: b.proposerIndex,
				ParentRoot:    b.parentRoot[:],
				StateRoot:     b.stateRoot[:],
				Body:          body,
			}, nil
		}
		var body *zond.BeaconBlockBody
		if bodyMessage != nil {
			var ok bool
			body, ok = bodyMessage.(*zond.BeaconBlockBody)
			if !ok {
				return nil, errIncorrectBodyVersion
			}
		}
		return &zond.BeaconBlock{
			Slot:          b.slot,
			ProposerIndex: b.proposerIndex,
			ParentRoot:    b.parentRoot[:],
			StateRoot:     b.stateRoot[:],
			Body:          body,
		}, nil
	default:
		return nil, errors.New("unsupported beacon block version")
	}
}

// Proto converts the beacon block body to a protobuf object.
func (b *BeaconBlockBody) Proto() (proto.Message, error) {
	if b == nil {
		return nil, nil
	}

	switch b.version {
	case version.Capella:
		if b.isBlinded {
			var ph *enginev1.ExecutionPayloadHeader
			var ok bool
			if b.executionPayloadHeader != nil {
				ph, ok = b.executionPayloadHeader.Proto().(*enginev1.ExecutionPayloadHeader)
				if !ok {
					return nil, errPayloadHeaderWrongType
				}
			}
			return &zond.BlindedBeaconBlockBody{
				RandaoReveal:                b.randaoReveal[:],
				Zond1Data:                   b.zond1Data,
				Graffiti:                    b.graffiti[:],
				ProposerSlashings:           b.proposerSlashings,
				AttesterSlashings:           b.attesterSlashings,
				Attestations:                b.attestations,
				Deposits:                    b.deposits,
				VoluntaryExits:              b.voluntaryExits,
				SyncAggregate:               b.syncAggregate,
				ExecutionPayloadHeader:      ph,
				DilithiumToExecutionChanges: b.dilithiumToExecutionChanges,
			}, nil
		}
		var p *enginev1.ExecutionPayload
		var ok bool
		if b.executionPayload != nil {
			p, ok = b.executionPayload.Proto().(*enginev1.ExecutionPayload)
			if !ok {
				return nil, errPayloadWrongType
			}
		}
		return &zond.BeaconBlockBody{
			RandaoReveal:                b.randaoReveal[:],
			Zond1Data:                   b.zond1Data,
			Graffiti:                    b.graffiti[:],
			ProposerSlashings:           b.proposerSlashings,
			AttesterSlashings:           b.attesterSlashings,
			Attestations:                b.attestations,
			Deposits:                    b.deposits,
			VoluntaryExits:              b.voluntaryExits,
			SyncAggregate:               b.syncAggregate,
			ExecutionPayload:            p,
			DilithiumToExecutionChanges: b.dilithiumToExecutionChanges,
		}, nil
	default:
		return nil, errors.New("unsupported beacon block body version")
	}
}

func initSignedBlockFromProtoCapella(pb *zond.SignedBeaconBlock) (*SignedBeaconBlock, error) {
	if pb == nil {
		return nil, errNilBlock
	}

	block, err := initBlockFromProtoCapella(pb.Block)
	if err != nil {
		return nil, err
	}
	b := &SignedBeaconBlock{
		version:   version.Capella,
		block:     block,
		signature: bytesutil.ToBytes4595(pb.Signature),
	}
	return b, nil
}

func initBlindedSignedBlockFromProtoCapella(pb *zond.SignedBlindedBeaconBlock) (*SignedBeaconBlock, error) {
	if pb == nil {
		return nil, errNilBlock
	}

	block, err := initBlindedBlockFromProtoCapella(pb.Block)
	if err != nil {
		return nil, err
	}
	b := &SignedBeaconBlock{
		version:   version.Capella,
		block:     block,
		signature: bytesutil.ToBytes4595(pb.Signature),
	}
	return b, nil
}

func initBlockFromProtoPhase0(pb *zond.BeaconBlock) (*BeaconBlock, error) {
	if pb == nil {
		return nil, errNilBlock
	}

	body, err := initBlockBodyFromProtoPhase0(pb.Body)
	if err != nil {
		return nil, err
	}
	b := &BeaconBlock{
		version:       version.Phase0,
		slot:          pb.Slot,
		proposerIndex: pb.ProposerIndex,
		parentRoot:    bytesutil.ToBytes32(pb.ParentRoot),
		stateRoot:     bytesutil.ToBytes32(pb.StateRoot),
		body:          body,
	}
	return b, nil
}

func initBlockFromProtoCapella(pb *zond.BeaconBlock) (*BeaconBlock, error) {
	if pb == nil {
		return nil, errNilBlock
	}

	body, err := initBlockBodyFromProtoCapella(pb.Body)
	if err != nil {
		return nil, err
	}
	b := &BeaconBlock{
		version:       version.Capella,
		slot:          pb.Slot,
		proposerIndex: pb.ProposerIndex,
		parentRoot:    bytesutil.ToBytes32(pb.ParentRoot),
		stateRoot:     bytesutil.ToBytes32(pb.StateRoot),
		body:          body,
	}
	return b, nil
}

func initBlindedBlockFromProtoCapella(pb *zond.BlindedBeaconBlock) (*BeaconBlock, error) {
	if pb == nil {
		return nil, errNilBlock
	}

	body, err := initBlindedBlockBodyFromProtoCapella(pb.Body)
	if err != nil {
		return nil, err
	}
	b := &BeaconBlock{
		version:       version.Capella,
		slot:          pb.Slot,
		proposerIndex: pb.ProposerIndex,
		parentRoot:    bytesutil.ToBytes32(pb.ParentRoot),
		stateRoot:     bytesutil.ToBytes32(pb.StateRoot),
		body:          body,
	}
	return b, nil
}

func initBlockBodyFromProtoPhase0(pb *zond.BeaconBlockBody) (*BeaconBlockBody, error) {
	if pb == nil {
		return nil, errNilBlockBody
	}

	b := &BeaconBlockBody{
		version:           version.Phase0,
		isBlinded:         false,
		randaoReveal:      bytesutil.ToBytes4595(pb.RandaoReveal),
		zond1Data:         pb.Zond1Data,
		graffiti:          bytesutil.ToBytes32(pb.Graffiti),
		proposerSlashings: pb.ProposerSlashings,
		attesterSlashings: pb.AttesterSlashings,
		attestations:      pb.Attestations,
		deposits:          pb.Deposits,
		voluntaryExits:    pb.VoluntaryExits,
	}
	return b, nil
}

func initBlockBodyFromProtoCapella(pb *zond.BeaconBlockBody) (*BeaconBlockBody, error) {
	if pb == nil {
		return nil, errNilBlockBody
	}

	p, err := WrappedExecutionPayload(pb.ExecutionPayload, 0)
	// We allow the payload to be nil
	if err != nil && err != consensus_types.ErrNilObjectWrapped {
		return nil, err
	}
	b := &BeaconBlockBody{
		version:                     version.Capella,
		isBlinded:                   false,
		randaoReveal:                bytesutil.ToBytes4595(pb.RandaoReveal),
		zond1Data:                   pb.Zond1Data,
		graffiti:                    bytesutil.ToBytes32(pb.Graffiti),
		proposerSlashings:           pb.ProposerSlashings,
		attesterSlashings:           pb.AttesterSlashings,
		attestations:                pb.Attestations,
		deposits:                    pb.Deposits,
		voluntaryExits:              pb.VoluntaryExits,
		syncAggregate:               pb.SyncAggregate,
		executionPayload:            p,
		dilithiumToExecutionChanges: pb.DilithiumToExecutionChanges,
	}
	return b, nil
}

func initBlindedBlockBodyFromProtoCapella(pb *zond.BlindedBeaconBlockBody) (*BeaconBlockBody, error) {
	if pb == nil {
		return nil, errNilBlockBody
	}

	ph, err := WrappedExecutionPayloadHeader(pb.ExecutionPayloadHeader, 0)
	// We allow the payload to be nil
	if err != nil && err != consensus_types.ErrNilObjectWrapped {
		return nil, err
	}
	b := &BeaconBlockBody{
		version:                     version.Capella,
		isBlinded:                   true,
		randaoReveal:                bytesutil.ToBytes4595(pb.RandaoReveal),
		zond1Data:                   pb.Zond1Data,
		graffiti:                    bytesutil.ToBytes32(pb.Graffiti),
		proposerSlashings:           pb.ProposerSlashings,
		attesterSlashings:           pb.AttesterSlashings,
		attestations:                pb.Attestations,
		deposits:                    pb.Deposits,
		voluntaryExits:              pb.VoluntaryExits,
		syncAggregate:               pb.SyncAggregate,
		executionPayloadHeader:      ph,
		dilithiumToExecutionChanges: pb.DilithiumToExecutionChanges,
	}
	return b, nil
}
