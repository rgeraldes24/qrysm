package migration

import (
	"testing"

	"github.com/theQRL/go-bitfield"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	zondpbv2 "github.com/theQRL/qrysm/v4/proto/zond/v2"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func Test_BellatrixToV1Alpha1SignedBlock(t *testing.T) {
	v2Block := util.HydrateV2CapellaSignedBeaconBlock(&zondpbv2.SignedBeaconBlockCapella{})
	v2Block.Message.Slot = slot
	v2Block.Message.ProposerIndex = validatorIndex
	v2Block.Message.ParentRoot = parentRoot
	v2Block.Message.StateRoot = stateRoot
	v2Block.Message.Body.RandaoReveal = randaoReveal
	v2Block.Message.Body.Eth1Data = &zondpbv1.Eth1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}
	syncCommitteeBits := bitfield.NewBitvector16()
	syncCommitteeBits.SetBitAt(100, true)
	v2Block.Message.Body.SyncAggregate = &zondpbv1.SyncAggregate{
		SyncCommitteeBits:       syncCommitteeBits,
		SyncCommitteeSignatures: [][]byte{signature},
	}
	v2Block.Message.Body.ExecutionPayload = &enginev1.ExecutionPayloadCapella{
		ParentHash:    parentHash,
		FeeRecipient:  feeRecipient,
		StateRoot:     stateRoot,
		ReceiptsRoot:  receiptsRoot,
		LogsBloom:     logsBloom,
		PrevRandao:    prevRandao,
		BlockNumber:   blockNumber,
		GasLimit:      gasLimit,
		GasUsed:       gasUsed,
		Timestamp:     timestamp,
		ExtraData:     extraData,
		BaseFeePerGas: baseFeePerGas,
		BlockHash:     blockHash,
		Transactions:  [][]byte{[]byte("transaction1"), []byte("transaction2")},
		// TODO(rgeraldes24): withdrawals
	}
	v2Block.Signature = signature

	alphaBlock, err := CapellaToV1Alpha1SignedBlock(v2Block)
	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	v2Root, err := v2Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v2Root, alphaRoot)
}

func Test_BlindedCapellaToV1Alpha1SignedBlock(t *testing.T) {
	v2Block := util.HydrateV2SignedBlindedBeaconBlockCapella(&zondpbv2.SignedBlindedBeaconBlockCapella{})
	v2Block.Message.Slot = slot
	v2Block.Message.ProposerIndex = validatorIndex
	v2Block.Message.ParentRoot = parentRoot
	v2Block.Message.StateRoot = stateRoot
	v2Block.Message.Body.RandaoReveal = randaoReveal
	v2Block.Message.Body.Eth1Data = &zondpbv1.Eth1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}
	syncCommitteeBits := bitfield.NewBitvector16()
	syncCommitteeBits.SetBitAt(100, true)
	v2Block.Message.Body.SyncAggregate = &zondpbv1.SyncAggregate{
		SyncCommitteeBits:       syncCommitteeBits,
		SyncCommitteeSignatures: [][]byte{signature},
	}
	v2Block.Message.Body.ExecutionPayloadHeader = &enginev1.ExecutionPayloadHeaderCapella{
		ParentHash:       parentHash,
		FeeRecipient:     feeRecipient,
		StateRoot:        stateRoot,
		ReceiptsRoot:     receiptsRoot,
		LogsBloom:        logsBloom,
		PrevRandao:       prevRandao,
		BlockNumber:      blockNumber,
		GasLimit:         gasLimit,
		GasUsed:          gasUsed,
		Timestamp:        timestamp,
		ExtraData:        extraData,
		BaseFeePerGas:    baseFeePerGas,
		BlockHash:        blockHash,
		TransactionsRoot: transactionsRoot,
		WithdrawalsRoot:  withdrawalsRoot,
	}
	v2Block.Signature = signature

	alphaBlock, err := BlindedCapellaToV1Alpha1SignedBlock(v2Block)
	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	v2Root, err := v2Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v2Root, alphaRoot)
}
