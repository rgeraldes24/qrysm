package execution

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	logTest "github.com/sirupsen/logrus/hooks/test"
	qrl "github.com/theQRL/go-qrl"
	"github.com/theQRL/go-qrl/accounts/abi"
	"github.com/theQRL/go-qrl/common"
	gqrltypes "github.com/theQRL/go-qrl/core/types"
	"github.com/theQRL/qrysm/beacon-chain/cache"
	"github.com/theQRL/qrysm/beacon-chain/cache/depositcache"
	"github.com/theQRL/qrysm/beacon-chain/cache/depositsnapshot"
	testDB "github.com/theQRL/qrysm/beacon-chain/db/testing"
	mockExecution "github.com/theQRL/qrysm/beacon-chain/execution/testing"
	"github.com/theQRL/qrysm/config/features"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/container/trie"
	contracts "github.com/theQRL/qrysm/contracts/deposit"
	"github.com/theQRL/qrysm/contracts/deposit/mock"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
)

type failingDepositCache struct {
	cache.DepositCache
	insertErr error
}

func (c *failingDepositCache) InsertDeposit(ctx context.Context, d *qrysmpb.Deposit, blockNum uint64, index int64, root [32]byte) error {
	if c.insertErr != nil {
		return c.insertErr
	}
	return c.DepositCache.InsertDeposit(ctx, d, blockNum, index, root)
}

func TestProcessDepositLog_RetryAfterFailure(t *testing.T) {
	deposits, _, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	data := deposits[0].Data
	contractABI, err := abi.JSON(strings.NewReader(contracts.DepositContractABI))
	require.NoError(t, err)
	makeLog := func(pubkey []byte, amount uint64, index uint64) *gqrltypes.Log {
		encoded, err := contractABI.Events["DepositEvent"].Inputs.Pack(pubkey, data.WithdrawalRecipient,
			bytesutil.Bytes8(amount), data.RandaoCommitment, data.Signature, bytesutil.Bytes8(index))
		require.NoError(t, err)
		return &gqrltypes.Log{Data: encoded, BlockNumber: 1, BlockHash: common.Hash{1}}
	}
	for _, snapshot := range []bool{false, true} {
		for _, failure := range []string{"none", "invalid deposit", "cache insertion"} {
			t.Run(fmt.Sprintf("snapshot=%t/%s", snapshot, failure), func(t *testing.T) {
				reset := features.InitWithReset(&features.Flags{EnableEIP4881: snapshot})
				defer reset()
				var depositTree cache.MerkleTree
				var deposits cache.DepositCache
				if snapshot {
					depositTree = depositsnapshot.NewDepositTree()
					deposits, err = depositsnapshot.New()
				} else {
					depositTree, err = trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
					require.NoError(t, err)
					deposits, err = depositcache.New()
				}
				require.NoError(t, err)
				depositCache := &failingDepositCache{DepositCache: deposits}
				s := &Service{cfg: &config{depositCache: depositCache}, depositTrie: depositTree, lastReceivedMerkleIndex: -1}
				ctx := context.Background()
				validLog := makeLog(data.PublicKey, data.Amount, 0)
				if failure != "none" {
					badLog := validLog
					if failure == "invalid deposit" {
						badLog = makeLog(data.PublicKey[:1], data.Amount, 0)
					} else {
						depositCache.insertErr = errors.New("cache unavailable")
					}
					require.NotNil(t, s.ProcessDepositLog(ctx, badLog))
					assert.Equal(t, int64(-1), s.lastReceivedMerkleIndex)
					require.Equal(t, 0, len(depositCache.AllDeposits(ctx, nil)))
					require.Equal(t, 0, len(depositCache.PendingDeposits(ctx, nil)))
					depositCache.insertErr = nil
					if failure == "cache insertion" {
						// A retry must not store different data under the leaf already inserted.
						require.NotNil(t, s.ProcessDepositLog(ctx, makeLog(data.PublicKey, data.Amount+1, 0)))
					}
				}
				require.NoError(t, s.ProcessDepositLog(ctx, validLog))
				require.Equal(t, int64(0), s.lastReceivedMerkleIndex)
				require.Equal(t, 1, s.depositTrie.NumOfItems())
				require.Equal(t, 1, len(depositCache.AllDeposits(ctx, nil)))
				require.Equal(t, 1, len(depositCache.PendingDeposits(ctx, nil)))
				require.DeepEqual(t, data, depositCache.AllDeposits(ctx, nil)[0].Data)
				root, err := s.depositTrie.HashTreeRoot()
				require.NoError(t, err)
				require.DeepEqual(t, root[:], depositCache.AllDepositContainers(ctx)[0].DepositRoot)
				// Duplicate delivery stays idempotent, and processing can advance.
				require.NoError(t, s.ProcessDepositLog(ctx, validLog))
				require.NoError(t, s.ProcessDepositLog(ctx, makeLog(data.PublicKey, data.Amount, 1)))
				require.Equal(t, int64(1), s.lastReceivedMerkleIndex)
				require.Equal(t, 2, s.depositTrie.NumOfItems())
				require.Equal(t, 2, len(depositCache.AllDeposits(ctx, nil)))
				require.Equal(t, 2, len(depositCache.PendingDeposits(ctx, nil)))
			})
		}
	}
}

