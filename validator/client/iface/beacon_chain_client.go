package iface

import (
	"context"

	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

type BeaconChainClient interface {
	ListValidatorBalances(ctx context.Context, in *zondpb.ListValidatorBalancesRequest) (*zondpb.ValidatorBalances, error)
	ListValidators(ctx context.Context, in *zondpb.ListValidatorsRequest) (*zondpb.Validators, error)
	GetValidatorPerformance(ctx context.Context, in *zondpb.ValidatorPerformanceRequest) (*zondpb.ValidatorPerformanceResponse, error)
}
