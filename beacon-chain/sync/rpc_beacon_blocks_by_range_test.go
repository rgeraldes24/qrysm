package sync

import (
	"context"
	"io"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-zond/common"
	gzondtypes "github.com/theQRL/go-zond/core/types"
	chainMock "github.com/theQRL/qrysm/beacon-chain/blockchain/testing"
	db2 "github.com/theQRL/qrysm/beacon-chain/db"
	db "github.com/theQRL/qrysm/beacon-chain/db/testing"
	mockExecution "github.com/theQRL/qrysm/beacon-chain/execution/testing"
	"github.com/theQRL/qrysm/beacon-chain/p2p"
	"github.com/theQRL/qrysm/beacon-chain/p2p/encoder"
	p2ptest "github.com/theQRL/qrysm/beacon-chain/p2p/testing"
	p2ptypes "github.com/theQRL/qrysm/beacon-chain/p2p/types"
	"github.com/theQRL/qrysm/beacon-chain/startup"
	"github.com/theQRL/qrysm/cmd/beacon-chain/flags"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	leakybucket "github.com/theQRL/qrysm/container/leaky-bucket"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
	"github.com/theQRL/qrysm/time/slots"
)

func TestRPCBeaconBlocksByRange_RPCHandlerReturnsBlocks(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	req := &zondpb.BeaconBlocksByRangeRequest{
		StartSlot: 100,
		Step:      64,
		Count:     16,
	}

	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	to, err := common.NewAddressFromString("Z095e7baea6a6c7c4c2dfeb977efac326af552d87")
	require.NoError(t, err)
	tx := gzondtypes.NewTx(&gzondtypes.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       0,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      nil,
	})
	txs := []*gzondtypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayloadCapella{
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
	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayloadCapella{
			blockHash: payload,
		},
	}

	// Populate the database with blocks that would match the request.
	var prevRoot [32]byte
	for i := req.StartSlot; i < req.StartSlot.Add(req.Count); i += primitives.Slot(1) {
		blk := util.NewBeaconBlockCapella()
		blk.Block.Slot = i
		blk.Block.Body.ExecutionPayload = payload
		copy(blk.Block.ParentRoot, prevRoot[:])
		prevRoot, err = blk.Block.HashTreeRoot()
		require.NoError(t, err)
		util.SaveBlock(t, context.Background(), d, blk)
	}

	clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
	// Start service with 160 as allowed blocks capacity (and almost zero capacity recovery).
	r := &Service{
		cfg: &config{
			p2p:                           p1,
			beaconDB:                      d,
			clock:                         clock,
			chain:                         &chainMock.ChainService{},
			executionPayloadReconstructor: mockEngine,
		},
		rateLimiter: newRateLimiter(p1),
	}
	pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(0.000001, int64(req.Count*10), time.Second, false)
	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		for i := req.StartSlot; i < req.StartSlot.Add(req.Count); i += primitives.Slot(1) {
			expectSuccess(t, stream)
			_, err := readContextFromStream(stream)
			assert.NoError(t, err)
			res := util.NewBeaconBlockCapella()
			assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
			if res.Block.Slot.SubSlot(req.StartSlot).Mod(1) != 0 {
				t.Errorf("Received unexpected block slot %d", res.Block.Slot)
			}
		}
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)

	err = r.beaconBlocksByRangeRPCHandler(context.Background(), req, stream1)
	require.NoError(t, err)

	// Make sure that rate limiter doesn't limit capacity exceedingly.
	remainingCapacity := r.rateLimiter.limiterMap[topic].Remaining(p2.PeerID().String())
	expectedCapacity := int64(req.Count*10 - req.Count)
	require.Equal(t, expectedCapacity, remainingCapacity, "Unexpected rate limiting capacity")

	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}

func TestRPCBeaconBlocksByRange_ReturnCorrectNumberBack(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	req := &zondpb.BeaconBlocksByRangeRequest{
		StartSlot: 0,
		Step:      1,
		Count:     200,
	}

	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	to, err := common.NewAddressFromString("Z095e7baea6a6c7c4c2dfeb977efac326af552d87")
	require.NoError(t, err)
	tx := gzondtypes.NewTx(&gzondtypes.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       0,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      nil,
	})
	txs := []*gzondtypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayloadCapella{
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
	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayloadCapella{
			blockHash: payload,
		},
	}

	var genRoot [32]byte
	// Populate the database with blocks that would match the request.
	for i := req.StartSlot; i < req.StartSlot.Add(req.Step*req.Count); i += primitives.Slot(req.Step) {
		blk := util.NewBeaconBlockCapella()
		blk.Block.Slot = i
		blk.Block.Body.ExecutionPayload = payload
		if i == 0 {
			rt, err := blk.Block.HashTreeRoot()
			require.NoError(t, err)
			genRoot = rt
		}
		util.SaveBlock(t, context.Background(), d, blk)
	}
	require.NoError(t, d.SaveGenesisBlockRoot(context.Background(), genRoot))

	clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
	// Start service with 160 as allowed blocks capacity (and almost zero capacity recovery).
	r := &Service{
		cfg: &config{
			p2p:                           p1,
			beaconDB:                      d,
			chain:                         &chainMock.ChainService{},
			clock:                         clock,
			executionPayloadReconstructor: mockEngine,
		},
		rateLimiter: newRateLimiter(p1),
	}
	pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(0.000001, int64(req.Count*10), time.Second, false)
	var wg sync.WaitGroup
	wg.Add(1)

	// Use a new request to test this out
	newReq := &zondpb.BeaconBlocksByRangeRequest{StartSlot: 0, Step: 1, Count: 1}

	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		for i := newReq.StartSlot; i < newReq.StartSlot.Add(newReq.Count*newReq.Step); i += primitives.Slot(newReq.Step) {
			expectSuccess(t, stream)
			_, err := readContextFromStream(stream)
			assert.NoError(t, err)
			res := util.NewBeaconBlockCapella()
			assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
			if res.Block.Slot.SubSlot(newReq.StartSlot).Mod(newReq.Step) != 0 {
				t.Errorf("Received unexpected block slot %d", res.Block.Slot)
			}
			// Expect EOF
			b := make([]byte, 1)
			_, err = stream.Read(b)
			require.ErrorContains(t, io.EOF.Error(), err)
		}
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)

	err = r.beaconBlocksByRangeRPCHandler(context.Background(), newReq, stream1)
	require.NoError(t, err)

	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}

