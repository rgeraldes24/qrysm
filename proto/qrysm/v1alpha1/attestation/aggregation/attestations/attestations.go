package attestations

import (
	"golang.org/x/exp/slices" // TODO(rgeraldes24) replace with stdlib with go 1.21
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/attestation"
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

	participantsToAdd := attestation.NewBits(baseAtt.ParticipationBits, newAtt.ParticipationBits)
	// base attestation already contains all the participants of the new attestation
	if len(participantsToAdd) == 0 {
		return baseAtt, nil
	}

	// update the signatures slice
	// 1. map the new attestation participants to their signature
	// 2. figure out the insert index of the participants to add(sorted) on the slice of
	// the base participants(sorted) and update the base signatures slice accordigly
	mapNewAttParticipantToSig := make(map[int][]byte)
	for i, participant := range newAtt.ParticipationBits.BitIndices() {
		// @NOTE(rgeraldes24) we could just map the ones we need 
		mapNewAttParticipantToSig[participant] = newAtt.Signatures[i]
	}

	baseParticipants := baseAtt.ParticipationBits.BitIndices()
	startingIdx := 0
	for i, participant := range participantsToAdd {
		insertIdx, err := attestation.SearchInsertIdxWithStartingIdx(baseParticipants, startingIdx, participantNum)
		if err != nil {
			return nil, err
		}

		// no need for more index searches; just append the signatures of the remaining 
		// participants that we need to add.
		if insertIdx > (len(baseParticipants) - 1) {
			for _, missingParticipant := range participantsToAdd[i:] {
				slices.Insert(baseAtt.Signatures, insertIdx, mapNewAttParticipantToSig[missingParticipant])
			}
			break
		}

		slices.Insert(baseParticipants, insertIdx, participant)
		slices.Insert(baseAtt.Signatures, insertIdx, mapNewAttParticipantToSig[participantNum]])
		startingIdx = insertIdx + 1
	}

	// update the participants bitlist
	participants, err := baseAtt.ParticipationBits.Or(newAtt.ParticipationBits)
	if err != nil {
		return nil, err
	}
	baseAtt.ParticipationBits = participants

	return baseAtt, nil
}
