package util

import (
	v1 "github.com/theQRL/qrysm/proto/qrl/v1"
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// NewBeaconBlockCapella creates a beacon block with minimum marshalable fields.
func NewBeaconBlockCapella() *zondpb.SignedBeaconBlockCapella {
	return HydrateSignedBeaconBlockCapella(&zondpb.SignedBeaconBlockCapella{})
}

// NewBlindedBeaconBlockCapella creates a blinded beacon block with minimum marshalable fields.
func NewBlindedBeaconBlockCapella() *zondpb.SignedBlindedBeaconBlockCapella {
	return HydrateSignedBlindedBeaconBlockCapella(&zondpb.SignedBlindedBeaconBlockCapella{})
}

// NewBlindedBeaconBlockCapellaV1 creates a blinded beacon block with minimum marshalable fields.
func NewBlindedBeaconBlockCapellaV1() *v1.SignedBlindedBeaconBlockCapella {
	return HydrateV1SignedBlindedBeaconBlockCapella(&v1.SignedBlindedBeaconBlockCapella{})
}
