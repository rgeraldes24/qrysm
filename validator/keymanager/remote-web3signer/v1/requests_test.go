package v1_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common/hexutil"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	v1 "github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1/mock"
)

var testGVR = bytes.Repeat([]byte{0x42}, fieldparams.RootLength)

// mapSignRequest calls the builder for the request's oneof.
func mapSignRequest(req *validatorpb.SignRequest, gvr []byte) (any, error) {
	switch req.Object.(type) {
	case *validatorpb.SignRequest_BlockZond, *validatorpb.SignRequest_BlindedBlockZond:
		return v1.GetBlockV2SignRequest(req, gvr)
	case *validatorpb.SignRequest_AttestationData:
		return v1.GetAttestationSignRequest(req, gvr)
	case *validatorpb.SignRequest_Slot:
		return v1.GetAggregationSlotSignRequest(req, gvr)
	case *validatorpb.SignRequest_AggregateAttestationAndProof:
		return v1.GetAggregateAndProofSignRequest(req, gvr)
	case *validatorpb.SignRequest_Exit:
		return v1.GetVoluntaryExitSignRequest(req, gvr)
	case *validatorpb.SignRequest_SyncMessageBlockRoot:
		return v1.GetSyncCommitteeMessageSignRequest(req, gvr)
	case *validatorpb.SignRequest_SyncAggregatorSelectionData:
		return v1.GetSyncCommitteeSelectionProofSignRequest(req, gvr)
	case *validatorpb.SignRequest_ContributionAndProof:
		return v1.GetSyncCommitteeContributionAndProofSignRequest(req, gvr)
	case *validatorpb.SignRequest_Registration:
		return v1.GetValidatorRegistrationSignRequest(req)
	}
	return nil, nil
}

// TestSignRequests_ProduceContractJSON maps every fixture and checks the JSON
// with the strict contract decoder.
func TestSignRequests_ProduceContractJSON(t *testing.T) {
	for _, typ := range mock.SignRequestTypes {
		t.Run(typ, func(t *testing.T) {
			got, err := mapSignRequest(mock.GetMockSignRequest(typ), testGVR)
			require.NoError(t, err)
			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			gotType, err := mock.ValidateSignRequestJSON(encoded)
			require.NoError(t, err)
			want := strings.TrimPrefix(typ, "BLINDED_")
			require.Equal(t, want, gotType)
		})
	}
}

func TestGetBlockV2SignRequest(t *testing.T) {
	for _, typ := range []string{mock.BlockV2, mock.BlindedBlockV2} {
		t.Run(typ, func(t *testing.T) {
			req := mock.GetMockSignRequest(typ)
			got, err := v1.GetBlockV2SignRequest(req, testGVR)
			require.NoError(t, err)

			var b interfaces.ReadOnlyBeaconBlock
			switch o := req.Object.(type) {
			case *validatorpb.SignRequest_BlockZond:
				b, err = blocks.NewBeaconBlock(o.BlockZond)
			case *validatorpb.SignRequest_BlindedBlockZond:
				b, err = blocks.NewBeaconBlock(o.BlindedBlockZond)
			}
			require.NoError(t, err)
			header, err := interfaces.BeaconBlockHeaderFromBlockInterface(b)
			require.NoError(t, err)

			assert.Equal(t, v1.TypeBlockV2, got.Type)
			assert.Equal(t, v1.BlockVersionZond, got.BeaconBlock.Version)
			assert.Equal(t, "7", got.BeaconBlock.BlockHeader.Slot)
			assert.Equal(t, "3", got.BeaconBlock.BlockHeader.ProposerIndex)
			assert.DeepEqual(t, hexutil.Bytes(header.BodyRoot), got.BeaconBlock.BlockHeader.BodyRoot)
			assert.DeepEqual(t, hexutil.Bytes(req.SigningRoot), got.SigningRoot)
			assert.DeepEqual(t, hexutil.Bytes(testGVR), got.ForkInfo.GenesisValidatorsRoot)
		})
	}
}

func TestGetAttestationSignRequest(t *testing.T) {
	got, err := v1.GetAttestationSignRequest(mock.GetMockSignRequest(mock.Attestation), testGVR)
	require.NoError(t, err)
	assert.Equal(t, v1.TypeAttestation, got.Type)
	assert.Equal(t, "7", got.Attestation.Slot)
	assert.Equal(t, "1", got.Attestation.Index)
	assert.Equal(t, "0", got.Attestation.Source.Epoch)
	assert.Equal(t, "1", got.Attestation.Target.Epoch)
}

func TestGetAggregationSlotSignRequest(t *testing.T) {
	got, err := v1.GetAggregationSlotSignRequest(mock.GetMockSignRequest(mock.AggregationSlot), testGVR)
	require.NoError(t, err)
	assert.Equal(t, v1.TypeAggregationSlot, got.Type)
	assert.Equal(t, "7", got.AggregationSlot.Slot)
}

