package mock

/*
import (
	"fmt"

	"github.com/theQRL/go-bitfield"
	dilithium2 "github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/go-zond/common/hexutil"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	zond "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	validatorpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/v4/testing/util"
	v1 "github.com/theQRL/qrysm/v4/validator/keymanager/remote-web3signer/v1"
)

/////////////////////////////////////////////////////////////////////////////////////////////////
//////////////// Mock Requests //////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////////////////////////

func MockSyncComitteeBits() []byte {
	currSize := new(zond.SyncAggregate).SyncCommitteeBits.Len()
	switch currSize {
	case 512:
		return bitfield.NewBitvector512()
	case 32:
		return bitfield.NewBitvector32()
	default:
		return nil
	}
}

func MockAggregationBits() []byte {
	currSize := new(zond.SyncCommitteeContribution).ParticipationBits.Len()
	switch currSize {
	case 128:
		return bitfield.NewBitvector128()
	case 8:
		return bitfield.NewBitvector8()
	default:
		return nil
	}
}

// GetMockSignRequest returns a mock SignRequest by type.
func GetMockSignRequest(t string) *validatorpb.SignRequest {
	switch t {
	case "AGGREGATION_SLOT":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_Slot{
				Slot: 0,
			},
			SigningSlot: 0,
		}
	case "AGGREGATE_AND_PROOF":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_AggregateAttestationAndProof{
				AggregateAttestationAndProof: &zond.AggregateAttestationAndProof{
					AggregatorIndex: 0,
					Aggregate: &zond.Attestation{
						ParticipationBits: bitfield.Bitlist{0b1101},
						Data: &zond.AttestationData{
							BeaconBlockRoot: make([]byte, fieldparams.RootLength),
							Source: &zond.Checkpoint{
								Root: make([]byte, fieldparams.RootLength),
							},
							Target: &zond.Checkpoint{
								Root: make([]byte, fieldparams.RootLength),
							},
						},
						Signatures: make([][]byte, 1),
					},
					SelectionProof: make([]byte, dilithium2.CryptoBytes),
				},
			},
			SigningSlot: 0,
		}
	case "ATTESTATION":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_AttestationData{
				AttestationData: &zond.AttestationData{
					BeaconBlockRoot: make([]byte, fieldparams.RootLength),
					Source: &zond.Checkpoint{
						Root: make([]byte, fieldparams.RootLength),
					},
					Target: &zond.Checkpoint{
						Root: make([]byte, fieldparams.RootLength),
					},
				},
			},
			SigningSlot: 0,
		}
	case "BLOCK":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_Block{
				Block: &zond.BeaconBlock{
					Slot:          0,
					ProposerIndex: 0,
					ParentRoot:    make([]byte, fieldparams.RootLength),
					StateRoot:     make([]byte, fieldparams.RootLength),
					Body: &zond.BeaconBlockBody{
						RandaoReveal: make([]byte, 32),
						Zond1Data: &zond.Zond1Data{
							DepositRoot:  make([]byte, fieldparams.RootLength),
							DepositCount: 0,
							BlockHash:    make([]byte, 32),
						},
						Graffiti: make([]byte, 32),
						ProposerSlashings: []*zond.ProposerSlashing{
							{
								Header_1: &zond.SignedBeaconBlockHeader{
									Header: &zond.BeaconBlockHeader{
										Slot:          0,
										ProposerIndex: 0,
										ParentRoot:    make([]byte, fieldparams.RootLength),
										StateRoot:     make([]byte, fieldparams.RootLength),
										BodyRoot:      make([]byte, fieldparams.RootLength),
									},
									Signature: make([]byte, dilithium2.CryptoBytes),
								},
								Header_2: &zond.SignedBeaconBlockHeader{
									Header: &zond.BeaconBlockHeader{
										Slot:          0,
										ProposerIndex: 0,
										ParentRoot:    make([]byte, fieldparams.RootLength),
										StateRoot:     make([]byte, fieldparams.RootLength),
										BodyRoot:      make([]byte, fieldparams.RootLength),
									},
									Signature: make([]byte, dilithium2.CryptoBytes),
								},
							},
						},
						AttesterSlashings: []*zond.AttesterSlashing{
							{
								Attestation_1: &zond.IndexedAttestation{
									AttestingIndices: []uint64{0, 1, 2},
									Data: &zond.AttestationData{
										BeaconBlockRoot: make([]byte, fieldparams.RootLength),
										Source: &zond.Checkpoint{
											Root: make([]byte, fieldparams.RootLength),
										},
										Target: &zond.Checkpoint{
											Root: make([]byte, fieldparams.RootLength),
										},
									},
									Signatures: make([][]byte, 3),
								},
								Attestation_2: &zond.IndexedAttestation{
									AttestingIndices: []uint64{0, 1, 2},
									Data: &zond.AttestationData{
										BeaconBlockRoot: make([]byte, fieldparams.RootLength),
										Source: &zond.Checkpoint{
											Root: make([]byte, fieldparams.RootLength),
										},
										Target: &zond.Checkpoint{
											Root: make([]byte, fieldparams.RootLength),
										},
									},
									Signatures: make([][]byte, 3),
								},
							},
						},
						Attestations: []*zond.Attestation{
							{
								ParticipationBits: bitfield.Bitlist{0b1101},
								Data: &zond.AttestationData{
									BeaconBlockRoot: make([]byte, fieldparams.RootLength),
									Source: &zond.Checkpoint{
										Root: make([]byte, fieldparams.RootLength),
									},
									Target: &zond.Checkpoint{
										Root: make([]byte, fieldparams.RootLength),
									},
								},
								Signatures: [][]byte{},
							},
						},
						Deposits: []*zond.Deposit{
							{
								Proof: [][]byte{[]byte("A")},
								Data: &zond.Deposit_Data{
									PublicKey:             make([]byte, dilithium2.CryptoPublicKeyBytes),
									WithdrawalCredentials: make([]byte, 32),
									Amount:                0,
									Signature:             make([]byte, dilithium2.CryptoBytes),
								},
							},
						},
						VoluntaryExits: []*zond.SignedVoluntaryExit{
							{
								Exit: &zond.VoluntaryExit{
									Epoch:          0,
									ValidatorIndex: 0,
								},
								Signature: make([]byte, dilithium2.CryptoBytes),
							},
						},
					},
				},
			},
			SigningSlot: 0,
		}
	case "BLOCK_CAPELLA":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_BlockCapella{
				BlockCapella: util.HydrateBeaconBlock(&zond.BeaconBlock{}),
			},
		}
	case "BLOCK_BLINDED_CAPELLA":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_BlindedBlockCapella{
				BlindedBlockCapella: util.HydrateBlindedBeaconBlock(&zond.BlindedBeaconBlock{}),
			},
		}
	case "RANDAO_REVEAL":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_Epoch{
				Epoch: 0,
			},
			SigningSlot: 0,
		}
	case "SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_ContributionAndProof{
				ContributionAndProof: &zond.ContributionAndProof{
					AggregatorIndex: 0,
					Contribution: &zond.SyncCommitteeContribution{
						Slot:              0,
						BlockRoot:         make([]byte, fieldparams.RootLength),
						SubcommitteeIndex: 0,
						ParticipationBits: MockAggregationBits(),
						Signatures:        [][]byte{},
					},
					SelectionProof: make([]byte, dilithium2.CryptoBytes),
				},
			},
			SigningSlot: 0,
		}
	case "SYNC_COMMITTEE_MESSAGE":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_SyncMessageBlockRoot{
				SyncMessageBlockRoot: make([]byte, fieldparams.RootLength),
			},
			SigningSlot: 0,
		}
	case "SYNC_COMMITTEE_SELECTION_PROOF":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_SyncAggregatorSelectionData{
				SyncAggregatorSelectionData: &zond.SyncAggregatorSelectionData{
					Slot:              0,
					SubcommitteeIndex: 0,
				},
			},
			SigningSlot: 0,
		}
	case "VOLUNTARY_EXIT":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_Exit{
				Exit: &zond.VoluntaryExit{
					Epoch:          0,
					ValidatorIndex: 0,
				},
			},
			SigningSlot: 0,
		}
	case "VALIDATOR_REGISTRATION":
		return &validatorpb.SignRequest{
			PublicKey:       make([]byte, dilithium2.CryptoPublicKeyBytes),
			SigningRoot:     make([]byte, fieldparams.RootLength),
			SignatureDomain: make([]byte, 4),
			Object: &validatorpb.SignRequest_Registration{
				Registration: &zond.ValidatorRegistrationV1{
					FeeRecipient: make([]byte, fieldparams.FeeRecipientLength),
					GasLimit:     uint64(0),
					Timestamp:    uint64(0),
					Pubkey:       make([]byte, dilithium2.CryptoBytes),
				},
			},
			SigningSlot: 0,
		}
	default:
		fmt.Printf("Web3signer sign request type: %v  not found", t)
		return nil
	}
}

// MockAggregationSlotSignRequest is a mock implementation of the AggregationSlotSignRequest.
func MockAggregationSlotSignRequest() *v1.AggregationSlotSignRequest {
	return &v1.AggregationSlotSignRequest{
		Type:            "AGGREGATION_SLOT",
		ForkInfo:        MockForkInfo(),
		SigningRoot:     make([]byte, fieldparams.RootLength),
		AggregationSlot: &v1.AggregationSlot{Slot: "0"},
	}
}

// MockAggregateAndProofSignRequest is a mock implementation of the AggregateAndProofSignRequest.
func MockAggregateAndProofSignRequest() *v1.AggregateAndProofSignRequest {
	return &v1.AggregateAndProofSignRequest{
		Type:        "AGGREGATE_AND_PROOF",
		ForkInfo:    MockForkInfo(),
		SigningRoot: make([]byte, fieldparams.RootLength),
		AggregateAndProof: &v1.AggregateAndProof{
			AggregatorIndex: "0",
			Aggregate:       MockAttestation(),
			SelectionProof:  make([]byte, dilithium2.CryptoBytes),
		},
	}
}

// MockAttestationSignRequest is a mock implementation of the AttestationSignRequest.
func MockAttestationSignRequest() *v1.AttestationSignRequest {
	return &v1.AttestationSignRequest{
		Type:        "ATTESTATION",
		ForkInfo:    MockForkInfo(),
		SigningRoot: make([]byte, fieldparams.RootLength),
		Attestation: MockAttestation().Data,
	}
}

// MockBlockSignRequest is a mock implementation of the BlockSignRequest.
func MockBlockSignRequest() *v1.BlockSignRequest {
	return &v1.BlockSignRequest{
		Type:        "BLOCK",
		ForkInfo:    MockForkInfo(),
		SigningRoot: make([]byte, fieldparams.RootLength),
		Block: &v1.BeaconBlock{
			Slot:          "0",
			ProposerIndex: "0",
			ParentRoot:    make([]byte, fieldparams.RootLength),
			StateRoot:     make([]byte, fieldparams.RootLength),
			Body:          MockBeaconBlockBody(),
		},
	}
}

func MockBlockBlindedSignRequest(bodyRoot []byte, version string) *v1.BlockBlindedSignRequest {
	return &v1.BlockBlindedSignRequest{
		Type:        "BLOCK_V1",
		ForkInfo:    MockForkInfo(),
		SigningRoot: make([]byte, fieldparams.RootLength),
		BeaconBlock: &v1.BeaconBlockBlinded{
			Version: version,
			BlockHeader: &v1.BeaconBlockHeader{
				Slot:          "0",
				ProposerIndex: "0",
				ParentRoot:    make([]byte, fieldparams.RootLength),
				StateRoot:     make([]byte, fieldparams.RootLength),
				BodyRoot:      bodyRoot,
			},
		},
	}
}

// MockRandaoRevealSignRequest is a mock implementation of the RandaoRevealSignRequest.
func MockRandaoRevealSignRequest() *v1.RandaoRevealSignRequest {
	return &v1.RandaoRevealSignRequest{
		Type:        "RANDAO_REVEAL",
		ForkInfo:    MockForkInfo(),
		SigningRoot: make([]byte, fieldparams.RootLength),
		RandaoReveal: &v1.RandaoReveal{
			Epoch: "0",
		},
	}
}

// MockSyncCommitteeContributionAndProofSignRequest is a mock implementation of the SyncCommitteeContributionAndProofSignRequest.
func MockSyncCommitteeContributionAndProofSignRequest() *v1.SyncCommitteeContributionAndProofSignRequest {
	return &v1.SyncCommitteeContributionAndProofSignRequest{
		Type:                 "SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF",
		ForkInfo:             MockForkInfo(),
		SigningRoot:          make([]byte, fieldparams.RootLength),
		ContributionAndProof: MockContributionAndProof(),
	}
}

// MockSyncCommitteeMessageSignRequest is a mock implementation of the SyncCommitteeMessageSignRequest.
func MockSyncCommitteeMessageSignRequest() *v1.SyncCommitteeMessageSignRequest {
	return &v1.SyncCommitteeMessageSignRequest{
		Type:        "SYNC_COMMITTEE_MESSAGE",
		ForkInfo:    MockForkInfo(),
		SigningRoot: make([]byte, fieldparams.RootLength),
		SyncCommitteeMessage: &v1.SyncCommitteeMessage{
			BeaconBlockRoot: make([]byte, fieldparams.RootLength),
			Slot:            "0",
		},
	}
}

// MockSyncCommitteeSelectionProofSignRequest is a mock implementation of the SyncCommitteeSelectionProofSignRequest.
func MockSyncCommitteeSelectionProofSignRequest() *v1.SyncCommitteeSelectionProofSignRequest {
	return &v1.SyncCommitteeSelectionProofSignRequest{
		Type:        "SYNC_COMMITTEE_SELECTION_PROOF",
		ForkInfo:    MockForkInfo(),
		SigningRoot: make([]byte, fieldparams.RootLength),
		SyncAggregatorSelectionData: &v1.SyncAggregatorSelectionData{
			Slot:              "0",
			SubcommitteeIndex: "0",
		},
	}
}

// MockVoluntaryExitSignRequest is a mock implementation of the VoluntaryExitSignRequest.
func MockVoluntaryExitSignRequest() *v1.VoluntaryExitSignRequest {
	return &v1.VoluntaryExitSignRequest{
		Type:        "VOLUNTARY_EXIT",
		ForkInfo:    MockForkInfo(),
		SigningRoot: make([]byte, fieldparams.RootLength),
		VoluntaryExit: &v1.VoluntaryExit{
			Epoch:          "0",
			ValidatorIndex: "0",
		},
	}
}

// MockValidatorRegistrationSignRequest is a mock implementation of the ValidatorRegistrationSignRequest.
func MockValidatorRegistrationSignRequest() *v1.ValidatorRegistrationSignRequest {
	return &v1.ValidatorRegistrationSignRequest{
		Type:        "VALIDATOR_REGISTRATION",
		SigningRoot: make([]byte, fieldparams.RootLength),
		ValidatorRegistration: &v1.ValidatorRegistration{
			FeeRecipient: make([]byte, fieldparams.FeeRecipientLength),
			GasLimit:     fmt.Sprint(0),
			Timestamp:    fmt.Sprint(0),
			Pubkey:       make([]byte, dilithium2.CryptoBytes),
		},
	}
}

/////////////////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////////////////////////////////////

// MockForkInfo is a mock implementation of the ForkInfo.
func MockForkInfo() *v1.ForkInfo {
	return &v1.ForkInfo{
		Fork: &v1.Fork{
			PreviousVersion: make([]byte, 4),
			CurrentVersion:  make([]byte, 4),
			Epoch:           "0",
		},
		GenesisValidatorsRoot: make([]byte, fieldparams.RootLength),
	}
}

// MockAttestation is a mock implementation of the Attestation.
func MockAttestation() *v1.Attestation {
	return &v1.Attestation{
		ParticipationBits: []byte(bitfield.Bitlist{0b1101}),
		Data: &v1.AttestationData{
			Slot:            "0",
			Index:           "0",
			BeaconBlockRoot: make([]byte, fieldparams.RootLength),
			Source: &v1.Checkpoint{
				Epoch: "0",
				Root:  hexutil.Encode(make([]byte, fieldparams.RootLength)),
			},
			Target: &v1.Checkpoint{
				Epoch: "0",
				Root:  hexutil.Encode(make([]byte, fieldparams.RootLength)),
			},
		},
		Signatures: []hexutil.Bytes{},
	}
}

func MockIndexedAttestation() *v1.IndexedAttestation {
	return &v1.IndexedAttestation{
		AttestingIndices: []string{"0", "1", "2"},
		Data: &v1.AttestationData{
			Slot:            "0",
			Index:           "0",
			BeaconBlockRoot: make([]byte, fieldparams.RootLength),
			Source: &v1.Checkpoint{
				Epoch: "0",
				Root:  hexutil.Encode(make([]byte, fieldparams.RootLength)),
			},
			Target: &v1.Checkpoint{
				Epoch: "0",
				Root:  hexutil.Encode(make([]byte, fieldparams.RootLength)),
			},
		},
		Signatures: []hexutil.Bytes{},
	}
}

func MockBeaconBlockBody() *v1.BeaconBlockBody {
	return &v1.BeaconBlockBody{
		RandaoReveal: make([]byte, 32),
		Zond1Data: &v1.Zond1Data{
			DepositRoot:  make([]byte, fieldparams.RootLength),
			DepositCount: "0",
			BlockHash:    make([]byte, 32),
		},
		Graffiti: make([]byte, 32),
		ProposerSlashings: []*v1.ProposerSlashing{
			{
				Signedheader1: &v1.SignedBeaconBlockHeader{
					Message: &v1.BeaconBlockHeader{
						Slot:          "0",
						ProposerIndex: "0",
						ParentRoot:    make([]byte, fieldparams.RootLength),
						StateRoot:     make([]byte, fieldparams.RootLength),
						BodyRoot:      make([]byte, fieldparams.RootLength),
					},
					Signature: make([]byte, dilithium2.CryptoBytes),
				},
				Signedheader2: &v1.SignedBeaconBlockHeader{
					Message: &v1.BeaconBlockHeader{
						Slot:          "0",
						ProposerIndex: "0",
						ParentRoot:    make([]byte, fieldparams.RootLength),
						StateRoot:     make([]byte, fieldparams.RootLength),
						BodyRoot:      make([]byte, fieldparams.RootLength),
					},
					Signature: make([]byte, dilithium2.CryptoBytes),
				},
			},
		},
		AttesterSlashings: []*v1.AttesterSlashing{
			{
				Attestation1: MockIndexedAttestation(),
				Attestation2: MockIndexedAttestation(),
			},
		},
		Attestations: []*v1.Attestation{
			MockAttestation(),
		},
		Deposits: []*v1.Deposit{
			{
				Proof: []string{"0x41"},
				Data: &v1.DepositData{
					PublicKey:             make([]byte, dilithium2.CryptoPublicKeyBytes),
					WithdrawalCredentials: make([]byte, 32),
					Amount:                "0",
					Signature:             make([]byte, dilithium2.CryptoBytes),
				},
			},
		},
		VoluntaryExits: []*v1.SignedVoluntaryExit{
			{
				Message: &v1.VoluntaryExit{
					Epoch:          "0",
					ValidatorIndex: "0",
				},
				Signature: make([]byte, dilithium2.CryptoBytes),
			},
		},
	}
}

func MockContributionAndProof() *v1.ContributionAndProof {
	return &v1.ContributionAndProof{
		AggregatorIndex: "0",
		Contribution: &v1.SyncCommitteeContribution{
			Slot:              "0",
			BeaconBlockRoot:   make([]byte, fieldparams.RootLength),
			SubcommitteeIndex: "0",
			ParticipationBits: MockAggregationBits(),
			Signatures:        []hexutil.Bytes{},
		},
		SelectionProof: make([]byte, dilithium2.CryptoBytes),
	}
}
*/
