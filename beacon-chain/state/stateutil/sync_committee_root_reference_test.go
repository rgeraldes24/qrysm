package stateutil

import (
	"crypto/sha256"
	"math/rand"
	"testing"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/container/trie"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
)

// A plain crypto/sha256 merkleization with the same layout SyncCommitteeRoot
// uses, independent of the vectorized hasher. The two must agree on every
// CPU; a mismatch means the hasher, not the expected value, is wrong.
func referenceSyncCommitteeRoot(pubkeys [][]byte) [32]byte {
	roots := make([][32]byte, 0, len(pubkeys))
	for _, pubkey := range pubkeys {
		roots = append(roots, referenceMerkleize(referenceChunks(pubkey)))
	}
	pubkeysRoot := referenceMerkleize(roots)
	return referenceMerkleize([][32]byte{pubkeysRoot})
}

func referenceChunks(b []byte) [][32]byte {
	var chunks [][32]byte
	for i := 0; i < len(b); i += 32 {
		var c [32]byte
		copy(c[:], b[i:])
		chunks = append(chunks, c)
	}
	return chunks
}

// Pads an odd layer with the zero hash of that height, then hashes pairs.
func referenceMerkleize(layer [][32]byte) [32]byte {
	for depth := 0; len(layer) > 1; depth++ {
		if len(layer)%2 == 1 {
			layer = append(layer, trie.ZeroHashes[depth])
		}
		next := make([][32]byte, len(layer)/2)
		for i := range next {
			var buf [64]byte
			copy(buf[:32], layer[2*i][:])
			copy(buf[32:], layer[2*i+1][:])
			next[i] = sha256.Sum256(buf[:])
		}
		layer = next
	}
	return layer[0]
}

func TestSyncCommitteeRoot_MatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	size := int(params.BeaconConfig().SyncCommitteeSize)
	keyLen := fieldparams.MLDSA87PubkeyLength

	for _, tc := range []struct {
		name string
		fill func(b []byte)
	}{
		{"random", func(b []byte) { _, _ = rng.Read(b) }},
		{"zero", func(b []byte) { clear(b) }},
		{"ones", func(b []byte) {
			for i := range b {
				b[i] = 0xff
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pubkeys := make([][]byte, size)
			for i := range pubkeys {
				pubkeys[i] = make([]byte, keyLen)
				tc.fill(pubkeys[i])
			}

			got, err := SyncCommitteeRoot(&qrysmpb.SyncCommittee{Pubkeys: pubkeys})
			require.NoError(t, err)
			require.Equal(t, referenceSyncCommitteeRoot(pubkeys), got)
		})
	}
}
