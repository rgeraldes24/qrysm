package blockchain

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/theQRL/qrysm/beacon-chain/core/feed"
	"github.com/theQRL/qrysm/beacon-chain/db"
	"github.com/theQRL/qrysm/beacon-chain/verification"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

type cancelAfterBatchSaveDB struct {
	db.HeadAccessDatabase
	cancel context.CancelFunc
}

func (d *cancelAfterBatchSaveDB) SaveBlocks(ctx context.Context, blks []interfaces.ReadOnlySignedBeaconBlock) error {
	if err := d.HeadAccessDatabase.SaveBlocks(ctx, blks); err != nil {
		return err
	}
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	return nil
}

func TestReceiveBlockBatch_UnimportedParentsRemainRetryable(t *testing.T) {
	for _, mode := range []string{"healthy", "cancel after persistence", "partial insertion", "block write failure"} {
		for _, gossipRetry := range []bool{false, true} {
			name := mode + map[bool]string{false: "/batch retry", true: "/gossip retry"}[gossipRetry]
			t.Run(name, func(t *testing.T) {
				f := newBatchExecutionFixture(t, 5)
				f.engine.ErrNewPayload, f.engine.ErrForkchoiceUpdated = nil, nil
				synctest.Test(t, func(t *testing.T) {
					t.Cleanup(synctest.Wait)
					driftGenesisTime(f.s, 6, 0)
					ctx, cancel := context.WithCancel(f.ctx)
					defer cancel()
					fc, originalDB := f.s.cfg.ForkChoiceStore, f.s.cfg.BeaconDB
					failure := errors.New("temporary batch import failure")
					switch mode {
					case "cancel after persistence":
						f.s.cfg.BeaconDB = &cancelAfterBatchSaveDB{HeadAccessDatabase: originalDB, cancel: cancel}
						failure = context.Canceled
					case "partial insertion":
						f.s.cfg.ForkChoiceStore = &retainedBatchPrefixStore{ForkChoicer: fc, failure: failure}
					case "block write failure":
						f.s.cfg.BeaconDB = &batchBlockSaveFailureDB{HeadAccessDatabase: originalDB, failure: failure}
					}
					err := f.s.ReceiveBlockBatch(ctx, f.blks[2:4])
					if mode == "healthy" {
						require.NoError(t, err)
					} else {
						require.ErrorIs(t, err, failure)
					}
					synctest.Wait()
					f.s.cfg.ForkChoiceStore, f.s.cfg.BeaconDB = fc, originalDB
					for _, b := range f.blks[2:4] {
						require.Equal(t, fc.HasNode(b.Root()), f.s.HasBlock(f.ctx, b.Root()), "only imported parents are available")
					}
					parent, child := f.blks[3], f.blks[4]
					require.Equal(t, mode != "block write failure", originalDB.HasBlock(f.ctx, parent.Root()))
					err = f.s.ReceiveBlock(f.ctx, child, child.Root())
					require.Equal(t, false, errors.Is(err, verification.ErrInvalid), "a local import failure must not blame valid children")
					if mode == "healthy" {
						require.NoError(t, err)
					} else {
						require.ErrorContains(t, "parent block has not been imported", err)
						if gossipRetry {
							for _, b := range f.blks[2:4] {
								require.NoError(t, f.s.ReceiveBlock(f.ctx, b, b.Root()))
							}
						} else {
							require.NoError(t, f.s.ReceiveBlockBatch(f.ctx, f.blks[2:4]))
						}
						require.Equal(t, true, f.s.HasBlock(f.ctx, parent.Root()))
						require.NoError(t, f.s.ReceiveBlock(f.ctx, child, child.Root()))
					}
					synctest.Wait()
					require.Equal(t, true, fc.HasNode(child.Root()))
				})
			})
		}
	}
}

func TestReceiveBlock_RetainsValidExecutionOnCancellation(t *testing.T) {
	for _, tc := range []struct {
		name                string
		batch, cancelImport bool
	}{
		{name: "healthy gossip"},
		{name: "cancelled gossip", cancelImport: true},
		{name: "cancelled batch control", batch: true, cancelImport: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newBatchExecutionFixture(t, 3)
			f.engine.ErrNewPayload, f.engine.ErrForkchoiceUpdated = nil, nil
			incoming, _ := emptyBranchBlock(t, f, f.states[1], 3, 'c')
			synctest.Test(t, func(t *testing.T) {
				t.Cleanup(synctest.Wait)
				driftGenesisTime(f.s, 4, 0)
				voteForRoot(t, f, f.blks[1].Root(), 0)
				ctx, cancel := context.WithCancel(f.ctx)
				defer cancel()
				fc := f.s.cfg.ForkChoiceStore
				if tc.cancelImport {
					f.s.cfg.ForkChoiceStore = &notificationInterruptedStore{ForkChoicer: fc, cancel: cancel}
				}
				events := make(chan *feed.Event, 32)
				sub := f.s.cfg.StateNotifier.StateFeed().Subscribe(events)
				defer sub.Unsubscribe()
				var err error
				if tc.batch {
					err = f.s.ReceiveBlockBatch(ctx, []blocks.ROBlock{incoming})
				} else {
					err = f.s.ReceiveBlock(ctx, incoming, incoming.Root())
				}
				if tc.cancelImport {
					require.ErrorIs(t, err, context.Canceled)
				} else {
					require.NoError(t, err)
				}
				synctest.Wait()
				f.s.cfg.ForkChoiceStore = fc
				require.Equal(t, true, fc.HasNode(incoming.Root()))
				requireBlockEventOptimism(t, events, []blocks.ROBlock{incoming}, 3)
				require.NoError(t, f.s.ReceiveBlock(f.ctx, incoming, incoming.Root()))
				// VALID head updates on B2's separate branch cannot repair C3.
				for i := 0; i < 2; i++ {
					f.s.UpdateHead(f.ctx, 4)
					synctest.Wait()
				}
				require.Equal(t, f.blks[1].Root(), f.s.CachedHeadRoot())
				optimistic, err := f.s.IsOptimisticForRoot(f.ctx, incoming.Root())
				require.NoError(t, err)
				assert.Equal(t, false, optimistic, "the retained block must keep the completed VALID verdict")
				assert.Equal(t, 0, processedBlockCounts(events)[incoming.Root()], "retries must not repeat notifications")
			})
		})
	}
}
