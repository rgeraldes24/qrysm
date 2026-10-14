package backfill

import (
	"context"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/db"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	"github.com/theQRL/qrysm/consensus-types/primitives"
)

// NewStatus correctly initializes a Status value with the required database value.
func NewStatus(store BackfillDB) *Status {
	return &Status{
		store: store,
	}
}

// Status provides a way to query the status of a backfill process that may be necessary to track when
// a node was initialized via checkpoint sync. With checkpoint sync, there will be a gap in node history from genesis
// until the checkpoint sync origin block. Status provides the means to check whether a Slot is missing from the
// database via the SlotCovered() method, and to see the current StartGap() and EndGap().
type Status struct {
	start       primitives.Slot
	end         primitives.Slot
	store       BackfillDB
	genesisSync bool
}

// SlotCovered uses StartGap() and EndGap() to determine if the given slot is covered by the current chain history.
// If the slot is <= StartGap(), or >= EndGap(), the result is true.
// If the slot is between StartGap() and EndGap(), the result is false.
func (s *Status) SlotCovered(sl primitives.Slot) bool {
	// short circuit if the node was synced from genesis
	if s.genesisSync {
		return true
	}
	if s.StartGap() < sl && sl < s.EndGap() {
		return false
	}
	return true
}

// StartGap returns the slot at the beginning of the range that needs to be backfilled.
func (s *Status) StartGap() primitives.Slot {
	return s.start
}

// EndGap returns the slot at the end of the range that needs to be backfilled.
func (s *Status) EndGap() primitives.Slot {
	return s.end
}

// Reload queries the database for backfill status, initializing the internal data and validating the database state.
func (s *Status) Reload(ctx context.Context) error {
	cpRoot, err := s.store.OriginCheckpointBlockRoot(ctx)
	if err != nil {
		// mark genesis sync and short circuit further lookups
		if errors.Is(err, db.ErrNotFoundOriginBlockRoot) {
			s.genesisSync = true
			return nil
		}
		return err
	}
	cpBlock, err := s.store.Block(ctx, cpRoot)
	if err != nil {
		return errors.Wrapf(err, "error retrieving block for origin checkpoint root=%#x", cpRoot)
	}
	if err := blocks.BeaconBlockIsNil(cpBlock); err != nil {
		return err
	}
	s.end = cpBlock.Block().Slot()

	_, err = s.store.GenesisBlockRoot(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFoundGenesisBlockRoot) {
			return errors.Wrap(err, "genesis block root required for checkpoint sync")
		}
		return err
	}

	bfRoot, err := s.store.BackfillBlockRoot(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNotFoundBackfillBlockRoot) {
			return errors.Wrap(err, "found origin checkpoint block root, but no backfill block root")
		}
		return err
	}
	bfBlock, err := s.store.Block(ctx, bfRoot)
	if err != nil {
		return errors.Wrapf(err, "error retrieving block for backfill root=%#x", bfRoot)
	}
	if err := blocks.BeaconBlockIsNil(bfBlock); err != nil {
		return err
	}
	s.start = bfBlock.Block().Slot()
	return nil
}

// BackfillDB describes the set of DB methods that the Status type needs to function.
type BackfillDB interface {
	SaveBackfillBlockRoot(ctx context.Context, blockRoot [32]byte) error
	GenesisBlockRoot(ctx context.Context) ([32]byte, error)
	OriginCheckpointBlockRoot(ctx context.Context) ([32]byte, error)
	BackfillBlockRoot(ctx context.Context) ([32]byte, error)
	Block(ctx context.Context, blockRoot [32]byte) (interfaces.ReadOnlySignedBeaconBlock, error)
}
