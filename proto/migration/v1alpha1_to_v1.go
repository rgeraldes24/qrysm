package migration

import (
	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/encoding/ssz"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpbalpha "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	"google.golang.org/protobuf/proto"
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

// V1Alpha1ToV1SignedBlock converts a v1alpha1 SignedBeaconBlock proto to a v1 proto.
func V1Alpha1ToV1SignedBlock(alphaBlk *zondpbalpha.SignedBeaconBlock) (*zondpbv1.SignedBeaconBlock, error) {
	marshaledBlk, err := proto.Marshal(alphaBlk)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1Block := &zondpbv1.SignedBeaconBlock{}
	if err := proto.Unmarshal(marshaledBlk, v1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1Block, nil
}

// V1ToV1Alpha1SignedBlock converts a v1 SignedBeaconBlock proto to a v1alpha1 proto.
func V1ToV1Alpha1SignedBlock(v1Blk *zondpbv1.SignedBeaconBlock) (*zondpbalpha.SignedBeaconBlock, error) {
	marshaledBlk, err := proto.Marshal(v1Blk)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1alpha1Block := &zondpbalpha.SignedBeaconBlock{}
	if err := proto.Unmarshal(marshaledBlk, v1alpha1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1alpha1Block, nil
}

// V1ToV1Alpha1SignedBlindedBlock converts a v1 SignedBlindedBeaconBlock proto to a v1alpha1 proto.
func V1ToV1Alpha1SignedBlindedBlock(v1Blk *zondpbv1.SignedBlindedBeaconBlock) (*zondpbalpha.SignedBlindedBeaconBlock, error) {
	marshaledBlk, err := proto.Marshal(v1Blk)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1alpha1Block := &zondpbalpha.SignedBlindedBeaconBlock{}
	if err := proto.Unmarshal(marshaledBlk, v1alpha1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1alpha1Block, nil
}

// V1Alpha1ToV1Block converts a v1alpha1 ReadOnlyBeaconBlock proto to a v1 proto.
func V1Alpha1ToV1Block(alphaBlk *zondpbalpha.BeaconBlock) (*zondpbv1.BeaconBlock, error) {
	marshaledBlk, err := proto.Marshal(alphaBlk)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1Block := &zondpbv1.BeaconBlock{}
	if err := proto.Unmarshal(marshaledBlk, v1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1Block, nil
}

func V1Alpha1ToV1BlindedBlock(alphaBlk *zondpbalpha.BlindedBeaconBlock) (*zondpbv1.BlindedBeaconBlock, error) {
	marshaledBlk, err := proto.Marshal(alphaBlk)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1Block := &zondpbv1.BlindedBeaconBlock{}
	if err := proto.Unmarshal(marshaledBlk, v1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	return v1Block, nil
}

// V1Alpha1ToV1AggregateAttAndProof converts a v1alpha1 aggregate attestation and proof to v1.
func V1Alpha1ToV1AggregateAttAndProof(v1alpha1Att *zondpbalpha.AggregateAttestationAndProof) *zondpbv1.AggregateAttestationAndProof {
	if v1alpha1Att == nil {
		return &zondpbv1.AggregateAttestationAndProof{}
	}
	return &zondpbv1.AggregateAttestationAndProof{
		AggregatorIndex: v1alpha1Att.AggregatorIndex,
		Aggregate:       V1Alpha1ToV1Attestation(v1alpha1Att.Aggregate),
		SelectionProof:  v1alpha1Att.SelectionProof,
	}
}

// V1ToV1Alpha1SignedAggregateAttAndProof converts a v1 signed aggregate attestation and proof to v1alpha1.
func V1ToV1Alpha1SignedAggregateAttAndProof(v1Att *zondpbv1.SignedAggregateAttestationAndProof) *zondpbalpha.SignedAggregateAttestationAndProof {
	if v1Att == nil {
		return &zondpbalpha.SignedAggregateAttestationAndProof{}
	}
	return &zondpbalpha.SignedAggregateAttestationAndProof{
		Message: &zondpbalpha.AggregateAttestationAndProof{
			AggregatorIndex: v1Att.Message.AggregatorIndex,
			Aggregate:       V1ToV1Alpha1Attestation(v1Att.Message.Aggregate),
			SelectionProof:  v1Att.Message.SelectionProof,
		},
		Signature: v1Att.Signature,
	}
}

// V1Alpha1ToV1IndexedAtt converts a v1alpha1 indexed attestation to v1.
func V1Alpha1ToV1IndexedAtt(v1alpha1Att *zondpbalpha.IndexedAttestation) *zondpbv1.IndexedAttestation {
	if v1alpha1Att == nil {
		return &zondpbv1.IndexedAttestation{}
	}
	return &zondpbv1.IndexedAttestation{
		AttestingIndices: v1alpha1Att.AttestingIndices,
		Data:             V1Alpha1ToV1AttData(v1alpha1Att.Data),
		Signatures:       v1alpha1Att.Signatures,
	}
}

// V1Alpha1ToV1Attestation converts a v1alpha1 attestation to v1.
func V1Alpha1ToV1Attestation(v1alpha1Att *zondpbalpha.Attestation) *zondpbv1.Attestation {
	if v1alpha1Att == nil {
		return &zondpbv1.Attestation{}
	}
	return &zondpbv1.Attestation{
		ParticipationBits:               v1alpha1Att.ParticipationBits,
		Data:                            V1Alpha1ToV1AttData(v1alpha1Att.Data),
		Signatures:                      v1alpha1Att.Signatures,
		SignaturesIdxToParticipationIdx: v1alpha1Att.SignaturesIdxToParticipationIdx,
	}
}

// V1ToV1Alpha1Attestation converts a v1 attestation to v1alpha1.
func V1ToV1Alpha1Attestation(v1Att *zondpbv1.Attestation) *zondpbalpha.Attestation {
	if v1Att == nil {
		return &zondpbalpha.Attestation{}
	}
	return &zondpbalpha.Attestation{
		ParticipationBits:               v1Att.ParticipationBits,
		Data:                            V1ToV1Alpha1AttData(v1Att.Data),
		Signatures:                      v1Att.Signatures,
		SignaturesIdxToParticipationIdx: v1Att.SignaturesIdxToParticipationIdx,
	}
}

// V1Alpha1ToV1AttData converts a v1alpha1 attestation data to v1.
func V1Alpha1ToV1AttData(v1alpha1AttData *zondpbalpha.AttestationData) *zondpbv1.AttestationData {
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

// V1Alpha1ToV1AttSlashing converts a v1alpha1 attester slashing to v1.
func V1Alpha1ToV1AttSlashing(v1alpha1Slashing *zondpbalpha.AttesterSlashing) *zondpbv1.AttesterSlashing {
	if v1alpha1Slashing == nil {
		return &zondpbv1.AttesterSlashing{}
	}
	return &zondpbv1.AttesterSlashing{
		Attestation_1: V1Alpha1ToV1IndexedAtt(v1alpha1Slashing.Attestation_1),
		Attestation_2: V1Alpha1ToV1IndexedAtt(v1alpha1Slashing.Attestation_2),
	}
}

// V1Alpha1ToV1SignedHeader converts a v1alpha1 signed beacon block header to v1.
func V1Alpha1ToV1SignedHeader(v1alpha1Hdr *zondpbalpha.SignedBeaconBlockHeader) *zondpbv1.SignedBeaconBlockHeader {
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

// V1ToV1Alpha1SignedHeader converts a v1 signed beacon block header to v1alpha1.
func V1ToV1Alpha1SignedHeader(v1Header *zondpbv1.SignedBeaconBlockHeader) *zondpbalpha.SignedBeaconBlockHeader {
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

// V1Alpha1ToV1ProposerSlashing converts a v1alpha1 proposer slashing to v1.
func V1Alpha1ToV1ProposerSlashing(v1alpha1Slashing *zondpbalpha.ProposerSlashing) *zondpbv1.ProposerSlashing {
	if v1alpha1Slashing == nil {
		return &zondpbv1.ProposerSlashing{}
	}
	return &zondpbv1.ProposerSlashing{
		SignedHeader_1: V1Alpha1ToV1SignedHeader(v1alpha1Slashing.Header_1),
		SignedHeader_2: V1Alpha1ToV1SignedHeader(v1alpha1Slashing.Header_2),
	}
}

// V1Alpha1ToV1Exit converts a v1alpha1 SignedVoluntaryExit to v1.
func V1Alpha1ToV1Exit(v1alpha1Exit *zondpbalpha.SignedVoluntaryExit) *zondpbv1.SignedVoluntaryExit {
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

// V1ToV1Alpha1Exit converts a v1 SignedVoluntaryExit to v1alpha1.
func V1ToV1Alpha1Exit(v1Exit *zondpbv1.SignedVoluntaryExit) *zondpbalpha.SignedVoluntaryExit {
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

// V1ToV1Alpha1IndexedAtt converts a v1 indexed attestation to v1alpha1.
func V1ToV1Alpha1IndexedAtt(v1Att *zondpbv1.IndexedAttestation) *zondpbalpha.IndexedAttestation {
	if v1Att == nil {
		return &zondpbalpha.IndexedAttestation{}
	}
	return &zondpbalpha.IndexedAttestation{
		AttestingIndices: v1Att.AttestingIndices,
		Data:             V1ToV1Alpha1AttData(v1Att.Data),
		Signatures:       v1Att.Signatures,
	}
}

// V1ToV1Alpha1AttData converts a v1 attestation data to v1alpha1.
func V1ToV1Alpha1AttData(v1AttData *zondpbv1.AttestationData) *zondpbalpha.AttestationData {
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

// V1ToV1Alpha1AttSlashing converts a v1 attester slashing to v1alpha1.
func V1ToV1Alpha1AttSlashing(v1Slashing *zondpbv1.AttesterSlashing) *zondpbalpha.AttesterSlashing {
	if v1Slashing == nil {
		return &zondpbalpha.AttesterSlashing{}
	}
	return &zondpbalpha.AttesterSlashing{
		Attestation_1: V1ToV1Alpha1IndexedAtt(v1Slashing.Attestation_1),
		Attestation_2: V1ToV1Alpha1IndexedAtt(v1Slashing.Attestation_2),
	}
}

// V1ToV1Alpha1ProposerSlashing converts a v1 proposer slashing to v1alpha1.
func V1ToV1Alpha1ProposerSlashing(v1Slashing *zondpbv1.ProposerSlashing) *zondpbalpha.ProposerSlashing {
	if v1Slashing == nil {
		return &zondpbalpha.ProposerSlashing{}
	}
	return &zondpbalpha.ProposerSlashing{
		Header_1: V1ToV1Alpha1SignedHeader(v1Slashing.SignedHeader_1),
		Header_2: V1ToV1Alpha1SignedHeader(v1Slashing.SignedHeader_2),
	}
}

// V1Alpha1ToV1Validator converts a v1alpha1 validator to v1.
func V1Alpha1ToV1Validator(v1Alpha1Validator *zondpbalpha.Validator) *zondpbv1.Validator {
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

// V1ToV1Alpha1Validator converts a v1 validator to v1alpha1.
func V1ToV1Alpha1Validator(v1Validator *zondpbv1.Validator) *zondpbalpha.Validator {
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

// SignedBeaconBlock converts a signed beacon block interface to a v1alpha1 block.
func SignedBeaconBlock(block interfaces.ReadOnlySignedBeaconBlock) (*zondpbv1.SignedBeaconBlock, error) {
	if block == nil || block.IsNil() {
		return nil, errors.New("could not find requested block")
	}
	blk, err := block.PbCapellaBlock()
	if err != nil {
		return nil, errors.Wrapf(err, "could not get raw block")
	}

	v1Block, err := V1Alpha1ToV1SignedBlock(blk)
	if err != nil {
		return nil, errors.New("could not convert block to v1 block")
	}

	return v1Block, nil
}

// BeaconStateToProto converts a state.BeaconState object to its protobuf equivalent.
func BeaconStateToProto(st state.BeaconState) (*zondpbv1.BeaconState, error) {
	sourceFork := st.Fork()
	sourceLatestBlockHeader := st.LatestBlockHeader()
	sourceZond1Data := st.Zond1Data()
	sourceZond1DataVotes := st.Zond1DataVotes()
	sourceValidators := st.Validators()
	sourceJustificationBits := st.JustificationBits()
	sourcePrevJustifiedCheckpoint := st.PreviousJustifiedCheckpoint()
	sourceCurrJustifiedCheckpoint := st.CurrentJustifiedCheckpoint()
	sourceFinalizedCheckpoint := st.FinalizedCheckpoint()

	resultZond1DataVotes := make([]*zondpbv1.Zond1Data, len(sourceZond1DataVotes))
	for i, vote := range sourceZond1DataVotes {
		resultZond1DataVotes[i] = &zondpbv1.Zond1Data{
			DepositRoot:  bytesutil.SafeCopyBytes(vote.DepositRoot),
			DepositCount: vote.DepositCount,
			BlockHash:    bytesutil.SafeCopyBytes(vote.BlockHash),
		}
	}
	resultValidators := make([]*zondpbv1.Validator, len(sourceValidators))
	for i, validator := range sourceValidators {
		resultValidators[i] = &zondpbv1.Validator{
			Pubkey:                     bytesutil.SafeCopyBytes(validator.PublicKey),
			WithdrawalCredentials:      bytesutil.SafeCopyBytes(validator.WithdrawalCredentials),
			EffectiveBalance:           validator.EffectiveBalance,
			Slashed:                    validator.Slashed,
			ActivationEligibilityEpoch: validator.ActivationEligibilityEpoch,
			ActivationEpoch:            validator.ActivationEpoch,
			ExitEpoch:                  validator.ExitEpoch,
			WithdrawableEpoch:          validator.WithdrawableEpoch,
		}
	}

	sourcePrevEpochParticipation, err := st.PreviousEpochParticipation()
	if err != nil {
		return nil, errors.Wrap(err, "could not get previous epoch participation")
	}
	sourceCurrEpochParticipation, err := st.CurrentEpochParticipation()
	if err != nil {
		return nil, errors.Wrap(err, "could not get current epoch participation")
	}
	sourceInactivityScores, err := st.InactivityScores()
	if err != nil {
		return nil, errors.Wrap(err, "could not get inactivity scores")
	}
	sourceCurrSyncCommittee, err := st.CurrentSyncCommittee()
	if err != nil {
		return nil, errors.Wrap(err, "could not get current sync committee")
	}
	sourceNextSyncCommittee, err := st.NextSyncCommittee()
	if err != nil {
		return nil, errors.Wrap(err, "could not get next sync committee")
	}
	executionPayloadHeaderInterface, err := st.LatestExecutionPayloadHeader()
	if err != nil {
		return nil, errors.Wrap(err, "could not get latest execution payload header")
	}
	sourceLatestExecutionPayloadHeader, ok := executionPayloadHeaderInterface.Proto().(*enginev1.ExecutionPayloadHeader)
	if !ok {
		return nil, errors.New("execution payload header has incorrect type")
	}
	sourceNextWithdrawalIndex, err := st.NextWithdrawalIndex()
	if err != nil {
		return nil, errors.Wrap(err, "could not get next withdrawal index")
	}
	sourceNextWithdrawalValIndex, err := st.NextWithdrawalValidatorIndex()
	if err != nil {
		return nil, errors.Wrap(err, "could not get next withdrawal validator index")
	}
	summaries, err := st.HistoricalSummaries()
	if err != nil {
		return nil, errors.Wrap(err, "could not get historical summaries")
	}
	sourceHistoricalSummaries := make([]*zondpbv1.HistoricalSummary, len(summaries))
	for i, summary := range summaries {
		sourceHistoricalSummaries[i] = &zondpbv1.HistoricalSummary{
			BlockSummaryRoot: summary.BlockSummaryRoot,
			StateSummaryRoot: summary.StateSummaryRoot,
		}
	}
	hRoots, err := st.HistoricalRoots()
	if err != nil {
		return nil, errors.Wrap(err, "could not get historical roots")
	}

	result := &zondpbv1.BeaconState{
		GenesisTime:           st.GenesisTime(),
		GenesisValidatorsRoot: bytesutil.SafeCopyBytes(st.GenesisValidatorsRoot()),
		Slot:                  st.Slot(),
		Fork: &zondpbv1.Fork{
			PreviousVersion: bytesutil.SafeCopyBytes(sourceFork.PreviousVersion),
			CurrentVersion:  bytesutil.SafeCopyBytes(sourceFork.CurrentVersion),
			Epoch:           sourceFork.Epoch,
		},
		LatestBlockHeader: &zondpbv1.BeaconBlockHeader{
			Slot:          sourceLatestBlockHeader.Slot,
			ProposerIndex: sourceLatestBlockHeader.ProposerIndex,
			ParentRoot:    bytesutil.SafeCopyBytes(sourceLatestBlockHeader.ParentRoot),
			StateRoot:     bytesutil.SafeCopyBytes(sourceLatestBlockHeader.StateRoot),
			BodyRoot:      bytesutil.SafeCopyBytes(sourceLatestBlockHeader.BodyRoot),
		},
		BlockRoots: bytesutil.SafeCopy2dBytes(st.BlockRoots()),
		StateRoots: bytesutil.SafeCopy2dBytes(st.StateRoots()),
		Zond1Data: &zondpbv1.Zond1Data{
			DepositRoot:  bytesutil.SafeCopyBytes(sourceZond1Data.DepositRoot),
			DepositCount: sourceZond1Data.DepositCount,
			BlockHash:    bytesutil.SafeCopyBytes(sourceZond1Data.BlockHash),
		},
		Zond1DataVotes:             resultZond1DataVotes,
		Zond1DepositIndex:          st.Zond1DepositIndex(),
		Validators:                 resultValidators,
		Balances:                   st.Balances(),
		RandaoMixes:                bytesutil.SafeCopy2dBytes(st.RandaoMixes()),
		Slashings:                  st.Slashings(),
		PreviousEpochParticipation: bytesutil.SafeCopyBytes(sourcePrevEpochParticipation),
		CurrentEpochParticipation:  bytesutil.SafeCopyBytes(sourceCurrEpochParticipation),
		JustificationBits:          bytesutil.SafeCopyBytes(sourceJustificationBits),
		PreviousJustifiedCheckpoint: &zondpbv1.Checkpoint{
			Epoch: sourcePrevJustifiedCheckpoint.Epoch,
			Root:  bytesutil.SafeCopyBytes(sourcePrevJustifiedCheckpoint.Root),
		},
		CurrentJustifiedCheckpoint: &zondpbv1.Checkpoint{
			Epoch: sourceCurrJustifiedCheckpoint.Epoch,
			Root:  bytesutil.SafeCopyBytes(sourceCurrJustifiedCheckpoint.Root),
		},
		FinalizedCheckpoint: &zondpbv1.Checkpoint{
			Epoch: sourceFinalizedCheckpoint.Epoch,
			Root:  bytesutil.SafeCopyBytes(sourceFinalizedCheckpoint.Root),
		},
		InactivityScores: sourceInactivityScores,
		CurrentSyncCommittee: &zondpbv1.SyncCommittee{
			Pubkeys: bytesutil.SafeCopy2dBytes(sourceCurrSyncCommittee.Pubkeys),
		},
		NextSyncCommittee: &zondpbv1.SyncCommittee{
			Pubkeys: bytesutil.SafeCopy2dBytes(sourceNextSyncCommittee.Pubkeys),
		},
		LatestExecutionPayloadHeader: &enginev1.ExecutionPayloadHeader{
			ParentHash:       bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.ParentHash),
			FeeRecipient:     bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.FeeRecipient),
			StateRoot:        bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.StateRoot),
			ReceiptsRoot:     bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.ReceiptsRoot),
			LogsBloom:        bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.LogsBloom),
			PrevRandao:       bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.PrevRandao),
			BlockNumber:      sourceLatestExecutionPayloadHeader.BlockNumber,
			GasLimit:         sourceLatestExecutionPayloadHeader.GasLimit,
			GasUsed:          sourceLatestExecutionPayloadHeader.GasUsed,
			Timestamp:        sourceLatestExecutionPayloadHeader.Timestamp,
			ExtraData:        bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.ExtraData),
			BaseFeePerGas:    bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.BaseFeePerGas),
			BlockHash:        bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.BlockHash),
			TransactionsRoot: bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.TransactionsRoot),
			WithdrawalsRoot:  bytesutil.SafeCopyBytes(sourceLatestExecutionPayloadHeader.WithdrawalsRoot),
		},
		NextWithdrawalIndex:          sourceNextWithdrawalIndex,
		NextWithdrawalValidatorIndex: sourceNextWithdrawalValIndex,
		HistoricalSummaries:          sourceHistoricalSummaries,
		HistoricalRoots:              hRoots,
	}

	return result, nil
}

// V1Alpha1ToV1SignedDilithiumToExecChange converts a v1alpha1 SignedDilithiumToExecutionChange object to its v1 equivalent.
func V1Alpha1ToV1SignedDilithiumToExecChange(alphaChange *zondpbalpha.SignedDilithiumToExecutionChange) *zondpbv1.SignedDilithiumToExecutionChange {
	result := &zondpbv1.SignedDilithiumToExecutionChange{
		Message: &zondpbv1.DilithiumToExecutionChange{
			ValidatorIndex:      alphaChange.Message.ValidatorIndex,
			FromDilithiumPubkey: bytesutil.SafeCopyBytes(alphaChange.Message.FromDilithiumPubkey),
			ToExecutionAddress:  bytesutil.SafeCopyBytes(alphaChange.Message.ToExecutionAddress),
		},
		Signature: bytesutil.SafeCopyBytes(alphaChange.Signature),
	}
	return result
}

// V1Alpha1ToV1SignedContributionAndProof converts a v1alpha1 SignedContributionAndProof object to its v1 equivalent.
func V1Alpha1ToV1SignedContributionAndProof(alphaContribution *zondpbalpha.SignedContributionAndProof) *zondpbv1.SignedContributionAndProof {
	result := &zondpbv1.SignedContributionAndProof{
		Message: &zondpbv1.ContributionAndProof{
			AggregatorIndex: alphaContribution.Message.AggregatorIndex,
			Contribution: &zondpbv1.SyncCommitteeContribution{
				Slot:              alphaContribution.Message.Contribution.Slot,
				BeaconBlockRoot:   alphaContribution.Message.Contribution.BlockRoot,
				SubcommitteeIndex: alphaContribution.Message.Contribution.SubcommitteeIndex,
				ParticipationBits: alphaContribution.Message.Contribution.ParticipationBits,
				Signatures:        alphaContribution.Message.Contribution.Signatures,
			},
			SelectionProof: alphaContribution.Message.SelectionProof,
		},
		Signature: alphaContribution.Signature,
	}
	return result
}

func V1ToV1Alpha1SignedDilithiumToExecutionChange(change *zondpbv1.SignedDilithiumToExecutionChange) *zondpbalpha.SignedDilithiumToExecutionChange {
	return &zondpbalpha.SignedDilithiumToExecutionChange{
		Message: &zondpbalpha.DilithiumToExecutionChange{
			ValidatorIndex:      change.Message.ValidatorIndex,
			FromDilithiumPubkey: bytesutil.SafeCopyBytes(change.Message.FromDilithiumPubkey),
			ToExecutionAddress:  bytesutil.SafeCopyBytes(change.Message.ToExecutionAddress),
		},
		Signature: bytesutil.SafeCopyBytes(change.Signature),
	}
}

func V1Alpha1BeaconBlockToV1Blinded(v1alpha1Block *zondpbalpha.BeaconBlock) (*zondpbv1.BlindedBeaconBlock, error) {
	sourceProposerSlashings := v1alpha1Block.Body.ProposerSlashings
	resultProposerSlashings := make([]*zondpbv1.ProposerSlashing, len(sourceProposerSlashings))
	for i, s := range sourceProposerSlashings {
		resultProposerSlashings[i] = &zondpbv1.ProposerSlashing{
			SignedHeader_1: &zondpbv1.SignedBeaconBlockHeader{
				Message: &zondpbv1.BeaconBlockHeader{
					Slot:          s.Header_1.Header.Slot,
					ProposerIndex: s.Header_1.Header.ProposerIndex,
					ParentRoot:    bytesutil.SafeCopyBytes(s.Header_1.Header.ParentRoot),
					StateRoot:     bytesutil.SafeCopyBytes(s.Header_1.Header.StateRoot),
					BodyRoot:      bytesutil.SafeCopyBytes(s.Header_1.Header.BodyRoot),
				},
				Signature: bytesutil.SafeCopyBytes(s.Header_1.Signature),
			},
			SignedHeader_2: &zondpbv1.SignedBeaconBlockHeader{
				Message: &zondpbv1.BeaconBlockHeader{
					Slot:          s.Header_2.Header.Slot,
					ProposerIndex: s.Header_2.Header.ProposerIndex,
					ParentRoot:    bytesutil.SafeCopyBytes(s.Header_2.Header.ParentRoot),
					StateRoot:     bytesutil.SafeCopyBytes(s.Header_2.Header.StateRoot),
					BodyRoot:      bytesutil.SafeCopyBytes(s.Header_2.Header.BodyRoot),
				},
				Signature: bytesutil.SafeCopyBytes(s.Header_2.Signature),
			},
		}
	}

	sourceAttesterSlashings := v1alpha1Block.Body.AttesterSlashings
	resultAttesterSlashings := make([]*zondpbv1.AttesterSlashing, len(sourceAttesterSlashings))
	for i, s := range sourceAttesterSlashings {
		att1Indices := make([]uint64, len(s.Attestation_1.AttestingIndices))
		copy(att1Indices, s.Attestation_1.AttestingIndices)
		att2Indices := make([]uint64, len(s.Attestation_2.AttestingIndices))
		copy(att2Indices, s.Attestation_2.AttestingIndices)
		signatures1 := make([][]byte, len(s.Attestation_1.Signatures))
		for i, sig := range s.Attestation_1.Signatures {
			signatures1[i] = bytesutil.SafeCopyBytes(sig)
		}
		signatures2 := make([][]byte, len(s.Attestation_2.Signatures))
		for i, sig := range s.Attestation_2.Signatures {
			signatures2[i] = bytesutil.SafeCopyBytes(sig)
		}
		resultAttesterSlashings[i] = &zondpbv1.AttesterSlashing{
			Attestation_1: &zondpbv1.IndexedAttestation{
				AttestingIndices: att1Indices,
				Data: &zondpbv1.AttestationData{
					Slot:            s.Attestation_1.Data.Slot,
					Index:           s.Attestation_1.Data.CommitteeIndex,
					BeaconBlockRoot: bytesutil.SafeCopyBytes(s.Attestation_1.Data.BeaconBlockRoot),
					Source: &zondpbv1.Checkpoint{
						Epoch: s.Attestation_1.Data.Source.Epoch,
						Root:  bytesutil.SafeCopyBytes(s.Attestation_1.Data.Source.Root),
					},
					Target: &zondpbv1.Checkpoint{
						Epoch: s.Attestation_1.Data.Target.Epoch,
						Root:  bytesutil.SafeCopyBytes(s.Attestation_1.Data.Target.Root),
					},
				},
				Signatures: signatures1,
			},
			Attestation_2: &zondpbv1.IndexedAttestation{
				AttestingIndices: att2Indices,
				Data: &zondpbv1.AttestationData{
					Slot:            s.Attestation_2.Data.Slot,
					Index:           s.Attestation_2.Data.CommitteeIndex,
					BeaconBlockRoot: bytesutil.SafeCopyBytes(s.Attestation_2.Data.BeaconBlockRoot),
					Source: &zondpbv1.Checkpoint{
						Epoch: s.Attestation_2.Data.Source.Epoch,
						Root:  bytesutil.SafeCopyBytes(s.Attestation_2.Data.Source.Root),
					},
					Target: &zondpbv1.Checkpoint{
						Epoch: s.Attestation_2.Data.Target.Epoch,
						Root:  bytesutil.SafeCopyBytes(s.Attestation_2.Data.Target.Root),
					},
				},
				Signatures: signatures2,
			},
		}
	}

	sourceAttestations := v1alpha1Block.Body.Attestations
	resultAttestations := make([]*zondpbv1.Attestation, len(sourceAttestations))
	for i, a := range sourceAttestations {
		signatures := make([][]byte, len(a.Signatures))
		for i, sig := range a.Signatures {
			signatures[i] = bytesutil.SafeCopyBytes(sig)
		}

		resultAttestations[i] = &zondpbv1.Attestation{
			ParticipationBits: bytesutil.SafeCopyBytes(a.ParticipationBits),
			Data: &zondpbv1.AttestationData{
				Slot:            a.Data.Slot,
				Index:           a.Data.CommitteeIndex,
				BeaconBlockRoot: bytesutil.SafeCopyBytes(a.Data.BeaconBlockRoot),
				Source: &zondpbv1.Checkpoint{
					Epoch: a.Data.Source.Epoch,
					Root:  bytesutil.SafeCopyBytes(a.Data.Source.Root),
				},
				Target: &zondpbv1.Checkpoint{
					Epoch: a.Data.Target.Epoch,
					Root:  bytesutil.SafeCopyBytes(a.Data.Target.Root),
				},
			},
			Signatures: signatures,
		}
	}

	sourceDeposits := v1alpha1Block.Body.Deposits
	resultDeposits := make([]*zondpbv1.Deposit, len(sourceDeposits))
	for i, d := range sourceDeposits {
		resultDeposits[i] = &zondpbv1.Deposit{
			Proof: bytesutil.SafeCopy2dBytes(d.Proof),
			Data: &zondpbv1.Deposit_Data{
				Pubkey:                bytesutil.SafeCopyBytes(d.Data.PublicKey),
				WithdrawalCredentials: bytesutil.SafeCopyBytes(d.Data.WithdrawalCredentials),
				Amount:                d.Data.Amount,
				Signature:             bytesutil.SafeCopyBytes(d.Data.Signature),
			},
		}
	}

	sourceExits := v1alpha1Block.Body.VoluntaryExits
	resultExits := make([]*zondpbv1.SignedVoluntaryExit, len(sourceExits))
	for i, e := range sourceExits {
		resultExits[i] = &zondpbv1.SignedVoluntaryExit{
			Message: &zondpbv1.VoluntaryExit{
				Epoch:          e.Exit.Epoch,
				ValidatorIndex: e.Exit.ValidatorIndex,
			},
			Signature: bytesutil.SafeCopyBytes(e.Signature),
		}
	}

	transactionsRoot, err := ssz.TransactionsRoot(v1alpha1Block.Body.ExecutionPayload.Transactions)
	if err != nil {
		return nil, errors.Wrapf(err, "could not calculate transactions root")
	}

	withdrawalsRoot, err := ssz.WithdrawalSliceRoot(v1alpha1Block.Body.ExecutionPayload.Withdrawals, fieldparams.MaxWithdrawalsPerPayload)
	if err != nil {
		return nil, errors.Wrapf(err, "could not calculate transactions root")
	}

	changes := make([]*zondpbv1.SignedDilithiumToExecutionChange, len(v1alpha1Block.Body.DilithiumToExecutionChanges))
	for i, change := range v1alpha1Block.Body.DilithiumToExecutionChanges {
		changes[i] = &zondpbv1.SignedDilithiumToExecutionChange{
			Message: &zondpbv1.DilithiumToExecutionChange{
				ValidatorIndex:      change.Message.ValidatorIndex,
				FromDilithiumPubkey: bytesutil.SafeCopyBytes(change.Message.FromDilithiumPubkey),
				ToExecutionAddress:  bytesutil.SafeCopyBytes(change.Message.ToExecutionAddress),
			},
			Signature: bytesutil.SafeCopyBytes(change.Signature),
		}
	}

	syncSigs := make([][]byte, len(v1alpha1Block.Body.SyncAggregate.SyncCommitteeSignatures))
	for i, sig := range v1alpha1Block.Body.SyncAggregate.SyncCommitteeSignatures {
		syncSigs[i] = bytesutil.SafeCopyBytes(sig)
	}

	resultBlockBody := &zondpbv1.BlindedBeaconBlockBody{
		RandaoReveal: bytesutil.SafeCopyBytes(v1alpha1Block.Body.RandaoReveal),
		Zond1Data: &zondpbv1.Zond1Data{
			DepositRoot:  bytesutil.SafeCopyBytes(v1alpha1Block.Body.Zond1Data.DepositRoot),
			DepositCount: v1alpha1Block.Body.Zond1Data.DepositCount,
			BlockHash:    bytesutil.SafeCopyBytes(v1alpha1Block.Body.Zond1Data.BlockHash),
		},
		Graffiti:          bytesutil.SafeCopyBytes(v1alpha1Block.Body.Graffiti),
		ProposerSlashings: resultProposerSlashings,
		AttesterSlashings: resultAttesterSlashings,
		Attestations:      resultAttestations,
		Deposits:          resultDeposits,
		VoluntaryExits:    resultExits,
		SyncAggregate: &zondpbv1.SyncAggregate{
			SyncCommitteeBits:       bytesutil.SafeCopyBytes(v1alpha1Block.Body.SyncAggregate.SyncCommitteeBits),
			SyncCommitteeSignatures: syncSigs,
		},
		ExecutionPayloadHeader: &enginev1.ExecutionPayloadHeader{
			ParentHash:       bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.ParentHash),
			FeeRecipient:     bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.FeeRecipient),
			StateRoot:        bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.StateRoot),
			ReceiptsRoot:     bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.ReceiptsRoot),
			LogsBloom:        bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.LogsBloom),
			PrevRandao:       bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.PrevRandao),
			BlockNumber:      v1alpha1Block.Body.ExecutionPayload.BlockNumber,
			GasLimit:         v1alpha1Block.Body.ExecutionPayload.GasLimit,
			GasUsed:          v1alpha1Block.Body.ExecutionPayload.GasUsed,
			Timestamp:        v1alpha1Block.Body.ExecutionPayload.Timestamp,
			ExtraData:        bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.ExtraData),
			BaseFeePerGas:    bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.BaseFeePerGas),
			BlockHash:        bytesutil.SafeCopyBytes(v1alpha1Block.Body.ExecutionPayload.BlockHash),
			TransactionsRoot: transactionsRoot[:],
			WithdrawalsRoot:  withdrawalsRoot[:],
		},
		DilithiumToExecutionChanges: changes,
	}
	v1Block := &zondpbv1.BlindedBeaconBlock{
		Slot:          v1alpha1Block.Slot,
		ProposerIndex: v1alpha1Block.ProposerIndex,
		ParentRoot:    bytesutil.SafeCopyBytes(v1alpha1Block.ParentRoot),
		StateRoot:     bytesutil.SafeCopyBytes(v1alpha1Block.StateRoot),
		Body:          resultBlockBody,
	}
	return v1Block, nil
}
