package deposit_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
	"github.com/theQRL/qrysm/contracts/deposit"
	"github.com/theQRL/qrysm/testing/require"
)

// TestDepositLogEmitterCode_GetDepositCountParseable calls get_deposit_count
// through the generated QRVM bindings (no native contract registered for the
// address) against an account holding only the log-emitter code. The beacon
// node performs this exact qrl_call in processPastLogs, so the emitter's
// return data must unpack as an ABI-encoded 8-byte little-endian count.
func TestDepositLogEmitterCode_GetDepositCountParseable(t *testing.T) {
	w, err := wallet.Generate(wallet.ML_DSA_87)
	require.NoError(t, err)
	emitterAddr := common.BytesToAddress([]byte("emitter-only-deposit-contract"))
	genesis := core.GenesisAlloc{
		w.GetAddress(): {Balance: big.NewInt(1e18)},
		emitterAddr:    {Balance: new(big.Int), Code: deposit.DepositLogEmitterCode()},
	}
	backend := backends.NewSimulatedBackend(genesis, 20000000)
	backend.Commit()

	caller, err := deposit.NewDepositContractCaller(emitterAddr, backend)
	require.NoError(t, err)
	count, err := caller.GetDepositCount(&bind.CallOpts{Context: context.Background()})
	require.NoError(t, err)
	require.DeepEqual(t, make([]byte, 8), count)
}
