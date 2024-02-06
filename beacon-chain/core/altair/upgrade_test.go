package altair_test

import (
	"context"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/altair"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestTranslateParticipation(t *testing.T) {
	ctx := context.Background()
	s, _ := util.DeterministicGenesisStateAltair(t, 64)
	require.NoError(t, s.SetSlot(s.Slot()+params.BeaconConfig().MinAttestationInclusionDelay))

	var err error
	newState, err := altair.TranslateParticipation(ctx, s, nil)
	require.NoError(t, err)
	participation, err := newState.PreviousEpochParticipation()
	require.NoError(t, err)
	require.DeepSSZEqual(t, make([]byte, 64), participation)

	aggBits := bitfield.NewBitlist(2)
	aggBits.SetBitAt(0, true)
	aggBits.SetBitAt(1, true)
	r, err := helpers.BlockRootAtSlot(s, 0)
	require.NoError(t, err)
	var pendingAtts []*zondpb.PendingAttestation
	for i := 0; i < 3; i++ {
		pendingAtts = append(pendingAtts, &zondpb.PendingAttestation{
			Data: &zondpb.AttestationData{
				CommitteeIndex:  primitives.CommitteeIndex(i),
				BeaconBlockRoot: r,
				Source:          &zondpb.Checkpoint{Epoch: 0, Root: make([]byte, 32)},
				Target:          &zondpb.Checkpoint{Epoch: 0, Root: make([]byte, 32)},
			},
			AggregationBits: aggBits,
			InclusionDelay:  1,
		})
	}

	newState, err = altair.TranslateParticipation(ctx, newState, pendingAtts)
	require.NoError(t, err)
	participation, err = newState.PreviousEpochParticipation()
	require.NoError(t, err)
	require.DeepNotSSZEqual(t, make([]byte, 64), participation)

	committee, err := helpers.BeaconCommitteeFromState(ctx, s, pendingAtts[0].Data.Slot, pendingAtts[0].Data.CommitteeIndex)
	require.NoError(t, err)
	indices, err := attestation.AttestingIndices(pendingAtts[0].AggregationBits, committee)
	require.NoError(t, err)
	for _, index := range indices {
		has, err := altair.HasValidatorFlag(participation[index], params.BeaconConfig().TimelySourceFlagIndex)
		require.NoError(t, err)
		require.Equal(t, true, has)
		has, err = altair.HasValidatorFlag(participation[index], params.BeaconConfig().TimelyTargetFlagIndex)
		require.NoError(t, err)
		require.Equal(t, true, has)
		has, err = altair.HasValidatorFlag(participation[index], params.BeaconConfig().TimelyHeadFlagIndex)
		require.NoError(t, err)
		require.Equal(t, true, has)
	}
}
