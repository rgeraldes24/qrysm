package iface

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

type NodeClient interface {
	GetSyncStatus(ctx context.Context, in *empty.Empty) (*qrysmpb.SyncStatus, error)
	GetGenesis(ctx context.Context, in *empty.Empty) (*qrysmpb.Genesis, error)
	GetVersion(ctx context.Context, in *empty.Empty) (*qrysmpb.Version, error)
	ListPeers(ctx context.Context, in *empty.Empty) (*qrysmpb.Peers, error)
}
