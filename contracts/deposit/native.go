package deposit

import (
	"encoding/binary"
	"math/big"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/accounts/abi/bind"
	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/container/trie"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

var nativeDepositContracts sync.Map

// DepositLogEmitterCode returns QRVM bytecode for a minimal contract that
// re-emits its calldata as a DepositEvent log and then returns an ABI-encoded
// empty deposit count (`bytes` of length 8, all zero):
//
//	CALLDATASIZE PUSH0 PUSH0 CALLDATACOPY            ; mem[0:cds] = calldata
//	PUSH32 <DepositEvent topic> CALLDATASIZE PUSH0 LOG1
//	PUSH1 32 PUSH0 MSTORE                            ; bytes head: offset 32
//	PUSH1 8 PUSH1 32 MSTORE                          ; bytes length: 8
//	PUSH0 PUSH1 64 MSTORE                            ; little-endian count: 0
//	PUSH1 96 PUSH0 RETURN
//
// Provision this code at the deposit contract address in test genesis allocs;
// NativeDepositContract then performs the deposit contract logic in-process
// and raw-transacts the ABI-packed event data to the emitter so that genuine
// DepositEvent logs land on chain. The return data keeps the beacon node's
// get_deposit_count qrl_call (processPastLogs) parseable; the count is only
// used as a log-batching heuristic there, so a constant zero is safe.
func DepositLogEmitterCode() []byte {
	// keccak256("DepositEvent(bytes,bytes,bytes,bytes,bytes)")
	topic := common.HexToHash("0x649bbc62d0e31342afea4e5cd82d4049e7e1ee912fc0889aa790803be39038c5")
	code := []byte{0x36, 0x5f, 0x5f, 0x37, 0x7f}
	code = append(code, topic[:]...)
	code = append(code, 0x36, 0x5f, 0xa1)
	return append(code,
		0x60, 0x20, 0x5f, 0x52,
		0x60, 0x08, 0x60, 0x20, 0x52,
		0x5f, 0x60, 0x40, 0x52,
		0x60, 0x60, 0x5f, 0xf3,
	)
}

// NativeDepositContract mirrors the deposit contract behavior for test-only
// simulated backends where stale Hyperion bytecode cannot run under 64-byte
// QRVM address stack semantics.
type NativeDepositContract struct {
	address common.Address
	bound   *bind.BoundContract

	mu           sync.Mutex
	depositTrie  *trie.SparseMerkleTrie
	depositCount uint64
}

// NewNativeDepositContract creates a native deposit contract bound to an
// already provisioned log-emitter account in the supplied backend.
func NewNativeDepositContract(address common.Address, backend bind.ContractBackend) (*NativeDepositContract, error) {
	parsed, err := abi.JSON(strings.NewReader(DepositContractABI))
	if err != nil {
		return nil, err
	}
	depositTrie, err := trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
	if err != nil {
		return nil, err
	}
	return &NativeDepositContract{
		address:     address,
		bound:       bind.NewBoundContract(address, parsed, backend, backend, backend),
		depositTrie: depositTrie,
	}, nil
}

// RegisterNativeDepositContract registers a native contract fallback for the
// generated bindings.
func RegisterNativeDepositContract(contract *NativeDepositContract) {
	nativeDepositContracts.Store(contract.address, contract)
}

func nativeDepositContract(address common.Address) *NativeDepositContract {
	contract, ok := nativeDepositContracts.Load(address)
	if !ok {
		return nil
	}
	return contract.(*NativeDepositContract)
}

func (n *NativeDepositContract) Contract() *DepositContract {
	return &DepositContract{
		DepositContractCaller:     DepositContractCaller{native: n},
		DepositContractTransactor: DepositContractTransactor{native: n},
		DepositContractFilterer:   DepositContractFilterer{native: n},
	}
}

func (n *NativeDepositContract) Deposit(opts *bind.TransactOpts, pubkey []byte, withdrawalCredentials []byte, signature []byte, depositDataRoot [32]byte) (*types.Transaction, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(pubkey) != 2592 {
		return nil, errors.New("execution reverted")
	}
	if len(withdrawalCredentials) != 32 {
		return nil, errors.New("execution reverted")
	}
	if len(signature) != 4627 {
		return nil, errors.New("execution reverted")
	}

	value := opts.Value
	if value == nil {
		value = new(big.Int)
	}
	if value.Cmp(big.NewInt(1e18)) < 0 {
		return nil, errors.New("execution reverted")
	}
	shorPerQuanta := new(big.Int).SetUint64(params.BeaconConfig().ShorPerQuanta)
	if new(big.Int).Mod(value, shorPerQuanta).Sign() != 0 {
		return nil, errors.New("execution reverted")
	}
	amount := new(big.Int).Div(new(big.Int).Set(value), shorPerQuanta)
	if !amount.IsUint64() {
		return nil, errors.New("execution reverted")
	}

	data := &qrysmpb.Deposit_Data{
		PublicKey:             pubkey,
		WithdrawalCredentials: withdrawalCredentials,
		Amount:                amount.Uint64(),
		Signature:             signature,
	}
	root, err := data.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	if root != depositDataRoot {
		return nil, errors.New("execution reverted")
	}

	if err := n.depositTrie.Insert(root[:], int(n.depositCount)); err != nil {
		return nil, err
	}
	amountBytes := littleEndian64(amount.Uint64())
	indexBytes := littleEndian64(n.depositCount)
	logData, err := depositLogData(pubkey, withdrawalCredentials, amountBytes, signature, indexBytes)
	if err != nil {
		return nil, err
	}
	tx, err := n.bound.RawTransact(opts, logData)
	if err != nil {
		return nil, err
	}
	n.depositCount++
	return tx, nil
}

func (n *NativeDepositContract) GetDepositRoot() ([32]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.depositTrie.HashTreeRoot()
}

func (n *NativeDepositContract) GetDepositCount() []byte {
	n.mu.Lock()
	defer n.mu.Unlock()
	return littleEndian64(n.depositCount)
}

func depositLogData(pubkey, withdrawalCredentials, amount, signature, index []byte) ([]byte, error) {
	parsed, err := abi.JSON(strings.NewReader(DepositContractABI))
	if err != nil {
		return nil, err
	}
	return parsed.Events["DepositEvent"].Inputs.NonIndexed().Pack(pubkey, withdrawalCredentials, amount, signature, index)
}

func littleEndian64(value uint64) []byte {
	enc := make([]byte, 8)
	binary.LittleEndian.PutUint64(enc, value)
	return enc
}
