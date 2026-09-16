package ml_dsa_87t

import (
	"errors"
	"fmt"

	cryptoerrors "github.com/theQRL/go-qrllib/crypto/errors"
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
// infinity public key.
//
// The FIPS 204 primitive (go-qrllib crypto/ml_dsa_87.Verify) accepts such keys
// by design, so that it stays conformant with the Wycheproof ZeroPublicKey
// vectors; rejection is a separate key-validation step, ValidatePublicKey,
// which go-qrllib's wallet-level Verify applies on every call. Rejecting the
// key here at parse time as well keeps it out of signature batches,
// seen-caches and error paths that assume a parsed key is a real key.
//
// It wraps go-qrllib's sentinel, so errors.Is works against either.
var ErrZeroT1PublicKey = fmt.Errorf("%w and is universally forgeable", cryptoerrors.ErrZeroT1PublicKey)

type PublicKey struct {
	p *ml_dsa_87.PK
}

// Marshal returns the public key bytes, or nil for a nil or uninitialized key
// so that callers never dereference a missing value.
func (p *PublicKey) Marshal() []byte {
	if p == nil || p.p == nil {
		return nil
	}
	return p.p[:]
}

// PublicKeyFromBytes returns a public key that owns its bytes. Parsing checks
// the length, runs go-qrllib's key validation (which today rejects only the
// universally forgeable all-zero-t1 key), and copies the bytes; every other
// byte string of the right length is a well-formed ML-DSA-87 public key. Keys
// must not be retained globally here, since callers may supply keys from
// deposits that later fail validation.
func PublicKeyFromBytes(pubKey []byte) (common.PublicKey, error) {
	if len(pubKey) != field_params.MLDSA87PubkeyLength {
		return nil, fmt.Errorf("public key must be %d bytes", field_params.MLDSA87PubkeyLength)
	}
	var p ml_dsa_87.PK
	copy(p[:], pubKey)
	if err := cryptomldsa87.ValidatePublicKey((*[cryptomldsa87.CRYPTO_PUBLIC_KEY_BYTES]uint8)(&p)); err != nil {
		if errors.Is(err, cryptoerrors.ErrZeroT1PublicKey) {
			return nil, ErrZeroT1PublicKey
		}
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	return &PublicKey{p: &p}, nil
}

func (p *PublicKey) Copy() common.PublicKey {
	if p == nil || p.p == nil {
		return nil
	}
	np := *p.p
	return &PublicKey{p: &np}
}

// Equals reports whether p2 is an initialized ML-DSA-87 public key with the
// same bytes. A nil, uninitialized, or foreign implementation never compares
// equal, so callers cannot be tricked into deduplicating against it.
func (p *PublicKey) Equals(p2 common.PublicKey) bool {
	other, ok := p2.(*PublicKey)
	if !ok || p == nil || other == nil || p.p == nil || other.p == nil {
		return false
	}
	return *p.p == *other.p
}
