package keyhandling

import (
	"encoding/hex"
)

const (
	// Algorithms.
	algoArgon2id = "argon2id"

	// Ciphers.
	cipherAes256Gcm = "aes-256-gcm"
)

type KeystoreCrypto struct {
	KDF      *KeystoreModule `json:"kdf"`
	Checksum *KeystoreModule `json:"checksum"`
	Cipher   *KeystoreModule `json:"cipher"`
}

func NewKeystoreCrypto(salt, aesIV, cipherText []uint8, argon2idT, argon2idM uint32, argon2idP uint8, dklen uint32) *KeystoreCrypto {
	return &KeystoreCrypto{
		KDF: &KeystoreModule{
			Function: algoArgon2id,
			Params: map[string]interface{}{
				"dklen": dklen,
				"m":     argon2idM,
				"p":     argon2idP,
				"salt":  hex.EncodeToString(salt),
				"t":     argon2idT,
			},
		},
		Cipher: &KeystoreModule{
			Function: cipherAes256Gcm,
			Params:   map[string]interface{}{"iv": hex.EncodeToString(aesIV)},
			Message:  hex.EncodeToString(cipherText),
		},
	}
}

func NewEmptyKeystoreCrypto() *KeystoreCrypto {
	return &KeystoreCrypto{
		KDF: &KeystoreModule{
			Params: map[string]interface{}{},
		},
		Cipher: &KeystoreModule{
			Params: map[string]interface{}{},
		},
		Checksum: &KeystoreModule{
			Params: map[string]interface{}{},
		},
	}
}
