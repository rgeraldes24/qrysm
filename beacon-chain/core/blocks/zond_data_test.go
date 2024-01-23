package blocks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	state_native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
	"google.golang.org/protobuf/proto"
)

func FakeDeposits(n uint64) []*zondpb.ZondData {
	deposits := make([]*zondpb.ZondData, n)
	for i := uint64(0); i < n; i++ {
		deposits[i] = &zondpb.ZondData{
			DepositCount: 1,
			DepositRoot:  bytesutil.PadTo([]byte("root"), 32),
		}
	}
	return deposits
}

func TestZondDataHasEnoughSupport(t *testing.T) {
	tests := []struct {
		stateVotes         []*zondpb.ZondData
		data               *zondpb.ZondData
		hasSupport         bool
		votingPeriodLength primitives.Epoch
	}{
		{
			stateVotes: FakeDeposits(uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))),
			data: &zondpb.ZondData{
				DepositCount: 1,
				DepositRoot:  bytesutil.PadTo([]byte("root"), 32),
			},
			hasSupport:         true,
			votingPeriodLength: 7,
		}, {
			stateVotes: FakeDeposits(uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))),
			data: &zondpb.ZondData{
				DepositCount: 1,
				DepositRoot:  bytesutil.PadTo([]byte("root"), 32),
			},
			hasSupport:         false,
			votingPeriodLength: 8,
		}, {
			stateVotes: FakeDeposits(uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))),
			data: &zondpb.ZondData{
				DepositCount: 1,
				DepositRoot:  bytesutil.PadTo([]byte("root"), 32),
			},
			hasSupport:         false,
			votingPeriodLength: 10,
		},
	}

	params.SetupTestConfigCleanup(t)
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			c := params.BeaconConfig()
			c.EpochsPerZondVotingPeriod = tt.votingPeriodLength
			params.OverrideBeaconConfig(c)

			s, err := state_native.InitializeFromProtoPhase0(&zondpb.BeaconState{
				ZondDataVotes: tt.stateVotes,
			})
			require.NoError(t, err)
			result, err := blocks.ZondDataHasEnoughSupport(s, tt.data)
			require.NoError(t, err)

			if result != tt.hasSupport {
				t.Errorf(
					"blocks.ZondDataHasEnoughSupport(%+v) = %t, wanted %t",
					tt.data,
					result,
					tt.hasSupport,
				)
			}
		})
	}
}

func TestAreZondDataEqual(t *testing.T) {
	type args struct {
		a *zondpb.ZondData
		b *zondpb.ZondData
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "true when both are nil",
			args: args{
				a: nil,
				b: nil,
			},
			want: true,
		},
		{
			name: "false when only one is nil",
			args: args{
				a: nil,
				b: &zondpb.ZondData{
					DepositRoot:  make([]byte, 32),
					DepositCount: 0,
					BlockHash:    make([]byte, 32),
				},
			},
			want: false,
		},
		{
			name: "true when real equality",
			args: args{
				a: &zondpb.ZondData{
					DepositRoot:  make([]byte, 32),
					DepositCount: 0,
					BlockHash:    make([]byte, 32),
				},
				b: &zondpb.ZondData{
					DepositRoot:  make([]byte, 32),
					DepositCount: 0,
					BlockHash:    make([]byte, 32),
				},
			},
			want: true,
		},
		{
			name: "false is field value differs",
			args: args{
				a: &zondpb.ZondData{
					DepositRoot:  make([]byte, 32),
					DepositCount: 0,
					BlockHash:    make([]byte, 32),
				},
				b: &zondpb.ZondData{
					DepositRoot:  make([]byte, 32),
					DepositCount: 64,
					BlockHash:    make([]byte, 32),
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, blocks.AreZondDataEqual(tt.args.a, tt.args.b))
		})
	}
}

func TestProcessZondData_SetsCorrectly(t *testing.T) {
	beaconState, err := state_native.InitializeFromProtoPhase0(&zondpb.BeaconState{
		ZondDataVotes: []*zondpb.ZondData{},
	})
	require.NoError(t, err)

	b := util.NewBeaconBlock()
	b.Block = &zondpb.BeaconBlock{
		Body: &zondpb.BeaconBlockBody{
			ZondData: &zondpb.ZondData{
				DepositRoot: []byte{2},
				BlockHash:   []byte{3},
			},
		},
	}

	period := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerZondVotingPeriod)))
	for i := uint64(0); i < period; i++ {
		processedState, err := blocks.ProcessZondDataInBlock(context.Background(), beaconState, b.Block.Body.ZondData)
		require.NoError(t, err)
		require.Equal(t, true, processedState.Version() == version.Phase0)
	}

	newZondDataVotes := beaconState.ZondDataVotes()
	if len(newZondDataVotes) <= 1 {
		t.Error("Expected new Zond data votes to have length > 1")
	}
	if !proto.Equal(beaconState.ZondData(), zondpb.CopyZondData(b.Block.Body.ZondData)) {
		t.Errorf(
			"Expected latest zond data to have been set to %v, received %v",
			b.Block.Body.ZondData,
			beaconState.ZondData(),
		)
	}
}
