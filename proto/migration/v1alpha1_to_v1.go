package migration

import (
	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	qrlpb "github.com/theQRL/qrysm/proto/qrl/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"google.golang.org/protobuf/proto"
)

// BlockIfaceToV1BlockHeader converts a signed beacon block interface into a signed beacon block header.
func BlockIfaceToV1BlockHeader(block interfaces.ReadOnlySignedBeaconBlock) (*qrlpb.SignedBeaconBlockHeader, error) {
	bodyRoot, err := block.Block().Body().HashTreeRoot()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get body root of block")
	}
	parentRoot := block.Block().ParentRoot()
	stateRoot := block.Block().StateRoot()
	sig := block.Signature()
	return &qrlpb.SignedBeaconBlockHeader{
		Message: &qrlpb.BeaconBlockHeader{
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
func V1Alpha1AggregateAttAndProofToV1(v1alpha1Att *qrysmpb.AggregateAttestationAndProof) *qrlpb.AggregateAttestationAndProof {
	if v1alpha1Att == nil {
		return &qrlpb.AggregateAttestationAndProof{}
	}
	return &qrlpb.AggregateAttestationAndProof{
		AggregatorIndex: v1alpha1Att.AggregatorIndex,
		Aggregate:       V1Alpha1AttestationToV1(v1alpha1Att.Aggregate),
		SelectionProof:  v1alpha1Att.SelectionProof,
	}
}

// V1Alpha1IndexedAttToV1 converts a v1alpha1 indexed attestation to v1.
func V1Alpha1IndexedAttToV1(v1alpha1Att *qrysmpb.IndexedAttestation) *qrlpb.IndexedAttestation {
	if v1alpha1Att == nil {
		return &qrlpb.IndexedAttestation{}
	}
	return &qrlpb.IndexedAttestation{
		AttestingIndices: v1alpha1Att.AttestingIndices,
		Data:             V1Alpha1AttDataToV1(v1alpha1Att.Data),
		Signatures:       v1alpha1Att.Signatures,
	}
}

// V1Alpha1AttestationToV1 converts a v1alpha1 attestation to v1.
func V1Alpha1AttestationToV1(v1alpha1Att *qrysmpb.Attestation) *qrlpb.Attestation {
	if v1alpha1Att == nil {
		return &qrlpb.Attestation{}
	}
	return &qrlpb.Attestation{
		AggregationBits: v1alpha1Att.AggregationBits,
		Data:            V1Alpha1AttDataToV1(v1alpha1Att.Data),
		Signatures:      v1alpha1Att.Signatures,
	}
}

// V1Alpha1AttDataToV1 converts a v1alpha1 attestation data to v1.
func V1Alpha1AttDataToV1(v1alpha1AttData *qrysmpb.AttestationData) *qrlpb.AttestationData {
	if v1alpha1AttData == nil || v1alpha1AttData.Source == nil || v1alpha1AttData.Target == nil {
		return &qrlpb.AttestationData{}
	}
	return &qrlpb.AttestationData{
		Slot:            v1alpha1AttData.Slot,
		Index:           v1alpha1AttData.CommitteeIndex,
		BeaconBlockRoot: v1alpha1AttData.BeaconBlockRoot,
		Source: &qrlpb.Checkpoint{
			Root:  v1alpha1AttData.Source.Root,
			Epoch: v1alpha1AttData.Source.Epoch,
		},
		Target: &qrlpb.Checkpoint{
			Root:  v1alpha1AttData.Target.Root,
			Epoch: v1alpha1AttData.Target.Epoch,
		},
	}
}

// V1Alpha1AttSlashingToV1 converts a v1alpha1 attester slashing to v1.
func V1Alpha1AttSlashingToV1(v1alpha1Slashing *qrysmpb.AttesterSlashing) *qrlpb.AttesterSlashing {
	if v1alpha1Slashing == nil {
		return &qrlpb.AttesterSlashing{}
	}
	return &qrlpb.AttesterSlashing{
		Attestation_1: V1Alpha1IndexedAttToV1(v1alpha1Slashing.Attestation_1),
		Attestation_2: V1Alpha1IndexedAttToV1(v1alpha1Slashing.Attestation_2),
	}
}

// V1Alpha1SignedHeaderToV1 converts a v1alpha1 signed beacon block header to v1.
func V1Alpha1SignedHeaderToV1(v1alpha1Hdr *qrysmpb.SignedBeaconBlockHeader) *qrlpb.SignedBeaconBlockHeader {
	if v1alpha1Hdr == nil || v1alpha1Hdr.Header == nil {
		return &qrlpb.SignedBeaconBlockHeader{}
	}
	return &qrlpb.SignedBeaconBlockHeader{
		Message: &qrlpb.BeaconBlockHeader{
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
func V1SignedHeaderToV1Alpha1(v1Header *qrlpb.SignedBeaconBlockHeader) *qrysmpb.SignedBeaconBlockHeader {
	if v1Header == nil || v1Header.Message == nil {
		return &qrysmpb.SignedBeaconBlockHeader{}
	}
	return &qrysmpb.SignedBeaconBlockHeader{
		Header: &qrysmpb.BeaconBlockHeader{
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
func V1Alpha1ProposerSlashingToV1(v1alpha1Slashing *qrysmpb.ProposerSlashing) *qrlpb.ProposerSlashing {
	if v1alpha1Slashing == nil {
		return &qrlpb.ProposerSlashing{}
	}
	return &qrlpb.ProposerSlashing{
		SignedHeader_1: V1Alpha1SignedHeaderToV1(v1alpha1Slashing.Header_1),
		SignedHeader_2: V1Alpha1SignedHeaderToV1(v1alpha1Slashing.Header_2),
	}
}

// V1Alpha1ExitToV1 converts a v1alpha1 SignedVoluntaryExit to v1.
func V1Alpha1ExitToV1(v1alpha1Exit *qrysmpb.SignedVoluntaryExit) *qrlpb.SignedVoluntaryExit {
	if v1alpha1Exit == nil || v1alpha1Exit.Exit == nil {
		return &qrlpb.SignedVoluntaryExit{}
	}
	return &qrlpb.SignedVoluntaryExit{
		Message: &qrlpb.VoluntaryExit{
			Epoch:          v1alpha1Exit.Exit.Epoch,
			ValidatorIndex: v1alpha1Exit.Exit.ValidatorIndex,
		},
		Signature: v1alpha1Exit.Signature,
	}
}

// V1IndexedAttToV1Alpha1 converts a v1 indexed attestation to v1alpha1.
func V1IndexedAttToV1Alpha1(v1Att *qrlpb.IndexedAttestation) *qrysmpb.IndexedAttestation {
	if v1Att == nil {
		return &qrysmpb.IndexedAttestation{}
	}
	return &qrysmpb.IndexedAttestation{
		AttestingIndices: v1Att.AttestingIndices,
		Data:             V1AttDataToV1Alpha1(v1Att.Data),
		Signatures:       v1Att.Signatures,
	}
}

// V1AttDataToV1Alpha1 converts a v1 attestation data to v1alpha1.
func V1AttDataToV1Alpha1(v1AttData *qrlpb.AttestationData) *qrysmpb.AttestationData {
	if v1AttData == nil || v1AttData.Source == nil || v1AttData.Target == nil {
		return &qrysmpb.AttestationData{}
	}
	return &qrysmpb.AttestationData{
		Slot:            v1AttData.Slot,
		CommitteeIndex:  v1AttData.Index,
		BeaconBlockRoot: v1AttData.BeaconBlockRoot,
		Source: &qrysmpb.Checkpoint{
			Root:  v1AttData.Source.Root,
			Epoch: v1AttData.Source.Epoch,
		},
		Target: &qrysmpb.Checkpoint{
			Root:  v1AttData.Target.Root,
			Epoch: v1AttData.Target.Epoch,
		},
	}
}

// V1AttSlashingToV1Alpha1 converts a v1 attester slashing to v1alpha1.
func V1AttSlashingToV1Alpha1(v1Slashing *qrlpb.AttesterSlashing) *qrysmpb.AttesterSlashing {
	if v1Slashing == nil {
		return &qrysmpb.AttesterSlashing{}
	}
	return &qrysmpb.AttesterSlashing{
		Attestation_1: V1IndexedAttToV1Alpha1(v1Slashing.Attestation_1),
		Attestation_2: V1IndexedAttToV1Alpha1(v1Slashing.Attestation_2),
	}
}

// V1ProposerSlashingToV1Alpha1 converts a v1 proposer slashing to v1alpha1.
func V1ProposerSlashingToV1Alpha1(v1Slashing *qrlpb.ProposerSlashing) *qrysmpb.ProposerSlashing {
	if v1Slashing == nil {
		return &qrysmpb.ProposerSlashing{}
	}
	return &qrysmpb.ProposerSlashing{
		Header_1: V1SignedHeaderToV1Alpha1(v1Slashing.SignedHeader_1),
		Header_2: V1SignedHeaderToV1Alpha1(v1Slashing.SignedHeader_2),
	}
}

// V1Alpha1BeaconBlockZondToV1 converts a v1alpha1 Zond beacon block to a v1
// Zond block.
func V1Alpha1BeaconBlockZondToV1(v1alpha1Block *qrysmpb.BeaconBlockZond) (*qrlpb.BeaconBlockZond, error) {
	marshaledBlk, err := proto.Marshal(v1alpha1Block)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1Block := &qrlpb.BeaconBlockZond{}
	if err := proto.Unmarshal(marshaledBlk, v1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	// Proposer slashing headers have different protobuf field numbers in v1.
	// Map them explicitly and preserve the independent storage of the conversion.
	for i, slashing := range v1alpha1Block.GetBody().GetProposerSlashings() {
		v1Block.Body.ProposerSlashings[i] = proto.Clone(V1Alpha1ProposerSlashingToV1(slashing)).(*qrlpb.ProposerSlashing)
	}
	return v1Block, nil
}

// V1Alpha1BeaconBlockBlindedZondToV1Blinded converts a v1alpha1 Blinded Zond beacon block to a v1 Blinded Zond block.
func V1Alpha1BeaconBlockBlindedZondToV1Blinded(v1alpha1Block *qrysmpb.BlindedBeaconBlockZond) (*qrlpb.BlindedBeaconBlockZond, error) {
	marshaledBlk, err := proto.Marshal(v1alpha1Block)
	if err != nil {
		return nil, errors.Wrap(err, "could not marshal block")
	}
	v1Block := &qrlpb.BlindedBeaconBlockZond{}
	if err := proto.Unmarshal(marshaledBlk, v1Block); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal block")
	}
	// Proposer slashing headers have different protobuf field numbers in v1.
	for i, slashing := range v1alpha1Block.GetBody().GetProposerSlashings() {
		v1Block.Body.ProposerSlashings[i] = proto.Clone(V1Alpha1ProposerSlashingToV1(slashing)).(*qrlpb.ProposerSlashing)
	}
	return v1Block, nil
}

// BeaconStateZondToProto converts a state.BeaconState object to its protobuf equivalent.
func BeaconStateZondToProto(st state.BeaconState) (*qrlpb.BeaconStateZond, error) {
	sourceFork := st.Fork()
	sourceLatestBlockHeader := st.LatestBlockHeader()
	sourceExecutionData := st.ExecutionData()
	sourceExecutionDataVotes := st.ExecutionDataVotes()
	sourceValidators := st.Validators()
	sourceJustificationBits := st.JustificationBits()
	sourcePrevJustifiedCheckpoint := st.PreviousJustifiedCheckpoint()
	sourceCurrJustifiedCheckpoint := st.CurrentJustifiedCheckpoint()
	sourceFinalizedCheckpoint := st.FinalizedCheckpoint()

	resultExecutionDataVotes := make([]*qrlpb.ExecutionData, len(sourceExecutionDataVotes))
	for i, vote := range sourceExecutionDataVotes {
		resultExecutionDataVotes[i] = &qrlpb.ExecutionData{
			DepositRoot:  bytesutil.SafeCopyBytes(vote.DepositRoot),
			DepositCount: vote.DepositCount,
			BlockHash:    bytesutil.SafeCopyBytes(vote.BlockHash),
		}
	}
	resultValidators := make([]*qrlpb.Validator, len(sourceValidators))
	for i, validator := range sourceValidators {
		resultValidators[i] = &qrlpb.Validator{
			Pubkey:                     bytesutil.SafeCopyBytes(validator.PublicKey),
			WithdrawalRecipient:        bytesutil.SafeCopyBytes(validator.WithdrawalRecipient),
			EffectiveBalance:           validator.EffectiveBalance,
			Slashed:                    validator.Slashed,
			ActivationEligibilityEpoch: validator.ActivationEligibilityEpoch,
			ActivationEpoch:            validator.ActivationEpoch,
			ExitEpoch:                  validator.ExitEpoch,
			WithdrawableEpoch:          validator.WithdrawableEpoch,
			RandaoCommitment:           bytesutil.SafeCopyBytes(validator.RandaoCommitment),
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
	sourceLatestExecutionPayloadHeader, ok := executionPayloadHeaderInterface.Proto().(*enginev1.ExecutionPayloadHeaderZond)
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
	sourceHistoricalSummaries := make([]*qrlpb.HistoricalSummary, len(summaries))
	for i, summary := range summaries {
		sourceHistoricalSummaries[i] = &qrlpb.HistoricalSummary{
			BlockSummaryRoot: summary.BlockSummaryRoot,
			StateSummaryRoot: summary.StateSummaryRoot,
		}
	}
	hRoots, err := st.HistoricalRoots()
	if err != nil {
		return nil, errors.Wrap(err, "could not get historical roots")
	}

	result := &qrlpb.BeaconStateZond{
		GenesisTime:           st.GenesisTime(),
		GenesisValidatorsRoot: bytesutil.SafeCopyBytes(st.GenesisValidatorsRoot()),
		Slot:                  st.Slot(),
		Fork: &qrlpb.Fork{
			PreviousVersion: bytesutil.SafeCopyBytes(sourceFork.PreviousVersion),
			CurrentVersion:  bytesutil.SafeCopyBytes(sourceFork.CurrentVersion),
			Epoch:           sourceFork.Epoch,
		},
		LatestBlockHeader: &qrlpb.BeaconBlockHeader{
			Slot:          sourceLatestBlockHeader.Slot,
			ProposerIndex: sourceLatestBlockHeader.ProposerIndex,
			ParentRoot:    bytesutil.SafeCopyBytes(sourceLatestBlockHeader.ParentRoot),
			StateRoot:     bytesutil.SafeCopyBytes(sourceLatestBlockHeader.StateRoot),
			BodyRoot:      bytesutil.SafeCopyBytes(sourceLatestBlockHeader.BodyRoot),
		},
		BlockRoots: bytesutil.SafeCopy2dBytes(st.BlockRoots()),
		StateRoots: bytesutil.SafeCopy2dBytes(st.StateRoots()),
		ExecutionData: &qrlpb.ExecutionData{
			DepositRoot:  bytesutil.SafeCopyBytes(sourceExecutionData.DepositRoot),
			DepositCount: sourceExecutionData.DepositCount,
			BlockHash:    bytesutil.SafeCopyBytes(sourceExecutionData.BlockHash),
		},
		ExecutionDataVotes:         resultExecutionDataVotes,
		ExecutionDepositIndex:      st.ExecutionDepositIndex(),
		Validators:                 resultValidators,
		Balances:                   st.Balances(),
		RandaoMixes:                bytesutil.SafeCopy2dBytes(st.RandaoMixes()),
		Slashings:                  st.Slashings(),
		PreviousEpochParticipation: bytesutil.SafeCopyBytes(sourcePrevEpochParticipation),
		CurrentEpochParticipation:  bytesutil.SafeCopyBytes(sourceCurrEpochParticipation),
		JustificationBits:          bytesutil.SafeCopyBytes(sourceJustificationBits),
		PreviousJustifiedCheckpoint: &qrlpb.Checkpoint{
			Epoch: sourcePrevJustifiedCheckpoint.Epoch,
			Root:  bytesutil.SafeCopyBytes(sourcePrevJustifiedCheckpoint.Root),
		},
		CurrentJustifiedCheckpoint: &qrlpb.Checkpoint{
			Epoch: sourceCurrJustifiedCheckpoint.Epoch,
			Root:  bytesutil.SafeCopyBytes(sourceCurrJustifiedCheckpoint.Root),
		},
		FinalizedCheckpoint: &qrlpb.Checkpoint{
			Epoch: sourceFinalizedCheckpoint.Epoch,
			Root:  bytesutil.SafeCopyBytes(sourceFinalizedCheckpoint.Root),
		},
		InactivityScores: sourceInactivityScores,
		CurrentSyncCommittee: &qrlpb.SyncCommittee{
			Pubkeys: bytesutil.SafeCopy2dBytes(sourceCurrSyncCommittee.Pubkeys),
		},
		NextSyncCommittee: &qrlpb.SyncCommittee{
			Pubkeys: bytesutil.SafeCopy2dBytes(sourceNextSyncCommittee.Pubkeys),
		},
		LatestExecutionPayloadHeader: &enginev1.ExecutionPayloadHeaderZond{
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

// V1Alpha1SignedContributionAndProofToV1 converts a v1alpha1 SignedContributionAndProof object to its v1 equivalent.
func V1Alpha1SignedContributionAndProofToV1(alphaContribution *qrysmpb.SignedContributionAndProof) *qrlpb.SignedContributionAndProof {
	result := &qrlpb.SignedContributionAndProof{
		Message: &qrlpb.ContributionAndProof{
			AggregatorIndex: alphaContribution.Message.AggregatorIndex,
			Contribution: &qrlpb.SyncCommitteeContribution{
				Slot:              alphaContribution.Message.Contribution.Slot,
				BeaconBlockRoot:   alphaContribution.Message.Contribution.BlockRoot,
				SubcommitteeIndex: alphaContribution.Message.Contribution.SubcommitteeIndex,
				AggregationBits:   alphaContribution.Message.Contribution.AggregationBits,
				Signatures:        alphaContribution.Message.Contribution.Signatures,
			},
			SelectionProof: alphaContribution.Message.SelectionProof,
		},
		Signature: alphaContribution.Signature,
	}
	return result
}
