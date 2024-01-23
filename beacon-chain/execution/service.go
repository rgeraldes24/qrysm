// Package execution defines a runtime service which is tasked with
// communicating with an zond endpoint, processing logs from a deposit
// contract, and the latest zond data headers for usage in the beacon node.
package execution

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/go-zond/accounts/abi/bind"
	"github.com/theQRL/go-zond/common"
	"github.com/theQRL/go-zond/common/hexutil"
	zondRPC "github.com/theQRL/go-zond/rpc"
	"github.com/theQRL/qrysm/v4/beacon-chain/cache"
	"github.com/theQRL/qrysm/v4/beacon-chain/cache/depositsnapshot"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/feed"
	statefeed "github.com/theQRL/qrysm/v4/beacon-chain/core/feed/state"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	"github.com/theQRL/qrysm/v4/beacon-chain/db"
	"github.com/theQRL/qrysm/v4/beacon-chain/execution/types"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stategen"
	"github.com/theQRL/qrysm/v4/config/features"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/container/trie"
	contracts "github.com/theQRL/qrysm/v4/contracts/deposit"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/monitoring/clientstats"
	"github.com/theQRL/qrysm/v4/network"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	prysmTime "github.com/theQRL/qrysm/v4/time"
	"github.com/theQRL/qrysm/v4/time/slots"
)

var (
	validDepositsCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "powchain_valid_deposits_received",
		Help: "The number of valid deposits received in the deposit contract",
	})
	blockNumberGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "powchain_block_number",
		Help: "The current block number in the proof-of-work chain",
	})
	missedDepositLogsCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "powchain_missed_deposit_logs",
		Help: "The number of times a missed deposit log is detected",
	})
)

var (
	// time to wait before trying to reconnect with the zond node.
	backOffPeriod = 15 * time.Second
	// amount of times before we log the status of the zond dial attempt.
	logThreshold = 8
	// period to log chainstart related information
	logPeriod = 1 * time.Minute
)

// ChainStartFetcher retrieves information pertaining to the chain start event
// of the beacon chain for usage across various services.
type ChainStartFetcher interface {
	ChainStartZondData() *zondpb.ZondData
	PreGenesisState() state.BeaconState
	ClearPreGenesisData()
}

// ChainInfoFetcher retrieves information about zond metadata at the Ethereum consensus genesis time.
type ChainInfoFetcher interface {
	GenesisExecutionChainInfo() (uint64, *big.Int)
	ExecutionClientConnected() bool
	ExecutionClientEndpoint() string
	ExecutionClientConnectionErr() error
}

// POWBlockFetcher defines a struct that can retrieve mainchain blocks.
type POWBlockFetcher interface {
	BlockTimeByHeight(ctx context.Context, height *big.Int) (uint64, error)
	BlockByTimestamp(ctx context.Context, time uint64) (*types.HeaderInfo, error)
	BlockHashByHeight(ctx context.Context, height *big.Int) (common.Hash, error)
	BlockExists(ctx context.Context, hash common.Hash) (bool, *big.Int, error)
}

// Chain defines a standard interface for the powchain service in Prysm.
type Chain interface {
	ChainStartFetcher
	ChainInfoFetcher
	POWBlockFetcher
}

// RPCClient defines the rpc methods required to interact with the zond node.
type RPCClient interface {
	Close()
	BatchCall(b []zondRPC.BatchElem) error
	CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error
}

type RPCClientEmpty struct {
}

func (RPCClientEmpty) Close() {}
func (RPCClientEmpty) BatchCall([]zondRPC.BatchElem) error {
	return errors.New("rpc client is not initialized")
}

func (RPCClientEmpty) CallContext(context.Context, interface{}, string, ...interface{}) error {
	return errors.New("rpc client is not initialized")
}

// config defines a config struct for dependencies into the service.
type config struct {
	depositContractAddr     common.Address
	beaconDB                db.HeadAccessDatabase
	depositCache            cache.DepositCache
	stateNotifier           statefeed.Notifier
	stateGen                *stategen.State
	zondHeaderReqLimit      uint64
	beaconNodeStatsUpdater  BeaconNodeStatsUpdater
	currHttpEndpoint        network.Endpoint
	headers                 []string
	finalizedStateAtStartup state.BeaconState
}

