package validator

import "github.com/theQRL/qrysm/v4/beacon-chain/rpc/zond/shared"

type AggregateAttestationResponse struct {
	Data *shared.Attestation `json:"data"`
}

type SubmitContributionAndProofsRequest struct {
	Data []*shared.SignedContributionAndProof `json:"data" validate:"required"`
}

type SubmitAggregateAndProofsRequest struct {
	Data []*shared.SignedAggregateAttestationAndProof `json:"data" validate:"required"`
}
