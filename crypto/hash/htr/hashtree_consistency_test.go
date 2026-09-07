package htr

import (
	"crypto/sha256"
	"math/rand"
	"testing"
)

// The hashing library picks a CPU-specific code path (SHA-NI, AVX2, AVX-512)
// at start-up. Compare its output with the standard library across sizes on
// both sides of the parallel threshold and across input patterns, so a wrong
// path on a particular CPU fails here by name rather than in an unrelated
// state test.
func TestVectorizedSha256_MatchesStandardLibrary(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	patterns := map[string]func(i int, chunk *[32]byte){
		"random": func(_ int, chunk *[32]byte) { _, _ = rng.Read(chunk[:]) },
		"zero":   func(_ int, chunk *[32]byte) { *chunk = [32]byte{} },
		// The last chunk of a pair is zero, as when an odd layer is padded.
		"trailing-zero": func(i int, chunk *[32]byte) {
			if i%2 == 1 {
				*chunk = [32]byte{}
			} else {
				_, _ = rng.Read(chunk[:])
			}
		},
		"ones": func(_ int, chunk *[32]byte) {
			for j := range chunk {
				chunk[j] = 0xff
			}
		},
	}
	// 41 is a 2592-byte public key packed into 81 chunks plus one padding
	// chunk; 21, 11, 6, 3 and 2 are the layers above it.
	sizes := []int{1, 2, 3, 6, 7, 8, 11, 15, 16, 17, 21, 31, 32, 33, 41, 64, 100, 255, 256, 257, 512, 1000, 2500, 2501, 4096, 4097, 8192}

	for name, fill := range patterns {
		for _, pairs := range sizes {
			input := make([][32]byte, 2*pairs)
			for i := range input {
				fill(i, &input[i])
			}

			got := VectorizedSha256(input)
			if len(got) != pairs {
				t.Fatalf("%s, %d pairs: got %d outputs", name, pairs, len(got))
			}

			for i := range pairs {
				var buf [64]byte
				copy(buf[:32], input[2*i][:])
				copy(buf[32:], input[2*i+1][:])
				if want := sha256.Sum256(buf[:]); got[i] != want {
					t.Fatalf("%s, %d pairs: hash %d differs from crypto/sha256", name, pairs, i)
				}
			}
		}
	}
}