// Service fetches important information about the canonical
// zond chain via a web3 endpoint using a zondclient.
// The beacon chain requires synchronization with the zond chain's current
// block hash, block number, and access to logs within the
// Validator Registration Contract on the zond chain to kick off the beacon
// chain's validator registration process.
type Service struct {
	connectedZond           bool
	isRunning               bool
	processingLock          sync.RWMutex
	latestZondDataLock      sync.RWMutex
	cfg                     *config
	ctx                     context.Context
	cancel                  context.CancelFunc
	zondHeadTicker          *time.Ticker
	httpLogger              bind.ContractFilterer
	rpcClient               RPCClient
	headerCache             *headerCache // cache to store block hash/block height.
	latestZondData          *zondpb.LatestZondData
	depositContractCaller   *contracts.DepositContractCaller
	depositTrie             cache.MerkleTree
	chainStartData          *zondpb.ChainStartData
	lastReceivedMerkleIndex int64 // Keeps track of the last received index to prevent log spam.
	runError                error
	preGenesisState         state.BeaconState
}

// NewService sets up a new instance with an ethclient when given a web3 endpoint as a string in the config.
func NewService(ctx context.Context, opts ...Option) (*Service, error) {
	ctx, cancel := context.WithCancel(ctx)
	_ = cancel // govet fix for lost cancel. Cancel is handled in service.Stop()
	var depositTrie cache.MerkleTree
	var err error
	if features.Get().EnableEIP4881 {
		depositTrie = depositsnapshot.NewDepositTree()
	} else {
		depositTrie, err = trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
		if err != nil {
			return nil, errors.Wrap(err, "could not set up deposit trie")
		}
	}
	genState, err := transition.EmptyGenesisState()
	if err != nil {
		return nil, errors.Wrap(err, "could not set up genesis state")
	}

	s := &Service{
		ctx:       ctx,
		cancel:    cancel,
		rpcClient: RPCClientEmpty{},
		cfg: &config{
			beaconNodeStatsUpdater: &NopBeaconNodeStatsUpdater{},
			zondHeaderReqLimit:     defaultZondHeaderReqLimit,
		},
		latestZondData: &zondpb.LatestZondData{
			BlockHeight:        0,
			BlockTime:          0,
			BlockHash:          []byte{},
			LastRequestedBlock: 0,
		},
		headerCache: newHeaderCache(),
		depositTrie: depositTrie,
		chainStartData: &zondpb.ChainStartData{
			ZondData:           &zondpb.ZondData{},
			ChainstartDeposits: make([]*zondpb.Deposit, 0),
		},
		lastReceivedMerkleIndex: -1,
		preGenesisState:         genState,
		zondHeadTicker:          time.NewTicker(time.Duration(params.BeaconConfig().SecondsPerZondBlock) * time.Second),
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	if err := s.ensureValidPowchainData(ctx); err != nil {
		return nil, errors.Wrap(err, "unable to validate powchain data")
	}

	zondData, err := s.cfg.beaconDB.ExecutionChainData(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to retrieve zond data")
	}
	if err := s.initializeZondData(ctx, zondData); err != nil {
		return nil, err
	}
	return s, nil
}

// Start the powchain service's main event loop.
func (s *Service) Start() {
	if err := s.setupExecutionClientConnections(s.ctx, s.cfg.currHttpEndpoint); err != nil {
		log.WithError(err).Error("Could not connect to execution endpoint")
	}
	// If the chain has not started already and we don't have access to zond nodes, we will not be
	// able to generate the genesis state.
	if !s.chainStartData.Chainstarted && s.cfg.currHttpEndpoint.Url == "" {
		// check for genesis state before shutting down the node,
		// if a genesis state exists, we can continue on.
		genState, err := s.cfg.beaconDB.GenesisState(s.ctx)
		if err != nil {
			log.Fatal(err)
		}
		if genState == nil || genState.IsNil() {
			log.Fatal("cannot create genesis state: no zond http endpoint defined")
		}
	}

	s.isRunning = true

	// Poll the execution client connection and fallback if errors occur.
	s.pollConnectionStatus(s.ctx)

	// Check transition configuration for the engine API client in the background.
	go s.checkTransitionConfiguration(s.ctx, make(chan *feed.Event, 1))

	go s.run(s.ctx.Done())
}

// Stop the web3 service's main event loop and associated goroutines.
func (s *Service) Stop() error {
	if s.cancel != nil {
		defer s.cancel()
	}
	if s.rpcClient != nil {
		s.rpcClient.Close()
	}
	return nil
}

// ClearPreGenesisData clears out the stored chainstart deposits and beacon state.
func (s *Service) ClearPreGenesisData() {
	s.chainStartData.ChainstartDeposits = []*zondpb.Deposit{}
	s.preGenesisState = &native.BeaconState{}
}

// ChainStartZondData returns the zond data at chainstart.
func (s *Service) ChainStartZondData() *zondpb.ZondData {
	return s.chainStartData.ZondData
}

// PreGenesisState returns a state that contains
// pre-chainstart deposits.
func (s *Service) PreGenesisState() state.BeaconState {
	return s.preGenesisState
}

// Status is service health checks. Return nil or error.
func (s *Service) Status() error {
	// Service don't start
	if !s.isRunning {
		return nil
	}
	// get error from run function
	return s.runError
}

// ExecutionClientConnected checks whether are connected via RPC.
func (s *Service) ExecutionClientConnected() bool {
	return s.connectedZond
}

// ExecutionClientEndpoint returns the URL of the current, connected execution client.
func (s *Service) ExecutionClientEndpoint() string {
	return s.cfg.currHttpEndpoint.Url
}

// ExecutionClientConnectionErr returns the error (if any) of the current connection.
func (s *Service) ExecutionClientConnectionErr() error {
	return s.runError
}

func (s *Service) updateBeaconNodeStats() {
	bs := clientstats.BeaconNodeStats{}
	if s.ExecutionClientConnected() {
		bs.SyncZondConnected = true
	}
	s.cfg.beaconNodeStatsUpdater.Update(bs)
}

func (s *Service) updateConnectedZond(state bool) {
	s.connectedZond = state
	s.updateBeaconNodeStats()
}

// refers to the latest zond block which follows the condition: zond_timestamp +
// SECONDS_PER_ZOND_BLOCK * ZOND_FOLLOW_DISTANCE <= current_unix_time
func (s *Service) followedBlockHeight(ctx context.Context) (uint64, error) {
	followTime := params.BeaconConfig().ZondFollowDistance * params.BeaconConfig().SecondsPerZondBlock
	latestBlockTime := uint64(0)
	if s.latestZondData.BlockTime > followTime {
		latestBlockTime = s.latestZondData.BlockTime - followTime
		// This should only come into play in testnets - when the chain hasn't advanced past the follow distance,
		// we don't want to consider any block before the genesis block.
		if s.latestZondData.BlockHeight < params.BeaconConfig().ZondFollowDistance {
			latestBlockTime = s.latestZondData.BlockTime
		}
	}
	blk, err := s.BlockByTimestamp(ctx, latestBlockTime)
	if err != nil {
		return 0, errors.Wrapf(err, "BlockByTimestamp=%d", latestBlockTime)
	}
	return blk.Number.Uint64(), nil
}

func (s *Service) initDepositCaches(ctx context.Context, ctrs []*zondpb.DepositContainer) error {
	if len(ctrs) == 0 {
		return nil
	}
	s.cfg.depositCache.InsertDepositContainers(ctx, ctrs)
	if !s.chainStartData.Chainstarted {
		// Do not add to pending cache if no genesis state exists.
		validDepositsCount.Add(float64(s.preGenesisState.ZondDepositIndex()))
		return nil
	}
	genesisState, err := s.cfg.beaconDB.GenesisState(ctx)
	if err != nil {
		return err
	}
	// Default to all post-genesis deposits in
	// the event we cannot find a finalized state.
	currIndex := genesisState.ZondDepositIndex()
	chkPt, err := s.cfg.beaconDB.FinalizedCheckpoint(ctx)
	if err != nil {
		return err
	}
	rt := bytesutil.ToBytes32(chkPt.Root)
	if rt != [32]byte{} {
		fState := s.cfg.finalizedStateAtStartup
		if fState == nil || fState.IsNil() {
			return errors.Errorf("finalized state with root %#x is nil", rt)
		}
		// Set deposit index to the one in the current archived state.
		currIndex = fState.ZondDepositIndex()

		// When a node pauses for some time and starts again, the deposits to finalize
		// accumulates. We finalize them here before we are ready to receive a block.
		// Otherwise, the first few blocks will be slower to compute as we will
		// hold the lock and be busy finalizing the deposits.
		// The deposit index in the state is always the index of the next deposit
		// to be included (rather than the last one to be processed). This was most likely
		// done as the state cannot represent signed integers.
		actualIndex := int64(currIndex) - 1 // lint:ignore uintcast -- deposit index will not exceed int64 in your lifetime.
		if err = s.cfg.depositCache.InsertFinalizedDeposits(ctx, actualIndex, common.Hash(fState.ZondData().BlockHash),
			0 /* Setting a zero value as we have no access to block height */); err != nil {
			return err
		}

		// Deposit proofs are only used during state transition and can be safely removed to save space.
		if err = s.cfg.depositCache.PruneProofs(ctx, actualIndex); err != nil {
			return errors.Wrap(err, "could not prune deposit proofs")
		}
	}
	validDepositsCount.Add(float64(currIndex))
	// Only add pending deposits if the container slice length
	// is more than the current index in state.
	if uint64(len(ctrs)) > currIndex {
		for _, c := range ctrs[currIndex:] {
			s.cfg.depositCache.InsertPendingDeposit(ctx, c.Deposit, c.ZondBlockHeight, c.Index, bytesutil.ToBytes32(c.DepositRoot))
		}
	}
	return nil
}

// processBlockHeader adds a newly observed zond block to the block cache and
// updates the latest blockHeight, blockHash, and blockTime properties of the service.
func (s *Service) processBlockHeader(header *types.HeaderInfo) {
	defer safelyHandlePanic()
	blockNumberGauge.Set(float64(header.Number.Int64()))
	s.latestZondDataLock.Lock()
	s.latestZondData.BlockHeight = header.Number.Uint64()
	s.latestZondData.BlockHash = header.Hash.Bytes()
	s.latestZondData.BlockTime = header.Time
	s.latestZondDataLock.Unlock()
	log.WithFields(logrus.Fields{
		"blockNumber": s.latestZondData.BlockHeight,
		"blockHash":   hexutil.Encode(s.latestZondData.BlockHash),
	}).Debug("Latest zond chain event")
}

// batchRequestHeaders requests the block range specified in the arguments. Instead of requesting
// each block in one call, it batches all requests into a single rpc call.
func (s *Service) batchRequestHeaders(startBlock, endBlock uint64) ([]*types.HeaderInfo, error) {
	if startBlock > endBlock {
		return nil, fmt.Errorf("start block height %d cannot be > end block height %d", startBlock, endBlock)
	}
	requestRange := (endBlock - startBlock) + 1
	elems := make([]zondRPC.BatchElem, 0, requestRange)
	headers := make([]*types.HeaderInfo, 0, requestRange)
	if requestRange == 0 {
		return headers, nil
	}
	for i := startBlock; i <= endBlock; i++ {
		header := &types.HeaderInfo{}
		elems = append(elems, zondRPC.BatchElem{
			Method: "zond_getBlockByNumber",
			Args:   []interface{}{hexutil.EncodeBig(big.NewInt(0).SetUint64(i)), false},
			Result: header,
			Error:  error(nil),
		})
		headers = append(headers, header)
	}
	ioErr := s.rpcClient.BatchCall(elems)
	if ioErr != nil {
		return nil, ioErr
	}
	for _, e := range elems {
		if e.Error != nil {
			return nil, e.Error
		}
	}
	for _, h := range headers {
		if h != nil {
			if err := s.headerCache.AddHeader(h); err != nil {
				return nil, err
			}
		}
	}
	return headers, nil
}

// safelyHandleHeader will recover and log any panic that occurs from the block
func safelyHandlePanic() {
	if r := recover(); r != nil {
		log.WithFields(logrus.Fields{
			"r": r,
		}).Error("Panicked when handling data from Zond 1.0 Chain! Recovering...")

		debug.PrintStack()
	}
}

func (s *Service) handleZondFollowDistance() {
	defer safelyHandlePanic()
	ctx := s.ctx

	// use a 5 minutes timeout for block time, because the max mining time is 278 sec (block 7208027)
	// (analyzed the time of the block from 2018-09-01 to 2019-02-13)
	fiveMinutesTimeout := prysmTime.Now().Add(-5 * time.Minute)
	// check that web3 client is syncing
	if time.Unix(int64(s.latestZondData.BlockTime), 0).Before(fiveMinutesTimeout) {
		log.Warn("Execution client is not syncing")
	}
	if !s.chainStartData.Chainstarted {
		if err := s.processChainStartFromBlockNum(ctx, big.NewInt(int64(s.latestZondData.LastRequestedBlock))); err != nil {
			s.runError = errors.Wrap(err, "processChainStartFromBlockNum")
			log.Error(err)
			return
		}
	}

	// If the last requested block has not changed,
	// we do not request batched logs as this means there are no new
	// logs for the powchain service to process. Also it is a potential
	// failure condition as would mean we have not respected the protocol threshold.
	if s.latestZondData.LastRequestedBlock == s.latestZondData.BlockHeight {
		log.Error("Beacon node is not respecting the follow distance")
		return
	}
	if err := s.requestBatchedHeadersAndLogs(ctx); err != nil {
		s.runError = errors.Wrap(err, "requestBatchedHeadersAndLogs")
		log.Error(err)
		return
	}
	// Reset the Status.
	if s.runError != nil {
		s.runError = nil
	}
}

func (s *Service) initPOWService() {
	// Use a custom logger to only log errors
	logCounter := 0
	errorLogger := func(err error, msg string) {
		if logCounter > logThreshold {
			log.WithError(err).Error(msg)
			logCounter = 0
		}
		logCounter++
	}

	// Run in a select loop to retry in the event of any failures.
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			ctx := s.ctx
			header, err := s.HeaderByNumber(ctx, nil)
			if err != nil {
				err = errors.Wrap(err, "HeaderByNumber")
				s.retryExecutionClientConnection(ctx, err)
				errorLogger(err, "Unable to retrieve latest execution client header")
				continue
			}

			s.latestZondDataLock.Lock()
			s.latestZondData.BlockHeight = header.Number.Uint64()
			s.latestZondData.BlockHash = header.Hash.Bytes()
			s.latestZondData.BlockTime = header.Time
			s.latestZondDataLock.Unlock()

			if err := s.processPastLogs(ctx); err != nil {
				err = errors.Wrap(err, "processPastLogs")
				s.retryExecutionClientConnection(ctx, err)
				errorLogger(
					err,
					"Unable to process past deposit contract logs, perhaps your execution client is not fully synced",
				)
				continue
			}
			// Cache zond headers from our voting period.
			if err := s.cacheHeadersForZondDataVote(ctx); err != nil {
				err = errors.Wrap(err, "cacheHeadersForZondDataVote")
				s.retryExecutionClientConnection(ctx, err)
				if errors.Is(err, errBlockTimeTooLate) {
					log.WithError(err).Debug("Unable to cache headers for execution client votes")
				} else {
					errorLogger(err, "Unable to cache headers for execution client votes")
				}
				continue
			}
			// Handle edge case with embedded genesis state by fetching genesis header to determine
			// its height.
			if s.chainStartData.Chainstarted && s.chainStartData.GenesisBlock == 0 {
				genHash := common.BytesToHash(s.chainStartData.ZondData.BlockHash)
				genBlock := s.chainStartData.GenesisBlock
				// In the event our provided chainstart data references a non-existent block hash,
				// we assume the genesis block to be 0.
				if genHash != [32]byte{} {
					genHeader, err := s.HeaderByHash(ctx, genHash)
					if err != nil {
						err = errors.Wrapf(err, "HeaderByHash, hash=%#x", genHash)
						s.retryExecutionClientConnection(ctx, err)
						errorLogger(err, "Unable to retrieve proof-of-stake genesis block data")
						continue
					}
					genBlock = genHeader.Number.Uint64()
				}
				s.chainStartData.GenesisBlock = genBlock
				if err := s.savePowchainData(ctx); err != nil {
					err = errors.Wrap(err, "savePowchainData")
					s.retryExecutionClientConnection(ctx, err)
					errorLogger(err, "Unable to save execution client data")
					continue
				}
			}
			return
		}
	}
}

