package cache

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/theQRL/qrysm/consensus-types/primitives"
)

// ErrNotCommittee will be returned when a cache object is not a pointer to
// a Committee struct.
var ErrNotCommittee = errors.New("object is not a committee struct")

// CommitteeKey identifies a shuffle by its seed and ordered active-validator
// indices. A seed alone does not distinguish branches with different exits or
// activations but the same RANDAO mix.
type CommitteeKey struct {
	seed        [32]byte
	indicesRoot [32]byte
}

// NewCommitteeKey commits to all inputs to a committee shuffle. It is only a
// cache identity; the original seed must still be used for shuffling.
func NewCommitteeKey(seed [32]byte, indices []primitives.ValidatorIndex) CommitteeKey {
	h := sha256.New()
	var encoded [8]byte
	for _, index := range indices {
		binary.LittleEndian.PutUint64(encoded[:], uint64(index))
		_, _ = h.Write(encoded[:])
	}
	cacheKey := CommitteeKey{seed: seed}
	h.Sum(cacheKey.indicesRoot[:0])
	return cacheKey
}

// Committees stores a shuffle and the ordered active indices it was computed from.
type Committees struct {
	CommitteeCount  uint64
	Seed            [32]byte
	ShuffledIndices []primitives.ValidatorIndex
	SortedIndices   []primitives.ValidatorIndex
}
