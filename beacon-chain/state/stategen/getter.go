package stategen

import (
	"context"
	stderrors "errors"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/beacon-chain/core/time"
	"github.com/theQRL/qrysm/beacon-chain/core/transition"
	forkchoicetypes "github.com/theQRL/qrysm/beacon-chain/forkchoice/types"
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/time/slots"
	"go.opencensus.io/trace"
)

var ErrNoDataForSlot = errors.New("cannot retrieve data for slot")

// HasState returns true if the state exists in cache or in DB.
func (s *State) HasState(ctx context.Context, blockRoot [32]byte) (bool, error) {
	has, err := s.hasStateInCache(ctx, blockRoot)
	if err != nil {
		return false, err
	}
	if has {
		return true, nil
	}
	return s.beaconDB.HasState(ctx, blockRoot), nil
}

// hasStateInCache returns true if the state exists in cache.
func (s *State) hasStateInCache(_ context.Context, blockRoot [32]byte) (bool, error) {
	if s.hotStateCache.has(blockRoot) {
		return true, nil
	}
	_, has, err := s.epochBoundaryStateCache.getByBlockRoot(blockRoot)
	if err != nil {
		return false, err
	}
	return has, nil
}

// StateByRootIfCachedNoCopy retrieves a state using the input block root only if the state is already in the cache.
func (s *State) StateByRootIfCachedNoCopy(blockRoot [32]byte) state.BeaconState {
	if !s.hotStateCache.has(blockRoot) {
		return nil
	}
	return s.hotStateCache.getWithoutCopy(blockRoot)
}

// StateByRoot retrieves the state using input block root.
func (s *State) StateByRoot(ctx context.Context, blockRoot [32]byte) (state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.StateByRoot")
	defer span.End()

	// Genesis case. If block root is zero hash, short circuit to use genesis state stored in DB.
	if blockRoot == params.BeaconConfig().ZeroHash {
		root, err := s.beaconDB.GenesisBlockRoot(ctx)
		if err != nil {
			return nil, stderrors.Join(ErrNoGenesisBlock, err)
		}
		blockRoot = root
	}
	return s.loadStateByRoot(ctx, blockRoot)
}

// BalancesByCheckpoint retrieves what fork choice weighs with from the state
// of the given checkpoint. As in store_target_checkpoint_state, the checkpoint
// block's state is advanced to the first slot of the checkpoint epoch when the
// block precedes it, so the balances and the active set are those of the
// checkpoint epoch even when the epoch's first slot has no block.
func (s *State) BalancesByCheckpoint(ctx context.Context, cp *forkchoicetypes.Checkpoint) (*forkchoicetypes.JustifiedBalances, error) {
	if cp == nil {
		return nil, errors.New("nil checkpoint")
	}
	st, err := s.StateByRoot(ctx, cp.Root)
	if err != nil {
		return nil, err
	}
	if st == nil || st.IsNil() {
		return nil, errNilState
	}
	startSlot, err := slots.EpochStart(cp.Epoch)
	if err != nil {
		return nil, err
	}
	if st.Slot() < startSlot {
		st, err = transition.ProcessSlots(ctx, st.Copy(), startSlot)
		if err != nil {
			return nil, errors.Wrapf(err, "could not advance checkpoint state to slot %d", startSlot)
		}
	}
	epoch := time.CurrentEpoch(st)

	jb := &forkchoicetypes.JustifiedBalances{Balances: make([]uint64, st.NumValidators())}
	if err := st.ReadFromEveryValidator(func(idx int, val state.ReadOnlyValidator) error {
		if !helpers.IsActiveValidatorUsingTrie(val, epoch) {
			return nil
		}
		jb.TotalActiveBalance += val.EffectiveBalance()
		if !val.Slashed() {
			jb.Balances[idx] = val.EffectiveBalance()
		}
		return nil
	}); err != nil {
		return nil, err
	}
	// get_total_active_balance never returns less than one increment.
	if jb.TotalActiveBalance < params.BeaconConfig().EffectiveBalanceIncrement {
		jb.TotalActiveBalance = params.BeaconConfig().EffectiveBalanceIncrement
	}
	return jb, nil
}

