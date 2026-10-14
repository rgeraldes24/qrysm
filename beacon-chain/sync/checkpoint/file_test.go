package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	dbtest "github.com/theQRL/qrysm/beacon-chain/db/testing"
	"github.com/theQRL/qrysm/testing/require"
)

func TestFileInitializer_StateReadErrorNamesStatePath(t *testing.T) {
	dir := t.TempDir()
	blockPath := filepath.Join(dir, "block.ssz")
	require.NoError(t, os.WriteFile(blockPath, []byte{0x01}, 0600))
	statePath := filepath.Join(dir, "missing-state.ssz")

	fi := &FileInitializer{blockPath: blockPath, statePath: statePath}
	err := fi.Initialize(context.Background(), dbtest.SetupDB(t))
	require.ErrorContains(t, "error reading state file "+statePath, err)
}
