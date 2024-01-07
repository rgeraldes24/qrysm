package client

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/theQRL/go-qrllib/common"
	dilithiumlib "github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	lruwrpr "github.com/theQRL/qrysm/v4/cache/lru"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	blocktest "github.com/theQRL/qrysm/v4/consensus-types/blocks/testing"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	validatorpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
	validatormock "github.com/theQRL/qrysm/v4/testing/validator-mock"
	testing2 "github.com/theQRL/qrysm/v4/validator/db/testing"
	"github.com/theQRL/qrysm/v4/validator/graffiti"
)

type mocks struct {
	validatorClient *validatormock.MockValidatorClient
	nodeClient      *validatormock.MockNodeClient
	slasherClient   *validatormock.MockSlasherClient
	signfunc        func(context.Context, *validatorpb.SignRequest) (dilithium.Signature, error)
}

type mockSignature struct{}

func (mockSignature) Verify(dilithium.PublicKey, []byte) bool {
	return true
}
func (mockSignature) AggregateVerify([]dilithium.PublicKey, [][32]byte) bool {
	return true
}
func (mockSignature) FastAggregateVerify([]dilithium.PublicKey, [32]byte) bool {
	return true
}
func (mockSignature) Eth2FastAggregateVerify([]dilithium.PublicKey, [32]byte) bool {
	return true
}
func (mockSignature) Marshal() []byte {
	return make([]byte, 32)
}
func (m mockSignature) Copy() dilithium.Signature {
	return m
}

func testKeyFromBytes(t *testing.T, b []byte) keypair {
	pri, err := dilithium.SecretKeyFromBytes(bytesutil.PadTo(b, common.SeedSize))
	require.NoError(t, err, "Failed to generate key from bytes")
	return keypair{pub: bytesutil.ToBytes2592(pri.PublicKey().Marshal()), pri: pri}
}

func setup(t *testing.T) (*validator, *mocks, dilithium.DilithiumKey, func()) {
	validatorKey, err := dilithium.RandKey()
	require.NoError(t, err)
	return setupWithKey(t, validatorKey)
}

func setupWithKey(t *testing.T, validatorKey dilithium.DilithiumKey) (*validator, *mocks, dilithium.DilithiumKey, func()) {
	var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	valDB := testing2.SetupDB(t, [][dilithiumlib.CryptoPublicKeyBytes]byte{pubKey})
	ctrl := gomock.NewController(t)
	m := &mocks{
		validatorClient: validatormock.NewMockValidatorClient(ctrl),
		nodeClient:      validatormock.NewMockNodeClient(ctrl),
		slasherClient:   validatormock.NewMockSlasherClient(ctrl),
		signfunc: func(ctx context.Context, req *validatorpb.SignRequest) (dilithium.Signature, error) {
			return mockSignature{}, nil
		},
	}
	aggregatedSlotCommitteeIDCache := lruwrpr.New(int(params.BeaconConfig().MaxCommitteesPerSlot))
	validator := &validator{
		db:                             valDB,
		keyManager:                     newMockKeymanager(t, keypair{pub: pubKey, pri: validatorKey}),
		validatorClient:                m.validatorClient,
		slashingProtectionClient:       m.slasherClient,
		graffiti:                       []byte{},
		attLogs:                        make(map[[32]byte]*attSubmitted),
		aggregatedSlotCommitteeIDCache: aggregatedSlotCommitteeIDCache,
	}

	return validator, m, validatorKey, ctrl.Finish
}

func TestProposeBlock_DoesNotProposeGenesisBlock(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, _, validatorKey, finish := setup(t)
	defer finish()
	var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.ProposeBlock(context.Background(), 0, pubKey)

	require.LogsContain(t, hook, "Assigned to genesis slot, skipping proposal")
}

func TestProposeBlock_DomainDataFailed(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(nil /*response*/, errors.New("uh oh"))

	validator.ProposeBlock(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, "Failed to sign randao reveal")
}

func TestProposeBlock_DomainDataIsNil(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Return(nil /*response*/, nil)

	validator.ProposeBlock(context.Background(), 1, pubKey)
	require.LogsContain(t, hook, domainDataErr)
}

func TestProposeBlock_RequestBlockFailed(t *testing.T) {
	params.SetupTestConfigCleanup(t)

	tests := []struct {
		name string
		slot primitives.Slot
	}{
		{
			name: "capella",
			slot: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logTest.NewGlobal()
			validator, m, validatorKey, finish := setup(t)
			defer finish()
			var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
			copy(pubKey[:], validatorKey.PublicKey().Marshal())

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			m.validatorClient.EXPECT().GetBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
			).Return(nil /*response*/, errors.New("uh oh"))

			validator.ProposeBlock(context.Background(), tt.slot, pubKey)
			require.LogsContain(t, hook, "Failed to request block from beacon node")
		})
	}
}