// run subscribes to all the services for the zond chain.
func (s *Service) run(done <-chan struct{}) {
	s.runError = nil

	s.initPOWService()

	chainstartTicker := time.NewTicker(logPeriod)
	defer chainstartTicker.Stop()

	for {
		select {
		case <-done:
			s.isRunning = false
			s.runError = nil
			s.rpcClient.Close()
			s.updateConnectedZond(false)
			log.Debug("Context closed, exiting goroutine")
			return
		case <-s.zondHeadTicker.C:
			head, err := s.HeaderByNumber(s.ctx, nil)
			if err != nil {
				s.pollConnectionStatus(s.ctx)
				log.WithError(err).Debug("Could not fetch latest zond header")
				continue
			}
			s.processBlockHeader(head)
			s.handleZondFollowDistance()
		case <-chainstartTicker.C:
			if s.chainStartData.Chainstarted {
				chainstartTicker.Stop()
				continue
			}
			s.logTillChainStart(context.Background())
		}
	}
}

// logs the current thresholds required to hit chainstart every minute.
func (s *Service) logTillChainStart(ctx context.Context) {
	if s.chainStartData.Chainstarted {
		return
	}
	_, blockTime, err := s.retrieveBlockHashAndTime(s.ctx, big.NewInt(int64(s.latestZondData.LastRequestedBlock)))
	if err != nil {
		log.Error(err)
		return
	}
	valCount, genesisTime := s.currentCountAndTime(ctx, blockTime)
	valNeeded := uint64(0)
	if valCount < params.BeaconConfig().MinGenesisActiveValidatorCount {
		valNeeded = params.BeaconConfig().MinGenesisActiveValidatorCount - valCount
	}
	secondsLeft := uint64(0)
	if genesisTime < params.BeaconConfig().MinGenesisTime {
		secondsLeft = params.BeaconConfig().MinGenesisTime - genesisTime
	}

	fields := logrus.Fields{
		"Additional validators needed": valNeeded,
	}
	if secondsLeft > 0 {
		fields["Generating genesis state in"] = time.Duration(secondsLeft) * time.Second
	}

	log.WithFields(fields).Info("Currently waiting for chainstart")
}

