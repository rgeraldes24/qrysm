package blocks

import (
	"testing"

	"github.com/prysmaticlabs/go-bitfield"
	dilithium2 "github.com/theQRL/go-qrllib/dilithium"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zond "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
)

type fields struct {
	root                        [32]byte
	sig                         [dilithium2.CryptoBytes]byte
	deposits                    []*zond.Deposit
	atts                        []*zond.Attestation
	proposerSlashings           []*zond.ProposerSlashing
	attesterSlashings           []*zond.AttesterSlashing
	voluntaryExits              []*zond.SignedVoluntaryExit
	syncAggregate               *zond.SyncAggregate
	execPayload                 *enginev1.ExecutionPayload
	execPayloadHeader           *enginev1.ExecutionPayloadHeader
	dilithiumToExecutionChanges []*zond.SignedDilithiumToExecutionChange
}

func Test_SignedBeaconBlock_Proto(t *testing.T) {
	f := getFields()

	t.Run("Capella", func(t *testing.T) {
		expectedBlock := &zond.SignedBeaconBlock{
			Block: &zond.BeaconBlock{
				Slot:          128,
				ProposerIndex: 128,
				ParentRoot:    f.root[:],
				StateRoot:     f.root[:],
				Body:          bodyPbCapella(),
			},
			Signature: f.sig[:],
		}
		block := &SignedBeaconBlock{
			version: version.Capella,
			block: &BeaconBlock{
				version:       version.Capella,
				slot:          128,
				proposerIndex: 128,
				parentRoot:    f.root,
				stateRoot:     f.root,
				body:          bodyCapella(t),
			},
			signature: f.sig,
		}

		result, err := block.Proto()
		require.NoError(t, err)
		resultBlock, ok := result.(*zond.SignedBeaconBlock)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBlock.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
	t.Run("CapellaBlind", func(t *testing.T) {
		expectedBlock := &zond.SignedBlindedBeaconBlock{
			Block: &zond.BlindedBeaconBlock{
				Slot:          128,
				ProposerIndex: 128,
				ParentRoot:    f.root[:],
				StateRoot:     f.root[:],
				Body:          bodyPbBlindedCapella(),
			},
			Signature: f.sig[:],
		}
		block := &SignedBeaconBlock{
			version: version.Capella,
			block: &BeaconBlock{
				version:       version.Capella,
				slot:          128,
				proposerIndex: 128,
				parentRoot:    f.root,
				stateRoot:     f.root,
				body:          bodyBlindedCapella(t),
			},
			signature: f.sig,
		}

		result, err := block.Proto()
		require.NoError(t, err)
		resultBlock, ok := result.(*zond.SignedBlindedBeaconBlock)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBlock.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
}

func Test_BeaconBlock_Proto(t *testing.T) {
	f := getFields()

	t.Run("Capella", func(t *testing.T) {
		expectedBlock := &zond.BeaconBlock{
			Slot:          128,
			ProposerIndex: 128,
			ParentRoot:    f.root[:],
			StateRoot:     f.root[:],
			Body:          bodyPbCapella(),
		}
		block := &BeaconBlock{
			version:       version.Capella,
			slot:          128,
			proposerIndex: 128,
			parentRoot:    f.root,
			stateRoot:     f.root,
			body:          bodyCapella(t),
		}

		result, err := block.Proto()
		require.NoError(t, err)
		resultBlock, ok := result.(*zond.BeaconBlock)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBlock.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
	t.Run("CapellaBlind", func(t *testing.T) {
		expectedBlock := &zond.BlindedBeaconBlock{
			Slot:          128,
			ProposerIndex: 128,
			ParentRoot:    f.root[:],
			StateRoot:     f.root[:],
			Body:          bodyPbBlindedCapella(),
		}
		block := &BeaconBlock{
			version:       version.Capella,
			slot:          128,
			proposerIndex: 128,
			parentRoot:    f.root,
			stateRoot:     f.root,
			body:          bodyBlindedCapella(t),
		}

		result, err := block.Proto()
		require.NoError(t, err)
		resultBlock, ok := result.(*zond.BlindedBeaconBlock)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBlock.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
}

func Test_BeaconBlockBody_Proto(t *testing.T) {
	t.Run("Capella", func(t *testing.T) {
		expectedBody := bodyPbCapella()
		body := bodyCapella(t)
		result, err := body.Proto()
		require.NoError(t, err)
		resultBlock, ok := result.(*zond.BeaconBlockBody)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBody.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
	t.Run("CapellaBlind", func(t *testing.T) {
		expectedBody := bodyPbBlindedCapella()
		body := bodyBlindedCapella(t)
		result, err := body.Proto()
		require.NoError(t, err)
		resultBlock, ok := result.(*zond.BlindedBeaconBlockBody)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBody.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
	t.Run("Capella - wrong payload type", func(t *testing.T) {
		body := bodyCapella(t)
		body.executionPayload = &executionPayloadHeader{}
		_, err := body.Proto()
		require.ErrorIs(t, err, errPayloadWrongType)
	})
	t.Run("CapellaBlind - wrong payload type", func(t *testing.T) {
		body := bodyBlindedCapella(t)
		body.executionPayloadHeader = &executionPayload{}
		_, err := body.Proto()
		require.ErrorIs(t, err, errPayloadHeaderWrongType)
	})
}

func Test_initSignedBlockFromProtoCapella(t *testing.T) {
	f := getFields()
	expectedBlock := &zond.SignedBeaconBlock{
		Block: &zond.BeaconBlock{
			Slot:          128,
			ProposerIndex: 128,
			ParentRoot:    f.root[:],
			StateRoot:     f.root[:],
			Body:          bodyPbCapella(),
		},
		Signature: f.sig[:],
	}
	resultBlock, err := initSignedBlockFromProtoCapella(expectedBlock)
	require.NoError(t, err)
	resultHTR, err := resultBlock.block.HashTreeRoot()
	require.NoError(t, err)
	expectedHTR, err := expectedBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, expectedHTR, resultHTR)
	assert.DeepEqual(t, expectedBlock.Signature, resultBlock.signature[:])
}

func Test_initBlindedSignedBlockFromProtoCapella(t *testing.T) {
	f := getFields()
	expectedBlock := &zond.SignedBlindedBeaconBlock{
		Block: &zond.BlindedBeaconBlock{
			Slot:          128,
			ProposerIndex: 128,
			ParentRoot:    f.root[:],
			StateRoot:     f.root[:],
			Body:          bodyPbBlindedCapella(),
		},
		Signature: f.sig[:],
	}
	resultBlock, err := initBlindedSignedBlockFromProtoCapella(expectedBlock)
	require.NoError(t, err)
	resultHTR, err := resultBlock.block.HashTreeRoot()
	require.NoError(t, err)
	expectedHTR, err := expectedBlock.Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, expectedHTR, resultHTR)
	assert.DeepEqual(t, expectedBlock.Signature, resultBlock.signature[:])
}

func Test_initBlockFromProtoCapella(t *testing.T) {
	f := getFields()
	expectedBlock := &zond.BeaconBlock{
		Slot:          128,
		ProposerIndex: 128,
		ParentRoot:    f.root[:],
		StateRoot:     f.root[:],
		Body:          bodyPbCapella(),
	}
	resultBlock, err := initBlockFromProtoCapella(expectedBlock)
	require.NoError(t, err)
	resultHTR, err := resultBlock.HashTreeRoot()
	require.NoError(t, err)
	expectedHTR, err := expectedBlock.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, expectedHTR, resultHTR)
}

func Test_initBlockFromProtoBlindedCapella(t *testing.T) {
	f := getFields()
	expectedBlock := &zond.BlindedBeaconBlock{
		Slot:          128,
		ProposerIndex: 128,
		ParentRoot:    f.root[:],
		StateRoot:     f.root[:],
		Body:          bodyPbBlindedCapella(),
	}
	resultBlock, err := initBlindedBlockFromProtoCapella(expectedBlock)
	require.NoError(t, err)
	resultHTR, err := resultBlock.HashTreeRoot()
	require.NoError(t, err)
	expectedHTR, err := expectedBlock.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, expectedHTR, resultHTR)
}

func Test_initBlockBodyFromProtoCapella(t *testing.T) {
	expectedBody := bodyPbCapella()
	resultBody, err := initBlockBodyFromProtoCapella(expectedBody)
	require.NoError(t, err)
	resultHTR, err := resultBody.HashTreeRoot()
	require.NoError(t, err)
	expectedHTR, err := expectedBody.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, expectedHTR, resultHTR)
}

func Test_initBlockBodyFromProtoBlindedCapella(t *testing.T) {
	expectedBody := bodyPbBlindedCapella()
	resultBody, err := initBlindedBlockBodyFromProtoCapella(expectedBody)
	require.NoError(t, err)
	resultHTR, err := resultBody.HashTreeRoot()
	require.NoError(t, err)
	expectedHTR, err := expectedBody.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, expectedHTR, resultHTR)
}

func bodyPbBellatrix() *zond.BeaconBlockBody {
	f := getFields()
	return &zond.BeaconBlockBody{
		RandaoReveal: f.sig[:],
		Zond1Data: &zond.Zond1Data{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		Graffiti:          f.root[:],
		ProposerSlashings: f.proposerSlashings,
		AttesterSlashings: f.attesterSlashings,
		Attestations:      f.atts,
		Deposits:          f.deposits,
		VoluntaryExits:    f.voluntaryExits,
		SyncAggregate:     f.syncAggregate,
		ExecutionPayload:  f.execPayload,
	}
}

func bodyPbCapella() *zond.BeaconBlockBody {
	f := getFields()
	return &zond.BeaconBlockBody{
		RandaoReveal: f.sig[:],
		Zond1Data: &zond.Zond1Data{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		Graffiti:                    f.root[:],
		ProposerSlashings:           f.proposerSlashings,
		AttesterSlashings:           f.attesterSlashings,
		Attestations:                f.atts,
		Deposits:                    f.deposits,
		VoluntaryExits:              f.voluntaryExits,
		SyncAggregate:               f.syncAggregate,
		ExecutionPayload:            f.execPayload,
		DilithiumToExecutionChanges: f.dilithiumToExecutionChanges,
	}
}

func bodyPbBlindedCapella() *zond.BlindedBeaconBlockBody {
	f := getFields()
	return &zond.BlindedBeaconBlockBody{
		RandaoReveal: f.sig[:],
		Zond1Data: &zond.Zond1Data{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		Graffiti:                    f.root[:],
		ProposerSlashings:           f.proposerSlashings,
		AttesterSlashings:           f.attesterSlashings,
		Attestations:                f.atts,
		Deposits:                    f.deposits,
		VoluntaryExits:              f.voluntaryExits,
		SyncAggregate:               f.syncAggregate,
		ExecutionPayloadHeader:      f.execPayloadHeader,
		DilithiumToExecutionChanges: f.dilithiumToExecutionChanges,
	}
}

func bodyCapella(t *testing.T) *BeaconBlockBody {
	f := getFields()
	p, err := WrappedExecutionPayload(f.execPayload, 0)
	require.NoError(t, err)
	return &BeaconBlockBody{
		version:      version.Capella,
		randaoReveal: f.sig,
		zond1Data: &zond.Zond1Data{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		graffiti:                    f.root,
		proposerSlashings:           f.proposerSlashings,
		attesterSlashings:           f.attesterSlashings,
		attestations:                f.atts,
		deposits:                    f.deposits,
		voluntaryExits:              f.voluntaryExits,
		syncAggregate:               f.syncAggregate,
		executionPayload:            p,
		dilithiumToExecutionChanges: f.dilithiumToExecutionChanges,
	}
}

func bodyBlindedCapella(t *testing.T) *BeaconBlockBody {
	f := getFields()
	ph, err := WrappedExecutionPayloadHeader(f.execPayloadHeader, 0)
	require.NoError(t, err)
	return &BeaconBlockBody{
		version:      version.Capella,
		isBlinded:    true,
		randaoReveal: f.sig,
		zond1Data: &zond.Zond1Data{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		graffiti:                    f.root,
		proposerSlashings:           f.proposerSlashings,
		attesterSlashings:           f.attesterSlashings,
		attestations:                f.atts,
		deposits:                    f.deposits,
		voluntaryExits:              f.voluntaryExits,
		syncAggregate:               f.syncAggregate,
		executionPayloadHeader:      ph,
		dilithiumToExecutionChanges: f.dilithiumToExecutionChanges,
	}
}

func getFields() fields {
	b20 := make([]byte, 20)
	b48 := make([]byte, 48)
	b256 := make([]byte, 256)
	var root [32]byte
	var sig [dilithium2.CryptoBytes]byte
	b20[0], b20[5], b20[10] = 'q', 'u', 'x'
	b48[0], b48[5], b48[10] = 'b', 'a', 'r'
	b256[0], b256[5], b256[10] = 'x', 'y', 'z'
	root[0], root[5], root[10] = 'a', 'b', 'c'
	sig[0], sig[5], sig[10] = 'd', 'e', 'f'
	deposits := make([]*zond.Deposit, 16)
	for i := range deposits {
		deposits[i] = &zond.Deposit{}
		deposits[i].Proof = make([][]byte, 33)
		for j := range deposits[i].Proof {
			deposits[i].Proof[j] = root[:]
		}
		deposits[i].Data = &zond.Deposit_Data{
			PublicKey:             b48,
			WithdrawalCredentials: root[:],
			Amount:                128,
			Signature:             sig[:],
		}
	}
	atts := make([]*zond.Attestation, 128)
	for i := range atts {
		atts[i] = &zond.Attestation{}
		atts[i].Signatures = [][]byte{sig[:]}
		atts[i].AggregationBits = bitfield.NewBitlist(1)
		atts[i].Data = &zond.AttestationData{
			Slot:            128,
			CommitteeIndex:  128,
			BeaconBlockRoot: root[:],
			Source: &zond.Checkpoint{
				Epoch: 128,
				Root:  root[:],
			},
			Target: &zond.Checkpoint{
				Epoch: 128,
				Root:  root[:],
			},
		}
	}
	proposerSlashing := &zond.ProposerSlashing{
		Header_1: &zond.SignedBeaconBlockHeader{
			Header: &zond.BeaconBlockHeader{
				Slot:          128,
				ProposerIndex: 128,
				ParentRoot:    root[:],
				StateRoot:     root[:],
				BodyRoot:      root[:],
			},
			Signature: sig[:],
		},
		Header_2: &zond.SignedBeaconBlockHeader{
			Header: &zond.BeaconBlockHeader{
				Slot:          128,
				ProposerIndex: 128,
				ParentRoot:    root[:],
				StateRoot:     root[:],
				BodyRoot:      root[:],
			},
			Signature: sig[:],
		},
	}
	attesterSlashing := &zond.AttesterSlashing{
		Attestation_1: &zond.IndexedAttestation{
			AttestingIndices: []uint64{1, 2, 8},
			Data: &zond.AttestationData{
				Slot:            128,
				CommitteeIndex:  128,
				BeaconBlockRoot: root[:],
				Source: &zond.Checkpoint{
					Epoch: 128,
					Root:  root[:],
				},
				Target: &zond.Checkpoint{
					Epoch: 128,
					Root:  root[:],
				},
			},
			Signatures: [][]byte{sig[:]},
		},
		Attestation_2: &zond.IndexedAttestation{
			AttestingIndices: []uint64{1, 2, 8},
			Data: &zond.AttestationData{
				Slot:            128,
				CommitteeIndex:  128,
				BeaconBlockRoot: root[:],
				Source: &zond.Checkpoint{
					Epoch: 128,
					Root:  root[:],
				},
				Target: &zond.Checkpoint{
					Epoch: 128,
					Root:  root[:],
				},
			},
			Signatures: [][]byte{sig[:]},
		},
	}
	voluntaryExit := &zond.SignedVoluntaryExit{
		Exit: &zond.VoluntaryExit{
			Epoch:          128,
			ValidatorIndex: 128,
		},
		Signature: sig[:],
	}
	syncCommitteeBits := bitfield.NewBitvector512()
	syncCommitteeBits.SetBitAt(1, true)
	syncCommitteeBits.SetBitAt(2, true)
	syncCommitteeBits.SetBitAt(8, true)
	syncAggregate := &zond.SyncAggregate{
		SyncCommitteeBits:       syncCommitteeBits,
		SyncCommitteeSignatures: [][]byte{sig[:]},
	}

	execPayload := &enginev1.ExecutionPayload{
		ParentHash:    root[:],
		FeeRecipient:  b20,
		StateRoot:     root[:],
		ReceiptsRoot:  root[:],
		LogsBloom:     b256,
		PrevRandao:    root[:],
		BlockNumber:   128,
		GasLimit:      128,
		GasUsed:       128,
		Timestamp:     128,
		ExtraData:     root[:],
		BaseFeePerGas: root[:],
		BlockHash:     root[:],
		Transactions: [][]byte{
			[]byte("transaction1"),
			[]byte("transaction2"),
			[]byte("transaction8"),
		},
		Withdrawals: []*enginev1.Withdrawal{
			{
				Index:   128,
				Address: b20,
				Amount:  128,
			},
		},
	}
	execPayloadHeader := &enginev1.ExecutionPayloadHeader{
		ParentHash:       root[:],
		FeeRecipient:     b20,
		StateRoot:        root[:],
		ReceiptsRoot:     root[:],
		LogsBloom:        b256,
		PrevRandao:       root[:],
		BlockNumber:      128,
		GasLimit:         128,
		GasUsed:          128,
		Timestamp:        128,
		ExtraData:        root[:],
		BaseFeePerGas:    root[:],
		BlockHash:        root[:],
		TransactionsRoot: root[:],
		WithdrawalsRoot:  root[:],
	}
	dilithiumToExecutionChanges := []*zond.SignedDilithiumToExecutionChange{{
		Message: &zond.DilithiumToExecutionChange{
			ValidatorIndex:      128,
			FromDilithiumPubkey: b48,
			ToExecutionAddress:  b20,
		},
		Signature: sig[:],
	}}

	return fields{
		root:                        root,
		sig:                         sig,
		deposits:                    deposits,
		atts:                        atts,
		proposerSlashings:           []*zond.ProposerSlashing{proposerSlashing},
		attesterSlashings:           []*zond.AttesterSlashing{attesterSlashing},
		voluntaryExits:              []*zond.SignedVoluntaryExit{voluntaryExit},
		syncAggregate:               syncAggregate,
		execPayload:                 execPayload,
		execPayloadHeader:           execPayloadHeader,
		dilithiumToExecutionChanges: dilithiumToExecutionChanges,
	}
}
