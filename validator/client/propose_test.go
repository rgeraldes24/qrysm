package client

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	logTest "github.com/sirupsen/logrus/hooks/test"
	common2 "github.com/theQRL/go-qrllib/common"
	dilithium2 "github.com/theQRL/go-qrllib/dilithium"
	lruwrpr "github.com/theQRL/qrysm/v4/cache/lru"
	"github.com/theQRL/qrysm/v4/config/params"
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
	pri, err := dilithium.SecretKeyFromBytes(bytesutil.PadTo(b, common2.SeedSize))
	require.NoError(t, err, "Failed to generate key from bytes")
	return keypair{pub: bytesutil.ToBytes2592(pri.PublicKey().Marshal()), pri: pri}
}

func setup(t *testing.T) (*validator, *mocks, dilithium.DilithiumKey, func()) {
	validatorKey, err := dilithium.RandKey()
	require.NoError(t, err)
	return setupWithKey(t, validatorKey)
}

func setupWithKey(t *testing.T, validatorKey dilithium.DilithiumKey) (*validator, *mocks, dilithium.DilithiumKey, func()) {
	var pubKey [dilithium2.CryptoPublicKeyBytes]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	valDB := testing2.SetupDB(t, [][dilithium2.CryptoPublicKeyBytes]byte{pubKey})
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
	var pubKey [dilithium2.CryptoPublicKeyBytes]byte
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	validator.ProposeBlock(context.Background(), 0, pubKey)

	require.LogsContain(t, hook, "Assigned to genesis slot, skipping proposal")
}

