package blocks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/core/blocks"
	state_native "github.com/theQRL/qrysm/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/runtime/version"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
	"google.golang.org/protobuf/proto"
)

func FakeDeposits(n uint64) []*zondpb.ExecutionNodeData {
	deposits := make([]*zondpb.ExecutionNodeData, n)
	for i := uint64(0); i < n; i++ {
		deposits[i] = &zondpb.ExecutionNodeData{
			DepositCount: 1,
			DepositRoot:  bytesutil.PadTo([]byte("root"), 32),
		}
	}
	return deposits
}

func TestExecutionNodeDataHasEnoughSupport(t *testing.T) {
	tests := []struct {
		stateVotes         []*zondpb.ExecutionNodeData
		data               *zondpb.ExecutionNodeData
		hasSupport         bool
		votingPeriodLength primitives.Epoch
	}{
		{
			stateVotes: FakeDeposits(uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))),
			data: &zondpb.ExecutionNodeData{
				DepositCount: 1,
				DepositRoot:  bytesutil.PadTo([]byte("root"), 32),
			},
			hasSupport:         true,
			votingPeriodLength: 7,
		}, {
			stateVotes: FakeDeposits(uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))),
			data: &zondpb.ExecutionNodeData{
				DepositCount: 1,
				DepositRoot:  bytesutil.PadTo([]byte("root"), 32),
			},
			hasSupport:         false,
			votingPeriodLength: 8,
		}, {
			stateVotes: FakeDeposits(uint64(params.BeaconConfig().SlotsPerEpoch.Mul(4))),
			data: &zondpb.ExecutionNodeData{
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
			c.EpochsPerEth1VotingPeriod = tt.votingPeriodLength
			params.OverrideBeaconConfig(c)

			s, err := state_native.InitializeFromProtoCapella(&zondpb.BeaconStateCapella{
				ExecutionNodeDataVotes: tt.stateVotes,
			})
			require.NoError(t, err)
			result, err := blocks.ExecutionNodeDataHasEnoughSupport(s, tt.data)
			require.NoError(t, err)

			if result != tt.hasSupport {
				t.Errorf(
					"blocks.ExecutionNodeDataHasEnoughSupport(%+v) = %t, wanted %t",
					tt.data,
					result,
					tt.hasSupport,
				)
			}
		})
	}
}

func TestAreExecutionNodeDataEqual(t *testing.T) {
	type args struct {
		a *zondpb.ExecutionNodeData
		b *zondpb.ExecutionNodeData
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
				b: &zondpb.ExecutionNodeData{
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
				a: &zondpb.ExecutionNodeData{
					DepositRoot:  make([]byte, 32),
					DepositCount: 0,
					BlockHash:    make([]byte, 32),
				},
				b: &zondpb.ExecutionNodeData{
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
				a: &zondpb.ExecutionNodeData{
					DepositRoot:  make([]byte, 32),
					DepositCount: 0,
					BlockHash:    make([]byte, 32),
				},
				b: &zondpb.ExecutionNodeData{
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
			assert.Equal(t, tt.want, blocks.AreExecutionNodeDataEqual(tt.args.a, tt.args.b))
		})
	}
}

func TestProcessExecutionNodeData_SetsCorrectly(t *testing.T) {
	beaconState, err := state_native.InitializeFromProtoCapella(&zondpb.BeaconStateCapella{
		ExecutionNodeDataVotes: []*zondpb.ExecutionNodeData{},
	})
	require.NoError(t, err)

	b := util.NewBeaconBlockCapella()
	b.Block = &zondpb.BeaconBlockCapella{
		Body: &zondpb.BeaconBlockBodyCapella{
			ExecutionNodeData: &zondpb.ExecutionNodeData{
				DepositRoot: []byte{2},
				BlockHash:   []byte{3},
			},
		},
	}

	period := uint64(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().EpochsPerEth1VotingPeriod)))
	for i := uint64(0); i < period; i++ {
		processedState, err := blocks.ProcessExecutionNodeDataInBlock(context.Background(), beaconState, b.Block.Body.ExecutionNodeData)
		require.NoError(t, err)
		require.Equal(t, true, processedState.Version() == version.Capella)
	}

	newExecutionNodeDataVotes := beaconState.ExecutionNodeDataVotes()
	if len(newExecutionNodeDataVotes) <= 1 {
		t.Error("Expected new execution node data votes to have length > 1")
	}
	if !proto.Equal(beaconState.ExecutionNodeData(), zondpb.CopyExecutionNodeData(b.Block.Body.ExecutionNodeData)) {
		t.Errorf(
			"Expected latest execution node data to have been set to %v, received %v",
			b.Block.Body.ExecutionNodeData,
			beaconState.ExecutionNodeData(),
		)
	}
}
