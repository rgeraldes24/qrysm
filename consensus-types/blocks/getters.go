package blocks

import (
	"fmt"

	"github.com/pkg/errors"
	ssz "github.com/prysmaticlabs/fastssz"
	log "github.com/sirupsen/logrus"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	consensus_types "github.com/theQRL/qrysm/consensus-types"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/runtime/version"
)

// BeaconBlockIsNil checks if any composite field of input signed beacon block is nil.
// Access to these nil fields will result in run time panic,
// it is recommended to run these checks as first line of defense.
func BeaconBlockIsNil(b interfaces.ReadOnlySignedBeaconBlock) error {
	if b == nil || b.IsNil() {
		return ErrNilSignedBeaconBlock
	}
	return nil
}

// Signature returns the respective block signature.
func (b *SignedBeaconBlock) Signature() [field_params.MLDSA87SignatureLength]byte {
	return b.signature
}

// Block returns the underlying beacon block object.
func (b *SignedBeaconBlock) Block() interfaces.ReadOnlyBeaconBlock {
	return b.block
}

// IsNil checks if the underlying beacon block is nil.
func (b *SignedBeaconBlock) IsNil() bool {
	return b == nil || b.block.IsNil()
}

// Copy performs a deep copy of the signed beacon block object.
func (b *SignedBeaconBlock) Copy() (interfaces.ReadOnlySignedBeaconBlock, error) {
	if b == nil {
		return nil, nil
	}

	pb, err := b.Proto()
	if err != nil {
		return nil, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			cp := qrysmpb.CopySignedBlindedBeaconBlockCapella(pb.(*qrysmpb.SignedBlindedBeaconBlockCapella))
			return initBlindedSignedBlockFromProtoCapella(cp)
		}
		cp := qrysmpb.CopySignedBeaconBlockCapella(pb.(*qrysmpb.SignedBeaconBlockCapella))
		return initSignedBlockFromProtoCapella(cp)
	default:
		return nil, errIncorrectBlockVersion
	}
}

// PbGenericBlock returns a generic signed beacon block.
func (b *SignedBeaconBlock) PbGenericBlock() (*qrysmpb.GenericSignedBeaconBlock, error) {
	pb, err := b.Proto()
	if err != nil {
		return nil, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return &qrysmpb.GenericSignedBeaconBlock{
				Block: &qrysmpb.GenericSignedBeaconBlock_BlindedCapella{BlindedCapella: pb.(*qrysmpb.SignedBlindedBeaconBlockCapella)},
			}, nil
		}
		return &qrysmpb.GenericSignedBeaconBlock{
			Block: &qrysmpb.GenericSignedBeaconBlock_Capella{Capella: pb.(*qrysmpb.SignedBeaconBlockCapella)},
		}, nil
	default:
		return nil, errIncorrectBlockVersion
	}
}

// PbCapellaBlock returns the underlying protobuf object.
func (b *SignedBeaconBlock) PbCapellaBlock() (*qrysmpb.SignedBeaconBlockCapella, error) {
	if b.IsBlinded() {
		return nil, consensus_types.ErrNotSupported("PbCapellaBlock", b.version)
	}
	pb, err := b.Proto()
	if err != nil {
		return nil, err
	}
	return pb.(*qrysmpb.SignedBeaconBlockCapella), nil
}

// PbBlindedCapellaBlock returns the underlying protobuf object.
func (b *SignedBeaconBlock) PbBlindedCapellaBlock() (*qrysmpb.SignedBlindedBeaconBlockCapella, error) {
	if !b.IsBlinded() {
		return nil, consensus_types.ErrNotSupported("PbBlindedCapellaBlock", b.version)
	}
	pb, err := b.Proto()
	if err != nil {
		return nil, err
	}
	return pb.(*qrysmpb.SignedBlindedBeaconBlockCapella), nil
}