func TestProposeBlock_DomainDataFailed(t *testing.T) {
	hook := logTest.NewGlobal()
	validator, m, validatorKey, finish := setup(t)
	defer finish()
	var pubKey [dilithium2.CryptoPublicKeyBytes]byte
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
	var pubKey [dilithium2.CryptoPublicKeyBytes]byte
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
			var pubKey [dilithium2.CryptoPublicKeyBytes]byte
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
			var pubKey [dilithium2.CryptoPublicKeyBytes]byte
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
			var pubKey [dilithium2.CryptoPublicKeyBytes]byte
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
	var pubKey [dilithium2.CryptoPublicKeyBytes]byte
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
			var pubKey [dilithium2.CryptoPublicKeyBytes]byte
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
			var pubKey [dilithium2.CryptoPublicKeyBytes]byte
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

// @TODO(rgeraldes24) double check tests - different results
/*
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
	// TODO(rgeraldes24) - double check
	// require.Equal(t, "8a0f1f3a9e029f3bfed8f46ef84159e5039a6bbf587ae8baa8dd7976f45bfa9060831672d0b97c40cc6afc2391f8370c9de9d61294f4e6c8af8db69d154be2fdb13d9df5bf2e5be0425a2a9fb6be9fb3721ce5a9f30e6e7e9f1c91c30f069fe62b82c58a0bc105157a5bfefb159fd2eb2794b543a25eacb5f55d757ca4cf94a1af5063468bd0d0a15fd5ad110019c8e9dc4a303d289073a5a978005906192c6470a82436e7f68f24b2cdb49a32bfd285edb74796e9eee79e142a5a2a9dd68f2c90dbb60969b06b4c7e05c009012fb4a81fa2bd4ca4af8e19f67c1e187af4cc08553e3c2be0b44601a2f96cc1d1321b6d2872ebeb01581216cb9a3faa3636181344db1d2805f83a89656ac41d7ff101249d9dfeead9d28875c7dfba567443e33fcf153f1fc8a76446745cd3c5c244deefe0151000412d5262f74e5210cf8ce4e84732a36741db3539b3116ea4c3ab023d692ff83bceed46bed43d4df68e7932ff226dd4930a8f072a7d830597d8fccabf018e8c9b2a078e318881ba1510abec7ad0289a81b39daba6d3837ba9b52e16aaccd2a10cb79679cc66407d56bb692b44ec49fdd7c4a4e63d163df147e5023c10d286a0512c86fb55169b9d75df5c5d07ec530f334da5d81337ffe59d20396a6a71b3137550b68b4cc89c61798e9c5d3a6d57ce68d1f6e29d00f00bb7f25663be4c6e2bad0d2c9c3d7feb1da10687faf4488ca13a1729ed2f34cba4944a33f5457059b9e53cf8393d1e7b4437c50f8617bc6a2b3356d52d2f15236e5a501ea4b6b7d8d1c1a473f3dcd4b8dc3d193a34da8b470e94bbde07ec41b628e590a9700ea666acff250b84533432be7a2db303798e091dacdb623f46fb60da4205cac39241100ad3fef7e3f6f7e544075451e370c761890bda8fa92bcab473ec8153135cf7e2dda782089ba4e16851a683d9a34b33253723b3bce6e4e6440fc2dda00fb5c0c9d0f45c000c5ef039acb552b5194bdfc3ee354bfa3da6d9840b6f181083919cd93511944cf8ee9c106d82dc7c5b482afa5cced010bf01c4bfe7e63fd6ed1ca45e0de2c49e5ad610cf9b1cbf07390266a9456122ab10b95cc44e9854588a57ff716386cb47994368a01e58b93e4cfabf6da3853904baac70f8ed3ebcafdb3896b030c7c0b5087f7604f16d56cd92fff8a29013eb7cef15cb26aa307314759a644d4ad3b7c08588e150c63bce1774e2b85556d4a2e73d02a89d0a62913af375b930114a480785f20093f7b051eef35e8c5932d66ef360652aab7641028c32679f2b1fc93311321e2441617acefc103d282bfb17e50984ddae1ec327258445adc1e188a7e90d5e9d8153c787bfc0d3de6057064a1aa33b93b4e3a11d1ed64d8d23a738d947a177ce9076fb79dab0c0f366f218f1b5e251eec4b745d29a00ef1ce1343e5d19db152d223213056046d2dac44b50523c9df302b942868955da16738b1608bd8ca3f2751427ff3526e69f1d94e676af18f89a9f45e059c0970314db4ec76731c5c232d181d0ae65cacf96cb7ce5d72c440d408150db466a002bdccf717fc2441623c7d2b16ae7ae209df1824afc5d18e419c22ae598f3bc6a80faa446bb0c1d47883c10970edeee3a4ba835000407c7d6937200c1f9914260a34a9c056f262bc9ca726027cc3d6e04ccd58dcc1dce466172434f66caa2efb1f36cd0c46d3cc3b1cee6da2bf255804e69c4099dbe9f751106d40dca1e91c42ba3cca1ac8d618e2c22f2023440247f4258c15776f48fa48bbb34fbe55339a51e397e3027a00c41071cf255b1b273b4da2564dd6c6ab3bed83c76f7883f76153b7ce6e00b0ff2ad7db5f8933cdf45322ddd24f5eebe03792b6ddd14a4b8c654c65a8cb0ea428589aa5f5f41a8d8f188e89e0b878450e216fd97f3ea49bd7f5d6aa65191979070bee3047d593338bb1570d35576d65f4e610b99ed9cd4f5289218960b3552dc0aa435a4ef75d7b88f6fc13daa4b03f0de91512ca33d99501b72d9164f35cf071cda47334eb73392c9308f3d5054d888bc915e2a62bc4a60ca97fe6601d1b681ffd1b7c42d844f3bc44cc1657df7a999dbb582ba53d5c8ab275473c77055e05107757bd29f6a2ba013492ada626872773a773ee31dff1e4958bc95d6fa00a50fdf6df8f95d6735d32aaabf73261c5b7620b026e406c55e4631f23eaa8f6d847fcd527f16e623c70920ac3ecf44140314d57348d1d0612c52faec0963562237521f7dbd9ceb456ffd561c81df02c6e019eeae12d268a3095e5b9c0c344ae3d9a8ac601c30a96b059c83cb46ac2b2e736032cf03b0f98249334fe5e52db5caf2f44e22cef1a5c430999f91db279b1388978953c42c294f57ced09082e03342a8fe3d5aa8605baf27143055a406f2f975033fcce65ac9ca40fca5ff3581f0d0a87ac6bfd8208569ec43c764d634e65e73760731b7e92b6fdd35291bc69d66933c145ecc72e4463ae2daa7f8c4729db4de3922e0a8e98599d081b0bfdf4765a276fd0ebbaa63ba9c290a9ce09b0b911bc333040170bdbbeae13a6a1290dc5649cb571ac9c2b1bc7e12b56bd96e32897a68b1861865b0f761fd981b476d3222b044ace603e1a0c998e958489ce0cc4b5b2f420dcf6ff76089356e20242bfea68cf576fb0e5db65e2ceb79b304500237bc5a8d6fcefdb1274d9771f2be1a471249a1fd252a5a6b8f832963c35b461a631cae57bdb2f424b0f03909b05796bd34a0d4262d6210fb00fd29dbbb94d636f2cf7d2f44181c514a9f61254797bc489c3e1e9b771e30a7c34aaff74348a02c0dcd383ac76a3dd4e7f9beb0019b378d7c95fa75915698c22a71eafc8f561d77b9e208472134b21a9be321d416c6749b598ea1120f2cb51001912d6bb8565b693f5ab5b080239ac64845d9c684ff468e91d42e6ee46fc282c4da54677e929a73f64915e5fa5edbf795212bc726e8f8f99b0179193bea7773169bd6e777a76fea117ef1669f14e6629ab8d4da45f557989f8b3ade0324bad327c64f7f7159679bc375c6d1617c745e2d2a2e6442b5af6aa417c356047f34168480925eece4d02dec8205aa04a5f63949a5a92ef3cce3a967e78766a8845f318e0d23ba7a4b025ce4d09ed5b7856dd73dba7b0c492f8093dfbfa515dfec0c2d3aaea4688a0c8f04f524f466656c6dd8a1e93e7a03e1ca72d69246a9e36918fc4d75c14ca983bd65d8e973a5b6fa01930cb57d710378ae194f1d0f50306a7cdca6e70fc59dd8598430d40343bd0eb79bb185c5118d01f30d26b3e413c08780e27029fcfcaead8a0cffcb818f2c8962601810b04124c17d43fa9d7ee0a79e2eab7588b8b95a1af90baa887af39216ebbc4cf65adf83cf8e0191040d83d1c392e8d75b70ef0280c3b0bfffa8f1b75d065ed841c7e32f0252df77756a262af0833ad0066e6ae4eb4a6351ccd4d1eb8710c828db7795939282f58a5bd08976c2766ab8a03aabc96611e270c083a010c4a80956c668f38ee677b78cf4bc87d45ecee19fc9e0e4fb8f3fd83acb80377b162d2c5696303f87e9c44d310bafbeea9fdb1bb05427835351878dd6a8084322037f4de68970c283e46cc311b270cf292b18169a0912859b83cd0eb93b91691633dabf95dd8d2f4d356c1f6bfb4980c4500ae9d931c64dfceb149fab89e77343e0098efc5ec8333974d213e944ea75f6ace7a99594d49340abee3177b02f70da37bedb51794e00da92de33a6072cf872459e8e1fedf1b0166a0f2d22e05d3965ee6dad20dc5998de9b733974b72468b78df8d6b01388d6549d6f6c31539b36062da92b357b087fc91b1abcb433f3720d1e7c8c00578adc43bba2d407d81bfa28d076217e2a20e9c012027a34f5b3a9f28aa935bf9fcb1f07368b99be7adfd524de83a1b62f7f2eff5d703ea0055e5f94afc57c21c95663e96bf013e8f9e7a33fefde5e97b9ea41d1f83dd68ef426418068fed7d2c6bd8f61ae1b4fa5529a471252b3f07f5704ce6f008cc978420230fea8eec21d55538a251eacc0bc71d8276666ada66e873fc81c647577278698efb9321f689143cdc0ff349dc35d74a330105981bd605080bd256c26f720bc2bf613ff45344bde471599f25664da03b4bd6f6517b91e6f4203f97b33b65328357ff06b439a67a48660b60d583cba9ea885dd2d6651c7e28d97aa31d992edf73ccd3596cf63ecf62d54a8059e576a89b159fce6f2f4f70c07c4b98c5b2b99fdebdb9f88d4d8940eed77392f17cd9879caf08df9597bba99af2b8b4b9f324a2726c092879cb9a3b6df407aee3498db90dbb42c982e02dc534ca2a0093fee60149ea6d5f9f01bc6a733566c18c93f7e008258f342ad2d5905b0302a97b65d7b2d35267b30801e3ae44878db58ea09a9bc34df737ad5bea374e73509af4a25434c575732d4e37615568ba775ebba6627a7ed4ad8edf220983bba55d94a78e0fc783a676d39dc7623e81a8c5386afb10ecc4758d53dd78226eadb5a284551f4f29732219c26cd144a97afe9d0e2316217a8b2b14cda71c57c0f991764561d927fc3e28fb8dd0f8e0484005d4eac520656ca2d76045df09226930e8ed87b17ea9a7c1c0d0a25c32697a7112b333cd8df23da4751cafc8f5f3bc68e07b07521e1328ca5c081d891976bab7d4de9f449feb6b0d5f76744546c298765f223f9af4db0b16bcf6c8648aa821819071fcceac693ce96609772a0584362d026c841424a7b3432ceca0bb595417a8d9dd27cf1bf9cb12a2dd3b9d56228892b5dff73fa2b26a7d21aa7677ac899a64a9d3f629c93dc1b3850dd464b12cce8f81c2f33e2d2bbdd7e384a2ddcafa404f908a5bf7788c5da16184d254163a19f183cff7f9b84f9f7473178c97952fd56f8234d076e3ef31483a3317607d98a8a301079205fababe96986fd89d000035798551ff42ff33c298faf9a42a7f962a284340321f7b7682e1a055c30cfe69ad521ea102a6f8d21c2dae395bb4cc126c036452c77a17fe2acf876dbf716be59b9a84668960a51320a55b3ce68645fbf0483579a454b1208f88513371d2ac9be9816211e6d939c18a189a9d7b13c446274a19b0d1e9e7a38a17195746c2fa70f297a83e08682596ea204a8c54ea15b5db4d5232addbfe1512c2b785669a24c65b69012f8a824f7be6414ff506e36d4db4f08b80a9c08f2bbc6e0c8c770cc5ffc15b2f4a25ca5d676a74d9902404cd7b3e777c8713b15232819ead4b0afc854254449f5025f85fce71fcc0be2969ffe066e2de9558714037d4d41c4d13635958d9d8aeec067349062eea39e2fdbe5d2ee7e45f57ef66d5259cc89f0ac3acfe33f5b0dc45500bfd30bdd1633830c000ddaf265d77c2758b5970e3f06e8c3af11b595651a135df40979635ded78e4456cce7dbe0fe1fe8ccf499f96203a586278c791987226c3394040b249e506de029e002ad67ec9c6cb5dd60f7b1ea525bd5e42e0c664180d36485e1140f0d7107d5c4b9e4db45db2485f8f6a88a8f80f47f63b8559ad469a577df764ba926ad6df200964ad18893ec1637d8873e24b7e8c55c93841fe2744750988c896e825b154da7b3576208b18253afc0a99370e6a52d1ebd97d1835f470c2cd3658376f7aba451e6e545e3b17e5519d6f11b27de2817b724084e94f0f4a1d1ea75528bcefe3b54a14b432b1bebcb6d6b629cb35173ba10e98039e6a62db2e80f941cc7918f4b156deee6423b45b4931326b628e3ad45241efbe4114f739cc0c6e15ddae3547dc9811acb4cd6b5ace912ee34824073617cf178524b983e39b70e63e54539cf7810c806c3fdfae6bbcdaaa30274d3928df49473936d90fdebcb7af0cae86a3ddc05315cc9c856fe8a6a97a4878b686bb2b3c0015a52c899a1d3520b325b862c674dc01a6aa73b300258927ccc6bc910a5a7ec528739ed8354eb4f64ff7a9de9e92c3d8b2561f3d1358f5cb4dea6d49a05c0223b0ac1dc107e82b1c4d0393f1345a3e8cf97cc0be8f671e2a7b2346277c91e9e5d00897a880099bd0325b1cc8974a6b6444c1c0e4ff62ba073b587fb5015b11eeef70ee368ca8ee3c33582332a9d0ced2e4ad6e4ed052de7db83a0bc4c50469eb0314d5d9a896ce57f26f635d75bca6f7203a8837e3b0db07dcc316565711e2516eb7dde42cee01047c97412f96dc201151c552dd9e77918d01d12c3aa071bca15eb23b47ecc37efd8dd27ff0a51ebf31c3bba46f662a7f606bf41bbb9da9bed6d77d4338cd168c169a484011668b54824982269dbef008a59be08c5e5b18c5363886cc829e422c138919e856957aab256d55e1f45650111536587609ba2b7355a469f9288954753f639b6a21136ef81eea48df326584898c4637a8aeb5fa5ebcac34eb3762a715f52750727aa8dd2b4c50517c94afb3cddc0721254193a4d0f701cafb066e7c9097f454859aa6ccd51b356da2e6e868a7dafa00000000000000000000000000000000000000000000000000000610181b21272d31", hex.EncodeToString(sig))
	require.Equal(t, "9f1d3a14f0e38b046eeda5a57ede31119c0cc2a16ee14c95c380cdaf3bfe70ac4a7fcd997cc9f5a08473a1c84132302a4545da8ac10f3867ad11d8a71595f1fcc9b747a546a87c7262ef98888b13477418a03e142ab9f3561008111a4a2cfe9538e2313556ba96b07fb16073f5a993a7184454aa5183884e74863856ed9f4a635bbd7c29f1205ff017f52c17f718fa35485721333642115de3c28c966ba2f69e7d47e735a550d5a34b1b3c543742f8fc12d1cff549c79c38fa53ce71aab3d9dca280d9a674e54a158c08a9d2e924399c1c6e8fb115a7535d22a79ed057d49c4f3d581a19acfa407bb70d6fe74dac53c2d4e50f1e842e180b5d4bfaad5ceecba8a285ec46e4b2d09a894ec3b5e5a4d0cf7273cba57eb421455f1f553bca3e9635b086aad045dd3febdb215f17b575e4419dca4eb68ad35afdf3affe514d4e7378e593fdbbb2dc73be115d137f19c53ba80bff819d5412601f02473c0497a340dd67897648014e264584845e5c1cde0554973d11496ff827e89d6549396c6a78d308afd0b82e059f3925d4c5be804179f035120e47dd4048dd67dbbc4cf9e9e55c76e27d9969130a59a0a98c576d683f1cf0a0c92fa88abfedf18805f61cc67242636c607191eaa380cd4999057dc8fe12616ebe9de32f2b66caf8401faaee629ff080d8470ac57d41fd9c64e5dc0e5fb237853bafc3821eec13b4444e449eafe4879ee80445771fba0bab1c0bba2773d4a6baea1ac2e8253f6446c0728b5afab1e2dadd3ce1d0449ee752ebb142314cef066b917aa645e90d47f228c223d2182b4333f6812cc7cb54614e5ac6de8f424958c4beb554264678fc3a0150156a7dce38433bbdd69002852d0cc9d7be11a9ba82917dbae12666de279fa25e3589f92ffa330ac173d31579714482ca64cbac572b2785d8b63b01f72365a5ca306a2a82090a4677054d385d016479c30f28bf23648d421d990f6e9e0e7d916d15eb229bbef8fd320c55e324e6d0e97c89ee6cf798591174f98f39a4f2c89b202a158697a61fc072f94d4b5e7396e8205617c9b0f24449450d792837adb2ef942ad278653435a28be97284af1022eb7158e9d852c8dd8f516e9d10b364a1b8dff320ac8050fe8891255cd5988bf14997b2810bcffc58f73401498b17ca3b9ea24132a9cc53e772b71c96d233fc7d9bdc99bba261114882274fa3fc5378b0f318f57dee1c7d087738e0c43c67ec89fbb8c8f88ae31763685758280550e5151bed42095d4a0894b64d5c40388c034d508db77805494529f21c2b7b6d42b882a9f36e7c7aea8c4e8549486f52bf5e94f27c5d13d129215e9befd67422f76eb2393b6cc31de78baa3404b1f0167f7c50fc658c0fc4f935524c549c7b28ca5eb6af25f9e438a8b9efbda939efddaa03d7723bd7302319fa0ba6c8a90d0cc81ab5eb70c0f768856db1e0ae15ed47fc808e81fd2abe1c575ae13e1dec2a0075d0fe65aaec3738ce555944e2653b753f83439b1bcee09758b6f532cc2bb890d28830483ca79e125600c41183fef25158d66b6ed95cc016380794f4d083f61eda79e018e607370603fb1f5a622ee91f666887389cc07a618a2ab773fbdb1b42ba651d6756aa7e3238265c4172c466a2dae4d07e00bda5a6726ab2128034f61e77f15105a66e7c1446e388094d770bc48fa80e1d90d6879bb76d3dd4283056fadaf78611b1ecc716b97a0ab6a8b6d7da86005af0f883bc4b8a4e0fef812e52018ef15029de9ecdbcbcb406b54047e789d5e1cf4f666a7bed8931357a173f1ca7ee96d55d8bdda270880949205aae0f01832e0c39182409bcb188a8a83f4532b3e138d4688405c77d8584bd66f161eb554a21a459114c59782f55234623878c1835b04e1261b1d67851364ecf869e225e9f1df9d3354237df8be2d27b9da39998d86974ab3e31c1a4d1901797fd1fc4835106ed99219dd2fa209b5f4f4155070efa5a4edecc6b7d2a7d7eabb3a758839acf048ac1c69317a724136ac245517bf4ef35eb9b290a88d09be0f8d2ec0eb8f9bf922011dbebd44ec506095b734d71328d3f1e73ce5b42b364b7231faba4b51aa93aafa32094f128dc3bf2caf97231e4debad5f95a43c98348511cb071322e89d6a1c3ffc93535d54ecb3f785d33ec4fa20b25cd63e8fdafba6c110dd7226485a657c7c4336f11498df323ed81ded5307845f669febffd68de6dcf26f0c4273bbd620c48696c364d0573db362c58d4d66d0b9a30668923fd8456e591b7d9c9d94bb756739e5473b052e09fa6c9c5bee2f9fcf50899c1e0120e4f1de581c9cbbe404f8f26e6a44e6c1509296089d22f7959b8dcd356aba496d64136d70750de2908d9f4e3b4a7e87a4ce7e9a798e8679b9f995b64c4c3cd1c499060586ebc35425999a9dc110b58296dce2e7b7fe60dfc4cb326a89ff5c2c1939d61b31fd7214f933279330caa3ee4b008ee02525090f8b6fb581bb03818c6d589cd970f77f160af07eae4142559db0b8e7bda05c876198274f72c261beb7743c063d3dfaf338aebd0a92d12de6025b60f575368cff9dc7e8bab3bac4deb0bdbfcd877f131ca17b73ea4fa6b1e2c1d650a166b37774f0c0b25bb566192a47461fe918677377100a07aac6ba400433e3421fc25b7ddb80147da465e535e4194f4ec347a6869fdd528115ba0527ee7b58f0a43c1f4458719b1479e139404df23d0d52129178bcf533b217972c2ad60abaee8c212ce6932a98f97d2aaa4f8d02415d25b418e137a37e97b0eb518035e7c85f00232647925b5c2dc7e617c4e0b9f7ba50771bb6b693d104112b8adec033bb995e487494ebae8b0cb93215f6afb1fdfb808fd7bc8b285f722954231eb0c4952bf00bbec055c77ae01907fb5ce10d12b2f0aab91de3db1795112f6783e28d6332630fae0d2571b076c7cb5904f482db0f35728b881f9f135b13c3aa40fc69ca0da7a6c9d86ce76dd01d0e0d34988a1f01eb7405a01f51bcd54d5491b0a3f6ea4026d942064a66a3ec025229558c63366d1bd3f1af6bef2db36127452eadc4537a2082d0a9bee9389b43e40eb7d8a6863d01f9ff87422053002248c1c1c9fc7f5a440b08b03f9abf6b6cd9f4f4b2348a64c09eefa30d81b6684a6db23e070dc7523a0aa9392f78425565940fef3be1d647d07fd7c5f3e48466255d8f8ca1c01b74f2125471f0118286565132eba78c9c40de29229a34535e66150fc318990d89bdfc449b43b6e4c91aa40dadb1c650c6f59d69e1ff36d5e122cb86ff9dbbdfdf355eea732aae30624c19ddbfe0a9171a563975a2d67b95ba6e64914a3d12e52b768b76b65b35baa3e809add7d7129eb3970b7972afae69f849a0720c7c09c801eca587c49245e9c7a9c91dd605d4930d76f5704981e8c9228530a3af14b67d399faa4294ecf6076f3e156bdbc07d01652940ef331ebf3ad3daced30d47d722b2530498d9594ba2d3806af7f9d31a4f845cb9fda7cdfe4cd93956c50523fcb85e26c535289957e89635232cf52bf106ec471436c1f5f31f5fe5a43cced519c404c64c0dd5105813427674fc06b4d8f6f030be6d7d92ad4867499503a301db25938cd5c0e9630b8bcb447f186f4cda68ef936a02f2c2790db86fbcfebb732c1efe348018e7d3c1f7e11c5d088b6eec3db3bd368b66f1e1d7830262af3b1837aebdba4806b2e3ac0f765c69c1e6a856083481c2f268fa82fe8e716843d04310d21f3dcfe147df8a1d1a7e01ac19483bda60ce626938f5eef898f5370c33996be710dc3482e5af16acb5b5e88043d64c4efea9ffa114ac08c95a4ea3d6d5e7051b2210bd03851c65e07b59a247b98344ce144e33f2b0b5068f416faeff243e24d5f38bd9f1d8b3238c15ac6f45ffa299f5d7516425f30b8fed0045874211475c13536269aee7457229fb986c7cb98b97e420cc44325254454e8f3313effaccd2b1f3c2e5e68ace4f9793000aa15527390ea39717d27b79fed62d6f472ba89c073037669d1637425fd18907b0b0b9522ffdfa63383e3a163ec33bdf388008d3e160f845e762584d4c80205f399b27f9f9c51eda52e8a04604efb265be3d091067490e35f4112b260a7a71fe6fa312713c0824f25dc423fda7e6f62db00316cbadc45a7ae077bf58ab69ccbfa2cae007e66be4c602009c9dd32dddb303f316aad289e23630b9316d57acaf3a10203f7911381b1cf2ba82f5d47950708345c68ecfb2609aacb7e9f596dc516b7def1c3a7906c6ec139d1de5fafe7fb5ce709501f961e65d19a867b179924b145d4902b8650c1ea4c77f941c1beb9d78a4099af7e6b403aeb82386d78e000d4cd4fad79e5d995b0e7e4924c1c4077a8298e54acc96a660aaf1b0c3c53107919d555105aaa3f415c55b0dbb93528ae5b4eee66b28dcf95cac238ae3c610a87277fec47eceb7518bd44d451c9575085c76f77a571aa2c799c0979dd9a80a4779790fa1941c7d9218b88604f22366c113373f1cd68aff6dfff1a191e5f6a4d88addd2b4628137a13babcfccd937ea006c6cce926b8bcd2938e8c5ed75a1b556d1ca2dc24d47780293cfd52b71f55ab70ce7db6bb95ec39c5ca67c9a4d45abd8997ad14bacbf532f8ad99c89853264795527f1cd8db0a41b7841f3634068ea4b8fd87735146c10e0bc238d91329bc252e1560a5eb9dd204f6cf530cad0d01e438370c9271c7fe9e0741651b836bf1dd72904a172993902de8fcd2de91f125f46eb1f252417852215016441b48f7930a7cc4a03aa9c817d34e8e751cc6013b9720267a09387fda342affbde577c0c44335f3a443486937d358b3ae39505b09af517e82a91e7a862d6a32e2a61cb444572ab7ccdf01281f0f6efcc22699de06c5ee869f7382f0ccb5d428ad491f85c934a59935a16ff6610548aa2fe04d29b5c5f74eba28a8fa87c4ab241f6e539c1f569b03f5820f748380e174fae55409a3846435036f16b884d50d2dfa8301abf19cc1cdd57748840ad7b41c4c29fd512aa0a1b0005e04e198377b09d197118976180c1abd1e666d72da39efba56b577bd94d6dfcd15b56caf90cc2bf4193fc6ec37fb59083e41b165b29a6381ad4202ad8d91e403541e46ffd082c13313af9e4f8b8976354b5794ef5afd877cd7ebef96ced4389de44f3c0a41e2e4b929bfc737e59d1f9fd511032d3507725bbb44b68f6ce941f457b6343594fbfed41acf6c902adb83f6e64b52b037e5cce639b31b897878f175307ca145528eff3c0f5f771c0332f2aa1f090bfc81387371e80010dca93d3a003931171fdd713226795843f17c984a701c051f1a9a6f624575fdff25b6483a4b407eea280f78fee70c40b66807e20d27659166447ecc96f43ae83b63b1c21dc2b0814bf90df21ec98ffe78975bacbd912b14931b646a2ea402cbefbd896d2c034c6d82035cb678910bea4ee8dcc2002a9e0894f572c69975faeda862cb59eef6f0ee47f137f335feab93cc429200c29791afe87809b7f8061a2fd141f7a2ba8290ca5447e25445ccb50895b376b7b164bcbf1903bea66d158993fde753149b7b566662b546c309ba88c5754efac0cd83a4798708f04c7aa4505c63f07c3a0d217c365b8d29a7f8740f286209561a96ea35dc31d7fc343403734dc5acbb0579a0ecc28f047130e82bc633f9c83d4707a5449670899252beb36c7d4ca7f997409c50c6ccd86c9f128f204aa1a496575535aeb93288c4a3d3449396bc831ee1887c5c6b6ff07b323d282037b6305da5aec43d255b078bdb6059a57757a211175a1a9ab28c396ac80c43e1da4efe0cdef40b8098b63fde846aa634736a5c39c0b608187f95c5c6d24958b878c31e270c2a052de8ffbe987a1fdf49f1b9d2d19684f385672612e324a624d56daccfaa5061892d4361eea7a81667e2a2a8969da11fa8128580f564bd5037c1ceed5659107bb7ffa1eeacd93dd1ec5e40fe179ad32a40feeee9626a7344d3abd595afd9bf3c59653fef1a645a16f404ebddb4160d7aa6bed3c15c42254a79055712376c4cee3ff62f1abf36ff353c25a82f932b8f755896a9e9f04fc6992a192ea4466cee54f2bcd5eff43b816923bb8cc5bf192497ee0a79df469640b318505cd35da2b72fa8d0695faf8ec5df716e6c411587c903d500913837e5e71769c26113c215d23f630fe36f50581ee01343402ec569d4f06ec848212d47dce03367cb2f8fd8dfa196def2eb7ae97d04249a2803dbaca727fee81cae5ac6926353e162f6d4681543a7451e27e744e087c5098a276a23a854150e1a9ef5d3bdbba7b14a6f70f0653b00fd3ffc23d9b9e16698643d9b3f451b529a6f01b0c0f9359b4758a84570920f8b6064ab6a6b1b12a02ad3e584648aa4d88b99b04c6ff037064893cbd8f8ff12305399add519333744475599b7b8f0071369b06721234447497bd9e3f612455e69a0c4fb071a1f262975768cc2c5d8f500000000000000000000000000000000000000070d171b1c252c38", hex.EncodeToString(sig))

	// Verify the returned block root matches the expected root using the proposer signature
	// domain.
	wantedBlockRoot, err := signing.ComputeSigningRoot(b, proposerDomain)
	if err != nil {
		require.NoError(t, err)
	}
	require.DeepEqual(t, wantedBlockRoot, blockRoot)
}
*/

func TestGetGraffiti_Ok(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := &mocks{
		validatorClient: validatormock.NewMockValidatorClient(ctrl),
	}
	pubKey := [dilithium2.CryptoPublicKeyBytes]byte{'a'}
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
	pubKey := [dilithium2.CryptoPublicKeyBytes]byte{'a'}
	valDB := testing2.SetupDB(t, [][dilithium2.CryptoPublicKeyBytes]byte{pubKey})
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
