package migration

import (
	"github.com/pkg/errors"
	qrlpb "github.com/theQRL/qrysm/proto/qrl/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"google.golang.org/protobuf/proto"
)

// CapellaToV1Alpha1SignedBlock converts a v1 SignedBeaconBlockCapella proto to a v1alpha1 proto.
func CapellaToV1Alpha1SignedBlock(capellaBlk *qrlpb.SignedBeaconBlockCapella) (*qrysmpb.SignedBeaconBlockCapella, error) {
	marshaledBlk, err := proto.Marshal(capellaBlk)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1alpha1Block := &qrysmpb.SignedBeaconBlockCapella{}
	if err := proto.Unmarshal(marshaledBlk, v1alpha1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1alpha1Block, nil
}

// BlindedCapellaToV1Alpha1SignedBlock converts a v1 SignedBlindedBeaconBlockCapella proto to a v1alpha1 proto.
func BlindedCapellaToV1Alpha1SignedBlock(capellaBlk *qrlpb.SignedBlindedBeaconBlockCapella) (*qrysmpb.SignedBlindedBeaconBlockCapella, error) {
	marshaledBlk, err := proto.Marshal(capellaBlk)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1alpha1Block := &qrysmpb.SignedBlindedBeaconBlockCapella{}
	if err := proto.Unmarshal(marshaledBlk, v1alpha1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1alpha1Block, nil
}
