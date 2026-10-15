package v1

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/common/hexutil"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/network/forks"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/time/slots"
)

// MapForkInfo maps the fork active at slot, plus the genesis validators root,
// to the contract's fork_info.
func MapForkInfo(slot primitives.Slot, genesisValidatorsRoot []byte) (*ForkInfo, error) {
	gvr, err := fixedBytes("genesis validators root", genesisValidatorsRoot, fieldparams.RootLength)
	if err != nil {
		return nil, err
	}
	fork, err := forks.Fork(slots.ToEpoch(slot))
	if err != nil {
		return nil, errors.Wrap(err, "could not get fork info")
	}
	return &ForkInfo{
		Fork: &Fork{
			PreviousVersion: fork.PreviousVersion,
			CurrentVersion:  fork.CurrentVersion,
			Epoch:           uintString(fork.Epoch),
		},
		GenesisValidatorsRoot: gvr,
	}, nil
}

// MapAggregateAndProof maps the AggregateAttestationAndProof proto to the contract.
func MapAggregateAndProof(from *qrysmpb.AggregateAttestationAndProof) (*AggregateAndProof, error) {
	if from == nil {
		return nil, errors.New("aggregate attestation and proof is nil")
	}
	aggregate, err := MapAttestation(from.Aggregate)
	if err != nil {
		return nil, err
	}
	selectionProof, err := fixedBytes("selection proof", from.SelectionProof, fieldparams.MLDSA87SignatureLength)
	if err != nil {
		return nil, err
	}
	return &AggregateAndProof{
		AggregatorIndex: uintString(from.AggregatorIndex),
		Aggregate:       aggregate,
		SelectionProof:  selectionProof,
	}, nil
}

// MapAttestation maps the Attestation proto to the contract. The aggregation
// bits keep their SSZ delimiter bit, and the signatures stay one per
// participant.
func MapAttestation(attestation *qrysmpb.Attestation) (*Attestation, error) {
	if attestation == nil {
		return nil, errors.New("attestation is nil")
	}
	if len(attestation.AggregationBits) == 0 {
		return nil, errors.New("aggregation bits in attestation are empty")
	}
	data, err := MapAttestationData(attestation.Data)
	if err != nil {
		return nil, err
	}
	signatures, err := signatureList(attestation.Signatures)
	if err != nil {
		return nil, err
	}
	return &Attestation{
		AggregationBits: hexutil.Bytes(attestation.AggregationBits),
		Data:            data,
		Signatures:      signatures,
	}, nil
}

// MapAttestationData maps the AttestationData proto to the contract.
func MapAttestationData(data *qrysmpb.AttestationData) (*AttestationData, error) {
	if data == nil {
		return nil, errors.New("attestation data is nil")
	}
	source, err := MapCheckPoint(data.Source)
	if err != nil {
		return nil, errors.Wrap(err, "could not map source for attestation data")
	}
	target, err := MapCheckPoint(data.Target)
	if err != nil {
		return nil, errors.Wrap(err, "could not map target for attestation data")
	}
	blockRoot, err := fixedBytes("beacon block root", data.BeaconBlockRoot, fieldparams.RootLength)
	if err != nil {
		return nil, err
	}
	return &AttestationData{
		Slot:            uintString(data.Slot),
		Index:           uintString(data.CommitteeIndex),
		BeaconBlockRoot: blockRoot,
		Source:          source,
		Target:          target,
	}, nil
}

// MapCheckPoint maps the Checkpoint proto to the contract.
func MapCheckPoint(checkpoint *qrysmpb.Checkpoint) (*Checkpoint, error) {
	if checkpoint == nil {
		return nil, errors.New("checkpoint is nil")
	}
	root, err := fixedBytes("checkpoint root", checkpoint.Root, fieldparams.RootLength)
	if err != nil {
		return nil, err
	}
	return &Checkpoint{
		Epoch: uintString(checkpoint.Epoch),
		Root:  root,
	}, nil
}

// MapSyncAggregatorSelectionData maps the SyncAggregatorSelectionData proto to the contract.
func MapSyncAggregatorSelectionData(data *qrysmpb.SyncAggregatorSelectionData) (*SyncAggregatorSelectionData, error) {
	if data == nil {
		return nil, errors.New("sync aggregator selection data is nil")
	}
	return &SyncAggregatorSelectionData{
		Slot:              uintString(data.Slot),
		SubcommitteeIndex: uintString(data.SubcommitteeIndex),
	}, nil
}

// MapContributionAndProof maps the ContributionAndProof proto to the contract.
// The proto field block_root becomes beacon_block_root.
func MapContributionAndProof(contribution *qrysmpb.ContributionAndProof) (*ContributionAndProof, error) {
	if contribution == nil {
		return nil, errors.New("contribution and proof is nil")
	}
	c := contribution.Contribution
	if c == nil {
		return nil, errors.New("sync committee contribution is nil")
	}
	if len(c.AggregationBits) == 0 {
		return nil, errors.New("aggregation bits in sync committee contribution are empty")
	}
	selectionProof, err := fixedBytes("selection proof", contribution.SelectionProof, fieldparams.MLDSA87SignatureLength)
	if err != nil {
		return nil, err
	}
	blockRoot, err := fixedBytes("beacon block root", c.BlockRoot, fieldparams.RootLength)
	if err != nil {
		return nil, err
	}
	signatures, err := signatureList(c.Signatures)
	if err != nil {
		return nil, err
	}
	return &ContributionAndProof{
		AggregatorIndex: uintString(contribution.AggregatorIndex),
		SelectionProof:  selectionProof,
		Contribution: &SyncCommitteeContribution{
			Slot:              uintString(c.Slot),
			BeaconBlockRoot:   blockRoot,
			SubcommitteeIndex: uintString(c.SubcommitteeIndex),
			AggregationBits:   hexutil.Bytes(c.AggregationBits),
			Signatures:        signatures,
		},
	}, nil
}

// uintString encodes a uint64-based value as the contract's decimal string.
func uintString[T ~uint64](v T) string {
	return strconv.FormatUint(uint64(v), 10)
}

// fixedBytes checks that b has exactly n bytes, so a request never carries a
// field the signer would reject for its length.
func fixedBytes(name string, b []byte, n int) (hexutil.Bytes, error) {
	if len(b) != n {
		return nil, fmt.Errorf("invalid %s length: got %d bytes, want %d", name, len(b), n)
	}
	return hexutil.Bytes(b), nil
}

// signatureList maps per-participant signatures. It never returns nil, so an
// empty list encodes as [] rather than null.
func signatureList(sigs [][]byte) ([]hexutil.Bytes, error) {
	out := make([]hexutil.Bytes, 0, len(sigs))
	for i, sig := range sigs {
		s, err := fixedBytes(fmt.Sprintf("signature %d", i), sig, fieldparams.MLDSA87SignatureLength)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
