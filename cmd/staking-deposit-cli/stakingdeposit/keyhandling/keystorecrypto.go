package keyhandling

import (
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/misc"
)

type KeystoreCrypto struct {
	KDF    *KeystoreModule `json:"kdf"`
	Cipher *KeystoreModule `json:"cipher"`
}

func NewKeystoreCrypto(salt, aesIV, cipherText []uint8) *KeystoreCrypto {
	return &KeystoreCrypto{
		KDF: &KeystoreModule{
			Function: "custom",
			Params:   map[string]interface{}{"salt": misc.EncodeHex(salt)},
		},
		Cipher: &KeystoreModule{
			Function: "aes-256-gcm",
			Params:   map[string]interface{}{"iv": misc.EncodeHex(aesIV)},
			Message:  misc.EncodeHex(cipherText),
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
	}
}
