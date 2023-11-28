package v1_test

/*
import (
	"reflect"
	"testing"

	"github.com/theQRL/go-zond/common/hexutil"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	validatorpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/v4/testing/require"
	v1 "github.com/theQRL/qrysm/v4/validator/keymanager/remote-web3signer/v1"
	"github.com/theQRL/qrysm/v4/validator/keymanager/remote-web3signer/v1/mock"
)

func TestGetAggregateAndProofSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.AggregateAndProofSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("AGGREGATE_AND_PROOF"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want:    mock.MockAggregateAndProofSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetAggregateAndProofSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAggregateAndProofSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAggregateAndProofSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAggregationSlotSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.AggregationSlotSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("AGGREGATION_SLOT"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want:    mock.MockAggregationSlotSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetAggregationSlotSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAggregationSlotSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAggregationSlotSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAttestationSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.AttestationSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("ATTESTATION"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want: mock.MockAttestationSignRequest(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetAttestationSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAttestationSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAttestationSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBlockSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.BlockSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("BLOCK"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want:    mock.MockBlockSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetBlockSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBlockSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBlockSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRandaoRevealSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.RandaoRevealSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("RANDAO_REVEAL"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want:    mock.MockRandaoRevealSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetRandaoRevealSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRandaoRevealSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetRandaoRevealSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSyncCommitteeContributionAndProofSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.SyncCommitteeContributionAndProofSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want:    mock.MockSyncCommitteeContributionAndProofSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetSyncCommitteeContributionAndProofSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSyncCommitteeContributionAndProofSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSyncCommitteeContributionAndProofSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSyncCommitteeMessageSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.SyncCommitteeMessageSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("SYNC_COMMITTEE_MESSAGE"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want:    mock.MockSyncCommitteeMessageSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetSyncCommitteeMessageSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSyncCommitteeMessageSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSyncCommitteeMessageSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSyncCommitteeSelectionProofSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.SyncCommitteeSelectionProofSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("SYNC_COMMITTEE_SELECTION_PROOF"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want:    mock.MockSyncCommitteeSelectionProofSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetSyncCommitteeSelectionProofSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSyncCommitteeSelectionProofSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSyncCommitteeSelectionProofSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetVoluntaryExitSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.VoluntaryExitSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request:               mock.GetMockSignRequest("VOLUNTARY_EXIT"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want:    mock.MockVoluntaryExitSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetVoluntaryExitSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVoluntaryExitSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetVoluntaryExitSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBlockBlindedSignRequest(t *testing.T) {
	type args struct {
		request               *validatorpb.SignRequest
		genesisValidatorsRoot []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.BlockBlindedSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test non blinded Capella",
			args: args{
				request:               mock.GetMockSignRequest("BLOCK_CAPELLA"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want: mock.MockBlockBlindedSignRequest(func(t *testing.T) []byte {
				bytevalue, err := hexutil.Decode("0x74b4bb048d39c75f175fbb2311062eb9867d79b712907f39544fcaf2d7e1b433")
				require.NoError(t, err)
				return bytevalue
			}(t), "CAPELLA"),
			wantErr: false,
		},
		{
			name: "Happy Path Test blinded Capella",
			args: args{
				request:               mock.GetMockSignRequest("BLOCK_BLINDED_CAPELLA"),
				genesisValidatorsRoot: make([]byte, fieldparams.RootLength),
			},
			want: mock.MockBlockBlindedSignRequest(func(t *testing.T) []byte {
				bytevalue, err := hexutil.Decode("0x54797f27f45a91d2cf4d73e509c62e464d648ec34e07ddba946adee742039e76")
				require.NoError(t, err)
				return bytevalue
			}(t), "CAPELLA"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetBlockBlindedSignRequest(tt.args.request, tt.args.genesisValidatorsRoot)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBlockBlindedSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBlockBlindedSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetValidatorRegistrationSignRequest(t *testing.T) {
	type args struct {
		request *validatorpb.SignRequest
	}
	tests := []struct {
		name    string
		args    args
		want    *v1.ValidatorRegistrationSignRequest
		wantErr bool
	}{
		{
			name: "Happy Path Test",
			args: args{
				request: mock.GetMockSignRequest("VALIDATOR_REGISTRATION"),
			},
			want:    mock.MockValidatorRegistrationSignRequest(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v1.GetValidatorRegistrationSignRequest(tt.args.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValidatorRegistrationSignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetValidatorRegistrationSignRequest() got = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
