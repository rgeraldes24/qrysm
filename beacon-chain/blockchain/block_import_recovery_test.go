package blockchain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/theQRL/qrysm/beacon-chain/core/feed"
	"github.com/theQRL/qrysm/beacon-chain/core/transition"
	"github.com/theQRL/qrysm/beacon-chain/db"
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/beacon-chain/verification"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
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

type delayedImportStateCopy struct {
	state.BeaconState
	entered, release chan struct{}
}

func (s *delayedImportStateCopy) Copy() state.BeaconState {
	close(s.entered)
	<-s.release
	return s.BeaconState.Copy()
}

func TestPostBlockProcess_NextSlotCacheSurvivesFailedBatch(t *testing.T) {
	f := newBatchExecutionFixture(t, 4)
	f.engine.ErrNewPayload, f.engine.ErrForkchoiceUpdated = nil, nil
	// E5 is a valid alternative child of C3 that skips slot 4.
	alternate, _ := emptyBranchBlock(t, f, f.states[3], 5, 'e')
	bad, err := f.blks[3].Copy()
	require.NoError(t, err)
	pb, err := bad.PbZondBlock()
	require.NoError(t, err)
	pb.Signature[0] ^= 1
	bad, err = blocks.NewSignedBeaconBlock(pb)
	require.NoError(t, err)
	badBlock, err := blocks.NewROBlock(bad)
	require.NoError(t, err)
	synctest.Test(t, func(t *testing.T) {
		t.Cleanup(synctest.Wait)
		driftGenesisTime(f.s, 6, 0)
		incoming := f.blks[2]
		root := incoming.Root()
		postState := f.states[3].Copy()
		// Gossip stores the same post-state that it passes to postBlockProcess.
		require.NoError(t, f.s.savePostStateInfo(f.ctx, root, incoming, postState))
		gate := &delayedImportStateCopy{BeaconState: postState, entered: make(chan struct{}), release: make(chan struct{})}
		release := sync.OnceFunc(func() { close(gate.release) })
		t.Cleanup(release)
		processed := make(chan error, 1)
		go func() {
			f.s.cfg.ForkChoiceStore.Lock()
			err := f.s.postBlockProcess(f.ctx, incoming, gate, true)
			f.s.cfg.ForkChoiceStore.Unlock()
			processed <- err
		}()
		<-gate.entered
		synctest.Wait()
		select {
		case err := <-processed:
			require.NoError(t, err)
			// The import returned before taking its snapshot: let a batch
			// consume and mutate its state before the worker copies it.
		default:
			// A synchronous snapshot holds the import lock, so finish it
			// before attempting the batch.
			release()
			require.NoError(t, <-processed)
		}
		require.Equal(t, true, IsInvalidBlock(f.s.ReceiveBlockBatch(f.ctx, []blocks.ROBlock{badBlock})))
		release()
		synctest.Wait()
		require.Equal(t, false, f.s.cfg.ForkChoiceStore.HasNode(badBlock.Root()))
		require.Equal(t, root, f.s.CachedHeadRoot())
		cachedRoot, cached := transition.LastCachedState()
		require.Equal(t, root, bytesutil.ToBytes32(cachedRoot))
		require.NotNil(t, cached)
		assert.Equal(t, primitives.Slot(4), cached.Slot(), "C3's cache entry must not contain D4's post-state")
		require.NoError(t, f.s.ReceiveBlock(f.ctx, alternate, alternate.Root()), "a valid child must survive the failed batch")
		synctest.Wait()
		require.Equal(t, true, f.s.cfg.ForkChoiceStore.HasNode(alternate.Root()))
	})
}

func TestReceiveBlockBatch_RetriesPrunedCanonicalBlocks(t *testing.T) {
	setupEpochTransitionTest(t)
	for _, mode := range []string{"pruned canonical", "retained canonical", "stored fork"} {
		t.Run(mode, func(t *testing.T) {
			f := newBatchExecutionFixture(t, 24)
			f.engine.ErrNewPayload, f.engine.ErrForkchoiceUpdated = nil, nil
			retry := f.blks[2:5]
			if mode == "retained canonical" {
				retry = f.blks[2:13]
			} else if mode == "stored fork" {
				fork, _ := emptyBranchBlock(t, f, f.states[2], 5, 'f')
				retry = []blocks.ROBlock{fork}
				require.NoError(t, f.s.cfg.BeaconDB.SaveBlock(f.ctx, fork))
			}
			failure := errors.New("temporary finality write failure")
			f.s.cfg.BeaconDB = &finalizationRetryDB{
				HeadAccessDatabase: f.s.cfg.BeaconDB, operation: "finality write", failures: 1, failure: failure,
			}
			synctest.Test(t, func(t *testing.T) {
				t.Cleanup(synctest.Wait)
				driftGenesisTime(f.s, 24, 0)
				events := make(chan *feed.Event, 32)
				sub := f.s.cfg.StateNotifier.StateFeed().Subscribe(events)
				defer sub.Unsubscribe()
				require.ErrorIs(t, f.s.ReceiveBlockBatch(f.ctx, f.blks[2:]), failure)
				synctest.Wait()
				require.Equal(t, primitives.Epoch(2), f.s.cfg.ForkChoiceStore.FinalizedCheckpoint().Epoch)
				require.Equal(t, primitives.Slot(2), f.s.HeadSlot())
				counts := processedBlockCounts(events)
				for _, b := range f.blks[2:] {
					require.Equal(t, 1, counts[b.Root()])
				}
				for _, b := range retry {
					// The range-sync duplicate filter only checks blocks at or
					// below the cached head, which the local failure left stale.
					require.Equal(t, true, f.s.HeadSlot() < b.Block().Slot())
					require.Equal(t, true, f.s.HasBlock(f.ctx, b.Root()))
				}
				err := f.s.ReceiveBlockBatch(f.ctx, retry)
				if mode == "stored fork" {
					require.Equal(t, true, errors.Is(err, verification.ErrInvalid))
					require.Equal(t, primitives.Slot(2), f.s.HeadSlot())
					return
				}
				require.NoError(t, err, "accepted canonical data must finish the prior local import failure")
				synctest.Wait()
				require.Equal(t, primitives.Slot(24), f.s.HeadSlot())
				saved, err := f.s.cfg.BeaconDB.FinalizedCheckpoint(f.ctx)
				require.NoError(t, err)
				validated, err := f.s.cfg.BeaconDB.LastValidatedCheckpoint(f.ctx)
				require.NoError(t, err)
				assert.Equal(t, primitives.Epoch(2), saved.Epoch)
				assert.Equal(t, f.blks[11].Root(), bytesutil.ToBytes32(saved.Root))
				assert.DeepSSZEqual(t, saved, validated)
				assert.Equal(t, 0, len(processedBlockCounts(events)), "retries must not repeat block notifications")
			})
		})
	}
}
