package util

import (
	"context"
	"reflect"
	"testing"

	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func TestNewBeaconState(t *testing.T) {
	st, err := NewBeaconState()
	require.NoError(t, err)
	b, err := st.MarshalSSZ()
	require.NoError(t, err)
	got := &zondpb.BeaconState{}
	require.NoError(t, got.UnmarshalSSZ(b))
	if !reflect.DeepEqual(st.ToProtoUnsafe(), got) {
		t.Fatal("State did not match after round trip marshal")
	}
}

func TestNewBeaconState_HashTreeRoot(t *testing.T) {
	st, err := NewBeaconState()
	require.NoError(t, err)
	_, err = st.HashTreeRoot(context.Background())
	require.NoError(t, err)
}
