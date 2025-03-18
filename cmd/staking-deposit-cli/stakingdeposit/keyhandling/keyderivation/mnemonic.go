package keyderivation

import (
	"math/rand"
	"strings"

	"github.com/theQRL/go-qrllib/qrl"
)

func GetRandomMnemonic() string {
	wordList := append([]string{}, qrl.WordList[:]...)
	// Randomly shuffles the word list
	rand.Shuffle(len(wordList), func(i, j int) {
		wordList[i], wordList[j] = wordList[j], wordList[i]
	})
	return strings.Join(wordList[:32], " ")
}
