package migration

import (
	"testing"

	"github.com/prysmaticlabs/go-bitfield"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpbalpha "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

var (
	slot              = primitives.Slot(1)
	epoch             = primitives.Epoch(1)
	validatorIndex    = primitives.ValidatorIndex(1)
	committeeIndex    = primitives.CommitteeIndex(1)
	depositCount      = uint64(2)
	attestingIndices  = []uint64{1, 2}
	blockNumber       = uint64(10)
	gasLimit          = uint64(10)
	gasUsed           = uint64(10)
	timestamp         = uint64(10)
	parentRoot        = bytesutil.PadTo([]byte("parentroot"), fieldparams.RootLength)
	stateRoot         = bytesutil.PadTo([]byte("stateroot"), fieldparams.RootLength)
	signature         = bytesutil.PadTo([]byte("signatures"), 4595)
	signatures        = [][]byte{bytesutil.PadTo([]byte("signatures"), 4595)}
	randaoReveal      = bytesutil.PadTo([]byte("randaoreveal"), 4595)
	depositRoot       = bytesutil.PadTo([]byte("depositroot"), fieldparams.RootLength)
	blockHash         = bytesutil.PadTo([]byte("blockhash"), 32)
	beaconBlockRoot   = bytesutil.PadTo([]byte("beaconblockroot"), fieldparams.RootLength)
	sourceRoot        = bytesutil.PadTo([]byte("sourceroot"), fieldparams.RootLength)
	targetRoot        = bytesutil.PadTo([]byte("targetroot"), fieldparams.RootLength)
	bodyRoot          = bytesutil.PadTo([]byte("bodyroot"), fieldparams.RootLength)
	selectionProof    = bytesutil.PadTo([]byte("selectionproof"), 96)
	parentHash        = bytesutil.PadTo([]byte("parenthash"), 32)
	feeRecipient      = bytesutil.PadTo([]byte("feerecipient"), 20)
	receiptsRoot      = bytesutil.PadTo([]byte("receiptsroot"), 32)
	logsBloom         = bytesutil.PadTo([]byte("logsbloom"), 256)
	prevRandao        = bytesutil.PadTo([]byte("prevrandao"), 32)
	extraData         = bytesutil.PadTo([]byte("extradata"), 32)
	baseFeePerGas     = bytesutil.PadTo([]byte("basefeepergas"), 32)
	transactionsRoot  = bytesutil.PadTo([]byte("transactions"), 32)
	participationBits = bitfield.Bitlist{0x01}
)

func TestBlockIfaceToV1BlockHeader(t *testing.T) {
	alphaBlock := util.HydrateSignedBeaconBlock(&zondpbalpha.SignedBeaconBlock{})
	alphaBlock.Block.Slot = slot
	alphaBlock.Block.ProposerIndex = validatorIndex
	alphaBlock.Block.ParentRoot = parentRoot
	alphaBlock.Block.StateRoot = stateRoot
	alphaBlock.Signature = signature

	wsb, err := blocks.NewSignedBeaconBlock(alphaBlock)
	require.NoError(t, err)
	v1Header, err := BlockIfaceToV1BlockHeader(wsb)
	require.NoError(t, err)
	bodyRoot, err := alphaBlock.Block.Body.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, bodyRoot[:], v1Header.Message.BodyRoot)
	assert.Equal(t, slot, v1Header.Message.Slot)
	assert.Equal(t, validatorIndex, v1Header.Message.ProposerIndex)
	assert.DeepEqual(t, parentRoot, v1Header.Message.ParentRoot)
	assert.DeepEqual(t, stateRoot, v1Header.Message.StateRoot)
	assert.DeepEqual(t, signature, v1Header.Signature)
}

func TestV1Alpha1ToV1AggregateAttAndProof(t *testing.T) {
	proof := [32]byte{1}
	att := util.HydrateAttestation(&zondpbalpha.Attestation{
		Data: &zondpbalpha.AttestationData{
			Slot: 5,
		},
	})
	alpha := &zondpbalpha.AggregateAttestationAndProof{
		AggregatorIndex: 1,
		Aggregate:       att,
		SelectionProof:  proof[:],
	}
	v1 := V1Alpha1ToV1AggregateAttAndProof(alpha)
	assert.Equal(t, v1.AggregatorIndex, primitives.ValidatorIndex(1))
	assert.DeepSSZEqual(t, v1.Aggregate.Data.Slot, att.Data.Slot)
	assert.DeepEqual(t, v1.SelectionProof, proof[:])
}

func TestV1Alpha1ToV1SignedBlock(t *testing.T) {
	alphaBlock := util.HydrateSignedBeaconBlock(&zondpbalpha.SignedBeaconBlock{})
	alphaBlock.Block.Slot = slot
	alphaBlock.Block.ProposerIndex = validatorIndex
	alphaBlock.Block.ParentRoot = parentRoot
	alphaBlock.Block.StateRoot = stateRoot
	alphaBlock.Block.Body.RandaoReveal = randaoReveal
	alphaBlock.Block.Body.Zond1Data = &zondpbalpha.Zond1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}
	alphaBlock.Signature = signature

	v1Block, err := V1Alpha1ToV1SignedBlock(alphaBlock)
	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestV1ToV1Alpha1SignedBlock(t *testing.T) {
	v1Block := util.HydrateV1SignedBeaconBlock(&zondpbv1.SignedBeaconBlock{})
	v1Block.Message.Slot = slot
	v1Block.Message.ProposerIndex = validatorIndex
	v1Block.Message.ParentRoot = parentRoot
	v1Block.Message.StateRoot = stateRoot
	v1Block.Message.Body.RandaoReveal = randaoReveal
	v1Block.Message.Body.Zond1Data = &zondpbv1.Zond1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}
	v1Block.Signature = signature

	alphaBlock, err := V1ToV1Alpha1SignedBlock(v1Block)

	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, alphaRoot)
}