func TestProcessDepositLog_InvalidEncoding(t *testing.T) {
	deposits, _, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	data := deposits[0].Data
	contractABI, err := abi.JSON(strings.NewReader(contracts.DepositContractABI))
	require.NoError(t, err)
	for _, tt := range []struct {
		name   string
		amount []byte
		index  []byte
	}{
		{"short amount", make([]byte, 7), make([]byte, 8)},
		{"long amount", make([]byte, 9), make([]byte, 8)},
		{"short index", make([]byte, 8), make([]byte, 7)},
		{"long index", make([]byte, 8), make([]byte, 9)},
		{"overflowing index", make([]byte, 8), bytesutil.Bytes8(1 << 63)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reset := features.InitWithReset(&features.Flags{})
			defer reset()
			depositTree, err := trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
			require.NoError(t, err)
			deposits, err := depositcache.New()
			require.NoError(t, err)
			s := &Service{cfg: &config{depositCache: deposits}, depositTrie: depositTree, lastReceivedMerkleIndex: -1}
			encoded, err := contractABI.Events["DepositEvent"].Inputs.Pack(data.PublicKey, data.WithdrawalRecipient,
				tt.amount, data.RandaoCommitment, data.Signature, tt.index)
			require.NoError(t, err)
			ctx := context.Background()
			require.NotNil(t, s.ProcessDepositLog(ctx, &gqrltypes.Log{Data: encoded}))
			require.Equal(t, int64(-1), s.lastReceivedMerkleIndex)
			require.Equal(t, 0, depositTree.NumOfItems())
			require.Equal(t, 0, len(deposits.AllDeposits(ctx, nil)))
		})
	}
}

func TestProcessDepositLog_OK(t *testing.T) {
	hook := logTest.NewGlobal()

	testAcc, err := mock.Setup()
	require.NoError(t, err, "Unable to set up simulated backend")

	beaconDB := testDB.SetupDB(t)
	depositCache, err := depositcache.New()
	require.NoError(t, err)

	server, endpoint, err := mockExecution.SetupRPCServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Stop()
	})
	web3Service, err := NewService(context.Background(),
		WithHttpEndpoint(endpoint),
		WithDepositContractAddress(testAcc.ContractAddr),
		WithDatabase(beaconDB),
		WithDepositCache(depositCache),
	)
	require.NoError(t, err, "unable to setup web3 execution chain service")
	web3Service = setDefaultMocks(web3Service)
	web3Service.depositContractCaller, err = contracts.NewDepositContractCaller(testAcc.ContractAddr, testAcc.Backend)
	require.NoError(t, err)

	testAcc.Backend.Commit()

	deposits, _, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)

	_, depositRoots, err := util.DeterministicDepositTrie(len(deposits))
	require.NoError(t, err)
	data := deposits[0].Data

	testAcc.TxOpts.Value = mock.Amount40000Quanta()
	testAcc.TxOpts.GasLimit = 1000000
	_, err = testAcc.Contract.Deposit(testAcc.TxOpts, data.PublicKey, data.WithdrawalRecipient, data.RandaoCommitment, data.Signature, depositRoots[0])
	require.NoError(t, err, "Could not deposit to deposit contract")

	testAcc.Backend.Commit()

	query := qrl.FilterQuery{
		Addresses: []common.Address{
			web3Service.cfg.depositContractAddr,
		},
	}

	logs, err := testAcc.Backend.FilterLogs(web3Service.ctx, query)
	require.NoError(t, err, "Unable to retrieve logs")

	h2 := testAcc.Backend.Blockchain().CurrentBlock()
	b2 := testAcc.Backend.Blockchain().GetBlock(h2.Hash(), h2.Number.Uint64())
	fmt.Println(len(b2.Transactions()))

	err = web3Service.ProcessLog(context.Background(), &logs[0])
	require.NoError(t, err)

	require.LogsDoNotContain(t, hook, "Could not unpack log")
	require.LogsDoNotContain(t, hook, "Could not save in trie")
	require.LogsDoNotContain(t, hook, "could not deserialize validator public key")
	require.LogsDoNotContain(t, hook, "could not convert bytes to signature")
	require.LogsDoNotContain(t, hook, "could not sign root for deposit data")
	require.LogsDoNotContain(t, hook, "deposit signature did not verify")
	require.LogsDoNotContain(t, hook, "could not tree hash deposit data")
	require.LogsDoNotContain(t, hook, "deposit merkle branch of deposit root did not verify for root")
	require.LogsContain(t, hook, "Deposit registered from deposit contract")

	hook.Reset()
}

func TestProcessLog_MalformedLogNoTopics_NoPanic(t *testing.T) {
	s := &Service{}
	// A log with no topics (e.g. from a hostile or buggy execution client) must be
	// skipped rather than panicking on depositLog.Topics[0].
	require.NoError(t, s.ProcessLog(context.Background(), &gqrltypes.Log{}))
}

