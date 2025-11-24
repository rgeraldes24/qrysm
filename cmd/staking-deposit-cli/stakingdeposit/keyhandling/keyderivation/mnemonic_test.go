package keyderivation

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theQRL/go-qrllib/wallet/ml_dsa_87"
)

func TestDescriptorToMnemonicWords(t *testing.T) {
	descriptor := ml_dsa_87.NewMLDSA87Descriptor().ToDescriptor()
	mnemonic := descriptorToMnemonicWords(descriptor)
	require.Len(t, mnemonic, 2)
	require.True(t, slices.Equal([]string{"absorb", "aback"}, mnemonic))
}

func TestGetRandomMnemonic(t *testing.T) {
	mnemonic := GetRandomMnemonic()
	require.Len(t, strings.Split(mnemonic, " "), 34)
	require.True(t, strings.HasPrefix(mnemonic, "absorb aback"))
}