func TestRPCBeaconBlocksByRange_ReconstructsPayloads(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	req := &zondpb.BeaconBlocksByRangeRequest{
		StartSlot: 0,
		Step:      1,
		Count:     200,
	}

	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	to, err := common.NewAddressFromString("Z095e7baea6a6c7c4c2dfeb977efac326af552d87")
	require.NoError(t, err)
	tx := gzondtypes.NewTx(&gzondtypes.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       0,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      nil,
	})
	txs := []*gzondtypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayloadCapella{
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
	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayloadCapella{
			blockHash: payload,
		},
	}
	wrappedPayload, err := blocks.WrappedExecutionPayloadCapella(payload, 0)
	require.NoError(t, err)
	header, err := blocks.PayloadToHeaderCapella(wrappedPayload)
	require.NoError(t, err)

	var genRoot [32]byte
	// Populate the database with blocks that would match the request.
	for i := req.StartSlot; i < req.StartSlot.Add(req.Step*req.Count); i += primitives.Slot(req.Step) {
		blk := util.NewBlindedBeaconBlockCapella()
		blk.Block.Slot = i
		blk.Block.Body.ExecutionPayloadHeader = header
		if i == 0 {
			rt, err := blk.Block.HashTreeRoot()
			require.NoError(t, err)
			genRoot = rt
		}
		util.SaveBlock(t, context.Background(), d, blk)
	}
	require.NoError(t, d.SaveGenesisBlockRoot(context.Background(), genRoot))

	clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
	// Start service with 160 as allowed blocks capacity (and almost zero capacity recovery).
	r := &Service{
		cfg: &config{
			p2p:                           p1,
			beaconDB:                      d,
			chain:                         &chainMock.ChainService{},
			clock:                         clock,
			executionPayloadReconstructor: mockEngine,
		},
		rateLimiter: newRateLimiter(p1),
	}
	pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(0.000001, int64(req.Count*10), time.Second, false)
	var wg sync.WaitGroup
	wg.Add(1)

	// Use a new request to test this out
	newReq := &zondpb.BeaconBlocksByRangeRequest{StartSlot: 0, Step: 1, Count: 1}

	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		for i := newReq.StartSlot; i < newReq.StartSlot.Add(newReq.Count*newReq.Step); i += primitives.Slot(newReq.Step) {
			expectSuccess(t, stream)
			_, err := readContextFromStream(stream)
			assert.NoError(t, err)
			res := util.NewBeaconBlockCapella()
			assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
			if res.Block.Slot.SubSlot(newReq.StartSlot).Mod(newReq.Step) != 0 {
				t.Errorf("Received unexpected block slot %d", res.Block.Slot)
			}
			// Expect EOF
			b := make([]byte, 1)
			_, err = stream.Read(b)
			require.ErrorContains(t, io.EOF.Error(), err)
		}
		require.Equal(t, uint64(1), mockEngine.NumReconstructedPayloads)
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)

	err = r.beaconBlocksByRangeRPCHandler(context.Background(), newReq, stream1)
	require.NoError(t, err)

	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}

func TestRPCBeaconBlocksByRange_RPCHandlerReturnsSortedBlocks(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	req := &zondpb.BeaconBlocksByRangeRequest{
		StartSlot: 200,
		Step:      21,
		Count:     33,
	}

	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	to, err := common.NewAddressFromString("Z095e7baea6a6c7c4c2dfeb977efac326af552d87")
	require.NoError(t, err)
	tx := gzondtypes.NewTx(&gzondtypes.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       0,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      nil,
	})
	txs := []*gzondtypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayloadCapella{
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
	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayloadCapella{
			blockHash: payload,
		},
	}

	endSlot := req.StartSlot.Add(req.Count - 1)
	expectedRoots := make([][32]byte, req.Count)
	// Populate the database with blocks that would match the request.
	var prevRoot [32]byte
	for i, j := req.StartSlot, 0; i <= endSlot; i++ {
		blk := util.NewBeaconBlockCapella()
		blk.Block.Slot = i
		blk.Block.Body.ExecutionPayload = payload
		copy(blk.Block.ParentRoot, prevRoot[:])
		rt, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		expectedRoots[j] = rt
		prevRoot = rt
		util.SaveBlock(t, context.Background(), d, blk)
		j++
	}

	clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
	// Start service with 160 as allowed blocks capacity (and almost zero capacity recovery).
	r := &Service{
		cfg: &config{
			p2p:                           p1,
			beaconDB:                      d,
			clock:                         clock,
			chain:                         &chainMock.ChainService{},
			executionPayloadReconstructor: mockEngine,
		},
		rateLimiter: newRateLimiter(p1),
	}
	pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(0.000001, int64(req.Count*10), time.Second, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		prevSlot := primitives.Slot(0)
		require.Equal(t, uint64(len(expectedRoots)), req.Count, "Number of roots not expected")
		for i, j := req.StartSlot, 0; i < req.StartSlot.Add(req.Count); i += primitives.Slot(1) {
			expectSuccess(t, stream)
			_, err := readContextFromStream(stream)
			assert.NoError(t, err)
			res := &zondpb.SignedBeaconBlockCapella{}
			assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
			if res.Block.Slot < prevSlot {
				t.Errorf("Received block is unsorted with slot %d lower than previous slot %d", res.Block.Slot, prevSlot)
			}
			rt, err := res.Block.HashTreeRoot()
			require.NoError(t, err)
			assert.Equal(t, expectedRoots[j], rt, "roots not equal")
			prevSlot = res.Block.Slot
			j++
		}
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	require.NoError(t, r.beaconBlocksByRangeRPCHandler(context.Background(), req, stream1))

	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}

