package ssz_test

import (
	"testing"

	"github.com/theQRL/qrysm/encoding/ssz"
	"github.com/theQRL/qrysm/testing/assert"
)

func TestGetDepth(t *testing.T) {
	trieSize := uint64(896745231)
	expected := uint8(30)

	result := ssz.Depth(trieSize)
	assert.Equal(t, expected, result)
}

func TestDepthOfOne(t *testing.T) {
	assert.Equal(t, uint8(0), ssz.Depth(1))
}

func TestMerkleizeVector_DoesNotWriteIntoCallerArray(t *testing.T) {
	leaves := make([][32]byte, 4)
	for i := range leaves {
		leaves[i][0] = byte(i + 1)
	}
	fourth := leaves[3]
	got := ssz.MerkleizeVector(leaves[:3], 4)
	assert.Equal(t, fourth, leaves[3], "padding must not overwrite the caller's spare capacity")
	padded := make([][32]byte, 4)
	copy(padded, leaves[:3])
	assert.Equal(t, ssz.MerkleizeVector(padded, 4), got)
}
