package interop

import (
	"math"
	"math/big"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/params"
	clparams "github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/contracts/deposit"
)

// defaultMinerAddress is used to send deposits and test transactions in the e2e test.
// This account is given a large initial balance in the genesis block in test setups.
const defaultTestChainId int64 = 1337
const defaultMixhash = "0x0000000000000000000000000000000000000000000000000000000000000000"
const defaultParenthash = "0x0000000000000000000000000000000000000000000000000000000000000000"
const defaultTestAccountBalance = "80000000000000000000000000"

var defaultTestAccountAddress, _ = common.NewAddressFromString("Qaf84bc06703edfc371a0177ac8b482622d5ad24204145f01746cb381dcd546c53b8825839cc61bfc1fc3d78bc560c7bb7a9895432e1e87435474a1bc5a2e1200")
var defaultCoinbase, _ = common.NewAddressFromString("Q00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")

var bigz = big.NewInt(0)
var testAccountBalance = big.NewInt(0)

// GqrlTestnetGenesis creates a genesis.json for execution clients with a set of defaults suitable for ephemeral testnets,
// like in an e2e test. The parameters are minimal but the full value is returned unmarshaled so that it can be
// customized as desired.
func GqrlTestnetGenesis(genesisTime uint64, cfg *clparams.BeaconChainConfig) *core.Genesis {
	cc := &params.ChainConfig{
		ChainID: big.NewInt(defaultTestChainId),
	}
	da := defaultDepositContractAllocation(cfg.DepositContractAddress)
	ma := minerAllocation()

	return &core.Genesis{
		Config:    cc,
		Timestamp: genesisTime,
		// NOTE(rgeraldes24): required by the genesis generation on the beacon node side
		// during the e2e tests
		ExtraData: make([]byte, 32),
		GasLimit:  params.MaxGasLimit, // shift 1 back from the max, just in case
		Mixhash:   common.HexToHash(defaultMixhash),
		Coinbase:  defaultCoinbase,
		Alloc: core.GenesisAlloc{
			da.Address: da.Account,
			ma.Address: ma.Account,
		},
		ParentHash: common.HexToHash(defaultParenthash),
	}
}

type depositAllocation struct {
	Address common.Address
	Account core.GenesisAccount
}

func minerAllocation() depositAllocation {
	return depositAllocation{
		Address: defaultTestAccountAddress,
		Account: core.GenesisAccount{
			Balance: testAccountBalance,
		},
	}
}

// defaultDepositContractAllocation provisions the deposit contract address
// with the deposit log-emitter code from contracts/deposit rather than
// compiled Hyperion deposit contract bytecode, which cannot run under 64-byte
// QRVM address stack semantics. Deposit senders must register a
// NativeDepositContract (see contracts/deposit) so the contract logic runs
// in-process and only DepositEvent log data goes on chain.
func defaultDepositContractAllocation(contractAddress string) depositAllocation {
	contractAddr, err := common.NewAddressFromString(contractAddress)
	if err != nil {
		panic(err) // lint:nopanic
	}
	return depositAllocation{
		Address: contractAddr,
		Account: core.GenesisAccount{
			Code:    deposit.DepositLogEmitterCode(),
			Balance: bigz,
			Nonce:   deterministicNonce(0),
		},
	}
}

func deterministicNonce(i uint64) uint64 {
	return math.MaxUint64/2 + i
}

func init() {
	err := testAccountBalance.UnmarshalText([]byte(defaultTestAccountBalance))
	if err != nil {
		panic(err)
	}
}
