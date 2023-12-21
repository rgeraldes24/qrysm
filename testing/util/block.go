package util

import (
	"context"

	"github.com/pkg/errors"
	dilithiumlib "github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/db/iface"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/crypto/rand"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	v1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
	"github.com/theQRL/qrysm/v4/testing/assertions"
	"github.com/theQRL/qrysm/v4/testing/require"
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
	NumDilithiumChanges  uint64
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
		NumDilithiumChanges:  0,
	}
}

// NewBeaconBlock creates a beacon block with minimum marshalable fields.
/*
func NewBeaconBlock() *zondpb.SignedBeaconBlock {
	return &zondpb.SignedBeaconBlock{
		Block: &zondpb.BeaconBlock{
			ParentRoot: make([]byte, fieldparams.RootLength),
			StateRoot:  make([]byte, fieldparams.RootLength),
			Body: &zondpb.BeaconBlockBody{
				RandaoReveal: make([]byte, dilithiumlib.CryptoBytes),
				Zond1Data: &zondpb.Zond1Data{
					DepositRoot: make([]byte, fieldparams.RootLength),
					BlockHash:   make([]byte, fieldparams.RootLength),
				},
				Graffiti:          make([]byte, fieldparams.RootLength),
				Attestations:      []*zondpb.Attestation{},
				AttesterSlashings: []*zondpb.AttesterSlashing{},
				Deposits:          []*zondpb.Deposit{},
				ProposerSlashings: []*zondpb.ProposerSlashing{},
				VoluntaryExits:    []*zondpb.SignedVoluntaryExit{},
			},
		},
		Signature: make([]byte, dilithiumlib.CryptoBytes),
	}
}
*/

