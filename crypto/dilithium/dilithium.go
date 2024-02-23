package dilithium

import (
	"github.com/theQRL/qrysm/v4/crypto/dilithium/common"
	"github.com/theQRL/qrysm/v4/crypto/dilithium/dilithiumt"
)

func SecretKeyFromSeed(seed []byte) (DilithiumKey, error) {
	return dilithiumt.SecretKeyFromSeed(seed)
}

func PublicKeyFromBytes(pubKey []byte) (PublicKey, error) {
	return dilithiumt.PublicKeyFromBytes(pubKey)
}

func SignatureFromBytes(sig []byte) (Signature, error) {
	return dilithiumt.SignatureFromBytes(sig)
}

func VerifySignature(sig []byte, msg [32]byte, pubKey common.PublicKey) (bool, error) {
	return dilithiumt.VerifySignature(sig, msg, pubKey)
}

func VerifyMultipleSignatures(sigs [][][]byte, msgs [][32]byte, pubKeys [][]common.PublicKey) (bool, error) {
	return dilithiumt.VerifyMultipleSignatures(sigs, msgs, pubKeys)
}

func RandKey() (common.SecretKey, error) {
	return dilithiumt.RandKey()
}
