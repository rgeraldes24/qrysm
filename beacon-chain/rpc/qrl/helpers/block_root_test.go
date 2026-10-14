package helpers

import (
	"context"
	"testing"

	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
)

func TestBlockRootFromState(t *testing.T) {
	ctx := context.Background()

	t.Run("zero header state root", func(t *testing.T) {
		// A state at the slot of its latest block: the header state root is not filled yet.
		st, err := util.NewBeaconStateZond()
		require.NoError(t, err)
		stateRoot, err := st.HashTreeRoot(ctx)
		require.NoError(t, err)
		header := st.LatestBlockHeader()
		header.StateRoot = stateRoot[:]
		want, err := header.HashTreeRoot()
		require.NoError(t, err)

		got, err := BlockRootFromState(ctx, st)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("filled header state root", func(t *testing.T) {
		// A state advanced past its latest block: the header already carries the state root.
		st, err := util.NewBeaconStateZond()
		require.NoError(t, err)
		header := st.LatestBlockHeader()
		header.StateRoot = bytesutil.PadTo([]byte("state root"), 32)
		require.NoError(t, st.SetLatestBlockHeader(header))
		want, err := header.HashTreeRoot()
		require.NoError(t, err)

		got, err := BlockRootFromState(ctx, st)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}
