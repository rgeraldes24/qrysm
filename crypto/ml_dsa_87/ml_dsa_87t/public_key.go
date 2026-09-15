package ml_dsa_87t

import (
	"fmt"
	"reflect"

	"github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87/common"
)

type PublicKey struct {
	p *ml_dsa_87.PK
}

func (p *PublicKey) Marshal() []byte {
	return p.p[:]
}

// PublicKeyFromBytes returns a public key that owns its bytes. Parsing only
// checks the length and copies the bytes; it does not decompress them. Keys
// must not be retained globally here, since callers may supply keys from
// deposits that later fail validation.
func PublicKeyFromBytes(pubKey []byte) (common.PublicKey, error) {
	if len(pubKey) != field_params.MLDSA87PubkeyLength {
		return nil, fmt.Errorf("public key must be %d bytes", field_params.MLDSA87PubkeyLength)
	}
	var p ml_dsa_87.PK
	copy(p[:], pubKey)
	return &PublicKey{p: &p}, nil
}

func (p *PublicKey) Copy() common.PublicKey {
	np := *p.p
	return &PublicKey{p: &np}
}

func (p *PublicKey) Equals(p2 common.PublicKey) bool {
	return reflect.DeepEqual(p.p, p2.(*PublicKey).p)
}