func TestRPCBeaconBlocksByRange_ReturnsGenesisBlock(t *testing.T) {
	p1 := p2ptest.NewTestP2P(t)
	p2 := p2ptest.NewTestP2P(t)
	p1.Connect(p2)
	assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
	d := db.SetupDB(t)

	req := &zondpb.BeaconBlocksByRangeRequest{
		StartSlot: 0,
		Step:      1,
		Count:     4,
	}

	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	to, err := common.NewAddressFromString("Z095e7baea6a6c7c4c2dfeb977efac326af552d87")
	require.NoError(t, err)
	tx := gzondtypes.NewTx(&gzondtypes.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       0,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      nil,
	})
	txs := []*gzondtypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayloadCapella{
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
	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayloadCapella{
			blockHash: payload,
		},
	}

	var prevRoot [32]byte
	// Populate the database with blocks that would match the request.
	for i := req.StartSlot; i < req.StartSlot.Add(req.Step*req.Count); i++ {
		blk := util.NewBeaconBlockCapella()
		blk.Block.Slot = i
		blk.Block.Body.ExecutionPayload = payload
		blk.Block.ParentRoot = prevRoot[:]
		rt, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)

		// Save genesis block
		if i == 0 {
			require.NoError(t, d.SaveGenesisBlockRoot(context.Background(), rt))
		}
		util.SaveBlock(t, context.Background(), d, blk)
		prevRoot = rt
	}

	clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
	r := &Service{
		cfg: &config{
			p2p:                           p1,
			beaconDB:                      d,
			clock:                         clock,
			chain:                         &chainMock.ChainService{},
			executionPayloadReconstructor: mockEngine,
		},
		rateLimiter: newRateLimiter(p1),
	}
	pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
	topic := string(pcl)
	r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(10000, 10000, time.Second, false)

	var wg sync.WaitGroup
	wg.Add(1)
	p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
		defer wg.Done()
		// check for genesis block
		expectSuccess(t, stream)
		_, err := readContextFromStream(stream)
		assert.NoError(t, err)
		res := &zondpb.SignedBeaconBlockCapella{}
		assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
		assert.Equal(t, primitives.Slot(0), res.Block.Slot, "genesis block was not returned")
		for i := req.StartSlot.Add(req.Step); i < primitives.Slot(req.Count*req.Step); i += primitives.Slot(req.Step) {
			expectSuccess(t, stream)
			_, err := readContextFromStream(stream)
			assert.NoError(t, err)
			res := &zondpb.SignedBeaconBlockCapella{}
			assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
		}
	})

	stream1, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
	require.NoError(t, err)
	require.NoError(t, r.beaconBlocksByRangeRPCHandler(context.Background(), req, stream1))

	if util.WaitTimeout(&wg, 1*time.Second) {
		t.Fatal("Did not receive stream within 1 sec")
	}
}

