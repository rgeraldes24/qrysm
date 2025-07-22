package grpc_api

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/validator/client/iface"
	"google.golang.org/grpc"
)

type grpcNodeClient struct {
	nodeClient qrysmpb.NodeClient
}

func (c *grpcNodeClient) GetSyncStatus(ctx context.Context, in *empty.Empty) (*qrysmpb.SyncStatus, error) {
	return c.nodeClient.GetSyncStatus(ctx, in)
}

func (c *grpcNodeClient) GetGenesis(ctx context.Context, in *empty.Empty) (*qrysmpb.Genesis, error) {
	return c.nodeClient.GetGenesis(ctx, in)
}

func (c *grpcNodeClient) GetVersion(ctx context.Context, in *empty.Empty) (*qrysmpb.Version, error) {
	return c.nodeClient.GetVersion(ctx, in)
}

func (c *grpcNodeClient) ListPeers(ctx context.Context, in *empty.Empty) (*qrysmpb.Peers, error) {
	return c.nodeClient.ListPeers(ctx, in)
}

func NewNodeClient(cc grpc.ClientConnInterface) iface.NodeClient {
	return &grpcNodeClient{qrysmpb.NewNodeClient(cc)}
}
