package beacon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	testing2 "github.com/theQRL/qrysm/v4/beacon-chain/blockchain/testing"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	doublylinkedtree "github.com/theQRL/qrysm/v4/beacon-chain/forkchoice/doubly-linked-tree"
	"github.com/theQRL/qrysm/v4/beacon-chain/rpc/testutil"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	mockSync "github.com/theQRL/qrysm/v4/beacon-chain/sync/initial-sync/testing"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zond "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	mock2 "github.com/theQRL/qrysm/v4/testing/mock"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestPublishBlock(t *testing.T) {
	ctrl := gomock.NewController(t)

	t.Run("Phase 0", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_Phase0)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(phase0Block)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("Altair", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_Altair)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(altairBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("Bellatrix", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_Bellatrix)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(bellatrixBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("Capella", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_Capella)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(capellaBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("invalid block", func(t *testing.T) {
		server := &Server{
			SyncChecker: &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(blindedBellatrixBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusBadRequest, writer.Code)
		assert.Equal(t, true, strings.Contains(writer.Body.String(), "Body does not represent a valid block type"))
	})
	t.Run("syncing", func(t *testing.T) {
		chainService := &testing2.ChainService{}
		server := &Server{
			SyncChecker:           &mockSync.Sync{IsSyncing: true},
			HeadFetcher:           chainService,
			TimeFetcher:           chainService,
			OptimisticModeFetcher: chainService,
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte("foo")))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusServiceUnavailable, writer.Code)
		assert.Equal(t, true, strings.Contains(writer.Body.String(), "Beacon node is currently syncing and not serving request on that endpoint"))
	})
}

func TestPublishBlockSSZ(t *testing.T) {
	ctrl := gomock.NewController(t)

	t.Run("Bellatrix", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_Bellatrix)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}
		var bellablock SignedBeaconBlockBellatrix
		err := json.Unmarshal([]byte(bellatrixBlock), &bellablock)
		require.NoError(t, err)
		genericBlock, err := bellablock.ToGeneric()
		require.NoError(t, err)
		sszvalue, err := genericBlock.GetBellatrix().MarshalSSZ()
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader(sszvalue))
		request.Header.Set("Accept", "application/octet-stream")
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("Capella", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_Capella)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		var cblock SignedBeaconBlockCapella
		err := json.Unmarshal([]byte(capellaBlock), &cblock)
		require.NoError(t, err)
		genericBlock, err := cblock.ToGeneric()
		require.NoError(t, err)
		sszvalue, err := genericBlock.GetCapella().MarshalSSZ()
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader(sszvalue))
		request.Header.Set("Accept", "application/octet-stream")
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("invalid block", func(t *testing.T) {
		server := &Server{
			SyncChecker: &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(blindedBellatrixBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlock(writer, request)
		assert.Equal(t, http.StatusBadRequest, writer.Code)
		assert.Equal(t, true, strings.Contains(writer.Body.String(), "Body does not represent a valid block type"))
	})
}

func TestPublishBlindedBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Run("Phase 0", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_Phase0)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(phase0Block)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlindedBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("Altair", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_Altair)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(altairBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlindedBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("Bellatrix", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_BlindedBellatrix)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(blindedBellatrixBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlindedBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("Capella", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_BlindedCapella)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(blindedCapellaBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlindedBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("invalid block", func(t *testing.T) {
		server := &Server{
			SyncChecker: &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(bellatrixBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlindedBlock(writer, request)
		assert.Equal(t, http.StatusBadRequest, writer.Code)
		assert.Equal(t, true, strings.Contains(writer.Body.String(), "Body does not represent a valid block type"))
	})
	t.Run("syncing", func(t *testing.T) {
		chainService := &testing2.ChainService{}
		server := &Server{
			SyncChecker:           &mockSync.Sync{IsSyncing: true},
			HeadFetcher:           chainService,
			TimeFetcher:           chainService,
			OptimisticModeFetcher: chainService,
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte("foo")))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlindedBlock(writer, request)
		assert.Equal(t, http.StatusServiceUnavailable, writer.Code)
		assert.Equal(t, true, strings.Contains(writer.Body.String(), "Beacon node is currently syncing and not serving request on that endpoint"))
	})
}

func TestPublishBlindedBlockSSZ(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Run("Capella", func(t *testing.T) {
		v1alpha1Server := mock2.NewMockBeaconNodeValidatorServer(ctrl)
		v1alpha1Server.EXPECT().ProposeBeaconBlock(gomock.Any(), mock.MatchedBy(func(req *zond.GenericSignedBeaconBlock) bool {
			_, ok := req.Block.(*zond.GenericSignedBeaconBlock_BlindedCapella)
			return ok
		}))
		server := &Server{
			V1Alpha1ValidatorServer: v1alpha1Server,
			SyncChecker:             &mockSync.Sync{IsSyncing: false},
		}

		var cblock SignedBlindedBeaconBlock
		err := json.Unmarshal([]byte(blindedCapellaBlock), &cblock)
		require.NoError(t, err)
		genericBlock, err := cblock.ToGeneric()
		require.NoError(t, err)
		sszvalue, err := genericBlock.GetBlindedCapella().MarshalSSZ()
		require.NoError(t, err)
		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader(sszvalue))
		request.Header.Set("Accept", "application/octet-stream")
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlindedBlock(writer, request)
		assert.Equal(t, http.StatusOK, writer.Code)
	})
	t.Run("invalid block", func(t *testing.T) {
		server := &Server{
			SyncChecker: &mockSync.Sync{IsSyncing: false},
		}

		request := httptest.NewRequest(http.MethodPost, "http://foo.example", bytes.NewReader([]byte(bellatrixBlock)))
		writer := httptest.NewRecorder()
		writer.Body = &bytes.Buffer{}
		server.PublishBlindedBlock(writer, request)
		assert.Equal(t, http.StatusBadRequest, writer.Code)
		assert.Equal(t, true, strings.Contains(writer.Body.String(), "Body does not represent a valid block type"))
	})
}

func TestValidateConsensus(t *testing.T) {
	ctx := context.Background()

	parentState, privs := util.DeterministicGenesisState(t, params.MinimalSpecConfig().MinGenesisActiveValidatorCount)
	parentBlock, err := util.GenerateFullBlock(parentState, privs, util.DefaultBlockGenConfig(), parentState.Slot())
	require.NoError(t, err)
	parentSbb, err := blocks.NewSignedBeaconBlock(parentBlock)
	require.NoError(t, err)
	st, err := transition.ExecuteStateTransition(ctx, parentState, parentSbb)
	require.NoError(t, err)
	block, err := util.GenerateFullBlock(st, privs, util.DefaultBlockGenConfig(), st.Slot())
	require.NoError(t, err)
	sbb, err := blocks.NewSignedBeaconBlock(block)
	require.NoError(t, err)
	parentRoot, err := parentSbb.Block().HashTreeRoot()
	require.NoError(t, err)
	server := &Server{
		Blocker: &testutil.MockBlocker{RootBlockMap: map[[32]byte]interfaces.ReadOnlySignedBeaconBlock{parentRoot: parentSbb}},
		Stater:  &testutil.MockStater{StatesByRoot: map[[32]byte]state.BeaconState{bytesutil.ToBytes32(parentBlock.Block.StateRoot): parentState}},
	}

	require.NoError(t, server.validateConsensus(ctx, sbb))
}

func TestValidateEquivocation(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		st, err := util.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(10))
		fc := doublylinkedtree.New()
		require.NoError(t, fc.InsertNode(context.Background(), st, bytesutil.ToBytes32([]byte("root"))))
		server := &Server{
			ForkchoiceFetcher: &testing2.ChainService{ForkChoiceStore: fc},
		}
		blk, err := blocks.NewSignedBeaconBlock(util.NewBeaconBlock())
		require.NoError(t, err)
		blk.SetSlot(st.Slot() + 1)

		require.NoError(t, server.validateEquivocation(blk.Block()))
	})
	t.Run("block already exists", func(t *testing.T) {
		st, err := util.NewBeaconState()
		require.NoError(t, err)
		require.NoError(t, st.SetSlot(10))
		fc := doublylinkedtree.New()
		require.NoError(t, fc.InsertNode(context.Background(), st, bytesutil.ToBytes32([]byte("root"))))
		server := &Server{
			ForkchoiceFetcher: &testing2.ChainService{ForkChoiceStore: fc},
		}
		blk, err := blocks.NewSignedBeaconBlock(util.NewBeaconBlock())
		require.NoError(t, err)
		blk.SetSlot(st.Slot())

		assert.ErrorContains(t, "already exists", server.validateEquivocation(blk.Block()))
	})
}

