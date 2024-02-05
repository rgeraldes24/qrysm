package migration

import (
	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	zondpbalpha "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
)

// BlockIfaceToV1BlockHeader converts a signed beacon block interface into a signed beacon block header.
func BlockIfaceToV1BlockHeader(block interfaces.ReadOnlySignedBeaconBlock) (*zondpbv1.SignedBeaconBlockHeader, error) {
	bodyRoot, err := block.Block().Body().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get body root of block")
	}
	parentRoot := block.Block().ParentRoot()
	stateRoot := block.Block().StateRoot()
	sig := block.Signature()
	return &zondpbv1.SignedBeaconBlockHeader{
		Message: &zondpbv1.BeaconBlockHeader{
			Slot:          block.Block().Slot(),
			ProposerIndex: block.Block().ProposerIndex(),
			ParentRoot:    parentRoot[:],
			StateRoot:     stateRoot[:],
			BodyRoot:      bodyRoot[:],
		},
		Signature: sig[:],
	}, nil
}

// V1Alpha1AggregateAttAndProofToV1 converts a v1alpha1 aggregate attestation and proof to v1.
func V1Alpha1AggregateAttAndProofToV1(v1alpha1Att *zondpbalpha.AggregateAttestationAndProof) *zondpbv1.AggregateAttestationAndProof {
	if v1alpha1Att == nil {
		return &zondpbv1.AggregateAttestationAndProof{}
	}
	return &zondpbv1.AggregateAttestationAndProof{
		AggregatorIndex: v1alpha1Att.AggregatorIndex,
		Aggregate:       V1Alpha1AttestationToV1(v1alpha1Att.Aggregate),
		SelectionProof:  v1alpha1Att.SelectionProof,
	}
}

// V1SignedAggregateAttAndProofToV1Alpha1 converts a v1 signed aggregate attestation and proof to v1alpha1.
func V1SignedAggregateAttAndProofToV1Alpha1(v1Att *zondpbv1.SignedAggregateAttestationAndProof) *zondpbalpha.SignedAggregateAttestationAndProof {
	if v1Att == nil {
		return &zondpbalpha.SignedAggregateAttestationAndProof{}
	}
	return &zondpbalpha.SignedAggregateAttestationAndProof{
		Message: &zondpbalpha.AggregateAttestationAndProof{
			AggregatorIndex: v1Att.Message.AggregatorIndex,
			Aggregate:       V1AttestationToV1Alpha1(v1Att.Message.Aggregate),
			SelectionProof:  v1Att.Message.SelectionProof,
		},
		Signature: v1Att.Signature,
	}
}

// V1Alpha1IndexedAttToV1 converts a v1alpha1 indexed attestation to v1.
func V1Alpha1IndexedAttToV1(v1alpha1Att *zondpbalpha.IndexedAttestation) *zondpbv1.IndexedAttestation {
	if v1alpha1Att == nil {
		return &zondpbv1.IndexedAttestation{}
	}
	return &zondpbv1.IndexedAttestation{
		AttestingIndices: v1alpha1Att.AttestingIndices,
		Data:             V1Alpha1AttDataToV1(v1alpha1Att.Data),
		Signature:        v1alpha1Att.Signature,
	}
}

// V1Alpha1AttestationToV1 converts a v1alpha1 attestation to v1.
func V1Alpha1AttestationToV1(v1alpha1Att *zondpbalpha.Attestation) *zondpbv1.Attestation {
	if v1alpha1Att == nil {
		return &zondpbv1.Attestation{}
	}
	return &zondpbv1.Attestation{
		AggregationBits: v1alpha1Att.AggregationBits,
		Data:            V1Alpha1AttDataToV1(v1alpha1Att.Data),
		Signature:       v1alpha1Att.Signature,
	}
}

// V1AttestationToV1Alpha1 converts a v1 attestation to v1alpha1.
func V1AttestationToV1Alpha1(v1Att *zondpbv1.Attestation) *zondpbalpha.Attestation {
	if v1Att == nil {
		return &zondpbalpha.Attestation{}
	}
	return &zondpbalpha.Attestation{
		AggregationBits: v1Att.AggregationBits,
		Data:            V1AttDataToV1Alpha1(v1Att.Data),
		Signature:       v1Att.Signature,
	}
}

// V1Alpha1AttDataToV1 converts a v1alpha1 attestation data to v1.
func V1Alpha1AttDataToV1(v1alpha1AttData *zondpbalpha.AttestationData) *zondpbv1.AttestationData {
	if v1alpha1AttData == nil || v1alpha1AttData.Source == nil || v1alpha1AttData.Target == nil {
		return &zondpbv1.AttestationData{}
	}
	return &zondpbv1.AttestationData{
		Slot:            v1alpha1AttData.Slot,
		Index:           v1alpha1AttData.CommitteeIndex,
		BeaconBlockRoot: v1alpha1AttData.BeaconBlockRoot,
		Source: &zondpbv1.Checkpoint{
			Root:  v1alpha1AttData.Source.Root,
			Epoch: v1alpha1AttData.Source.Epoch,
		},
		Target: &zondpbv1.Checkpoint{
			Root:  v1alpha1AttData.Target.Root,
			Epoch: v1alpha1AttData.Target.Epoch,
		},
	}
}

