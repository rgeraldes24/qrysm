package field_params_test

import (
	"fmt"
	"testing"

	"github.com/theQRL/go-bitfield"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
)

func testFieldParametersMatchConfig(t *testing.T) {
	require.Equal(t, uint64(params.BeaconConfig().SlotsPerHistoricalRoot), uint64(fieldparams.BlockRootsLength))
	require.Equal(t, uint64(params.BeaconConfig().SlotsPerHistoricalRoot), uint64(fieldparams.StateRootsLength))
	require.Equal(t, params.BeaconConfig().HistoricalRootsLimit, uint64(fieldparams.HistoricalRootsLength))
	require.Equal(t, uint64(params.BeaconConfig().EpochsPerHistoricalVector), uint64(fieldparams.RandaoMixesLength))
	require.Equal(t, params.BeaconConfig().ValidatorRegistryLimit, uint64(fieldparams.ValidatorRegistryLimit))
	require.Equal(t, uint64(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerExecutionVotingPeriod))), uint64(fieldparams.ExecutionDataVotesLength))
	require.Equal(t, uint64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().MaxAttestations)), uint64(fieldparams.PreviousEpochAttestationsLength))
	require.Equal(t, uint64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().MaxAttestations)), uint64(fieldparams.CurrentEpochAttestationsLength))
	require.Equal(t, uint64(params.BeaconConfig().EpochsPerSlashingsVector), uint64(fieldparams.SlashingsLength))
	require.Equal(t, params.BeaconConfig().SyncCommitteeSize, uint64(fieldparams.SyncCommitteeLength))
	require.Equal(t, params.BeaconConfig().MaxValidatorsPerCommittee, uint64(fieldparams.MaxValidatorsPerCommittee))
	require.Equal(t, params.BeaconConfig().MaxProposerSlashings, uint64(fieldparams.MaxProposerSlashings))
	require.Equal(t, params.BeaconConfig().MaxAttesterSlashings, uint64(fieldparams.MaxAttesterSlashings))
	require.Equal(t, params.BeaconConfig().MaxAttestations, uint64(fieldparams.MaxAttestations))
	require.Equal(t, params.BeaconConfig().MaxDeposits, uint64(fieldparams.MaxDeposits))
	require.Equal(t, params.BeaconConfig().MaxVoluntaryExits, uint64(fieldparams.MaxVoluntaryExits))
	require.Equal(t, params.BeaconConfig().MaxWithdrawalsPerPayload, uint64(fieldparams.MaxWithdrawalsPerPayload))
}

func TestAttestationCommitteeLimitMatchesSSZ(t *testing.T) {
	// Exercise the compiled codec, so the validation bound cannot drift away
	// from the bitlist limit in the protobuf schema or generated SSZ code.
	const limit uint64 = fieldparams.MaxValidatorsPerCommittee
	for _, size := range []uint64{limit, limit + 1, 2 * limit} {
		t.Run(fmt.Sprintf("committee_size_%d", size), func(t *testing.T) {
			att := &qrysmpb.Attestation{
				AggregationBits: bitfield.NewBitlist(size),
				Data: &qrysmpb.AttestationData{
					BeaconBlockRoot: make([]byte, fieldparams.RootLength),
					Source:          &qrysmpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
					Target:          &qrysmpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
				},
				Signatures: [][]byte{make([]byte, fieldparams.MLDSA87SignatureLength)},
			}
			att.AggregationBits.SetBitAt(0, true)
			encoded, err := att.MarshalSSZ()
			decoded := new(qrysmpb.Attestation)
			if err == nil {
				err = decoded.UnmarshalSSZ(encoded)
			}
			if size > fieldparams.MaxValidatorsPerCommittee {
				require.NotNil(t, err, "committee above the compiled limit must not round-trip")
				return
			}
			require.NoError(t, err)
			require.DeepEqual(t, att, decoded)
		})
	}
}
