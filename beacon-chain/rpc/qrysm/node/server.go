package node

import (
	"github.com/theQRL/qrysm/beacon-chain/blockchain"
	"github.com/theQRL/qrysm/beacon-chain/p2p"
)

type Server struct {
	OptimisticModeFetcher blockchain.OptimisticModeFetcher
	PeersFetcher          p2p.PeersProvider
	PeerManager           p2p.PeerManager
	MetadataProvider      p2p.MetadataProvider
	HeadFetcher           blockchain.HeadFetcher
}