func TestV1ToV1Alpha1SignedBlindedBlock(t *testing.T) {
	v1Block := util.HydrateV1SignedBlindedBeaconBlock(&zondpbv1.SignedBlindedBeaconBlock{})
	v1Block.Message.Slot = slot
	v1Block.Message.ProposerIndex = validatorIndex
	v1Block.Message.ParentRoot = parentRoot
	v1Block.Message.StateRoot = stateRoot
	v1Block.Message.Body.RandaoReveal = randaoReveal
	v1Block.Message.Body.Zond1Data = &zondpbv1.Zond1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}
	v1Block.Signature = signature

	alphaBlock, err := V1ToV1Alpha1SignedBlindedBlock(v1Block)
	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, alphaRoot)
}

func TestV1Alpha1ToV1Block(t *testing.T) {
	alphaBlock := util.HydrateBeaconBlock(&zondpbalpha.BeaconBlock{})
	alphaBlock.Slot = slot
	alphaBlock.ProposerIndex = validatorIndex
	alphaBlock.ParentRoot = parentRoot
	alphaBlock.StateRoot = stateRoot
	alphaBlock.Body.RandaoReveal = randaoReveal
	alphaBlock.Body.Zond1Data = &zondpbalpha.Zond1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}

	v1Block, err := V1Alpha1ToV1Block(alphaBlock)
	require.NoError(t, err)
	v1Root, err := v1Block.HashTreeRoot()
	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestV1Alpha1ToV1BlindedBlock(t *testing.T) {
	alphaBlock := util.HydrateBlindedBeaconBlock(&zondpbalpha.BlindedBeaconBlock{})
	alphaBlock.Slot = slot
	alphaBlock.ProposerIndex = validatorIndex
	alphaBlock.ParentRoot = parentRoot
	alphaBlock.StateRoot = stateRoot
	alphaBlock.Body.RandaoReveal = randaoReveal
	alphaBlock.Body.Zond1Data = &zondpbalpha.Zond1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}

	v1Block, err := V1Alpha1ToV1BlindedBlock(alphaBlock)
	require.NoError(t, err)
	v1Root, err := v1Block.HashTreeRoot()
	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestV1Alpha1ToV1AttSlashing(t *testing.T) {
	alphaAttestation := &zondpbalpha.IndexedAttestation{
		AttestingIndices: attestingIndices,
		Data: &zondpbalpha.AttestationData{
			Slot:            slot,
			CommitteeIndex:  committeeIndex,
			BeaconBlockRoot: beaconBlockRoot,
			Source: &zondpbalpha.Checkpoint{
				Epoch: epoch,
				Root:  sourceRoot,
			},
			Target: &zondpbalpha.Checkpoint{
				Epoch: epoch,
				Root:  targetRoot,
			},
		},
		Signatures: [][]byte{signature},
	}
	alphaSlashing := &zondpbalpha.AttesterSlashing{
		Attestation_1: alphaAttestation,
		Attestation_2: alphaAttestation,
	}

	v1Slashing := V1Alpha1ToV1AttSlashing(alphaSlashing)
	alphaRoot, err := alphaSlashing.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Slashing.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestV1Alpha1ToV1SignedHeader(t *testing.T) {
	alphaHeader := util.HydrateSignedBeaconHeader(&zondpbalpha.SignedBeaconBlockHeader{})
	alphaHeader.Header.Slot = slot
	alphaHeader.Header.ProposerIndex = validatorIndex
	alphaHeader.Header.ParentRoot = parentRoot
	alphaHeader.Header.StateRoot = stateRoot
	alphaHeader.Header.BodyRoot = bodyRoot
	alphaHeader.Signature = signature
	v1Header := V1Alpha1ToV1SignedHeader(alphaHeader)
	assert.DeepEqual(t, bodyRoot[:], v1Header.Message.BodyRoot)
	assert.Equal(t, slot, v1Header.Message.Slot)
	assert.Equal(t, validatorIndex, v1Header.Message.ProposerIndex)
	assert.DeepEqual(t, parentRoot, v1Header.Message.ParentRoot)
	assert.DeepEqual(t, stateRoot, v1Header.Message.StateRoot)
	assert.DeepEqual(t, signature, v1Header.Signature)
}

func TestV1ToV1Alpha1SignedHeader(t *testing.T) {
	v1Header := &zondpbv1.SignedBeaconBlockHeader{
		Message: &zondpbv1.BeaconBlockHeader{
			Slot:          slot,
			ProposerIndex: validatorIndex,
			ParentRoot:    parentRoot,
			StateRoot:     stateRoot,
			BodyRoot:      bodyRoot,
		},
		Signature: signature,
	}
	v1alpha1 := V1ToV1Alpha1SignedHeader(v1Header)
	assert.DeepEqual(t, bodyRoot[:], v1alpha1.Header.BodyRoot)
	assert.Equal(t, slot, v1alpha1.Header.Slot)
	assert.Equal(t, validatorIndex, v1alpha1.Header.ProposerIndex)
	assert.DeepEqual(t, parentRoot, v1alpha1.Header.ParentRoot)
	assert.DeepEqual(t, stateRoot, v1alpha1.Header.StateRoot)
	assert.DeepEqual(t, signature, v1alpha1.Signature)
}

func TestV1Alpha1ToV1ProposerSlashing(t *testing.T) {
	alphaHeader := util.HydrateSignedBeaconHeader(&zondpbalpha.SignedBeaconBlockHeader{})
	alphaHeader.Header.Slot = slot
	alphaHeader.Header.ProposerIndex = validatorIndex
	alphaHeader.Header.ParentRoot = parentRoot
	alphaHeader.Header.StateRoot = stateRoot
	alphaHeader.Header.BodyRoot = bodyRoot
	alphaHeader.Signature = signature
	alphaSlashing := &zondpbalpha.ProposerSlashing{
		Header_1: alphaHeader,
		Header_2: alphaHeader,
	}

	v1Slashing := V1Alpha1ToV1ProposerSlashing(alphaSlashing)
	alphaRoot, err := alphaSlashing.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Slashing.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestV1Alpha1ToV1Exit(t *testing.T) {
	alphaExit := &zondpbalpha.SignedVoluntaryExit{
		Exit: &zondpbalpha.VoluntaryExit{
			Epoch:          epoch,
			ValidatorIndex: validatorIndex,
		},
		Signature: signature,
	}

	v1Exit := V1Alpha1ToV1Exit(alphaExit)
	alphaRoot, err := alphaExit.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Exit.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestV1ToV1Alpha1Exit(t *testing.T) {
	v1Exit := &zondpbv1.SignedVoluntaryExit{
		Message: &zondpbv1.VoluntaryExit{
			Epoch:          epoch,
			ValidatorIndex: validatorIndex,
		},
		Signature: signature,
	}

	alphaExit := V1ToV1Alpha1Exit(v1Exit)
	alphaRoot, err := alphaExit.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Exit.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestV1ToV1Alpha1IndexedAtt(t *testing.T) {
	alphaAttestation := &zondpbalpha.IndexedAttestation{
		AttestingIndices: attestingIndices,
		Data: &zondpbalpha.AttestationData{
			Slot:            slot,
			CommitteeIndex:  committeeIndex,
			BeaconBlockRoot: beaconBlockRoot,
			Source: &zondpbalpha.Checkpoint{
				Epoch: epoch,
				Root:  sourceRoot,
			},
			Target: &zondpbalpha.Checkpoint{
				Epoch: epoch,
				Root:  targetRoot,
			},
		},
		Signatures: [][]byte{signature},
	}
	v1IdxAtt := V1Alpha1ToV1IndexedAtt(alphaAttestation)
	assert.DeepEqual(t, attestingIndices, v1IdxAtt.AttestingIndices)
	assert.Equal(t, slot, v1IdxAtt.Data.Slot)
	assert.Equal(t, committeeIndex, v1IdxAtt.Data.Index)
	assert.DeepEqual(t, beaconBlockRoot, v1IdxAtt.Data.BeaconBlockRoot)
	assert.Equal(t, epoch, v1IdxAtt.Data.Source.Epoch)
	assert.DeepEqual(t, sourceRoot, v1IdxAtt.Data.Source.Root)
	assert.Equal(t, epoch, v1IdxAtt.Data.Target.Epoch)
	assert.DeepEqual(t, targetRoot, v1IdxAtt.Data.Target.Root)
	assert.DeepEqual(t, signature, v1IdxAtt.Signatures)
}

func TestV1ToV1Alpha1AttData(t *testing.T) {
	v1AttData := &zondpbv1.AttestationData{
		Slot:            slot,
		Index:           committeeIndex,
		BeaconBlockRoot: beaconBlockRoot,
		Source: &zondpbv1.Checkpoint{
			Epoch: epoch,
			Root:  sourceRoot,
		},
		Target: &zondpbv1.Checkpoint{
			Epoch: epoch,
			Root:  targetRoot,
		},
	}
	alphaAttData := V1ToV1Alpha1AttData(v1AttData)
	assert.Equal(t, slot, alphaAttData.Slot)
	assert.Equal(t, committeeIndex, alphaAttData.CommitteeIndex)
	assert.DeepEqual(t, beaconBlockRoot[:], alphaAttData.BeaconBlockRoot)
	assert.Equal(t, epoch, alphaAttData.Source.Epoch)
	assert.DeepEqual(t, sourceRoot[:], alphaAttData.Source.Root)
	assert.Equal(t, epoch, alphaAttData.Target.Epoch)
	assert.DeepEqual(t, targetRoot[:], alphaAttData.Source.Root)
}

func TestV1Alpha1ToV1AttData(t *testing.T) {
	alphaAttData := &zondpbalpha.AttestationData{
		Slot:            slot,
		CommitteeIndex:  committeeIndex,
		BeaconBlockRoot: beaconBlockRoot,
		Source: &zondpbalpha.Checkpoint{
			Epoch: epoch,
			Root:  sourceRoot,
		},
		Target: &zondpbalpha.Checkpoint{
			Epoch: epoch,
			Root:  targetRoot,
		},
	}
	v1AttData := V1Alpha1ToV1AttData(alphaAttData)
	assert.Equal(t, slot, v1AttData.Slot)
	assert.Equal(t, committeeIndex, v1AttData.Index)
	assert.DeepEqual(t, beaconBlockRoot[:], v1AttData.BeaconBlockRoot)
	assert.Equal(t, epoch, v1AttData.Source.Epoch)
	assert.DeepEqual(t, sourceRoot[:], v1AttData.Source.Root)
	assert.Equal(t, epoch, v1AttData.Target.Epoch)
	assert.DeepEqual(t, targetRoot[:], v1AttData.Source.Root)
}

func TestV1ToV1Alpha1AttSlashing(t *testing.T) {
	v1Attestation := &zondpbv1.IndexedAttestation{
		AttestingIndices: attestingIndices,
		Data: &zondpbv1.AttestationData{
			Slot:            slot,
			Index:           committeeIndex,
			BeaconBlockRoot: beaconBlockRoot,
			Source: &zondpbv1.Checkpoint{
				Epoch: epoch,
				Root:  sourceRoot,
			},
			Target: &zondpbv1.Checkpoint{
				Epoch: epoch,
				Root:  targetRoot,
			},
		},
		Signatures: [][]byte{signature},
	}
	v1Slashing := &zondpbv1.AttesterSlashing{
		Attestation_1: v1Attestation,
		Attestation_2: v1Attestation,
	}

	alphaSlashing := V1ToV1Alpha1AttSlashing(v1Slashing)
	alphaRoot, err := alphaSlashing.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Slashing.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, alphaRoot)
}

func TestV1ToV1Alpha1ProposerSlashing(t *testing.T) {
	v1Header := &zondpbv1.SignedBeaconBlockHeader{
		Message: &zondpbv1.BeaconBlockHeader{
			Slot:          slot,
			ProposerIndex: validatorIndex,
			ParentRoot:    parentRoot,
			StateRoot:     stateRoot,
			BodyRoot:      bodyRoot,
		},
		Signature: signature,
	}
	v1Slashing := &zondpbv1.ProposerSlashing{
		SignedHeader_1: v1Header,
		SignedHeader_2: v1Header,
	}

	alphaSlashing := V1ToV1Alpha1ProposerSlashing(v1Slashing)
	alphaRoot, err := alphaSlashing.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Slashing.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestV1Alpha1ToV1Attestation(t *testing.T) {
	alphaAtt := &zondpbalpha.Attestation{
		ParticipationBits: participationBits,
		Data: &zondpbalpha.AttestationData{
			Slot:            slot,
			CommitteeIndex:  committeeIndex,
			BeaconBlockRoot: beaconBlockRoot,
			Source: &zondpbalpha.Checkpoint{
				Epoch: epoch,
				Root:  sourceRoot,
			},
			Target: &zondpbalpha.Checkpoint{
				Epoch: epoch,
				Root:  targetRoot,
			},
		},
		Signatures: [][]byte{signature},
	}

	v1Att := V1Alpha1ToV1Attestation(alphaAtt)
	v1Root, err := v1Att.HashTreeRoot()
	require.NoError(t, err)
	alphaRoot, err := alphaAtt.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, alphaRoot)
}

func TestBlockInterfaceToV1Block(t *testing.T) {
	v1Alpha1Block := util.HydrateSignedBeaconBlock(&zondpbalpha.SignedBeaconBlock{})
	v1Alpha1Block.Block.Slot = slot
	v1Alpha1Block.Block.ProposerIndex = validatorIndex
	v1Alpha1Block.Block.ParentRoot = parentRoot
	v1Alpha1Block.Block.StateRoot = stateRoot
	v1Alpha1Block.Block.Body.RandaoReveal = randaoReveal
	v1Alpha1Block.Block.Body.Zond1Data = &zondpbalpha.Zond1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}
	v1Alpha1Block.Signature = signature

	wsb, err := blocks.NewSignedBeaconBlock(v1Alpha1Block)
	require.NoError(t, err)
	v1Block, err := SignedBeaconBlock(wsb)
	require.NoError(t, err)
	v1Root, err := v1Block.HashTreeRoot()
	require.NoError(t, err)
	v1Alpha1Root, err := v1Alpha1Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, v1Alpha1Root)
}

func TestV1Alpha1ToV1Validator(t *testing.T) {
	v1Alpha1Validator := &zondpbalpha.Validator{
		PublicKey:                  []byte("pubkey"),
		WithdrawalCredentials:      []byte("withdraw"),
		EffectiveBalance:           99,
		Slashed:                    true,
		ActivationEligibilityEpoch: 1,
		ActivationEpoch:            11,
		ExitEpoch:                  111,
		WithdrawableEpoch:          1111,
	}

	v1Validator := V1Alpha1ToV1Validator(v1Alpha1Validator)
	require.NotNil(t, v1Validator)
	assert.DeepEqual(t, []byte("pubkey"), v1Validator.Pubkey)
	assert.DeepEqual(t, []byte("withdraw"), v1Validator.WithdrawalCredentials)
	assert.Equal(t, uint64(99), v1Validator.EffectiveBalance)
	assert.Equal(t, true, v1Validator.Slashed)
	assert.Equal(t, primitives.Epoch(1), v1Validator.ActivationEligibilityEpoch)
	assert.Equal(t, primitives.Epoch(11), v1Validator.ActivationEpoch)
	assert.Equal(t, primitives.Epoch(111), v1Validator.ExitEpoch)
	assert.Equal(t, primitives.Epoch(1111), v1Validator.WithdrawableEpoch)
}

func TestV1ToV1Alpha1Validator(t *testing.T) {
	v1Validator := &zondpbv1.Validator{
		Pubkey:                     []byte("pubkey"),
		WithdrawalCredentials:      []byte("withdraw"),
		EffectiveBalance:           99,
		Slashed:                    true,
		ActivationEligibilityEpoch: 1,
		ActivationEpoch:            11,
		ExitEpoch:                  111,
		WithdrawableEpoch:          1111,
	}

	v1Alpha1Validator := V1ToV1Alpha1Validator(v1Validator)
	require.NotNil(t, v1Alpha1Validator)
	assert.DeepEqual(t, []byte("pubkey"), v1Alpha1Validator.PublicKey)
	assert.DeepEqual(t, []byte("withdraw"), v1Alpha1Validator.WithdrawalCredentials)
	assert.Equal(t, uint64(99), v1Alpha1Validator.EffectiveBalance)
	assert.Equal(t, true, v1Alpha1Validator.Slashed)
	assert.Equal(t, primitives.Epoch(1), v1Alpha1Validator.ActivationEligibilityEpoch)
	assert.Equal(t, primitives.Epoch(11), v1Alpha1Validator.ActivationEpoch)
	assert.Equal(t, primitives.Epoch(111), v1Alpha1Validator.ExitEpoch)
	assert.Equal(t, primitives.Epoch(1111), v1Alpha1Validator.WithdrawableEpoch)
}

func TestSignedBeaconBlock(t *testing.T) {
	alphaBlock := util.HydrateSignedBeaconBlock(&zondpbalpha.SignedBeaconBlock{})
	alphaBlock.Block.Slot = slot
	alphaBlock.Block.ProposerIndex = validatorIndex
	alphaBlock.Block.ParentRoot = parentRoot
	alphaBlock.Block.StateRoot = stateRoot
	alphaBlock.Signature = signature

	wsb, err := blocks.NewSignedBeaconBlock(alphaBlock)
	require.NoError(t, err)
	sbb, err := SignedBeaconBlock(wsb)
	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	sbbRoot, err := sbb.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, sbbRoot)
}

