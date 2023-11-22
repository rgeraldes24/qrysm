package util

import (
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// NewBeaconBlock creates a beacon block with minimum marshalable fields.
func NewBeaconBlock() *zondpb.SignedBeaconBlock {
	return HydrateSignedBeaconBlock(&zondpb.SignedBeaconBlock{})
}

// NewBlindedBeaconBlock creates a blinded beacon block with minimum marshalable fields.
func NewBlindedBeaconBlock() *zondpb.SignedBlindedBeaconBlock {
	return HydrateSignedBlindedBeaconBlock(&zondpb.SignedBlindedBeaconBlock{})
}