// ToBlinded converts a non-blinded block to its blinded equivalent.
func (b *SignedBeaconBlock) ToBlinded() (interfaces.ReadOnlySignedBeaconBlock, error) {
	if b.IsBlinded() {
		return b, nil
	}
	if b.block.IsNil() {
		return nil, errors.New("cannot convert nil block to blinded format")
	}
	payload, err := b.block.Body().Execution()
	if err != nil {
		return nil, err
	}

	switch p := payload.Proto().(type) {
	case *enginev1.ExecutionPayloadCapella:
		header, err := PayloadToHeaderCapella(payload)
		if err != nil {
			return nil, err
		}
		return initBlindedSignedBlockFromProtoCapella(
			&qrysmpb.SignedBlindedBeaconBlockCapella{
				Block: &qrysmpb.BlindedBeaconBlockCapella{
					Slot:          b.block.slot,
					ProposerIndex: b.block.proposerIndex,
					ParentRoot:    b.block.parentRoot[:],
					StateRoot:     b.block.stateRoot[:],
					Body: &qrysmpb.BlindedBeaconBlockBodyCapella{
						RandaoReveal:           b.block.body.randaoReveal[:],
						ExecutionData:          b.block.body.executionData,
						Graffiti:               b.block.body.graffiti[:],
						ProposerSlashings:      b.block.body.proposerSlashings,
						AttesterSlashings:      b.block.body.attesterSlashings,
						Attestations:           b.block.body.attestations,
						Deposits:               b.block.body.deposits,
						VoluntaryExits:         b.block.body.voluntaryExits,
						SyncAggregate:          b.block.body.syncAggregate,
						ExecutionPayloadHeader: header,
					},
				},
				Signature: b.signature[:],
			})
	default:
		return nil, fmt.Errorf("%T is not an execution payload header", p)
	}
}

// Version of the underlying protobuf object.
func (b *SignedBeaconBlock) Version() int {
	return b.version
}

// IsBlinded metadata on whether a block is blinded
func (b *SignedBeaconBlock) IsBlinded() bool {
	return b.block.body.isBlinded
}

// ValueInShor metadata on the payload value returned by the builder. Value is 0 by default if local.
func (b *SignedBeaconBlock) ValueInShor() uint64 {
	exec, err := b.block.body.Execution()
	if err != nil {
		log.WithError(err).Warn("failed to retrieve execution payload")
		return 0
	}
	val, err := exec.ValueInShor()
	if err != nil {
		log.WithError(err).Warn("failed to retrieve value in shor")
		return 0
	}
	return val
}

// Header converts the underlying protobuf object from blinded block to header format.
func (b *SignedBeaconBlock) Header() (*qrysmpb.SignedBeaconBlockHeader, error) {
	if b.IsNil() {
		return nil, errNilBlock
	}
	root, err := b.block.body.HashTreeRoot()
	if err != nil {
		return nil, errors.Wrapf(err, "could not hash block body")
	}

	return &qrysmpb.SignedBeaconBlockHeader{
		Header: &qrysmpb.BeaconBlockHeader{
			Slot:          b.block.slot,
			ProposerIndex: b.block.proposerIndex,
			ParentRoot:    b.block.parentRoot[:],
			StateRoot:     b.block.stateRoot[:],
			BodyRoot:      root[:],
		},
		Signature: b.signature[:],
	}, nil
}

// MarshalSSZ marshals the signed beacon block to its relevant ssz form.
func (b *SignedBeaconBlock) MarshalSSZ() ([]byte, error) {
	pb, err := b.Proto()
	if err != nil {
		return []byte{}, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return pb.(*qrysmpb.SignedBlindedBeaconBlockCapella).MarshalSSZ()
		}
		return pb.(*qrysmpb.SignedBeaconBlockCapella).MarshalSSZ()
	default:
		return []byte{}, errIncorrectBlockVersion
	}
}