func TestRPCBeaconBlocksByRange_RPCHandlerRateLimitOverflow(t *testing.T) {
	d := db.SetupDB(t)

	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	to, err := common.NewAddressFromString("Z095e7baea6a6c7c4c2dfeb977efac326af552d87")
	require.NoError(t, err)
	tx := gzondtypes.NewTx(&gzondtypes.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       0,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      nil,
	})
	txs := []*gzondtypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayloadCapella{
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
	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayloadCapella{
			blockHash: payload,
		},
	}

	saveBlocks := func(req *zondpb.BeaconBlocksByRangeRequest) {
		// Populate the database with blocks that would match the request.
		var parentRoot [32]byte
		// Default to 1 to be inline with the spec.
		req.Step = 1
		for i := req.StartSlot; i < req.StartSlot.Add(req.Step*req.Count); i += primitives.Slot(req.Step) {
			block := util.NewBeaconBlockCapella()
			block.Block.Slot = i
			block.Block.Body.ExecutionPayload = payload
			if req.Step == 1 {
				block.Block.ParentRoot = parentRoot[:]
			}
			util.SaveBlock(t, context.Background(), d, block)
			rt, err := block.Block.HashTreeRoot()
			require.NoError(t, err)
			parentRoot = rt
		}
	}
	sendRequest := func(p1, p2 *p2ptest.TestP2P, r *Service,
		req *zondpb.BeaconBlocksByRangeRequest, validateBlocks bool, success bool) error {
		var wg sync.WaitGroup
		wg.Add(1)
		pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
		p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
			defer wg.Done()
			if !validateBlocks {
				return
			}
			// Use a step of 1 to be inline with our specs.
			req.Step = 1
			for i := req.StartSlot; i < req.StartSlot.Add(req.Count*req.Step); i += primitives.Slot(req.Step) {
				if !success {
					continue
				}
				expectSuccess(t, stream)
				_, err := readContextFromStream(stream)
				assert.NoError(t, err)
				res := util.NewBeaconBlockCapella()
				assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, res))
				if res.Block.Slot.SubSlot(req.StartSlot).Mod(req.Step) != 0 {
					t.Errorf("Received unexpected block slot %d", res.Block.Slot)
				}
			}
		})
		stream, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
		require.NoError(t, err)
		if err := r.beaconBlocksByRangeRPCHandler(context.Background(), req, stream); err != nil {
			return err
		}
		if util.WaitTimeout(&wg, 1*time.Second) {
			t.Fatal("Did not receive stream within 1 sec")
		}
		return nil
	}

	t.Run("high request count param and no overflow", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

		capacity := int64(flags.Get().BlockBatchLimit * 3)
		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				chain:                         &chainMock.ChainService{},
				clock:                         clock,
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1),
		}

		pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
		topic := string(pcl)
		r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(0.000001, capacity, time.Second, false)
		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 100,
			Step:      5,
			Count:     uint64(capacity),
		}
		saveBlocks(req)

		assert.NoError(t, sendRequest(p1, p2, r, req, true, true))

		remainingCapacity := r.rateLimiter.limiterMap[topic].Remaining(p2.PeerID().String())
		expectedCapacity := int64(0) // Whole capacity is used, but no overflow.
		assert.Equal(t, expectedCapacity, remainingCapacity, "Unexpected rate limiting capacity")
	})

	t.Run("high request count param and overflow", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

		capacity := int64(flags.Get().BlockBatchLimit * 3)
		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				clock:                         clock,
				chain:                         &chainMock.ChainService{},
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1),
		}

		pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
		topic := string(pcl)
		r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(0.000001, capacity, time.Second, false)

		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 100,
			Step:      5,
			Count:     uint64(capacity + 1),
		}
		saveBlocks(req)

		for i := 0; i < p2.Peers().Scorers().BadResponsesScorer().Params().Threshold; i++ {
			err := sendRequest(p1, p2, r, req, false, true)
			assert.ErrorContains(t, p2ptypes.ErrRateLimited.Error(), err)
		}

		remainingCapacity := r.rateLimiter.limiterMap[topic].Remaining(p2.PeerID().String())
		expectedCapacity := int64(0) // Whole capacity is used.
		assert.Equal(t, expectedCapacity, remainingCapacity, "Unexpected rate limiting capacity")
	})

	t.Run("many requests with count set to max blocks per second", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

		capacity := int64(flags.Get().BlockBatchLimit * flags.Get().BlockBatchLimitBurstFactor)
		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				clock:                         clock,
				chain:                         &chainMock.ChainService{},
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1),
		}
		pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
		topic := string(pcl)
		r.rateLimiter.limiterMap[topic] = leakybucket.NewCollector(0.000001, capacity, time.Second, false)

		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 100,
			Step:      1,
			Count:     uint64(flags.Get().BlockBatchLimit),
		}
		saveBlocks(req)

		for i := 0; i < flags.Get().BlockBatchLimitBurstFactor; i++ {
			assert.NoError(t, sendRequest(p1, p2, r, req, true, false))
		}

		// One more request should result in overflow.
		for i := 0; i < p2.Peers().Scorers().BadResponsesScorer().Params().Threshold; i++ {
			err := sendRequest(p1, p2, r, req, false, false)
			assert.ErrorContains(t, p2ptypes.ErrRateLimited.Error(), err)
		}

		remainingCapacity := r.rateLimiter.limiterMap[topic].Remaining(p2.PeerID().String())
		expectedCapacity := int64(0) // Whole capacity is used.
		assert.Equal(t, expectedCapacity, remainingCapacity, "Unexpected rate limiting capacity")
	})
}

func TestRPCBeaconBlocksByRange_validateRangeRequest(t *testing.T) {
	slotsSinceGenesis := primitives.Slot(1000)
	offset := int64(slotsSinceGenesis.Mul(params.BeaconConfig().SecondsPerSlot))
	clock := startup.NewClock(time.Now().Add(time.Second*time.Duration(-1*offset)), [32]byte{})

	tests := []struct {
		name          string
		req           *zondpb.BeaconBlocksByRangeRequest
		expectedError error
		errorToLog    string
	}{
		{
			name: "Zero Count",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Count: 0,
				Step:  1,
			},
			expectedError: p2ptypes.ErrInvalidRequest,
			errorToLog:    "validation did not fail with bad count",
		},
		{
			name: "Over limit Count",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Count: params.BeaconNetworkConfig().MaxRequestBlocks + 1,
				Step:  1,
			},
			expectedError: p2ptypes.ErrInvalidRequest,
			errorToLog:    "validation did not fail with bad count",
		},
		{
			name: "Correct Count",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Count: params.BeaconNetworkConfig().MaxRequestBlocks - 1,
				Step:  1,
			},
			errorToLog: "validation failed with correct count",
		},
		{
			name: "Zero Step",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Step:  0,
				Count: 1,
			},
			expectedError: nil, // The Step param is ignored in v2 RPC
		},
		{
			name: "Over limit Step",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Step:  rangeLimit + 1,
				Count: 1,
			},
			expectedError: nil, // The Step param is ignored in v2 RPC
		},
		{
			name: "Correct Step",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Step:  rangeLimit - 1,
				Count: 2,
			},
			errorToLog: "validation failed with correct step",
		},
		{
			name: "Over Limit Start Slot",
			req: &zondpb.BeaconBlocksByRangeRequest{
				StartSlot: slotsSinceGenesis.Add((2 * rangeLimit) + 1),
				Step:      1,
				Count:     1,
			},
			expectedError: p2ptypes.ErrInvalidRequest,
			errorToLog:    "validation did not fail with bad start slot",
		},
		{
			name: "Over Limit End Slot",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Step:  1,
				Count: params.BeaconNetworkConfig().MaxRequestBlocks + 1,
			},
			expectedError: p2ptypes.ErrInvalidRequest,
			errorToLog:    "validation did not fail with bad end slot",
		},
		{
			name: "Exceed Range Limit",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Step:  3,
				Count: uint64(slotsSinceGenesis / 2),
			},
			expectedError: nil, // this is fine with the deprecation of Step
		},
		{
			name: "Valid Request",
			req: &zondpb.BeaconBlocksByRangeRequest{
				Step:      1,
				Count:     params.BeaconNetworkConfig().MaxRequestBlocks - 1,
				StartSlot: 50,
			},
			errorToLog: "validation failed with valid params",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateRangeRequest(tt.req, clock.CurrentSlot())
			if tt.expectedError != nil {
				assert.ErrorContains(t, tt.expectedError.Error(), err, tt.errorToLog)
			} else {
				assert.NoError(t, err, tt.errorToLog)
			}
		})
	}
}

