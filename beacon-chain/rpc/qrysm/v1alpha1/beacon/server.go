// Package beacon defines a gRPC beacon service implementation, providing
// useful endpoints for checking fetching chain-specific data such as
// blocks, committees, validators, assignments, and more.
package beacon

import (
	"github.com/theQRL/qrysm/beacon-chain/blockchain"
	"github.com/theQRL/qrysm/beacon-chain/cache"
	"github.com/theQRL/qrysm/beacon-chain/db"
	"github.com/theQRL/qrysm/beacon-chain/execution"
	"github.com/theQRL/qrysm/beacon-chain/operations/attestations"
	"github.com/theQRL/qrysm/beacon-chain/operations/slashings"
	"github.com/theQRL/qrysm/beacon-chain/p2p"
	"github.com/theQRL/qrysm/beacon-chain/rpc/core"
	"github.com/theQRL/qrysm/beacon-chain/state/stategen"
)

// Server defines a server implementation of the gRPC Beacon Chain service,
// providing RPC endpoints to access data relevant to the QRL beacon chain.
type Server struct {
	BeaconDB              db.ReadOnlyDatabase
	ChainStartFetcher     execution.ChainStartFetcher
	HeadFetcher           blockchain.HeadFetcher
	CanonicalFetcher      blockchain.CanonicalFetcher
	FinalizationFetcher   blockchain.FinalizationFetcher
	DepositFetcher        cache.DepositFetcher
	GenesisTimeFetcher    blockchain.TimeFetcher
	Broadcaster           p2p.Broadcaster
	AttestationsPool      attestations.Pool
	SlashingsPool         slashings.PoolManager
	StateGen              stategen.StateManager
	ReplayerBuilder       stategen.ReplayerBuilder
	OptimisticModeFetcher blockchain.OptimisticModeFetcher
	CoreService           *core.Service
}
