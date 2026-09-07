package htr

import (
	"crypto/sha256"
	"math/rand"
	"testing"
)

// The hashing library picks a CPU-specific code path (SHA-NI, AVX2, AVX-512)
// at start-up. Compare its output with the standard library across sizes on
// both sides of the parallel threshold, so a wrong path on a particular CPU
// fails here by name rather than in an unrelated state test.
func TestVectorizedSha256_MatchesStandardLibrary(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, pairs := range []int{1, 2, 3, 7, 8, 15, 16, 17, 31, 32, 33, 64, 100, 255, 256, 257, 1000, 2500, 2501, 4096, 4097} {
		input := make([][32]byte, 2*pairs)
		for i := range input {
			_, _ = rng.Read(input[i][:])
		}

		got := VectorizedSha256(input)
		if len(got) != pairs {
			t.Fatalf("%d pairs: got %d outputs", pairs, len(got))
		}

		for i := range pairs {
			var buf [64]byte
			copy(buf[:32], input[2*i][:])
			copy(buf[32:], input[2*i+1][:])
			if want := sha256.Sum256(buf[:]); got[i] != want {
				t.Fatalf("%d pairs: hash %d differs from crypto/sha256", pairs, i)
			}
		}
	}
}