func TestProposeBlock_ProposeBlockFailed(t *testing.T) {
	tests := []struct {
		name  string
		block *zondpb.GenericBeaconBlock
	}{
		{
			name: "capella",
			block: &zondpb.GenericBeaconBlock{
				Block: &zondpb.GenericBeaconBlock_Capella{
					Capella: util.NewBeaconBlock().Block,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logTest.NewGlobal()
			validator, m, validatorKey, finish := setup(t)
			defer finish()
			var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
			copy(pubKey[:], validatorKey.PublicKey().Marshal())

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			m.validatorClient.EXPECT().GetBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
			).Return(tt.block, nil /*err*/)

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			m.validatorClient.EXPECT().ProposeBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.GenericSignedBeaconBlock{}),
			).Return(nil /*response*/, errors.New("uh oh"))

			validator.ProposeBlock(context.Background(), 1, pubKey)
			require.LogsContain(t, hook, "Failed to propose block")
		})
	}
}

func TestProposeBlock_BlocksDoubleProposal(t *testing.T) {
	slot := params.BeaconConfig().SlotsPerEpoch.Mul(5).Add(2)
	var blockGraffiti [32]byte
	copy(blockGraffiti[:], "someothergraffiti")

	tests := []struct {
		name   string
		blocks []*zondpb.GenericBeaconBlock
	}{
		{
			name: "capella",
			blocks: func() []*zondpb.GenericBeaconBlock {
				block0, block1 := util.NewBeaconBlock(), util.NewBeaconBlock()
				block1.Block.Body.Graffiti = blockGraffiti[:]

				var blocks []*zondpb.GenericBeaconBlock
				for _, block := range []*zondpb.SignedBeaconBlock{block0, block1} {
					block.Block.Slot = slot
					blocks = append(blocks, &zondpb.GenericBeaconBlock{
						Block: &zondpb.GenericBeaconBlock_Capella{
							Capella: block.Block,
						},
					})
				}
				return blocks
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logTest.NewGlobal()
			validator, m, validatorKey, finish := setup(t)
			defer finish()
			var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
			copy(pubKey[:], validatorKey.PublicKey().Marshal())

			var dummyRoot [32]byte
			// Save a dummy proposal history at slot 0.
			err := validator.db.SaveProposalHistoryForSlot(context.Background(), pubKey, 0, dummyRoot[:])
			require.NoError(t, err)

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Times(1).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			m.validatorClient.EXPECT().GetBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
			).Return(tt.blocks[0], nil /*err*/)

			m.validatorClient.EXPECT().GetBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
			).Return(tt.blocks[1], nil /*err*/)

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Times(3).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			m.validatorClient.EXPECT().ProposeBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.GenericSignedBeaconBlock{}),
			).Return(&zondpb.ProposeResponse{BlockRoot: make([]byte, 32)}, nil /*error*/)

			validator.ProposeBlock(context.Background(), slot, pubKey)
			require.LogsDoNotContain(t, hook, failedBlockSignLocalErr)

			validator.ProposeBlock(context.Background(), slot, pubKey)
			require.LogsContain(t, hook, failedBlockSignLocalErr)
		})
	}
}

func TestProposeBlock_BlocksDoubleProposal_After54KEpochs(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())

	var dummyRoot [32]byte
	// Save a dummy proposal history at slot 0.
	err := validator.db.SaveProposalHistoryForSlot(context.Background(), pubKey, 0, dummyRoot[:])
	require.NoError(t, err)

	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(1).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	testBlock := util.NewBeaconBlock()
	farFuture := params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().WeakSubjectivityPeriod + 9))
	testBlock.Block.Slot = farFuture
	m.validatorClient.EXPECT().GetBeaconBlock(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
	).Return(&zondpb.GenericBeaconBlock{
		Block: &zondpb.GenericBeaconBlock_Capella{
			Capella: testBlock.Block,
		},
	}, nil /*err*/)

	secondTestBlock := util.NewBeaconBlock()
	secondTestBlock.Block.Slot = farFuture
	var blockGraffiti [32]byte
	copy(blockGraffiti[:], "someothergraffiti")
	secondTestBlock.Block.Body.Graffiti = blockGraffiti[:]
	m.validatorClient.EXPECT().GetBeaconBlock(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
	).Return(&zondpb.GenericBeaconBlock{
		Block: &zondpb.GenericBeaconBlock_Capella{
			Capella: secondTestBlock.Block,
		},
	}, nil /*err*/)
	m.validatorClient.EXPECT().DomainData(
		gomock.Any(), // ctx
		gomock.Any(), // epoch
	).Times(3).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

	m.validatorClient.EXPECT().ProposeBeaconBlock(
		gomock.Any(), // ctx
		gomock.AssignableToTypeOf(&zondpb.GenericSignedBeaconBlock{}),
	).Return(&zondpb.ProposeResponse{BlockRoot: make([]byte, 32)}, nil /*error*/)

	validator.ProposeBlock(context.Background(), farFuture, pubKey)
	require.LogsDoNotContain(t, hook, failedBlockSignLocalErr)

	validator.ProposeBlock(context.Background(), farFuture, pubKey)
	require.LogsContain(t, hook, failedBlockSignLocalErr)
}

