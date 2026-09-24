package blockchain

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	"github.com/theQRL/qrysm/beacon-chain/core/feed"
	statefeed "github.com/theQRL/qrysm/beacon-chain/core/feed/state"
	"github.com/theQRL/qrysm/beacon-chain/execution"
	mockExecution "github.com/theQRL/qrysm/beacon-chain/execution/testing"
	"github.com/theQRL/qrysm/beacon-chain/forkchoice"
	payloadattribute "github.com/theQRL/qrysm/consensus-types/payload-attribute"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

type recordedExecutionCheckpoint struct{ head, safe, finalized [32]byte }
type checkpointRecordingEngine struct {
	*mockExecution.EngineClient
	checkpoints []recordedExecutionCheckpoint
}

func (e *checkpointRecordingEngine) ForkchoiceUpdated(ctx context.Context, fcs *enginev1.ForkchoiceState, attr payloadattribute.Attributer) (*enginev1.PayloadIDBytes, []byte, error) {
	e.checkpoints = append(e.checkpoints, recordedExecutionCheckpoint{
		head: bytesutil.ToBytes32(fcs.HeadBlockHash), safe: bytesutil.ToBytes32(fcs.SafeBlockHash), finalized: bytesutil.ToBytes32(fcs.FinalizedBlockHash),
	})
	return e.EngineClient.ForkchoiceUpdated(ctx, fcs, attr)
}

func TestService_TickExecutionFinality(t *testing.T) {
	setupEpochTransitionTest(t)
	for _, tc := range []struct {
		name      string
		nextBlock bool
		engineErr error
	}{
		{name: "unchanged head"},
		{name: "syncing response", engineErr: execution.ErrAcceptedSyncingPayloadStatus},
		{name: "retry failed notification", engineErr: errors.New("engine unavailable")},
		{name: "next head control", nextBlock: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newBatchExecutionFixture(t, 24)
			f.engine.ErrNewPayload, f.engine.ErrForkchoiceUpdated = nil, nil
			engine := &checkpointRecordingEngine{EngineClient: f.engine}
			f.s.cfg.ExecutionEngineCaller = engine
			synctest.Test(t, func(t *testing.T) {
				t.Cleanup(synctest.Wait)
				driftGenesisTime(f.s, 23, 0)
				require.NoError(t, f.s.ReceiveBlockBatch(f.ctx, f.blks[2:23]))
				synctest.Wait()
				require.Equal(t, 1, len(engine.checkpoints))
				before := engine.checkpoints[0]
				require.Equal(t, primitives.Epoch(0), f.s.cfg.ForkChoiceStore.FinalizedCheckpoint().Epoch)
				oldHead, err := f.s.HeadRoot(f.ctx)
				require.NoError(t, err)
				events := make(chan *feed.Event, 16)
				sub := f.s.cfg.StateNotifier.StateFeed().Subscribe(events)
				defer sub.Unsubscribe()
				f.engine.ErrForkchoiceUpdated = tc.engineErr
				driftGenesisTime(f.s, 24, 0)
				require.NoError(t, f.s.NewSlot(f.ctx, 24))
				f.s.UpdateHead(f.ctx, 24)
				synctest.Wait()
				require.Equal(t, 2, len(engine.checkpoints), "checkpoint change must notify execution immediately")
				calls := 2
				if tc.engineErr != nil && tc.engineErr != execution.ErrAcceptedSyncingPayloadStatus {
					f.engine.ErrForkchoiceUpdated = nil
					f.s.UpdateHead(f.ctx, 24)
					synctest.Wait()
					calls++
					require.Equal(t, calls, len(engine.checkpoints), "failed notification must be retried")
				}
				require.Equal(t, primitives.Epoch(2), f.s.cfg.ForkChoiceStore.FinalizedCheckpoint().Epoch)
				currentHead, err := f.s.HeadRoot(f.ctx)
				require.NoError(t, err)
				require.DeepEqual(t, oldHead, currentHead)
				finalizedPayload, err := f.blks[11].Block().Body().Execution()
				require.NoError(t, err)
				want := bytesutil.ToBytes32(finalizedPayload.BlockHash())
				require.NotEqual(t, before.finalized, want)
				last := engine.checkpoints[len(engine.checkpoints)-1]
				assert.Equal(t, before.head, last.head)
				assert.Equal(t, want, last.finalized)
				for len(events) > 0 {
					typ := (<-events).Type
					assert.NotEqual(t, statefeed.NewHead, typ, "checkpoint-only update must not publish a head event")
					assert.NotEqual(t, statefeed.Reorg, typ)
				}
				// Repeated calls with accepted checkpoints must be deduplicated.
				f.s.UpdateHead(f.ctx, 24)
				driftGenesisTime(f.s, 25, 0)
				require.NoError(t, f.s.NewSlot(f.ctx, 25))
				f.s.UpdateHead(f.ctx, 25)
				synctest.Wait()
				require.Equal(t, calls, len(engine.checkpoints))
				if tc.nextBlock {
					require.NoError(t, f.s.ReceiveBlock(f.ctx, f.blks[23], f.blks[23].Root()))
					synctest.Wait()
					require.Equal(t, calls+1, len(engine.checkpoints))
					assert.Equal(t, want, engine.checkpoints[calls].finalized)
				}
			})
		})
	}
}