// V1Alpha1AttSlashingToV1 converts a v1alpha1 attester slashing to v1.
func V1Alpha1AttSlashingToV1(v1alpha1Slashing *zondpbalpha.AttesterSlashing) *zondpbv1.AttesterSlashing {
	if v1alpha1Slashing == nil {
		return &zondpbv1.AttesterSlashing{}
	}
	return &zondpbv1.AttesterSlashing{
		Attestation_1: V1Alpha1IndexedAttToV1(v1alpha1Slashing.Attestation_1),
		Attestation_2: V1Alpha1IndexedAttToV1(v1alpha1Slashing.Attestation_2),
	}
}

// V1Alpha1SignedHeaderToV1 converts a v1alpha1 signed beacon block header to v1.
func V1Alpha1SignedHeaderToV1(v1alpha1Hdr *zondpbalpha.SignedBeaconBlockHeader) *zondpbv1.SignedBeaconBlockHeader {
	if v1alpha1Hdr == nil || v1alpha1Hdr.Header == nil {
		return &zondpbv1.SignedBeaconBlockHeader{}
	}
	return &zondpbv1.SignedBeaconBlockHeader{
		Message: &zondpbv1.BeaconBlockHeader{
			Slot:          v1alpha1Hdr.Header.Slot,
			ProposerIndex: v1alpha1Hdr.Header.ProposerIndex,
			ParentRoot:    v1alpha1Hdr.Header.ParentRoot,
			StateRoot:     v1alpha1Hdr.Header.StateRoot,
			BodyRoot:      v1alpha1Hdr.Header.BodyRoot,
		},
		Signature: v1alpha1Hdr.Signature,
	}
}

// V1SignedHeaderToV1Alpha1 converts a v1 signed beacon block header to v1alpha1.
func V1SignedHeaderToV1Alpha1(v1Header *zondpbv1.SignedBeaconBlockHeader) *zondpbalpha.SignedBeaconBlockHeader {
	if v1Header == nil || v1Header.Message == nil {
		return &zondpbalpha.SignedBeaconBlockHeader{}
	}
	return &zondpbalpha.SignedBeaconBlockHeader{
		Header: &zondpbalpha.BeaconBlockHeader{
			Slot:          v1Header.Message.Slot,
			ProposerIndex: v1Header.Message.ProposerIndex,
			ParentRoot:    v1Header.Message.ParentRoot,
			StateRoot:     v1Header.Message.StateRoot,
			BodyRoot:      v1Header.Message.BodyRoot,
		},
		Signature: v1Header.Signature,
	}
}

// V1Alpha1ProposerSlashingToV1 converts a v1alpha1 proposer slashing to v1.
func V1Alpha1ProposerSlashingToV1(v1alpha1Slashing *zondpbalpha.ProposerSlashing) *zondpbv1.ProposerSlashing {
	if v1alpha1Slashing == nil {
		return &zondpbv1.ProposerSlashing{}
	}
	return &zondpbv1.ProposerSlashing{
		SignedHeader_1: V1Alpha1SignedHeaderToV1(v1alpha1Slashing.Header_1),
		SignedHeader_2: V1Alpha1SignedHeaderToV1(v1alpha1Slashing.Header_2),
	}
}

// V1Alpha1ExitToV1 converts a v1alpha1 SignedVoluntaryExit to v1.
func V1Alpha1ExitToV1(v1alpha1Exit *zondpbalpha.SignedVoluntaryExit) *zondpbv1.SignedVoluntaryExit {
	if v1alpha1Exit == nil || v1alpha1Exit.Exit == nil {
		return &zondpbv1.SignedVoluntaryExit{}
	}
	return &zondpbv1.SignedVoluntaryExit{
		Message: &zondpbv1.VoluntaryExit{
			Epoch:          v1alpha1Exit.Exit.Epoch,
			ValidatorIndex: v1alpha1Exit.Exit.ValidatorIndex,
		},
		Signature: v1alpha1Exit.Signature,
	}
}

// V1ExitToV1Alpha1 converts a v1 SignedVoluntaryExit to v1alpha1.
func V1ExitToV1Alpha1(v1Exit *zondpbv1.SignedVoluntaryExit) *zondpbalpha.SignedVoluntaryExit {
	if v1Exit == nil || v1Exit.Message == nil {
		return &zondpbalpha.SignedVoluntaryExit{}
	}
	return &zondpbalpha.SignedVoluntaryExit{
		Exit: &zondpbalpha.VoluntaryExit{
			Epoch:          v1Exit.Message.Epoch,
			ValidatorIndex: v1Exit.Message.ValidatorIndex,
		},
		Signature: v1Exit.Signature,
	}
}

