package blockchain

import (
	"context"
	"errors"
	"testing"

	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/qrysm/beacon-chain/core/blocks"
	forkchoicetypes "github.com/theQRL/qrysm/beacon-chain/forkchoice/types"
	"github.com/theQRL/qrysm/config/features"
	"github.com/theQRL/qrysm/config/params"
	consensusblocks "github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
)

func Test_setupForkchoiceCheckpoints_BalanceFailure(t *testing.T) {
	service, tr := minimalTestService(t)
	st, _ := util.DeterministicGenesisStateZond(t, 64)
	require.NoError(t, service.saveGenesisData(tr.ctx, st))
	root := service.originBlockRoot
	require.NoError(t, service.cfg.BeaconDB.SaveJustifiedCheckpoint(tr.ctx, &qrysmpb.Checkpoint{Epoch: 1, Root: root[:]}))
	previous := service.cfg.ForkChoiceStore.JustifiedCheckpoint()
	attempts := 0
	service.cfg.ForkChoiceStore.SetBalancesByRooter(func(context.Context, *forkchoicetypes.Checkpoint) (*forkchoicetypes.JustifiedBalances, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("temporary checkpoint-state read failure")
		}
		return &forkchoicetypes.JustifiedBalances{Balances: []uint64{32}, TotalActiveBalance: 32}, nil
	})
	require.ErrorContains(t, "could not initialize justified checkpoint and balances", service.setupForkchoiceCheckpoints())
	require.DeepEqual(t, previous, service.cfg.ForkChoiceStore.JustifiedCheckpoint())

	require.NoError(t, service.setupForkchoiceCheckpoints())
	require.Equal(t, 2, attempts)
	require.DeepEqual(t, &forkchoicetypes.Checkpoint{Epoch: 1, Root: root}, service.cfg.ForkChoiceStore.JustifiedCheckpoint())
}

func Test_startupHeadRoot(t *testing.T) {
	service, tr := minimalTestService(t)
	ctx := tr.ctx
	hook := logTest.NewGlobal()
	cp := service.FinalizedCheckpt()
	require.DeepEqual(t, cp.Root, params.BeaconConfig().ZeroHash[:])
	gr := [32]byte{'r', 'o', 'o', 't'}
	service.originBlockRoot = gr
	require.NoError(t, service.cfg.BeaconDB.SaveGenesisBlockRoot(ctx, gr))
	t.Run("start from finalized", func(t *testing.T) {
		require.Equal(t, service.startupHeadRoot(), gr)
	})
	t.Run("head requested, error path", func(t *testing.T) {
		resetCfg := features.InitWithReset(&features.Flags{
			ForceHead: "head",
		})
		defer resetCfg()
		require.Equal(t, service.startupHeadRoot(), gr)
		require.LogsContain(t, hook, "Could not get head block root, starting with justified block as head")
	})

	st, _ := util.DeterministicGenesisStateZond(t, 64)
	hr := [32]byte{'h', 'e', 'a', 'd'}
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st, hr), "Could not save genesis state")
	require.NoError(t, service.cfg.BeaconDB.SaveHeadBlockRoot(ctx, hr), "Could not save genesis state")
	require.NoError(t, service.cfg.BeaconDB.SaveHeadBlockRoot(ctx, hr))

	t.Run("start from head", func(t *testing.T) {
		resetCfg := features.InitWithReset(&features.Flags{
			ForceHead: "head",
		})
		defer resetCfg()
		require.Equal(t, service.startupHeadRoot(), hr)
	})
}

