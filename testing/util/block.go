package util

import (
	"context"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/beacon-chain/core/time"
	"github.com/theQRL/qrysm/beacon-chain/db/iface"
	"github.com/theQRL/qrysm/beacon-chain/state"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/blocks"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/crypto/rand"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	qrlpb "github.com/theQRL/qrysm/proto/qrl/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assertions"
	"github.com/theQRL/qrysm/testing/require"
)

// BlockGenConfig is used to define the requested conditions
// for block generation.
type BlockGenConfig struct {
	NumProposerSlashings uint64
	NumAttesterSlashings uint64
	NumAttestations      uint64
	NumDeposits          uint64
	NumVoluntaryExits    uint64
	NumTransactions      uint64
	FullSyncAggregate    bool
}

// DefaultBlockGenConfig returns the block config that utilizes the
// current params in the beacon config.
func DefaultBlockGenConfig() *BlockGenConfig {
	return &BlockGenConfig{
		NumProposerSlashings: 0,
		NumAttesterSlashings: 0,
		NumAttestations:      1,
		NumDeposits:          0,
		NumVoluntaryExits:    0,
		NumTransactions:      0,
	}
}

// GenerateProposerSlashingForValidator for a specific validator index.
func GenerateProposerSlashingForValidator(
	bState state.BeaconState,
	priv ml_dsa_87.MLDSA87Key,
	idx primitives.ValidatorIndex,
) (*qrysmpb.ProposerSlashing, error) {
	header1 := HydrateSignedBeaconHeader(&qrysmpb.SignedBeaconBlockHeader{
		Header: &qrysmpb.BeaconBlockHeader{
			ProposerIndex: idx,
			Slot:          bState.Slot(),
			BodyRoot:      bytesutil.PadTo([]byte{0, 1, 0}, fieldparams.RootLength),
		},
	})
	currentEpoch := time.CurrentEpoch(bState)
	var err error
	header1.Signature, err = signing.ComputeDomainAndSign(bState, currentEpoch, header1.Header, params.BeaconConfig().DomainBeaconProposer, priv)
	if err != nil {
		return nil, err
	}

	header2 := &qrysmpb.SignedBeaconBlockHeader{
		Header: &qrysmpb.BeaconBlockHeader{
			ProposerIndex: idx,
			Slot:          bState.Slot(),
			BodyRoot:      bytesutil.PadTo([]byte{0, 2, 0}, fieldparams.RootLength),
			StateRoot:     make([]byte, fieldparams.RootLength),
			ParentRoot:    make([]byte, fieldparams.RootLength),
		},
	}
	header2.Signature, err = signing.ComputeDomainAndSign(bState, currentEpoch, header2.Header, params.BeaconConfig().DomainBeaconProposer, priv)
	if err != nil {
		return nil, err
	}

	return &qrysmpb.ProposerSlashing{
		Header_1: header1,
		Header_2: header2,
	}, nil
}

func generateProposerSlashings(
	bState state.BeaconState,
	privs []ml_dsa_87.MLDSA87Key,
	numSlashings uint64,
) ([]*qrysmpb.ProposerSlashing, error) {
	proposerSlashings := make([]*qrysmpb.ProposerSlashing, numSlashings)
	for i := range numSlashings {
		proposerIndex, err := randValIndex(bState)
		if err != nil {
			return nil, err
		}
		slashing, err := GenerateProposerSlashingForValidator(bState, privs[proposerIndex], proposerIndex)
		if err != nil {
			return nil, err
		}
		proposerSlashings[i] = slashing
	}
	return proposerSlashings, nil
}

