package doublylinkedtree

import (
	"context"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/core/epoch/precompute"
	forkchoicetypes "github.com/theQRL/qrysm/beacon-chain/forkchoice/types"
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/time/slots"
)

func (s *Store) setUnrealizedJustifiedEpoch(root [32]byte, epoch primitives.Epoch) error {
	node, ok := s.nodeByRoot[root]
	if !ok || node == nil {
		return errors.Wrap(ErrNilNode, "could not set unrealized justified epoch")
	}
	if epoch < node.unrealizedJustifiedEpoch {
		return errInvalidUnrealizedJustifiedEpoch
	}
	node.unrealizedJustifiedEpoch = epoch
	return nil
}

func (s *Store) setUnrealizedFinalizedEpoch(root [32]byte, epoch primitives.Epoch) error {
	node, ok := s.nodeByRoot[root]
	if !ok || node == nil {
		return errors.Wrap(ErrNilNode, "could not set unrealized finalized epoch")
	}
	if epoch < node.unrealizedFinalizedEpoch {
		return errInvalidUnrealizedFinalizedEpoch
	}
	node.unrealizedFinalizedEpoch = epoch
	return nil
}

// updateUnrealizedCheckpoints "realizes" the unrealized justified and finalized
// epochs stored within nodes. It should be called at the beginning of each epoch.
func (f *ForkChoice) updateUnrealizedCheckpoints(ctx context.Context) error {
	for _, node := range f.store.nodeByRoot {
		node.justifiedEpoch = node.unrealizedJustifiedEpoch
		node.finalizedEpoch = node.unrealizedFinalizedEpoch
		if node.justifiedEpoch > f.store.justifiedCheckpoint.Epoch {
			f.store.prevJustifiedCheckpoint = f.store.justifiedCheckpoint
			f.store.justifiedCheckpoint = f.store.unrealizedJustifiedCheckpoint
			if err := f.updateJustifiedBalances(ctx, f.store.justifiedCheckpoint); err != nil {
				return errors.Wrap(err, "could not update justified balances")
			}
		}
		if node.finalizedEpoch > f.store.finalizedCheckpoint.Epoch {
			f.store.finalizedCheckpoint = f.store.unrealizedFinalizedCheckpoint
		}
	}
	return nil
}

func (s *Store) pullTips(state state.BeaconState, node *Node, jc, fc *qrysmpb.Checkpoint) (*qrysmpb.Checkpoint, *qrysmpb.Checkpoint) {
	if node.parent == nil { // Nothing to do if the parent is nil.
		return jc, fc
	}
	currentEpoch := slots.ToEpoch(slots.CurrentSlot(s.genesisTime))
	stateEpoch := slots.ToEpoch(state.Slot())
	// Exit early only when the parent already justified the current epoch,
	// which a child cannot exceed. Skipping the computation because fewer
	// than two thirds of the epoch's slots have elapsed is not exact:
	// committees are equal by validator count, not by balance, so uneven
	// effective balances can reach the two-thirds quorum earlier, and a
	// missed justification lets the head leave a checkpoint the spec pins.
	if node.parent.unrealizedJustifiedEpoch == currentEpoch {
		node.unrealizedJustifiedEpoch = node.parent.unrealizedJustifiedEpoch
		node.unrealizedFinalizedEpoch = node.parent.unrealizedFinalizedEpoch
		return jc, fc
	}

	uj, uf, err := precompute.UnrealizedCheckpoints(state)
	if err != nil {
		log.WithError(err).Debug("could not compute unrealized checkpoints")
		uj, uf = jc, fc
	}

	// Update store's unrealized checkpoints.
	if uj.Epoch > s.unrealizedJustifiedCheckpoint.Epoch {
		s.unrealizedJustifiedCheckpoint = &forkchoicetypes.Checkpoint{
			Epoch: uj.Epoch, Root: bytesutil.ToBytes32(uj.Root),
		}
	}
	if uf.Epoch > s.unrealizedFinalizedCheckpoint.Epoch {
		s.unrealizedJustifiedCheckpoint = &forkchoicetypes.Checkpoint{
			Epoch: uj.Epoch, Root: bytesutil.ToBytes32(uj.Root),
		}
		s.unrealizedFinalizedCheckpoint = &forkchoicetypes.Checkpoint{
			Epoch: uf.Epoch, Root: bytesutil.ToBytes32(uf.Root),
		}
	}

	// Update node's checkpoints.
	node.unrealizedJustifiedEpoch, node.unrealizedFinalizedEpoch = uj.Epoch, uf.Epoch
	if stateEpoch < currentEpoch {
		jc, fc = uj, uf
		node.justifiedEpoch = uj.Epoch
		node.finalizedEpoch = uf.Epoch
	}
	return jc, fc
}