// StateByRootInitialSync retrieves the state from the DB for the initial syncing phase.
// It assumes initial syncing applies a linear chain, so the returned state is not copied.
// It invalidates cache for parent root because pre-state will get mutated.
//
// WARNING: Do not use this method for anything other than initial syncing purpose or block tree is applied.
func (s *State) StateByRootInitialSync(ctx context.Context, blockRoot [32]byte) (state.BeaconState, error) {
	// Genesis case. If block root is zero hash, short circuit to use genesis state stored in DB.
	if blockRoot == params.BeaconConfig().ZeroHash {
		return s.beaconDB.GenesisState(ctx)
	}

	// To invalidate cache for parent root because pre-state will get mutated.
	// It is a parent root because StateByRootInitialSync is always used to fetch the block's parent state.
	defer s.hotStateCache.delete(blockRoot)

	if s.hotStateCache.has(blockRoot) {
		return s.hotStateCache.getWithoutCopy(blockRoot), nil
	}

	cachedInfo, ok, err := s.epochBoundaryStateCache.getByBlockRoot(blockRoot)
	if err != nil {
		return nil, err
	}
	if ok {
		return cachedInfo.state, nil
	}

	summary, err := s.stateSummary(ctx, blockRoot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get state summary")
	}

	startState, blockRoots, err := s.latestAncestorAndBlockRootsForSlot(ctx, blockRoot, summary.Slot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get ancestor state")
	}
	if startState == nil || startState.IsNil() {
		return nil, errUnknownState
	}
	if startState.Slot() == summary.Slot {
		return startState, nil
	}

	startState, err = s.replayBlockRoots(ctx, startState, blockRoots, summary.Slot)
	if err != nil {
		return nil, errors.Wrap(err, "could not replay blocks")
	}

	return startState, nil
}

// This returns the state summary object of a given block root. It first checks the cache, then checks the DB.
func (s *State) stateSummary(ctx context.Context, blockRoot [32]byte) (*qrysmpb.StateSummary, error) {
	var summary *qrysmpb.StateSummary
	var err error

	summary, err = s.beaconDB.StateSummary(ctx, blockRoot)
	if err != nil {
		return nil, err
	}

	if summary == nil {
		return s.recoverStateSummary(ctx, blockRoot)
	}
	return summary, nil
}

// RecoverStateSummary recovers state summary object of a given block root by using the saved block in DB.
func (s *State) recoverStateSummary(ctx context.Context, blockRoot [32]byte) (*qrysmpb.StateSummary, error) {
	if s.beaconDB.HasBlock(ctx, blockRoot) {
		b, err := s.beaconDB.Block(ctx, blockRoot)
		if err != nil {
			return nil, err
		}
		summary := &qrysmpb.StateSummary{Slot: b.Block().Slot(), Root: blockRoot[:]}
		if err := s.beaconDB.SaveStateSummary(ctx, summary); err != nil {
			return nil, err
		}
		return summary, nil
	}
	return nil, errors.New("could not find block in DB")
}

// DeleteStateFromCaches deletes the state from the caches.
func (s *State) DeleteStateFromCaches(_ context.Context, blockRoot [32]byte) error {
	s.hotStateCache.delete(blockRoot)
	return s.epochBoundaryStateCache.delete(blockRoot)
}

// This loads a beacon state from either the cache or DB, then replays blocks up the slot of the requested block root.
func (s *State) loadStateByRoot(ctx context.Context, blockRoot [32]byte) (state.BeaconState, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.loadStateByRoot")
	defer span.End()

	// First, it checks if the state exists in hot state cache.
	cachedState := s.hotStateCache.get(blockRoot)
	if cachedState != nil && !cachedState.IsNil() {
		return cachedState, nil
	}

	// Second, it checks if the state exists in epoch boundary state cache.
	cachedInfo, ok, err := s.epochBoundaryStateCache.getByBlockRoot(blockRoot)
	if err != nil {
		return nil, err
	}
	if ok {
		return cachedInfo.state, nil
	}

	// Short circuit if the state is already in the DB.
	if s.beaconDB.HasState(ctx, blockRoot) {
		return s.beaconDB.State(ctx, blockRoot)
	}

	summary, err := s.stateSummary(ctx, blockRoot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get state summary")
	}
	targetSlot := summary.Slot

	// Since the requested state is not in caches or DB, start replaying using the last
	// available ancestor state which is retrieved using input block's root.
	startState, blockRoots, err := s.latestAncestorAndBlockRootsForSlot(ctx, blockRoot, targetSlot)
	if err != nil {
		return nil, errors.Wrap(err, "could not get ancestor state")
	}
	if startState == nil || startState.IsNil() {
		return nil, errUnknownBoundaryState
	}

	if startState.Slot() == targetSlot {
		return startState, nil
	}

	replayBlockCount.Observe(float64(len(blockRoots)))
	return s.replayBlockRoots(ctx, startState, blockRoots, targetSlot)
}

