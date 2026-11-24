package client

import (
	"context"
	"time"

	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/validator/client/iface"
)

type dutyDependentRootProvider interface {
	GetDutyDependentRoots(ctx context.Context, epoch primitives.Epoch) ([]byte, []byte, error)
}

// validatorClientDutyDependentRootsProvider reads the duty dependent roots
// from a GetDuties call without public keys on the validator client's own
// connection, so the roots come from the same beacon node as the duties.
type validatorClientDutyDependentRootsProvider struct {
	validatorClient iface.ValidatorClient
	timeout         time.Duration
}

func (p *validatorClientDutyDependentRootsProvider) GetDutyDependentRoots(ctx context.Context, epoch primitives.Epoch) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	resp, err := p.validatorClient.GetDuties(ctx, &qrysmpb.DutiesRequest{Epoch: epoch})
	if err != nil {
		return nil, nil, err
	}
	return resp.GetPreviousDutyDependentRoot(), resp.GetCurrentDutyDependentRoot(), nil
}
