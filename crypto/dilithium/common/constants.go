package common

import (
	field_params "github.com/theQRL/qrysm/v4/config/fieldparams"
)

// ZeroSecretKey represents a zero secret key.
var ZeroSecretKey = [32]byte{}

// InfinitePublicKey represents an infinite public key (G1 Point at Infinity).
var InfinitePublicKey = [field_params.DilithiumPubkeyLength]byte{0xC0}

// InfiniteSignature represents an infinite signature (G2 Point at Infinity).
var InfiniteSignature = [field_params.DilithiumSignatureLength]byte{0xC0}
