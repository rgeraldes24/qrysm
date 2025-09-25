package blocks

import (
	"testing"

	"github.com/theQRL/go-bitfield"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/runtime/version"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

type fields struct {
	root                      [32]byte
	sig                       [field_params.MLDSA87SignatureLength]byte
	deposits                  []*qrysmpb.Deposit
	atts                      []*qrysmpb.Attestation
	proposerSlashings         []*qrysmpb.ProposerSlashing
	attesterSlashings         []*qrysmpb.AttesterSlashing
	voluntaryExits            []*qrysmpb.SignedVoluntaryExit
	syncAggregate             *qrysmpb.SyncAggregate
	execPayloadCapella        *enginev1.ExecutionPayloadCapella
	execPayloadHeaderCapella  *enginev1.ExecutionPayloadHeaderCapella
	mlDSA87ToExecutionChanges []*qrysmpb.SignedMLDSA87ToExecutionChange
}

func Test_SignedBeaconBlock_Proto(t *testing.T) {
	f := getFields()

	t.Run("Capella", func(t *testing.T) {
		expectedBlock := &qrysmpb.SignedBeaconBlockCapella{
			Block: &qrysmpb.BeaconBlockCapella{
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
		resultBlock, ok := result.(*qrysmpb.SignedBeaconBlockCapella)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBlock.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
	t.Run("CapellaBlind", func(t *testing.T) {
		expectedBlock := &qrysmpb.SignedBlindedBeaconBlockCapella{
			Block: &qrysmpb.BlindedBeaconBlockCapella{
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
		resultBlock, ok := result.(*qrysmpb.SignedBlindedBeaconBlockCapella)
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
		expectedBlock := &qrysmpb.BeaconBlockCapella{
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
		resultBlock, ok := result.(*qrysmpb.BeaconBlockCapella)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBlock.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
	t.Run("CapellaBlind", func(t *testing.T) {
		expectedBlock := &qrysmpb.BlindedBeaconBlockCapella{
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
		resultBlock, ok := result.(*qrysmpb.BlindedBeaconBlockCapella)
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
		resultBlock, ok := result.(*qrysmpb.BeaconBlockBodyCapella)
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
		resultBlock, ok := result.(*qrysmpb.BlindedBeaconBlockBodyCapella)
		require.Equal(t, true, ok)
		resultHTR, err := resultBlock.HashTreeRoot()
		require.NoError(t, err)
		expectedHTR, err := expectedBody.HashTreeRoot()
		require.NoError(t, err)
		assert.DeepEqual(t, expectedHTR, resultHTR)
	})
	t.Run("Capella - wrong payload type", func(t *testing.T) {
		body := bodyCapella(t)
		body.executionPayload = &executionPayloadHeaderCapella{}
		_, err := body.Proto()
		require.ErrorIs(t, err, errPayloadWrongType)
	})
	t.Run("CapellaBlind - wrong payload type", func(t *testing.T) {
		body := bodyBlindedCapella(t)
		body.executionPayloadHeader = &executionPayloadCapella{}
		_, err := body.Proto()
		require.ErrorIs(t, err, errPayloadHeaderWrongType)
	})
}

func Test_initSignedBlockFromProtoCapella(t *testing.T) {
	f := getFields()
	expectedBlock := &qrysmpb.SignedBeaconBlockCapella{
		Block: &qrysmpb.BeaconBlockCapella{
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
	expectedBlock := &qrysmpb.SignedBlindedBeaconBlockCapella{
		Block: &qrysmpb.BlindedBeaconBlockCapella{
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
	expectedBlock := &qrysmpb.BeaconBlockCapella{
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
	expectedBlock := &qrysmpb.BlindedBeaconBlockCapella{
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

func bodyPbCapella() *qrysmpb.BeaconBlockBodyCapella {
	f := getFields()
	return &qrysmpb.BeaconBlockBodyCapella{
		RandaoReveal: f.sig[:],
		ExecutionData: &qrysmpb.ExecutionData{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		Graffiti:                  f.root[:],
		ProposerSlashings:         f.proposerSlashings,
		AttesterSlashings:         f.attesterSlashings,
		Attestations:              f.atts,
		Deposits:                  f.deposits,
		VoluntaryExits:            f.voluntaryExits,
		SyncAggregate:             f.syncAggregate,
		ExecutionPayload:          f.execPayloadCapella,
		Mldsa87ToExecutionChanges: f.mlDSA87ToExecutionChanges,
	}
}

func bodyPbBlindedCapella() *qrysmpb.BlindedBeaconBlockBodyCapella {
	f := getFields()
	return &qrysmpb.BlindedBeaconBlockBodyCapella{
		RandaoReveal: f.sig[:],
		ExecutionData: &qrysmpb.ExecutionData{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		Graffiti:                  f.root[:],
		ProposerSlashings:         f.proposerSlashings,
		AttesterSlashings:         f.attesterSlashings,
		Attestations:              f.atts,
		Deposits:                  f.deposits,
		VoluntaryExits:            f.voluntaryExits,
		SyncAggregate:             f.syncAggregate,
		ExecutionPayloadHeader:    f.execPayloadHeaderCapella,
		Mldsa87ToExecutionChanges: f.mlDSA87ToExecutionChanges,
	}
}

func bodyCapella(t *testing.T) *BeaconBlockBody {
	f := getFields()
	p, err := WrappedExecutionPayloadCapella(f.execPayloadCapella, 0)
	require.NoError(t, err)
	return &BeaconBlockBody{
		version:      version.Capella,
		randaoReveal: f.sig,
		executionData: &qrysmpb.ExecutionData{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		graffiti:                  f.root,
		proposerSlashings:         f.proposerSlashings,
		attesterSlashings:         f.attesterSlashings,
		attestations:              f.atts,
		deposits:                  f.deposits,
		voluntaryExits:            f.voluntaryExits,
		syncAggregate:             f.syncAggregate,
		executionPayload:          p,
		mlDSA87ToExecutionChanges: f.mlDSA87ToExecutionChanges,
	}
}

func bodyBlindedCapella(t *testing.T) *BeaconBlockBody {
	f := getFields()
	ph, err := WrappedExecutionPayloadHeaderCapella(f.execPayloadHeaderCapella, 0)
	require.NoError(t, err)
	return &BeaconBlockBody{
		version:      version.Capella,
		isBlinded:    true,
		randaoReveal: f.sig,
		executionData: &qrysmpb.ExecutionData{
			DepositRoot:  f.root[:],
			DepositCount: 128,
			BlockHash:    f.root[:],
		},
		graffiti:                  f.root,
		proposerSlashings:         f.proposerSlashings,
		attesterSlashings:         f.attesterSlashings,
		attestations:              f.atts,
		deposits:                  f.deposits,
		voluntaryExits:            f.voluntaryExits,
		syncAggregate:             f.syncAggregate,
		executionPayloadHeader:    ph,
		mlDSA87ToExecutionChanges: f.mlDSA87ToExecutionChanges,
	}
}

func getFields() fields {
	b20 := make([]byte, 20)
	b2592 := make([]byte, 2592)
	b256 := make([]byte, 256)
	var root [32]byte
	var sig [field_params.MLDSA87SignatureLength]byte
	b20[0], b20[5], b20[10] = 'q', 'u', 'x'
	b2592[0], b2592[5], b2592[10] = 'b', 'a', 'r'
	b256[0], b256[5], b256[10] = 'x', 'y', 'z'
	root[0], root[5], root[10] = 'a', 'b', 'c'
	sig[0], sig[5], sig[10] = 'd', 'e', 'f'
	deposits := make([]*qrysmpb.Deposit, 16)
	for i := range deposits {
		deposits[i] = &qrysmpb.Deposit{}
		deposits[i].Proof = make([][]byte, 33)
		for j := range deposits[i].Proof {
			deposits[i].Proof[j] = root[:]
		}
		deposits[i].Data = &qrysmpb.Deposit_Data{
			PublicKey:             b2592,
			WithdrawalCredentials: root[:],
			Amount:                128,
			Signature:             sig[:],
		}
	}
	atts := make([]*qrysmpb.Attestation, 128)
	for i := range atts {
		atts[i] = &qrysmpb.Attestation{}
		atts[i].Signatures = [][]byte{sig[:]}
		atts[i].AggregationBits = bitfield.NewBitlist(1)
		atts[i].Data = &qrysmpb.AttestationData{
			Slot:            128,
			CommitteeIndex:  128,
			BeaconBlockRoot: root[:],
			Source: &qrysmpb.Checkpoint{
				Epoch: 128,
				Root:  root[:],
			},
			Target: &qrysmpb.Checkpoint{
				Epoch: 128,
				Root:  root[:],
			},
		}
	}
	proposerSlashing := &qrysmpb.ProposerSlashing{
		Header_1: &qrysmpb.SignedBeaconBlockHeader{
			Header: &qrysmpb.BeaconBlockHeader{
				Slot:          128,
				ProposerIndex: 128,
				ParentRoot:    root[:],
				StateRoot:     root[:],
				BodyRoot:      root[:],
			},
			Signature: sig[:],
		},
		Header_2: &qrysmpb.SignedBeaconBlockHeader{
			Header: &qrysmpb.BeaconBlockHeader{
				Slot:          128,
				ProposerIndex: 128,
				ParentRoot:    root[:],
				StateRoot:     root[:],
				BodyRoot:      root[:],
			},
			Signature: sig[:],
		},
	}
	attesterSlashing := &qrysmpb.AttesterSlashing{
		Attestation_1: &qrysmpb.IndexedAttestation{
			AttestingIndices: []uint64{1, 2, 8},
			Data: &qrysmpb.AttestationData{
				Slot:            128,
				CommitteeIndex:  128,
				BeaconBlockRoot: root[:],
				Source: &qrysmpb.Checkpoint{
					Epoch: 128,
					Root:  root[:],
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 128,
					Root:  root[:],
				},
			},
			Signatures: [][]byte{sig[:]},
		},
		Attestation_2: &qrysmpb.IndexedAttestation{
			AttestingIndices: []uint64{1, 2, 8},
			Data: &qrysmpb.AttestationData{
				Slot:            128,
				CommitteeIndex:  128,
				BeaconBlockRoot: root[:],
				Source: &qrysmpb.Checkpoint{
					Epoch: 128,
					Root:  root[:],
				},
				Target: &qrysmpb.Checkpoint{
					Epoch: 128,
					Root:  root[:],
				},
			},
			Signatures: [][]byte{sig[:]},
		},
	}
	voluntaryExit := &qrysmpb.SignedVoluntaryExit{
		Exit: &qrysmpb.VoluntaryExit{
			Epoch:          128,
			ValidatorIndex: 128,
		},
		Signature: sig[:],
	}
	syncCommitteeBits := bitfield.NewBitvector16()
	syncCommitteeBits.SetBitAt(1, true)
	syncCommitteeBits.SetBitAt(2, true)
	syncCommitteeBits.SetBitAt(8, true)
	syncAggregate := &qrysmpb.SyncAggregate{
		SyncCommitteeBits:       syncCommitteeBits,
		SyncCommitteeSignatures: [][]byte{sig[:]},
	}
	execPayloadCapella := &enginev1.ExecutionPayloadCapella{
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
	execPayloadHeaderCapella := &enginev1.ExecutionPayloadHeaderCapella{
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
	mlDSA87ToExecutionChanges := []*qrysmpb.SignedMLDSA87ToExecutionChange{{
		Message: &qrysmpb.MLDSA87ToExecutionChange{
			ValidatorIndex:     128,
			FromMldsa87Pubkey:  b2592,
			ToExecutionAddress: b20,
		},
		Signature: sig[:],
	}}

	return fields{
		root:                      root,
		sig:                       sig,
		deposits:                  deposits,
		atts:                      atts,
		proposerSlashings:         []*qrysmpb.ProposerSlashing{proposerSlashing},
		attesterSlashings:         []*qrysmpb.AttesterSlashing{attesterSlashing},
		voluntaryExits:            []*qrysmpb.SignedVoluntaryExit{voluntaryExit},
		syncAggregate:             syncAggregate,
		execPayloadCapella:        execPayloadCapella,
		execPayloadHeaderCapella:  execPayloadHeaderCapella,
		mlDSA87ToExecutionChanges: mlDSA87ToExecutionChanges,
	}
}
