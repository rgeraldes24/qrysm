package aggregation

import (
	"fmt"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/config/params"
	aggtesting "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/attestation/aggregation/testing"
)

func BenchmarkMaxCover(b *testing.B) {
	bitlistLen := params.BeaconConfig().MaxValidatorsPerCommittee
	tests := []struct {
		numCandidates uint64
		numMarkedBits uint64
		allowOverlaps bool
	}{
		{
			numCandidates: 32,
			numMarkedBits: 1,
		},
		{
			numCandidates: 128,
			numMarkedBits: 1,
		},
		{
			numCandidates: 256,
			numMarkedBits: 1,
		},
		{
			numCandidates: 32,
			numMarkedBits: 8,
		},
		{
			numCandidates: 1024,
			numMarkedBits: 8,
		},
		{
			numCandidates: 2048,
			numMarkedBits: 8,
		},
		{
			numCandidates: 1024,
			numMarkedBits: 32,
		},
		{
			numCandidates: 2048,
			numMarkedBits: 32,
		},
		{
			numCandidates: 1024,
			numMarkedBits: 128,
		},
		{
			numCandidates: 2048,
			numMarkedBits: 128,
		},
		{
			numCandidates: 1024,
			numMarkedBits: 512,
		},
		{
			numCandidates: 2048,
			numMarkedBits: 512,
		},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("%d_attestations_with_%d_bit(s)_set", tt.numCandidates, tt.numMarkedBits)
		b.Run(name, func(b *testing.B) {
			var bitlists []*bitfield.Bitlist64
			if tt.numMarkedBits == 1 {
				bitlists = aggtesting.Bitlists64WithSingleBitSet(tt.numCandidates, bitlistLen)
			} else {
				bitlists = aggtesting.Bitlists64WithMultipleBitSet(b, tt.numCandidates, bitlistLen, tt.numMarkedBits)

			}
			for b.Loop() {
				_, _, err := MaxCover(bitlists, len(bitlists), tt.allowOverlaps)
				_ = err
			}
		})
	}
}
