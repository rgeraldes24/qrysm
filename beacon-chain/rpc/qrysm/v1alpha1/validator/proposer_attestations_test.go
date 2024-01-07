package validator

import (
	"bytes"
	"sort"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestProposer_ProposerAtts_sortByProfitability(t *testing.T) {
	atts := proposerAtts([]*zondpb.Attestation{
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 4}, ParticipationBits: bitfield.Bitlist{0b11100000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b11000000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b11100000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 4}, ParticipationBits: bitfield.Bitlist{0b11110000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b11100000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 3}, ParticipationBits: bitfield.Bitlist{0b11000000}}),
	})
	want := proposerAtts([]*zondpb.Attestation{
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 4}, ParticipationBits: bitfield.Bitlist{0b11110000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 4}, ParticipationBits: bitfield.Bitlist{0b11100000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 3}, ParticipationBits: bitfield.Bitlist{0b11000000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b11100000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b11100000}}),
		util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b11000000}}),
	})
	atts, err := atts.sortByProfitability()
	if err != nil {
		t.Error(err)
	}
	require.DeepEqual(t, want, atts)
}

func TestProposer_ProposerAtts_sortByProfitabilityUsingMaxCover(t *testing.T) {
	type testData struct {
		slot primitives.Slot
		bits bitfield.Bitlist
	}
	getAtts := func(data []testData) proposerAtts {
		var atts proposerAtts
		for _, att := range data {
			atts = append(atts, util.HydrateAttestation(&zondpb.Attestation{
				Data: &zondpb.AttestationData{Slot: att.slot}, ParticipationBits: att.bits}))
		}
		return atts
	}

	t.Run("no atts", func(t *testing.T) {
		atts := getAtts([]testData{})
		want := getAtts([]testData{})
		atts, err := atts.sortByProfitability()
		if err != nil {
			t.Error(err)
		}
		require.DeepEqual(t, want, atts)
	})

	t.Run("single att", func(t *testing.T) {
		atts := getAtts([]testData{
			{4, bitfield.Bitlist{0b11100000, 0b1}},
		})
		want := getAtts([]testData{
			{4, bitfield.Bitlist{0b11100000, 0b1}},
		})
		atts, err := atts.sortByProfitability()
		if err != nil {
			t.Error(err)
		}
		require.DeepEqual(t, want, atts)
	})

	t.Run("single att per slot", func(t *testing.T) {
		atts := getAtts([]testData{
			{1, bitfield.Bitlist{0b11000000, 0b1}},
			{4, bitfield.Bitlist{0b11100000, 0b1}},
		})
		want := getAtts([]testData{
			{4, bitfield.Bitlist{0b11100000, 0b1}},
			{1, bitfield.Bitlist{0b11000000, 0b1}},
		})
		atts, err := atts.sortByProfitability()
		if err != nil {
			t.Error(err)
		}
		require.DeepEqual(t, want, atts)
	})

	t.Run("two atts on one of the slots", func(t *testing.T) {
		atts := getAtts([]testData{
			{1, bitfield.Bitlist{0b11000000, 0b1}},
			{4, bitfield.Bitlist{0b11100000, 0b1}},
			{4, bitfield.Bitlist{0b11110000, 0b1}},
		})
		want := getAtts([]testData{
			{4, bitfield.Bitlist{0b11110000, 0b1}},
			{4, bitfield.Bitlist{0b11100000, 0b1}},
			{1, bitfield.Bitlist{0b11000000, 0b1}},
		})
		atts, err := atts.sortByProfitability()
		if err != nil {
			t.Error(err)
		}
		require.DeepEqual(t, want, atts)
	})

	t.Run("compare to native sort", func(t *testing.T) {
		// The max-cover based approach will select 0b00001100 instead, despite lower bit count
		// (since it has two new/unknown bits).
		t.Run("max-cover", func(t *testing.T) {
			atts := getAtts([]testData{
				{1, bitfield.Bitlist{0b11000011, 0b1}},
				{1, bitfield.Bitlist{0b11001000, 0b1}},
				{1, bitfield.Bitlist{0b00001100, 0b1}},
			})
			want := getAtts([]testData{
				{1, bitfield.Bitlist{0b11000011, 0b1}},
				{1, bitfield.Bitlist{0b00001100, 0b1}},
				{1, bitfield.Bitlist{0b11001000, 0b1}},
			})
			atts, err := atts.sortByProfitability()
			if err != nil {
				t.Error(err)
			}
			require.DeepEqual(t, want, atts)
		})
	})

	t.Run("multiple slots", func(t *testing.T) {
		atts := getAtts([]testData{
			{2, bitfield.Bitlist{0b11100000, 0b1}},
			{4, bitfield.Bitlist{0b11100000, 0b1}},
			{1, bitfield.Bitlist{0b11000000, 0b1}},
			{4, bitfield.Bitlist{0b11110000, 0b1}},
			{1, bitfield.Bitlist{0b11100000, 0b1}},
			{3, bitfield.Bitlist{0b11000000, 0b1}},
		})
		want := getAtts([]testData{
			{4, bitfield.Bitlist{0b11110000, 0b1}},
			{4, bitfield.Bitlist{0b11100000, 0b1}},
			{3, bitfield.Bitlist{0b11000000, 0b1}},
			{2, bitfield.Bitlist{0b11100000, 0b1}},
			{1, bitfield.Bitlist{0b11100000, 0b1}},
			{1, bitfield.Bitlist{0b11000000, 0b1}},
		})
		atts, err := atts.sortByProfitability()
		if err != nil {
			t.Error(err)
		}
		require.DeepEqual(t, want, atts)
	})

	t.Run("selected and non selected atts sorted by bit count", func(t *testing.T) {
		// Items at slot 4, must be first split into two lists by max-cover, with
		// 0b10000011 scoring higher (as it provides more info in addition to already selected
		// attestations) than 0b11100001 (despite naive bit count suggesting otherwise). Then,
		// both selected and non-selected attestations must be additionally sorted by bit count.
		atts := getAtts([]testData{
			{4, bitfield.Bitlist{0b00000001, 0b1}},
			{4, bitfield.Bitlist{0b11100001, 0b1}},
			{1, bitfield.Bitlist{0b11000000, 0b1}},
			{2, bitfield.Bitlist{0b11100000, 0b1}},
			{4, bitfield.Bitlist{0b10000011, 0b1}},
			{4, bitfield.Bitlist{0b11111000, 0b1}},
			{1, bitfield.Bitlist{0b11100000, 0b1}},
			{3, bitfield.Bitlist{0b11000000, 0b1}},
		})
		want := getAtts([]testData{
			{4, bitfield.Bitlist{0b11111000, 0b1}},
			{4, bitfield.Bitlist{0b10000011, 0b1}},
			{4, bitfield.Bitlist{0b11100001, 0b1}},
			{4, bitfield.Bitlist{0b00000001, 0b1}},
			{3, bitfield.Bitlist{0b11000000, 0b1}},
			{2, bitfield.Bitlist{0b11100000, 0b1}},
			{1, bitfield.Bitlist{0b11100000, 0b1}},
			{1, bitfield.Bitlist{0b11000000, 0b1}},
		})
		atts, err := atts.sortByProfitability()
		if err != nil {
			t.Error(err)
		}
		require.DeepEqual(t, want, atts)
	})
}

