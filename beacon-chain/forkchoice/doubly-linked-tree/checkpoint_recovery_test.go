package doublylinkedtree

import (
	"context"
	"errors"
	"testing"

	"github.com/theQRL/go-bitfield"
	forkchoicetypes "github.com/theQRL/qrysm/beacon-chain/forkchoice/types"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
)

func TestForkChoice_CheckpointBalanceFailureCanRetry(t *testing.T) {
	ctx := context.Background()
	f := setup(2, 0)
	epochSlots := params.BeaconConfig().SlotsPerEpoch
	driftGenesisTime(f, 4*epochSlots+8, 30)
	f.justifiedBalances = []uint64{32, 31}
	j, a, b, c, d := indexToHash(1), indexToHash(2), indexToHash(3), indexToHash(4), indexToHash(5)
	for _, block := range []struct {
		slot         primitives.Slot
		root, parent [32]byte
	}{{2 * epochSlots, j, [32]byte{}}, {3 * epochSlots, a, j}, {3*epochSlots + 1, b, a}, {3*epochSlots + 2, c, a}} {
		st, ro, err := prepareForkchoiceState(ctx, block.slot, block.root, block.parent, block.root, 2, 0)
		require.NoError(t, err)
		require.NoError(t, f.InsertNode(ctx, st, ro))
	}
	require.NoError(t, f.UpdateJustifiedCheckpoint(ctx, &forkchoicetypes.Checkpoint{Epoch: 2, Root: j}))
	f.ProcessAttestation(ctx, []uint64{0}, b, 3)
	f.ProcessAttestation(ctx, []uint64{1}, c, 3)
	head, err := f.Head(ctx)
	require.NoError(t, err)
	require.Equal(t, b, head)

	attempts := 0
	f.SetBalancesByRooter(func(context.Context, *forkchoicetypes.Checkpoint) (*forkchoicetypes.JustifiedBalances, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("temporary checkpoint-state read failure")
		}
		return &forkchoicetypes.JustifiedBalances{Balances: []uint64{31, 32}, TotalActiveBalance: 63}, nil
	})
	st, ro, err := prepareForkchoiceState(ctx, 4*epochSlots, d, c, d, 3, 0)
	require.NoError(t, err)
	require.NoError(t, st.SetCurrentJustifiedCheckpoint(&qrysmpb.Checkpoint{Epoch: 3, Root: a[:]}))
	require.ErrorContains(t, "temporary checkpoint-state read failure", f.InsertNode(ctx, st, ro))
	require.Equal(t, false, f.HasNode(d))
	require.DeepEqual(t, &forkchoicetypes.Checkpoint{Epoch: 2, Root: j}, f.JustifiedCheckpoint())
	require.DeepEqual(t, []uint64{32, 31}, f.justifiedBalances)

	// A retransmission succeeds, and the newly justified state should now
	// determine the winning branch: validator 1 outweighs validator 0.
	require.NoError(t, f.InsertNode(ctx, st, ro))
	head, err = f.Head(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.DeepEqual(t, []uint64{31, 32}, f.justifiedBalances)
	require.Equal(t, d, head)
}

