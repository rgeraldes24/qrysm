package v1

import (
	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/common"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
)

// ErrRandaoRevealSignRequest is returned for the epoch oneof: on the target
// stack RANDAO is a hash-onion reveal, not a signature, and goes through the
// randao_reveal endpoint instead.
var ErrRandaoRevealSignRequest = errors.New("unsupported: RANDAO is a hash-onion reveal, not a signature")

// GetBlockV2SignRequest maps a full or blinded Zond block to a BLOCK_V2 request
// carrying the block header.
func GetBlockV2SignRequest(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*BlockV2SignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	var b interfaces.ReadOnlyBeaconBlock
	switch o := request.Object.(type) {
	case *validatorpb.SignRequest_BlockZond:
		if o == nil || o.BlockZond == nil {
			return nil, errors.New("invalid sign request: zond block is nil")
		}
		block, err := blocks.NewBeaconBlock(o.BlockZond)
		if err != nil {
			return nil, err
		}
		b = block
	case *validatorpb.SignRequest_BlindedBlockZond:
		if o == nil || o.BlindedBlockZond == nil {
			return nil, errors.New("invalid sign request: blinded zond block is nil")
		}
		block, err := blocks.NewBeaconBlock(o.BlindedBlockZond)
		if err != nil {
			return nil, err
		}
		b = block
	default:
		return nil, errors.New("failed to cast request object to a zond block")
	}
	header, err := interfaces.BeaconBlockHeaderFromBlockInterface(b)
	if err != nil {
		return nil, err
	}
	envelope, err := newEnvelope(request, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &BlockV2SignRequest{
		Type:        TypeBlockV2,
		ForkInfo:    envelope.forkInfo,
		SigningRoot: envelope.signingRoot,
		BeaconBlock: &BeaconBlockV2{
			Version: BlockVersionZond,
			BlockHeader: &BeaconBlockHeader{
				Slot:          uintString(header.Slot),
				ProposerIndex: uintString(header.ProposerIndex),
				ParentRoot:    header.ParentRoot,
				StateRoot:     header.StateRoot,
				BodyRoot:      header.BodyRoot,
			},
		},
	}, nil
}

// GetAttestationSignRequest maps the request for signing type ATTESTATION.
func GetAttestationSignRequest(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*AttestationSignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	attestation, ok := request.Object.(*validatorpb.SignRequest_AttestationData)
	if !ok || attestation == nil {
		return nil, errors.New("failed to cast request object to attestation data")
	}
	data, err := MapAttestationData(attestation.AttestationData)
	if err != nil {
		return nil, err
	}
	envelope, err := newEnvelope(request, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &AttestationSignRequest{
		Type:        TypeAttestation,
		ForkInfo:    envelope.forkInfo,
		SigningRoot: envelope.signingRoot,
		Attestation: data,
	}, nil
}

// GetAggregationSlotSignRequest maps the request for signing type AGGREGATION_SLOT.
func GetAggregationSlotSignRequest(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*AggregationSlotSignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	aggregationSlot, ok := request.Object.(*validatorpb.SignRequest_Slot)
	if !ok || aggregationSlot == nil {
		return nil, errors.New("failed to cast request object to aggregation slot")
	}
	envelope, err := newEnvelope(request, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &AggregationSlotSignRequest{
		Type:            TypeAggregationSlot,
		ForkInfo:        envelope.forkInfo,
		SigningRoot:     envelope.signingRoot,
		AggregationSlot: &AggregationSlot{Slot: uintString(aggregationSlot.Slot)},
	}, nil
}

// GetAggregateAndProofSignRequest maps the request for signing type AGGREGATE_AND_PROOF.
func GetAggregateAndProofSignRequest(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*AggregateAndProofSignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	aggregate, ok := request.Object.(*validatorpb.SignRequest_AggregateAttestationAndProof)
	if !ok || aggregate == nil {
		return nil, errors.New("failed to cast request object to aggregate attestation and proof")
	}
	aggregateAndProof, err := MapAggregateAndProof(aggregate.AggregateAttestationAndProof)
	if err != nil {
		return nil, err
	}
	envelope, err := newEnvelope(request, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &AggregateAndProofSignRequest{
		Type:              TypeAggregateAndProof,
		ForkInfo:          envelope.forkInfo,
		SigningRoot:       envelope.signingRoot,
		AggregateAndProof: aggregateAndProof,
	}, nil
}

// GetVoluntaryExitSignRequest maps the request for signing type VOLUNTARY_EXIT.
// The caller sets SigningSlot to the first slot of the exit epoch.
func GetVoluntaryExitSignRequest(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*VoluntaryExitSignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	voluntaryExit, ok := request.Object.(*validatorpb.SignRequest_Exit)
	if !ok || voluntaryExit == nil || voluntaryExit.Exit == nil {
		return nil, errors.New("failed to cast request object to voluntary exit")
	}
	envelope, err := newEnvelope(request, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &VoluntaryExitSignRequest{
		Type:        TypeVoluntaryExit,
		ForkInfo:    envelope.forkInfo,
		SigningRoot: envelope.signingRoot,
		VoluntaryExit: &VoluntaryExit{
			Epoch:          uintString(voluntaryExit.Exit.Epoch),
			ValidatorIndex: uintString(voluntaryExit.Exit.ValidatorIndex),
		},
	}, nil
}

// GetSyncCommitteeMessageSignRequest maps the request for signing type
// SYNC_COMMITTEE_MESSAGE. The oneof only carries the block root, so the slot is
// the request's signing slot.
func GetSyncCommitteeMessageSignRequest(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*SyncCommitteeMessageSignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	message, ok := request.Object.(*validatorpb.SignRequest_SyncMessageBlockRoot)
	if !ok || message == nil {
		return nil, errors.New("failed to cast request object to sync committee message")
	}
	blockRoot, err := fixedBytes("sync committee message block root", message.SyncMessageBlockRoot, fieldparams.RootLength)
	if err != nil {
		return nil, err
	}
	envelope, err := newEnvelope(request, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &SyncCommitteeMessageSignRequest{
		Type:        TypeSyncCommitteeMessage,
		ForkInfo:    envelope.forkInfo,
		SigningRoot: envelope.signingRoot,
		SyncCommitteeMessage: &SyncCommitteeMessage{
			BeaconBlockRoot: blockRoot,
			Slot:            uintString(request.SigningSlot),
		},
	}, nil
}

// GetSyncCommitteeSelectionProofSignRequest maps the request for signing type
// SYNC_COMMITTEE_SELECTION_PROOF.
func GetSyncCommitteeSelectionProofSignRequest(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*SyncCommitteeSelectionProofSignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	selectionProof, ok := request.Object.(*validatorpb.SignRequest_SyncAggregatorSelectionData)
	if !ok || selectionProof == nil {
		return nil, errors.New("failed to cast request object to sync committee selection proof")
	}
	data, err := MapSyncAggregatorSelectionData(selectionProof.SyncAggregatorSelectionData)
	if err != nil {
		return nil, err
	}
	envelope, err := newEnvelope(request, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &SyncCommitteeSelectionProofSignRequest{
		Type:                        TypeSyncCommitteeSelectionProof,
		ForkInfo:                    envelope.forkInfo,
		SigningRoot:                 envelope.signingRoot,
		SyncAggregatorSelectionData: data,
	}, nil
}

// GetSyncCommitteeContributionAndProofSignRequest maps the request for signing
// type SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF.
func GetSyncCommitteeContributionAndProofSignRequest(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*SyncCommitteeContributionAndProofSignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	contribution, ok := request.Object.(*validatorpb.SignRequest_ContributionAndProof)
	if !ok || contribution == nil {
		return nil, errors.New("failed to cast request object to sync committee contribution and proof")
	}
	contributionAndProof, err := MapContributionAndProof(contribution.ContributionAndProof)
	if err != nil {
		return nil, err
	}
	envelope, err := newEnvelope(request, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &SyncCommitteeContributionAndProofSignRequest{
		Type:                 TypeSyncCommitteeContributionAndProof,
		ForkInfo:             envelope.forkInfo,
		SigningRoot:          envelope.signingRoot,
		ContributionAndProof: contributionAndProof,
	}, nil
}

// GetValidatorRegistrationSignRequest maps the request for signing type
// VALIDATOR_REGISTRATION. It carries no fork_info.
func GetValidatorRegistrationSignRequest(request *validatorpb.SignRequest) (*ValidatorRegistrationSignRequest, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}
	registration, ok := request.Object.(*validatorpb.SignRequest_Registration)
	if !ok || registration == nil || registration.Registration == nil {
		return nil, errors.New("failed to cast request object to validator registration")
	}
	r := registration.Registration
	signingRoot, err := fixedBytes("signing root", request.SigningRoot, fieldparams.RootLength)
	if err != nil {
		return nil, err
	}
	feeRecipient, err := fixedBytes("fee recipient", r.FeeRecipient, fieldparams.FeeRecipientLength)
	if err != nil {
		return nil, err
	}
	pubkey, err := fixedBytes("registration public key", r.Pubkey, fieldparams.MLDSA87PubkeyLength)
	if err != nil {
		return nil, err
	}
	return &ValidatorRegistrationSignRequest{
		Type:        TypeValidatorRegistration,
		SigningRoot: signingRoot,
		ValidatorRegistration: &ValidatorRegistration{
			FeeRecipient: common.BytesToAddress(feeRecipient),
			GasLimit:     uintString(r.GasLimit),
			Timestamp:    uintString(r.Timestamp),
			Pubkey:       pubkey,
		},
	}, nil
}

// GetRandaoRevealRequest builds the body of a RANDAO reveal request for the
// proposal at slot.
func GetRandaoRevealRequest(slot primitives.Slot, commitment, genesisValidatorsRoot []byte) (*RandaoRevealRequest, error) {
	c, err := fixedBytes("randao commitment", commitment, fieldparams.RandaoCommitmentLength)
	if err != nil {
		return nil, err
	}
	forkInfo, err := MapForkInfo(slot, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &RandaoRevealRequest{
		Slot:             uintString(slot),
		RandaoCommitment: c,
		ForkInfo:         forkInfo,
	}, nil
}

// envelope holds the fields every request except the registration carries.
type envelope struct {
	forkInfo    *ForkInfo
	signingRoot []byte
}

func newEnvelope(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (*envelope, error) {
	signingRoot, err := fixedBytes("signing root", request.SigningRoot, fieldparams.RootLength)
	if err != nil {
		return nil, err
	}
	forkInfo, err := MapForkInfo(request.SigningSlot, genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}
	return &envelope{forkInfo: forkInfo, signingRoot: signingRoot}, nil
}