func TestGetAggregateAndProofSignRequest(t *testing.T) {
	got, err := v1.GetAggregateAndProofSignRequest(mock.GetMockSignRequest(mock.AggregateAndProof), testGVR)
	require.NoError(t, err)
	assert.Equal(t, v1.TypeAggregateAndProof, got.Type)
	assert.Equal(t, "5", got.AggregateAndProof.AggregatorIndex)
	// 4 bits with bits 0 and 2 set, plus the delimiter at bit 4.
	assert.DeepEqual(t, hexutil.Bytes{0x15}, got.AggregateAndProof.Aggregate.AggregationBits)
	require.Equal(t, 2, len(got.AggregateAndProof.Aggregate.Signatures))
	assert.DeepEqual(t, hexutil.Bytes(mock.MockSignature(0x02)), got.AggregateAndProof.Aggregate.Signatures[1])
	assert.DeepEqual(t, hexutil.Bytes(mock.MockSignature(0x03)), got.AggregateAndProof.SelectionProof)
}

func TestGetVoluntaryExitSignRequest(t *testing.T) {
	got, err := v1.GetVoluntaryExitSignRequest(mock.GetMockSignRequest(mock.VoluntaryExit), testGVR)
	require.NoError(t, err)
	assert.Equal(t, v1.TypeVoluntaryExit, got.Type)
	assert.Equal(t, "0", got.VoluntaryExit.Epoch)
	assert.Equal(t, "9", got.VoluntaryExit.ValidatorIndex)
}

func TestGetSyncCommitteeMessageSignRequest(t *testing.T) {
	req := mock.GetMockSignRequest(mock.SyncCommitteeMessage)
	got, err := v1.GetSyncCommitteeMessageSignRequest(req, testGVR)
	require.NoError(t, err)
	assert.Equal(t, v1.TypeSyncCommitteeMessage, got.Type)
	// The oneof carries only the root; the slot is the signing slot.
	assert.Equal(t, "7", got.SyncCommitteeMessage.Slot)
	assert.DeepEqual(t, hexutil.Bytes(req.GetSyncMessageBlockRoot()), got.SyncCommitteeMessage.BeaconBlockRoot)
}

func TestGetSyncCommitteeSelectionProofSignRequest(t *testing.T) {
	got, err := v1.GetSyncCommitteeSelectionProofSignRequest(mock.GetMockSignRequest(mock.SyncCommitteeSelectionProof), testGVR)
	require.NoError(t, err)
	assert.Equal(t, v1.TypeSyncCommitteeSelectionProof, got.Type)
	assert.Equal(t, "7", got.SyncAggregatorSelectionData.Slot)
	assert.Equal(t, "0", got.SyncAggregatorSelectionData.SubcommitteeIndex)
}

func TestGetSyncCommitteeContributionAndProofSignRequest(t *testing.T) {
	req := mock.GetMockSignRequest(mock.SyncCommitteeContributionAndProof)
	got, err := v1.GetSyncCommitteeContributionAndProofSignRequest(req, testGVR)
	require.NoError(t, err)
	assert.Equal(t, v1.TypeSyncCommitteeContributionAndProof, got.Type)
	c := got.ContributionAndProof.Contribution
	assert.DeepEqual(t, hexutil.Bytes(mock.MockAggregationBits()), c.AggregationBits)
	assert.Equal(t, fieldparams.SyncCommitteeAggregationBytesLength, len(c.AggregationBits))
	assert.DeepEqual(t, hexutil.Bytes(req.GetContributionAndProof().Contribution.BlockRoot), c.BeaconBlockRoot)
	require.Equal(t, 2, len(c.Signatures))
}

func TestGetValidatorRegistrationSignRequest(t *testing.T) {
	got, err := v1.GetValidatorRegistrationSignRequest(mock.GetMockSignRequest(mock.ValidatorRegistration))
	require.NoError(t, err)
	assert.Equal(t, v1.TypeValidatorRegistration, got.Type)
	assert.Equal(t, "30000000", got.ValidatorRegistration.GasLimit)
	assert.Equal(t, "1760000000", got.ValidatorRegistration.Timestamp)

	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	_, hasForkInfo := decoded["fork_info"]
	assert.Equal(t, false, hasForkInfo, "a registration must not carry fork_info")
	feeRecipient := decoded["validator_registration"].(map[string]any)["fee_recipient"].(string)
	require.Equal(t, "Q", feeRecipient[:1])
	assert.Equal(t, strings.Repeat("0d", fieldparams.FeeRecipientLength), strings.ToLower(feeRecipient[1:]))
}

