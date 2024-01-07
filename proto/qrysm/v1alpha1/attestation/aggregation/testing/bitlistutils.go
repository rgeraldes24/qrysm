package testing

import (
	"math/rand"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/time"
)

// BitlistWithAllBitsSet creates list of bitlists with all bits set.
func BitlistWithAllBitsSet(length uint64) bitfield.Bitlist {
	b := bitfield.NewBitlist(length)
	for i := uint64(0); i < length; i++ {
		b.SetBitAt(i, true)
	}
	return b
}

// BitlistsWithSingleBitSet creates list of bitlists with a single bit set in each.
func BitlistsWithSingleBitSet(n, length uint64) []bitfield.Bitlist {
	lists := make([]bitfield.Bitlist, n)
	for i := uint64(0); i < n; i++ {
		b := bitfield.NewBitlist(length)
		b.SetBitAt(i%length, true)
		lists[i] = b
	}
	return lists
}

// Bitlists64WithSingleBitSet creates list of bitlists with a single bit set in each.
func Bitlists64WithSingleBitSet(n, length uint64) []*bitfield.Bitlist64 {
	lists := make([]*bitfield.Bitlist64, n)
	for i := uint64(0); i < n; i++ {
		b := bitfield.NewBitlist64(length)
		b.SetBitAt(i%length, true)
		lists[i] = b
	}
	return lists
}

// BitlistsWithMultipleBitSet creates list of bitlists with random n bits set.
func BitlistsWithMultipleBitSet(t testing.TB, n, length, count uint64) []bitfield.Bitlist {
	seed := time.Now().UnixNano()
	t.Logf("bitlistsWithMultipleBitSet random seed: %v", seed)
	rand.Seed(seed)
	lists := make([]bitfield.Bitlist, n)
	for i := uint64(0); i < n; i++ {
		b := bitfield.NewBitlist(length)
		keys := rand.Perm(int(length)) // lint:ignore uintcast -- This is safe in test code.
		for _, key := range keys[:count] {
			b.SetBitAt(uint64(key), true)
		}
		lists[i] = b
	}
	return lists
}

// Bitlists64WithMultipleBitSet creates list of bitlists with random n bits set.
func Bitlists64WithMultipleBitSet(t testing.TB, n, length, count uint64) []*bitfield.Bitlist64 {
	seed := time.Now().UnixNano()
	t.Logf("Bitlists64WithMultipleBitSet random seed: %v", seed)
	rand.Seed(seed)
	lists := make([]*bitfield.Bitlist64, n)
	for i := uint64(0); i < n; i++ {
		b := bitfield.NewBitlist64(length)
		keys := rand.Perm(int(length)) // lint:ignore uintcast -- This is safe in test code.
		for _, key := range keys[:count] {
			b.SetBitAt(uint64(key), true)
		}
		lists[i] = b
	}
	return lists
}

// MakeAttestationsFromBitlists creates list of attestations from list of bitlist.
func MakeAttestationsFromBitlists(bl []bitfield.Bitlist) []*zondpb.Attestation {
	atts := make([]*zondpb.Attestation, len(bl))
	for i, b := range bl {
		atts[i] = &zondpb.Attestation{
			ParticipationBits: b,
			Data: &zondpb.AttestationData{
				Slot:           42,
				CommitteeIndex: 1,
			},
			Signatures: [][]byte{},
		}
	}
	return atts
}

// MakeSyncContributionsFromBitVector creates list of sync contributions from list of bitvector.
func MakeSyncContributionsFromBitVector(bl []bitfield.Bitvector128) []*zondpb.SyncCommitteeContribution {
	c := make([]*zondpb.SyncCommitteeContribution, len(bl))
	for i, b := range bl {
		c[i] = &zondpb.SyncCommitteeContribution{
			Slot:              primitives.Slot(1),
			SubcommitteeIndex: 2,
			ParticipationBits: b,
			Signatures:        [][]byte{},
		}
	}
	return c
}
