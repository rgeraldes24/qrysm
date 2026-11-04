package interop

import (
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/qrysm/testing/require"
)

func TestDefaultGenesisAddresses(t *testing.T) {
	require.Equal(t, common.Address{}, defaultCoinbase)
	require.NotEqual(t, common.Address{}, defaultTestAccountAddress)
	require.Equal(t, "qaf84bc06703edfc371a0177ac8b482622d5ad24204145f01746cb381dcd546c53b8825839cc61bfc1fc3d78bc560c7bb7a9895432e1e87435474a1bc5a2e1200", strings.ToLower(defaultTestAccountAddress.Hex()))
}

func TestMustAddress_PanicsOnMalformedLiteral(t *testing.T) {
	defer func() {
		require.NotNil(t, recover(), "expected a panic for a 48-byte address literal")
	}()
	mustAddress("Q000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
}