func TestRPCBeaconBlocksByRange_EnforceResponseInvariants(t *testing.T) {
	d := db.SetupDB(t)
	hook := logTest.NewGlobal()

	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	to, err := common.NewAddressFromString("Z095e7baea6a6c7c4c2dfeb977efac326af552d87")
	require.NoError(t, err)
	tx := gzondtypes.NewTx(&gzondtypes.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       0,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      nil,
	})
	txs := []*gzondtypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayloadCapella{
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
	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayloadCapella{
			blockHash: payload,
		},
	}

	saveBlocks := func(req *zondpb.BeaconBlocksByRangeRequest) {
		// Populate the database with blocks that would match the request.
		var parentRoot [32]byte
		for i := req.StartSlot; i < req.StartSlot.Add(req.Step*req.Count); i += primitives.Slot(req.Step) {
			block := util.NewBeaconBlockCapella()
			block.Block.Slot = i
			block.Block.Body.ExecutionPayload = payload
			block.Block.ParentRoot = parentRoot[:]
			util.SaveBlock(t, context.Background(), d, block)
			rt, err := block.Block.HashTreeRoot()
			require.NoError(t, err)
			parentRoot = rt
		}
	}
	pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
	sendRequest := func(p1, p2 *p2ptest.TestP2P, r *Service,
		req *zondpb.BeaconBlocksByRangeRequest, processBlocks func([]*zondpb.SignedBeaconBlockCapella)) error {
		var wg sync.WaitGroup
		wg.Add(1)
		p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
			defer wg.Done()
			blocks := make([]*zondpb.SignedBeaconBlockCapella, 0, req.Count)
			for i := req.StartSlot; i < req.StartSlot.Add(req.Count*req.Step); i += primitives.Slot(req.Step) {
				expectSuccess(t, stream)
				_, err := readContextFromStream(stream)
				assert.NoError(t, err)
				blk := util.NewBeaconBlockCapella()
				assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, blk))
				if blk.Block.Slot.SubSlot(req.StartSlot).Mod(req.Step) != 0 {
					t.Errorf("Received unexpected block slot %d", blk.Block.Slot)
				}
				blocks = append(blocks, blk)
			}
			processBlocks(blocks)
		})
		stream, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
		require.NoError(t, err)
		if err := r.beaconBlocksByRangeRPCHandler(context.Background(), req, stream); err != nil {
			return err
		}
		if util.WaitTimeout(&wg, 1*time.Second) {
			t.Fatal("Did not receive stream within 1 sec")
		}
		return nil
	}

	t.Run("assert range", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				chain:                         &chainMock.ChainService{},
				clock:                         clock,
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1)}
		r.rateLimiter.limiterMap[string(pcl)] = leakybucket.NewCollector(0.000001, 640, time.Second, false)
		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 448,
			Step:      1,
			Count:     64,
		}
		saveBlocks(req)

		hook.Reset()
		err := sendRequest(p1, p2, r, req, func(blocks []*zondpb.SignedBeaconBlockCapella) {
			assert.Equal(t, req.Count, uint64(len(blocks)))
			for _, blk := range blocks {
				if blk.Block.Slot < req.StartSlot || blk.Block.Slot >= req.StartSlot.Add(req.Count*req.Step) {
					t.Errorf("Block slot is out of range: %d is not within [%d, %d)",
						blk.Block.Slot, req.StartSlot, req.StartSlot.Add(req.Count*req.Step))
				}
			}
		})
		assert.NoError(t, err)
		require.LogsDoNotContain(t, hook, "Disconnecting bad peer")
	})
}

