package ml_dsa_87t

import (
	"fmt"
	"reflect"

	"github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
	"github.com/theQRL/qrysm/cache/nonblocking"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87/common"
)

var maxKeys = 2_000_000
var pubkeyCache *nonblocking.LRU[[field_params.MLDSA87PubkeyLength]byte, common.PublicKey]

type PublicKey struct {
	p *ml_dsa_87.PK
}

func (p *PublicKey) Marshal() []byte {
	return p.p[:]
}

func PublicKeyFromBytes(pubKey []byte) (common.PublicKey, error) {
	return publicKeyFromBytes(pubKey, true)
}

func publicKeyFromBytes(pubKey []byte, cacheCopy bool) (common.PublicKey, error) {
	if len(pubKey) != field_params.MLDSA87PubkeyLength {
		return nil, fmt.Errorf("public key must be %d bytes", field_params.MLDSA87PubkeyLength)
	}
	newKey := (*[field_params.MLDSA87PubkeyLength]uint8)(pubKey)
	if cv, ok := pubkeyCache.Get(*newKey); ok {
		if cacheCopy {
			return cv.(*PublicKey).Copy(), nil
		}
		return cv.(*PublicKey), nil
	}
	var p ml_dsa_87.PK
	copy(p[:], pubKey)
	pubKeyObj := &PublicKey{p: &p}
	copiedKey := pubKeyObj.Copy()
	cacheKey := *newKey
	pubkeyCache.Add(cacheKey, copiedKey)
	return pubKeyObj, nil
}

func (p *PublicKey) Copy() common.PublicKey {
	np := *p.p
	return &PublicKey{p: &np}
}

func (p *PublicKey) Equals(p2 common.PublicKey) bool {
	return reflect.DeepEqual(p.p, p2.(*PublicKey).p)
}