func TestGetRandaoRevealRequest(t *testing.T) {
	commitment := bytes.Repeat([]byte{0x33}, fieldparams.RandaoCommitmentLength)
	got, err := v1.GetRandaoRevealRequest(62, commitment, testGVR)
	require.NoError(t, err)
	assert.Equal(t, "62", got.Slot)
	assert.DeepEqual(t, hexutil.Bytes(commitment), got.RandaoCommitment)
	assert.DeepEqual(t, hexutil.Bytes(testGVR), got.ForkInfo.GenesisValidatorsRoot)

	_, err = v1.GetRandaoRevealRequest(62, commitment[:31], testGVR)
	require.ErrorContains(t, "invalid randao commitment length", err)
	_, err = v1.GetRandaoRevealRequest(62, commitment, testGVR[:31])
	require.ErrorContains(t, "invalid genesis validators root length", err)
}

func TestSignRequests_InvalidInput(t *testing.T) {
	withSigningRoot := func(typ string, root []byte) *validatorpb.SignRequest {
		req := mock.GetMockSignRequest(typ)
		req.SigningRoot = root
		return req
	}
	tests := []struct {
		name    string
		req     *validatorpb.SignRequest
		gvr     []byte
		wantErr string
	}{
		{
			name:    "short signing root",
			req:     withSigningRoot(mock.Attestation, make([]byte, 31)),
			gvr:     testGVR,
			wantErr: "invalid signing root length",
		},
		{
			name:    "short registration signing root",
			req:     withSigningRoot(mock.ValidatorRegistration, nil),
			wantErr: "invalid signing root length",
		},
		{
			name:    "bad genesis validators root",
			req:     mock.GetMockSignRequest(mock.AggregationSlot),
			gvr:     []byte{1, 2, 3},
			wantErr: "invalid genesis validators root length",
		},
		{
			name: "nil attestation data",
			req: &validatorpb.SignRequest{
				SigningRoot: make([]byte, 32),
				Object:      &validatorpb.SignRequest_AttestationData{},
			},
			gvr:     testGVR,
			wantErr: "attestation data is nil",
		},
		{
			name: "short aggregate signature",
			req: func() *validatorpb.SignRequest {
				req := mock.GetMockSignRequest(mock.AggregateAndProof)
				req.GetAggregateAttestationAndProof().Aggregate.Signatures[0] = make([]byte, 10)
				return req
			}(),
			gvr:     testGVR,
			wantErr: "invalid signature 0 length",
		},
		{
			name: "short contribution block root",
			req: func() *validatorpb.SignRequest {
				req := mock.GetMockSignRequest(mock.SyncCommitteeContributionAndProof)
				req.GetContributionAndProof().Contribution.BlockRoot = make([]byte, 31)
				return req
			}(),
			gvr:     testGVR,
			wantErr: "invalid beacon block root length",
		},
		{
			name: "short sync committee message root",
			req: func() *validatorpb.SignRequest {
				req := mock.GetMockSignRequest(mock.SyncCommitteeMessage)
				req.Object = &validatorpb.SignRequest_SyncMessageBlockRoot{SyncMessageBlockRoot: []byte{1}}
				return req
			}(),
			gvr:     testGVR,
			wantErr: "invalid sync committee message block root length",
		},
		{
			name: "short fee recipient",
			req: func() *validatorpb.SignRequest {
				req := mock.GetMockSignRequest(mock.ValidatorRegistration)
				req.GetRegistration().FeeRecipient = make([]byte, 20)
				return req
			}(),
			wantErr: "invalid fee recipient length",
		},
		{
			name: "nil exit",
			req: &validatorpb.SignRequest{
				SigningRoot: make([]byte, 32),
				Object:      &validatorpb.SignRequest_Exit{},
			},
			gvr:     testGVR,
			wantErr: "failed to cast request object to voluntary exit",
		},
		{
			name: "nil block",
			req: &validatorpb.SignRequest{
				SigningRoot: make([]byte, 32),
				Object:      &validatorpb.SignRequest_BlockZond{},
			},
			gvr:     testGVR,
			wantErr: "zond block is nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mapSignRequest(tt.req, tt.gvr)
			require.ErrorContains(t, tt.wantErr, err)
		})
	}
}

func TestSignRequests_WrongOneof(t *testing.T) {
	req := mock.GetMockSignRequest(mock.AggregationSlot)
	_, err := v1.GetAttestationSignRequest(req, testGVR)
	require.ErrorContains(t, "failed to cast request object to attestation data", err)
	_, err = v1.GetBlockV2SignRequest(req, testGVR)
	require.ErrorContains(t, "failed to cast request object to a zond block", err)
	_, err = v1.GetValidatorRegistrationSignRequest(req)
	require.ErrorContains(t, "failed to cast request object to validator registration", err)
	_, err = v1.GetAttestationSignRequest(nil, testGVR)
	require.ErrorContains(t, "nil sign request provided", err)
}
