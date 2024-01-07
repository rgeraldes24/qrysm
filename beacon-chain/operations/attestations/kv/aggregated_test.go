package kv

import (
	"context"
	"sort"
	"testing"

	c "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	fssz "github.com/prysmaticlabs/fastssz"
	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/config/features"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestKV_Aggregated_AggregateUnaggregatedAttestations(t *testing.T) {
	resetFn := features.InitWithReset(&features.Flags{
		AggregateParallel: true,
	})
	defer resetFn()

	cache := NewAttCaches()
	priv, err := dilithium.RandKey()
	require.NoError(t, err)
	sig1 := priv.Sign([]byte{'a'})
	//sig2 := priv.Sign([]byte{'b'})
	att1 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b1001}, Signatures: [][]byte{sig1.Marshal()}})
	att2 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b1010}, Signatures: [][]byte{sig1.Marshal()}})
	att3 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b1100}, Signatures: [][]byte{sig1.Marshal()}})
	att4 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b1001}, Signatures: [][]byte{sig1.Marshal()}})
	att5 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b1001}, Signatures: [][]byte{sig1.Marshal()}})
	att6 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b1010}, Signatures: [][]byte{sig1.Marshal()}})
	att7 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b1100}, Signatures: [][]byte{sig1.Marshal()}})
	att8 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b1001}, Signatures: [][]byte{sig1.Marshal()}})
	atts := []*zondpb.Attestation{att1, att2, att3, att4, att5, att6, att7, att8}
	require.NoError(t, cache.SaveUnaggregatedAttestations(atts))
	require.NoError(t, cache.AggregateUnaggregatedAttestations(context.Background()))

	require.Equal(t, 1, len(cache.AggregatedAttestationsBySlotIndex(context.Background(), 1, 0)), "Did not aggregate correctly")
	require.Equal(t, 1, len(cache.AggregatedAttestationsBySlotIndex(context.Background(), 2, 0)), "Did not aggregate correctly")
}

func TestKV_Aggregated_SaveAggregatedAttestation(t *testing.T) {
	tests := []struct {
		name          string
		att           *zondpb.Attestation
		count         int
		wantErrString string
	}{
		{
			name:          "nil attestation",
			att:           nil,
			wantErrString: "attestation can't be nil",
		},
		{
			name:          "nil attestation data",
			att:           &zondpb.Attestation{},
			wantErrString: "attestation's data can't be nil",
		},
		{
			name: "not aggregated",
			att: util.HydrateAttestation(&zondpb.Attestation{
				Data: &zondpb.AttestationData{}, ParticipationBits: bitfield.Bitlist{0b10100}}),
			wantErrString: "attestation is not aggregated",
		},
		{
			name: "invalid hash",
			att: &zondpb.Attestation{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					BeaconBlockRoot: []byte{0b0},
				}),
				ParticipationBits: bitfield.Bitlist{0b10111},
			},
			wantErrString: "could not tree hash attestation: --.BeaconBlockRoot (" + fssz.ErrBytesLength.Error() + ")",
		},
		{
			name: "already seen",
			att: util.HydrateAttestation(&zondpb.Attestation{
				Data: &zondpb.AttestationData{
					Slot: 100,
				},
				ParticipationBits: bitfield.Bitlist{0b11101001},
			}),
			count: 0,
		},
		{
			name: "normal save",
			att: util.HydrateAttestation(&zondpb.Attestation{
				Data: &zondpb.AttestationData{
					Slot: 1,
				},
				ParticipationBits: bitfield.Bitlist{0b1101},
			}),
			count: 1,
		},
	}
	r, err := hashFn(util.HydrateAttestationData(&zondpb.AttestationData{
		Slot: 100,
	}))
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			cache.seenAtt.Set(string(r[:]), []bitfield.Bitlist{{0xff}}, c.DefaultExpiration)
			assert.Equal(t, 0, len(cache.unAggregatedAtt), "Invalid start pool, atts: %d", len(cache.unAggregatedAtt))

			err := cache.SaveAggregatedAttestation(tt.att)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, tt.wantErrString, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.count, len(cache.aggregatedAtt), "Wrong attestation count")
			assert.Equal(t, tt.count, cache.AggregatedAttestationCount(), "Wrong attestation count")
		})
	}
}

