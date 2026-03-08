package testing

import (
	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// NewSignedBeaconBlockFromGeneric creates a signed beacon block
// from a protobuf generic signed beacon block.
func NewSignedBeaconBlockFromGeneric(gb *qrysmpb.GenericSignedBeaconBlock) (interfaces.ReadOnlySignedBeaconBlock, error) {
	if gb == nil {
		return nil, blocks.ErrNilObject
	}
	switch bb := gb.Block.(type) {
	case *qrysmpb.GenericSignedBeaconBlock_Capella:
		return blocks.NewSignedBeaconBlock(bb.Capella)
	case *qrysmpb.GenericSignedBeaconBlock_BlindedCapella:
		return blocks.NewSignedBeaconBlock(bb.BlindedCapella)
	default:
		return nil, errors.Wrapf(blocks.ErrUnsupportedSignedBeaconBlock, "unable to create block from type %T", gb)
	}
}
