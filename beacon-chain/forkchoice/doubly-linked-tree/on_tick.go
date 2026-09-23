package doublylinkedtree

import (
	"context"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/time/slots"
)

// NewSlot expires proposer boost and, at epoch boundaries, realizes checkpoint
// observations from earlier epochs. It tolerates delayed and repeated ticks,
// including when a new-epoch block acquires the store lock before its tick.
func (f *ForkChoice) NewSlot(ctx context.Context, slot primitives.Slot) error {
	f.store.expireProposerBoost(slot)

	// Return if it's not a new epoch.
	if !slots.IsEpochStart(slot) {
		return nil
	}

	// Prepare pruning before realizing checkpoints or node epochs, so an
	// interrupted traversal leaves the epoch transition available for retry.
	epoch := slots.ToEpoch(slot)
	_, finalized := f.store.unrealizedCheckpointsBefore(epoch)
	plan, err := f.store.preparePrune(ctx, finalized)
	if err != nil {
		return err
	}
	if err := f.updateUnrealizedCheckpoints(ctx, epoch); err != nil {
		return errors.Wrap(err, "could not update unrealized checkpoints")
	}
	f.store.applyPrune(plan)
	return nil
}