// GenerateAttesterSlashingForValidator for a specific validator index.
func GenerateAttesterSlashingForValidator(
	bState state.BeaconState,
	priv ml_dsa_87.MLDSA87Key,
	idx primitives.ValidatorIndex,
) (*qrysmpb.AttesterSlashing, error) {
	currentEpoch := time.CurrentEpoch(bState)

	att1 := &qrysmpb.IndexedAttestation{
		Data: &qrysmpb.AttestationData{
			Slot:            bState.Slot(),
			CommitteeIndex:  0,
			BeaconBlockRoot: make([]byte, fieldparams.RootLength),
			Target: &qrysmpb.Checkpoint{
				Epoch: currentEpoch,
				Root:  params.BeaconConfig().ZeroHash[:],
			},
			Source: &qrysmpb.Checkpoint{
				Epoch: currentEpoch + 1,
				Root:  params.BeaconConfig().ZeroHash[:],
			},
		},
		AttestingIndices: []uint64{uint64(idx)},
	}
	sig, err := signing.ComputeDomainAndSign(bState, currentEpoch, att1.Data, params.BeaconConfig().DomainBeaconAttester, priv)
	if err != nil {
		return nil, err
	}
	att1.Signatures = [][]byte{sig}

	att2 := &qrysmpb.IndexedAttestation{
		Data: &qrysmpb.AttestationData{
			Slot:            bState.Slot(),
			CommitteeIndex:  0,
			BeaconBlockRoot: make([]byte, fieldparams.RootLength),
			Target: &qrysmpb.Checkpoint{
				Epoch: currentEpoch,
				Root:  params.BeaconConfig().ZeroHash[:],
			},
			Source: &qrysmpb.Checkpoint{
				Epoch: currentEpoch,
				Root:  params.BeaconConfig().ZeroHash[:],
			},
		},
		AttestingIndices: []uint64{uint64(idx)},
	}
	sig2, err := signing.ComputeDomainAndSign(bState, currentEpoch, att2.Data, params.BeaconConfig().DomainBeaconAttester, priv)
	if err != nil {
		return nil, err
	}
	att2.Signatures = [][]byte{sig2}

	return &qrysmpb.AttesterSlashing{
		Attestation_1: att1,
		Attestation_2: att2,
	}, nil
}

func generateAttesterSlashings(
	bState state.BeaconState,
	privs []ml_dsa_87.MLDSA87Key,
	numSlashings uint64,
) ([]*qrysmpb.AttesterSlashing, error) {
	attesterSlashings := make([]*qrysmpb.AttesterSlashing, numSlashings)
	randGen := rand.NewDeterministicGenerator()
	for i := range numSlashings {
		committeeIndex := randGen.Uint64() % helpers.SlotCommitteeCount(uint64(bState.NumValidators()))
		committee, err := helpers.BeaconCommitteeFromState(context.Background(), bState, bState.Slot(), primitives.CommitteeIndex(committeeIndex))
		if err != nil {
			return nil, err
		}
		randIndex := randGen.Uint64() % uint64(len(committee))
		valIndex := committee[randIndex]
		slashing, err := GenerateAttesterSlashingForValidator(bState, privs[valIndex], valIndex)
		if err != nil {
			return nil, err
		}
		attesterSlashings[i] = slashing
	}
	return attesterSlashings, nil
}

func generateDepositsAndExecutionData(
	bState state.BeaconState,
	numDeposits uint64,
) (
	[]*qrysmpb.Deposit,
	*qrysmpb.ExecutionData,
	error,
) {
	previousDepsLen := bState.ExecutionDepositIndex()
	currentDeposits, _, err := DeterministicDepositsAndKeys(previousDepsLen + numDeposits)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get deposits")
	}
	executionData, err := DeterministicExecutionData(len(currentDeposits))
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get executiondata")
	}
	return currentDeposits[previousDepsLen:], executionData, nil
}

func GenerateVoluntaryExits(bState state.BeaconState, k ml_dsa_87.MLDSA87Key, idx primitives.ValidatorIndex) (*qrysmpb.SignedVoluntaryExit, error) {
	currentEpoch := time.CurrentEpoch(bState)
	exit := &qrysmpb.SignedVoluntaryExit{
		Exit: &qrysmpb.VoluntaryExit{
			Epoch:          time.PrevEpoch(bState),
			ValidatorIndex: idx,
		},
	}
	var err error
	exit.Signature, err = signing.ComputeDomainAndSign(bState, currentEpoch, exit.Exit, params.BeaconConfig().DomainVoluntaryExit, k)
	if err != nil {
		return nil, err
	}
	return exit, nil
}

func generateVoluntaryExits(
	bState state.BeaconState,
	privs []ml_dsa_87.MLDSA87Key,
	numExits uint64,
) ([]*qrysmpb.SignedVoluntaryExit, error) {
	currentEpoch := time.CurrentEpoch(bState)

	voluntaryExits := make([]*qrysmpb.SignedVoluntaryExit, numExits)
	valMap := map[primitives.ValidatorIndex]bool{}
	for i := 0; i < len(voluntaryExits); i++ {
		valIndex, err := randValIndex(bState)
		if err != nil {
			return nil, err
		}
		// Retry if validator exit already exists.
		if valMap[valIndex] {
			i--
			continue
		}
		exit := &qrysmpb.SignedVoluntaryExit{
			Exit: &qrysmpb.VoluntaryExit{
				Epoch:          time.PrevEpoch(bState),
				ValidatorIndex: valIndex,
			},
		}
		exit.Signature, err = signing.ComputeDomainAndSign(bState, currentEpoch, exit.Exit, params.BeaconConfig().DomainVoluntaryExit, privs[valIndex])
		if err != nil {
			return nil, err
		}
		voluntaryExits[i] = exit
		valMap[valIndex] = true
	}
	return voluntaryExits, nil
}