func TestV1ToV1Alpha1SignedAggregateAttAndProof(t *testing.T) {
	v1Att := &zondpbv1.SignedAggregateAttestationAndProof{
		Message: &zondpbv1.AggregateAttestationAndProof{
			AggregatorIndex: 1,
			Aggregate:       util.HydrateV1Attestation(&zondpbv1.Attestation{}),
			SelectionProof:  selectionProof,
		},
		Signature: signature,
	}
	v1Alpha1Att := V1ToV1Alpha1SignedAggregateAttAndProof(v1Att)

	v1Root, err := v1Att.HashTreeRoot()
	require.NoError(t, err)
	v1Alpha1Root, err := v1Alpha1Att.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, v1Alpha1Root)
}

func TestV1Alpha1ToV1IndexedAtt(t *testing.T) {
	alphaAttestation := &zondpbalpha.IndexedAttestation{
		AttestingIndices: attestingIndices,
		Data: &zondpbalpha.AttestationData{
			Slot:            slot,
			CommitteeIndex:  committeeIndex,
			BeaconBlockRoot: beaconBlockRoot,
			Source: &zondpbalpha.Checkpoint{
				Epoch: epoch,
				Root:  sourceRoot,
			},
			Target: &zondpbalpha.Checkpoint{
				Epoch: epoch,
				Root:  targetRoot,
			},
		},
		Signatures: [][]byte{signature},
	}
	v1IdxAtt := V1Alpha1ToV1IndexedAtt(alphaAttestation)
	assert.DeepEqual(t, attestingIndices, v1IdxAtt.AttestingIndices)
	assert.Equal(t, slot, v1IdxAtt.Data.Slot)
	assert.Equal(t, committeeIndex, v1IdxAtt.Data.Index)
	assert.DeepEqual(t, beaconBlockRoot, v1IdxAtt.Data.BeaconBlockRoot)
	assert.Equal(t, epoch, v1IdxAtt.Data.Source.Epoch)
	assert.DeepEqual(t, sourceRoot, v1IdxAtt.Data.Source.Root)
	assert.Equal(t, epoch, v1IdxAtt.Data.Target.Epoch)
	assert.DeepEqual(t, targetRoot, v1IdxAtt.Data.Target.Root)
	assert.DeepEqual(t, signature, v1IdxAtt.Signatures)
}

