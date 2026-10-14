package cache

import (
	"errors"

	"github.com/theQRL/qrysm/consensus-types/primitives"
)

// ErrNotProposerIndices will be returned when a cache object is not a pointer to
// a ProposerIndices struct.
var ErrNotProposerIndices = errors.New("object is not a proposer indices struct")

// ProposerIndices defines the cached struct for proposer indices. Despite its
// name, BlockRoot holds the cache key: the state root at the last slot of the
// previous epoch.
type ProposerIndices struct {
	BlockRoot       [32]byte
	ProposerIndices []primitives.ValidatorIndex
}
