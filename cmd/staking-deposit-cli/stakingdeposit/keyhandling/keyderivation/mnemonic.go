package keyderivation

import (
	"math/rand"
	"strings"

	"github.com/theQRL/go-qrllib/qrl"
	"github.com/theQRL/go-qrllib/wallet/common/descriptor"
	"github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
)

func descriptorToMnemonicWords(desc descriptor.Descriptor) []string {
	descBytes := desc.ToBytes()
	val := uint32(descBytes[0])<<16 | uint32(descBytes[1])<<8 | uint32(descBytes[2])
	high := (val >> 12) & 0xFFF
	low := val & 0xFFF
	return []string{
		qrl.WordList[high],
		qrl.WordList[low],
	}
}

func GetRandomMnemonic() string {
	descriptorWords := descriptorToMnemonicWords(ml_dsa_87.NewMLDSA87Descriptor().ToDescriptor())

	wordList := append([]string{}, qrl.WordList[:]...)
	rand.Shuffle(len(wordList), func(i, j int) {
		wordList[i], wordList[j] = wordList[j], wordList[i]
	})
	randomWords := wordList[:32]

	return strings.Join(append(descriptorWords, randomWords...), " ")
}
