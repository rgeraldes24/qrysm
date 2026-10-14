package util

import (
	"testing"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
)

func TestHydrateAttestation(t *testing.T) {
	a := HydrateAttestation(&qrysmpb.Attestation{})
	_, err := a.HashTreeRoot()
	require.NoError(t, err)
	require.DeepEqual(t, a.Signatures[0], make([]byte, fieldparams.MLDSA87SignatureLength))
}

func TestHydrateAttestationData(t *testing.T) {
	d := HydrateAttestationData(&qrysmpb.AttestationData{})
	_, err := d.HashTreeRoot()
	require.NoError(t, err)
	require.DeepEqual(t, d.BeaconBlockRoot, make([]byte, fieldparams.RootLength))
	require.DeepEqual(t, d.Target.Root, make([]byte, fieldparams.RootLength))
	require.DeepEqual(t, d.Source.Root, make([]byte, fieldparams.RootLength))
}

func TestHydrateIndexedAttestation(t *testing.T) {
	a := &qrysmpb.IndexedAttestation{}
	a = HydrateIndexedAttestation(a)
	_, err := a.HashTreeRoot()
	require.NoError(t, err)
	_, err = a.Data.HashTreeRoot()
	require.NoError(t, err)
}

func TestGenerateAttestations_EpochBoundary(t *testing.T) {
	gs, pk := DeterministicGenesisStateZond(t, 32)
	_, err := GenerateAttestations(gs, pk, 1, params.BeaconConfig().SlotsPerEpoch, false)
	require.NoError(t, err)
}
