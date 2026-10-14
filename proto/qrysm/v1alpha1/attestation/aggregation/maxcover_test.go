package aggregation

import (
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/testing/assert"
)

func TestMaxCover_MaxCover(t *testing.T) {
	problemSet := func() []*bitfield.Bitlist64 {
		return []*bitfield.Bitlist64{
			bitfield.NewBitlist64From([]uint64{0b00000100}),
			bitfield.NewBitlist64From([]uint64{0b00011011}),
			bitfield.NewBitlist64From([]uint64{0b00011011}),
			bitfield.NewBitlist64From([]uint64{0b00000001}),
			bitfield.NewBitlist64From([]uint64{0b00011010}),
		}
	}
	type args struct {
		k             int
		candidates    []*bitfield.Bitlist64
		allowOverlaps bool
	}
	type BitSetAggregation struct {
		Coverage *bitfield.Bitlist64
		Keys     []int
	}
	tests := []struct {
		name      string
		args      args
		want      *BitSetAggregation
		wantedErr string
	}{
		{
			name:      "nil problem",
			args:      args{},
			wantedErr: ErrInvalidMaxCoverProblem.Error(),
		},
		{
			name: "different bitlengths (pick first, combine with third)",
			args: args{k: 3, candidates: []*bitfield.Bitlist64{
				bitfield.NewBitlist64From([]uint64{0b00000001, 0b11100000, 0b10000000}),
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b00011111}),
				bitfield.NewBitlist64From([]uint64{0b00000110, 0b00000000, 0b01000000}),
			}},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b00000111, 0b11100000, 0b11000000}),
				Keys:     []int{0, 2},
			},
		},
		{
			name: "different bitlengths (pick first, no other combination)",
			args: args{k: 3, candidates: []*bitfield.Bitlist64{
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b00011111}),
				bitfield.NewBitlist64From([]uint64{0b00000001, 0b11100000, 0b1}),
				bitfield.NewBitlist64From([]uint64{0b00000110, 0b00000000, 0b1}),
			}},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b00000000, 0b00011111}),
				Keys:     []int{0},
			},
		},
		{
			name: "k=0",
			args: args{k: 0, candidates: problemSet()},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b0}),
				Keys:     []int{},
			},
		},
		{
			name: "k=1",
			args: args{k: 1, candidates: problemSet()},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b0011011}),
				Keys:     []int{1},
			},
		},
		{
			name: "k=2",
			args: args{k: 2, candidates: problemSet()},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b0011111}),
				Keys:     []int{0, 1},
			},
		},
		{
			name: "k=3",
			args: args{k: 3, candidates: problemSet()},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b0011111}),
				Keys:     []int{0, 1},
			},
		},
		{
			name: "k=5",
			args: args{k: 5, candidates: problemSet()},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b0011111}),
				Keys:     []int{0, 1},
			},
		},
		{
			name: "k=50",
			args: args{k: 50, candidates: problemSet()},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b0011111}),
				Keys:     []int{0, 1},
			},
		},
		{
			name: "suboptimal", // Greedy algorithm selects: 0, 2, 3, while 1,4,5 is optimal.
			args: args{k: 3, candidates: []*bitfield.Bitlist64{
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b00011111}),
				bitfield.NewBitlist64From([]uint64{0b00000001, 0b11100000}),
				bitfield.NewBitlist64From([]uint64{0b00000110, 0b00000000}),
				bitfield.NewBitlist64From([]uint64{0b00110000, 0b01110000}),
				bitfield.NewBitlist64From([]uint64{0b00000110, 0b10001100}),
				bitfield.NewBitlist64From([]uint64{0b01001001, 0b00000011}),
			}},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b00000111, 0b11111111}),
				Keys:     []int{0, 1, 2},
			},
		},
		{
			name: "allow overlaps",
			args: args{k: 5, allowOverlaps: true, candidates: []*bitfield.Bitlist64{
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b00000001, 0b11111110}),
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b00001110, 0b00001110}),
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b01110000, 0b01110000}),
				bitfield.NewBitlist64From([]uint64{0b00000111, 0b10000001, 0b10000000}),
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b00000110, 0b00000110}),
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b00000001, 0b01100010}),
				bitfield.NewBitlist64From([]uint64{0b00001000, 0b00001000, 0b10000010}),
			}},
			want: &BitSetAggregation{
				Coverage: bitfield.NewBitlist64From([]uint64{0b00001111, 0xff, 0b11111110}),
				Keys:     []int{0, 1, 2, 3, 6},
			},
		},
		{
			name: "empty bitlists",
			args: args{k: 5, allowOverlaps: true, candidates: []*bitfield.Bitlist64{
				bitfield.NewBitlist64From([]uint64{}),
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b00001110, 0b00001110}),
				bitfield.NewBitlist64From([]uint64{0b00000000, 0b01110000, 0b01110000}),
			}},
			wantedErr: "empty bitlists: invalid max_cover problem",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selectedCandidates, coverage, err := MaxCover(tt.args.candidates, tt.args.k, tt.args.allowOverlaps)
			if tt.wantedErr != "" {
				assert.ErrorContains(t, tt.wantedErr, err)
			} else {
				assert.NoError(t, err)
				assert.DeepEqual(t, tt.want.Coverage, coverage)
				selectedKeys := make([]int, selectedCandidates.Count())
				selectedCandidates.NoAllocBitIndices(selectedKeys)
				assert.DeepEqual(t, tt.want.Keys, selectedKeys)
			}
		})
	}
}
