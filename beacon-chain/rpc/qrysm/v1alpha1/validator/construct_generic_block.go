package validator

import (
	"fmt"

	"github.com/theQRL/qrysm/consensus-types/interfaces"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/runtime/version"
	"google.golang.org/protobuf/proto"
)

// constructGenericBeaconBlock constructs a `GenericBeaconBlock` based on the block version and other parameters.
func (vs *Server) constructGenericBeaconBlock(sBlk interfaces.SignedBeaconBlock) (*qrysmpb.GenericBeaconBlock, error) {
	if sBlk == nil || sBlk.Block() == nil {
		return nil, fmt.Errorf("block cannot be nil")
	}

	blockProto, err := sBlk.Block().Proto()
	if err != nil {
		return nil, err
	}

	isBlinded := sBlk.IsBlinded()
	payloadValue := sBlk.ValueInGplanck()

	switch sBlk.Version() {
	case version.Capella:
		return vs.constructCapellaBlock(blockProto, isBlinded, payloadValue), nil
	default:
		return nil, fmt.Errorf("unknown block version: %d", sBlk.Version())
	}
}

func (vs *Server) constructCapellaBlock(pb proto.Message, isBlinded bool, payloadValue uint64) *qrysmpb.GenericBeaconBlock {
	if isBlinded {
		return &qrysmpb.GenericBeaconBlock{Block: &qrysmpb.GenericBeaconBlock_BlindedCapella{BlindedCapella: pb.(*qrysmpb.BlindedBeaconBlockCapella)}, IsBlinded: true, PayloadValue: payloadValue}
	}
	return &qrysmpb.GenericBeaconBlock{Block: &qrysmpb.GenericBeaconBlock_Capella{Capella: pb.(*qrysmpb.BeaconBlockCapella)}, IsBlinded: false, PayloadValue: payloadValue}
}
