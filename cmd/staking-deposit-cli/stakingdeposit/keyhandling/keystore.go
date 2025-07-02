package keyhandling

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/google/uuid"
	"github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/misc"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"golang.org/x/crypto/sha3"
)

type Keystore struct {
	Crypto      *KeystoreCrypto `json:"crypto"`
	Description string          `json:"description"`
	PubKey      string          `json:"pubkey"`
	Path        string          `json:"path"`
	UUID        string          `json:"uuid"`
	Version     uint64          `json:"version"`
}

func (k *Keystore) ToJSON() []byte {
	b, err := json.Marshal(k)
	if err != nil {
		panic("failed to marshal keystore to json")
	}
	return b
}

func (k *Keystore) Save(fileFolder string) error {
	f, err := os.Create(fileFolder)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err)
		}
	}()
	if _, err := f.Write(k.ToJSON()); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(fileFolder, 0440); err != nil {
			return err
		}
	}
	return nil
}

// TODO(rgeraldes24)
func (k *Keystore) Decrypt(password string) [field_params.DilithiumSeedLength]byte {
	salt, ok := k.Crypto.KDF.Params["salt"]
	if !ok {
		panic("salt not found in KDF Params")
	}
	binSalt := misc.DecodeHex(salt.(string))

	decryptionKey, err := passwordToDecryptionKey(password, binSalt)
	if err != nil {
		panic(fmt.Errorf("passwordToDecryptionKey | reason %v", err))
	}

	block, err := aes.NewCipher(decryptionKey[:])
	if err != nil {
		panic(fmt.Errorf("aes.NewCipher failed | reason %v", err))
	}

	cipherText := misc.DecodeHex(k.Crypto.Cipher.Message)
	aesIV, ok := k.Crypto.Cipher.Params["iv"]
	if !ok {
		panic(fmt.Errorf("aesIV not found in Cipher Params"))
	}
	binAESIV := misc.DecodeHex(aesIV.(string))

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	plainText, err := aesgcm.Open(nil, binAESIV, cipherText, nil)
	if err != nil {
		panic(err)
	}

	return [field_params.DilithiumSeedLength]byte(plainText)
}

func NewKeystoreFromJSON(data []uint8) *Keystore {
	k := NewEmptyKeystore()
	err := json.Unmarshal(data, k)
	if err != nil {
		panic(fmt.Errorf("failed to marshal keystore to json | reason %v", err))
	}
	return k
}

func NewKeystoreFromFile(path string) *Keystore {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("cannot read file %s | reason %v", path, err))
	}
	return NewKeystoreFromJSON(data)
}

func NewEmptyKeystore() *Keystore {
	k := &Keystore{}
	k.Crypto = NewEmptyKeystoreCrypto()
	return k
}

func Encrypt(seed [field_params.DilithiumSeedLength]uint8, password, path string, salt, aesIV []byte) (*Keystore, error) {
	if salt == nil {
		salt = make([]uint8, 32)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return nil, err
		}
	}
	if aesIV == nil {
		aesIV = make([]uint8, 12)
		if _, err := io.ReadFull(rand.Reader, aesIV); err != nil {
			return nil, err
		}
	}

	decryptionKey, err := passwordToDecryptionKey(password, salt)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(decryptionKey[:])
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	cipherText := aesgcm.Seal(nil, aesIV, seed[:], nil)

	d, err := dilithium.NewDilithiumFromSeed(seed)
	if err != nil {
		return nil, err
	}
	pk := d.GetPK()
	return &Keystore{
		UUID:   uuid.New().String(),
		Crypto: NewKeystoreCrypto(salt, aesIV, cipherText),
		PubKey: misc.EncodeHex(pk[:]),
		Path:   path,
	}, nil
}

// TODO(rgeraldes24): remove
func passwordToDecryptionKey(password string, salt []byte) ([32]byte, error) {
	h := sha3.NewShake256()
	if _, err := h.Write([]byte(password)); err != nil {
		return [32]byte{}, fmt.Errorf("shake256 hash write failed %v", err)
	}

	if _, err := h.Write(salt); err != nil {
		return [32]byte{}, fmt.Errorf("shake256 hash write failed %v", err)
	}

	var decryptionKey [32]uint8
	_, err := h.Read(decryptionKey[:])
	return decryptionKey, err
}
