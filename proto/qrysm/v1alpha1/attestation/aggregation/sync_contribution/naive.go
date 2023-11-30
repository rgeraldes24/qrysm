package sync_contribution

import (
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation/aggregation"
	"golang.org/x/exp/slices" // TODO(rgeraldes24) replace with stdlib with go 1.21
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

	contributorsToAdd := attestation.NewBits(baseContribution.ParticipationBits, newContribution.ParticipationBits)
	// base contribution already contains all the participants of the new attestation
	if len(contributorsToAdd) == 0 {
		return baseContribution, nil
	}

	// update the signatures slice
	// 1. map the new contribution participants to their signature
	// 2. figure out the insert index of the contributors to add(sorted) on the slice of
	// the base contributors(sorted) and update the base signatures slice accordigly
	mapNewContributionParticipantToSig := make(map[int][]byte)
	for i, participant := range newContribution.ParticipationBits.BitIndices() {
		// @NOTE(rgeraldes24) we could just map the ones we need
		mapNewContributionParticipantToSig[participant] = newContribution.Signatures[i]
	}

	baseParticipants := baseContribution.ParticipationBits.BitIndices()
	startingIdx := 0
	for i, participant := range contributorsToAdd {
		insertIdx, err := attestation.SearchInsertIdxWithStartingIdx(baseParticipants, startingIdx, participantNum)
		if err != nil {
			return nil, err
		}

		// no need for more index searches; just append the signatures of the remaining
		// participants that we need to add.
		if insertIdx > (len(baseParticipants) - 1) {
			for _, missingParticipant := range participantsToAdd[i:] {
				slices.Insert(baseContribution.Signatures, insertIdx, mapNewContributionParticipantToSig[missingParticipant])
			}
			break
		}

		slices.Insert(baseParticipants, insertIdx, participant)
		slices.Insert(baseContribution.Signatures, insertIdx, mapNewContributionParticipantToSig[participant])
		startingIdx = insertIdx + 1
	}

	// update the participants bitlist
	participants, err := baseContribution.ParticipationBits.Or(newContribution.ParticipationBits)
	if err != nil {
		return nil, err
	}
	baseContribution.ParticipationBits = participants

	return baseContribution, nil
}