// cacheHeadersForZondDataVote makes sure that voting for zonddata after startup utilizes cached headers
// instead of making multiple RPC requests to the zond endpoint.
func (s *Service) cacheHeadersForZondDataVote(ctx context.Context) error {
	// Find the end block to request from.
	end, err := s.followedBlockHeight(ctx)
	if err != nil {
		return errors.Wrap(err, "followedBlockHeight")
	}
	start, err := s.determineEarliestVotingBlock(ctx, end)
	if err != nil {
		return errors.Wrapf(err, "determineEarliestVotingBlock=%d", end)
	}
	return s.cacheBlockHeaders(start, end)
}

// Caches block headers from the desired range.
func (s *Service) cacheBlockHeaders(start, end uint64) error {
	batchSize := s.cfg.zondHeaderReqLimit
	for i := start; i < end; i += batchSize {
		startReq := i
		endReq := i + batchSize
		if endReq > 0 {
			// Reduce the end request by one
			// to prevent total batch size from exceeding
			// the allotted limit.
			endReq -= 1
		}
		if endReq > end {
			endReq = end
		}
		// We call batchRequestHeaders for its header caching side-effect, so we don't need the return value.
		_, err := s.batchRequestHeaders(startReq, endReq)
		if err != nil {
			if clientTimedOutError(err) {
				// Reduce batch size as zond node is
				// unable to respond to the request in time.
				batchSize /= 2
				// Always have it greater than 0.
				if batchSize == 0 {
					batchSize += 1
				}

				// Reset request value
				if i > batchSize {
					i -= batchSize
				}
				continue
			}
			return errors.Wrapf(err, "cacheBlockHeaders, start=%d, end=%d", startReq, endReq)
		}
	}
	return nil
}

