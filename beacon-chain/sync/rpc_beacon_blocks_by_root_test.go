package sync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	db "github.com/theQRL/qrysm/v4/beacon-chain/db/testing"
	"github.com/theQRL/qrysm/v4/beacon-chain/p2p"
	p2ptest "github.com/theQRL/qrysm/v4/beacon-chain/p2p/testing"
	p2pTypes "github.com/theQRL/qrysm/v4/beacon-chain/p2p/types"
	leakybucket "github.com/theQRL/qrysm/v4/container/leaky-bucket"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

// TODO(rgeraldes24): RPCBlocksByRootTopicV1
/*
func TestRecentBeaconBlocksRPCHandler_ReturnsBlocks(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	var blkRoots p2pTypes.BeaconBlockByRootsReq
	// Populate the database with blocks that would match the request.
	for i := primitives.Slot(1); i < 11; i++ {
		blk := util.NewBeaconBlock()
		blk.Block.Slot = i
		root, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		util.SaveBlock(t, context.Background(), d, blk)
		blkRoots = append(blkRoots, root)
	}

	r := &Service{cfg: &config{p2p: p1, beaconDB: d, clock: startup.NewClock(time.Unix(0, 0), [32]byte{})}, rateLimiter: newRateLimiter(p1)}
	r.cfg.chain = &mock.ChainService{ValidatorsRoot: [32]byte{}}
	pcl := protocol.ID(p2p.RPCBlocksByRootTopicV1)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(10000, 10000, time.Second, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		for i := range blkRoots {
			expectSuccess(t, stream)
			res := util.NewBeaconBlock()
			assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
			if uint64(res.Block.Slot) != uint64(i+1) {
				t.Errorf("Received unexpected block slot %d but wanted %d", res.Block.Slot, i+1)
			}
		}
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	err = r.beaconBlocksRootRPCHandler(context.Background(), &blkRoots, stream1)
	assert.NoError(t, err)

	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}
*/

/*
func TestRecentBeaconBlocksRPCHandler_ReturnsBlocks_ReconstructsPayload(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	// Start service with 160 as allowed blocks capacity (and almost zero capacity recovery).
	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	tx := gzondTypes.NewTransaction(
		0,
		common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"),
		big.NewInt(0), 0, big.NewInt(0),
		nil,
	)
	txs := []*gzondTypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	var err error
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayload{
		ParentHash:    parent,
		FeeRecipient:  make([]byte, fieldparams.FeeRecipientLength),
		StateRoot:     stateRoot,
		ReceiptsRoot:  receiptsRoot,
		LogsBloom:     logsBloom,
		PrevRandao:    blockHash[:],
		BlockNumber:   0,
		GasLimit:      0,
		GasUsed:       0,
		Timestamp:     0,
		ExtraData:     make([]byte, 0),
		BlockHash:     blockHash[:],
		BaseFeePerGas: bytesutil.PadTo([]byte("baseFeePerGas"), fieldparams.RootLength),
		Transactions:  encodedBinaryTxs,
	}
	wrappedPayload, err := blocks.WrappedExecutionPayload(payload, 0)
	require.NoError(t, err)
	header, err := blocks.PayloadToHeader(wrappedPayload)
	require.NoError(t, err)

	var blkRoots p2pTypes.BeaconBlockByRootsReq
	// Populate the database with blocks that would match the request.
	for i := primitives.Slot(1); i < 11; i++ {
		blk := util.NewBlindedBeaconBlock()
		blk.Block.Body.ExecutionPayloadHeader = header
		blk.Block.Slot = i
		root, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		wsb, err := blocks.NewSignedBeaconBlock(blk)
		require.NoError(t, err)
		require.NoError(t, d.SaveBlock(context.Background(), wsb))
		blkRoots = append(blkRoots, root)
	}

	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayload{
			blockHash: payload,
		},
	}
	r := &Service{cfg: &config{
		p2p:                           p1,
		beaconDB:                      d,
		executionPayloadReconstructor: mockEngine,
		chain:                         &mock.ChainService{ValidatorsRoot: [32]byte{}},
		clock:                         startup.NewClock(time.Unix(0, 0), [32]byte{}),
	}, rateLimiter: newRateLimiter(p1)}
	pcl := protocol.ID(p2p.RPCBlocksByRootTopicV1)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(10000, 10000, time.Second, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		for i := range blkRoots {
			expectSuccess(t, stream)
			res := util.NewBeaconBlock()
			assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
			if uint64(res.Block.Slot) != uint64(i+1) {
				t.Errorf("Received unexpected block slot %d but wanted %d", res.Block.Slot, i+1)
			}
		}
		require.Equal(t, uint64(10), mockEngine.NumReconstructedPayloads)
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	err = r.beaconBlocksRootRPCHandler(context.Background(), &blkRoots, stream1)
	assert.NoError(t, err)

	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}
*/