func TestProposeBlock_AllowsPastProposals(t *testing.T) {
	slot := params.BeaconConfig().SlotsPerEpoch.Mul(uint64(params.BeaconConfig().WeakSubjectivityPeriod + 9))

	tests := []struct {
		name     string
		pastSlot primitives.Slot
	}{
		{
			name:     "400 slots ago",
			pastSlot: slot.Sub(400),
		},
		{
			name:     "same epoch",
			pastSlot: slot.Sub(4),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logTest.NewGlobal()
			validator, m, validatorKey, finish := setup(t)
			defer finish()
			var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
			copy(pubKey[:], validatorKey.PublicKey().Marshal())

			// Save a dummy proposal history at slot 0.
			err := validator.db.SaveProposalHistoryForSlot(context.Background(), pubKey, 0, []byte{})
			require.NoError(t, err)

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Times(2).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			blk := util.NewBeaconBlock()
			blk.Block.Slot = slot
			m.validatorClient.EXPECT().GetBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
			).Return(&zondpb.GenericBeaconBlock{
				Block: &zondpb.GenericBeaconBlock_Capella{
					Capella: blk.Block,
				},
			}, nil /*err*/)

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Times(2).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			m.validatorClient.EXPECT().ProposeBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.GenericSignedBeaconBlock{}),
			).Times(2).Return(&zondpb.ProposeResponse{BlockRoot: make([]byte, 32)}, nil /*error*/)

			validator.ProposeBlock(context.Background(), slot, pubKey)
			require.LogsDoNotContain(t, hook, failedBlockSignLocalErr)

			blk2 := util.NewBeaconBlock()
			blk2.Block.Slot = tt.pastSlot
			m.validatorClient.EXPECT().GetBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
			).Return(&zondpb.GenericBeaconBlock{
				Block: &zondpb.GenericBeaconBlock_Capella{
					Capella: blk2.Block,
				},
			}, nil /*err*/)
			validator.ProposeBlock(context.Background(), tt.pastSlot, pubKey)
			require.LogsDoNotContain(t, hook, failedBlockSignLocalErr)
		})
	}
}

func TestProposeBlock_BroadcastsBlock(t *testing.T) {
	testProposeBlock(t, make([]byte, 32) /*graffiti*/)
}

func TestProposeBlock_BroadcastsBlock_WithGraffiti(t *testing.T) {
	blockGraffiti := []byte("12345678901234567890123456789012")
	testProposeBlock(t, blockGraffiti)
}

func testProposeBlock(t *testing.T, graffiti []byte) {
	tests := []struct {
		name  string
		block *zondpb.GenericBeaconBlock
	}{
		{
			name: "capella",
			block: &zondpb.GenericBeaconBlock{
				Block: &zondpb.GenericBeaconBlock_Capella{
					Capella: func() *zondpb.BeaconBlock {
						blk := util.NewBeaconBlock()
						blk.Block.Body.Graffiti = graffiti
						return blk.Block
					}(),
				},
			},
		},
		{
			name: "capella blind block",
			block: &zondpb.GenericBeaconBlock{
				Block: &zondpb.GenericBeaconBlock_BlindedCapella{
					BlindedCapella: func() *zondpb.BlindedBeaconBlock {
						blk := util.NewBlindedBeaconBlock()
						blk.Block.Body.Graffiti = graffiti
						return blk.Block
					}(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logTest.NewGlobal()
			validator, m, validatorKey, finish := setup(t)
			defer finish()
			var pubKey [dilithiumlib.CryptoPublicKeyBytes]byte
			copy(pubKey[:], validatorKey.PublicKey().Marshal())

			validator.graffiti = graffiti

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			m.validatorClient.EXPECT().GetBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.BlockRequest{}),
			).DoAndReturn(func(ctx context.Context, req *zondpb.BlockRequest) (*zondpb.GenericBeaconBlock, error) {
				assert.DeepEqual(t, graffiti, req.Graffiti, "Unexpected graffiti in request")

				return tt.block, nil
			})

			m.validatorClient.EXPECT().DomainData(
				gomock.Any(), // ctx
				gomock.Any(), // epoch
			).Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil /*err*/)

			var sentBlock interfaces.ReadOnlySignedBeaconBlock
			var err error

			m.validatorClient.EXPECT().ProposeBeaconBlock(
				gomock.Any(), // ctx
				gomock.AssignableToTypeOf(&zondpb.GenericSignedBeaconBlock{}),
			).DoAndReturn(func(ctx context.Context, block *zondpb.GenericSignedBeaconBlock) (*zondpb.ProposeResponse, error) {
				sentBlock, err = blocktest.NewSignedBeaconBlockFromGeneric(block)
				require.NoError(t, err)
				return &zondpb.ProposeResponse{BlockRoot: make([]byte, 32)}, nil
			})

			validator.ProposeBlock(context.Background(), 1, pubKey)
			g := sentBlock.Block().Body().Graffiti()
			assert.Equal(t, string(validator.graffiti), string(g[:]))
			require.LogsContain(t, hook, "Submitted new block")
		})
	}
}

