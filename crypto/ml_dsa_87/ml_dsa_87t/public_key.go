package ml_dsa_87t

import (
	"errors"
	"fmt"
	"reflect"

	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
	"github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87/common"
)

// ErrZeroT1PublicKey is returned for a public key whose t1 component (every
// byte after the 32-byte rho seed) is all zero. Such a key is universally
// forgeable: with t1 = 0 the verifier's reconstructed commitment no longer
// depends on the challenge, so (c~ = H(mu || w1Encode(0)), z = 0, h = 0) is a
// valid signature for any message. This is the ML-DSA analogue of the BLS
// infinity public key. go-qrllib rejects such keys at verify time; rejecting
// them at parse time keeps them out of signature batches, seen-caches and
// error paths that assume a parsed key is a real key.
var ErrZeroT1PublicKey = errors.New("public key t1 is all zero and is universally forgeable")

type PublicKey struct {
	p *ml_dsa_87.PK
}

func (p *PublicKey) Marshal() []byte {
	return p.p[:]
}

// PublicKeyFromBytes returns a public key that owns its bytes. Parsing checks
// the length, rejects the universally forgeable all-zero-t1 key, and copies
// the bytes; it does not otherwise validate the key (every other byte string
// of the right length is a well-formed ML-DSA-87 public key). Keys must not
// be retained globally here, since callers may supply keys from deposits that
// later fail validation.
func PublicKeyFromBytes(pubKey []byte) (common.PublicKey, error) {
	if len(pubKey) != field_params.MLDSA87PubkeyLength {
		return nil, fmt.Errorf("public key must be %d bytes", field_params.MLDSA87PubkeyLength)
	}
	if hasZeroT1(pubKey) {
		return nil, ErrZeroT1PublicKey
	}
	var p ml_dsa_87.PK
	copy(p[:], pubKey)
	return &PublicKey{p: &p}, nil
}

// hasZeroT1 reports whether the t1 region of a packed ML-DSA-87 public key
// (everything after the rho seed) is all zero. The caller guarantees that
// pubKey has the full public key length.
func hasZeroT1(pubKey []byte) bool {
	var acc byte
	for _, b := range pubKey[cryptomldsa87.SEED_BYTES:] {
		acc |= b
	}
	return acc == 0
}

func (p *PublicKey) Copy() common.PublicKey {
	if p == nil || p.p == nil {
		return nil
	}
	np := *p.p
	return &PublicKey{p: &np}
}

func (p *PublicKey) Equals(p2 common.PublicKey) bool {
	return reflect.DeepEqual(p.p, p2.(*PublicKey).p)
}
