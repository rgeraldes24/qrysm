package submit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/stakingdeposit"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/testing/require"
)

func TestImportDepositDataJSON(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "deposit_data-1.json"), []byte(`[{"amount": 5}]`), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "keystore-1.json"), []byte(`{}`), 0600))
		depositDataList, err := importDepositDataJSON(dir)
		require.NoError(t, err)
		require.Equal(t, 1, len(depositDataList))
		require.Equal(t, uint64(5), depositDataList[0].Amount)
	})
	t.Run("no file", func(t *testing.T) {
		_, err := importDepositDataJSON(t.TempDir())
		require.ErrorContains(t, "deposit data file not found", err)
	})
	t.Run("several files", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "deposit_data-1.json"), []byte(`[]`), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "deposit_data-2.json"), []byte(`[]`), 0600))
		_, err := importDepositDataJSON(dir)
		require.ErrorContains(t, "found 2 deposit data files", err)
	})
}

func TestSubmittableDepositTotal(t *testing.T) {
	cfg := params.BeaconConfig()
	depositDataList := []*stakingdeposit.DepositData{
		{Amount: cfg.MaxEffectiveBalance},
		{Amount: cfg.MinDepositAmount - 1},
		{Amount: cfg.MinDepositAmount},
		{Amount: cfg.MaxEffectiveBalance + 1},
	}
	total, num := submittableDepositTotal(depositDataList)
	require.Equal(t, cfg.MaxEffectiveBalance+cfg.MinDepositAmount, total)
	require.Equal(t, 2, num)
}
