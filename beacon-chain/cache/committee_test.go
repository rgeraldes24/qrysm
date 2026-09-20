//go:build !fuzz

package cache

import (
	"context"
	"math"
	"sort"
	"strconv"
	"testing"

	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

func TestCommitteeKeyFn_OK(t *testing.T) {
	item := &Committees{
		CommitteeCount:  1,
		Seed:            [32]byte{'A'},
		ShuffledIndices: []primitives.ValidatorIndex{1, 2, 3, 4, 5},
	}

	k, err := committeeKeyFn(item)
	require.NoError(t, err)
	assert.Equal(t, key(item.Seed), k)
}

func TestCommitteeKeyFn_InvalidObj(t *testing.T) {
	_, err := committeeKeyFn("bad")
	assert.Equal(t, ErrNotCommittee, err)
}

func TestCommitteeCache_CommitteesByEpoch(t *testing.T) {
	cache := NewCommitteesCache()

	item := &Committees{
		ShuffledIndices: []primitives.ValidatorIndex{1, 2, 3, 4, 5, 6},
		Seed:            [32]byte{'A'},
		CommitteeCount:  3,
	}

	slot := params.BeaconConfig().SlotsPerEpoch
	committeeIndex := primitives.CommitteeIndex(1)
	indices, err := cache.Committee(context.Background(), slot, item.Seed, committeeIndex)
	require.NoError(t, err)
	if indices != nil {
		t.Error("Expected committee not to exist in empty cache")
	}
	require.NoError(t, cache.AddCommitteeShuffledList(context.Background(), item))

	wantedIndex := primitives.CommitteeIndex(0)
	indices, err = cache.Committee(context.Background(), slot, item.Seed, wantedIndex)
	require.NoError(t, err)

	start, end := startEndIndices(item, uint64(wantedIndex))
	assert.DeepEqual(t, item.ShuffledIndices[start:end], indices)
}

func TestCommitteeCache_ActiveIndices(t *testing.T) {
	cache := NewCommitteesCache()

	item := &Committees{Seed: [32]byte{'A'}, SortedIndices: []primitives.ValidatorIndex{1, 2, 3, 4, 5, 6}}
	indices, err := cache.ActiveIndices(context.Background(), item.Seed)
	require.NoError(t, err)
	if indices != nil {
		t.Error("Expected committee not to exist in empty cache")
	}

	require.NoError(t, cache.AddCommitteeShuffledList(context.Background(), item))

	indices, err = cache.ActiveIndices(context.Background(), item.Seed)
	require.NoError(t, err)
	assert.DeepEqual(t, item.SortedIndices, indices)
}

func TestCommitteeCache_ActiveCount(t *testing.T) {
	cache := NewCommitteesCache()

	item := &Committees{Seed: [32]byte{'A'}, SortedIndices: []primitives.ValidatorIndex{1, 2, 3, 4, 5, 6}}
	count, err := cache.ActiveIndicesCount(context.Background(), item.Seed)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Expected active count not to exist in empty cache")

	require.NoError(t, cache.AddCommitteeShuffledList(context.Background(), item))

	count, err = cache.ActiveIndicesCount(context.Background(), item.Seed)
	require.NoError(t, err)
	assert.Equal(t, len(item.SortedIndices), count)
}

func TestCommitteeCache_CanRotate(t *testing.T) {
	cache := NewCommitteesCache()

	// Should rotate out all the epochs except 190 through 199.
	start := 100
	end := 200
	for i := start; i < end; i++ {
		s := []byte(strconv.Itoa(i))
		item := &Committees{Seed: bytesutil.ToBytes32(s)}
		require.NoError(t, cache.AddCommitteeShuffledList(context.Background(), item))
	}

	k := cache.CommitteeCache.Keys()
	assert.Equal(t, maxCommitteesCacheSize, len(k))

	sort.Slice(k, func(i, j int) bool {
		return k[i].(string) < k[j].(string)
	})
	wanted := end - maxCommitteesCacheSize
	s := bytesutil.ToBytes32([]byte(strconv.Itoa(wanted)))
	assert.Equal(t, key(s), k[0], "incorrect key received for slot 190")

	s = bytesutil.ToBytes32([]byte(strconv.Itoa(199)))
	assert.Equal(t, key(s), k[len(k)-1], "incorrect key received for slot 199")
}

func TestCommitteeCacheOutOfRange(t *testing.T) {
	cache := NewCommitteesCache()
	seed := bytesutil.ToBytes32([]byte("foo"))
	comms := &Committees{
		CommitteeCount:  1,
		Seed:            seed,
		ShuffledIndices: []primitives.ValidatorIndex{0},
		SortedIndices:   []primitives.ValidatorIndex{},
	}
	key, err := committeeKeyFn(comms)
	assert.NoError(t, err)
	_ = cache.CommitteeCache.Add(key, comms)

	_, err = cache.Committee(context.Background(), 0, seed, math.MaxUint64) // Overflow!
	require.NotNil(t, err, "Did not fail as expected")
}

func TestCommitteeCache_RejectsCommitteeIndexAtOrBeyondSlotCount(t *testing.T) {
	cache := NewCommitteesCache()
	const validatorCount = 256
	shuffled := make([]primitives.ValidatorIndex, validatorCount)
	for i := range shuffled {
		shuffled[i] = primitives.ValidatorIndex(i)
	}
	seed := bytesutil.ToBytes32([]byte("bounds"))
	// One committee per slot, as UpdateCommitteeCache stores it.
	item := &Committees{
		CommitteeCount:  uint64(params.BeaconConfig().SlotsPerEpoch),
		Seed:            seed,
		ShuffledIndices: shuffled,
		SortedIndices:   shuffled,
	}
	require.NoError(t, cache.AddCommitteeShuffledList(context.Background(), item))

	slot := primitives.Slot(5)
	legit, err := cache.Committee(context.Background(), slot, seed, 0)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(legit))

	// Index 1 would otherwise resolve to the next slot's committee, and
	// 256 * 2^56 wraps to 0 in uint64 so the huge index would resolve to this
	// slot's own committee.
	for _, index := range []primitives.CommitteeIndex{1, 1 << 56} {
		_, err := cache.Committee(context.Background(), slot, seed, index)
		require.ErrorContains(t, "requested index out of bound", err, "committee index %d", index)
	}
}

func TestCommitteeCache_DoesNothingWhenCancelledContext(t *testing.T) {
	cache := NewCommitteesCache()

	item := &Committees{Seed: [32]byte{'A'}, SortedIndices: []primitives.ValidatorIndex{1, 2, 3, 4, 5, 6}}
	count, err := cache.ActiveIndicesCount(context.Background(), item.Seed)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Expected active count not to exist in empty cache")

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, cache.AddCommitteeShuffledList(cancelled, item), context.Canceled)

	count, err = cache.ActiveIndicesCount(context.Background(), item.Seed)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