func randValIndex(bState state.BeaconState) (primitives.ValidatorIndex, error) {
	activeCount, err := helpers.ActiveValidatorCount(context.Background(), bState, time.CurrentEpoch(bState))
	if err != nil {
		return 0, err
	}
	return primitives.ValidatorIndex(rand.NewGenerator().Uint64() % activeCount), nil
}

// HydrateSignedBeaconHeader hydrates a signed beacon block header with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateSignedBeaconHeader(h *qrysmpb.SignedBeaconBlockHeader) *qrysmpb.SignedBeaconBlockHeader {
	if h.Signature == nil {
		h.Signature = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	h.Header = HydrateBeaconHeader(h.Header)
	return h
}

// HydrateBeaconHeader hydrates a beacon block header with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBeaconHeader(h *qrysmpb.BeaconBlockHeader) *qrysmpb.BeaconBlockHeader {
	if h == nil {
		h = &qrysmpb.BeaconBlockHeader{}
	}
	if h.BodyRoot == nil {
		h.BodyRoot = make([]byte, fieldparams.RootLength)
	}
	if h.StateRoot == nil {
		h.StateRoot = make([]byte, fieldparams.RootLength)
	}
	if h.ParentRoot == nil {
		h.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	return h
}

// HydrateSignedBeaconBlockCapella hydrates a signed beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateSignedBeaconBlockCapella(b *qrysmpb.SignedBeaconBlockCapella) *qrysmpb.SignedBeaconBlockCapella {
	if b.Signature == nil {
		b.Signature = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	b.Block = HydrateBeaconBlockCapella(b.Block)
	return b
}

// HydrateBeaconBlockCapella hydrates a beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBeaconBlockCapella(b *qrysmpb.BeaconBlockCapella) *qrysmpb.BeaconBlockCapella {
	if b == nil {
		b = &qrysmpb.BeaconBlockCapella{}
	}
	if b.ParentRoot == nil {
		b.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	if b.StateRoot == nil {
		b.StateRoot = make([]byte, fieldparams.RootLength)
	}
	b.Body = HydrateBeaconBlockBodyCapella(b.Body)
	return b
}

// HydrateBeaconBlockBodyCapella hydrates a beacon block body with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBeaconBlockBodyCapella(b *qrysmpb.BeaconBlockBodyCapella) *qrysmpb.BeaconBlockBodyCapella {
	if b == nil {
		b = &qrysmpb.BeaconBlockBodyCapella{}
	}
	if b.RandaoReveal == nil {
		b.RandaoReveal = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	if b.Graffiti == nil {
		b.Graffiti = make([]byte, fieldparams.RootLength)
	}
	if b.ExecutionData == nil {
		b.ExecutionData = &qrysmpb.ExecutionData{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, fieldparams.RootLength),
		}
	}
	if b.SyncAggregate == nil {
		b.SyncAggregate = &qrysmpb.SyncAggregate{
			SyncCommitteeBits:       make([]byte, fieldparams.SyncAggregateSyncCommitteeBytesLength),
			SyncCommitteeSignatures: make([][]byte, 0),
		}
	}
	if b.ExecutionPayload == nil {
		b.ExecutionPayload = &enginev1.ExecutionPayloadCapella{
			ParentHash:    make([]byte, fieldparams.RootLength),
			FeeRecipient:  make([]byte, 20),
			StateRoot:     make([]byte, fieldparams.RootLength),
			ReceiptsRoot:  make([]byte, fieldparams.RootLength),
			LogsBloom:     make([]byte, 256),
			PrevRandao:    make([]byte, fieldparams.RootLength),
			BaseFeePerGas: make([]byte, fieldparams.RootLength),
			BlockHash:     make([]byte, fieldparams.RootLength),
			Transactions:  make([][]byte, 0),
			ExtraData:     make([]byte, 0),
			Withdrawals:   make([]*enginev1.Withdrawal, 0),
		}
	}

	if b.ProposerSlashings == nil {
		b.ProposerSlashings = make([]*qrysmpb.ProposerSlashing, 0)
	}

	if b.AttesterSlashings == nil {
		b.AttesterSlashings = make([]*qrysmpb.AttesterSlashing, 0)
	}

	if b.VoluntaryExits == nil {
		b.VoluntaryExits = make([]*qrysmpb.SignedVoluntaryExit, 0)
	}

	if b.Deposits == nil {
		b.Deposits = make([]*qrysmpb.Deposit, 0)
	}

	if b.Attestations == nil {
		b.Attestations = make([]*qrysmpb.Attestation, 0)
	}

	return b
}