func TestProcessDepositLog_InsertsPendingDeposit(t *testing.T) {
	hook := logTest.NewGlobal()
	testAcc, err := mock.Setup()
	require.NoError(t, err, "Unable to set up simulated backend")
	beaconDB := testDB.SetupDB(t)
	depositCache, err := depositcache.New()
	require.NoError(t, err)
	server, endpoint, err := mockExecution.SetupRPCServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Stop()
	})

	web3Service, err := NewService(context.Background(),
		WithHttpEndpoint(endpoint),
		WithDepositContractAddress(testAcc.ContractAddr),
		WithDatabase(beaconDB),
		WithDepositCache(depositCache),
	)
	require.NoError(t, err, "unable to setup web3 execution chain service")
	web3Service = setDefaultMocks(web3Service)
	web3Service.depositContractCaller, err = contracts.NewDepositContractCaller(testAcc.ContractAddr, testAcc.Backend)
	require.NoError(t, err)

	testAcc.Backend.Commit()

	deposits, _, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	_, depositRoots, err := util.DeterministicDepositTrie(len(deposits))
	require.NoError(t, err)
	data := deposits[0].Data

	testAcc.TxOpts.Value = mock.Amount40000Quanta()
	testAcc.TxOpts.GasLimit = 1000000

	_, err = testAcc.Contract.Deposit(testAcc.TxOpts, data.PublicKey, data.WithdrawalRecipient, data.RandaoCommitment, data.Signature, depositRoots[0])
	require.NoError(t, err, "Could not deposit to deposit contract")

	_, err = testAcc.Contract.Deposit(testAcc.TxOpts, data.PublicKey, data.WithdrawalRecipient, data.RandaoCommitment, data.Signature, depositRoots[0])
	require.NoError(t, err, "Could not deposit to deposit contract")

	testAcc.Backend.Commit()

	query := qrl.FilterQuery{
		Addresses: []common.Address{
			web3Service.cfg.depositContractAddr,
		},
	}
	logs, err := testAcc.Backend.FilterLogs(web3Service.ctx, query)
	require.NoError(t, err, "Unable to retrieve logs")

	err = web3Service.ProcessDepositLog(context.Background(), &logs[0])
	require.NoError(t, err)
	err = web3Service.ProcessDepositLog(context.Background(), &logs[1])
	require.NoError(t, err)

	pendingDeposits := web3Service.cfg.depositCache.PendingDeposits(context.Background(), nil /*blockNum*/)
	require.Equal(t, 2, len(pendingDeposits), "Unexpected number of deposits")

	hook.Reset()
}

func TestUnpackDepositLogData_OK(t *testing.T) {
	testAcc, err := mock.Setup()
	require.NoError(t, err, "Unable to set up simulated backend")
	beaconDB := testDB.SetupDB(t)
	server, endpoint, err := mockExecution.SetupRPCServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		server.Stop()
	})
	web3Service, err := NewService(context.Background(),
		WithHttpEndpoint(endpoint),
		WithDepositContractAddress(testAcc.ContractAddr),
		WithDatabase(beaconDB),
	)
	require.NoError(t, err, "unable to setup web3 execution chain service")
	web3Service = setDefaultMocks(web3Service)
	web3Service.depositContractCaller, err = contracts.NewDepositContractCaller(testAcc.ContractAddr, testAcc.Backend)
	require.NoError(t, err)

	testAcc.Backend.Commit()

	deposits, _, err := util.DeterministicDepositsAndKeys(1)
	require.NoError(t, err)
	_, depositRoots, err := util.DeterministicDepositTrie(len(deposits))
	require.NoError(t, err)
	data := deposits[0].Data

	testAcc.TxOpts.Value = mock.Amount40000Quanta()
	testAcc.TxOpts.GasLimit = 1000000
	_, err = testAcc.Contract.Deposit(testAcc.TxOpts, data.PublicKey, data.WithdrawalRecipient, data.RandaoCommitment, data.Signature, depositRoots[0])
	require.NoError(t, err, "Could not deposit to deposit contract")
	testAcc.Backend.Commit()

	query := qrl.FilterQuery{
		Addresses: []common.Address{
			web3Service.cfg.depositContractAddr,
		},
	}

	logz, err := testAcc.Backend.FilterLogs(web3Service.ctx, query)
	require.NoError(t, err, "Unable to retrieve logs")

	loggedPubkey, loggedRecipient, _, loggedCommitment, loggedSig, index, err := contracts.UnpackDepositLogData(logz[0].Data)
	require.NoError(t, err, "Unable to unpack logs")

	require.Equal(t, uint64(0), binary.LittleEndian.Uint64(index), "Retrieved merkle tree index is incorrect")
	require.DeepEqual(t, data.PublicKey, loggedPubkey, "Pubkey is not the same as the data that was put in")
	require.DeepEqual(t, data.Signature, loggedSig, "Proof of Possession is not the same as the data that was put in")
	require.DeepEqual(t, data.WithdrawalRecipient, loggedRecipient, "Withdrawal recipient is not the same as the data that was put in")
	require.DeepEqual(t, data.RandaoCommitment, loggedCommitment, "Randao commitment is not the same as the data that was put in")
}
