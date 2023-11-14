package util

import (
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// NewBeaconBlockCapella creates a beacon block with minimum marshalable fields.
func NewBeaconBlock() *zondpb.SignedBeaconBlock {
	return HydrateSignedBeaconBlock(&zondpb.SignedBeaconBlock{})
}

// NewBlindedBeaconBlockCapella creates a blinded beacon block with minimum marshalable fields.
func NewBlindedBeaconBlock() *zondpb.SignedBlindedBeaconBlock {
	// TODO return HydrateV2SignedBlindedBeaconBlockCapella(&v2.SignedBlindedBeaconBlockCapella{})
	return HydrateSignedBlindedBeaconBlockCapella(&zondpb.SignedBlindedBeaconBlock{})
}
