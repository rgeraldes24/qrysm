package sync_contribution

import (
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation/aggregation"
)

// naiveSyncContributionAggregation aggregates naively, without any complex algorithms or optimizations.
// Note: this is currently a naive implementation to the order of O(mn^2).
func naiveSyncContributionAggregation(contributions []*zondpb.SyncCommitteeContribution) ([]*zondpb.SyncCommitteeContribution, error) {
	if len(contributions) <= 1 {
		return contributions, nil
	}

	// Naive aggregation. O(n^2) time.
	for i, a := range contributions {
		if i >= len(contributions) {
			break
		}
		for j := i + 1; j < len(contributions); j++ {
			b := contributions[j]
			if o, err := a.ParticipationBits.Overlaps(b.ParticipationBits); err != nil {
				return nil, err
			} else if !o {
				var err error
				a, err = naiveAggregate(a, b)
				if err != nil {
					return nil, err
				}
				// Delete b
				contributions = append(contributions[:j], contributions[j+1:]...)
				j--
				contributions[i] = a
			}
		}
	}

	// Naive deduplication of identical contributions. O(n^2) time.
	for i, a := range contributions {
		for j := i + 1; j < len(contributions); j++ {
			b := contributions[j]

			if a.ParticipationBits.Len() != b.ParticipationBits.Len() {
				continue
			}

			if c, err := a.ParticipationBits.Contains(b.ParticipationBits); err != nil {
				return nil, err
			} else if c {
				// If b is fully contained in a, then b can be removed.
				contributions = append(contributions[:j], contributions[j+1:]...)
				j--
			} else if c, err := b.ParticipationBits.Contains(a.ParticipationBits); err != nil {
				return nil, err
			} else if c {
				// if a is fully contained in b, then a can be removed.
				contributions = append(contributions[:i], contributions[i+1:]...)
				break // Stop the inner loop, advance a.
			}
		}
	}

	return contributions, nil
}

// aggregates pair of sync contributions c1 and c2 together.
func naiveAggregate(c1, c2 *zondpb.SyncCommitteeContribution) (*zondpb.SyncCommitteeContribution, error) {
	o, err := c1.ParticipationBits.Overlaps(c2.ParticipationBits)
	if err != nil {
		return nil, err
	}
	if o {
		return nil, aggregation.ErrBitsOverlap
	}

	baseContribution := zondpb.CopySyncCommitteeContribution(c1)
	newContribution := zondpb.CopySyncCommitteeContribution(c2)
	if newContribution.ParticipationBits.Count() > baseContribution.ParticipationBits.Count() {
		baseContribution, newContribution = newContribution, baseContribution
	}

	newParticipants := make([]uint64, 0)
	for i := 0; i < len(baseContribution.ParticipationBits); i++ {
		// start checking the byte and move to bits if a new participant is found
		if baseContribution.ParticipationBits[i]^(baseContribution.ParticipationBits[i]|newContribution.ParticipationBits[i]) != 0 {
			// identify the new participants in this byte
			var bitIdx uint64 = uint64(i) * 8
			for j := 0; j < 8; j, bitIdx = j+1, bitIdx+1 {
				// base contribution bit must be set to zero and the new contribution bit must be set to one
				if !baseContribution.ParticipationBits.BitAt(bitIdx) && newContribution.ParticipationBits.BitAt(bitIdx) {
					newParticipants = append(newParticipants, bitIdx)
				}
			}
		}
	}

	// base contribution already contains all the participants of the new contribution
	if len(newParticipants) == 0 {
		return baseContribution, nil
	}

	// TODO(rgeraldes24)

	/*
		// convert the signaturesIdxToParticipationIdx from a list to a map to allow for
		// a quick search for the sig index that we will use to include the new signature
		mapParticipationIdxToSigIdx := make(map[uint64]int)
		for sigIdx, participationIdx := range newContribution.SignaturesIdxToParticipationIdx {
			mapParticipationIdxToSigIdx[participationIdx] = sigIdx
		}

		// include sig and participation
		for _, participationIdx := range newParticipants {
			sigIdx, ok := mapParticipationIdxToSigIdx[participationIdx]
			if !ok {
				return nil, fmt.Errorf("Signature for validator with index %d not found", participationIdx)
			}
			baseContribution.Signatures = append(baseContribution.Signatures, newContribution.Signatures[sigIdx])
			baseContribution.SignaturesIdxToParticipationIdx = append(baseContribution.SignaturesIdxToParticipationIdx, participationIdx)
			baseContribution.ParticipationBits.SetBitAt(participationIdx, true)
		}
	*/

	return baseContribution, nil
}
