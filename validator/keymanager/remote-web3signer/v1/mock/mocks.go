// Package mock provides sign request fixtures for the remote-web3signer
// keymanager tests, built from the current qrysm protos.
package mock

import (
	"bytes"
	"fmt"

	"github.com/theQRL/go-bitfield"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/testing/util"
)

// Sign request fixture names accepted by GetMockSignRequest. They are the
// contract's artifact types, plus BLINDED_BLOCK_V2 for the blinded block
// oneof and RANDAO_REVEAL for the epoch oneof the client refuses.
const (
	BlockV2                           = "BLOCK_V2"
	BlindedBlockV2                    = "BLINDED_BLOCK_V2"
	Attestation                       = "ATTESTATION"
	AggregationSlot                   = "AGGREGATION_SLOT"
	AggregateAndProof                 = "AGGREGATE_AND_PROOF"
	VoluntaryExit                     = "VOLUNTARY_EXIT"
	SyncCommitteeMessage              = "SYNC_COMMITTEE_MESSAGE"
	SyncCommitteeSelectionProof       = "SYNC_COMMITTEE_SELECTION_PROOF"
	SyncCommitteeContributionAndProof = "SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF"
	ValidatorRegistration             = "VALIDATOR_REGISTRATION"
	RandaoReveal                      = "RANDAO_REVEAL"
)

// SignRequestTypes lists every fixture GetMockSignRequest builds for a type
// the client supports.
var SignRequestTypes = []string{
	BlockV2,
	BlindedBlockV2,
	Attestation,
	AggregationSlot,
	AggregateAndProof,
	VoluntaryExit,
	SyncCommitteeMessage,
	SyncCommitteeSelectionProof,
	SyncCommitteeContributionAndProof,
	ValidatorRegistration,
}

// MockAggregationBits returns a sync committee contribution bitvector of the
// compiled preset's length with its first and third bits set.
func MockAggregationBits() []byte {
	bits := qrysmpb.NewSyncCommitteeAggregationBits()
	bits.SetBitAt(0, true)
	bits.SetBitAt(2, true)
	return bits
}

// MockAttestationBits returns an attestation bitlist of 4 bits with the first
// and third set, plus the SSZ delimiter bit (0x15).
func MockAttestationBits() bitfield.Bitlist {
	bits := bitfield.NewBitlist(4)
	bits.SetBitAt(0, true)
	bits.SetBitAt(2, true)
	return bits
}

// MockSignature returns a signature-sized byte pattern.
func MockSignature(b byte) []byte {
	return bytes.Repeat([]byte{b}, fieldparams.MLDSA87SignatureLength)
}

// GetMockSignRequest returns a SignRequest fixture by type. The public key and
// signing root are zero; callers set them when the test needs a real key.
func GetMockSignRequest(t string) *validatorpb.SignRequest {
	req := &validatorpb.SignRequest{
		PublicKey:       make([]byte, fieldparams.MLDSA87PubkeyLength),
		SigningRoot:     make([]byte, fieldparams.RootLength),
		SignatureDomain: make([]byte, 4),
		SigningSlot:     7,
	}
	switch t {
	case BlockV2:
		req.Object = &validatorpb.SignRequest_BlockZond{
			BlockZond: util.HydrateBeaconBlockZond(&qrysmpb.BeaconBlockZond{Slot: 7, ProposerIndex: 3}),
		}
	case BlindedBlockV2:
		req.Object = &validatorpb.SignRequest_BlindedBlockZond{
			BlindedBlockZond: util.HydrateBlindedBeaconBlockZond(&qrysmpb.BlindedBeaconBlockZond{Slot: 7, ProposerIndex: 3}),
		}
	case Attestation:
		req.Object = &validatorpb.SignRequest_AttestationData{
			AttestationData: mockAttestationData(),
		}
	case AggregationSlot:
		req.Object = &validatorpb.SignRequest_Slot{Slot: 7}
	case AggregateAndProof:
		req.Object = &validatorpb.SignRequest_AggregateAttestationAndProof{
			AggregateAttestationAndProof: &qrysmpb.AggregateAttestationAndProof{
				AggregatorIndex: 5,
				Aggregate: &qrysmpb.Attestation{
					AggregationBits: MockAttestationBits(),
					Data:            mockAttestationData(),
					Signatures:      [][]byte{MockSignature(0x01), MockSignature(0x02)},
				},
				SelectionProof: MockSignature(0x03),
			},
		}
	case VoluntaryExit:
		req.SigningSlot = 0
		req.Object = &validatorpb.SignRequest_Exit{
			Exit: &qrysmpb.VoluntaryExit{Epoch: 0, ValidatorIndex: 9},
		}
	case SyncCommitteeMessage:
		req.Object = &validatorpb.SignRequest_SyncMessageBlockRoot{
			SyncMessageBlockRoot: bytes.Repeat([]byte{0x0b}, fieldparams.RootLength),
		}
	case SyncCommitteeSelectionProof:
		req.Object = &validatorpb.SignRequest_SyncAggregatorSelectionData{
			SyncAggregatorSelectionData: &qrysmpb.SyncAggregatorSelectionData{Slot: 7, SubcommitteeIndex: 0},
		}
	case SyncCommitteeContributionAndProof:
		req.Object = &validatorpb.SignRequest_ContributionAndProof{
			ContributionAndProof: &qrysmpb.ContributionAndProof{
				AggregatorIndex: 4,
				Contribution: &qrysmpb.SyncCommitteeContribution{
					Slot:              7,
					BlockRoot:         bytes.Repeat([]byte{0x0c}, fieldparams.RootLength),
					SubcommitteeIndex: 0,
					AggregationBits:   MockAggregationBits(),
					Signatures:        [][]byte{MockSignature(0x04), MockSignature(0x05)},
				},
				SelectionProof: MockSignature(0x06),
			},
		}
	case ValidatorRegistration:
		req.SigningSlot = 0
		req.Object = &validatorpb.SignRequest_Registration{
			Registration: &qrysmpb.ValidatorRegistrationV1{
				FeeRecipient: bytes.Repeat([]byte{0x0d}, fieldparams.FeeRecipientLength),
				GasLimit:     30000000,
				Timestamp:    1760000000,
				Pubkey:       make([]byte, fieldparams.MLDSA87PubkeyLength),
			},
		}
	case RandaoReveal:
		req.Object = &validatorpb.SignRequest_Epoch{Epoch: 1}
	default:
		panic(fmt.Sprintf("unknown mock sign request type %q", t))
	}
	return req
}

func mockAttestationData() *qrysmpb.AttestationData {
	return &qrysmpb.AttestationData{
		Slot:            7,
		CommitteeIndex:  1,
		BeaconBlockRoot: bytes.Repeat([]byte{0x0a}, fieldparams.RootLength),
		Source:          &qrysmpb.Checkpoint{Epoch: 0, Root: bytes.Repeat([]byte{0x01}, fieldparams.RootLength)},
		Target:          &qrysmpb.Checkpoint{Epoch: 1, Root: bytes.Repeat([]byte{0x02}, fieldparams.RootLength)},
	}
}