// latestAncestorAndBlockRootsForSlot returns the highest available ancestor
// state and the canonical roots that must be replayed to targetSlot. The roots
// are returned in ascending slot order. It intentionally retains roots, not
// decoded blocks, while walking backwards through the lineage. State summaries
// can represent a pre-block state, so blocks after targetSlot are excluded even
// when blockRoot itself is at a later slot.
func (s *State) latestAncestorAndBlockRootsForSlot(
	ctx context.Context,
	blockRoot [32]byte,
	targetSlot primitives.Slot,
) (state.BeaconState, [][32]byte, error) {
	ctx, span := trace.StartSpan(ctx, "stateGen.latestAncestor")
	defer span.End()

	if finalizedState := s.finalizedStateIfRoot(blockRoot); finalizedState != nil {
		return finalizedState, nil, nil
	}

	currentRoot := blockRoot
	b, err := s.beaconDB.Block(ctx, blockRoot)
	if err != nil {
		return nil, nil, err
	}
	if err := blocks.BeaconBlockIsNil(b); err != nil {
		return nil, nil, err
	}
	roots := make([][32]byte, 0)
	appendCurrentRoot := func() {
		if b.Block().Slot() <= targetSlot {
			roots = append(roots, currentRoot)
		}
	}

	for {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		// Is the state the genesis state.
		parentRoot := b.Block().ParentRoot()
		if parentRoot == params.BeaconConfig().ZeroHash {
			ancestor, err := s.beaconDB.GenesisState(ctx)
			if err != nil {
				return nil, nil, errors.Wrap(err, "could not get genesis state")
			}
			appendCurrentRoot()
			reverseBlockRoots(roots)
			return ancestor, roots, nil
		}

		// Return an error if slot hasn't been covered by checkpoint sync.
		ps := b.Block().Slot() - 1
		if !s.slotAvailable(ps) {
			return nil, nil, errors.Wrapf(ErrNoDataForSlot, "slot %d not in db due to checkpoint sync", ps)
		}
		// Does the state exist in the hot state cache.
		if s.hotStateCache.has(parentRoot) {
			appendCurrentRoot()
			reverseBlockRoots(roots)
			return s.hotStateCache.get(parentRoot), roots, nil
		}

		// Does the state exist in finalized info cache.
		if finalizedState := s.finalizedStateIfRoot(parentRoot); finalizedState != nil {
			appendCurrentRoot()
			reverseBlockRoots(roots)
			return finalizedState, roots, nil
		}

		// Does the state exist in epoch boundary cache.
		cachedInfo, ok, err := s.epochBoundaryStateCache.getByBlockRoot(parentRoot)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			appendCurrentRoot()
			reverseBlockRoots(roots)
			return cachedInfo.state, roots, nil
		}

		// Does the state exists in DB.
		if s.beaconDB.HasState(ctx, parentRoot) {
			ancestor, err := s.beaconDB.State(ctx, parentRoot)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to retrieve state from db")
			}
			appendCurrentRoot()
			reverseBlockRoots(roots)
			return ancestor, roots, nil
		}

		appendCurrentRoot()
		currentRoot = parentRoot
		b, err = s.beaconDB.Block(ctx, parentRoot)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to retrieve block from db")
		}
		if b == nil || b.IsNil() {
			return nil, nil, errUnknownBlock
		}
	}
}

func (s *State) CombinedCache() *CombinedCache {
	getters := make([]CachedGetter, 0)
	if s.hotStateCache != nil {
		getters = append(getters, s.hotStateCache)
	}
	if s.epochBoundaryStateCache != nil {
		getters = append(getters, s.epochBoundaryStateCache)
	}
	return &CombinedCache{getters: getters}
}

func (s *State) slotAvailable(slot primitives.Slot) bool {
	// default to assuming node was initialized from genesis - backfill only needs to be specified for checkpoint sync
	if s.backfillStatus == nil {
		return true
	}
	return s.backfillStatus.SlotCovered(slot)
}
