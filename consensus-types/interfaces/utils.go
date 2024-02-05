package interfaces

import (
	"github.com/pkg/errors"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// SignedBeaconBlockHeaderFromBlockInterface function to retrieve signed block header from block.
func SignedBeaconBlockHeaderFromBlockInterface(sb ReadOnlySignedBeaconBlock) (*zondpb.SignedBeaconBlockHeader, error) {
	b := sb.Block()
	if b.IsNil() || b.Body().IsNil() {
		return nil, errors.New("nil block")
	}

	h, err := BeaconBlockHeaderFromBlockInterface(b)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block header of block")
	}
	sig := sb.Signature()
	return &zondpb.SignedBeaconBlockHeader{
		Header:    h,
		Signature: sig[:],
	}, nil
}

// BeaconBlockHeaderFromBlockInterface function to retrieve block header from block.
func BeaconBlockHeaderFromBlockInterface(block ReadOnlyBeaconBlock) (*zondpb.BeaconBlockHeader, error) {
	if block.Body().IsNil() {
		return nil, errors.New("nil block body")
	}

	bodyRoot, err := block.Body().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get body root of block")
	}
	parentRoot := block.ParentRoot()
	stateRoot := block.StateRoot()
	return &zondpb.BeaconBlockHeader{
		Slot:          block.Slot(),
		ProposerIndex: block.ProposerIndex(),
		ParentRoot:    parentRoot[:],
		StateRoot:     stateRoot[:],
		BodyRoot:      bodyRoot[:],
	}, nil
}
