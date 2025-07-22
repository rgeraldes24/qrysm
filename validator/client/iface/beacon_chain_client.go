package iface

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

type BeaconChainClient interface {
	GetChainHead(ctx context.Context, in *empty.Empty) (*qrysmpb.ChainHead, error)
	ListValidatorBalances(ctx context.Context, in *qrysmpb.ListValidatorBalancesRequest) (*qrysmpb.ValidatorBalances, error)
	ListValidators(ctx context.Context, in *qrysmpb.ListValidatorsRequest) (*qrysmpb.Validators, error)
	GetValidatorPerformance(ctx context.Context, in *qrysmpb.ValidatorPerformanceRequest) (*qrysmpb.ValidatorPerformanceResponse, error)
}