const (
	phase0Block = `{
  "message": {
    "slot": "1",
    "proposer_index": "1",
    "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "body": {
      "randao_reveal": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
      "zond1_data": {
        "deposit_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "deposit_count": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
      },
      "graffiti": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
      "proposer_slashings": [
        {
          "signed_header_1": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          },
          "signed_header_2": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "attester_slashings": [
        {
          "attestation_1": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          },
          "attestation_2": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          }
        }
      ],
      "attestations": [
        {
          "aggregation_bits": "0x01",
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
          "data": {
            "slot": "1",
            "index": "1",
            "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "source": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "target": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            }
          }
        }
      ],
      "deposits": [
        {
          "proof": [
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
          ],
          "data": {
            "pubkey": "0x93247f2209abcacf57b75a51dafae777f9dd38bc7053d1af526f220a7489a6d3a2753e5f3e8b1cfe39b56f43611df74a",
            "withdrawal_credentials": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "amount": "1",
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "voluntary_exits": [
        {
          "message": {
            "epoch": "1",
            "validator_index": "1"
          },
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
        }
      ]
    }
  },
  "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
}`
	altairBlock = `{
  "message": {
    "slot": "1",
    "proposer_index": "1",
    "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "body": {
      "randao_reveal": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
      "zond1_data": {
        "deposit_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "deposit_count": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
      },
      "graffiti": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
      "proposer_slashings": [
        {
          "signed_header_1": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          },
          "signed_header_2": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "attester_slashings": [
        {
          "attestation_1": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          },
          "attestation_2": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          }
        }
      ],
      "attestations": [
        {
          "aggregation_bits": "0x01",
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
          "data": {
            "slot": "1",
            "index": "1",
            "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "source": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "target": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            }
          }
        }
      ],
      "deposits": [
        {
          "proof": [
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
          ],
          "data": {
            "pubkey": "0x93247f2209abcacf57b75a51dafae777f9dd38bc7053d1af526f220a7489a6d3a2753e5f3e8b1cfe39b56f43611df74a",
            "withdrawal_credentials": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "amount": "1",
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "voluntary_exits": [
        {
          "message": {
            "epoch": "1",
            "validator_index": "1"
          },
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
        }
      ],
      "sync_aggregate": {
        "sync_committee_bits": "0x01",
        "sync_committee_signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
      }
    }
  },
  "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
}`
	bellatrixBlock = `{
  "message": {
    "slot": "1",
    "proposer_index": "1",
    "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "body": {
      "randao_reveal": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
      "zond1_data": {
        "deposit_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "deposit_count": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
      },
      "graffiti": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
      "proposer_slashings": [
        {
          "signed_header_1": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          },
          "signed_header_2": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "attester_slashings": [
        {
          "attestation_1": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          },
          "attestation_2": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          }
        }
      ],
      "attestations": [
        {
          "aggregation_bits": "0x01",
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
          "data": {
            "slot": "1",
            "index": "1",
            "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "source": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "target": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            }
          }
        }
      ],
      "deposits": [
        {
          "proof": [
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
          ],
          "data": {
            "pubkey": "0x93247f2209abcacf57b75a51dafae777f9dd38bc7053d1af526f220a7489a6d3a2753e5f3e8b1cfe39b56f43611df74a",
            "withdrawal_credentials": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "amount": "1",
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "voluntary_exits": [
        {
          "message": {
            "epoch": "1",
            "validator_index": "1"
          },
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
        }
      ],
      "sync_aggregate": {
        "sync_committee_bits": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "sync_committee_signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
      },
      "execution_payload": {
        "parent_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "fee_recipient": "0xabcf8e0d4e9587369b2301d0790347320302cc09",
        "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "receipts_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "logs_bloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "prev_randao": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "block_number": "1",
        "gas_limit": "1",
        "gas_used": "1",
        "timestamp": "1",
        "extra_data": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "base_fee_per_gas": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "transactions": [
          "0x02f878831469668303f51d843b9ac9f9843b9aca0082520894c93269b73096998db66be0441e836d873535cb9c8894a19041886f000080c001a031cc29234036afbf9a1fb9476b463367cb1f957ac0b919b69bbc798436e604aaa018c4e9c3914eb27aadd0b91e10b18655739fcf8c1fc398763a9f1beecb8ddc86"
        ]
      }
    }
  },
  "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
}`
	blindedBellatrixBlock = `{
  "message": {
    "slot": "1",
    "proposer_index": "1",
    "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "body": {
      "randao_reveal": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
      "zond1_data": {
        "deposit_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "deposit_count": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
      },
      "graffiti": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
      "proposer_slashings": [
        {
          "signed_header_1": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          },
          "signed_header_2": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "attester_slashings": [
        {
          "attestation_1": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          },
          "attestation_2": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          }
        }
      ],
      "attestations": [
        {
          "aggregation_bits": "0x01",
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
          "data": {
            "slot": "1",
            "index": "1",
            "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "source": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "target": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            }
          }
        }
      ],
      "deposits": [
        {
          "proof": [
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
          ],
          "data": {
            "pubkey": "0x93247f2209abcacf57b75a51dafae777f9dd38bc7053d1af526f220a7489a6d3a2753e5f3e8b1cfe39b56f43611df74a",
            "withdrawal_credentials": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "amount": "1",
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "voluntary_exits": [
        {
          "message": {
            "epoch": "1",
            "validator_index": "1"
          },
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
        }
      ],
      "sync_aggregate": {
        "sync_committee_bits": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "sync_committee_signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
      },
      "execution_payload_header": {
        "parent_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "fee_recipient": "0xabcf8e0d4e9587369b2301d0790347320302cc09",
        "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "receipts_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "logs_bloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "prev_randao": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "block_number": "1",
        "gas_limit": "1",
        "gas_used": "1",
        "timestamp": "1",
        "extra_data": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "base_fee_per_gas": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "transactions_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
      }
    }
  },
  "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
}`
	capellaBlock = `{
  "message": {
    "slot": "1",
    "proposer_index": "1",
    "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "body": {
      "randao_reveal": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
      "zond1_data": {
        "deposit_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "deposit_count": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
      },
      "graffiti": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
      "proposer_slashings": [
        {
          "signed_header_1": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          },
          "signed_header_2": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "attester_slashings": [
        {
          "attestation_1": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          },
          "attestation_2": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          }
        }
      ],
      "attestations": [
        {
          "aggregation_bits": "0x01",
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
          "data": {
            "slot": "1",
            "index": "1",
            "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "source": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "target": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            }
          }
        }
      ],
      "deposits": [
        {
          "proof": [
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
          ],
          "data": {
            "pubkey": "0x93247f2209abcacf57b75a51dafae777f9dd38bc7053d1af526f220a7489a6d3a2753e5f3e8b1cfe39b56f43611df74a",
            "withdrawal_credentials": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "amount": "1",
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "voluntary_exits": [
        {
          "message": {
            "epoch": "1",
            "validator_index": "1"
          },
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
        }
      ],
      "sync_aggregate": {
        "sync_committee_bits": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "sync_committee_signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
      },
      "execution_payload": {
        "parent_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "fee_recipient": "0xabcf8e0d4e9587369b2301d0790347320302cc09",
        "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "receipts_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "logs_bloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "prev_randao": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "block_number": "1",
        "gas_limit": "1",
        "gas_used": "1",
        "timestamp": "1",
        "extra_data": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "base_fee_per_gas": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "transactions": [
          "0x02f878831469668303f51d843b9ac9f9843b9aca0082520894c93269b73096998db66be0441e836d873535cb9c8894a19041886f000080c001a031cc29234036afbf9a1fb9476b463367cb1f957ac0b919b69bbc798436e604aaa018c4e9c3914eb27aadd0b91e10b18655739fcf8c1fc398763a9f1beecb8ddc86"
        ],
        "withdrawals": [
          {
            "index": "1",
            "validator_index": "1",
            "address": "0xabcf8e0d4e9587369b2301d0790347320302cc09",
            "amount": "1"
          }
        ]
      },
      "dilithium_to_execution_changes": [
        {
          "message": {
            "validator_index": "1",
            "from_dilithium_pubkey": "0x93247f2209abcacf57b75a51dafae777f9dd38bc7053d1af526f220a7489a6d3a2753e5f3e8b1cfe39b56f43611df74a",
            "to_execution_address": "0xabcf8e0d4e9587369b2301d0790347320302cc09"
          },
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
        }
      ]
    }
  },
  "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
}`
	blindedCapellaBlock = `{
  "message": {
    "slot": "1",
    "proposer_index": "1",
    "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
    "body": {
      "randao_reveal": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
      "zond1_data": {
        "deposit_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "deposit_count": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
      },
      "graffiti": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
      "proposer_slashings": [
        {
          "signed_header_1": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          },
          "signed_header_2": {
            "message": {
              "slot": "1",
              "proposer_index": "1",
              "parent_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "body_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "attester_slashings": [
        {
          "attestation_1": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          },
          "attestation_2": {
            "attesting_indices": [
              "1"
            ],
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
            "data": {
              "slot": "1",
              "index": "1",
              "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
              "source": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              },
              "target": {
                "epoch": "1",
                "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
              }
            }
          }
        }
      ],
      "attestations": [
        {
          "aggregation_bits": "0x01",
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505",
          "data": {
            "slot": "1",
            "index": "1",
            "beacon_block_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "source": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            },
            "target": {
              "epoch": "1",
              "root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
            }
          }
        }
      ],
      "deposits": [
        {
          "proof": [
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
          ],
          "data": {
            "pubkey": "0x93247f2209abcacf57b75a51dafae777f9dd38bc7053d1af526f220a7489a6d3a2753e5f3e8b1cfe39b56f43611df74a",
            "withdrawal_credentials": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
            "amount": "1",
            "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
          }
        }
      ],
      "voluntary_exits": [
        {
          "message": {
            "epoch": "1",
            "validator_index": "1"
          },
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
        }
      ],
      "sync_aggregate": {
        "sync_committee_bits": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "sync_committee_signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
      },
      "execution_payload_header": {
        "parent_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "fee_recipient": "0xabcf8e0d4e9587369b2301d0790347320302cc09",
        "state_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "receipts_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "logs_bloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "prev_randao": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "block_number": "1",
        "gas_limit": "1",
        "gas_used": "1",
        "timestamp": "1",
        "extra_data": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "base_fee_per_gas": "1",
        "block_hash": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "transactions_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2",
        "withdrawals_root": "0xcf8e0d4e9587369b2301d0790347320302cc0943d5a1884560367e8208d920f2"
      },
      "dilithium_to_execution_changes": [
        {
          "message": {
            "validator_index": "1",
            "from_dilithium_pubkey": "0x93247f2209abcacf57b75a51dafae777f9dd38bc7053d1af526f220a7489a6d3a2753e5f3e8b1cfe39b56f43611df74a",
            "to_execution_address": "0xabcf8e0d4e9587369b2301d0790347320302cc09"
          },
          "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
        }
      ]
    }
  },
  "signature": "0x1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505cc411d61252fb6cb3fa0017b679f8bb2305b26a285fa2737f175668d0dff91cc1b66ac1fb663c9bc59509846d6ec05345bd908eda73e670af888da41af171505"
}`
)