func TestRPCBeaconBlocksByRange_FilterBlocks(t *testing.T) {
	hook := logTest.NewGlobal()

	parent := bytesutil.PadTo([]byte("parentHash"), fieldparams.RootLength)
	stateRoot := bytesutil.PadTo([]byte("stateRoot"), fieldparams.RootLength)
	receiptsRoot := bytesutil.PadTo([]byte("receiptsRoot"), fieldparams.RootLength)
	logsBloom := bytesutil.PadTo([]byte("logs"), fieldparams.LogsBloomLength)
	to, err := common.NewAddressFromString("Z095e7baea6a6c7c4c2dfeb977efac326af552d87")
	require.NoError(t, err)
	tx := gzondtypes.NewTx(&gzondtypes.DynamicFeeTx{
		Nonce:     0,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       0,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Data:      nil,
	})
	txs := []*gzondtypes.Transaction{tx}
	encodedBinaryTxs := make([][]byte, 1)
	encodedBinaryTxs[0], err = txs[0].MarshalBinary()
	require.NoError(t, err)
	blockHash := bytesutil.ToBytes32([]byte("foo"))
	payload := &enginev1.ExecutionPayloadCapella{
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
	mockEngine := &mockExecution.EngineClient{
		ExecutionPayloadByBlockHash: map[[32]byte]*enginev1.ExecutionPayloadCapella{
			blockHash: payload,
		},
	}

	saveBlocks := func(d db2.Database, chain *chainMock.ChainService, req *zondpb.BeaconBlocksByRangeRequest, finalized bool) {
		blk := util.NewBeaconBlockCapella()
		blk.Block.Slot = 0
		blk.Block.Body.ExecutionPayload = payload
		previousRoot, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)

		util.SaveBlock(t, context.Background(), d, blk)
		require.NoError(t, d.SaveGenesisBlockRoot(context.Background(), previousRoot))
		blks := make([]*zondpb.SignedBeaconBlockCapella, req.Count)
		// Populate the database with blocks that would match the request.
		for i, j := req.StartSlot, 0; i < req.StartSlot.Add(req.Step*req.Count); i += primitives.Slot(req.Step) {
			parentRoot := make([]byte, fieldparams.RootLength)
			copy(parentRoot, previousRoot[:])
			blks[j] = util.NewBeaconBlockCapella()
			blks[j].Block.Slot = i
			blks[j].Block.Body.ExecutionPayload = payload
			blks[j].Block.ParentRoot = parentRoot
			var err error
			previousRoot, err = blks[j].Block.HashTreeRoot()
			require.NoError(t, err)
			previousRoot, err = blks[j].Block.HashTreeRoot()
			require.NoError(t, err)
			util.SaveBlock(t, context.Background(), d, blks[j])
			j++
		}
		stateSummaries := make([]*zondpb.StateSummary, len(blks))

		if finalized {
			if chain.CanonicalRoots == nil {
				chain.CanonicalRoots = map[[32]byte]bool{}
			}
			for i, b := range blks {
				bRoot, err := b.Block.HashTreeRoot()
				require.NoError(t, err)
				stateSummaries[i] = &zondpb.StateSummary{
					Slot: b.Block.Slot,
					Root: bRoot[:],
				}
				chain.CanonicalRoots[bRoot] = true
			}
			require.NoError(t, d.SaveStateSummaries(context.Background(), stateSummaries))
			require.NoError(t, d.SaveFinalizedCheckpoint(context.Background(), &zondpb.Checkpoint{
				Epoch: slots.ToEpoch(stateSummaries[len(stateSummaries)-1].Slot),
				Root:  stateSummaries[len(stateSummaries)-1].Root,
			}))
		}
	}
	saveBadBlocks := func(d db2.Database, chain *chainMock.ChainService,
		req *zondpb.BeaconBlocksByRangeRequest, badBlockNum uint64, finalized bool) {
		blk := util.NewBeaconBlockCapella()
		blk.Block.Slot = 0
		blk.Block.Body.ExecutionPayload = payload
		previousRoot, err := blk.Block.HashTreeRoot()
		require.NoError(t, err)
		genRoot := previousRoot

		util.SaveBlock(t, context.Background(), d, blk)
		require.NoError(t, d.SaveGenesisBlockRoot(context.Background(), previousRoot))
		blks := make([]*zondpb.SignedBeaconBlockCapella, req.Count)
		// Populate the database with blocks with non linear roots.
		for i, j := req.StartSlot, 0; i < req.StartSlot.Add(req.Step*req.Count); i += primitives.Slot(req.Step) {
			parentRoot := make([]byte, fieldparams.RootLength)
			copy(parentRoot, previousRoot[:])
			blks[j] = util.NewBeaconBlockCapella()
			blks[j].Block.Slot = i
			blks[j].Block.Body.ExecutionPayload = payload
			blks[j].Block.ParentRoot = parentRoot
			// Make the 2nd block have a bad root.
			if j == int(badBlockNum) {
				blks[j].Block.ParentRoot = genRoot[:]
			}
			var err error
			previousRoot, err = blks[j].Block.HashTreeRoot()
			require.NoError(t, err)
			previousRoot, err = blks[j].Block.HashTreeRoot()
			require.NoError(t, err)
			util.SaveBlock(t, context.Background(), d, blks[j])
			j++
		}
		stateSummaries := make([]*zondpb.StateSummary, len(blks))
		if finalized {
			if chain.CanonicalRoots == nil {
				chain.CanonicalRoots = map[[32]byte]bool{}
			}
			for i, b := range blks {
				bRoot, err := b.Block.HashTreeRoot()
				require.NoError(t, err)
				stateSummaries[i] = &zondpb.StateSummary{
					Slot: b.Block.Slot,
					Root: bRoot[:],
				}
				chain.CanonicalRoots[bRoot] = true
			}
			require.NoError(t, d.SaveStateSummaries(context.Background(), stateSummaries))
			require.NoError(t, d.SaveFinalizedCheckpoint(context.Background(), &zondpb.Checkpoint{
				Epoch: slots.ToEpoch(stateSummaries[len(stateSummaries)-1].Slot),
				Root:  stateSummaries[len(stateSummaries)-1].Root,
			}))
		}
	}
	pcl := protocol.ID(p2p.RPCBlocksByRangeTopicV2)
	sendRequest := func(p1, p2 *p2ptest.TestP2P, r *Service,
		req *zondpb.BeaconBlocksByRangeRequest, processBlocks func([]*zondpb.SignedBeaconBlockCapella)) error {
		var wg sync.WaitGroup
		wg.Add(1)
		p2.BHost.SetStreamHandler(pcl, func(stream network.Stream) {
			defer wg.Done()
			blocks := make([]*zondpb.SignedBeaconBlockCapella, 0, req.Count)
			for i := req.StartSlot; i < req.StartSlot.Add(req.Count*req.Step); i += primitives.Slot(req.Step) {
				code, _, err := ReadStatusCode(stream, &encoder.SszNetworkEncoder{})
				if err != nil && err != io.EOF {
					t.Fatal(err)
				}
				if code != 0 || err == io.EOF {
					break
				}

				_, err = readContextFromStream(stream)
				assert.NoError(t, err)

				blk := util.NewBeaconBlockCapella()
				assert.NoError(t, r.cfg.p2p.Encoding().DecodeWithMaxLength(stream, blk))
				if blk.Block.Slot.SubSlot(req.StartSlot).Mod(req.Step) != 0 {
					t.Errorf("Received unexpected block slot %d", blk.Block.Slot)
				}
				blocks = append(blocks, blk)
			}
			processBlocks(blocks)
		})
		stream, err := p1.BHost.NewStream(context.Background(), p2.BHost.ID(), pcl)
		require.NoError(t, err)
		if err := r.beaconBlocksByRangeRPCHandler(context.Background(), req, stream); err != nil {
			return err
		}
		if util.WaitTimeout(&wg, 1*time.Second) {
			t.Fatal("Did not receive stream within 1 sec")
		}
		return nil
	}

	t.Run("process normal range", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		d := db.SetupDB(t)

		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				clock:                         clock,
				chain:                         &chainMock.ChainService{},
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1),
		}
		r.rateLimiter.limiterMap[string(pcl)] = leakybucket.NewCollector(0.000001, 640, time.Second, false)
		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 1,
			Step:      1,
			Count:     64,
		}
		saveBlocks(d, r.cfg.chain.(*chainMock.ChainService), req, true)

		hook.Reset()
		err := sendRequest(p1, p2, r, req, func(blocks []*zondpb.SignedBeaconBlockCapella) {
			assert.Equal(t, req.Count, uint64(len(blocks)))
			for _, blk := range blocks {
				if blk.Block.Slot < req.StartSlot || blk.Block.Slot >= req.StartSlot.Add(req.Count*req.Step) {
					t.Errorf("Block slot is out of range: %d is not within [%d, %d)",
						blk.Block.Slot, req.StartSlot, req.StartSlot.Add(req.Count*req.Step))
				}
			}
		})
		assert.NoError(t, err)
		require.LogsDoNotContain(t, hook, "Disconnecting bad peer")
	})

	t.Run("process non linear blocks", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		d := db.SetupDB(t)

		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				clock:                         clock,
				chain:                         &chainMock.ChainService{},
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1),
		}
		r.rateLimiter.limiterMap[string(pcl)] = leakybucket.NewCollector(0.000001, 640, time.Second, false)
		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 1,
			Step:      1,
			Count:     64,
		}
		saveBadBlocks(d, r.cfg.chain.(*chainMock.ChainService), req, 2, true)

		hook.Reset()
		err := sendRequest(p1, p2, r, req, func(blocks []*zondpb.SignedBeaconBlockCapella) {
			assert.Equal(t, uint64(2), uint64(len(blocks)))
			var prevRoot [32]byte
			for _, blk := range blocks {
				if blk.Block.Slot < req.StartSlot || blk.Block.Slot >= req.StartSlot.Add(req.Count*req.Step) {
					t.Errorf("Block slot is out of range: %d is not within [%d, %d)",
						blk.Block.Slot, req.StartSlot, req.StartSlot.Add(req.Count*req.Step))
				}
				if prevRoot != [32]byte{} && bytesutil.ToBytes32(blk.Block.ParentRoot) != prevRoot {
					t.Errorf("non linear chain received, expected %#x but got %#x", prevRoot, blk.Block.ParentRoot)
				}
			}
		})
		assert.NoError(t, err)
		require.LogsDoNotContain(t, hook, "Disconnecting bad peer")
	})

	t.Run("process non linear blocks with 2nd bad batch", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		d := db.SetupDB(t)

		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")
		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				chain:                         &chainMock.ChainService{},
				clock:                         clock,
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1),
		}
		r.rateLimiter.limiterMap[string(pcl)] = leakybucket.NewCollector(0.000001, 640, time.Second, false)
		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 1,
			Step:      1,
			Count:     128,
		}
		saveBadBlocks(d, r.cfg.chain.(*chainMock.ChainService), req, 65, true)

		hook.Reset()
		err := sendRequest(p1, p2, r, req, func(blocks []*zondpb.SignedBeaconBlockCapella) {
			assert.Equal(t, uint64(65), uint64(len(blocks)))
			var prevRoot [32]byte
			for _, blk := range blocks {
				if blk.Block.Slot < req.StartSlot || blk.Block.Slot >= req.StartSlot.Add(req.Count*req.Step) {
					t.Errorf("Block slot is out of range: %d is not within [%d, %d)",
						blk.Block.Slot, req.StartSlot, req.StartSlot.Add(req.Count*req.Step))
				}
				if prevRoot != [32]byte{} && bytesutil.ToBytes32(blk.Block.ParentRoot) != prevRoot {
					t.Errorf("non linear chain received, expected %#x but got %#x", prevRoot, blk.Block.ParentRoot)
				}
			}
		})
		assert.NoError(t, err)
		require.LogsDoNotContain(t, hook, "Disconnecting bad peer")
	})

	t.Run("only return finalized blocks", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		d := db.SetupDB(t)

		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				chain:                         &chainMock.ChainService{},
				clock:                         clock,
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1),
		}
		r.rateLimiter.limiterMap[string(pcl)] = leakybucket.NewCollector(0.000001, 640, time.Second, false)
		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 1,
			Step:      1,
			Count:     64,
		}
		saveBlocks(d, r.cfg.chain.(*chainMock.ChainService), req, true)
		req.StartSlot = 65
		req.Step = 1
		req.Count = 128
		// Save unfinalized chain.
		saveBlocks(d, r.cfg.chain.(*chainMock.ChainService), req, false)

		req.StartSlot = 1
		hook.Reset()
		err := sendRequest(p1, p2, r, req, func(blocks []*zondpb.SignedBeaconBlockCapella) {
			assert.Equal(t, uint64(64), uint64(len(blocks)))
			var prevRoot [32]byte
			for _, blk := range blocks {
				if blk.Block.Slot < req.StartSlot || blk.Block.Slot >= 65 {
					t.Errorf("Block slot is out of range: %d is not within [%d, 64)",
						blk.Block.Slot, req.StartSlot)
				}
				if prevRoot != [32]byte{} && bytesutil.ToBytes32(blk.Block.ParentRoot) != prevRoot {
					t.Errorf("non linear chain received, expected %#x but got %#x", prevRoot, blk.Block.ParentRoot)
				}
			}
		})
		assert.NoError(t, err)
		require.LogsDoNotContain(t, hook, "Disconnecting bad peer")
	})
	t.Run("reject duplicate and non canonical blocks", func(t *testing.T) {
		p1 := p2ptest.NewTestP2P(t)
		p2 := p2ptest.NewTestP2P(t)
		d := db.SetupDB(t)

		p1.Connect(p2)
		assert.Equal(t, 1, len(p1.BHost.Network().Peers()), "Expected peers to be connected")

		clock := startup.NewClock(time.Unix(0, 0), [32]byte{})
		r := &Service{
			cfg: &config{
				p2p:                           p1,
				beaconDB:                      d,
				chain:                         &chainMock.ChainService{},
				clock:                         clock,
				executionPayloadReconstructor: mockEngine,
			},
			rateLimiter: newRateLimiter(p1),
		}
		r.rateLimiter.limiterMap[string(pcl)] = leakybucket.NewCollector(0.000001, 640, time.Second, false)
		req := &zondpb.BeaconBlocksByRangeRequest{
			StartSlot: 1,
			Step:      1,
			Count:     64,
		}
		saveBlocks(d, r.cfg.chain.(*chainMock.ChainService), req, true)

		// Create a duplicate set of unfinalized blocks.
		req.StartSlot = 1
		req.Step = 1
		req.Count = 300
		// Save unfinalized chain.
		saveBlocks(d, r.cfg.chain.(*chainMock.ChainService), req, false)

		req.Count = 64
		hook.Reset()
		err := sendRequest(p1, p2, r, req, func(blocks []*zondpb.SignedBeaconBlockCapella) {
			assert.Equal(t, uint64(64), uint64(len(blocks)))
			var prevRoot [32]byte
			for _, blk := range blocks {
				if blk.Block.Slot < req.StartSlot || blk.Block.Slot >= 65 {
					t.Errorf("Block slot is out of range: %d is not within [%d, 64)",
						blk.Block.Slot, req.StartSlot)
				}
				if prevRoot != [32]byte{} && bytesutil.ToBytes32(blk.Block.ParentRoot) != prevRoot {
					t.Errorf("non linear chain received, expected %#x but got %#x", prevRoot, blk.Block.ParentRoot)
				}
			}
		})
		assert.NoError(t, err)
		require.LogsDoNotContain(t, hook, "Disconnecting bad peer")
	})
}