func TestV1ToV1Alpha1Attestation(t *testing.T) {
	v1Att := util.HydrateV1Attestation(&zondpbv1.Attestation{})
	v1Alpha1Att := V1ToV1Alpha1Attestation(v1Att)

	v1Root, err := v1Att.HashTreeRoot()
	require.NoError(t, err)
	v1Alpha1Root, err := v1Alpha1Att.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, v1Alpha1Root)
}

func TestV1Alpha1ToV1SignedContributionAndProof(t *testing.T) {
	alphaContribution := &zondpbalpha.SignedContributionAndProof{
		Message: &zondpbalpha.ContributionAndProof{
			AggregatorIndex: validatorIndex,
			Contribution: &zondpbalpha.SyncCommitteeContribution{
				Slot:              slot,
				BlockRoot:         blockHash,
				SubcommitteeIndex: 1,
				ParticipationBits: bitfield.NewBitvector128(),
				Signatures:        signatures,
			},
			SelectionProof: signature,
		},
		Signature: signature,
	}
	v1Contribution := V1Alpha1ToV1SignedContributionAndProof(alphaContribution)
	require.NotNil(t, v1Contribution)
	require.NotNil(t, v1Contribution.Message)
	require.NotNil(t, v1Contribution.Message.Contribution)
	assert.DeepEqual(t, signature, v1Contribution.Signature)
	msg := v1Contribution.Message
	assert.Equal(t, validatorIndex, msg.AggregatorIndex)
	assert.DeepEqual(t, signature, msg.SelectionProof)
	contrib := msg.Contribution
	assert.Equal(t, slot, contrib.Slot)
	assert.DeepEqual(t, blockHash, contrib.BeaconBlockRoot)
	assert.Equal(t, uint64(1), contrib.SubcommitteeIndex)
	assert.DeepEqual(t, bitfield.NewBitvector128(), contrib.ParticipationBits)
	assert.DeepEqual(t, signatures, contrib.Signatures)
}

