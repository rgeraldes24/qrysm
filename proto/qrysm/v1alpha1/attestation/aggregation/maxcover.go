package aggregation

import (
	"github.com/pkg/errors"
	"github.com/theQRL/go-bitfield"
)

// ErrInvalidMaxCoverProblem is returned when Maximum Coverage problem was initialized incorrectly.
var ErrInvalidMaxCoverProblem = errors.New("invalid max_cover problem")

// MaxCover finds the k-cover of Maximum Coverage problem.
//
// Problem is defined as MaxCover(U, S, k): S', where:
// U is a finite set of objects, where |U| = n. Furthermore, let S = {S_1, ..., S_m} be all
// subsets of U, that's their union is equal to U. Then, Maximum Coverage is the problem of
// finding such a collection S' of subsets from S, where |S'| <= k, and union of all subsets in S'
// covering U with maximum cardinality.
//
// The current implementation captures the original MaxCover problem, and the variant where
// additional invariant is enforced: all elements of S' must be disjoint. This comes handy when
// we need to aggregate bitsets, and overlaps are not allowed.
//
// For more details, see:
// "Analysis of the Greedy Approach in Problems of Maximum k-Coverage" by Hochbaum and Pathria.
// https://hochbaum.ieor.berkeley.edu/html/pub/HPathria-max-k-coverage-greedy.pdf
func MaxCover(candidates []*bitfield.Bitlist64, k int, allowOverlaps bool) (selected, coverage *bitfield.Bitlist64, err error) {
	if len(candidates) == 0 {
		return nil, nil, errors.Wrap(ErrInvalidMaxCoverProblem, "cannot calculate set coverage")
	}
	if len(candidates) < k {
		k = len(candidates)
	}

	// Track usable candidates, and candidates selected for coverage as two bitlists.
	selectedCandidates := bitfield.NewBitlist64(uint64(len(candidates)))
	usableCandidates := bitfield.NewBitlist64(uint64(len(candidates))).Not()

	// Track bits covered so far as a bitlist.
	coveredBits := bitfield.NewBitlist64(candidates[0].Len())
	remainingBits, err := union(candidates)
	if err != nil {
		return nil, nil, err
	}
	if remainingBits == nil {
		return nil, nil, errors.Wrap(ErrInvalidMaxCoverProblem, "empty bitlists")
	}

	attempts := 0
	tmpBitlist := bitfield.NewBitlist64(candidates[0].Len()) // Used as return param for NoAlloc*() methods.
	indices := make([]int, usableCandidates.Count())
	for selectedCandidates.Count() < uint64(k) && usableCandidates.Count() > 0 {
		// Safe-guard, each iteration should come with at least one candidate selected.
		if attempts > k {
			break
		}
		attempts++

		// Greedy select the next best candidate (from usable ones) to cover the remaining bits maximally.
		maxScore := uint64(0)
		bestIdx := uint64(0)
		indices = indices[0:usableCandidates.Count()]
		usableCandidates.NoAllocBitIndices(indices)
		for _, idx := range indices {
			// Score is calculated by taking into account uncovered bits only.
			score := uint64(0)
			if candidates[idx].Len() == remainingBits.Len() {
				var err error
				score, err = candidates[idx].AndCount(remainingBits)
				if err != nil {
					return nil, nil, err
				}
			}

			// Filter out zero-score candidates.
			if score == 0 {
				usableCandidates.SetBitAt(uint64(idx), false)
				continue
			}

			// Filter out overlapping candidates (if overlapping is not allowed).
			wrongLen := coveredBits.Len() != candidates[idx].Len()
			overlaps := func(idx int) (bool, error) {
				o, err := coveredBits.Overlaps(candidates[idx])
				return !allowOverlaps && o, err
			}
			if wrongLen { // Shortcut for wrong length check
				usableCandidates.SetBitAt(uint64(idx), false)
				continue
			} else if o, err := overlaps(idx); err != nil {
				return nil, nil, err
			} else if o {
				usableCandidates.SetBitAt(uint64(idx), false)
				continue
			}

			// Track the candidate with the best score.
			if score > maxScore {
				maxScore = score
				bestIdx = uint64(idx)
			}
		}
		// Process greedy selected candidate.
		if maxScore > 0 {
			if err := coveredBits.NoAllocOr(candidates[bestIdx], coveredBits); err != nil {
				return nil, nil, err
			}
			selectedCandidates.SetBitAt(bestIdx, true)
			candidates[bestIdx].NoAllocNot(tmpBitlist)
			if err := remainingBits.NoAllocAnd(tmpBitlist, remainingBits); err != nil {
				return nil, nil, err
			}
			usableCandidates.SetBitAt(bestIdx, false)
		}
	}
	return selectedCandidates, coveredBits, nil
}

func union(candidates []*bitfield.Bitlist64) (*bitfield.Bitlist64, error) {
	if len(candidates) == 0 || candidates[0].Len() == 0 {
		return nil, nil
	}
	ret := bitfield.NewBitlist64(candidates[0].Len())
	for _, bl := range candidates {
		if ret.Len() == bl.Len() {
			if err := ret.NoAllocOr(bl, ret); err != nil {
				return nil, err
			}
		}
	}
	return ret, nil
}