func TestProposeExit_ValidatorIndexFailed(t *testing.T) {
	_, m, validatorKey, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().ValidatorIndex(
		gomock.Any(),
		gomock.Any(),
	).Return(nil, errors.New("uh oh"))

	err := ProposeExit(
		context.Background(),
		m.validatorClient,
		m.signfunc,
		validatorKey.PublicKey().Marshal(),
		params.BeaconConfig().GenesisEpoch,
	)
	assert.NotNil(t, err)
	assert.ErrorContains(t, "uh oh", err)
	assert.ErrorContains(t, "gRPC call to get validator index failed", err)
}

func TestProposeExit_DomainDataFailed(t *testing.T) {
	_, m, validatorKey, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().
		ValidatorIndex(gomock.Any(), gomock.Any()).
		Return(&zondpb.ValidatorIndexResponse{Index: 1}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("uh oh"))

	err := ProposeExit(
		context.Background(),
		m.validatorClient,
		m.signfunc,
		validatorKey.PublicKey().Marshal(),
		params.BeaconConfig().GenesisEpoch,
	)
	assert.NotNil(t, err)
	assert.ErrorContains(t, domainDataErr, err)
	assert.ErrorContains(t, "uh oh", err)
	assert.ErrorContains(t, "failed to sign voluntary exit", err)
}