func TestV1Alpha1BeaconBlockToV1Blinded(t *testing.T) {
	alphaBlock := util.HydrateBeaconBlock(&zondpbalpha.BeaconBlock{})
	alphaBlock.Slot = slot
	alphaBlock.ProposerIndex = validatorIndex
	alphaBlock.ParentRoot = parentRoot
	alphaBlock.StateRoot = stateRoot
	alphaBlock.Body.RandaoReveal = randaoReveal
	alphaBlock.Body.Zond1Data = &zondpbalpha.Zond1Data{
		DepositRoot:  depositRoot,
		DepositCount: depositCount,
		BlockHash:    blockHash,
	}
	syncCommitteeBits := bitfield.NewBitvector512()
	syncCommitteeBits.SetBitAt(100, true)
	alphaBlock.Body.SyncAggregate = &zondpbalpha.SyncAggregate{
		SyncCommitteeBits:       syncCommitteeBits,
		SyncCommitteeSignatures: [][]byte{signature},
	}
	alphaBlock.Body.ExecutionPayload.Transactions = [][]byte{[]byte("transaction1"), []byte("transaction2")}

	v1Block, err := V1Alpha1BeaconBlockToV1Blinded(alphaBlock)
	require.NoError(t, err)
	alphaRoot, err := alphaBlock.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Block.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func TestBeaconStateToProto(t *testing.T) {
	source, err := util.NewBeaconState(util.FillRootsNaturalOpt, func(state *zondpbalpha.BeaconState) error {
		state.GenesisTime = 1
		state.GenesisValidatorsRoot = bytesutil.PadTo([]byte("genesisvalidatorsroot"), 32)
		state.Slot = 2
		state.Fork = &zondpbalpha.Fork{
			PreviousVersion: bytesutil.PadTo([]byte("123"), 4),
			CurrentVersion:  bytesutil.PadTo([]byte("456"), 4),
			Epoch:           3,
		}
		state.LatestBlockHeader = &zondpbalpha.BeaconBlockHeader{
			Slot:          4,
			ProposerIndex: 5,
			ParentRoot:    bytesutil.PadTo([]byte("lbhparentroot"), 32),
			StateRoot:     bytesutil.PadTo([]byte("lbhstateroot"), 32),
			BodyRoot:      bytesutil.PadTo([]byte("lbhbodyroot"), 32),
		}
		state.BlockRoots = [][]byte{bytesutil.PadTo([]byte("blockroots"), 32)}
		state.StateRoots = [][]byte{bytesutil.PadTo([]byte("stateroots"), 32)}
		state.HistoricalRoots = [][]byte{bytesutil.PadTo([]byte("historicalroots"), 32)}
		state.Zond1Data = &zondpbalpha.Zond1Data{
			DepositRoot:  bytesutil.PadTo([]byte("e1ddepositroot"), 32),
			DepositCount: 6,
			BlockHash:    bytesutil.PadTo([]byte("e1dblockhash"), 32),
		}
		state.Zond1DataVotes = []*zondpbalpha.Zond1Data{{
			DepositRoot:  bytesutil.PadTo([]byte("e1dvdepositroot"), 32),
			DepositCount: 7,
			BlockHash:    bytesutil.PadTo([]byte("e1dvblockhash"), 32),
		}}
		state.Zond1DepositIndex = 8
		state.Validators = []*zondpbalpha.Validator{{
			PublicKey:                  bytesutil.PadTo([]byte("publickey"), 48),
			WithdrawalCredentials:      bytesutil.PadTo([]byte("withdrawalcredentials"), 32),
			EffectiveBalance:           9,
			Slashed:                    true,
			ActivationEligibilityEpoch: 10,
			ActivationEpoch:            11,
			ExitEpoch:                  12,
			WithdrawableEpoch:          13,
		}}
		state.Balances = []uint64{14}
		state.RandaoMixes = [][]byte{bytesutil.PadTo([]byte("randaomixes"), 32)}
		state.Slashings = []uint64{15}
		state.JustificationBits = bitfield.Bitvector4{1}
		state.PreviousJustifiedCheckpoint = &zondpbalpha.Checkpoint{
			Epoch: 30,
			Root:  bytesutil.PadTo([]byte("pjcroot"), 32),
		}
		state.CurrentJustifiedCheckpoint = &zondpbalpha.Checkpoint{
			Epoch: 31,
			Root:  bytesutil.PadTo([]byte("cjcroot"), 32),
		}
		state.FinalizedCheckpoint = &zondpbalpha.Checkpoint{
			Epoch: 32,
			Root:  bytesutil.PadTo([]byte("fcroot"), 32),
		}
		state.PreviousEpochParticipation = []byte("previousepochparticipation")
		state.CurrentEpochParticipation = []byte("currentepochparticipation")
		state.InactivityScores = []uint64{1, 2, 3}
		state.CurrentSyncCommittee = &zondpbalpha.SyncCommittee{
			Pubkeys: [][]byte{bytesutil.PadTo([]byte("cscpubkeys"), 48)},
		}
		state.NextSyncCommittee = &zondpbalpha.SyncCommittee{
			Pubkeys: [][]byte{bytesutil.PadTo([]byte("nscpubkeys"), 48)},
		}
		state.LatestExecutionPayloadHeader = &enginev1.ExecutionPayloadHeader{
			ParentHash:       bytesutil.PadTo([]byte("parenthash"), 32),
			FeeRecipient:     bytesutil.PadTo([]byte("feerecipient"), 20),
			StateRoot:        bytesutil.PadTo([]byte("stateroot"), 32),
			ReceiptsRoot:     bytesutil.PadTo([]byte("receiptroot"), 32),
			LogsBloom:        bytesutil.PadTo([]byte("logsbloom"), 256),
			PrevRandao:       bytesutil.PadTo([]byte("prevrandao"), 32),
			BlockNumber:      123,
			GasLimit:         456,
			GasUsed:          789,
			Timestamp:        012,
			ExtraData:        []byte("extradata"),
			BaseFeePerGas:    bytesutil.PadTo([]byte("basefeepergas"), 32),
			BlockHash:        bytesutil.PadTo([]byte("blockhash"), 32),
			TransactionsRoot: bytesutil.PadTo([]byte("transactionsroot"), 32),
			WithdrawalsRoot:  bytesutil.PadTo([]byte("withdrawalsroot"), 32),
		}
		state.NextWithdrawalIndex = 123
		state.NextWithdrawalValidatorIndex = 123
		state.HistoricalSummaries = []*zondpbalpha.HistoricalSummary{
			{
				BlockSummaryRoot: bytesutil.PadTo([]byte("blocksummaryroot"), 32),
				StateSummaryRoot: bytesutil.PadTo([]byte("statesummaryroot"), 32),
			},
			{
				BlockSummaryRoot: bytesutil.PadTo([]byte("blocksummaryroot2"), 32),
				StateSummaryRoot: bytesutil.PadTo([]byte("statesummaryroot2"), 32),
			}}
		return nil
	})
	require.NoError(t, err)

	result, err := BeaconStateToProto(source)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint64(1), result.GenesisTime)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("genesisvalidatorsroot"), 32), result.GenesisValidatorsRoot)
	assert.Equal(t, primitives.Slot(2), result.Slot)
	resultFork := result.Fork
	require.NotNil(t, resultFork)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("123"), 4), resultFork.PreviousVersion)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("456"), 4), resultFork.CurrentVersion)
	assert.Equal(t, primitives.Epoch(3), resultFork.Epoch)
	resultLatestBlockHeader := result.LatestBlockHeader
	require.NotNil(t, resultLatestBlockHeader)
	assert.Equal(t, primitives.Slot(4), resultLatestBlockHeader.Slot)
	assert.Equal(t, primitives.ValidatorIndex(5), resultLatestBlockHeader.ProposerIndex)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("lbhparentroot"), 32), resultLatestBlockHeader.ParentRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("lbhstateroot"), 32), resultLatestBlockHeader.StateRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("lbhbodyroot"), 32), resultLatestBlockHeader.BodyRoot)
	assert.Equal(t, 8192, len(result.BlockRoots))
	assert.DeepEqual(t, bytesutil.PadTo([]byte("blockroots"), 32), result.BlockRoots[0])
	assert.Equal(t, 8192, len(result.StateRoots))
	assert.DeepEqual(t, bytesutil.PadTo([]byte("stateroots"), 32), result.StateRoots[0])
	assert.Equal(t, 1, len(result.HistoricalRoots))
	assert.DeepEqual(t, bytesutil.PadTo([]byte("historicalroots"), 32), result.HistoricalRoots[0])
	resultZond1Data := result.Zond1Data
	require.NotNil(t, resultZond1Data)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("e1ddepositroot"), 32), resultZond1Data.DepositRoot)
	assert.Equal(t, uint64(6), resultZond1Data.DepositCount)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("e1dblockhash"), 32), resultZond1Data.BlockHash)
	require.Equal(t, 1, len(result.Zond1DataVotes))
	resultZond1DataVote := result.Zond1DataVotes[0]
	require.NotNil(t, resultZond1DataVote)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("e1dvdepositroot"), 32), resultZond1DataVote.DepositRoot)
	assert.Equal(t, uint64(7), resultZond1DataVote.DepositCount)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("e1dvblockhash"), 32), resultZond1DataVote.BlockHash)
	assert.Equal(t, uint64(8), result.Zond1DepositIndex)
	require.Equal(t, 1, len(result.Validators))
	resultValidator := result.Validators[0]
	require.NotNil(t, resultValidator)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("publickey"), 48), resultValidator.Pubkey)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("withdrawalcredentials"), 32), resultValidator.WithdrawalCredentials)
	assert.Equal(t, uint64(9), resultValidator.EffectiveBalance)
	assert.Equal(t, true, resultValidator.Slashed)
	assert.Equal(t, primitives.Epoch(10), resultValidator.ActivationEligibilityEpoch)
	assert.Equal(t, primitives.Epoch(11), resultValidator.ActivationEpoch)
	assert.Equal(t, primitives.Epoch(12), resultValidator.ExitEpoch)
	assert.Equal(t, primitives.Epoch(13), resultValidator.WithdrawableEpoch)
	assert.DeepEqual(t, []uint64{14}, result.Balances)
	assert.Equal(t, 65536, len(result.RandaoMixes))
	assert.DeepEqual(t, bytesutil.PadTo([]byte("randaomixes"), 32), result.RandaoMixes[0])
	assert.DeepEqual(t, []uint64{15}, result.Slashings)
	assert.DeepEqual(t, bitfield.Bitvector4{1}, result.JustificationBits)
	resultPrevJustifiedCheckpoint := result.PreviousJustifiedCheckpoint
	require.NotNil(t, resultPrevJustifiedCheckpoint)
	assert.Equal(t, primitives.Epoch(30), resultPrevJustifiedCheckpoint.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("pjcroot"), 32), resultPrevJustifiedCheckpoint.Root)
	resultCurrJustifiedCheckpoint := result.CurrentJustifiedCheckpoint
	require.NotNil(t, resultCurrJustifiedCheckpoint)
	assert.Equal(t, primitives.Epoch(31), resultCurrJustifiedCheckpoint.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("cjcroot"), 32), resultCurrJustifiedCheckpoint.Root)
	resultFinalizedCheckpoint := result.FinalizedCheckpoint
	require.NotNil(t, resultFinalizedCheckpoint)
	assert.Equal(t, primitives.Epoch(32), resultFinalizedCheckpoint.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("fcroot"), 32), resultFinalizedCheckpoint.Root)
	assert.DeepEqual(t, []byte("previousepochparticipation"), result.PreviousEpochParticipation)
	assert.DeepEqual(t, []byte("currentepochparticipation"), result.CurrentEpochParticipation)
	assert.DeepEqual(t, []uint64{1, 2, 3}, result.InactivityScores)
	require.NotNil(t, result.CurrentSyncCommittee)
	assert.DeepEqual(t, [][]byte{bytesutil.PadTo([]byte("cscpubkeys"), 48)}, result.CurrentSyncCommittee.Pubkeys)
	require.NotNil(t, result.NextSyncCommittee)
	assert.DeepEqual(t, [][]byte{bytesutil.PadTo([]byte("nscpubkeys"), 48)}, result.NextSyncCommittee.Pubkeys)
	resultLatestExecutionPayloadHeader := result.LatestExecutionPayloadHeader
	require.NotNil(t, resultLatestExecutionPayloadHeader)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("parenthash"), 32), resultLatestExecutionPayloadHeader.ParentHash)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("feerecipient"), 20), resultLatestExecutionPayloadHeader.FeeRecipient)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("stateroot"), 32), resultLatestExecutionPayloadHeader.StateRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("receiptroot"), 32), resultLatestExecutionPayloadHeader.ReceiptsRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("logsbloom"), 256), resultLatestExecutionPayloadHeader.LogsBloom)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("prevrandao"), 32), resultLatestExecutionPayloadHeader.PrevRandao)
	assert.Equal(t, uint64(123), resultLatestExecutionPayloadHeader.BlockNumber)
	assert.Equal(t, uint64(456), resultLatestExecutionPayloadHeader.GasLimit)
	assert.Equal(t, uint64(789), resultLatestExecutionPayloadHeader.GasUsed)
	assert.Equal(t, uint64(012), resultLatestExecutionPayloadHeader.Timestamp)
	assert.DeepEqual(t, []byte("extradata"), resultLatestExecutionPayloadHeader.ExtraData)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("basefeepergas"), 32), resultLatestExecutionPayloadHeader.BaseFeePerGas)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("blockhash"), 32), resultLatestExecutionPayloadHeader.BlockHash)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("transactionsroot"), 32), resultLatestExecutionPayloadHeader.TransactionsRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("withdrawalsroot"), 32), resultLatestExecutionPayloadHeader.WithdrawalsRoot)
	assert.Equal(t, uint64(123), result.NextWithdrawalIndex)
	assert.Equal(t, primitives.ValidatorIndex(123), result.NextWithdrawalValidatorIndex)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("blocksummaryroot"), 32), result.HistoricalSummaries[0].BlockSummaryRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("statesummaryroot"), 32), result.HistoricalSummaries[0].StateSummaryRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("blocksummaryroot2"), 32), result.HistoricalSummaries[1].BlockSummaryRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("statesummaryroot2"), 32), result.HistoricalSummaries[1].StateSummaryRoot)
}