// MarshalSSZTo marshals the signed beacon block's ssz
// form to the provided byte buffer.
func (b *SignedBeaconBlock) MarshalSSZTo(dst []byte) ([]byte, error) {
	pb, err := b.Proto()
	if err != nil {
		return []byte{}, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return pb.(*qrysmpb.SignedBlindedBeaconBlockCapella).MarshalSSZTo(dst)
		}
		return pb.(*qrysmpb.SignedBeaconBlockCapella).MarshalSSZTo(dst)
	default:
		return []byte{}, errIncorrectBlockVersion
	}
}

// SizeSSZ returns the size of the serialized signed block
//
// WARNING: This function panics. It is required to change the signature
// of fastssz's SizeSSZ() interface function to avoid panicking.
// Changing the signature causes very problematic issues with wealdtech deps.
// For the time being panicking is preferable.
func (b *SignedBeaconBlock) SizeSSZ() int {
	pb, err := b.Proto()
	if err != nil {
		panic(err)
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return pb.(*qrysmpb.SignedBlindedBeaconBlockCapella).SizeSSZ()
		}
		return pb.(*qrysmpb.SignedBeaconBlockCapella).SizeSSZ()
	default:
		panic(incorrectBlockVersion)
	}
}

// UnmarshalSSZ unmarshals the signed beacon block from its relevant ssz form.
func (b *SignedBeaconBlock) UnmarshalSSZ(buf []byte) error {
	var newBlock *SignedBeaconBlock
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			pb := &qrysmpb.SignedBlindedBeaconBlockCapella{}
			if err := pb.UnmarshalSSZ(buf); err != nil {
				return err
			}
			var err error
			newBlock, err = initBlindedSignedBlockFromProtoCapella(pb)
			if err != nil {
				return err
			}
		} else {
			pb := &qrysmpb.SignedBeaconBlockCapella{}
			if err := pb.UnmarshalSSZ(buf); err != nil {
				return err
			}
			var err error
			newBlock, err = initSignedBlockFromProtoCapella(pb)
			if err != nil {
				return err
			}
		}
	default:
		return errIncorrectBlockVersion
	}
	*b = *newBlock
	return nil
}

// Slot returns the respective slot of the block.
func (b *BeaconBlock) Slot() primitives.Slot {
	return b.slot
}

// ProposerIndex returns the proposer index of the beacon block.
func (b *BeaconBlock) ProposerIndex() primitives.ValidatorIndex {
	return b.proposerIndex
}

// ParentRoot returns the parent root of beacon block.
func (b *BeaconBlock) ParentRoot() [field_params.RootLength]byte {
	return b.parentRoot
}

// StateRoot returns the state root of the beacon block.
func (b *BeaconBlock) StateRoot() [field_params.RootLength]byte {
	return b.stateRoot
}

// Body returns the underlying block body.
func (b *BeaconBlock) Body() interfaces.ReadOnlyBeaconBlockBody {
	return b.body
}

// IsNil checks if the beacon block is nil.
func (b *BeaconBlock) IsNil() bool {
	return b == nil || b.Body().IsNil()
}

// IsBlinded checks if the beacon block is a blinded block.
func (b *BeaconBlock) IsBlinded() bool {
	return b.body.isBlinded
}

// Version of the underlying protobuf object.
func (b *BeaconBlock) Version() int {
	return b.version
}

// HashTreeRoot returns the ssz root of the block.
func (b *BeaconBlock) HashTreeRoot() ([field_params.RootLength]byte, error) {
	pb, err := b.Proto()
	if err != nil {
		return [field_params.RootLength]byte{}, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return pb.(*qrysmpb.BlindedBeaconBlockCapella).HashTreeRoot()
		}
		return pb.(*qrysmpb.BeaconBlockCapella).HashTreeRoot()
	default:
		return [field_params.RootLength]byte{}, errIncorrectBlockVersion
	}
}

