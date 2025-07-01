// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// Modified by Prysmatic Labs 2018
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package keystore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/crypto/dilithium"
	"golang.org/x/crypto/argon2"
)

// Keystore defines a keystore with a directory path and argon2id values.
type Keystore struct {
	keysDirPath          string
	argon2idT, argon2idM uint32
	argon2idP            uint8
}

// GetKey from file using the filename path and a decryption password.
func (_ Keystore) GetKey(filename, password string) (*Key, error) {
	// Load the key from the keystore and decrypt its contents
	keyJSON, err := os.ReadFile(filename) // #nosec G304 -- ReadFile is safe
	if err != nil {
		return nil, err
	}
	return DecryptKey(keyJSON, password)
}

// GetKeys from directory using the prefix to filter relevant files
// and a decryption password.
func (_ Keystore) GetKeys(directory, filePrefix, password string, warnOnFail bool) (map[string]*Key, error) {
	// Load the key from the keystore and decrypt its contents
	// #nosec G304
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]*Key)
	for _, f := range files {
		n := f.Name()
		filePath := filepath.Join(directory, n)
		filePath = filepath.Clean(filePath)
		if f.Type()&os.ModeSymlink == os.ModeSymlink {
			if targetFilePath, err := filepath.EvalSymlinks(filePath); err == nil {
				filePath = targetFilePath
				// Override link stats with target file's stats.
				dirEntry, err := os.Stat(filePath)
				if err != nil {
					return nil, err
				}
				f = fs.FileInfoToDirEntry(dirEntry)
			}
		}
		cp := strings.Contains(n, strings.TrimPrefix(filePrefix, "/"))
		if f.Type().IsRegular() && cp {
			// #nosec G304
			keyJSON, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			key, err := DecryptKey(keyJSON, password)
			if err != nil {
				if warnOnFail {
					log.WithError(err).WithField("keyfile", string(keyJSON)).Warn("Failed to decrypt key")
				}
				continue
			}
			keys[hex.EncodeToString(key.PublicKey.Marshal())] = key
		}
	}
	return keys, nil
}

// StoreKey in filepath and encrypt it with a password.
func (ks Keystore) StoreKey(filename string, key *Key, auth string) error {
	keyJSON, err := EncryptKey(key, auth, ks.argon2idT, ks.argon2idM, ks.argon2idP)
	if err != nil {
		return err
	}
	return writeKeyFile(filename, keyJSON)
}

// JoinPath joins the filename with the keystore directory path.
func (ks Keystore) JoinPath(filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(ks.keysDirPath, filename)
}

// EncryptKey encrypts a key using the specified argon2id parameters into a JSON
// blob that can be decrypted later on.
func EncryptKey(key *Key, password string, argon2idT, argon2idM uint32, argon2idP uint8) ([]byte, error) {
	authArray := []byte(password)
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		panic("reading from crypto/rand failed: " + err.Error())
	}

	derivedKey := argon2.IDKey(authArray, salt, argon2idT, argon2idM, argon2idP, argon2idDKLen)

	keyBytes := key.SecretKey.Marshal()

	iv := make([]byte, GCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, errors.New("reading from crypto/rand failed: " + err.Error())
	}

	cipherText, err := encryptGCM(nil, derivedKey, iv, keyBytes, nil)
	if err != nil {
		return nil, err
	}

	argon2idParamsJSON := make(map[string]interface{}, 5)
	argon2idParamsJSON["t"] = argon2idT
	argon2idParamsJSON["m"] = argon2idM
	argon2idParamsJSON["p"] = argon2idP
	argon2idParamsJSON["dklen"] = argon2idDKLen
	argon2idParamsJSON["salt"] = hex.EncodeToString(salt)

	cipherParamsJSON := cipherparamsJSON{
		IV: hex.EncodeToString(iv),
	}

	cryptoStruct := cryptoJSON{
		Cipher:       "aes-256-gcm",
		CipherText:   hex.EncodeToString(cipherText),
		CipherParams: cipherParamsJSON,
		KDF:          keyHeaderKDF,
		KDFParams:    argon2idParamsJSON,
	}
	encryptedJSON := encryptedKeyJSON{
		hex.EncodeToString(key.PublicKey.Marshal()),
		cryptoStruct,
		key.ID.String(),
	}
	return json.Marshal(encryptedJSON)
}

// DecryptKey decrypts a key from a JSON blob, returning the private key itself.
func DecryptKey(keyJSON []byte, password string) (*Key, error) {
	var keyBytes, keyID []byte
	var err error

	k := new(encryptedKeyJSON)
	if err := json.Unmarshal(keyJSON, k); err != nil {
		return nil, err
	}

	keyBytes, keyID, err = decryptKeyJSON(k, password)
	// Handle any decryption errors and return the key
	if err != nil {
		return nil, err
	}

	secretKey, err := dilithium.SecretKeyFromSeed(keyBytes)
	if err != nil {
		return nil, err
	}

	return &Key{
		ID:        keyID,
		PublicKey: secretKey.PublicKey(),
		SecretKey: secretKey,
	}, nil
}

func decryptKeyJSON(keyProtected *encryptedKeyJSON, auth string) (keyBytes, keyID []byte, err error) {
	keyID = uuid.Parse(keyProtected.ID)
	if keyProtected.Crypto.Cipher != "aes-256-gcm" {
		return nil, nil, fmt.Errorf("cipher not supported: %v", keyProtected.Crypto.Cipher)
	}

	iv, err := hex.DecodeString(keyProtected.Crypto.CipherParams.IV)
	if err != nil {
		return nil, nil, err
	}

	cipherText, err := hex.DecodeString(keyProtected.Crypto.CipherText)
	if err != nil {
		return nil, nil, err
	}

	derivedKey, err := kdfKey(keyProtected.Crypto, auth)
	if err != nil {
		return nil, nil, err
	}

	plainText, err := decryptGCM(derivedKey, iv, cipherText, nil)
	if err != nil {
		return nil, nil, err
	}

	return plainText, keyID, nil
}

func kdfKey(cryptoJSON cryptoJSON, auth string) ([]byte, error) {
	authArray := []byte(auth)
	salt, err := hex.DecodeString(cryptoJSON.KDFParams["salt"].(string))
	if err != nil {
		return nil, err
	}
	dkLen := uint32(ensureInt(cryptoJSON.KDFParams["dklen"]))

	if cryptoJSON.KDF == keyHeaderKDF {
		t := uint32(ensureInt(cryptoJSON.KDFParams["t"]))
		m := uint32(ensureInt(cryptoJSON.KDFParams["m"]))
		p := uint8(ensureInt(cryptoJSON.KDFParams["p"]))
		return argon2.IDKey(authArray, salt, t, m, p, dkLen), nil
	}

	return nil, fmt.Errorf("unsupported KDF: %s", cryptoJSON.KDF)
}