func TestProposeExit_DomainDataIsNil(t *testing.T) {
	_, m, validatorKey, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().
		ValidatorIndex(gomock.Any(), gomock.Any()).
		Return(&zondpb.ValidatorIndexResponse{Index: 1}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(nil, nil)

	err := ProposeExit(
		context.Background(),
		m.validatorClient,
		m.signfunc,
		validatorKey.PublicKey().Marshal(),
		params.BeaconConfig().GenesisEpoch,
	)
	assert.NotNil(t, err)
	assert.ErrorContains(t, domainDataErr, err)
	assert.ErrorContains(t, "failed to sign voluntary exit", err)
}

func TestProposeBlock_ProposeExitFailed(t *testing.T) {
	_, m, validatorKey, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().
		ValidatorIndex(gomock.Any(), gomock.Any()).
		Return(&zondpb.ValidatorIndexResponse{Index: 1}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil)

	m.validatorClient.EXPECT().
		ProposeExit(gomock.Any(), gomock.AssignableToTypeOf(&zondpb.SignedVoluntaryExit{})).
		Return(nil, errors.New("uh oh"))

	err := ProposeExit(
		context.Background(),
		m.validatorClient,
		m.signfunc,
		validatorKey.PublicKey().Marshal(),
		params.BeaconConfig().GenesisEpoch,
	)
	assert.NotNil(t, err)
	assert.ErrorContains(t, "uh oh", err)
	assert.ErrorContains(t, "failed to propose voluntary exit", err)
}

func TestProposeExit_BroadcastsBlock(t *testing.T) {
	_, m, validatorKey, finish := setup(t)
	defer finish()

	m.validatorClient.EXPECT().
		ValidatorIndex(gomock.Any(), gomock.Any()).
		Return(&zondpb.ValidatorIndexResponse{Index: 1}, nil)

	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(&zondpb.DomainResponse{SignatureDomain: make([]byte, 32)}, nil)

	m.validatorClient.EXPECT().
		ProposeExit(gomock.Any(), gomock.AssignableToTypeOf(&zondpb.SignedVoluntaryExit{})).
		Return(&zondpb.ProposeExitResponse{}, nil)

	assert.NoError(t, ProposeExit(
		context.Background(),
		m.validatorClient,
		m.signfunc,
		validatorKey.PublicKey().Marshal(),
		params.BeaconConfig().GenesisEpoch,
	))
}

func TestSignBlock(t *testing.T) {
	validator, m, _, finish := setup(t)
	defer finish()

	proposerDomain := make([]byte, 32)
	m.validatorClient.EXPECT().
		DomainData(gomock.Any(), gomock.Any()).
		Return(&zondpb.DomainResponse{SignatureDomain: proposerDomain}, nil)
	ctx := context.Background()
	blk := util.NewBeaconBlock()
	blk.Block.Slot = 1
	blk.Block.ProposerIndex = 100

	kp := testKeyFromBytes(t, []byte{1})

	validator.keyManager = newMockKeymanager(t, kp)
	b, err := blocks.NewBeaconBlock(blk.Block)
	require.NoError(t, err)
	sig, blockRoot, err := validator.signBlock(ctx, kp.pub, 0, 0, b)
	require.NoError(t, err, "%x,%v", sig, err)
	require.Equal(t, "8a0f1f3a9e029f3bfed8f46ef84159e5039a6bbf587ae8baa8dd7976f45bfa9060831672d0b97c40cc6afc2391f8370c9de9d61294f4e6c8af8db69d154be2fdb13d9df5bf2e5be0425a2a9fb6be9fb3721ce5a9f30e6e7e9f1c91c30f069fe62b82c58a0bc105157a5bfefb159fd2eb2794b543a25eacb5f55d757ca4cf94a1af5063468bd0d0a15fd5ad110019c8e9dc4a303d289073a5a978005906192c6470a82436e7f68f24b2cdb49a32bfd285edb74796e9eee79e142a5a2a9dd68f2c90dbb60969b06b4c7e05c009012fb4a81fa2bd4ca4af8e19f67c1e187af4cc08553e3c2be0b44601a2f96cc1d1321b6d2872ebeb01581216cb9a3faa3636181344db1d2805f83a89656ac41d7ff101249d9dfeead9d28875c7dfba567443e33fcf153f1fc8a76446745cd3c5c244deefe0151000412d5262f74e5210cf8ce4e84732a36741db3539b3116ea4c3ab023d692ff83bceed46bed43d4df68e7932ff226dd4930a8f072a7d830597d8fccabf018e8c9b2a078e318881ba1510abec7ad0289a81b39daba6d3837ba9b52e16aaccd2a10cb79679cc66407d56bb692b44ec49fdd7c4a4e63d163df147e5023c10d286a0512c86fb55169b9d75df5c5d07ec530f334da5d81337ffe59d20396a6a71b3137550b68b4cc89c61798e9c5d3a6d57ce68d1f6e29d00f00bb7f25663be4c6e2bad0d2c9c3d7feb1da10687faf4488ca13a1729ed2f34cba4944a33f5457059b9e53cf8393d1e7b4437c50f8617bc6a2b3356d52d2f15236e5a501ea4b6b7d8d1c1a473f3dcd4b8dc3d193a34da8b470e94bbde07ec41b628e590a9700ea666acff250b84533432be7a2db303798e091dacdb623f46fb60da4205cac39241100ad3fef7e3f6f7e544075451e370c761890bda8fa92bcab473ec8153135cf7e2dda782089ba4e16851a683d9a34b33253723b3bce6e4e6440fc2dda00fb5c0c9d0f45c000c5ef039acb552b5194bdfc3ee354bfa3da6d9840b6f181083919cd93511944cf8ee9c106d82dc7c5b482afa5cced010bf01c4bfe7e63fd6ed1ca45e0de2c49e5ad610cf9b1cbf07390266a9456122ab10b95cc44e9854588a57ff716386cb47994368a01e58b93e4cfabf6da3853904baac70f8ed3ebcafdb3896b030c7c0b5087f7604f16d56cd92fff8a29013eb7cef15cb26aa307314759a644d4ad3b7c08588e150c63bce1774e2b85556d4a2e73d02a89d0a62913af375b930114a480785f20093f7b051eef35e8c5932d66ef360652aab7641028c32679f2b1fc93311321e2441617acefc103d282bfb17e50984ddae1ec327258445adc1e188a7e90d5e9d8153c787bfc0d3de6057064a1aa33b93b4e3a11d1ed64d8d23a738d947a177ce9076fb79dab0c0f366f218f1b5e251eec4b745d29a00ef1ce1343e5d19db152d223213056046d2dac44b50523c9df302b942868955da16738b1608bd8ca3f2751427ff3526e69f1d94e676af18f89a9f45e059c0970314db4ec76731c5c232d181d0ae65cacf96cb7ce5d72c440d408150db466a002bdccf717fc2441623c7d2b16ae7ae209df1824afc5d18e419c22ae598f3bc6a80faa446bb0c1d47883c10970edeee3a4ba835000407c7d6937200c1f9914260a34a9c056f262bc9ca726027cc3d6e04ccd58dcc1dce466172434f66caa2efb1f36cd0c46d3cc3b1cee6da2bf255804e69c4099dbe9f751106d40dca1e91c42ba3cca1ac8d618e2c22f2023440247f4258c15776f48fa48bbb34fbe55339a51e397e3027a00c41071cf255b1b273b4da2564dd6c6ab3bed83c76f7883f76153b7ce6e00b0ff2ad7db5f8933cdf45322ddd24f5eebe03792b6ddd14a4b8c654c65a8cb0ea428589aa5f5f41a8d8f188e89e0b878450e216fd97f3ea49bd7f5d6aa65191979070bee3047d593338bb1570d35576d65f4e610b99ed9cd4f5289218960b3552dc0aa435a4ef75d7b88f6fc13daa4b03f0de91512ca33d99501b72d9164f35cf071cda47334eb73392c9308f3d5054d888bc915e2a62bc4a60ca97fe6601d1b681ffd1b7c42d844f3bc44cc1657df7a999dbb582ba53d5c8ab275473c77055e05107757bd29f6a2ba013492ada626872773a773ee31dff1e4958bc95d6fa00a50fdf6df8f95d6735d32aaabf73261c5b7620b026e406c55e4631f23eaa8f6d847fcd527f16e623c70920ac3ecf44140314d57348d1d0612c52faec0963562237521f7dbd9ceb456ffd561c81df02c6e019eeae12d268a3095e5b9c0c344ae3d9a8ac601c30a96b059c83cb46ac2b2e736032cf03b0f98249334fe5e52db5caf2f44e22cef1a5c430999f91db279b1388978953c42c294f57ced09082e03342a8fe3d5aa8605baf27143055a406f2f975033fcce65ac9ca40fca5ff3581f0d0a87ac6bfd8208569ec43c764d634e65e73760731b7e92b6fdd35291bc69d66933c145ecc72e4463ae2daa7f8c4729db4de3922e0a8e98599d081b0bfdf4765a276fd0ebbaa63ba9c290a9ce09b0b911bc333040170bdbbeae13a6a1290dc5649cb571ac9c2b1bc7e12b56bd96e32897a68b1861865b0f761fd981b476d3222b044ace603e1a0c998e958489ce0cc4b5b2f420dcf6ff76089356e20242bfea68cf576fb0e5db65e2ceb79b304500237bc5a8d6fcefdb1274d9771f2be1a471249a1fd252a5a6b8f832963c35b461a631cae57bdb2f424b0f03909b05796bd34a0d4262d6210fb00fd29dbbb94d636f2cf7d2f44181c514a9f61254797bc489c3e1e9b771e30a7c34aaff74348a02c0dcd383ac76a3dd4e7f9beb0019b378d7c95fa75915698c22a71eafc8f561d77b9e208472134b21a9be321d416c6749b598ea1120f2cb51001912d6bb8565b693f5ab5b080239ac64845d9c684ff468e91d42e6ee46fc282c4da54677e929a73f64915e5fa5edbf795212bc726e8f8f99b0179193bea7773169bd6e777a76fea117ef1669f14e6629ab8d4da45f557989f8b3ade0324bad327c64f7f7159679bc375c6d1617c745e2d2a2e6442b5af6aa417c356047f34168480925eece4d02dec8205aa04a5f63949a5a92ef3cce3a967e78766a8845f318e0d23ba7a4b025ce4d09ed5b7856dd73dba7b0c492f8093dfbfa515dfec0c2d3aaea4688a0c8f04f524f466656c6dd8a1e93e7a03e1ca72d69246a9e36918fc4d75c14ca983bd65d8e973a5b6fa01930cb57d710378ae194f1d0f50306a7cdca6e70fc59dd8598430d40343bd0eb79bb185c5118d01f30d26b3e413c08780e27029fcfcaead8a0cffcb818f2c8962601810b04124c17d43fa9d7ee0a79e2eab7588b8b95a1af90baa887af39216ebbc4cf65adf83cf8e0191040d83d1c392e8d75b70ef0280c3b0bfffa8f1b75d065ed841c7e32f0252df77756a262af0833ad0066e6ae4eb4a6351ccd4d1eb8710c828db7795939282f58a5bd08976c2766ab8a03aabc96611e270c083a010c4a80956c668f38ee677b78cf4bc87d45ecee19fc9e0e4fb8f3fd83acb80377b162d2c5696303f87e9c44d310bafbeea9fdb1bb05427835351878dd6a8084322037f4de68970c283e46cc311b270cf292b18169a0912859b83cd0eb93b91691633dabf95dd8d2f4d356c1f6bfb4980c4500ae9d931c64dfceb149fab89e77343e0098efc5ec8333974d213e944ea75f6ace7a99594d49340abee3177b02f70da37bedb51794e00da92de33a6072cf872459e8e1fedf1b0166a0f2d22e05d3965ee6dad20dc5998de9b733974b72468b78df8d6b01388d6549d6f6c31539b36062da92b357b087fc91b1abcb433f3720d1e7c8c00578adc43bba2d407d81bfa28d076217e2a20e9c012027a34f5b3a9f28aa935bf9fcb1f07368b99be7adfd524de83a1b62f7f2eff5d703ea0055e5f94afc57c21c95663e96bf013e8f9e7a33fefde5e97b9ea41d1f83dd68ef426418068fed7d2c6bd8f61ae1b4fa5529a471252b3f07f5704ce6f008cc978420230fea8eec21d55538a251eacc0bc71d8276666ada66e873fc81c647577278698efb9321f689143cdc0ff349dc35d74a330105981bd605080bd256c26f720bc2bf613ff45344bde471599f25664da03b4bd6f6517b91e6f4203f97b33b65328357ff06b439a67a48660b60d583cba9ea885dd2d6651c7e28d97aa31d992edf73ccd3596cf63ecf62d54a8059e576a89b159fce6f2f4f70c07c4b98c5b2b99fdebdb9f88d4d8940eed77392f17cd9879caf08df9597bba99af2b8b4b9f324a2726c092879cb9a3b6df407aee3498db90dbb42c982e02dc534ca2a0093fee60149ea6d5f9f01bc6a733566c18c93f7e008258f342ad2d5905b0302a97b65d7b2d35267b30801e3ae44878db58ea09a9bc34df737ad5bea374e73509af4a25434c575732d4e37615568ba775ebba6627a7ed4ad8edf220983bba55d94a78e0fc783a676d39dc7623e81a8c5386afb10ecc4758d53dd78226eadb5a284551f4f29732219c26cd144a97afe9d0e2316217a8b2b14cda71c57c0f991764561d927fc3e28fb8dd0f8e0484005d4eac520656ca2d76045df09226930e8ed87b17ea9a7c1c0d0a25c32697a7112b333cd8df23da4751cafc8f5f3bc68e07b07521e1328ca5c081d891976bab7d4de9f449feb6b0d5f76744546c298765f223f9af4db0b16bcf6c8648aa821819071fcceac693ce96609772a0584362d026c841424a7b3432ceca0bb595417a8d9dd27cf1bf9cb12a2dd3b9d56228892b5dff73fa2b26a7d21aa7677ac899a64a9d3f629c93dc1b3850dd464b12cce8f81c2f33e2d2bbdd7e384a2ddcafa404f908a5bf7788c5da16184d254163a19f183cff7f9b84f9f7473178c97952fd56f8234d076e3ef31483a3317607d98a8a301079205fababe96986fd89d000035798551ff42ff33c298faf9a42a7f962a284340321f7b7682e1a055c30cfe69ad521ea102a6f8d21c2dae395bb4cc126c036452c77a17fe2acf876dbf716be59b9a84668960a51320a55b3ce68645fbf0483579a454b1208f88513371d2ac9be9816211e6d939c18a189a9d7b13c446274a19b0d1e9e7a38a17195746c2fa70f297a83e08682596ea204a8c54ea15b5db4d5232addbfe1512c2b785669a24c65b69012f8a824f7be6414ff506e36d4db4f08b80a9c08f2bbc6e0c8c770cc5ffc15b2f4a25ca5d676a74d9902404cd7b3e777c8713b15232819ead4b0afc854254449f5025f85fce71fcc0be2969ffe066e2de9558714037d4d41c4d13635958d9d8aeec067349062eea39e2fdbe5d2ee7e45f57ef66d5259cc89f0ac3acfe33f5b0dc45500bfd30bdd1633830c000ddaf265d77c2758b5970e3f06e8c3af11b595651a135df40979635ded78e4456cce7dbe0fe1fe8ccf499f96203a586278c791987226c3394040b249e506de029e002ad67ec9c6cb5dd60f7b1ea525bd5e42e0c664180d36485e1140f0d7107d5c4b9e4db45db2485f8f6a88a8f80f47f63b8559ad469a577df764ba926ad6df200964ad18893ec1637d8873e24b7e8c55c93841fe2744750988c896e825b154da7b3576208b18253afc0a99370e6a52d1ebd97d1835f470c2cd3658376f7aba451e6e545e3b17e5519d6f11b27de2817b724084e94f0f4a1d1ea75528bcefe3b54a14b432b1bebcb6d6b629cb35173ba10e98039e6a62db2e80f941cc7918f4b156deee6423b45b4931326b628e3ad45241efbe4114f739cc0c6e15ddae3547dc9811acb4cd6b5ace912ee34824073617cf178524b983e39b70e63e54539cf7810c806c3fdfae6bbcdaaa30274d3928df49473936d90fdebcb7af0cae86a3ddc05315cc9c856fe8a6a97a4878b686bb2b3c0015a52c899a1d3520b325b862c674dc01a6aa73b300258927ccc6bc910a5a7ec528739ed8354eb4f64ff7a9de9e92c3d8b2561f3d1358f5cb4dea6d49a05c0223b0ac1dc107e82b1c4d0393f1345a3e8cf97cc0be8f671e2a7b2346277c91e9e5d00897a880099bd0325b1cc8974a6b6444c1c0e4ff62ba073b587fb5015b11eeef70ee368ca8ee3c33582332a9d0ced2e4ad6e4ed052de7db83a0bc4c50469eb0314d5d9a896ce57f26f635d75bca6f7203a8837e3b0db07dcc316565711e2516eb7dde42cee01047c97412f96dc201151c552dd9e77918d01d12c3aa071bca15eb23b47ecc37efd8dd27ff0a51ebf31c3bba46f662a7f606bf41bbb9da9bed6d77d4338cd168c169a484011668b54824982269dbef008a59be08c5e5b18c5363886cc829e422c138919e856957aab256d55e1f45650111536587609ba2b7355a469f9288954753f639b6a21136ef81eea48df326584898c4637a8aeb5fa5ebcac34eb3762a715f52750727aa8dd2b4c50517c94afb3cddc0721254193a4d0f701cafb066e7c9097f454859aa6ccd51b356da2e6e868a7dafa00000000000000000000000000000000000000000000000000000610181b21272d31", hex.EncodeToString(sig))

	// Verify the returned block root matches the expected root using the proposer signature
	// domain.
	wantedBlockRoot, err := signing.ComputeSigningRoot(b, proposerDomain)
	if err != nil {
		require.NoError(t, err)
	}
	require.DeepEqual(t, wantedBlockRoot, blockRoot)
}

func TestGetGraffiti_Ok(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := &mocks{
		validatorClient: validatormock.NewMockValidatorClient(ctrl),
	}
	pubKey := [dilithiumlib.CryptoPublicKeyBytes]byte{'a'}
	tests := []struct {
		name string
		v    *validator
		want []byte
	}{
		{name: "use default cli graffiti",
			v: &validator{
				graffiti: []byte{'b'},
				graffitiStruct: &graffiti.Graffiti{
					Default: "c",
					Random:  []string{"d", "e"},
					Specific: map[primitives.ValidatorIndex]string{
						1: "f",
						2: "g",
					},
				},
			},
			want: []byte{'b'},
		},
		{name: "use default file graffiti",
			v: &validator{
				validatorClient: m.validatorClient,
				graffitiStruct: &graffiti.Graffiti{
					Default: "c",
				},
			},
			want: []byte{'c'},
		},
		{name: "use random file graffiti",
			v: &validator{
				validatorClient: m.validatorClient,
				graffitiStruct: &graffiti.Graffiti{
					Random:  []string{"d"},
					Default: "c",
				},
			},
			want: []byte{'d'},
		},
		{name: "use validator file graffiti, has validator",
			v: &validator{
				validatorClient: m.validatorClient,
				graffitiStruct: &graffiti.Graffiti{
					Random:  []string{"d"},
					Default: "c",
					Specific: map[primitives.ValidatorIndex]string{
						1: "f",
						2: "g",
					},
				},
			},
			want: []byte{'g'},
		},
		{name: "use validator file graffiti, none specified",
			v: &validator{
				validatorClient: m.validatorClient,
				graffitiStruct:  &graffiti.Graffiti{},
			},
			want: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.name, "use default cli graffiti") {
				m.validatorClient.EXPECT().
					ValidatorIndex(gomock.Any(), &zondpb.ValidatorIndexRequest{PublicKey: pubKey[:]}).
					Return(&zondpb.ValidatorIndexResponse{Index: 2}, nil)
			}
			got, err := tt.v.getGraffiti(context.Background(), pubKey)
			require.NoError(t, err)
			require.DeepEqual(t, tt.want, got)
		})
	}
}

func TestGetGraffitiOrdered_Ok(t *testing.T) {
	pubKey := [dilithiumlib.CryptoPublicKeyBytes]byte{'a'}
	valDB := testing2.SetupDB(t, [][dilithiumlib.CryptoPublicKeyBytes]byte{pubKey})
	ctrl := gomock.NewController(t)
	m := &mocks{
		validatorClient: validatormock.NewMockValidatorClient(ctrl),
	}
	m.validatorClient.EXPECT().
		ValidatorIndex(gomock.Any(), &zondpb.ValidatorIndexRequest{PublicKey: pubKey[:]}).
		Times(5).
		Return(&zondpb.ValidatorIndexResponse{Index: 2}, nil)

	v := &validator{
		db:              valDB,
		validatorClient: m.validatorClient,
		graffitiStruct: &graffiti.Graffiti{
			Ordered: []string{"a", "b", "c"},
			Default: "d",
		},
	}
	for _, want := range [][]byte{{'a'}, {'b'}, {'c'}, {'d'}, {'d'}} {
		got, err := v.getGraffiti(context.Background(), pubKey)
		require.NoError(t, err)
		require.DeepEqual(t, want, got)
	}
}
