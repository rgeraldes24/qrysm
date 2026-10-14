package misc

import (
	"encoding/hex"
	"fmt"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
)

func StrSeedToBinSeed(strSeed string) [fieldparams.MLDSA87SeedLength]uint8 {
	var seed [fieldparams.MLDSA87SeedLength]uint8

	unSizedSeed := DecodeHex(strSeed)

	copy(seed[:], unSizedSeed)
	return seed
}

func DecodeHex(hexString string) []byte {
	if hexString[:2] == "0x" {
		hexString = hexString[2:]
	}
	hexBytes, err := hex.DecodeString(hexString)
	if err != nil {
		panic(fmt.Errorf("failed to decode string %s | reason %v",
			hexString, err))
	}
	return hexBytes
}

func EncodeHex(hexBytes []byte) string {
	return fmt.Sprintf("0x%x", hexBytes)
}
