//go:build !noMainnetGenesis
// +build !noMainnetGenesis

package genesis

import (
	_ "embed"

	"github.com/theQRL/qrysm/v4/config/params"
)

// TODO(rgeraldes24): review

var (
	// TODO(rgeraldes24): add final mainnet genesis
	// deposit new-seed --num-validators=64 --chain-name=mainnet
	//go:embed mainnet.ssz.snappy
	mainnetRawSSZCompressed []byte // 2.8Mb
)

func init() {
	embeddedStates[params.MainnetName] = &mainnetRawSSZCompressed
}
