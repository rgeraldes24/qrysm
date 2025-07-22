package node

import (
	zondpbservice "github.com/theQRL/qrysm/proto/qrl/service"
)

var _ zondpbservice.BeaconNodeServer = (*Server)(nil)