func TestKV_Aggregated_SaveAggregatedAttestations(t *testing.T) {
	tests := []struct {
		name          string
		atts          []*zondpb.Attestation
		count         int
		wantErrString string
	}{
		{
			name: "no duplicates",
			atts: []*zondpb.Attestation{
				util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1},
					ParticipationBits: bitfield.Bitlist{0b1101}}),
				util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1},
					ParticipationBits: bitfield.Bitlist{0b1101}}),
			},
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			assert.Equal(t, 0, len(cache.aggregatedAtt), "Invalid start pool, atts: %d", len(cache.unAggregatedAtt))
			err := cache.SaveAggregatedAttestations(tt.atts)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, tt.wantErrString, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.count, len(cache.aggregatedAtt), "Wrong attestation count")
			assert.Equal(t, tt.count, cache.AggregatedAttestationCount(), "Wrong attestation count")
		})
	}
}

func TestKV_Aggregated_SaveAggregatedAttestations_SomeGoodSomeBad(t *testing.T) {
	tests := []struct {
		name          string
		atts          []*zondpb.Attestation
		count         int
		wantErrString string
	}{
		{
			name: "the first attestation is bad",
			atts: []*zondpb.Attestation{
				util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1},
					ParticipationBits: bitfield.Bitlist{0b1100}}),
				util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1},
					ParticipationBits: bitfield.Bitlist{0b1101}}),
			},
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			assert.Equal(t, 0, len(cache.aggregatedAtt), "Invalid start pool, atts: %d", len(cache.unAggregatedAtt))
			err := cache.SaveAggregatedAttestations(tt.atts)
			if tt.wantErrString != "" {
				assert.ErrorContains(t, tt.wantErrString, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.count, len(cache.aggregatedAtt), "Wrong attestation count")
			assert.Equal(t, tt.count, cache.AggregatedAttestationCount(), "Wrong attestation count")
		})
	}
}

func TestKV_Aggregated_AggregatedAttestations(t *testing.T) {
	cache := NewAttCaches()

	att1 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b1101}})
	att2 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b1101}})
	att3 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 3}, ParticipationBits: bitfield.Bitlist{0b1101}})
	atts := []*zondpb.Attestation{att1, att2, att3}

	for _, att := range atts {
		require.NoError(t, cache.SaveAggregatedAttestation(att))
	}

	returned := cache.AggregatedAttestations()
	sort.Slice(returned, func(i, j int) bool {
		return returned[i].Data.Slot < returned[j].Data.Slot
	})
	assert.DeepSSZEqual(t, atts, returned)
}

