package state_native_test

import (
	"testing"

	state_native "github.com/theQRL/qrysm/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/config/params"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	zond "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
)

func BenchmarkAppendExecutionNodeDataVotes(b *testing.B) {
	st, err := state_native.InitializeFromProtoCapella(&qrysmpb.BeaconStateCapella{})
	require.NoError(b, err)

	max := params.BeaconConfig().ExecutionNodeDataVotesLength()

	if max < 2 {
		b.Fatalf("ExecutionNodeDataVotesLength is less than 2")
	}

	for i := uint64(0); i < max-2; i++ {
		err := st.AppendExecutionNodeDataVotes(&qrysmpb.ExecutionNodeData{
			DepositCount: i,
			DepositRoot:  make([]byte, 64),
			BlockHash:    make([]byte, 64),
		})
		require.NoError(b, err)
	}

	ref := st.Copy()

	for i := 0; i < b.N; i++ {
		err := ref.AppendExecutionNodeDataVotes(&zond.ExecutionNodeData{DepositCount: uint64(i)})
		require.NoError(b, err)
		ref = st.Copy()
	}
}