// GenerateProposerSlashingForValidator for a specific validator index.
func GenerateProposerSlashingForValidator(
	bState state.BeaconState,
	priv dilithium.DilithiumKey,
	idx primitives.ValidatorIndex,
) (*zondpb.ProposerSlashing, error) {
	header1 := HydrateSignedBeaconHeader(&zondpb.SignedBeaconBlockHeader{
		Header: &zondpb.BeaconBlockHeader{
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

	header2 := &zondpb.SignedBeaconBlockHeader{
		Header: &zondpb.BeaconBlockHeader{
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

	return &zondpb.ProposerSlashing{
		Header_1: header1,
		Header_2: header2,
	}, nil
}

func generateProposerSlashings(
	bState state.BeaconState,
	privs []dilithium.DilithiumKey,
	numSlashings uint64,
) ([]*zondpb.ProposerSlashing, error) {
	proposerSlashings := make([]*zondpb.ProposerSlashing, numSlashings)
	for i := uint64(0); i < numSlashings; i++ {
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
	priv dilithium.DilithiumKey,
	idx primitives.ValidatorIndex,
) (*zondpb.AttesterSlashing, error) {
	currentEpoch := time.CurrentEpoch(bState)

	att1 := &zondpb.IndexedAttestation{
		Data: &zondpb.AttestationData{
			Slot:            bState.Slot(),
			CommitteeIndex:  0,
			BeaconBlockRoot: make([]byte, fieldparams.RootLength),
			Target: &zondpb.Checkpoint{
				Epoch: currentEpoch,
				Root:  params.BeaconConfig().ZeroHash[:],
			},
			Source: &zondpb.Checkpoint{
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

	att2 := &zondpb.IndexedAttestation{
		Data: &zondpb.AttestationData{
			Slot:            bState.Slot(),
			CommitteeIndex:  0,
			BeaconBlockRoot: make([]byte, fieldparams.RootLength),
			Target: &zondpb.Checkpoint{
				Epoch: currentEpoch,
				Root:  params.BeaconConfig().ZeroHash[:],
			},
			Source: &zondpb.Checkpoint{
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

	return &zondpb.AttesterSlashing{
		Attestation_1: att1,
		Attestation_2: att2,
	}, nil
}

func generateAttesterSlashings(
	bState state.BeaconState,
	privs []dilithium.DilithiumKey,
	numSlashings uint64,
) ([]*zondpb.AttesterSlashing, error) {
	attesterSlashings := make([]*zondpb.AttesterSlashing, numSlashings)
	randGen := rand.NewDeterministicGenerator()
	for i := uint64(0); i < numSlashings; i++ {
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

func generateDepositsAndZond1Data(
	bState state.BeaconState,
	numDeposits uint64,
) (
	[]*zondpb.Deposit,
	*zondpb.Zond1Data,
	error,
) {
	previousDepsLen := bState.Zond1DepositIndex()
	currentDeposits, _, err := DeterministicDepositsAndKeys(previousDepsLen + numDeposits)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get deposits")
	}
	zond1Data, err := DeterministicZond1Data(len(currentDeposits))
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get zond1data")
	}
	return currentDeposits[previousDepsLen:], zond1Data, nil
}

func GenerateVoluntaryExits(bState state.BeaconState, k dilithium.DilithiumKey, idx primitives.ValidatorIndex) (*zondpb.SignedVoluntaryExit, error) {
	currentEpoch := time.CurrentEpoch(bState)
	exit := &zondpb.SignedVoluntaryExit{
		Exit: &zondpb.VoluntaryExit{
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
	privs []dilithium.DilithiumKey,
	numExits uint64,
) ([]*zondpb.SignedVoluntaryExit, error) {
	currentEpoch := time.CurrentEpoch(bState)

	voluntaryExits := make([]*zondpb.SignedVoluntaryExit, numExits)
	for i := 0; i < len(voluntaryExits); i++ {
		valIndex, err := randValIndex(bState)
		if err != nil {
			return nil, err
		}
		exit := &zondpb.SignedVoluntaryExit{
			Exit: &zondpb.VoluntaryExit{
				Epoch:          time.PrevEpoch(bState),
				ValidatorIndex: valIndex,
			},
		}
		exit.Signature, err = signing.ComputeDomainAndSign(bState, currentEpoch, exit.Exit, params.BeaconConfig().DomainVoluntaryExit, privs[valIndex])
		if err != nil {
			return nil, err
		}
		voluntaryExits[i] = exit
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
func HydrateSignedBeaconHeader(h *zondpb.SignedBeaconBlockHeader) *zondpb.SignedBeaconBlockHeader {
	if h.Signature == nil {
		h.Signature = make([]byte, dilithiumlib.CryptoBytes)
	}
	h.Header = HydrateBeaconHeader(h.Header)
	return h
}

// HydrateBeaconHeader hydrates a beacon block header with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBeaconHeader(h *zondpb.BeaconBlockHeader) *zondpb.BeaconBlockHeader {
	if h == nil {
		h = &zondpb.BeaconBlockHeader{}
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

// HydrateSignedBeaconBlock hydrates a signed beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateSignedBeaconBlock(b *zondpb.SignedBeaconBlock) *zondpb.SignedBeaconBlock {
	if b.Signature == nil {
		b.Signature = make([]byte, dilithiumlib.CryptoBytes)
	}
	b.Block = HydrateBeaconBlock(b.Block)
	return b
}

// HydrateBeaconBlock hydrates a beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBeaconBlock(b *zondpb.BeaconBlock) *zondpb.BeaconBlock {
	if b == nil {
		b = &zondpb.BeaconBlock{}
	}
	if b.ParentRoot == nil {
		b.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	if b.StateRoot == nil {
		b.StateRoot = make([]byte, fieldparams.RootLength)
	}
	b.Body = HydrateBeaconBlockBody(b.Body)
	return b
}

// HydrateBeaconBlockBody hydrates a beacon block body with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBeaconBlockBody(b *zondpb.BeaconBlockBody) *zondpb.BeaconBlockBody {
	if b == nil {
		b = &zondpb.BeaconBlockBody{}
	}
	if b.RandaoReveal == nil {
		b.RandaoReveal = make([]byte, dilithiumlib.CryptoBytes)
	}
	if b.Graffiti == nil {
		b.Graffiti = make([]byte, fieldparams.RootLength)
	}
	if b.Zond1Data == nil {
		b.Zond1Data = &zondpb.Zond1Data{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, fieldparams.RootLength),
		}
	}

	if b.SyncAggregate == nil {
		b.SyncAggregate = &zondpb.SyncAggregate{
			SyncCommitteeBits:       make([]byte, fieldparams.SyncAggregateSyncCommitteeBytesLength),
			SyncCommitteeSignatures: make([][]byte, 0),
		}
	}
	if b.ExecutionPayload == nil {
		b.ExecutionPayload = &enginev1.ExecutionPayload{
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
		b.ProposerSlashings = make([]*zondpb.ProposerSlashing, 0)
	}

	if b.AttesterSlashings == nil {
		b.AttesterSlashings = make([]*zondpb.AttesterSlashing, 0)
	}

	if b.VoluntaryExits == nil {
		b.VoluntaryExits = make([]*zondpb.SignedVoluntaryExit, 0)
	}

	if b.Deposits == nil {
		b.Deposits = make([]*zondpb.Deposit, 0)
	}

	if b.Attestations == nil {
		b.Attestations = make([]*zondpb.Attestation, 0)
	}

	if b.DilithiumToExecutionChanges == nil {
		b.DilithiumToExecutionChanges = make([]*zondpb.SignedDilithiumToExecutionChange, 0)
	}

	return b
}

// HydrateV1SignedBeaconBlock hydrates a signed beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1SignedBeaconBlock(b *v1.SignedBeaconBlock) *v1.SignedBeaconBlock {
	if b.Signature == nil {
		b.Signature = make([]byte, dilithiumlib.CryptoBytes)
	}
	b.Message = HydrateV1BeaconBlock(b.Message)
	return b
}

// HydrateV1BeaconBlock hydrates a beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1BeaconBlock(b *v1.BeaconBlock) *v1.BeaconBlock {
	if b == nil {
		b = &v1.BeaconBlock{}
	}
	if b.ParentRoot == nil {
		b.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	if b.StateRoot == nil {
		b.StateRoot = make([]byte, fieldparams.RootLength)
	}
	b.Body = HydrateV1BeaconBlockBody(b.Body)
	return b
}

// HydrateV1BeaconBlockBody hydrates a beacon block body with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1BeaconBlockBody(b *v1.BeaconBlockBody) *v1.BeaconBlockBody {
	if b == nil {
		b = &v1.BeaconBlockBody{}
	}
	if b.RandaoReveal == nil {
		b.RandaoReveal = make([]byte, dilithiumlib.CryptoBytes)
	}
	if b.Graffiti == nil {
		b.Graffiti = make([]byte, fieldparams.RootLength)
	}
	if b.Zond1Data == nil {
		b.Zond1Data = &v1.Zond1Data{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, fieldparams.RootLength),
		}
	}
	if b.SyncAggregate == nil {
		b.SyncAggregate = &v1.SyncAggregate{
			SyncCommitteeBits:       make([]byte, 64),
			SyncCommitteeSignatures: make([][]byte, 0),
		}
	}
	if b.ExecutionPayload == nil {
		b.ExecutionPayload = &enginev1.ExecutionPayload{
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

// HydrateSignedBlindedBeaconBlock hydrates a signed blinded beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateSignedBlindedBeaconBlock(b *zondpb.SignedBlindedBeaconBlock) *zondpb.SignedBlindedBeaconBlock {
	if b.Signature == nil {
		b.Signature = make([]byte, dilithiumlib.CryptoBytes)
	}
	b.Block = HydrateBlindedBeaconBlock(b.Block)
	return b
}

// HydrateBlindedBeaconBlock hydrates a blinded beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBlindedBeaconBlock(b *zondpb.BlindedBeaconBlock) *zondpb.BlindedBeaconBlock {
	if b == nil {
		b = &zondpb.BlindedBeaconBlock{}
	}
	if b.ParentRoot == nil {
		b.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	if b.StateRoot == nil {
		b.StateRoot = make([]byte, fieldparams.RootLength)
	}
	b.Body = HydrateBlindedBeaconBlockBody(b.Body)
	return b
}

// HydrateBlindedBeaconBlockBody hydrates a blinded beacon block body with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateBlindedBeaconBlockBody(b *zondpb.BlindedBeaconBlockBody) *zondpb.BlindedBeaconBlockBody {
	if b == nil {
		b = &zondpb.BlindedBeaconBlockBody{}
	}
	if b.RandaoReveal == nil {
		b.RandaoReveal = make([]byte, dilithiumlib.CryptoBytes)
	}
	if b.Graffiti == nil {
		b.Graffiti = make([]byte, 32)
	}
	if b.Zond1Data == nil {
		b.Zond1Data = &zondpb.Zond1Data{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, 32),
		}
	}
	if b.SyncAggregate == nil {
		b.SyncAggregate = &zondpb.SyncAggregate{
			SyncCommitteeBits:       make([]byte, fieldparams.SyncAggregateSyncCommitteeBytesLength),
			SyncCommitteeSignatures: make([][]byte, 0),
		}
	}
	if b.ExecutionPayloadHeader == nil {
		b.ExecutionPayloadHeader = &enginev1.ExecutionPayloadHeader{
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
	return b
}

// HydrateV1SignedBlindedBeaconBlock hydrates a signed blinded beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1SignedBlindedBeaconBlock(b *v1.SignedBlindedBeaconBlock) *v1.SignedBlindedBeaconBlock {
	if b.Signature == nil {
		b.Signature = make([]byte, dilithiumlib.CryptoBytes)
	}
	b.Message = HydrateV1BlindedBeaconBlock(b.Message)
	return b
}

// HydrateV1BlindedBeaconBlock hydrates a blinded beacon block with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1BlindedBeaconBlock(b *v1.BlindedBeaconBlock) *v1.BlindedBeaconBlock {
	if b == nil {
		b = &v1.BlindedBeaconBlock{}
	}
	if b.ParentRoot == nil {
		b.ParentRoot = make([]byte, fieldparams.RootLength)
	}
	if b.StateRoot == nil {
		b.StateRoot = make([]byte, fieldparams.RootLength)
	}
	b.Body = HydrateV1BlindedBeaconBlockBody(b.Body)
	return b
}

// HydrateV1BlindedBeaconBlockBody hydrates a blinded beacon block body with correct field length sizes
// to comply with fssz marshalling and unmarshalling rules.
func HydrateV1BlindedBeaconBlockBody(b *v1.BlindedBeaconBlockBody) *v1.BlindedBeaconBlockBody {
	if b == nil {
		b = &v1.BlindedBeaconBlockBody{}
	}
	if b.RandaoReveal == nil {
		b.RandaoReveal = make([]byte, dilithiumlib.CryptoBytes)
	}
	if b.Graffiti == nil {
		b.Graffiti = make([]byte, 32)
	}
	if b.Zond1Data == nil {
		b.Zond1Data = &v1.Zond1Data{
			DepositRoot: make([]byte, fieldparams.RootLength),
			BlockHash:   make([]byte, 32),
		}
	}
	if b.SyncAggregate == nil {
		b.SyncAggregate = &v1.SyncAggregate{
			SyncCommitteeBits:       make([]byte, 64),
			SyncCommitteeSignatures: make([][]byte, 0),
		}
	}
	if b.ExecutionPayloadHeader == nil {
		b.ExecutionPayloadHeader = &enginev1.ExecutionPayloadHeader{
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

func SaveBlock(tb assertions.AssertionTestingTB, ctx context.Context, db iface.NoHeadAccessDatabase, b interface{}) interfaces.SignedBeaconBlock {
	wsb, err := blocks.NewSignedBeaconBlock(b)
	require.NoError(tb, err)
	require.NoError(tb, db.SaveBlock(ctx, wsb))
	return wsb
}
