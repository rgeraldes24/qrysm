package shared

import (
	"strconv"

	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zond "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

type Attestation struct {
	ParticipationBits string           `json:"aggregation_bits" validate:"required,hexadecimal"`
	Data              *AttestationData `json:"data" validate:"required"`
	Signatures        []string         `json:"signatures" validate:"required,hexadecimal"`
}

type AttestationData struct {
	Slot            string      `json:"slot" validate:"required,number,gte=0"`
	CommitteeIndex  string      `json:"index" validate:"required,number,gte=0"`
	BeaconBlockRoot string      `json:"beacon_block_root" validate:"required,hexadecimal"`
	Source          *Checkpoint `json:"source" validate:"required"`
	Target          *Checkpoint `json:"target" validate:"required"`
}

type Checkpoint struct {
	Epoch string `json:"epoch" validate:"required,number,gte=0"`
	Root  string `json:"root" validate:"required,hexadecimal"`
}

type SignedContributionAndProof struct {
	Message   *ContributionAndProof `json:"message" validate:"required"`
	Signature string                `json:"signature" validate:"required,hexadecimal"`
}

type ContributionAndProof struct {
	AggregatorIndex string                     `json:"aggregator_index" validate:"required,number,gte=0"`
	Contribution    *SyncCommitteeContribution `json:"contribution" validate:"required"`
	SelectionProof  string                     `json:"selection_proof" validate:"required,hexadecimal"`
}

type SyncCommitteeContribution struct {
	Slot              string   `json:"slot" validate:"required,number,gte=0"`
	BeaconBlockRoot   string   `json:"beacon_block_root" hex:"true" validate:"required,hexadecimal"`
	SubcommitteeIndex string   `json:"subcommittee_index" validate:"required,number,gte=0"`
	ParticipationBits string   `json:"aggregation_bits" hex:"true" validate:"required,hexadecimal"`
	Signatures        []string `json:"signatures" hex:"true" validate:"required,hexadecimal"`
}

type SignedAggregateAttestationAndProof struct {
	Message   *AggregateAttestationAndProof `json:"message" validate:"required"`
	Signature string                        `json:"signature" validate:"required,hexadecimal"`
}

type AggregateAttestationAndProof struct {
	AggregatorIndex string       `json:"aggregator_index" validate:"required,number,gte=0"`
	Aggregate       *Attestation `json:"aggregate" validate:"required"`
	SelectionProof  string       `json:"selection_proof" validate:"required,hexadecimal"`
}

func (s *SignedContributionAndProof) ToConsensus() (*zond.SignedContributionAndProof, error) {
	msg, err := s.Message.ToConsensus()
	if err != nil {
		return nil, NewDecodeError(err, "Message")
	}
	sig, err := hexutil.Decode(s.Signature)
	if err != nil {
		return nil, NewDecodeError(err, "Signature")
	}

	return &zond.SignedContributionAndProof{
		Message:   msg,
		Signature: sig,
	}, nil
}

func (c *ContributionAndProof) ToConsensus() (*zond.ContributionAndProof, error) {
	contribution, err := c.Contribution.ToConsensus()
	if err != nil {
		return nil, NewDecodeError(err, "Contribution")
	}
	aggregatorIndex, err := strconv.ParseUint(c.AggregatorIndex, 10, 64)
	if err != nil {
		return nil, NewDecodeError(err, "AggregatorIndex")
	}
	selectionProof, err := hexutil.Decode(c.SelectionProof)
	if err != nil {
		return nil, NewDecodeError(err, "SelectionProof")
	}

	return &zond.ContributionAndProof{
		AggregatorIndex: primitives.ValidatorIndex(aggregatorIndex),
		Contribution:    contribution,
		SelectionProof:  selectionProof,
	}, nil
}

func (s *SyncCommitteeContribution) ToConsensus() (*zond.SyncCommitteeContribution, error) {
	slot, err := strconv.ParseUint(s.Slot, 10, 64)
	if err != nil {
		return nil, NewDecodeError(err, "Slot")
	}
	bbRoot, err := hexutil.Decode(s.BeaconBlockRoot)
	if err != nil {
		return nil, NewDecodeError(err, "BeaconBlockRoot")
	}
	subcommitteeIndex, err := strconv.ParseUint(s.SubcommitteeIndex, 10, 64)
	if err != nil {
		return nil, NewDecodeError(err, "SubcommitteeIndex")
	}
	participationBits, err := hexutil.Decode(s.ParticipationBits)
	if err != nil {
		return nil, NewDecodeError(err, "ParticipationBits")
	}
	sigs := make([][]byte, len(s.Signatures))
	for i, sig := range s.Signatures {
		s, err := hexutil.Decode(sig)
		if err != nil {
			return nil, NewDecodeError(err, "Signatures")
		}
		sigs[i] = s
	}

	return &zond.SyncCommitteeContribution{
		Slot:              primitives.Slot(slot),
		BlockRoot:         bbRoot,
		SubcommitteeIndex: subcommitteeIndex,
		ParticipationBits: participationBits,
		Signatures:        sigs,
	}, nil
}

func (s *SignedAggregateAttestationAndProof) ToConsensus() (*zond.SignedAggregateAttestationAndProof, error) {
	msg, err := s.Message.ToConsensus()
	if err != nil {
		return nil, NewDecodeError(err, "Message")
	}
	sig, err := hexutil.Decode(s.Signature)
	if err != nil {
		return nil, NewDecodeError(err, "Signature")
	}

	return &zond.SignedAggregateAttestationAndProof{
		Message:   msg,
		Signature: sig,
	}, nil
}

func (a *AggregateAttestationAndProof) ToConsensus() (*zond.AggregateAttestationAndProof, error) {
	aggIndex, err := strconv.ParseUint(a.AggregatorIndex, 10, 64)
	if err != nil {
		return nil, NewDecodeError(err, "AggregatorIndex")
	}
	agg, err := a.Aggregate.ToConsensus()
	if err != nil {
		return nil, NewDecodeError(err, "Aggregate")
	}
	proof, err := hexutil.Decode(a.SelectionProof)
	if err != nil {
		return nil, NewDecodeError(err, "SelectionProof")
	}
	return &zond.AggregateAttestationAndProof{
		AggregatorIndex: primitives.ValidatorIndex(aggIndex),
		Aggregate:       agg,
		SelectionProof:  proof,
	}, nil
}

func (a *Attestation) ToConsensus() (*zond.Attestation, error) {
	participationBits, err := hexutil.Decode(a.ParticipationBits)
	if err != nil {
		return nil, NewDecodeError(err, "ParticipationBits")
	}
	data, err := a.Data.ToConsensus()
	if err != nil {
		return nil, NewDecodeError(err, "Data")
	}
	sigs := make([][]byte, len(a.Signatures))
	for i, sig := range a.Signatures {
		s, err := hexutil.Decode(sig)
		if err != nil {
			return nil, NewDecodeError(err, "Signatures")
		}
		sigs[i] = s
	}

	return &zond.Attestation{
		ParticipationBits: participationBits,
		Data:              data,
		Signatures:        sigs,
	}, nil
}

func (a *AttestationData) ToConsensus() (*zond.AttestationData, error) {
	slot, err := strconv.ParseUint(a.Slot, 10, 64)
	if err != nil {
		return nil, NewDecodeError(err, "Slot")
	}
	committeeIndex, err := strconv.ParseUint(a.CommitteeIndex, 10, 64)
	if err != nil {
		return nil, NewDecodeError(err, "CommitteeIndex")
	}
	bbRoot, err := hexutil.Decode(a.BeaconBlockRoot)
	if err != nil {
		return nil, NewDecodeError(err, "BeaconBlockRoot")
	}
	source, err := a.Source.ToConsensus()
	if err != nil {
		return nil, NewDecodeError(err, "Source")
	}
	target, err := a.Target.ToConsensus()
	if err != nil {
		return nil, NewDecodeError(err, "Target")
	}

	return &zond.AttestationData{
		Slot:            primitives.Slot(slot),
		CommitteeIndex:  primitives.CommitteeIndex(committeeIndex),
		BeaconBlockRoot: bbRoot,
		Source:          source,
		Target:          target,
	}, nil
}

func (c *Checkpoint) ToConsensus() (*zond.Checkpoint, error) {
	epoch, err := strconv.ParseUint(c.Epoch, 10, 64)
	if err != nil {
		return nil, NewDecodeError(err, "Epoch")
	}
	root, err := hexutil.Decode(c.Root)
	if err != nil {
		return nil, NewDecodeError(err, "Root")
	}

	return &zond.Checkpoint{
		Epoch: primitives.Epoch(epoch),
		Root:  root,
	}, nil
}