func TestForkChoice_CheckpointUpdatesPreserveBalancesOnFailure(t *testing.T) {
	for _, mode := range []string{"setter", "epoch boundary"} {
		for _, failure := range []string{"read error", "nil balances"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				ctx := context.Background()
				f := setup(2, 0)
				epochSlots := params.BeaconConfig().SlotsPerEpoch
				driftGenesisTime(f, 4*epochSlots, 30)
				oldRoot, newRoot := indexToHash(1), indexToHash(2)
				_, oldBlock, err := prepareForkchoiceState(ctx, 2*epochSlots, oldRoot, [32]byte{}, oldRoot, 1, 0)
				require.NoError(t, err)
				_, err = f.store.insert(ctx, oldBlock, 1, 0)
				require.NoError(t, err)
				_, newBlock, err := prepareForkchoiceState(ctx, 3*epochSlots, newRoot, oldRoot, newRoot, 2, 0)
				require.NoError(t, err)
				node, err := f.store.insert(ctx, newBlock, 2, 0)
				require.NoError(t, err)
				oldCheckpoint := &forkchoicetypes.Checkpoint{Epoch: 2, Root: oldRoot}
				newCheckpoint := &forkchoicetypes.Checkpoint{Epoch: 3, Root: newRoot}
				f.justifiedBalances = []uint64{32, 31}
				require.NoError(t, f.UpdateJustifiedCheckpoint(ctx, oldCheckpoint))
				previous := f.PreviousJustifiedCheckpoint()
				f.store.committeeWeight = 100
				node.unrealizedJustifiedEpoch = newCheckpoint.Epoch
				node.unrealizedJustifiedRoot = newCheckpoint.Root
				f.store.unrealizedJustifiedCheckpoint = newCheckpoint
				attempts := 0
				f.SetBalancesByRooter(func(context.Context, *forkchoicetypes.Checkpoint) (*forkchoicetypes.JustifiedBalances, error) {
					attempts++
					if attempts == 1 {
						if failure == "nil balances" {
							return nil, nil
						}
						return nil, errors.New("temporary read error")
					}
					return &forkchoicetypes.JustifiedBalances{
						Balances: []uint64{31, 32, 16}, TotalActiveBalance: 200 * uint64(epochSlots),
					}, nil
				})
				update := func() error {
					if mode == "setter" {
						return f.UpdateJustifiedCheckpoint(ctx, newCheckpoint)
					}
					return f.NewSlot(ctx, 4*epochSlots)
				}
				require.ErrorContains(t, "could not update justified balances", update())
				require.DeepEqual(t, oldCheckpoint, f.JustifiedCheckpoint())
				require.DeepEqual(t, previous, f.PreviousJustifiedCheckpoint())
				require.DeepEqual(t, []uint64{32, 31}, f.justifiedBalances)
				require.Equal(t, uint64(100), f.store.committeeWeight)
				require.Equal(t, uint64(2), f.numActiveValidators)

				require.NoError(t, update())
				require.Equal(t, 2, attempts)
				require.DeepEqual(t, newCheckpoint, f.JustifiedCheckpoint())
				require.DeepEqual(t, oldCheckpoint, f.PreviousJustifiedCheckpoint())
				require.DeepEqual(t, []uint64{31, 32, 16}, f.justifiedBalances)
				require.Equal(t, uint64(200), f.store.committeeWeight)
				require.Equal(t, uint64(3), f.numActiveValidators)
			})
		}
	}
}

func TestStore_PruneSameRootAtLaterEpoch(t *testing.T) {
	ctx := context.Background()
	f := setup(0, 0)
	epochSlots := params.BeaconConfig().SlotsPerEpoch
	driftGenesisTime(f, 4*epochSlots, 30)
	a, b, c := indexToHash(1), indexToHash(2), indexToHash(3)
	for _, block := range []struct {
		slot         primitives.Slot
		root, parent [32]byte
	}{{epochSlots - 1, a, [32]byte{}}, {epochSlots + 1, b, a}, {2*epochSlots + 1, c, a}} {
		_, ro, err := prepareForkchoiceState(ctx, block.slot, block.root, block.parent, block.root, 0, 0)
		require.NoError(t, err)
		_, err = f.store.insert(ctx, ro, 0, 0)
		require.NoError(t, err)
	}
	require.NoError(t, f.UpdateJustifiedCheckpoint(ctx, &forkchoicetypes.Checkpoint{Epoch: 2, Root: a}))
	require.NoError(t, f.UpdateFinalizedCheckpoint(&forkchoicetypes.Checkpoint{Epoch: 1, Root: a}))
	require.NoError(t, f.store.prune(ctx))
	require.Equal(t, true, f.HasNode(b))

	// The same block can be the checkpoint root of consecutive epochs when
	// the canonical branch skipped all intervening slots. The branch through
	// b was compatible with checkpoint 1, but conflicts with checkpoint 2.
	require.NoError(t, f.UpdateJustifiedCheckpoint(ctx, &forkchoicetypes.Checkpoint{Epoch: 3, Root: c}))
	require.NoError(t, f.UpdateFinalizedCheckpoint(&forkchoicetypes.Checkpoint{Epoch: 2, Root: a}))
	require.NoError(t, f.store.prune(ctx))
	require.Equal(t, false, f.HasNode(b))
	require.Equal(t, true, f.HasNode(c))
	require.Equal(t, 1, len(f.store.treeRootNode.children))
	require.Equal(t, c, f.store.treeRootNode.children[0].root)
	_, exists := f.store.nodeByPayload[b]
	require.Equal(t, false, exists)
	head, err := f.Head(ctx)
	require.NoError(t, err)
	require.Equal(t, c, head)
	require.NoError(t, f.store.prune(ctx))
	require.Equal(t, 2, f.NodeCount())
}

