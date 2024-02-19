package blockchain

import (
	"context"
	"testing"

	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestReportEpochMetrics_BadHeadState(t *testing.T) {
	s, err := util.NewBeaconStateCapella()
	require.NoError(t, err)
	h, err := util.NewBeaconStateCapella()
	require.NoError(t, err)
	require.NoError(t, h.SetValidators(nil))
	err = reportEpochMetrics(context.Background(), s, h)
	// require.ErrorContains(t, "failed to initialize precompute: state has nil validator slice", err)
	// NOTE(rgeraldes24) - initial message in the capella flow is different; double check
	require.ErrorContains(t, "could not read every validator: state has nil validator slice", err)
}

// TODO(rgeraldes24) - same as below, old flow, replace with new tests?
/*
func TestReportEpochMetrics_BadAttestation(t *testing.T) {
	s, err := util.NewBeaconStateCapella()
	require.NoError(t, err)
	h, err := util.NewBeaconStateCapella()
	require.NoError(t, err)
	// TODO(rgeraldes24)
	// require.NoError(t, h.AppendCurrentEpochAttestations(&zond.PendingAttestation{InclusionDelay: 0}))
	err = reportEpochMetrics(context.Background(), s, h)
	require.ErrorContains(t, "attestation with inclusion delay of 0", err)
}
*/

// NOTE(rgeraldes24) This test is no longer valid since it uses the phase0 flow
// that uses the AttestedCurrentEpoch func which throws this error.
// The Capella flow includes the participation bits to identify the ones that attested
// and that func (AttestedCurrentEpoch) is no longer necessary.
/*
func TestReportEpochMetrics_SlashedValidatorOutOfBound(t *testing.T) {
	h, _ := util.DeterministicGenesisStateCapella(t, 1)
	v, err := h.ValidatorAtIndex(0)
	require.NoError(t, err)
	v.Slashed = true
	require.NoError(t, h.UpdateValidatorAtIndex(0, v))
	// TODO(rgeraldes24)
	// require.NoError(t, h.AppendCurrentEpochAttestations(&zond.PendingAttestation{InclusionDelay: 1, Data: util.HydrateAttestationData(&zond.AttestationData{})}))
	err = reportEpochMetrics(context.Background(), h, h)
	require.ErrorContains(t, "slot 0 out of bounds", err)
}
*/