func TestProposer_ProposerAtts_dedup(t *testing.T) {
	data1 := util.HydrateAttestationData(&zondpb.AttestationData{
		Slot: 4,
	})
	data2 := util.HydrateAttestationData(&zondpb.AttestationData{
		Slot: 5,
	})
	tests := []struct {
		name string
		atts proposerAtts
		want proposerAtts
	}{
		{
			name: "nil list",
			atts: nil,
			want: proposerAtts(nil),
		},
		{
			name: "empty list",
			atts: proposerAtts{},
			want: proposerAtts{},
		},
		{
			name: "single item",
			atts: proposerAtts{
				&zondpb.Attestation{ParticipationBits: bitfield.Bitlist{}},
			},
			want: proposerAtts{
				&zondpb.Attestation{ParticipationBits: bitfield.Bitlist{}},
			},
		},
		{
			name: "two items no duplicates",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10111110, 0x01}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01111111, 0x01}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01111111, 0x01}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10111110, 0x01}},
			},
		},
		{
			name: "two items with duplicates",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0xba, 0x01}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0xba, 0x01}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0xba, 0x01}},
			},
		},
		{
			name: "sorted no duplicates",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00101011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10100000, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00010000, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00101011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10100000, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00010000, 0b1}},
			},
		},
		{
			name: "sorted with duplicates",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000001, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
			},
		},
		{
			name: "all equal",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
			},
		},
		{
			name: "unsorted no duplicates",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00100010, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10100101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00010000, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10100101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00100010, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00010000, 0b1}},
			},
		},
		{
			name: "unsorted with duplicates",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10100101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10100101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000001, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000001, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10100101, 0b1}},
			},
		},
		{
			name: "no proper subset (same root)",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10000001, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00011001, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00011001, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b10000001, 0b1}},
			},
		},
		{
			name: "proper subset (same root)",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000001, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000001, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
			},
		},
		{
			name: "no proper subset (different root)",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000101, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b10000001, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b00011001, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b00011001, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b10000001, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000101, 0b1}},
			},
		},
		{
			name: "proper subset (different root 1)",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000001, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b00000011, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00000001, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b01101101, 0b1}},
			},
		},
		{
			name: "proper subset (different root 2)",
			atts: proposerAtts{
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b00001111, 0b1}},
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
			},
			want: proposerAtts{
				&zondpb.Attestation{Data: data2, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
				&zondpb.Attestation{Data: data1, ParticipationBits: bitfield.Bitlist{0b11001111, 0b1}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atts, err := tt.atts.dedup()
			if err != nil {
				t.Error(err)
			}
			sort.Slice(atts, func(i, j int) bool {
				if atts[i].ParticipationBits.Count() == atts[j].ParticipationBits.Count() {
					if atts[i].Data.Slot == atts[j].Data.Slot {
						return bytes.Compare(atts[i].ParticipationBits, atts[j].ParticipationBits) <= 0
					}
					return atts[i].Data.Slot > atts[j].Data.Slot
				}
				return atts[i].ParticipationBits.Count() > atts[j].ParticipationBits.Count()
			})
			assert.DeepEqual(t, tt.want, atts)
		})
	}
}
