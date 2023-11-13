package blocks_test

import (
	"testing"

	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	consensus_types "github.com/theQRL/qrysm/v4/consensus-types"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func TestWrapExecutionPayload(t *testing.T) {
	data := &enginev1.ExecutionPayload{
		ParentHash:    []byte("parenthash"),
		FeeRecipient:  []byte("feerecipient"),
		StateRoot:     []byte("stateroot"),
		ReceiptsRoot:  []byte("receiptsroot"),
		LogsBloom:     []byte("logsbloom"),
		PrevRandao:    []byte("prevrandao"),
		BlockNumber:   11,
		GasLimit:      22,
		GasUsed:       33,
		Timestamp:     44,
		ExtraData:     []byte("extradata"),
		BaseFeePerGas: []byte("basefeepergas"),
		BlockHash:     []byte("blockhash"),
		Transactions:  [][]byte{[]byte("transaction")},
		Withdrawals: []*enginev1.Withdrawal{{
			Index:          55,
			ValidatorIndex: 66,
			Address:        []byte("executionaddress"),
			Amount:         77,
		}},
	}
	payload, err := blocks.WrappedExecutionPayload(data, 10)
	require.NoError(t, err)
	v, err := payload.ValueInGwei()
	require.NoError(t, err)
	assert.Equal(t, uint64(10), v)

	assert.DeepEqual(t, data, payload.Proto())
}

func TestWrapExecutionPayloadHeader(t *testing.T) {
	data := &enginev1.ExecutionPayloadHeader{
		ParentHash:       []byte("parenthash"),
		FeeRecipient:     []byte("feerecipient"),
		StateRoot:        []byte("stateroot"),
		ReceiptsRoot:     []byte("receiptsroot"),
		LogsBloom:        []byte("logsbloom"),
		PrevRandao:       []byte("prevrandao"),
		BlockNumber:      11,
		GasLimit:         22,
		GasUsed:          33,
		Timestamp:        44,
		ExtraData:        []byte("extradata"),
		BaseFeePerGas:    []byte("basefeepergas"),
		BlockHash:        []byte("blockhash"),
		TransactionsRoot: []byte("transactionsroot"),
		WithdrawalsRoot:  []byte("withdrawalsroot"),
	}
	payload, err := blocks.WrappedExecutionPayloadHeader(data, 10)
	require.NoError(t, err)

	v, err := payload.ValueInGwei()
	require.NoError(t, err)
	assert.Equal(t, uint64(10), v)

	assert.DeepEqual(t, data, payload.Proto())

	txRoot, err := payload.TransactionsRoot()
	require.NoError(t, err)
	require.DeepEqual(t, txRoot, data.TransactionsRoot)

	wrRoot, err := payload.WithdrawalsRoot()
	require.NoError(t, err)
	require.DeepEqual(t, wrRoot, data.WithdrawalsRoot)
}

func TestWrapExecutionPayload_IsNil(t *testing.T) {
	_, err := blocks.WrappedExecutionPayload(nil, 0)
	require.Equal(t, consensus_types.ErrNilObjectWrapped, err)

	data := &enginev1.ExecutionPayload{GasUsed: 54}
	payload, err := blocks.WrappedExecutionPayload(data, 0)
	require.NoError(t, err)

	assert.Equal(t, false, payload.IsNil())
}

func TestWrapExecutionPayloadHeader_IsNil(t *testing.T) {
	_, err := blocks.WrappedExecutionPayloadHeader(nil, 0)
	require.Equal(t, consensus_types.ErrNilObjectWrapped, err)

	data := &enginev1.ExecutionPayloadHeader{GasUsed: 54}
	payload, err := blocks.WrappedExecutionPayloadHeader(data, 0)
	require.NoError(t, err)

	assert.Equal(t, false, payload.IsNil())
}

func TestWrapExecutionPayload_SSZ(t *testing.T) {
	payload := createWrappedPayload(t)
	rt, err := payload.HashTreeRoot()
	assert.NoError(t, err)
	assert.NotEmpty(t, rt)

	var b []byte
	b, err = payload.MarshalSSZTo(b)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(b))
	encoded, err := payload.MarshalSSZ()
	require.NoError(t, err)
	assert.NotEqual(t, 0, payload.SizeSSZ())
	assert.NoError(t, payload.UnmarshalSSZ(encoded))
}

func TestWrapExecutionPayloadHeader_SSZ(t *testing.T) {
	payload := createWrappedPayloadHeader(t)
	rt, err := payload.HashTreeRoot()
	assert.NoError(t, err)
	assert.NotEmpty(t, rt)

	var b []byte
	b, err = payload.MarshalSSZTo(b)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(b))
	encoded, err := payload.MarshalSSZ()
	require.NoError(t, err)
	assert.NotEqual(t, 0, payload.SizeSSZ())
	assert.NoError(t, payload.UnmarshalSSZ(encoded))
}

func Test_executionPayloadCapella_Pb(t *testing.T) {
	payload := createWrappedPayload(t)
	pb, err := payload.PbCapella()
	require.NoError(t, err)
	assert.DeepEqual(t, payload.Proto(), pb)
}

func Test_executionPayloadHeaderCapella_Pb(t *testing.T) {
	payload := createWrappedPayloadHeader(t)
	_, err := payload.PbCapella()
	require.ErrorIs(t, err, consensus_types.ErrUnsupportedField)
}

func createWrappedPayload(t testing.TB) interfaces.ExecutionData {
	payload, err := blocks.WrappedExecutionPayload(&enginev1.ExecutionPayload{
		ParentHash:    make([]byte, fieldparams.RootLength),
		FeeRecipient:  make([]byte, fieldparams.FeeRecipientLength),
		StateRoot:     make([]byte, fieldparams.RootLength),
		ReceiptsRoot:  make([]byte, fieldparams.RootLength),
		LogsBloom:     make([]byte, fieldparams.LogsBloomLength),
		PrevRandao:    make([]byte, fieldparams.RootLength),
		BlockNumber:   0,
		GasLimit:      0,
		GasUsed:       0,
		Timestamp:     0,
		ExtraData:     make([]byte, 0),
		BaseFeePerGas: make([]byte, fieldparams.RootLength),
		BlockHash:     make([]byte, fieldparams.RootLength),
		Transactions:  make([][]byte, 0),
		Withdrawals:   make([]*enginev1.Withdrawal, 0),
	}, 0)
	require.NoError(t, err)
	return payload
}

func createWrappedPayloadHeader(t testing.TB) interfaces.ExecutionData {
	payload, err := blocks.WrappedExecutionPayloadHeader(&enginev1.ExecutionPayloadHeader{
		ParentHash:       make([]byte, fieldparams.RootLength),
		FeeRecipient:     make([]byte, fieldparams.FeeRecipientLength),
		StateRoot:        make([]byte, fieldparams.RootLength),
		ReceiptsRoot:     make([]byte, fieldparams.RootLength),
		LogsBloom:        make([]byte, fieldparams.LogsBloomLength),
		PrevRandao:       make([]byte, fieldparams.RootLength),
		BlockNumber:      0,
		GasLimit:         0,
		GasUsed:          0,
		Timestamp:        0,
		ExtraData:        make([]byte, 0),
		BaseFeePerGas:    make([]byte, fieldparams.RootLength),
		BlockHash:        make([]byte, fieldparams.RootLength),
		TransactionsRoot: make([]byte, fieldparams.RootLength),
		WithdrawalsRoot:  make([]byte, fieldparams.RootLength),
	}, 0)
	require.NoError(t, err)
	return payload
}