// Determines the earliest voting block from which to start caching all our previous headers from.
func (s *Service) determineEarliestVotingBlock(ctx context.Context, followBlock uint64) (uint64, error) {
	genesisTime := s.chainStartData.GenesisTime
	currSlot := slots.CurrentSlot(genesisTime)

	// In the event genesis has not occurred yet, we just request to go back follow_distance blocks.
	if genesisTime == 0 || currSlot == 0 {
		earliestBlk := uint64(0)
		if followBlock > params.BeaconConfig().ZondFollowDistance {
			earliestBlk = followBlock - params.BeaconConfig().ZondFollowDistance
		}
		return earliestBlk, nil
	}
	// This should only come into play in testnets - when the chain hasn't advanced past the follow distance,
	// we don't want to consider any block before the genesis block.
	if s.latestZondData.BlockHeight < params.BeaconConfig().ZondFollowDistance {
		return 0, nil
	}
	votingTime := slots.VotingPeriodStartTime(genesisTime, currSlot)
	followBackDist := 2 * params.BeaconConfig().SecondsPerZondBlock * params.BeaconConfig().ZondFollowDistance
	if followBackDist > votingTime {
		return 0, errors.Errorf("invalid genesis time provided. %d > %d", followBackDist, votingTime)
	}
	earliestValidTime := votingTime - followBackDist
	if earliestValidTime < genesisTime {
		return 0, nil
	}
	hdr, err := s.BlockByTimestamp(ctx, earliestValidTime)
	if err != nil {
		return 0, err
	}
	return hdr.Number.Uint64(), nil
}

