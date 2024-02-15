package migration

import (
	"testing"

	"github.com/theQRL/go-bitfield"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpbalpha "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

var (
	slot             = primitives.Slot(1)
	epoch            = primitives.Epoch(1)
	validatorIndex   = primitives.ValidatorIndex(1)
	committeeIndex   = primitives.CommitteeIndex(1)
	depositCount     = uint64(2)
	attestingIndices = []uint64{1, 2}
	blockNumber      = uint64(10)
	gasLimit         = uint64(10)
	gasUsed          = uint64(10)
	timestamp        = uint64(10)
	parentRoot       = bytesutil.PadTo([]byte("parentroot"), fieldparams.RootLength)
	stateRoot        = bytesutil.PadTo([]byte("stateroot"), fieldparams.RootLength)
	signature        = bytesutil.PadTo([]byte("signature"), 4595)
	signatures       = [][]byte{signature}
	randaoReveal     = bytesutil.PadTo([]byte("randaoreveal"), 4595)
	depositRoot      = bytesutil.PadTo([]byte("depositroot"), fieldparams.RootLength)
	blockHash        = bytesutil.PadTo([]byte("blockhash"), 32)
	beaconBlockRoot  = bytesutil.PadTo([]byte("beaconblockroot"), fieldparams.RootLength)
	sourceRoot       = bytesutil.PadTo([]byte("sourceroot"), fieldparams.RootLength)
	targetRoot       = bytesutil.PadTo([]byte("targetroot"), fieldparams.RootLength)
	bodyRoot         = bytesutil.PadTo([]byte("bodyroot"), fieldparams.RootLength)
	selectionProof   = bytesutil.PadTo([]byte("selectionproof"), 4595)
	parentHash       = bytesutil.PadTo([]byte("parenthash"), 32)
	feeRecipient     = bytesutil.PadTo([]byte("feerecipient"), 20)
	receiptsRoot     = bytesutil.PadTo([]byte("receiptsroot"), 32)
	logsBloom        = bytesutil.PadTo([]byte("logsbloom"), 256)
	prevRandao       = bytesutil.PadTo([]byte("prevrandao"), 32)
	extraData        = bytesutil.PadTo([]byte("extradata"), 32)
	baseFeePerGas    = bytesutil.PadTo([]byte("basefeepergas"), 32)
	transactionsRoot = bytesutil.PadTo([]byte("transactions"), 32)
	withdrawalsRoot  = bytesutil.PadTo([]byte("withdrawals"), 32)
	aggregationBits  = bitfield.Bitlist{0x01}
)

func Test_V1Alpha1AggregateAttAndProofToV1(t *testing.T) {
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
	v1 := V1Alpha1AggregateAttAndProofToV1(alpha)
	assert.Equal(t, v1.AggregatorIndex, primitives.ValidatorIndex(1))
	assert.DeepSSZEqual(t, v1.Aggregate.Data.Slot, att.Data.Slot)
	assert.DeepEqual(t, v1.SelectionProof, proof[:])
}

func Test_V1Alpha1AttSlashingToV1(t *testing.T) {
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

	v1Slashing := V1Alpha1AttSlashingToV1(alphaSlashing)
	alphaRoot, err := alphaSlashing.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Slashing.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func Test_V1Alpha1ProposerSlashingToV1(t *testing.T) {
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

	v1Slashing := V1Alpha1ProposerSlashingToV1(alphaSlashing)
	alphaRoot, err := alphaSlashing.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Slashing.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func Test_V1Alpha1ExitToV1(t *testing.T) {
	alphaExit := &zondpbalpha.SignedVoluntaryExit{
		Exit: &zondpbalpha.VoluntaryExit{
			Epoch:          epoch,
			ValidatorIndex: validatorIndex,
		},
		Signature: signature,
	}

	v1Exit := V1Alpha1ExitToV1(alphaExit)
	alphaRoot, err := alphaExit.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Exit.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func Test_V1ExitToV1Alpha1(t *testing.T) {
	v1Exit := &zondpbv1.SignedVoluntaryExit{
		Message: &zondpbv1.VoluntaryExit{
			Epoch:          epoch,
			ValidatorIndex: validatorIndex,
		},
		Signature: signature,
	}

	alphaExit := V1ExitToV1Alpha1(v1Exit)
	alphaRoot, err := alphaExit.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Exit.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func Test_V1AttSlashingToV1Alpha1(t *testing.T) {
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

	alphaSlashing := V1AttSlashingToV1Alpha1(v1Slashing)
	alphaRoot, err := alphaSlashing.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Slashing.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, alphaRoot)
}

func Test_V1ProposerSlashingToV1Alpha1(t *testing.T) {
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

	alphaSlashing := V1ProposerSlashingToV1Alpha1(v1Slashing)
	alphaRoot, err := alphaSlashing.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Slashing.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, alphaRoot, v1Root)
}

func Test_V1Alpha1AttToV1(t *testing.T) {
	alphaAtt := &zondpbalpha.Attestation{
		AggregationBits: aggregationBits,
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

	v1Att := V1Alpha1AttestationToV1(alphaAtt)
	v1Root, err := v1Att.HashTreeRoot()
	require.NoError(t, err)
	alphaRoot, err := alphaAtt.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, alphaRoot)
}

func Test_V1AttToV1Alpha1(t *testing.T) {
	v1Att := &zondpbv1.Attestation{
		AggregationBits: aggregationBits,
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

	alphaAtt := V1AttToV1Alpha1(v1Att)
	alphaRoot, err := alphaAtt.HashTreeRoot()
	require.NoError(t, err)
	v1Root, err := v1Att.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, alphaRoot)
}

func Test_V1Alpha1ValidatorToV1(t *testing.T) {
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

	v1Validator := V1Alpha1ValidatorToV1(v1Alpha1Validator)
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

func Test_V1ValidatorToV1Alpha1(t *testing.T) {
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

	v1Alpha1Validator := V1ValidatorToV1Alpha1(v1Validator)
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

func Test_V1SignedAggregateAttAndProofToV1Alpha1(t *testing.T) {
	v1Att := &zondpbv1.SignedAggregateAttestationAndProof{
		Message: &zondpbv1.AggregateAttestationAndProof{
			AggregatorIndex: 1,
			Aggregate:       util.HydrateV1Attestation(&zondpbv1.Attestation{}),
			SelectionProof:  selectionProof,
		},
		Signature: signature,
	}
	v1Alpha1Att := V1SignedAggregateAttAndProofToV1Alpha1(v1Att)

	v1Root, err := v1Att.HashTreeRoot()
	require.NoError(t, err)
	v1Alpha1Root, err := v1Alpha1Att.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, v1Alpha1Root)
}

func Test_V1AttestationToV1Alpha1(t *testing.T) {
	v1Att := util.HydrateV1Attestation(&zondpbv1.Attestation{})
	v1Alpha1Att := V1AttToV1Alpha1(v1Att)

	v1Root, err := v1Att.HashTreeRoot()
	require.NoError(t, err)
	v1Alpha1Root, err := v1Alpha1Att.HashTreeRoot()
	require.NoError(t, err)
	assert.DeepEqual(t, v1Root, v1Alpha1Root)
}
