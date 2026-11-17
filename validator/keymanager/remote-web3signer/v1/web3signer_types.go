// Package v1 defines the JSON request types of the remote signer's consensus
// API (/api/v1/consensus), contract version 1.0.0, and maps qrysm's sign
// requests onto them.
//
// Encoding: uint64 values are decimal strings, fixed-size bytes are 0x-prefixed
// lowercase hex, signatures of aggregates are per-participant lists, and the
// 64-byte fee recipient uses the Q address form. The signer decodes strictly,
// so these types must not carry fields the contract does not define.
package v1

import (
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/common/hexutil"
)

// Artifact types of the sign endpoint.
const (
	TypeBlockV2                           = "BLOCK_V2"
	TypeAttestation                       = "ATTESTATION"
	TypeAggregationSlot                   = "AGGREGATION_SLOT"
	TypeAggregateAndProof                 = "AGGREGATE_AND_PROOF"
	TypeVoluntaryExit                     = "VOLUNTARY_EXIT"
	TypeSyncCommitteeMessage              = "SYNC_COMMITTEE_MESSAGE"
	TypeSyncCommitteeSelectionProof       = "SYNC_COMMITTEE_SELECTION_PROOF"
	TypeSyncCommitteeContributionAndProof = "SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF"
	TypeValidatorRegistration             = "VALIDATOR_REGISTRATION"
)

// BlockVersionZond is the only BLOCK_V2 version qrysm signs.
const BlockVersionZond = "ZOND"

// BlockV2SignRequest is a BLOCK_V2 sign request. It carries only the block
// header: the header's hash tree root equals that of the full and of the
// blinded block.
type BlockV2SignRequest struct {
	Type        string         `json:"type"`
	ForkInfo    *ForkInfo      `json:"fork_info"`
	SigningRoot hexutil.Bytes  `json:"signing_root"`
	BeaconBlock *BeaconBlockV2 `json:"beacon_block"`
}

// AttestationSignRequest is an ATTESTATION sign request.
type AttestationSignRequest struct {
	Type        string           `json:"type"`
	ForkInfo    *ForkInfo        `json:"fork_info"`
	SigningRoot hexutil.Bytes    `json:"signing_root"`
	Attestation *AttestationData `json:"attestation"`
}

// AggregationSlotSignRequest is an AGGREGATION_SLOT (aggregation selection
// proof) sign request.
type AggregationSlotSignRequest struct {
	Type            string           `json:"type"`
	ForkInfo        *ForkInfo        `json:"fork_info"`
	SigningRoot     hexutil.Bytes    `json:"signing_root"`
	AggregationSlot *AggregationSlot `json:"aggregation_slot"`
}

// AggregateAndProofSignRequest is an AGGREGATE_AND_PROOF sign request.
type AggregateAndProofSignRequest struct {
	Type              string             `json:"type"`
	ForkInfo          *ForkInfo          `json:"fork_info"`
	SigningRoot       hexutil.Bytes      `json:"signing_root"`
	AggregateAndProof *AggregateAndProof `json:"aggregate_and_proof"`
}

// VoluntaryExitSignRequest is a VOLUNTARY_EXIT sign request.
type VoluntaryExitSignRequest struct {
	Type          string         `json:"type"`
	ForkInfo      *ForkInfo      `json:"fork_info"`
	SigningRoot   hexutil.Bytes  `json:"signing_root"`
	VoluntaryExit *VoluntaryExit `json:"voluntary_exit"`
}

// SyncCommitteeMessageSignRequest is a SYNC_COMMITTEE_MESSAGE sign request.
type SyncCommitteeMessageSignRequest struct {
	Type                 string                `json:"type"`
	ForkInfo             *ForkInfo             `json:"fork_info"`
	SigningRoot          hexutil.Bytes         `json:"signing_root"`
	SyncCommitteeMessage *SyncCommitteeMessage `json:"sync_committee_message"`
}

// SyncCommitteeSelectionProofSignRequest is a SYNC_COMMITTEE_SELECTION_PROOF
// sign request.
type SyncCommitteeSelectionProofSignRequest struct {
	Type                        string                       `json:"type"`
	ForkInfo                    *ForkInfo                    `json:"fork_info"`
	SigningRoot                 hexutil.Bytes                `json:"signing_root"`
	SyncAggregatorSelectionData *SyncAggregatorSelectionData `json:"sync_aggregator_selection_data"`
}

// SyncCommitteeContributionAndProofSignRequest is a
// SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF sign request.
type SyncCommitteeContributionAndProofSignRequest struct {
	Type                 string                `json:"type"`
	ForkInfo             *ForkInfo             `json:"fork_info"`
	SigningRoot          hexutil.Bytes         `json:"signing_root"`
	ContributionAndProof *ContributionAndProof `json:"contribution_and_proof"`
}

// ValidatorRegistrationSignRequest is a VALIDATOR_REGISTRATION sign request.
// It has no fork_info: the builder domain uses the genesis fork version and a
// zero genesis validators root.
type ValidatorRegistrationSignRequest struct {
	Type                  string                 `json:"type"`
	SigningRoot           hexutil.Bytes          `json:"signing_root"`
	ValidatorRegistration *ValidatorRegistration `json:"validator_registration"`
}

