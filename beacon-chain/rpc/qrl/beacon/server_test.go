package beacon

import zondpbservice "github.com/theQRL/qrysm/proto/qrl/service"

var _ zondpbservice.BeaconChainServer = (*Server)(nil)