func TestKV_Aggregated_DeleteAggregatedAttestation(t *testing.T) {
	t.Run("nil attestation", func(t *testing.T) {
		cache := NewAttCaches()
		assert.ErrorContains(t, "attestation can't be nil", cache.DeleteAggregatedAttestation(nil))
		att := util.HydrateAttestation(&zondpb.Attestation{ParticipationBits: bitfield.Bitlist{0b10101}, Data: &zondpb.AttestationData{Slot: 2}})
		assert.NoError(t, cache.DeleteAggregatedAttestation(att))
	})
	t.Run("non aggregated attestation", func(t *testing.T) {
		cache := NewAttCaches()
		att := util.HydrateAttestation(&zondpb.Attestation{ParticipationBits: bitfield.Bitlist{0b1001}, Data: &zondpb.AttestationData{Slot: 2}})
		err := cache.DeleteAggregatedAttestation(att)
		assert.ErrorContains(t, "attestation is not aggregated", err)
	})
	t.Run("invalid hash", func(t *testing.T) {
		cache := NewAttCaches()
		att := &zondpb.Attestation{
			ParticipationBits: bitfield.Bitlist{0b1111},
			Data: &zondpb.AttestationData{
				Slot:   2,
				Source: &zondpb.Checkpoint{},
				Target: &zondpb.Checkpoint{},
			},
		}
		err := cache.DeleteAggregatedAttestation(att)
		wantErr := "could not tree hash attestation data: --.BeaconBlockRoot (" + fssz.ErrBytesLength.Error() + ")"
		assert.ErrorContains(t, wantErr, err)
	})
	t.Run("nonexistent attestation", func(t *testing.T) {
		cache := NewAttCaches()
		att := util.HydrateAttestation(&zondpb.Attestation{ParticipationBits: bitfield.Bitlist{0b1111}, Data: &zondpb.AttestationData{Slot: 2}})
		assert.NoError(t, cache.DeleteAggregatedAttestation(att))
	})
	t.Run("non-filtered deletion", func(t *testing.T) {
		// NOTE(rgeraldes24): this test is not ok on the original repo even though it passes
		// could not create signature from byte slice: could not unmarshal bytes into signature
		// which leads to a deletion of an attestation on the SaveAggregatedAttestations that
		// ends up influencing the final result; the result should be two attestations instead
		// of just att2 because att3 and att4 are aggregated which should lead to a filtered deletion
		// since we ask to delete att3 below
		// I've disabled att4 for now to have a non-filtered deletion
		priv, err := dilithium.RandKey()
		require.NoError(t, err)
		// sig0 := priv.Sign([]byte{'a'}).Marshal()
		sig1 := priv.Sign([]byte{'b'}).Marshal()
		// sig2 := priv.Sign([]byte{'c'}).Marshal()
		sig3 := priv.Sign([]byte{'d'}).Marshal()
		cache := NewAttCaches()
		att1 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b11010}, Signatures: [][]byte{sig1, sig3}})
		att2 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b11010}, Signatures: [][]byte{sig1, sig3}})
		att3 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 3}, ParticipationBits: bitfield.Bitlist{0b11010}, Signatures: [][]byte{sig1, sig3}})
		// att4 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 3}, ParticipationBits: bitfield.Bitlist{0b10101}, Signatures: [][]byte{sig0, sig2}})
		atts := []*zondpb.Attestation{att1, att2, att3 /*att4*/}
		require.NoError(t, cache.SaveAggregatedAttestations(atts))
		require.NoError(t, cache.DeleteAggregatedAttestation(att1))
		require.NoError(t, cache.DeleteAggregatedAttestation(att3))

		returned := cache.AggregatedAttestations()
		wanted := []*zondpb.Attestation{att2}
		assert.DeepEqual(t, wanted, returned)
	})
	t.Run("filtered deletion", func(t *testing.T) {
		cache := NewAttCaches()
		att1 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b110101}})
		att2 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b110111}})
		att3 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b110100}})
		att4 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 2}, ParticipationBits: bitfield.Bitlist{0b110101}})
		atts := []*zondpb.Attestation{att1, att2, att3, att4}
		require.NoError(t, cache.SaveAggregatedAttestations(atts))

		assert.Equal(t, 2, cache.AggregatedAttestationCount(), "Unexpected number of atts")
		require.NoError(t, cache.DeleteAggregatedAttestation(att4))

		returned := cache.AggregatedAttestations()
		wanted := []*zondpb.Attestation{att1, att2}
		sort.Slice(returned, func(i, j int) bool {
			return string(returned[i].ParticipationBits) < string(returned[j].ParticipationBits)
		})
		assert.DeepEqual(t, wanted, returned)
	})
}

