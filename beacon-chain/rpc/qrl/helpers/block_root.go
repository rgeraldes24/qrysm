package helpers

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/config/params"
)

// BlockRootFromState returns the root of the block that produced the given state.
//
// Unlike st.LatestBlockHeader().HashTreeRoot(), a zeroed header state_root is replaced with
// st.HashTreeRoot(ctx) before hashing. The state root is still zero in the header of a state
// at the slot of its latest block, so hashing the header as is does not give the block root.
func BlockRootFromState(ctx context.Context, st state.BeaconState) ([32]byte, error) {
	header := st.LatestBlockHeader()
	if header == nil {
		return [32]byte{}, errors.New("state has a nil latest block header")
	}

	zeroHash := params.BeaconConfig().ZeroHash
	if len(header.StateRoot) == 0 || bytes.Equal(header.StateRoot, zeroHash[:]) {
		stateRoot, err := st.HashTreeRoot(ctx)
		if err != nil {
			return [32]byte{}, errors.Wrap(err, "could not compute state root")
		}
		header.StateRoot = stateRoot[:]
	}

	root, err := header.HashTreeRoot()
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute block header root")
	}
	return root, nil
}
