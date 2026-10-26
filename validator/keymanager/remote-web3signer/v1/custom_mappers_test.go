package v1_test

import (
	"encoding/json"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
	v1 "github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1/mock"
)

func TestMapForkInfo(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.E2ETestConfig())
	genesis := hexutil.Bytes(params.BeaconConfig().GenesisForkVersion)

	for _, slot := range []uint64{0, 61, 1 << 40} {
		got, err := v1.MapForkInfo(primitives.Slot(slot), testGVR)
		require.NoError(t, err)
		// qrysm has a single fork: {G, G, "0"} at every slot.
		assert.DeepEqual(t, genesis, got.Fork.PreviousVersion)
		assert.DeepEqual(t, genesis, got.Fork.CurrentVersion)
		assert.Equal(t, "0", got.Fork.Epoch)
		assert.DeepEqual(t, hexutil.Bytes(testGVR), got.GenesisValidatorsRoot)
	}
	encoded, err := json.Marshal(&v1.ForkInfo{Fork: &v1.Fork{
		PreviousVersion: genesis, CurrentVersion: genesis, Epoch: "0",
	}, GenesisValidatorsRoot: testGVR})
	require.NoError(t, err)
	assert.Equal(t,
		`{"fork":{"previous_version":"0x000000fd","current_version":"0x000000fd","epoch":"0"},"genesis_validators_root":"0x4242424242424242424242424242424242424242424242424242424242424242"}`,
		string(encoded))
}

func TestMapAttestation(t *testing.T) {
	tests := []struct {
		name    string
		in      *qrysmpb.Attestation
		wantErr string
		wantLen int
	}{
		{
			name: "no participants encodes an empty list",
			in: &qrysmpb.Attestation{
				AggregationBits: bitfield.NewBitlist(4),
				Data:            util.HydrateAttestationData(&qrysmpb.AttestationData{}),
			},
			wantLen: 0,
		},
		{
			name: "two participants",
			in: util.HydrateAttestation(&qrysmpb.Attestation{
				AggregationBits: mock.MockAttestationBits(),
				Signatures:      [][]byte{mock.MockSignature(1), mock.MockSignature(2)},
			}),
			wantLen: 2,
		},
		{name: "nil", in: nil, wantErr: "attestation is nil"},
		{
			name:    "empty bits",
			in:      &qrysmpb.Attestation{Data: util.HydrateAttestationData(&qrysmpb.AttestationData{})},
			wantErr: "aggregation bits in attestation are empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.MapAttestation(tt.in)
			if tt.wantErr != "" {
				require.ErrorContains(t, tt.wantErr, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantLen, len(got.Signatures))
			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			var decoded map[string]any
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			sigs, ok := decoded["signatures"].([]any)
			require.Equal(t, true, ok, "signatures must encode as a list, never null")
			require.Equal(t, tt.wantLen, len(sigs))
			_, singular := decoded["signature"]
			assert.Equal(t, false, singular)
		})
	}
}

func TestMapAttestationData(t *testing.T) {
	data := util.HydrateAttestationData(&qrysmpb.AttestationData{Slot: 12, CommitteeIndex: 3})
	got, err := v1.MapAttestationData(data)
	require.NoError(t, err)
	assert.Equal(t, "12", got.Slot)
	assert.Equal(t, "3", got.Index)

	data.Target = nil
	_, err = v1.MapAttestationData(data)
	require.ErrorContains(t, "could not map target for attestation data", err)
	_, err = v1.MapAttestationData(nil)
	require.ErrorContains(t, "attestation data is nil", err)
}

func TestMapAggregateAndProof(t *testing.T) {
	_, err := v1.MapAggregateAndProof(nil)
	require.ErrorContains(t, "aggregate attestation and proof is nil", err)
	_, err = v1.MapAggregateAndProof(&qrysmpb.AggregateAttestationAndProof{
		Aggregate:      util.HydrateAttestation(&qrysmpb.Attestation{AggregationBits: bitfield.NewBitlist(1)}),
		SelectionProof: []byte{1},
	})
	require.ErrorContains(t, "invalid selection proof length", err)
}

func TestMapContributionAndProof(t *testing.T) {
	_, err := v1.MapContributionAndProof(nil)
	require.ErrorContains(t, "contribution and proof is nil", err)
	_, err = v1.MapContributionAndProof(&qrysmpb.ContributionAndProof{})
	require.ErrorContains(t, "sync committee contribution is nil", err)
	_, err = v1.MapContributionAndProof(&qrysmpb.ContributionAndProof{
		Contribution: &qrysmpb.SyncCommitteeContribution{},
	})
	require.ErrorContains(t, "aggregation bits in sync committee contribution are empty", err)
}

func TestMapSyncAggregatorSelectionData(t *testing.T) {
	got, err := v1.MapSyncAggregatorSelectionData(&qrysmpb.SyncAggregatorSelectionData{Slot: 18446744073709551615})
	require.NoError(t, err)
	assert.Equal(t, "18446744073709551615", got.Slot)
	assert.Equal(t, "0", got.SubcommitteeIndex)
	_, err = v1.MapSyncAggregatorSelectionData(nil)
	require.ErrorContains(t, "sync aggregator selection data is nil", err)
}