// HydrateSignedBlindedBeaconBlockCapella hydrates a signed blinded beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateSignedBlindedBeaconBlockCapella(b *qrysmpb.SignedBlindedBeaconBlockCapella) *qrysmpb.SignedBlindedBeaconBlockCapella {
	if b.Signature == nil {
		b.Signature = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	b.Block = HydrateBlindedBeaconBlockCapella(b.Block)
	return b
}

// HydrateBlindedBeaconBlockCapella hydrates a blinded beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBlindedBeaconBlockCapella(b *qrysmpb.BlindedBeaconBlockCapella) *qrysmpb.BlindedBeaconBlockCapella {
	if b == nil {
		b = &qrysmpb.BlindedBeaconBlockCapella{}
	}
	if b.ParentRoot == nil {
		b.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	if b.StateRoot == nil {
		b.StateRoot = make([]byte, fieldparams.RootLength)
	}
	b.Body = HydrateBlindedBeaconBlockBodyCapella(b.Body)
	return b
}

// HydrateBlindedBeaconBlockBodyCapella hydrates a blinded beacon block body with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBlindedBeaconBlockBodyCapella(b *qrysmpb.BlindedBeaconBlockBodyCapella) *qrysmpb.BlindedBeaconBlockBodyCapella {
	if b == nil {
		b = &qrysmpb.BlindedBeaconBlockBodyCapella{}
	}
	if b.RandaoReveal == nil {
		b.RandaoReveal = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	if b.Graffiti == nil {
		b.Graffiti = make([]byte, 32)
	}
	if b.ExecutionData == nil {
		b.ExecutionData = &qrysmpb.ExecutionData{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, 32),
		}
	}
	if b.SyncAggregate == nil {
		b.SyncAggregate = &qrysmpb.SyncAggregate{
			SyncCommitteeBits:       make([]byte, fieldparams.SyncAggregateSyncCommitteeBytesLength),
			SyncCommitteeSignatures: [][]byte{},
		}
	}
	if b.ExecutionPayloadHeader == nil {
		b.ExecutionPayloadHeader = &enginev1.ExecutionPayloadHeaderCapella{
			ParentHash:       make([]byte, 32),
			FeeRecipient:     make([]byte, 20),
			StateRoot:        make([]byte, fieldparams.RootLength),
			ReceiptsRoot:     make([]byte, fieldparams.RootLength),
			LogsBloom:        make([]byte, 256),
			PrevRandao:       make([]byte, 32),
			BaseFeePerGas:    make([]byte, 32),
			BlockHash:        make([]byte, 32),
			TransactionsRoot: make([]byte, fieldparams.RootLength),
			ExtraData:        make([]byte, 0),
			WithdrawalsRoot:  make([]byte, fieldparams.RootLength),
		}
	}

	if b.ProposerSlashings == nil {
		b.ProposerSlashings = make([]*qrysmpb.ProposerSlashing, 0)
	}

	if b.AttesterSlashings == nil {
		b.AttesterSlashings = make([]*qrysmpb.AttesterSlashing, 0)
	}

	if b.VoluntaryExits == nil {
		b.VoluntaryExits = make([]*qrysmpb.SignedVoluntaryExit, 0)
	}

	if b.Deposits == nil {
		b.Deposits = make([]*qrysmpb.Deposit, 0)
	}

	if b.Attestations == nil {
		b.Attestations = make([]*qrysmpb.Attestation, 0)
	}

	return b
}

// HydrateV1SignedBlindedBeaconBlockCapella hydrates a signed blinded beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1SignedBlindedBeaconBlockCapella(b *qrlpb.SignedBlindedBeaconBlockCapella) *qrlpb.SignedBlindedBeaconBlockCapella {
	if b.Signature == nil {
		b.Signature = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	b.Message = HydrateV1BlindedBeaconBlockCapella(b.Message)
	return b
}

// HydrateV1BlindedBeaconBlockCapella hydrates a blinded beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1BlindedBeaconBlockCapella(b *qrlpb.BlindedBeaconBlockCapella) *qrlpb.BlindedBeaconBlockCapella {
	if b == nil {
		b = &qrlpb.BlindedBeaconBlockCapella{}
	}
	if b.ParentRoot == nil {
		b.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	if b.StateRoot == nil {
		b.StateRoot = make([]byte, fieldparams.RootLength)
	}
	b.Body = HydrateV1BlindedBeaconBlockBodyCapella(b.Body)
	return b
}

