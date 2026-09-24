package kv

import (
	"context"
	"fmt"
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/state/genesis"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
	bolt "go.etcd.io/bbolt"
)

func TestSaveOrigin(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	// Embedded Genesis works with Mainnet config
	params.OverrideBeaconConfig(params.MainnetConfig().Copy())

	ctx := context.Background()
	db := setupDB(t)

	st, err := genesis.State(params.MainnetName)
	require.NoError(t, err)

	sb, err := st.MarshalSSZ()
	require.NoError(t, err)
	require.NoError(t, db.LoadGenesis(ctx, sb))

	// this is necessary for mainnet, because LoadGenesis is short-circuited by the embedded state,
	// so the genesis root key is never written to the db.
	require.NoError(t, db.EnsureEmbeddedGenesis(ctx))

	cst, err := util.NewBeaconStateZond()
	require.NoError(t, err)
	csb, err := cst.MarshalSSZ()
	require.NoError(t, err)
	cb := util.NewBeaconBlockZond()
	scb, err := blocks.NewSignedBeaconBlock(cb)
	require.NoError(t, err)
	cbb, err := scb.MarshalSSZ()
	require.NoError(t, err)
	require.NoError(t, db.SaveOrigin(ctx, csb, cbb))

	broot, err := scb.Block().HashTreeRoot()
	require.NoError(t, err)
	require.Equal(t, true, db.IsFinalizedBlock(ctx, broot))
	validated, err := db.LastValidatedCheckpoint(ctx)
	require.NoError(t, err)
	require.DeepEqual(t, broot[:], validated.Root)
}

func TestSaveOrigin_ActiveValidatorCapacity(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	if fieldparams.Preset == "minimal" {
		cfg := params.MinimalSpecConfig().Copy()
		cfg.ConfigName = params.MainnetTestName
		params.FillTestVersions(cfg, 128)
		require.NoError(t, params.SetActive(cfg))
	}
	cfg := params.BeaconConfig()
	capacity, err := cfg.MaxActiveValidators()
	require.NoError(t, err)
	ctx := context.Background()
	const currentEpoch = primitives.Epoch(10)
	for _, tc := range []struct {
		name            string
		active          uint64
		scheduled       uint64
		inactive        uint64
		activationEpoch primitives.Epoch
		exitEpoch       primitives.Epoch // Optional exit for the first active validator.
		wantEpoch       primitives.Epoch // Zero means the import should succeed.
	}{
		{name: "at capacity", active: capacity},
		{name: "above capacity", active: capacity + 1, wantEpoch: currentEpoch},
		{name: "inactive records beyond capacity", active: capacity, inactive: 2},
		{name: "exit at checkpoint epoch", active: capacity + 1, exitEpoch: currentEpoch},
		{name: "scheduled activations reach capacity", active: capacity - 1, scheduled: 1, activationEpoch: currentEpoch + 1},
		{name: "scheduled activations exceed capacity", active: capacity, scheduled: 1, activationEpoch: currentEpoch + 1, wantEpoch: currentEpoch + 1},
		{name: "same epoch replacement at capacity", active: capacity, scheduled: 1, activationEpoch: currentEpoch + 1, exitEpoch: currentEpoch + 1},
		{name: "earlier exit frees capacity", active: capacity, scheduled: 1, activationEpoch: currentEpoch + 2, exitEpoch: currentEpoch + 1},
		{name: "overflow before later exit", active: capacity, scheduled: 1, activationEpoch: currentEpoch + 1, exitEpoch: currentEpoch + 2, wantEpoch: currentEpoch + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupDB(t)
			require.NoError(t, db.SaveGenesisData(ctx, genesisStateWithValidatorCounts(t, 1, 0)))
			genesisRoot, err := db.GenesisBlockRoot(ctx)
			require.NoError(t, err)

			cst := genesisStateWithValidatorCounts(t, tc.active, tc.scheduled+tc.inactive)
			require.NoError(t, cst.SetSlot(primitives.Slot(currentEpoch)*cfg.SlotsPerEpoch))
			for i := uint64(0); i < tc.active+tc.scheduled; i++ {
				validator, err := cst.ValidatorAtIndex(primitives.ValidatorIndex(i))
				require.NoError(t, err)
				// A genesis-only check would miss every active validator in this checkpoint.
				validator.ActivationEpoch = 5
				if i >= tc.active {
					validator.ActivationEpoch = tc.activationEpoch
					validator.ActivationEligibilityEpoch = 4
				}
				if i == 0 && tc.exitEpoch != 0 {
					validator.ExitEpoch = tc.exitEpoch
				}
				require.NoError(t, cst.UpdateValidatorAtIndex(primitives.ValidatorIndex(i), validator))
			}
			csb, err := cst.MarshalSSZ()
			require.NoError(t, err)
			stateRoot, err := cst.HashTreeRoot(ctx)
			require.NoError(t, err)
			cb := util.NewBeaconBlockZond()
			cb.Block.Slot = cst.Slot()
			cb.Block.ParentRoot = genesisRoot[:]
			cb.Block.StateRoot = stateRoot[:]
			scb, err := blocks.NewSignedBeaconBlock(cb)
			require.NoError(t, err)
			cbb, err := scb.MarshalSSZ()
			require.NoError(t, err)
			blockRoot, err := scb.Block().HashTreeRoot()
			require.NoError(t, err)

			var transactionID int
			require.NoError(t, db.db.View(func(tx *bolt.Tx) error {
				transactionID = tx.ID()
				return nil
			}))
			err = db.SaveOrigin(ctx, csb, cbb)
			if tc.wantEpoch != 0 {
				require.ErrorContains(t, fmt.Sprintf("checkpoint active validator count %d at epoch %d exceeds committee capacity %d", capacity+1, tc.wantEpoch, capacity), err)
				// An unchanged transaction ID proves no write was committed to any
				// bucket, including the backfill marker that used to be saved first.
				require.NoError(t, db.db.View(func(tx *bolt.Tx) error {
					require.Equal(t, transactionID, tx.ID(), "rejected checkpoint modified the database")
					return nil
				}))
				_, err = db.BackfillBlockRoot(ctx)
				require.ErrorIs(t, err, ErrNotFoundBackfillBlockRoot)
				_, err = db.OriginCheckpointBlockRoot(ctx)
				require.ErrorIs(t, err, ErrNotFoundOriginBlockRoot)
				headRoot, err := db.HeadBlockRoot()
				require.NoError(t, err)
				require.Equal(t, genesisRoot, headRoot)
				require.Equal(t, false, db.HasStateSummary(ctx, blockRoot))
				return
			}
			require.NoError(t, err)
			originRoot, err := db.OriginCheckpointBlockRoot(ctx)
			require.NoError(t, err)
			require.Equal(t, blockRoot, originRoot)
			backfillRoot, err := db.BackfillBlockRoot(ctx)
			require.NoError(t, err)
			require.Equal(t, genesisRoot, backfillRoot)
			require.Equal(t, true, db.IsFinalizedBlock(ctx, blockRoot))
			validated, err := db.LastValidatedCheckpoint(ctx)
			require.NoError(t, err)
			require.Equal(t, currentEpoch, validated.Epoch)
			require.DeepEqual(t, blockRoot[:], validated.Root)
			loaded, err := db.State(ctx, blockRoot)
			require.NoError(t, err)
			require.NotNil(t, loaded)
			loadedRoot, err := loaded.HashTreeRoot(ctx)
			require.NoError(t, err)
			require.Equal(t, stateRoot, loadedRoot)
		})
	}
}
