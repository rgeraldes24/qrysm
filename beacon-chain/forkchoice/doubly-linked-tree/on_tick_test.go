package doublylinkedtree

import (
	"context"
	"testing"
	"testing/synctest"

	forkchoicetypes "github.com/theQRL/qrysm/beacon-chain/forkchoice/types"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/testing/require"
)

func TestForkChoice_ProposerBoostTickOrder(t *testing.T) {
	for _, previousBoost := range []bool{false, true} {
		for _, tickFirst := range []bool{true, false} {
			name := map[bool]string{false: "no previous boost", true: "previous boost"}[previousBoost] + "/" + map[bool]string{true: "tick before block", false: "block before tick"}[tickFirst]
			t.Run(name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					ctx := context.Background()
					f := setup(0, 0)
					f.store.committeeWeight = 100
					old, timely, second := [32]byte{0x99}, [32]byte{0x11}, [32]byte{0xff}
					insert := func(slot primitives.Slot, root [32]byte) {
						t.Helper()
						st, block, err := prepareForkchoiceState(ctx, slot, root, [32]byte{}, root, 0, 0)
						require.NoError(t, err)
						require.NoError(t, f.InsertNode(ctx, st, block))
					}
					checkHead := func(want [32]byte) {
						t.Helper()
						head, err := f.Head(ctx)
						require.NoError(t, err)
						require.Equal(t, want, head)
					}
					driftGenesisTime(f, 9, 1)
					require.NoError(t, f.NewSlot(ctx, 9))
					if !previousBoost {
						driftGenesisTime(f, 10, 1)
					}
					insert(9, old)
					checkHead(old)
					driftGenesisTime(f, 10, 1)
					if tickFirst {
						require.NoError(t, f.NewSlot(ctx, 10))
					}
					insert(10, timely)
					checkHead(timely)
					if !tickFirst {
						require.NoError(t, f.NewSlot(ctx, 10))
					}
					checkHead(timely)
					// Delayed and repeated ticks must preserve the first block's
					// boost, including when another block arrives in the same slot.
					for _, slot := range []primitives.Slot{9, 10, 10} {
						require.NoError(t, f.NewSlot(ctx, slot))
					}
					insert(10, second)
					require.Equal(t, timely, f.ProposerBoost())
					checkHead(timely)
					// A genuinely newer slot expires the boost and removes its score.
					driftGenesisTime(f, 11, 1)
					require.NoError(t, f.NewSlot(ctx, 11))
					require.Equal(t, [32]byte{}, f.ProposerBoost())
					checkHead(second)
					weight, err := f.Weight(timely)
					require.NoError(t, err)
					require.Equal(t, uint64(0), weight)
				})
			})
		}
	}
}

func TestStore_NewSlot(t *testing.T) {
	ctx := context.Background()
	bj := [32]byte{'z'}

	type args struct {
		slot          primitives.Slot
		finalized     *forkchoicetypes.Checkpoint
		justified     *forkchoicetypes.Checkpoint
		bestJustified *forkchoicetypes.Checkpoint
		shouldEqual   bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Not epoch boundary. No change",
			args: args{
				slot:          params.BeaconConfig().SlotsPerEpoch + 1,
				finalized:     &forkchoicetypes.Checkpoint{Epoch: 1, Root: [32]byte{'a'}},
				justified:     &forkchoicetypes.Checkpoint{Epoch: 2, Root: [32]byte{'b'}},
				bestJustified: &forkchoicetypes.Checkpoint{Epoch: 3, Root: bj},
				shouldEqual:   false,
			},
		},
		{
			name: "Justified higher than best justified. No change",
			args: args{
				slot:          params.BeaconConfig().SlotsPerEpoch,
				finalized:     &forkchoicetypes.Checkpoint{Epoch: 1, Root: [32]byte{'a'}},
				justified:     &forkchoicetypes.Checkpoint{Epoch: 3, Root: [32]byte{'b'}},
				bestJustified: &forkchoicetypes.Checkpoint{Epoch: 2, Root: bj},
				shouldEqual:   false,
			},
		},
		{
			name: "Best justified not on the same chain as finalized. No change",
			args: args{
				slot:          params.BeaconConfig().SlotsPerEpoch,
				finalized:     &forkchoicetypes.Checkpoint{Epoch: 1, Root: [32]byte{'a'}},
				justified:     &forkchoicetypes.Checkpoint{Epoch: 2, Root: [32]byte{'b'}},
				bestJustified: &forkchoicetypes.Checkpoint{Epoch: 3, Root: [32]byte{'d'}},
				shouldEqual:   false,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := setup(test.args.justified.Epoch, test.args.finalized.Epoch)
			state, blkRoot, err := prepareForkchoiceState(ctx, 0, [32]byte{}, [32]byte{}, [32]byte{}, 0, 0)
			require.NoError(t, err)
			require.NoError(t, f.InsertNode(ctx, state, blkRoot)) // genesis
			state, blkRoot, err = prepareForkchoiceState(ctx, 32, [32]byte{'a'}, [32]byte{}, [32]byte{}, 0, 0)
			require.NoError(t, err)
			require.NoError(t, f.InsertNode(ctx, state, blkRoot)) // finalized
			state, blkRoot, err = prepareForkchoiceState(ctx, 64, [32]byte{'b'}, [32]byte{'a'}, [32]byte{}, 0, 0)
			require.NoError(t, err)
			require.NoError(t, f.InsertNode(ctx, state, blkRoot)) // justified
			state, blkRoot, err = prepareForkchoiceState(ctx, 96, bj, [32]byte{'a'}, [32]byte{}, 0, 0)
			require.NoError(t, err)
			require.NoError(t, f.InsertNode(ctx, state, blkRoot)) // best justified
			state, blkRoot, err = prepareForkchoiceState(ctx, 97, [32]byte{'d'}, [32]byte{}, [32]byte{}, 0, 0)
			require.NoError(t, err)
			require.NoError(t, f.InsertNode(ctx, state, blkRoot)) // bad

			require.NoError(t, f.UpdateFinalizedCheckpoint(test.args.finalized))
			require.NoError(t, f.UpdateJustifiedCheckpoint(ctx, test.args.justified))

			require.NoError(t, f.NewSlot(ctx, test.args.slot))
			bcp := test.args.bestJustified
			if test.args.shouldEqual {
				cp := f.JustifiedCheckpoint()
				require.Equal(t, bcp.Epoch, cp.Epoch)
				require.Equal(t, bcp.Root, cp.Root)
			} else {
				cp := f.JustifiedCheckpoint()
				epochsEqual := bcp.Epoch == cp.Epoch
				rootsEqual := bcp.Root == cp.Root
				require.Equal(t, false, epochsEqual && rootsEqual)
			}
		})
	}
}