// HydrateV1BlindedBeaconBlockBodyCapella hydrates a blinded beacon block body with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1BlindedBeaconBlockBodyCapella(b *qrlpb.BlindedBeaconBlockBodyCapella) *qrlpb.BlindedBeaconBlockBodyCapella {
	if b == nil {
		b = &qrlpb.BlindedBeaconBlockBodyCapella{}
	}
	if b.RandaoReveal == nil {
		b.RandaoReveal = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	if b.Graffiti == nil {
		b.Graffiti = make([]byte, 32)
	}
	if b.ExecutionData == nil {
		b.ExecutionData = &qrlpb.ExecutionData{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, 32),
		}
	}
	if b.SyncAggregate == nil {
		b.SyncAggregate = &qrlpb.SyncAggregate{
			SyncCommitteeBits:       make([]byte, 64),
			SyncCommitteeSignatures: [][]byte{},
		}
	}
	if b.ExecutionPayloadHeader == nil {
		b.ExecutionPayloadHeader = &enginev1.ExecutionPayloadHeaderCapella{
			ParentHash:       make([]byte, 32),
			FeeRecipient:     make([]byte, 20),
			StateRoot:        make([]byte, fieldparams.RootLength),
			ReceiptsRoot:     make([]byte, fieldparams.RootLength),
			LogsBloom:        make([]byte, 256),
			PrevRandao:       make([]byte, 32),
			BaseFeePerGas:    make([]byte, 32),
			BlockHash:        make([]byte, 32),
			TransactionsRoot: make([]byte, fieldparams.RootLength),
			WithdrawalsRoot:  make([]byte, fieldparams.RootLength),
		}
	}
	return b
}

// HydrateV1CapellaSignedBeaconBlock hydrates a signed beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1CapellaSignedBeaconBlock(b *qrlpb.SignedBeaconBlockCapella) *qrlpb.SignedBeaconBlockCapella {
	if b.Signature == nil {
		b.Signature = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	b.Message = HydrateV1CapellaBeaconBlock(b.Message)
	return b
}

// HydrateV1CapellaBeaconBlock hydrates a beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1CapellaBeaconBlock(b *qrlpb.BeaconBlockCapella) *qrlpb.BeaconBlockCapella {
	if b == nil {
		b = &qrlpb.BeaconBlockCapella{}
	}
	if b.ParentRoot == nil {
		b.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	if b.StateRoot == nil {
		b.StateRoot = make([]byte, fieldparams.RootLength)
	}
	b.Body = HydrateV1CapellaBeaconBlockBody(b.Body)
	return b
}

// HydrateV1CapellaBeaconBlockBody hydrates a beacon block body with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1CapellaBeaconBlockBody(b *qrlpb.BeaconBlockBodyCapella) *qrlpb.BeaconBlockBodyCapella {
	if b == nil {
		b = &qrlpb.BeaconBlockBodyCapella{}
	}
	if b.RandaoReveal == nil {
		b.RandaoReveal = make([]byte, fieldparams.MLDSA87SignatureLength)
	}
	if b.Graffiti == nil {
		b.Graffiti = make([]byte, fieldparams.RootLength)
	}
	if b.ExecutionData == nil {
		b.ExecutionData = &qrlpb.ExecutionData{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, fieldparams.RootLength),
		}
	}
	if b.SyncAggregate == nil {
		b.SyncAggregate = &qrlpb.SyncAggregate{
			SyncCommitteeBits:       make([]byte, 64),
			SyncCommitteeSignatures: [][]byte{},
		}
	}
	if b.ExecutionPayload == nil {
		b.ExecutionPayload = &enginev1.ExecutionPayloadCapella{
			ParentHash:    make([]byte, fieldparams.RootLength),
			FeeRecipient:  make([]byte, 20),
			StateRoot:     make([]byte, fieldparams.RootLength),
			ReceiptsRoot:  make([]byte, fieldparams.RootLength),
			LogsBloom:     make([]byte, 256),
			PrevRandao:    make([]byte, fieldparams.RootLength),
			ExtraData:     make([]byte, fieldparams.RootLength),
			BaseFeePerGas: make([]byte, fieldparams.RootLength),
			BlockHash:     make([]byte, fieldparams.RootLength),
		}
	}
	return b
}

func SaveBlock(tb assertions.AssertionTestingTB, ctx context.Context, db iface.NoHeadAccessDatabase, b any) interfaces.SignedBeaconBlock {
	wsb, err := blocks.NewSignedBeaconBlock(b)
	require.NoError(tb, err)
	require.NoError(tb, db.SaveBlock(ctx, wsb))
	return wsb
}