// TODO(rgeraldes24): /beacon_blocks_by_root/1 has been deprecated in capella
/*
func TestRecentBeaconBlocks_RPCRequestSent(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.DelaySend = true

	blockA := util.NewBeaconBlock()
	blockA.Block.Slot = 111
	blockB := util.NewBeaconBlock()
	blockB.Block.Slot = 40
	// Set up a head state with data we expect.
	blockARoot, err := blockA.Block.HashTreeRoot()
	require.NoError(t, err)
	blockBRoot, err := blockB.Block.HashTreeRoot()
	require.NoError(t, err)
	genesisState, err := transition.GenesisBeaconState(context.Background(), nil, 0, &zondpb.Zond1Data{}, &enginev1.ExecutionPayload{})
	require.NoError(t, err)
	require.NoError(t, genesisState.SetSlot(111))
	require.NoError(t, genesisState.UpdateBlockRootAtIndex(111%uint64(params.BeaconConfig().SlotsPerHistoricalRoot), blockARoot))
	finalizedCheckpt := &zondpb.Checkpoint{
		Epoch: 5,
		Root:  blockBRoot[:],
	}

	expectedRoots := p2pTypes.BeaconBlockByRootsReq{blockBRoot, blockARoot}

	chain := &mock.ChainService{
		State:               genesisState,
		FinalizedCheckPoint: finalizedCheckpt,
		Root:                blockARoot[:],
		Genesis:             time.Now(),
		ValidatorsRoot:      [32]byte{},
	}
	r := &Service{
		cfg: &config{
			p2p:   p1,
			chain: chain,
			clock: startup.NewClock(chain.Genesis, chain.ValidatorsRoot),
		},
		slotToPendingBlocks: gcache.New(time.Second, 2*time.Second),
		seenPendingBlocks:   make(map[[32]byte]bool),
		ctx:                 context.Background(),
		rateLimiter:         newRateLimiter(p1),
	}

	// Setup streams
	pcl := protocol.ID("/zond2/beacon_chain/req/beacon_blocks_by_root/1/ssz_snappy")
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(10000, 10000, time.Second, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		out := new(p2pTypes.BeaconBlockByRootsReq)
		assert.NoError(t, p2.Encoding().DecodeWithMaxLength(stream, out))
		assert.DeepEqual(t, &expectedRoots, out, "Did not receive expected message")
		response := []*zondpb.SignedBeaconBlock{blockB, blockA}
		for _, blk := range response {
			_, err := stream.Write([]byte{responseCodeSuccess})
			assert.NoError(t, err, "Could not write to stream")
			_, err = p2.Encoding().EncodeWithMaxLength(stream, blk)
			assert.NoError(t, err, "Could not send response back")
		}
		assert.NoError(t, stream.Close())
	})

	p1.Connect(p2)
	require.NoError(t, r.sendRecentBeaconBlocksRequest(context.Background(), &expectedRoots, p2.PeerID()))

	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}
*/

func TestRecentBeaconBlocksRPCHandler_HandleZeroBlocks(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	r := &Service{cfg: &config{p2p: p1, beaconDB: d}, rateLimiter: newRateLimiter(p1)}
	pcl := protocol.ID(p2p.RPCBlocksByRootTopicV2)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(1, 1, time.Second, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		expectFailure(t, 1, "no block roots provided in request", stream)
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	err = r.beaconBlocksRootRPCHandler(context.Background(), &p2pTypes.BeaconBlockByRootsReq{}, stream1)
	assert.ErrorContains(t, "no block roots provided", err)
	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}

	r.rateLimiter.RLock() // retrieveCollector requires a lock to be held.
	defer r.rateLimiter.RUnlock()
	lter, err := r.rateLimiter.retrieveCollector(topic)
	require.NoError(t, err)
	assert.Equal(t, 1, int(lter.Count(stream1.Conn().RemotePeer().String())))
}