// HashTreeRootWith ssz hashes the BeaconBlock object with a hasher.
func (b *BeaconBlock) HashTreeRootWith(h *ssz.Hasher) error {
	pb, err := b.Proto()
	if err != nil {
		return err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return pb.(*qrysmpb.BlindedBeaconBlockCapella).HashTreeRootWith(h)
		}
		return pb.(*qrysmpb.BeaconBlockCapella).HashTreeRootWith(h)
	default:
		return errIncorrectBlockVersion
	}
}

// MarshalSSZ marshals the block into its respective
// ssz form.
func (b *BeaconBlock) MarshalSSZ() ([]byte, error) {
	pb, err := b.Proto()
	if err != nil {
		return []byte{}, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return pb.(*qrysmpb.BlindedBeaconBlockCapella).MarshalSSZ()
		}
		return pb.(*qrysmpb.BeaconBlockCapella).MarshalSSZ()
	default:
		return []byte{}, errIncorrectBlockVersion
	}
}

// MarshalSSZTo marshals the beacon block's ssz
// form to the provided byte buffer.
func (b *BeaconBlock) MarshalSSZTo(dst []byte) ([]byte, error) {
	pb, err := b.Proto()
	if err != nil {
		return []byte{}, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return pb.(*qrysmpb.BlindedBeaconBlockCapella).MarshalSSZTo(dst)
		}
		return pb.(*qrysmpb.BeaconBlockCapella).MarshalSSZTo(dst)
	default:
		return []byte{}, errIncorrectBlockVersion
	}
}

// SizeSSZ returns the size of the serialized block.
//
// WARNING: This function panics. It is required to change the signature
// of fastssz's SizeSSZ() interface function to avoid panicking.
// Changing the signature causes very problematic issues with wealdtech deps.
// For the time being panicking is preferable.
func (b *BeaconBlock) SizeSSZ() int {
	pb, err := b.Proto()
	if err != nil {
		panic(err)
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return pb.(*qrysmpb.BlindedBeaconBlockCapella).SizeSSZ()
		}
		return pb.(*qrysmpb.BeaconBlockCapella).SizeSSZ()
	default:
		panic(incorrectBodyVersion)
	}
}

// UnmarshalSSZ unmarshals the beacon block from its relevant ssz form.
func (b *BeaconBlock) UnmarshalSSZ(buf []byte) error {
	var newBlock *BeaconBlock
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			pb := &qrysmpb.BlindedBeaconBlockCapella{}
			if err := pb.UnmarshalSSZ(buf); err != nil {
				return err
			}
			var err error
			newBlock, err = initBlindedBlockFromProtoCapella(pb)
			if err != nil {
				return err
			}
		} else {
			pb := &qrysmpb.BeaconBlockCapella{}
			if err := pb.UnmarshalSSZ(buf); err != nil {
				return err
			}
			var err error
			newBlock, err = initBlockFromProtoCapella(pb)
			if err != nil {
				return err
			}
		}
	default:
		return errIncorrectBlockVersion
	}
	*b = *newBlock
	return nil
}

// AsSignRequestObject returns the underlying sign request object.
func (b *BeaconBlock) AsSignRequestObject() (validatorpb.SignRequestObject, error) {
	pb, err := b.Proto()
	if err != nil {
		return nil, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			return &validatorpb.SignRequest_BlindedBlockCapella{BlindedBlockCapella: pb.(*qrysmpb.BlindedBeaconBlockCapella)}, nil
		}
		return &validatorpb.SignRequest_BlockCapella{BlockCapella: pb.(*qrysmpb.BeaconBlockCapella)}, nil
	default:
		return nil, errIncorrectBlockVersion
	}
}

