package validator

import (
	"bytes"
	"sort"
	"testing"

	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	v1alpha1 "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
)

func TestProposerSyncContributions_FilterByBlockRoot(t *testing.T) {
	rootA := [32]byte{'a'}
	rootB := [32]byte{'b'}
	var participationBits [fieldparams.SyncCommitteeAggregationBytesLength]byte
	tests := []struct {
		name string
		cs   proposerSyncContributions
		want proposerSyncContributions
	}{
		{
			name: "empty list",
			cs:   proposerSyncContributions{},
			want: proposerSyncContributions{},
		},
		{
			name: "single item, not found",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits[:]},
			},
			want: proposerSyncContributions{},
		},
		{
			name: "single item with filter, found",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 0},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootB[:], Slot: 1},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:]},
			},
		},
		{
			name: "multiple items with filter, found",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 0},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootB[:], Slot: 1},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 2},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootB[:], Slot: 3},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 0},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:], Slot: 2},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.cs.filterByBlockRoot(rootA)
			assert.DeepEqual(t, tt.want, cs)
		})
	}
}

func TestProposerSyncContributions_FilterBySubcommitteeID(t *testing.T) {
	rootA := [32]byte{'a'}
	rootB := [32]byte{'b'}
	var participationBits [fieldparams.SyncCommitteeAggregationBytesLength]byte
	tests := []struct {
		name string
		cs   proposerSyncContributions
		want proposerSyncContributions
	}{
		{
			name: "empty list",
			cs:   proposerSyncContributions{},
			want: proposerSyncContributions{},
		},
		{
			name: "single item, not found",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits[:], SubcommitteeIndex: 1},
			},
			want: proposerSyncContributions{},
		},
		{
			name: "single item with filter",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:], SubcommitteeIndex: 0},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 1},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:]},
			},
		},
		{
			name: "multiple items with filter",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:], SubcommitteeIndex: 0},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 1},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 0},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 2},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootA[:], SubcommitteeIndex: 0},
				&v1alpha1.SyncCommitteeContribution{BlockRoot: rootB[:], SubcommitteeIndex: 0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.cs.filterBySubIndex(0)
			assert.DeepEqual(t, tt.want, cs)
		})
	}
}

