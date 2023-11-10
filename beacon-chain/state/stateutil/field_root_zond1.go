package stateutil

import (
	"github.com/pkg/errors"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// Zond1Root computes the HashTreeRoot Merkleization of
// a BeaconBlockHeader struct according to the eth2
// Simple Serialize specification.
func Zond1Root(zond1Data *zondpb.Zond1Data) ([32]byte, error) {
	if zond1Data == nil {
		return [32]byte{}, errors.New("nil zond1 data")
	}
	return Zond1DataRootWithHasher(zond1Data)
}

// Zond1DataVotesRoot computes the HashTreeRoot Merkleization of
// a list of Zond1Data structs according to the eth2
// Simple Serialize specification.
func Zond1DataVotesRoot(zond1DataVotes []*zondpb.Zond1Data) ([32]byte, error) {
	return Zond1DatasRoot(zond1DataVotes)
}