func (b *BeaconBlock) Copy() (interfaces.ReadOnlyBeaconBlock, error) {
	if b == nil {
		return nil, nil
	}

	pb, err := b.Proto()
	if err != nil {
		return nil, err
	}
	switch b.version {
	case version.Capella:
		if b.IsBlinded() {
			cp := qrysmpb.CopyBlindedBeaconBlockCapella(pb.(*qrysmpb.BlindedBeaconBlockCapella))
			return initBlindedBlockFromProtoCapella(cp)
		}
		cp := qrysmpb.CopyBeaconBlockCapella(pb.(*qrysmpb.BeaconBlockCapella))
		return initBlockFromProtoCapella(cp)
	default:
		return nil, errIncorrectBlockVersion
	}
}

// IsNil checks if the block body is nil.
func (b *BeaconBlockBody) IsNil() bool {
	return b == nil
}

// RandaoReveal returns the randao reveal from the block body.
func (b *BeaconBlockBody) RandaoReveal() [field_params.MLDSA87SignatureLength]byte {
	return b.randaoReveal
}

// ExecutionData returns the execution data in the block.
func (b *BeaconBlockBody) ExecutionData() *qrysmpb.ExecutionData {
	return b.executionData
}

// Graffiti returns the graffiti in the block.
func (b *BeaconBlockBody) Graffiti() [field_params.RootLength]byte {
	return b.graffiti
}

// ProposerSlashings returns the proposer slashings in the block.
func (b *BeaconBlockBody) ProposerSlashings() []*qrysmpb.ProposerSlashing {
	return b.proposerSlashings
}

// AttesterSlashings returns the attester slashings in the block.
func (b *BeaconBlockBody) AttesterSlashings() []*qrysmpb.AttesterSlashing {
	return b.attesterSlashings
}

// Attestations returns the stored attestations in the block.
func (b *BeaconBlockBody) Attestations() []*qrysmpb.Attestation {
	return b.attestations
}

// Deposits returns the stored deposits in the block.
func (b *BeaconBlockBody) Deposits() []*qrysmpb.Deposit {
	return b.deposits
}

// VoluntaryExits returns the voluntary exits in the block.
func (b *BeaconBlockBody) VoluntaryExits() []*qrysmpb.SignedVoluntaryExit {
	return b.voluntaryExits
}

// SyncAggregate returns the sync aggregate in the block.
func (b *BeaconBlockBody) SyncAggregate() (*qrysmpb.SyncAggregate, error) {
	return b.syncAggregate, nil
}

// Execution returns the execution payload of the block body.
func (b *BeaconBlockBody) Execution() (interfaces.ExecutionData, error) {
	switch b.version {
	case version.Capella:
		if b.isBlinded {
			var ph *enginev1.ExecutionPayloadHeaderCapella
			var ok bool
			if b.executionPayloadHeader != nil {
				ph, ok = b.executionPayloadHeader.Proto().(*enginev1.ExecutionPayloadHeaderCapella)
				if !ok {
					return nil, errPayloadHeaderWrongType
				}
				return WrappedExecutionPayloadHeaderCapella(ph, 0)
			}
		}
		var p *enginev1.ExecutionPayloadCapella
		var ok bool
		if b.executionPayload != nil {
			p, ok = b.executionPayload.Proto().(*enginev1.ExecutionPayloadCapella)
			if !ok {
				return nil, errPayloadWrongType
			}
		}
		return WrappedExecutionPayloadCapella(p, 0)
	default:
		return nil, errIncorrectBlockVersion
	}
}

// HashTreeRoot returns the ssz root of the block body.
func (b *BeaconBlockBody) HashTreeRoot() ([field_params.RootLength]byte, error) {
	pb, err := b.Proto()
	if err != nil {
		return [field_params.RootLength]byte{}, err
	}
	switch b.version {
	case version.Capella:
		if b.isBlinded {
			return pb.(*qrysmpb.BlindedBeaconBlockBodyCapella).HashTreeRoot()
		}
		return pb.(*qrysmpb.BeaconBlockBodyCapella).HashTreeRoot()
	default:
		return [field_params.RootLength]byte{}, errIncorrectBodyVersion
	}
}
