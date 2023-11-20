package attestations

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation/aggregation"
)

// attList represents list of attestations, defined for easier en masse operations (filtering, sorting).
type attList []*zondpb.Attestation

var _ = logrus.WithField("prefix", "aggregation.attestations")

// ErrInvalidAttestationCount is returned when insufficient number
// of attestations is provided for aggregation.
var ErrInvalidAttestationCount = errors.New("invalid number of attestations")

// Aggregate aggregates attestations. The minimal number of attestations is returned.
// Aggregation occurs in-place i.e. contents of input array will be modified. Should you need to
// preserve input attestations, clone them before aggregating.
func Aggregate(atts []*zondpb.Attestation) ([]*zondpb.Attestation, error) {
	return MaxCoverAttestationAggregation(atts)
}

/*
// AggregateDisjointOneBitAtts aggregates unaggregated attestations with the
// exact same attestation data.
func AggregateDisjointOneBitAtts(atts []*zondpb.Attestation) (*zondpb.Attestation, error) {
	if len(atts) == 0 {
		return nil, nil
	}
	if len(atts) == 1 {
		return atts[0], nil
	}
	coverage, err := atts[0].ParticipationBits.ToBitlist64()
	if err != nil {
		return nil, errors.Wrap(err, "could not get aggregation bits")
	}
	for _, att := range atts[1:] {
		bits, err := att.ParticipationBits.ToBitlist64()
		if err != nil {
			return nil, errors.Wrap(err, "could not get aggregation bits")
		}
		err = coverage.NoAllocOr(bits, coverage)
		if err != nil {
			return nil, errors.Wrap(err, "could not get aggregation bits")
		}
	}
	keys := make([]int, len(atts))
	for i := 0; i < len(atts); i++ {
		keys[i] = i
	}
	idx, err := aggregateAttestations(atts, keys, coverage)
	if err != nil {
		return nil, errors.Wrap(err, "could not aggregate attestations")
	}
	if idx != 0 {
		return nil, errors.New("could not aggregate attestations, obtained non zero index")
	}
	return atts[0], nil
}
*/

// AggregatePair aggregates pair of attestations a1 and a2 together.
func AggregatePair(a1, a2 *zondpb.Attestation) (*zondpb.Attestation, error) {
	o, err := a1.ParticipationBits.Overlaps(a2.ParticipationBits)
	if err != nil {
		return nil, err
	}
	if o {
		return nil, aggregation.ErrBitsOverlap
	}

	baseAtt := zondpb.CopyAttestation(a1)
	newAtt := zondpb.CopyAttestation(a2)
	if newAtt.ParticipationBits.Count() > baseAtt.ParticipationBits.Count() {
		baseAtt, newAtt = newAtt, baseAtt
	}

	newParticipants := make([]uint64, 0)
	for i := 0; i < len(baseAtt.ParticipationBits); i++ {
		// start checking the byte and move to bits if a new participant is found
		if baseAtt.ParticipationBits[i]^(baseAtt.ParticipationBits[i]|newAtt.ParticipationBits[i]) != 0 {
			// identify the new participants in this byte
			var bitIdx uint64 = uint64(i) * 8
			for j := 0; j < 8; j, bitIdx = j+1, bitIdx+1 {
				// base attestation bit must be set to zero and the new attestation bit must be set to one
				if !baseAtt.ParticipationBits.BitAt(bitIdx) && newAtt.ParticipationBits.BitAt(bitIdx) {
					newParticipants = append(newParticipants, bitIdx)
				}
			}
		}
	}

	// base attestation already contains all the participants of the new attestation
	if len(newParticipants) == 0 {
		return baseAtt, nil
	}

	// convert the signaturesIdxToParticipationIdx from a list to a map to allow for
	// a quick search for the sig index that we will use to include the new signature
	mapParticipationIdxToSigIdx := make(map[uint64]int)
	for sigIdx, participationIdx := range newAtt.SignaturesIdxToParticipationIdx {
		mapParticipationIdxToSigIdx[participationIdx] = sigIdx
	}

	// include sig and participation
	for _, participationIdx := range newParticipants {
		sigIdx, ok := mapParticipationIdxToSigIdx[participationIdx]
		if !ok {
			return nil, fmt.Errorf("Signature for validator with index %d not found", participationIdx)
		}
		baseAtt.Signatures = append(baseAtt.Signatures, newAtt.Signatures[sigIdx])
		baseAtt.SignaturesIdxToParticipationIdx = append(baseAtt.SignaturesIdxToParticipationIdx, participationIdx)
		baseAtt.ParticipationBits.SetBitAt(participationIdx, true)
	}

	return baseAtt, nil
}