func TestV1Alpha1ToV1SignedDilithiumToExecChange(t *testing.T) {
	alphaChange := &zondpbalpha.SignedDilithiumToExecutionChange{
		Message: &zondpbalpha.DilithiumToExecutionChange{
			ValidatorIndex:      validatorIndex,
			FromDilithiumPubkey: bytesutil.PadTo([]byte("fromdilithiumpubkey"), 48),
			ToExecutionAddress:  bytesutil.PadTo([]byte("toexecutionaddress"), 20),
		},
		Signature: signature,
	}
	change := V1Alpha1ToV1SignedDilithiumToExecChange(alphaChange)
	require.NotNil(t, change)
	require.NotNil(t, change.Message)
	assert.DeepEqual(t, signature, change.Signature)
	assert.Equal(t, validatorIndex, change.Message.ValidatorIndex)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("fromdilithiumpubkey"), 48), change.Message.FromDilithiumPubkey)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("toexecutionaddress"), 20), change.Message.ToExecutionAddress)
}

func TestV1ToV1Alpha1SignedDilithiumToExecutionChange(t *testing.T) {
	v1Change := &zondpbv1.SignedDilithiumToExecutionChange{
		Message: &zondpbv1.DilithiumToExecutionChange{
			ValidatorIndex:      validatorIndex,
			FromDilithiumPubkey: bytesutil.PadTo([]byte("fromdilithiumpubkey"), 48),
			ToExecutionAddress:  bytesutil.PadTo([]byte("toexecutionaddress"), 20),
		},
		Signature: signature,
	}
	change := V1ToV1Alpha1SignedDilithiumToExecutionChange(v1Change)
	require.NotNil(t, change)
	require.NotNil(t, change.Message)
	assert.DeepEqual(t, signature, change.Signature)
	assert.Equal(t, validatorIndex, change.Message.ValidatorIndex)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("fromdilithiumpubkey"), 48), change.Message.FromDilithiumPubkey)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("toexecutionaddress"), 20), change.Message.ToExecutionAddress)
}
