package types

type Encryptor interface {
	Name() string

	Version() uint

	Encrypt(data []byte, key string) (map[string]interface{}, error)

	Decrypt(data map[string]interface{}, key string) ([]byte, error)
}