// initializes our service from the provided zonddata object by initializing all the relevant
// fields and data.
func (s *Service) initializeZondData(ctx context.Context, zondDataInDB *zondpb.ZondChainData) error {
	// The node has no zonddata persisted on disk, so we exit and instead
	// request from contract logs.
	if zondDataInDB == nil {
		return nil
	}
	var err error
	if features.Get().EnableEIP4881 {
		if zondDataInDB.DepositSnapshot != nil {
			s.depositTrie, err = depositsnapshot.DepositTreeFromSnapshotProto(zondDataInDB.DepositSnapshot)
		} else {
			if err := s.migrateOldDepositTree(zondDataInDB); err != nil {
				return err
			}
		}
	} else {
		s.depositTrie, err = trie.CreateTrieFromProto(zondDataInDB.Trie)
	}
	if err != nil {
		return err
	}
	s.chainStartData = zondDataInDB.ChainstartData
	if !reflect.ValueOf(zondDataInDB.BeaconState).IsZero() {
		s.preGenesisState, err = native.InitializeFromProtoPhase0(zondDataInDB.BeaconState)
		if err != nil {
			return errors.Wrap(err, "Could not initialize state trie")
		}
	}
	s.latestZondData = zondDataInDB.CurrentZondData
	if features.Get().EnableEIP4881 {
		ctrs := zondDataInDB.DepositContainers
		// Look at previously finalized index, as we are building off a finalized
		// snapshot rather than the full trie.
		lastFinalizedIndex := int64(s.depositTrie.NumOfItems() - 1)
		// Correctly initialize missing deposits into active trie.
		for _, c := range ctrs {
			if c.Index > lastFinalizedIndex {
				depRoot, err := c.Deposit.Data.HashTreeRoot()
				if err != nil {
					return err
				}
				if err := s.depositTrie.Insert(depRoot[:], int(c.Index)); err != nil {
					return err
				}
			}
		}
	}
	numOfItems := s.depositTrie.NumOfItems()
	s.lastReceivedMerkleIndex = int64(numOfItems - 1)
	if err := s.initDepositCaches(ctx, zondDataInDB.DepositContainers); err != nil {
		return errors.Wrap(err, "could not initialize caches")
	}
	return nil
}