func TestSetOptimisticToInvalid_UnrealizedCheckpointRecovery(t *testing.T) {
	ctx := context.Background()
	f := setup(0, 0)
	epochSlots := params.BeaconConfig().SlotsPerEpoch
	driftGenesisTime(f, 4*epochSlots-1, 30)
	j, good, goodTip := indexToHash(1), indexToHash(2), indexToHash(3)
	bad, badTip := indexToHash(4), indexToHash(5)
	for _, block := range []struct {
		slot         primitives.Slot
		root, parent [32]byte
	}{{epochSlots, j, [32]byte{}}, {2 * epochSlots, good, j}, {3 * epochSlots, bad, j}} {
		_, ro, err := prepareForkchoiceState(ctx, block.slot, block.root, block.parent, block.root, 0, 0)
		require.NoError(t, err)
		_, err = f.store.insert(ctx, ro, 0, 0)
		require.NoError(t, err)
	}
	require.NoError(t, f.UpdateJustifiedCheckpoint(ctx, &forkchoicetypes.Checkpoint{Epoch: 1, Root: j}))
	// Compute pulled-up tips through the actual public insertion path. The
	// valid branch has a previous-epoch quorum; the optimistic branch has a
	// current-epoch quorum. Neither realizes its justification yet.
	for _, block := range []struct {
		slot         primitives.Slot
		root, parent [32]byte
		current      bool
	}{{3*epochSlots + 1, goodTip, good, false}, {4*epochSlots - 2, badTip, bad, true}} {
		st, _ := util.DeterministicGenesisStateZond(t, 128)
		require.NoError(t, st.SetSlot(block.slot))
		require.NoError(t, st.SetCurrentJustifiedCheckpoint(&qrysmpb.Checkpoint{Epoch: 1, Root: j[:]}))
		require.NoError(t, st.SetPreviousJustifiedCheckpoint(&qrysmpb.Checkpoint{Epoch: 1, Root: j[:]}))
		require.NoError(t, st.SetFinalizedCheckpoint(&qrysmpb.Checkpoint{Root: make([]byte, 32)}))
		flags := make([]byte, st.NumValidators())
		for i := 0; i < 100; i++ {
			flags[i] = 1 << params.BeaconConfig().TimelyTargetFlagIndex
		}
		if block.current {
			require.NoError(t, st.UpdateBlockRootAtIndex(uint64(3*epochSlots), bad))
			require.NoError(t, st.SetCurrentParticipationBits(flags))
			require.NoError(t, st.SetPreviousParticipationBits(flags))
			require.NoError(t, st.SetJustificationBits(bitfield.Bitvector4{0x02}))
		} else {
			require.NoError(t, st.UpdateBlockRootAtIndex(uint64(2*epochSlots), good))
			require.NoError(t, st.SetPreviousParticipationBits(flags))
		}
		_, ro, err := prepareForkchoiceState(ctx, block.slot, block.root, block.parent, block.root, 1, 0)
		require.NoError(t, err)
		require.NoError(t, f.InsertNode(ctx, st, ro))
	}
	require.NoError(t, f.SetOptimisticToValid(ctx, goodTip))
	require.Equal(t, primitives.Epoch(2), f.store.nodeByRoot[goodTip].unrealizedJustifiedEpoch)
	require.Equal(t, primitives.Epoch(3), f.store.unrealizedJustifiedCheckpoint.Epoch)
	require.Equal(t, bad, f.store.unrealizedJustifiedCheckpoint.Root)
	// This finalization is observed only on the invalid branch, even though
	// its root is a valid common ancestor that will survive invalidation.
	require.DeepEqual(t, &forkchoicetypes.Checkpoint{Epoch: 1, Root: j}, f.store.unrealizedFinalizedCheckpoint)
	_, err := f.SetOptimisticToInvalid(ctx, bad, j, j)
	require.NoError(t, err)
	require.Equal(t, true, f.HasNode(f.JustifiedCheckpoint().Root))
	require.DeepEqual(t, &forkchoicetypes.Checkpoint{Epoch: 2, Root: good}, f.store.unrealizedJustifiedCheckpoint)
	require.DeepEqual(t, f.FinalizedCheckpoint(), f.store.unrealizedFinalizedCheckpoint)
	head, err := f.Head(ctx)
	require.NoError(t, err)
	require.Equal(t, goodTip, head)

	driftGenesisTime(f, 4*epochSlots, 30)
	require.NoError(t, f.NewSlot(ctx, 4*epochSlots))
	head, err = f.Head(ctx)
	require.NoError(t, err)
	require.Equal(t, goodTip, head)
	require.DeepEqual(t, &forkchoicetypes.Checkpoint{Epoch: 2, Root: good}, f.JustifiedCheckpoint())
	require.Equal(t, primitives.Epoch(0), f.FinalizedCheckpoint().Epoch)
}