// V1AttToV1Alpha1 converts a v1 attestation to v1alpha1.
func V1AttToV1Alpha1(v1Att *zondpbv1.Attestation) *zondpbalpha.Attestation {
	if v1Att == nil {
		return &zondpbalpha.Attestation{}
	}
	return &zondpbalpha.Attestation{
		AggregationBits: v1Att.AggregationBits,
		Data:            V1AttDataToV1Alpha1(v1Att.Data),
		Signature:       v1Att.Signature,
	}
}

// V1IndexedAttToV1Alpha1 converts a v1 indexed attestation to v1alpha1.
func V1IndexedAttToV1Alpha1(v1Att *zondpbv1.IndexedAttestation) *zondpbalpha.IndexedAttestation {
	if v1Att == nil {
		return &zondpbalpha.IndexedAttestation{}
	}
	return &zondpbalpha.IndexedAttestation{
		AttestingIndices: v1Att.AttestingIndices,
		Data:             V1AttDataToV1Alpha1(v1Att.Data),
		Signature:        v1Att.Signature,
	}
}

// V1AttDataToV1Alpha1 converts a v1 attestation data to v1alpha1.
func V1AttDataToV1Alpha1(v1AttData *zondpbv1.AttestationData) *zondpbalpha.AttestationData {
	if v1AttData == nil || v1AttData.Source == nil || v1AttData.Target == nil {
		return &zondpbalpha.AttestationData{}
	}
	return &zondpbalpha.AttestationData{
		Slot:            v1AttData.Slot,
		CommitteeIndex:  v1AttData.Index,
		BeaconBlockRoot: v1AttData.BeaconBlockRoot,
		Source: &zondpbalpha.Checkpoint{
			Root:  v1AttData.Source.Root,
			Epoch: v1AttData.Source.Epoch,
		},
		Target: &zondpbalpha.Checkpoint{
			Root:  v1AttData.Target.Root,
			Epoch: v1AttData.Target.Epoch,
		},
	}
}

// V1AttSlashingToV1Alpha1 converts a v1 attester slashing to v1alpha1.
func V1AttSlashingToV1Alpha1(v1Slashing *zondpbv1.AttesterSlashing) *zondpbalpha.AttesterSlashing {
	if v1Slashing == nil {
		return &zondpbalpha.AttesterSlashing{}
	}
	return &zondpbalpha.AttesterSlashing{
		Attestation_1: V1IndexedAttToV1Alpha1(v1Slashing.Attestation_1),
		Attestation_2: V1IndexedAttToV1Alpha1(v1Slashing.Attestation_2),
	}
}

// V1ProposerSlashingToV1Alpha1 converts a v1 proposer slashing to v1alpha1.
func V1ProposerSlashingToV1Alpha1(v1Slashing *zondpbv1.ProposerSlashing) *zondpbalpha.ProposerSlashing {
	if v1Slashing == nil {
		return &zondpbalpha.ProposerSlashing{}
	}
	return &zondpbalpha.ProposerSlashing{
		Header_1: V1SignedHeaderToV1Alpha1(v1Slashing.SignedHeader_1),
		Header_2: V1SignedHeaderToV1Alpha1(v1Slashing.SignedHeader_2),
	}
}

// V1Alpha1ValidatorToV1 converts a v1alpha1 validator to v1.
func V1Alpha1ValidatorToV1(v1Alpha1Validator *zondpbalpha.Validator) *zondpbv1.Validator {
	if v1Alpha1Validator == nil {
		return &zondpbv1.Validator{}
	}
	return &zondpbv1.Validator{
		Pubkey:                     v1Alpha1Validator.PublicKey,
		WithdrawalCredentials:      v1Alpha1Validator.WithdrawalCredentials,
		EffectiveBalance:           v1Alpha1Validator.EffectiveBalance,
		Slashed:                    v1Alpha1Validator.Slashed,
		ActivationEligibilityEpoch: v1Alpha1Validator.ActivationEligibilityEpoch,
		ActivationEpoch:            v1Alpha1Validator.ActivationEpoch,
		ExitEpoch:                  v1Alpha1Validator.ExitEpoch,
		WithdrawableEpoch:          v1Alpha1Validator.WithdrawableEpoch,
	}
}

// V1ValidatorToV1Alpha1 converts a v1 validator to v1alpha1.
func V1ValidatorToV1Alpha1(v1Validator *zondpbv1.Validator) *zondpbalpha.Validator {
	if v1Validator == nil {
		return &zondpbalpha.Validator{}
	}
	return &zondpbalpha.Validator{
		PublicKey:                  v1Validator.Pubkey,
		WithdrawalCredentials:      v1Validator.WithdrawalCredentials,
		EffectiveBalance:           v1Validator.EffectiveBalance,
		Slashed:                    v1Validator.Slashed,
		ActivationEligibilityEpoch: v1Validator.ActivationEligibilityEpoch,
		ActivationEpoch:            v1Validator.ActivationEpoch,
		ExitEpoch:                  v1Validator.ExitEpoch,
		WithdrawableEpoch:          v1Validator.WithdrawableEpoch,
	}
}