func TestProposerSyncContributions_Dedup(t *testing.T) {
	// Prepare aggregation bits for all scenarios
	var participationBits1, participationBits2_1, participationBits2_2, participationBits3, participationBits4_1, participationBits4_2, participationBits4_3, participationBits4_4, participationBits4_5, participationBits5_1, participationBits5_2, participationBits5_3, participationBits5_4, participationBits5_5, participationBits6, participationBits7_1, participationBits7_2, participationBits7_3, participationBits7_4, participationBits7_5, participationBits8_1, participationBits8_2, participationBits8_3, participationBits8_4, participationBits8_5, participationBits8_6, participationBits9_1, participationBits9_2, participationBits9_3, participationBits9_4, participationBits10_1, participationBits10_2, participationBits10_3, participationBits10_4, participationBits10_5, participationBits11_1, participationBits11_2, participationBits11_3, participationBits11_4, participationBits12_1, participationBits12_2, participationBits12_3, participationBits12_4, participationBits12_5, participationBits13_1, participationBits13_2 [fieldparams.SyncCommitteeAggregationBytesLength]byte
	b2_1, b2_2 := []byte{0b10111110, 0x01}, []byte{0b01111111, 0x01}
	copy(participationBits2_1[:], b2_1)
	copy(participationBits2_2[:], b2_2)
	b3 := []byte{0xba, 0x01}
	copy(participationBits3[:], b3)
	b4_1, b4_2, b4_3, b4_4, b4_5 := []byte{0b11001111, 0b1}, []byte{0b01101101, 0b1}, []byte{0b00101011, 0b1}, []byte{0b10100000, 0b1}, []byte{0b00010000, 0b1}
	copy(participationBits4_1[:], b4_1)
	copy(participationBits4_2[:], b4_2)
	copy(participationBits4_3[:], b4_3)
	copy(participationBits4_4[:], b4_4)
	copy(participationBits4_5[:], b4_5)
	b5_1, b5_2, b5_3, b5_4, b5_5 := []byte{0b11001111, 0b1}, []byte{0b01101101, 0b1}, []byte{0b00001111, 0b1}, []byte{0b00000011, 0b1}, []byte{0b00000001, 0b1}
	copy(participationBits5_1[:], b5_1)
	copy(participationBits5_2[:], b5_2)
	copy(participationBits5_3[:], b5_3)
	copy(participationBits5_4[:], b5_4)
	copy(participationBits5_5[:], b5_5)
	b6 := []byte{0b00000011, 0b1}
	copy(participationBits6[:], b6)
	b7_1, b7_2, b7_3, b7_4, b7_5 := []byte{0b01101101, 0b1}, []byte{0b00100010, 0b1}, []byte{0b10100101, 0b1}, []byte{0b00010000, 0b1}, []byte{0b11001111, 0b1}
	copy(participationBits7_1[:], b7_1)
	copy(participationBits7_2[:], b7_2)
	copy(participationBits7_3[:], b7_3)
	copy(participationBits7_4[:], b7_4)
	copy(participationBits7_5[:], b7_5)
	b8_1, b8_2, b8_3, b8_4, b8_5, b8_6 := []byte{0b00001111, 0b1}, []byte{0b11001111, 0b1}, []byte{0b10100101, 0b1}, []byte{0b00000001, 0b1}, []byte{0b00000011, 0b1}, []byte{0b01101101, 0b1}
	copy(participationBits8_1[:], b8_1)
	copy(participationBits8_2[:], b8_2)
	copy(participationBits8_3[:], b8_3)
	copy(participationBits8_4[:], b8_4)
	copy(participationBits8_5[:], b8_5)
	copy(participationBits8_6[:], b8_6)
	b9_1, b9_2, b9_3, b9_4 := []byte{0b00000101, 0b1}, []byte{0b00000011, 0b1}, []byte{0b10000001, 0b1}, []byte{0b00011001, 0b1}
	copy(participationBits9_1[:], b9_1)
	copy(participationBits9_2[:], b9_2)
	copy(participationBits9_3[:], b9_3)
	copy(participationBits9_4[:], b9_4)
	b10_1, b10_2, b10_3, b10_4, b10_5 := []byte{0b00001111, 0b1}, []byte{0b11001111, 0b1}, []byte{0b00000001, 0b1}, []byte{0b00000011, 0b1}, []byte{0b01101101, 0b1}
	copy(participationBits10_1[:], b10_1)
	copy(participationBits10_2[:], b10_2)
	copy(participationBits10_3[:], b10_3)
	copy(participationBits10_4[:], b10_4)
	copy(participationBits10_5[:], b10_5)
	b11_1, b11_2, b11_3, b11_4 := []byte{0b00000101, 0b1}, []byte{0b00000011, 0b1}, []byte{0b10000001, 0b1}, []byte{0b00011001, 0b1}
	copy(participationBits11_1[:], b11_1)
	copy(participationBits11_2[:], b11_2)
	copy(participationBits11_3[:], b11_3)
	copy(participationBits11_4[:], b11_4)
	b12_1, b12_2, b12_3, b12_4, b12_5 := []byte{0b00001111, 0b1}, []byte{0b11001111, 0b1}, []byte{0b00000001, 0b1}, []byte{0b00000011, 0b1}, []byte{0b01101101, 0b1}
	copy(participationBits12_1[:], b12_1)
	copy(participationBits12_2[:], b12_2)
	copy(participationBits12_3[:], b12_3)
	copy(participationBits12_4[:], b12_4)
	copy(participationBits12_5[:], b12_5)
	b13_1, b13_2 := []byte{0b00001111, 0b1}, []byte{0b11001111, 0b1}
	copy(participationBits13_1[:], b13_1)
	copy(participationBits13_2[:], b13_2)

	tests := []struct {
		name string
		cs   proposerSyncContributions
		want proposerSyncContributions
	}{
		{
			name: "nil list",
			cs:   nil,
			want: proposerSyncContributions(nil),
		},
		{
			name: "empty list",
			cs:   proposerSyncContributions{},
			want: proposerSyncContributions{},
		},
		{
			name: "single item",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits1[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits1[:]},
			},
		},
		{
			name: "two items no duplicates",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits2_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits2_2[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits2_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits2_1[:]},
			},
		},
		{
			name: "two items with duplicates",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits3[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits3[:]},
			},
		},
		{
			name: "sorted no duplicates",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_4[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_5[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_4[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_5[:]},
			},
		},
		{
			name: "sorted with duplicates",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_4[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_4[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_5[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits5_2[:]},
			},
		},
		{
			name: "all equal",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits6[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits6[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits6[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits6[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits6[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits6[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits6[:]},
			},
		},
		{
			name: "unsorted no duplicates",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_4[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_5[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_5[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits7_4[:]},
			},
		},
		{
			name: "unsorted with duplicates",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_4[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_5[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_6[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_4[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_6[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits8_3[:]},
			},
		},
		{
			name: "no proper subset (same root)",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits9_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits9_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits9_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits9_4[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits9_4[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits9_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits9_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits9_3[:]},
			},
		},
		{
			name: "proper subset (same root)",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_4[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_5[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits10_5[:]},
			},
		},
		{
			name: "no proper subset (different index)",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits11_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits11_2[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits11_3[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits11_4[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits11_4[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits11_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits11_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits11_1[:]},
			},
		},
		{
			name: "proper subset (different index 1)",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits12_1[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits12_2[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits12_1[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits12_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits12_3[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits12_4[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits12_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits12_3[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits12_5[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits12_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits12_5[:]},
			},
		},
		{
			name: "proper subset (different index 2)",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits13_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits13_2[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits13_1[:]},
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits13_2[:]},
			},
			want: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{SubcommitteeIndex: 1, ParticipationBits: participationBits13_2[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits13_2[:]},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs, err := tt.cs.dedup()
			if err != nil {
				t.Error(err)
			}
			sort.Slice(cs, func(i, j int) bool {
				if cs[i].ParticipationBits.Count() == cs[j].ParticipationBits.Count() {
					if cs[i].SubcommitteeIndex == cs[j].SubcommitteeIndex {
						return bytes.Compare(cs[i].ParticipationBits, cs[j].ParticipationBits) <= 0
					}
					return cs[i].SubcommitteeIndex > cs[j].SubcommitteeIndex
				}
				return cs[i].ParticipationBits.Count() > cs[j].ParticipationBits.Count()
			})
			assert.DeepEqual(t, tt.want, cs)
		})
	}
}

func TestProposerSyncContributions_MostProfitable(t *testing.T) {
	// Prepare aggregation bits for all scenarios.
	var participationBits1, participationBits2_1, participationBits2_2, participationBits3_1, participationBits3_2, participationBits4_1, participationBits4_2 [fieldparams.SyncCommitteeAggregationBytesLength]byte
	b1 := []byte{0b01}
	copy(participationBits1[:], b1)
	b2_1, b2_2 := []byte{0b01}, []byte{0b10}
	copy(participationBits2_1[:], b2_1)
	copy(participationBits2_2[:], b2_2)
	b3_1, b3_2 := []byte{0b0101}, []byte{0b0100}
	copy(participationBits3_1[:], b3_1)
	copy(participationBits3_2[:], b3_2)
	b4_1, b4_2 := []byte{0b0101}, []byte{0b0111}
	copy(participationBits4_1[:], b4_1)
	copy(participationBits4_2[:], b4_2)

	tests := []struct {
		name string
		cs   proposerSyncContributions
		want *v1alpha1.SyncCommitteeContribution
	}{
		{
			name: "Same item",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits1[:]},
			},
			want: &v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits1[:]},
		},
		{
			name: "Same item again",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits2_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits2_2[:]},
			},
			want: &v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits2_1[:]},
		},
		{
			name: "most profitable at the start",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits3_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits3_2[:]},
			},
			want: &v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits3_1[:]},
		},
		{
			name: "most profitable at the end",
			cs: proposerSyncContributions{
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_1[:]},
				&v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_2[:]},
			},
			want: &v1alpha1.SyncCommitteeContribution{ParticipationBits: participationBits4_2[:]},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.cs.mostProfitable()
			assert.DeepEqual(t, tt.want, cs)
		})
	}
}