// Validates that all deposit containers are valid and have their relevant indices
// in order.
func validateDepositContainers(ctrs []*zondpb.DepositContainer) bool {
	ctrLen := len(ctrs)
	// Exit for empty containers.
	if ctrLen == 0 {
		return true
	}
	// Sort deposits in ascending order.
	sort.Slice(ctrs, func(i, j int) bool {
		return ctrs[i].Index < ctrs[j].Index
	})
	startIndex := int64(0)
	for _, c := range ctrs {
		if c.Index != startIndex {
			log.Info("Recovering missing deposit containers, node is re-requesting missing deposit data")
			return false
		}
		startIndex++
	}
	return true
}

// Validates the current powchain data is saved and makes sure that any
// embedded genesis state is correctly accounted for.
func (s *Service) ensureValidPowchainData(ctx context.Context) error {
	genState, err := s.cfg.beaconDB.GenesisState(ctx)
	if err != nil {
		return err
	}
	// Exit early if no genesis state is saved.
	if genState == nil || genState.IsNil() {
		return nil
	}
	zondData, err := s.cfg.beaconDB.ExecutionChainData(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to retrieve zond data")
	}
	if zondData == nil || !zondData.ChainstartData.Chainstarted || !validateDepositContainers(zondData.DepositContainers) {
		pbState, err := native.ProtobufBeaconStatePhase0(s.preGenesisState.ToProtoUnsafe())
		if err != nil {
			return err
		}
		s.chainStartData = &zondpb.ChainStartData{
			Chainstarted:       true,
			GenesisTime:        genState.GenesisTime(),
			GenesisBlock:       0,
			ZondData:           genState.ZondData(),
			ChainstartDeposits: make([]*zondpb.Deposit, 0),
		}
		zondData = &zondpb.ZondChainData{
			CurrentZondData:   s.latestZondData,
			ChainstartData:    s.chainStartData,
			BeaconState:       pbState,
			DepositContainers: s.cfg.depositCache.AllDepositContainers(ctx),
		}
		if features.Get().EnableEIP4881 {
			trie, ok := s.depositTrie.(*depositsnapshot.DepositTree)
			if !ok {
				return errors.New("deposit trie was not EIP4881 DepositTree")
			}
			zondData.DepositSnapshot, err = trie.ToProto()
			if err != nil {
				return err
			}
		} else {
			trie, ok := s.depositTrie.(*trie.SparseMerkleTrie)
			if !ok {
				return errors.New("deposit trie was not SparseMerkleTrie")
			}
			zondData.Trie = trie.ToProto()
		}
		return s.cfg.beaconDB.SaveExecutionChainData(ctx, zondData)
	}
	return nil
}

