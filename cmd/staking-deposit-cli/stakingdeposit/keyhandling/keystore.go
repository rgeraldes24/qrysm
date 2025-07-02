package keyhandling

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/google/uuid"
	"github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/go-zond/accounts/keystore"
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/misc"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/encoding/bytesutil"
)

type Keystore struct {
	Crypto      keystore.CryptoJSON `json:"crypto"`
	Description string              `json:"description"`
	PubKey      string              `json:"pubkey"`
	Path        string              `json:"path"`
	UUID        string              `json:"uuid"`
	Version     uint64              `json:"version"`
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

func (k *Keystore) Decrypt(password string) [field_params.DilithiumSeedLength]byte {
	seed, err := keystore.DecryptDataV1(k.Crypto, password)
	if err != nil {
		panic(fmt.Errorf("keystore.DecryptDataV1 failed | reason %v", err))
	}
	return bytesutil.ToBytes48(seed)
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

func NewEmptyCryptoJSON() keystore.CryptoJSON {
	return keystore.CryptoJSON{
		KDFParams: map[string]interface{}{},
	}
}

func NewEmptyKeystore() *Keystore {
	k := &Keystore{}
	k.Crypto = NewEmptyCryptoJSON()
	return k
}

func Encrypt(seed [field_params.DilithiumSeedLength]uint8, password, path string) (*Keystore, error) {
	cjson, err := keystore.EncryptDataV1(seed[:], []byte(password), keystore.StandardArgon2idT, keystore.StandardArgon2idM, keystore.StandardArgon2idP)
	if err != nil {
		return nil, err
	}

	d, err := dilithium.NewDilithiumFromSeed(seed)
	if err != nil {
		return nil, err
	}
	pk := d.GetPK()
	return &Keystore{
		UUID:   uuid.New().String(),
		Crypto: cjson,
		PubKey: misc.EncodeHex(pk[:]),
		Path:   path,
	}, nil
}