// RandaoRevealRequest is the body of POST /api/v1/consensus/randao_reveal/{identifier}.
type RandaoRevealRequest struct {
	Slot             string        `json:"slot"`
	RandaoCommitment hexutil.Bytes `json:"randao_commitment"`
	ForkInfo         *ForkInfo     `json:"fork_info"`
}

////////////////////////////////////////////////////////////////////////////////
// sub properties of Sign Requests /////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

// ForkInfo binds a request to a network. Under qrysm's single fork it is always
// {GENESIS_FORK_VERSION, GENESIS_FORK_VERSION, "0"} plus the genesis validators
// root.
type ForkInfo struct {
	Fork                  *Fork         `json:"fork"`
	GenesisValidatorsRoot hexutil.Bytes `json:"genesis_validators_root"`
}

// Fork a sub property of ForkInfo.
type Fork struct {
	PreviousVersion hexutil.Bytes `json:"previous_version"`
	CurrentVersion  hexutil.Bytes `json:"current_version"`
	Epoch           string        `json:"epoch"`
}

// BeaconBlockV2 a sub property of BlockV2SignRequest.
type BeaconBlockV2 struct {
	Version     string             `json:"version"`
	BlockHeader *BeaconBlockHeader `json:"block_header"`
}

// BeaconBlockHeader a sub property of BeaconBlockV2.
type BeaconBlockHeader struct {
	Slot          string        `json:"slot"`
	ProposerIndex string        `json:"proposer_index"`
	ParentRoot    hexutil.Bytes `json:"parent_root"`
	StateRoot     hexutil.Bytes `json:"state_root"`
	BodyRoot      hexutil.Bytes `json:"body_root"`
}

// AttestationData a sub property of AttestationSignRequest and Attestation.
// Index carries the proto field committee_index.
type AttestationData struct {
	Slot            string        `json:"slot"`
	Index           string        `json:"index"`
	BeaconBlockRoot hexutil.Bytes `json:"beacon_block_root"`
	Source          *Checkpoint   `json:"source"`
	Target          *Checkpoint   `json:"target"`
}

// Checkpoint a sub property of AttestationData.
type Checkpoint struct {
	Epoch string        `json:"epoch"`
	Root  hexutil.Bytes `json:"root"`
}

// AggregationSlot a sub property of AggregationSlotSignRequest.
type AggregationSlot struct {
	Slot string `json:"slot"`
}

// AggregateAndProof a sub property of AggregateAndProofSignRequest.
type AggregateAndProof struct {
	AggregatorIndex string        `json:"aggregator_index"`
	Aggregate       *Attestation  `json:"aggregate"`
	SelectionProof  hexutil.Bytes `json:"selection_proof"`
}

// Attestation a sub property of AggregateAndProof. AggregationBits are the SSZ
// bitlist bytes including the delimiter bit; Signatures holds one signature per
// set bit.
type Attestation struct {
	AggregationBits hexutil.Bytes    `json:"aggregation_bits"`
	Data            *AttestationData `json:"data"`
	Signatures      []hexutil.Bytes  `json:"signatures"`
}

// VoluntaryExit a sub property of VoluntaryExitSignRequest.
type VoluntaryExit struct {
	Epoch          string `json:"epoch"`
	ValidatorIndex string `json:"validator_index"`
}

// SyncCommitteeMessage a sub property of SyncCommitteeMessageSignRequest.
type SyncCommitteeMessage struct {
	BeaconBlockRoot hexutil.Bytes `json:"beacon_block_root"`
	Slot            string        `json:"slot"`
}

// SyncAggregatorSelectionData a sub property of SyncCommitteeSelectionProofSignRequest.
type SyncAggregatorSelectionData struct {
	Slot              string `json:"slot"`
	SubcommitteeIndex string `json:"subcommittee_index"`
}

// ContributionAndProof a sub property of SyncCommitteeContributionAndProofSignRequest.
type ContributionAndProof struct {
	AggregatorIndex string                     `json:"aggregator_index"`
	SelectionProof  hexutil.Bytes              `json:"selection_proof"`
	Contribution    *SyncCommitteeContribution `json:"contribution"`
}

// SyncCommitteeContribution a sub property of ContributionAndProof.
// BeaconBlockRoot carries the proto field block_root.
type SyncCommitteeContribution struct {
	Slot              string          `json:"slot"`
	BeaconBlockRoot   hexutil.Bytes   `json:"beacon_block_root"`
	SubcommitteeIndex string          `json:"subcommittee_index"`
	AggregationBits   hexutil.Bytes   `json:"aggregation_bits"`
	Signatures        []hexutil.Bytes `json:"signatures"`
}

// ValidatorRegistration a sub property of ValidatorRegistrationSignRequest.
type ValidatorRegistration struct {
	FeeRecipient common.Address `json:"fee_recipient"`
	GasLimit     string         `json:"gas_limit"`
	Timestamp    string         `json:"timestamp"`
	Pubkey       hexutil.Bytes  `json:"pubkey"`
}