func dedupEndpoints(endpoints []string) []string {
	selectionMap := make(map[string]bool)
	newEndpoints := make([]string, 0, len(endpoints))
	for _, point := range endpoints {
		if selectionMap[point] {
			continue
		}
		newEndpoints = append(newEndpoints, point)
		selectionMap[point] = true
	}
	return newEndpoints
}

func (s *Service) migrateOldDepositTree(zondDataInDB *zondpb.ZondChainData) error {
	oldDepositTrie, err := trie.CreateTrieFromProto(zondDataInDB.Trie)
	if err != nil {
		return err
	}
	newDepositTrie := depositsnapshot.NewDepositTree()
	for i, item := range oldDepositTrie.Items() {
		if err = newDepositTrie.Insert(item, i); err != nil {
			return errors.Wrapf(err, "could not insert item at index %d into deposit snapshot tree", i)
		}
	}
	newDepositRoot, err := newDepositTrie.HashTreeRoot()
	if err != nil {
		return err
	}
	depositRoot, err := oldDepositTrie.HashTreeRoot()
	if err != nil {
		return err
	}
	if newDepositRoot != depositRoot {
		return errors.Wrapf(err, "mismatched deposit roots, old %#x != new %#x", depositRoot, newDepositRoot)
	}
	s.depositTrie = newDepositTrie
	return nil
}
