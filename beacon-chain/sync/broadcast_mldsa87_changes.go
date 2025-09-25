package sync

import (
	"context"
	"time"

	"github.com/theQRL/qrysm/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	types "github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/rand"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

const broadcastMLDSA87ChangesRateLimit = 128

// This routine broadcasts known ML-DSA-87 changes at the Capella fork.
func (s *Service) broadcastMLDSA87Changes(currSlot types.Slot) {
	capellaSlotStart := primitives.Slot(0)
	if currSlot != capellaSlotStart {
		return
	}
	changes, err := s.cfg.mlDSA87ToExecPool.PendingMLDSA87ToExecChanges()
	if err != nil {
		log.WithError(err).Error("could not get ML-DSA-87 to execution changes")
	}
	if len(changes) == 0 {
		return
	}
	source := rand.NewGenerator()
	length := len(changes)
	broadcastChanges := make([]*qrysmpb.SignedMLDSA87ToExecutionChange, length)
	for i := 0; i < length; i++ {
		idx := source.Intn(len(changes))
		broadcastChanges[i] = changes[idx]
		changes = append(changes[:idx], changes[idx+1:]...)
	}

	go s.rateMLDSA87Changes(s.ctx, broadcastChanges)
}

func (s *Service) broadcastMLDSA87Batch(ctx context.Context, ptr *[]*qrysmpb.SignedMLDSA87ToExecutionChange) {
	limit := broadcastMLDSA87ChangesRateLimit
	if len(*ptr) < broadcastMLDSA87ChangesRateLimit {
		limit = len(*ptr)
	}
	st, err := s.cfg.chain.HeadStateReadOnly(ctx)
	if err != nil {
		log.WithError(err).Error("could not get head state")
		return
	}
	for _, ch := range (*ptr)[:limit] {
		if ch != nil {
			_, err := blocks.ValidateMLDSA87ToExecutionChange(st, ch)
			if err != nil {
				log.WithError(err).Error("could not validate ML-DSA-87 to execution change")
				continue
			}
			if err := s.cfg.p2p.Broadcast(ctx, ch); err != nil {
				log.WithError(err).Error("could not broadcast ML-DSA-87 to execution changes.")
			}
		}
	}
	*ptr = (*ptr)[limit:]
}

func (s *Service) rateMLDSA87Changes(ctx context.Context, changes []*qrysmpb.SignedMLDSA87ToExecutionChange) {
	s.broadcastMLDSA87Batch(ctx, &changes)
	if len(changes) == 0 {
		return
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.broadcastMLDSA87Batch(ctx, &changes)
			if len(changes) == 0 {
				return
			}
		}
	}
}
