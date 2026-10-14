package ssz

import (
	"github.com/theQRL/qrysm/container/trie"
	"github.com/theQRL/qrysm/crypto/hash/htr"
)

const (
	mask0 = ^uint64((1 << (1 << iota)) - 1)
	mask1
	mask2
	mask3
	mask4
	mask5
)

const (
	bit0 = uint8(1 << iota)
	bit1
	bit2
	bit3
	bit4
	bit5
)

// Depth retrieves the appropriate depth for the provided trie size.
func Depth(v uint64) (out uint8) {
	// bitmagic: binary search through a uint32, offset down by 1 to not round powers of 2 up.
	// Then adding 1 to it to not get the index of the first bit, but the length of the bits (depth of tree)
	// Zero is a special case, it has a 0 depth.
	// Example:
	//  (in out): (0 0), (1 0), (2 1), (3 2), (4 2), (5 3), (6 3), (7 3), (8 3), (9 4)
	if v <= 1 {
		return 0
	}
	v--
	if v&mask5 != 0 {
		v >>= bit5
		out |= bit5
	}
	if v&mask4 != 0 {
		v >>= bit4
		out |= bit4
	}
	if v&mask3 != 0 {
		v >>= bit3
		out |= bit3
	}
	if v&mask2 != 0 {
		v >>= bit2
		out |= bit2
	}
	if v&mask1 != 0 {
		v >>= bit1
		out |= bit1
	}
	if v&mask0 != 0 {
		out |= bit0
	}
	out++
	return
}

// MerkleizeVector uses our optimized routine to hash a list of 32-byte
// elements.
func MerkleizeVector(elements [][32]byte, length uint64) [32]byte {
	depth := Depth(length)
	// Return zerohash at depth
	if len(elements) == 0 {
		return trie.ZeroHashes[depth]
	}
	// Odd layers are padded with append below. Cap the slice so the append
	// can never write into spare capacity of the caller's backing array.
	elements = elements[:len(elements):len(elements)]
	for i := range depth {
		layerLen := len(elements)
		oddNodeLength := layerLen%2 == 1
		if oddNodeLength {
			zerohash := trie.ZeroHashes[i]
			elements = append(elements, zerohash)
		}
		elements = htr.VectorizedSha256(elements)
	}
	return elements[0]
}