func TestRPCBeaconBlocksByRange_FilterBlocks_PreviousRoot(t *testing.T) {
	req := &zondpb.BeaconBlocksByRangeRequest{
		StartSlot: 100,
		Step:      1,
		Count:     uint64(flags.Get().BlockBatchLimit) * 2,
	}

	// Populate the database with blocks that would match the request.
	var prevRoot [32]byte
	var err error
	var blks []blocks.ROBlock
	for i := req.StartSlot; i < req.StartSlot.Add(req.Count); i += primitives.Slot(1) {
		blk := util.NewBeaconBlockCapella()
		blk.Block.Slot = i
		copy(blk.Block.ParentRoot, prevRoot[:])
		prevRoot, err = blk.Block.HashTreeRoot()
		require.NoError(t, err)
		wsb, err := blocks.NewSignedBeaconBlock(blk)
		require.NoError(t, err)
		copiedRt := prevRoot
		b, err := blocks.NewROBlockWithRoot(wsb, copiedRt)
		require.NoError(t, err)
		blks = append(blks, b)
	}

	chain := &chainMock.ChainService{}
	cf := canonicalFilter{canonical: chain.IsCanonical}
	seq, nseq, err := cf.filter(context.Background(), blks)
	require.NoError(t, err)
	require.Equal(t, len(blks), len(seq))
	require.Equal(t, 0, len(nseq))

	// pointer should reference a new root.
	require.NotEqual(t, cf.prevRoot, [32]byte{})
}