func Test_setupForkchoiceTree_Finalized(t *testing.T) {
	service, tr := minimalTestService(t)
	ctx := tr.ctx

	st, _ := util.DeterministicGenesisStateZond(t, 64)
	stateRoot, err := st.HashTreeRoot(ctx)
	require.NoError(t, err, "Could not hash genesis state")

	require.NoError(t, service.saveGenesisData(ctx, st))

	genesis := blocks.NewGenesisBlock(stateRoot[:])
	wsb, err := consensusblocks.NewSignedBeaconBlock(genesis)
	require.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wsb), "Could not save genesis block")
	parentRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err, "Could not get signing root")
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st, parentRoot), "Could not save genesis state")
	require.NoError(t, service.cfg.BeaconDB.SaveHeadBlockRoot(ctx, parentRoot), "Could not save genesis state")
	require.NoError(t, service.cfg.BeaconDB.SaveJustifiedCheckpoint(ctx, &qrysmpb.Checkpoint{Root: parentRoot[:]}))
	require.NoError(t, service.cfg.BeaconDB.SaveFinalizedCheckpoint(ctx, &qrysmpb.Checkpoint{Root: parentRoot[:]}))
	require.NoError(t, service.setupForkchoiceTree(st))
	require.Equal(t, 1, service.cfg.ForkChoiceStore.NodeCount())
}

func Test_setupForkchoiceTree_Head(t *testing.T) {
	service, tr := minimalTestService(t)
	ctx := tr.ctx
	resetCfg := features.InitWithReset(&features.Flags{
		ForceHead: "head",
	})
	defer resetCfg()

	genesisState, keys := util.DeterministicGenesisStateZond(t, 64)
	stateRoot, err := genesisState.HashTreeRoot(ctx)
	require.NoError(t, err, "Could not hash genesis state")
	genesis := blocks.NewGenesisBlock(stateRoot[:])
	wsb, err := consensusblocks.NewSignedBeaconBlock(genesis)
	require.NoError(t, err)
	genesisRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err, "Could not get signing root")
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wsb), "Could not save genesis block")
	require.NoError(t, service.saveGenesisData(ctx, genesisState))

	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, genesisState, genesisRoot), "Could not save genesis state")
	require.NoError(t, service.cfg.BeaconDB.SaveHeadBlockRoot(ctx, genesisRoot), "Could not save genesis state")

	st, err := service.HeadState(ctx)
	require.NoError(t, err)
	b, err := util.GenerateFullBlockZond(st, keys, util.DefaultBlockGenConfig(), primitives.Slot(1))
	require.NoError(t, err)
	wsb, err = consensusblocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	root, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	preState, err := service.getBlockPreState(ctx, wsb.Block())
	require.NoError(t, err)
	postState, err := service.validateStateTransition(ctx, preState, wsb)
	require.NoError(t, err)
	require.NoError(t, service.savePostStateInfo(ctx, root, wsb, postState))

	b, err = util.GenerateFullBlockZond(postState, keys, util.DefaultBlockGenConfig(), primitives.Slot(2))
	require.NoError(t, err)
	wsb, err = consensusblocks.NewSignedBeaconBlock(b)
	require.NoError(t, err)
	root, err = b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, service.savePostStateInfo(ctx, root, wsb, preState))

	require.NoError(t, service.cfg.BeaconDB.SaveHeadBlockRoot(ctx, root))
	cp := service.FinalizedCheckpt()
	fRoot := service.ensureRootNotZeros([32]byte(cp.Root))
	require.NotEqual(t, fRoot, root)
	require.Equal(t, root, service.startupHeadRoot())
	require.NoError(t, service.setupForkchoiceTree(st))
	require.Equal(t, 3, service.cfg.ForkChoiceStore.NodeCount())
}