type executionSafeHashStore struct {
	forkchoice.ForkChoicer
	safeHash [32]byte
}

func (s *executionSafeHashStore) UnrealizedJustifiedPayloadBlockHash() [32]byte {
	return s.safeHash
}

func TestService_ExecutionSafeHashUpdate(t *testing.T) {
	f := newBatchExecutionFixture(t, 2)
	f.engine.ErrForkchoiceUpdated = nil
	engine := &checkpointRecordingEngine{EngineClient: f.engine}
	f.s.cfg.ExecutionEngineCaller = engine
	payload, err := f.blks[0].Block().Body().Execution()
	require.NoError(t, err)
	store := &executionSafeHashStore{ForkChoicer: f.s.cfg.ForkChoiceStore, safeHash: bytesutil.ToBytes32(payload.BlockHash())}
	f.s.cfg.ForkChoiceStore = store
	synctest.Test(t, func(t *testing.T) {
		t.Cleanup(synctest.Wait)
		driftGenesisTime(f.s, 3, 0)
		// Retain an attestation included in the existing head to detect any
		// accidental pruning during a checkpoint-only notification.
		require.NoError(t, f.s.cfg.AttPool.SaveUnaggregatedAttestation(f.blks[1].Block().Body().Attestations()[0]))
		f.s.UpdateHead(f.ctx, 3)
		synctest.Wait()
		require.Equal(t, 1, len(engine.checkpoints))
		require.Equal(t, store.safeHash, engine.checkpoints[0].safe)
		require.Equal(t, [32]byte{}, engine.checkpoints[0].finalized)
		require.Equal(t, f.blks[1].Root(), f.s.CachedHeadRoot())
		require.Equal(t, 1, f.s.cfg.AttPool.UnaggregatedAttestationCount())
		f.s.UpdateHead(f.ctx, 3)
		synctest.Wait()
		require.Equal(t, 1, len(engine.checkpoints))

		// An RPC error leaves the engine's state uncertain. If a safe candidate
		// is discarded afterwards, resend the previously accepted checkpoint
		// too: the failed request may already have changed execution's view.
		acceptedSafe := store.safeHash
		store.safeHash = [32]byte{}
		f.engine.ErrForkchoiceUpdated = errors.New("engine response lost")
		f.s.UpdateHead(f.ctx, 3)
		synctest.Wait()
		require.Equal(t, 2, len(engine.checkpoints))
		store.safeHash = acceptedSafe
		f.engine.ErrForkchoiceUpdated = nil
		f.s.UpdateHead(f.ctx, 3)
		synctest.Wait()
		require.Equal(t, 3, len(engine.checkpoints))
		require.Equal(t, acceptedSafe, engine.checkpoints[2].safe)
		f.s.UpdateHead(f.ctx, 3)
		synctest.Wait()
		require.Equal(t, 3, len(engine.checkpoints))
	})
}
