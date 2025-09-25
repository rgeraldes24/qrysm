package ml_dsa_87t

import (
	"fmt"

	"github.com/theQRL/qrysm/cache/nonblocking"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87/common"
)

func init() {
	onEvict := func(_ [field_params.MLDSA87PubkeyLength]byte, _ common.PublicKey) {}
	keysCache, err := nonblocking.NewLRU(maxKeys, onEvict)
	if err != nil {
		panic(fmt.Sprintf("Could not initiate public keys cache: %v", err))
	}
	pubkeyCache = keysCache
}