func TestKV_Aggregated_HasAggregatedAttestation(t *testing.T) {
	tests := []struct {
		name     string
		existing []*zondpb.Attestation
		input    *zondpb.Attestation
		want     bool
		err      error
	}{
		{
			name:  "nil attestation",
			input: nil,
			want:  false,
			err:   errors.New("can't be nil"),
		},
		{
			name: "nil attestation data",
			input: &zondpb.Attestation{
				ParticipationBits: bitfield.Bitlist{0b1111},
			},
			want: false,
			err:  errors.New("can't be nil"),
		},
		{
			name: "empty cache aggregated",
			input: util.HydrateAttestation(&zondpb.Attestation{
				Data: &zondpb.AttestationData{
					Slot: 1,
				},
				ParticipationBits: bitfield.Bitlist{0b1111}}),
			want: false,
		},
		{
			name: "empty cache unaggregated",
			input: util.HydrateAttestation(&zondpb.Attestation{
				Data: &zondpb.AttestationData{
					Slot: 1,
				},
				ParticipationBits: bitfield.Bitlist{0b1001}}),
			want: false,
		},
		{
			name: "single attestation in cache with exact match",
			existing: []*zondpb.Attestation{{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1111}},
			},
			input: &zondpb.Attestation{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1111}},
			want: true,
		},
		{
			name: "single attestation in cache with subset aggregation",
			existing: []*zondpb.Attestation{{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1111}},
			},
			input: &zondpb.Attestation{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1110}},
			want: true,
		},
		{
			name: "single attestation in cache with superset aggregation",
			existing: []*zondpb.Attestation{{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1110}},
			},
			input: &zondpb.Attestation{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1111}},
			want: false,
		},
		{
			name: "multiple attestations with same data in cache with overlapping aggregation, input is subset",
			existing: []*zondpb.Attestation{
				{
					Data: util.HydrateAttestationData(&zondpb.AttestationData{
						Slot: 1,
					}),
					ParticipationBits: bitfield.Bitlist{0b1111000},
				},
				{
					Data: util.HydrateAttestationData(&zondpb.AttestationData{
						Slot: 1,
					}),
					ParticipationBits: bitfield.Bitlist{0b1100111},
				},
			},
			input: &zondpb.Attestation{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1100000}},
			want: true,
		},
		{
			name: "multiple attestations with same data in cache with overlapping aggregation and input is superset",
			existing: []*zondpb.Attestation{
				{
					Data: util.HydrateAttestationData(&zondpb.AttestationData{
						Slot: 1,
					}),
					ParticipationBits: bitfield.Bitlist{0b1111000},
				},
				{
					Data: util.HydrateAttestationData(&zondpb.AttestationData{
						Slot: 1,
					}),
					ParticipationBits: bitfield.Bitlist{0b1100111},
				},
			},
			input: &zondpb.Attestation{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1111111}},
			want: false,
		},
		{
			name: "multiple attestations with different data in cache",
			existing: []*zondpb.Attestation{
				{
					Data: util.HydrateAttestationData(&zondpb.AttestationData{
						Slot: 2,
					}),
					ParticipationBits: bitfield.Bitlist{0b1111000},
				},
				{
					Data: util.HydrateAttestationData(&zondpb.AttestationData{
						Slot: 3,
					}),
					ParticipationBits: bitfield.Bitlist{0b1100111},
				},
			},
			input: &zondpb.Attestation{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 1,
				}),
				ParticipationBits: bitfield.Bitlist{0b1111111}},
			want: false,
		},
		{
			name: "attestations with different bitlist lengths",
			existing: []*zondpb.Attestation{
				{
					Data: util.HydrateAttestationData(&zondpb.AttestationData{
						Slot: 2,
					}),
					ParticipationBits: bitfield.Bitlist{0b1111000},
				},
			},
			input: &zondpb.Attestation{
				Data: util.HydrateAttestationData(&zondpb.AttestationData{
					Slot: 2,
				}),
				ParticipationBits: bitfield.Bitlist{0b1111},
			},
			want: false,
			err:  bitfield.ErrBitlistDifferentLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewAttCaches()
			require.NoError(t, cache.SaveAggregatedAttestations(tt.existing))

			if tt.input != nil && tt.input.Signatures == nil {
				tt.input.Signatures = [][]byte{}
			}

			if tt.err != nil {
				_, err := cache.HasAggregatedAttestation(tt.input)
				require.ErrorContains(t, tt.err.Error(), err)
			} else {
				result, err := cache.HasAggregatedAttestation(tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)

				// Same test for block attestations
				cache = NewAttCaches()
				assert.NoError(t, cache.SaveBlockAttestations(tt.existing))

				result, err = cache.HasAggregatedAttestation(tt.input)
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestKV_Aggregated_DuplicateAggregatedAttestations(t *testing.T) {
	cache := NewAttCaches()

	att1 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b1101}})
	att2 := util.HydrateAttestation(&zondpb.Attestation{Data: &zondpb.AttestationData{Slot: 1}, ParticipationBits: bitfield.Bitlist{0b1111}})
	atts := []*zondpb.Attestation{att1, att2}

	for _, att := range atts {
		require.NoError(t, cache.SaveAggregatedAttestation(att))
	}

	returned := cache.AggregatedAttestations()

	// It should have only returned att2.
	assert.DeepSSZEqual(t, att2, returned[0], "Did not receive correct aggregated atts")
	assert.Equal(t, 1, len(returned), "Did not receive correct aggregated atts")
}