// Regression test: the justified checkpoint in the DB references a root whose
// block is absent (BeaconDB.Block returns nil, nil for it). Startup must fall
// back to the finalized block as head instead of panicking on the nil block.
func Test_setupForkchoiceTree_MissingHeadBlock(t *testing.T) {
	service, tr := minimalTestService(t)
	ctx := tr.ctx
	hook := logTest.NewGlobal()

	st, _ := util.DeterministicGenesisStateZond(t, 64)
	stateRoot, err := st.HashTreeRoot(ctx)
	require.NoError(t, err, "Could not hash genesis state")
	require.NoError(t, service.saveGenesisData(ctx, st))

	genesis := blocks.NewGenesisBlock(stateRoot[:])
	wsb, err := consensusblocks.NewSignedBeaconBlock(genesis)
	require.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wsb), "Could not save genesis block")
	genesisRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err, "Could not get signing root")
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st, genesisRoot), "Could not save genesis state")

	// The justified checkpoint points to a root whose state exists but whose
	// block is not in the DB, e.g. left behind by an unclean stop.
	missingRoot := [32]byte{'m', 'i', 's', 's', 'i', 'n', 'g'}
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, st, missingRoot), "Could not save state")
	require.NoError(t, service.cfg.BeaconDB.SaveJustifiedCheckpoint(ctx, &qrysmpb.Checkpoint{Epoch: 1, Root: missingRoot[:]}))
	require.NoError(t, service.cfg.BeaconDB.SaveFinalizedCheckpoint(ctx, &qrysmpb.Checkpoint{Root: genesisRoot[:]}))
	require.NoError(t, service.setupForkchoiceCheckpoints())

	blk, err := service.cfg.BeaconDB.Block(ctx, missingRoot)
	require.NoError(t, err)
	require.Equal(t, true, blk == nil)
	require.Equal(t, missingRoot, service.startupHeadRoot())

	require.NoError(t, service.setupForkchoiceTree(st))
	require.LogsContain(t, hook, "starting with finalized block as head")
	require.Equal(t, 1, service.cfg.ForkChoiceStore.NodeCount())
}

// Regression test: the head block is in the DB but an ancestor between it and
// the finalized block is not. The chain walk must surface an error so startup
// falls back to the finalized block as head instead of panicking.
func Test_setupForkchoiceTree_MissingAncestor(t *testing.T) {
	service, tr := minimalTestService(t)
	ctx := tr.ctx
	hook := logTest.NewGlobal()

	genesisState, keys := util.DeterministicGenesisStateZond(t, 64)
	stateRoot, err := genesisState.HashTreeRoot(ctx)
	require.NoError(t, err, "Could not hash genesis state")
	genesis := blocks.NewGenesisBlock(stateRoot[:])
	wsb, err := consensusblocks.NewSignedBeaconBlock(genesis)
	require.NoError(t, err)
	genesisRoot, err := genesis.Block.HashTreeRoot()
	require.NoError(t, err, "Could not get signing root")
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wsb), "Could not save genesis block")
	require.NoError(t, service.saveGenesisData(ctx, genesisState))
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, genesisState, genesisRoot), "Could not save genesis state")

	// Block 1 is processed to derive its post state but never saved to the DB.
	st, err := service.HeadState(ctx)
	require.NoError(t, err)
	b1, err := util.GenerateFullBlockZond(st, keys, util.DefaultBlockGenConfig(), primitives.Slot(1))
	require.NoError(t, err)
	wsb1, err := consensusblocks.NewSignedBeaconBlock(b1)
	require.NoError(t, err)
	preState, err := service.getBlockPreState(ctx, wsb1.Block())
	require.NoError(t, err)
	postState, err := service.validateStateTransition(ctx, preState, wsb1)
	require.NoError(t, err)

	// Block 2, child of the missing block 1, is the startup head.
	b2, err := util.GenerateFullBlockZond(postState, keys, util.DefaultBlockGenConfig(), primitives.Slot(2))
	require.NoError(t, err)
	wsb2, err := consensusblocks.NewSignedBeaconBlock(b2)
	require.NoError(t, err)
	root2, err := b2.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, service.cfg.BeaconDB.SaveBlock(ctx, wsb2))
	require.NoError(t, service.cfg.BeaconDB.SaveState(ctx, postState, root2))

	require.NoError(t, service.cfg.BeaconDB.SaveJustifiedCheckpoint(ctx, &qrysmpb.Checkpoint{Epoch: 1, Root: root2[:]}))
	require.NoError(t, service.cfg.BeaconDB.SaveFinalizedCheckpoint(ctx, &qrysmpb.Checkpoint{Root: genesisRoot[:]}))
	require.NoError(t, service.setupForkchoiceCheckpoints())
	require.Equal(t, root2, service.startupHeadRoot())

	require.NoError(t, service.setupForkchoiceTree(genesisState))
	require.LogsContain(t, hook, "Could not build forkchoice chain, starting with finalized block as head")
	require.Equal(t, 1, service.cfg.ForkChoiceStore.NodeCount())
}
