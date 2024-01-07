package beacon

import (
	"context"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/theQRL/go-bitfield"
	mock "github.com/theQRL/qrysm/v4/beacon-chain/blockchain/testing"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/altair"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	"github.com/theQRL/qrysm/v4/beacon-chain/db"
	dbTest "github.com/theQRL/qrysm/v4/beacon-chain/db/testing"
	doublylinkedtree "github.com/theQRL/qrysm/v4/beacon-chain/forkchoice/doubly-linked-tree"
	"github.com/theQRL/qrysm/v4/beacon-chain/rpc/core"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stategen"
	mockstategen "github.com/theQRL/qrysm/v4/beacon-chain/state/stategen/mock"
	mockSync "github.com/theQRL/qrysm/v4/beacon-chain/sync/initial-sync/testing"
	"github.com/theQRL/qrysm/v4/cmd"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	blocktest "github.com/theQRL/qrysm/v4/consensus-types/blocks/testing"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
	qrysmTime "github.com/theQRL/qrysm/v4/time"
	"github.com/theQRL/qrysm/v4/time/slots"
	"google.golang.org/protobuf/proto"
)

const (
	errNoEpochInfoError = "Cannot retrieve information about an epoch in the future"
)

func TestServer_GetValidatorActiveSetChanges_CannotRequestFutureEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()
	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		BeaconDB: beaconDB,
	}

	wanted := errNoEpochInfoError
	_, err = bs.GetValidatorActiveSetChanges(
		ctx,
		&zondpb.GetValidatorActiveSetChangesRequest{
			QueryFilter: &zondpb.GetValidatorActiveSetChangesRequest_Epoch{
				Epoch: slots.ToEpoch(bs.GenesisTimeFetcher.CurrentSlot()) + 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidatorBalances_CannotRequestFutureEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	bs := &Server{
		BeaconDB: beaconDB,
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{},
	}

	wanted := errNoEpochInfoError
	_, err = bs.ListValidatorBalances(
		ctx,
		&zondpb.ListValidatorBalancesRequest{
			QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{
				Epoch: slots.ToEpoch(bs.GenesisTimeFetcher.CurrentSlot()) + 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidatorBalances_NoResults(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB, doublylinkedtree.New()),
	}

	headState, err := util.NewBeaconState()
	require.NoError(t, err)
	b := util.NewBeaconBlock()
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs.ReplayerBuilder = mockstategen.NewMockReplayerBuilder(mockstategen.WithMockState(headState))

	wanted := &zondpb.ValidatorBalances{
		Balances:      make([]*zondpb.ValidatorBalances_Balance, 0),
		TotalSize:     int32(0),
		NextPageToken: strconv.Itoa(0),
	}
	res, err := bs.ListValidatorBalances(
		ctx,
		&zondpb.ListValidatorBalancesRequest{
			QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{
				Epoch: 0,
			},
		},
	)
	require.NoError(t, err)
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
}

func TestServer_ListValidatorBalances_DefaultResponse_NoArchive(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	numItems := 100
	validators := make([]*zondpb.Validator, numItems)
	balances := make([]uint64, numItems)
	balancesResponse := make([]*zondpb.ValidatorBalances_Balance, numItems)
	for i := 0; i < numItems; i++ {
		validators[i] = &zondpb.Validator{
			PublicKey:             pubKey(uint64(i)),
			WithdrawalCredentials: make([]byte, 32),
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
		balancesResponse[i] = &zondpb.ValidatorBalances_Balance{
			PublicKey: pubKey(uint64(i)),
			Index:     primitives.ValidatorIndex(i),
			Balance:   params.BeaconConfig().MaxEffectiveBalance,
			Status:    "EXITED",
		}
	}
	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetBalances(balances))
	b := util.NewBeaconBlock()
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))
	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB, doublylinkedtree.New()),
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		ReplayerBuilder: mockstategen.NewMockReplayerBuilder(mockstategen.WithMockState(st)),
	}
	res, err := bs.ListValidatorBalances(
		ctx,
		&zondpb.ListValidatorBalancesRequest{
			QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0},
		},
	)
	require.NoError(t, err)
	assert.DeepEqual(t, balancesResponse, res.Balances)
}

func TestServer_ListValidatorBalances_PaginationOutOfRange(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	_, _, headState := setupValidators(t, beaconDB, 100)
	b := util.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB, doublylinkedtree.New()),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		ReplayerBuilder: mockstategen.NewMockReplayerBuilder(mockstategen.WithMockState(headState)),
	}

	wanted := fmt.Sprintf("page start %d >= list %d", 200, len(headState.Balances()))
	_, err = bs.ListValidatorBalances(context.Background(), &zondpb.ListValidatorBalancesRequest{
		PageToken:   strconv.Itoa(2),
		PageSize:    100,
		QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0},
	})
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidatorBalances_ExceedsMaxPageSize(t *testing.T) {
	bs := &Server{}
	exceedsMax := int32(cmd.Get().MaxRPCPageSize + 1)

	wanted := fmt.Sprintf(
		"Requested page size %d can not be greater than max size %d",
		exceedsMax,
		cmd.Get().MaxRPCPageSize,
	)
	req := &zondpb.ListValidatorBalancesRequest{PageSize: exceedsMax}
	_, err := bs.ListValidatorBalances(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func pubKey(i uint64) []byte {
	pubKey := make([]byte, params.BeaconConfig().DilithiumPubkeyLength)
	binary.LittleEndian.PutUint64(pubKey, i)
	return pubKey
}

func TestServer_ListValidatorBalances_Pagination_Default(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	_, _, headState := setupValidators(t, beaconDB, 100)
	b := util.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB, doublylinkedtree.New()),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		ReplayerBuilder: mockstategen.NewMockReplayerBuilder(mockstategen.WithMockState(headState)),
	}

	tests := []struct {
		req *zondpb.ListValidatorBalancesRequest
		res *zondpb.ValidatorBalances
	}{
		{req: &zondpb.ListValidatorBalancesRequest{PublicKeys: [][]byte{pubKey(99)}, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{Index: 99, PublicKey: pubKey(99), Balance: 99, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     1,
			},
		},
		{req: &zondpb.ListValidatorBalancesRequest{Indices: []primitives.ValidatorIndex{1, 2, 3}, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{Index: 1, PublicKey: pubKey(1), Balance: 1, Status: "EXITED"},
					{Index: 2, PublicKey: pubKey(2), Balance: 2, Status: "EXITED"},
					{Index: 3, PublicKey: pubKey(3), Balance: 3, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     3,
			},
		},
		{req: &zondpb.ListValidatorBalancesRequest{PublicKeys: [][]byte{pubKey(10), pubKey(11), pubKey(12)}, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{Index: 10, PublicKey: pubKey(10), Balance: 10, Status: "EXITED"},
					{Index: 11, PublicKey: pubKey(11), Balance: 11, Status: "EXITED"},
					{Index: 12, PublicKey: pubKey(12), Balance: 12, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     3,
			}},
		{req: &zondpb.ListValidatorBalancesRequest{PublicKeys: [][]byte{pubKey(2), pubKey(3)}, Indices: []primitives.ValidatorIndex{3, 4}, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}}, // Duplication
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{Index: 2, PublicKey: pubKey(2), Balance: 2, Status: "EXITED"},
					{Index: 3, PublicKey: pubKey(3), Balance: 3, Status: "EXITED"},
					{Index: 4, PublicKey: pubKey(4), Balance: 4, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     3,
			}},
		{req: &zondpb.ListValidatorBalancesRequest{PublicKeys: [][]byte{{}}, Indices: []primitives.ValidatorIndex{3, 4}, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}}, // Public key has a blank value
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{Index: 3, PublicKey: pubKey(3), Balance: 3, Status: "EXITED"},
					{Index: 4, PublicKey: pubKey(4), Balance: 4, Status: "EXITED"},
				},
				NextPageToken: "",
				TotalSize:     2,
			}},
	}
	for _, test := range tests {
		res, err := bs.ListValidatorBalances(context.Background(), test.req)
		require.NoError(t, err)
		if !proto.Equal(res, test.res) {
			t.Errorf("Expected %v, received %v", test.res, res)
		}
	}
}

func TestServer_ListValidatorBalances_Pagination_CustomPageSizes(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	count := 1000
	_, _, headState := setupValidators(t, beaconDB, count)
	b := util.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB, doublylinkedtree.New()),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		ReplayerBuilder: mockstategen.NewMockReplayerBuilder(mockstategen.WithMockState(headState)),
	}

	tests := []struct {
		req *zondpb.ListValidatorBalancesRequest
		res *zondpb.ValidatorBalances
	}{
		{req: &zondpb.ListValidatorBalancesRequest{PageToken: strconv.Itoa(1), PageSize: 3, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{PublicKey: pubKey(3), Index: 3, Balance: uint64(3), Status: "EXITED"},
					{PublicKey: pubKey(4), Index: 4, Balance: uint64(4), Status: "EXITED"},
					{PublicKey: pubKey(5), Index: 5, Balance: uint64(5), Status: "EXITED"}},
				NextPageToken: strconv.Itoa(2),
				TotalSize:     int32(count)}},
		{req: &zondpb.ListValidatorBalancesRequest{PageToken: strconv.Itoa(10), PageSize: 5, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{PublicKey: pubKey(50), Index: 50, Balance: uint64(50), Status: "EXITED"},
					{PublicKey: pubKey(51), Index: 51, Balance: uint64(51), Status: "EXITED"},
					{PublicKey: pubKey(52), Index: 52, Balance: uint64(52), Status: "EXITED"},
					{PublicKey: pubKey(53), Index: 53, Balance: uint64(53), Status: "EXITED"},
					{PublicKey: pubKey(54), Index: 54, Balance: uint64(54), Status: "EXITED"}},
				NextPageToken: strconv.Itoa(11),
				TotalSize:     int32(count)}},
		{req: &zondpb.ListValidatorBalancesRequest{PageToken: strconv.Itoa(33), PageSize: 3, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{PublicKey: pubKey(99), Index: 99, Balance: uint64(99), Status: "EXITED"},
					{PublicKey: pubKey(100), Index: 100, Balance: uint64(100), Status: "EXITED"},
					{PublicKey: pubKey(101), Index: 101, Balance: uint64(101), Status: "EXITED"},
				},
				NextPageToken: "34",
				TotalSize:     int32(count)}},
		{req: &zondpb.ListValidatorBalancesRequest{PageSize: 2, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}},
			res: &zondpb.ValidatorBalances{
				Balances: []*zondpb.ValidatorBalances_Balance{
					{PublicKey: pubKey(0), Index: 0, Balance: uint64(0), Status: "EXITED"},
					{PublicKey: pubKey(1), Index: 1, Balance: uint64(1), Status: "EXITED"}},
				NextPageToken: strconv.Itoa(1),
				TotalSize:     int32(count)}},
	}
	for _, test := range tests {
		res, err := bs.ListValidatorBalances(context.Background(), test.req)
		require.NoError(t, err)
		if !proto.Equal(res, test.res) {
			t.Errorf("Expected %v, received %v", test.res, res)
		}
	}
}

func TestServer_ListValidatorBalances_OutOfRange(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	_, _, headState := setupValidators(t, beaconDB, 1)
	b := util.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB, doublylinkedtree.New()),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		ReplayerBuilder: mockstategen.NewMockReplayerBuilder(mockstategen.WithMockState(headState)),
	}

	req := &zondpb.ListValidatorBalancesRequest{Indices: []primitives.ValidatorIndex{primitives.ValidatorIndex(1)}, QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0}}
	wanted := "Validator index 1 >= balance list 1"
	_, err = bs.ListValidatorBalances(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_CannotRequestFutureEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	bs := &Server{
		BeaconDB: beaconDB,
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		HeadFetcher: &mock.ChainService{
			State: st,
		},
	}

	wanted := errNoEpochInfoError
	_, err = bs.ListValidators(
		ctx,
		&zondpb.ListValidatorsRequest{
			QueryFilter: &zondpb.ListValidatorsRequest_Epoch{
				Epoch: 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_reqStateIsNil(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	secondsPerEpoch := params.BeaconConfig().SecondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	bs := &Server{
		BeaconDB: beaconDB,
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 1.
			Genesis: time.Now().Add(time.Duration(-1*int64(secondsPerEpoch)) * time.Second),
		},
		HeadFetcher: &mock.ChainService{
			State: nil,
		},
		StateGen: &mockstategen.MockStateManager{
			StatesBySlot: map[primitives.Slot]state.BeaconState{
				0: nil,
			},
		},
	}
	// request uses HeadFetcher to get reqState.
	req1 := &zondpb.ListValidatorsRequest{PageToken: strconv.Itoa(1), PageSize: 100}
	wanted := "Requested state is nil"
	_, err := bs.ListValidators(context.Background(), req1)
	assert.ErrorContains(t, wanted, err)

	// request uses StateGen to get reqState.
	req2 := &zondpb.ListValidatorsRequest{
		QueryFilter: &zondpb.ListValidatorsRequest_Genesis{},
		PageToken:   strconv.Itoa(1),
		PageSize:    100,
	}
	_, err = bs.ListValidators(context.Background(), req2)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_NoResults(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(0))
	gRoot := [32]byte{'g'}
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))
	bs := &Server{
		BeaconDB: beaconDB,
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}
	wanted := &zondpb.Validators{
		ValidatorList: make([]*zondpb.Validators_ValidatorContainer, 0),
		TotalSize:     int32(0),
		NextPageToken: strconv.Itoa(0),
	}
	res, err := bs.ListValidators(
		ctx,
		&zondpb.ListValidatorsRequest{
			QueryFilter: &zondpb.ListValidatorsRequest_Epoch{
				Epoch: 0,
			},
		},
	)
	require.NoError(t, err)
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted %v, received %v", wanted, res)
	}
}

func TestServer_ListValidators_OnlyActiveValidators(t *testing.T) {
	ctx := context.Background()
	beaconDB := dbTest.SetupDB(t)
	count := 100
	balances := make([]uint64, count)
	validators := make([]*zondpb.Validator, count)
	activeValidators := make([]*zondpb.Validators_ValidatorContainer, 0)
	for i := 0; i < count; i++ {
		pubKey := pubKey(uint64(i))
		balances[i] = params.BeaconConfig().MaxEffectiveBalance

		// We mark even validators as active, and odd validators as inactive.
		if i%2 == 0 {
			val := &zondpb.Validator{
				PublicKey:             pubKey,
				WithdrawalCredentials: make([]byte, 32),
				ActivationEpoch:       0,
				ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			}
			validators[i] = val
			activeValidators = append(activeValidators, &zondpb.Validators_ValidatorContainer{
				Index:     primitives.ValidatorIndex(i),
				Validator: val,
			})
		} else {
			validators[i] = &zondpb.Validator{
				PublicKey:             pubKey,
				WithdrawalCredentials: make([]byte, 32),
				ActivationEpoch:       0,
				ExitEpoch:             0,
			}
		}
	}
	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetBalances(balances))

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}

	b := util.NewBeaconBlock()
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))

	received, err := bs.ListValidators(ctx, &zondpb.ListValidatorsRequest{
		Active: true,
	})
	require.NoError(t, err)
	assert.DeepSSZEqual(t, activeValidators, received.ValidatorList)
}

func TestServer_ListValidators_InactiveInTheMiddle(t *testing.T) {
	ctx := context.Background()
	beaconDB := dbTest.SetupDB(t)
	count := 100
	balances := make([]uint64, count)
	validators := make([]*zondpb.Validator, count)
	activeValidators := make([]*zondpb.Validators_ValidatorContainer, 0)
	for i := 0; i < count; i++ {
		pubKey := pubKey(uint64(i))
		balances[i] = params.BeaconConfig().MaxEffectiveBalance

		// We mark even validators as active, and odd validators as inactive.
		if i%2 == 0 {
			val := &zondpb.Validator{
				PublicKey:             pubKey,
				WithdrawalCredentials: make([]byte, 32),
				ActivationEpoch:       0,
				ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			}
			validators[i] = val
			activeValidators = append(activeValidators, &zondpb.Validators_ValidatorContainer{
				Index:     primitives.ValidatorIndex(i),
				Validator: val,
			})
		} else {
			validators[i] = &zondpb.Validator{
				PublicKey:             pubKey,
				WithdrawalCredentials: make([]byte, 32),
				ActivationEpoch:       0,
				ExitEpoch:             0,
			}
		}
	}

	// Set first validator to be inactive.
	validators[0].ActivationEpoch = params.BeaconConfig().FarFutureEpoch
	activeValidators[0].Validator.ActivationEpoch = params.BeaconConfig().FarFutureEpoch

	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetBalances(balances))

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}

	b := util.NewBeaconBlock()
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))

	received, err := bs.ListValidators(ctx, &zondpb.ListValidatorsRequest{
		Active: true,
	})
	require.NoError(t, err)

	require.Equal(t, count/2-1, len(received.ValidatorList))
	require.Equal(t, count/2-1, int(received.TotalSize))
}

func TestServer_ListValidatorBalances_UnknownValidatorInResponse(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	_, _, headState := setupValidators(t, beaconDB, 4)
	b := util.NewBeaconBlock()
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB, doublylinkedtree.New()),
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		ReplayerBuilder: mockstategen.NewMockReplayerBuilder(mockstategen.WithMockState(headState)),
	}

	nonExistentPubKey := [32]byte{8}
	req := &zondpb.ListValidatorBalancesRequest{
		PublicKeys: [][]byte{
			pubKey(1),
			pubKey(2),
			nonExistentPubKey[:],
		},
		QueryFilter: &zondpb.ListValidatorBalancesRequest_Epoch{Epoch: 0},
	}

	wanted := &zondpb.ValidatorBalances{
		Balances: []*zondpb.ValidatorBalances_Balance{
			{Status: "UNKNOWN"},
			{Index: 1, PublicKey: pubKey(1), Balance: 1, Status: "EXITED"},
			{Index: 2, PublicKey: pubKey(2), Balance: 2, Status: "EXITED"},
		},
		NextPageToken: "",
		TotalSize:     3,
	}
	res, err := bs.ListValidatorBalances(context.Background(), req)
	require.NoError(t, err)
	if !proto.Equal(res, wanted) {
		t.Errorf("Expected %v, received %v", wanted, res)
	}
}

func TestServer_ListValidators_NoPagination(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	validators, _, headState := setupValidators(t, beaconDB, 100)
	want := make([]*zondpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &zondpb.Validators_ValidatorContainer{
			Index:     primitives.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &zondpb.Checkpoint{
				Epoch: 0,
			},
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}

	received, err := bs.ListValidators(context.Background(), &zondpb.ListValidatorsRequest{})
	require.NoError(t, err)
	assert.DeepSSZEqual(t, want, received.ValidatorList, "Incorrect respond of validators")
}

func TestServer_ListValidators_StategenNotUsed(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	validators, _, headState := setupValidators(t, beaconDB, 100)
	want := make([]*zondpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &zondpb.Validators_ValidatorContainer{
			Index:     primitives.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
	}

	received, err := bs.ListValidators(context.Background(), &zondpb.ListValidatorsRequest{})
	require.NoError(t, err)
	assert.DeepEqual(t, want, received.ValidatorList, "Incorrect respond of validators")
}

func TestServer_ListValidators_IndicesPubKeys(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	validators, _, headState := setupValidators(t, beaconDB, 100)
	indicesWanted := []primitives.ValidatorIndex{2, 7, 11, 17}
	pubkeyIndicesWanted := []primitives.ValidatorIndex{3, 5, 9, 15}
	allIndicesWanted := append(indicesWanted, pubkeyIndicesWanted...)
	want := make([]*zondpb.Validators_ValidatorContainer, len(allIndicesWanted))
	for i, idx := range allIndicesWanted {
		want[i] = &zondpb.Validators_ValidatorContainer{
			Index:     idx,
			Validator: validators[idx],
		}
	}
	sort.Slice(want, func(i int, j int) bool {
		return want[i].Index < want[j].Index
	})

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &zondpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}

	pubKeysWanted := make([][]byte, len(pubkeyIndicesWanted))
	for i, indice := range pubkeyIndicesWanted {
		pubKeysWanted[i] = pubKey(uint64(indice))
	}
	req := &zondpb.ListValidatorsRequest{
		Indices:    indicesWanted,
		PublicKeys: pubKeysWanted,
	}
	received, err := bs.ListValidators(context.Background(), req)
	require.NoError(t, err)
	assert.DeepEqual(t, want, received.ValidatorList, "Incorrect respond of validators")
}

func TestServer_ListValidators_Pagination(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	count := 100
	_, _, headState := setupValidators(t, beaconDB, count)

	bs := &Server{
		BeaconDB: beaconDB,
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &zondpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}

	tests := []struct {
		req *zondpb.ListValidatorsRequest
		res *zondpb.Validators
	}{
		{req: &zondpb.ListValidatorsRequest{PageToken: strconv.Itoa(1), PageSize: 3},
			res: &zondpb.Validators{
				ValidatorList: []*zondpb.Validators_ValidatorContainer{
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(3),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 3,
					},
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(4),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 4,
					},
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(5),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 5,
					},
				},
				NextPageToken: strconv.Itoa(2),
				TotalSize:     int32(count)}},
		{req: &zondpb.ListValidatorsRequest{PageToken: strconv.Itoa(10), PageSize: 5},
			res: &zondpb.Validators{
				ValidatorList: []*zondpb.Validators_ValidatorContainer{
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(50),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 50,
					},
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(51),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 51,
					},
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(52),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 52,
					},
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(53),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 53,
					},
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(54),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 54,
					},
				},
				NextPageToken: strconv.Itoa(11),
				TotalSize:     int32(count)}},
		{req: &zondpb.ListValidatorsRequest{PageToken: strconv.Itoa(33), PageSize: 3},
			res: &zondpb.Validators{
				ValidatorList: []*zondpb.Validators_ValidatorContainer{
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(99),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 99,
					},
				},
				NextPageToken: "",
				TotalSize:     int32(count)}},
		{req: &zondpb.ListValidatorsRequest{PageSize: 2},
			res: &zondpb.Validators{
				ValidatorList: []*zondpb.Validators_ValidatorContainer{
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(0),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 0,
					},
					{
						Validator: &zondpb.Validator{
							PublicKey:             pubKey(1),
							WithdrawalCredentials: make([]byte, 32),
						},
						Index: 1,
					},
				},
				NextPageToken: strconv.Itoa(1),
				TotalSize:     int32(count)}},
	}
	for _, test := range tests {
		res, err := bs.ListValidators(context.Background(), test.req)
		require.NoError(t, err)
		if !proto.Equal(res, test.res) {
			t.Errorf("Incorrect validator response, wanted %v, received %v", test.res, res)
		}
	}
}

func TestServer_ListValidators_PaginationOutOfRange(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	count := 1
	validators, _, headState := setupValidators(t, beaconDB, count)

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &zondpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}

	req := &zondpb.ListValidatorsRequest{PageToken: strconv.Itoa(1), PageSize: 100}
	wanted := fmt.Sprintf("page start %d >= list %d", req.PageSize, len(validators))
	_, err := bs.ListValidators(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_ExceedsMaxPageSize(t *testing.T) {
	bs := &Server{}
	exceedsMax := int32(cmd.Get().MaxRPCPageSize + 1)

	wanted := fmt.Sprintf("Requested page size %d can not be greater than max size %d", exceedsMax, cmd.Get().MaxRPCPageSize)
	req := &zondpb.ListValidatorsRequest{PageToken: strconv.Itoa(0), PageSize: exceedsMax}
	_, err := bs.ListValidators(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_ListValidators_DefaultPageSize(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	validators, _, headState := setupValidators(t, beaconDB, 1000)
	want := make([]*zondpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &zondpb.Validators_ValidatorContainer{
			Index:     primitives.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &zondpb.Checkpoint{
				Epoch: 0,
			},
		},
		GenesisTimeFetcher: &mock.ChainService{
			// We are in epoch 0.
			Genesis: time.Now(),
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}

	req := &zondpb.ListValidatorsRequest{}
	res, err := bs.ListValidators(context.Background(), req)
	require.NoError(t, err)

	i := 0
	j := params.BeaconConfig().DefaultPageSize
	assert.DeepEqual(t, want[i:j], res.ValidatorList, "Incorrect respond of validators")
}

func TestServer_ListValidators_FromOldEpoch(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.BeaconConfig())
	transition.SkipSlotCache.Disable()

	ctx := context.Background()
	slot := primitives.Slot(0)
	epochs := 10
	numVals := uint64(10)

	beaconDB := dbTest.SetupDB(t)
	b := util.NewBeaconBlock()
	b.Block.Slot = slot
	util.SaveBlock(t, ctx, beaconDB, b)

	r, err := b.Block.HashTreeRoot()
	require.NoError(t, err)

	st, _ := util.DeterministicGenesisState(t, numVals)
	require.NoError(t, st.SetSlot(slot))
	require.Equal(t, int(numVals), len(st.Validators()))

	require.NoError(t, beaconDB.SaveState(ctx, st, r))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, r))

	secondsPerEpoch := params.BeaconConfig().SecondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: time.Now().Add(time.Duration(-1*int64(uint64(epochs)*secondsPerEpoch)) * time.Second),
		},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	req := &zondpb.ListValidatorsRequest{
		QueryFilter: &zondpb.ListValidatorsRequest_Genesis{
			Genesis: true,
		},
	}
	res, err := bs.ListValidators(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, epochs, len(res.ValidatorList))

	vals := st.Validators()
	want := make([]*zondpb.Validators_ValidatorContainer, 0)
	for i, v := range vals {
		want = append(want, &zondpb.Validators_ValidatorContainer{
			Index:     primitives.ValidatorIndex(i),
			Validator: v,
		})
	}
	req = &zondpb.ListValidatorsRequest{
		QueryFilter: &zondpb.ListValidatorsRequest_Epoch{
			Epoch: 10,
		},
	}
	res, err = bs.ListValidators(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, len(want), len(res.ValidatorList), "incorrect number of validators")
	assert.DeepSSZEqual(t, want, res.ValidatorList, "mismatch in validator values")
}

func TestServer_ListValidators_ProcessHeadStateSlots(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MinimalSpecConfig())

	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	headSlot := primitives.Slot(32)
	numValidators := params.BeaconConfig().MinGenesisActiveValidatorCount
	validators := make([]*zondpb.Validator, numValidators)
	balances := make([]uint64, numValidators)
	for i := uint64(0); i < numValidators; i++ {
		validators[i] = &zondpb.Validator{
			ActivationEpoch:       0,
			PublicKey:             make([]byte, 2592),
			WithdrawalCredentials: make([]byte, 32),
			EffectiveBalance:      params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}
	want := make([]*zondpb.Validators_ValidatorContainer, len(validators))
	for i := 0; i < len(validators); i++ {
		want[i] = &zondpb.Validators_ValidatorContainer{
			Index:     primitives.ValidatorIndex(i),
			Validator: validators[i],
		}
	}

	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetSlot(headSlot))
	require.NoError(t, st.SetValidators(validators))
	require.NoError(t, st.SetBalances(balances))
	b := util.NewBeaconBlock()
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveState(ctx, st, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	secondsPerEpoch := params.BeaconConfig().SecondsPerSlot * uint64(params.BeaconConfig().SlotsPerEpoch)
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: time.Now().Add(time.Duration(-1*int64(secondsPerEpoch)) * time.Second),
		},
		StateGen: stategen.New(beaconDB, doublylinkedtree.New()),
	}

	req := &zondpb.ListValidatorsRequest{
		QueryFilter: &zondpb.ListValidatorsRequest_Epoch{
			Epoch: 1,
		},
	}
	res, err := bs.ListValidators(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, len(want), len(res.ValidatorList), "Incorrect number of validators")
	for i := 0; i < len(res.ValidatorList); i++ {
		assert.DeepEqual(t, want[i], res.ValidatorList[i])
	}
}

func TestServer_GetValidator(t *testing.T) {
	count := primitives.Epoch(30)
	validators := make([]*zondpb.Validator, count)
	for i := primitives.Epoch(0); i < count; i++ {
		validators[i] = &zondpb.Validator{
			ActivationEpoch:       i,
			PublicKey:             pubKey(uint64(i)),
			WithdrawalCredentials: make([]byte, 32),
		}
	}

	st, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, st.SetValidators(validators))

	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: st,
		},
	}

	tests := []struct {
		req       *zondpb.GetValidatorRequest
		res       *zondpb.Validator
		wantedErr string
	}{
		{
			req: &zondpb.GetValidatorRequest{
				QueryFilter: &zondpb.GetValidatorRequest_Index{
					Index: 0,
				},
			},
			res: validators[0],
		},
		{
			req: &zondpb.GetValidatorRequest{
				QueryFilter: &zondpb.GetValidatorRequest_Index{
					Index: primitives.ValidatorIndex(count - 1),
				},
			},
			res: validators[count-1],
		},
		{
			req: &zondpb.GetValidatorRequest{
				QueryFilter: &zondpb.GetValidatorRequest_PublicKey{
					PublicKey: pubKey(5),
				},
			},
			res: validators[5],
		},
		{
			req: &zondpb.GetValidatorRequest{
				QueryFilter: &zondpb.GetValidatorRequest_PublicKey{
					PublicKey: []byte("bad-keyxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"),
				},
			},
			res:       nil,
			wantedErr: "No validator matched filter criteria",
		},
		{
			req: &zondpb.GetValidatorRequest{
				QueryFilter: &zondpb.GetValidatorRequest_Index{
					Index: primitives.ValidatorIndex(len(validators)),
				},
			},
			res:       nil,
			wantedErr: fmt.Sprintf("there are only %d validators", len(validators)),
		},
	}

	for _, test := range tests {
		res, err := bs.GetValidator(context.Background(), test.req)
		if test.wantedErr != "" {
			require.ErrorContains(t, test.wantedErr, err)
		} else {
			require.NoError(t, err)
		}
		assert.DeepEqual(t, test.res, res)
	}
}

func TestServer_GetValidatorActiveSetChanges(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	validators := make([]*zondpb.Validator, 8)
	headState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(0))
	require.NoError(t, headState.SetValidators(validators))
	for i := 0; i < len(validators); i++ {
		activationEpoch := params.BeaconConfig().FarFutureEpoch
		withdrawableEpoch := params.BeaconConfig().FarFutureEpoch
		exitEpoch := params.BeaconConfig().FarFutureEpoch
		slashed := false
		balance := params.BeaconConfig().MaxEffectiveBalance
		// Mark indices divisible by two as activated.
		if i%2 == 0 {
			activationEpoch = 0
		} else if i%3 == 0 {
			// Mark indices divisible by 3 as slashed.
			withdrawableEpoch = params.BeaconConfig().EpochsPerSlashingsVector
			slashed = true
		} else if i%5 == 0 {
			// Mark indices divisible by 5 as exited.
			exitEpoch = 0
			withdrawableEpoch = params.BeaconConfig().MinValidatorWithdrawabilityDelay
		} else if i%7 == 0 {
			// Mark indices divisible by 7 as ejected.
			exitEpoch = 0
			withdrawableEpoch = params.BeaconConfig().MinValidatorWithdrawabilityDelay
			balance = params.BeaconConfig().EjectionBalance
		}
		err := headState.UpdateValidatorAtIndex(primitives.ValidatorIndex(i), &zondpb.Validator{
			ActivationEpoch:       activationEpoch,
			PublicKey:             pubKey(uint64(i)),
			EffectiveBalance:      balance,
			WithdrawalCredentials: make([]byte, 32),
			WithdrawableEpoch:     withdrawableEpoch,
			Slashed:               slashed,
			ExitEpoch:             exitEpoch,
		})
		require.NoError(t, err)
	}
	b := util.NewBeaconBlock()
	util.SaveBlock(t, ctx, beaconDB, b)

	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, gRoot))

	bs := &Server{
		FinalizationFetcher: &mock.ChainService{
			FinalizedCheckPoint: &zondpb.Checkpoint{Epoch: 0, Root: make([]byte, fieldparams.RootLength)},
		},
		GenesisTimeFetcher: &mock.ChainService{},
	}
	addDefaultReplayerBuilder(bs, beaconDB)
	res, err := bs.GetValidatorActiveSetChanges(ctx, &zondpb.GetValidatorActiveSetChangesRequest{
		QueryFilter: &zondpb.GetValidatorActiveSetChangesRequest_Genesis{Genesis: true},
	})
	require.NoError(t, err)
	wantedActive := [][]byte{
		pubKey(0),
		pubKey(2),
		pubKey(4),
		pubKey(6),
	}
	wantedActiveIndices := []primitives.ValidatorIndex{0, 2, 4, 6}
	wantedExited := [][]byte{
		pubKey(5),
	}
	wantedExitedIndices := []primitives.ValidatorIndex{5}
	wantedSlashed := [][]byte{
		pubKey(3),
	}
	wantedSlashedIndices := []primitives.ValidatorIndex{3}
	wantedEjected := [][]byte{
		pubKey(7),
	}
	wantedEjectedIndices := []primitives.ValidatorIndex{7}
	wanted := &zondpb.ActiveSetChanges{
		Epoch:               0,
		ActivatedPublicKeys: wantedActive,
		ActivatedIndices:    wantedActiveIndices,
		ExitedPublicKeys:    wantedExited,
		ExitedIndices:       wantedExitedIndices,
		SlashedPublicKeys:   wantedSlashed,
		SlashedIndices:      wantedSlashedIndices,
		EjectedPublicKeys:   wantedEjected,
		EjectedIndices:      wantedEjectedIndices,
	}
	if !proto.Equal(wanted, res) {
		t.Errorf("Wanted \n%v, received \n%v", wanted, res)
	}
}

func TestServer_GetValidatorParticipation_CannotRequestFutureEpoch(t *testing.T) {
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	headState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(0))
	bs := &Server{
		BeaconDB: beaconDB,
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
		GenesisTimeFetcher: &mock.ChainService{},
		StateGen:           stategen.New(beaconDB, doublylinkedtree.New()),
	}

	wanted := "Cannot retrieve information about an epoch"
	_, err = bs.GetValidatorParticipation(
		ctx,
		&zondpb.GetValidatorParticipationRequest{
			QueryFilter: &zondpb.GetValidatorParticipationRequest_Epoch{
				Epoch: slots.ToEpoch(bs.GenesisTimeFetcher.CurrentSlot()) + 1,
			},
		},
	)
	assert.ErrorContains(t, wanted, err)
}

// FIX(rgeraldes24) - this test can be removed since we have the new version with bits bellow
/*
func TestServer_GetValidatorParticipation_CurrentAndPrevEpoch(t *testing.T) {
	helpers.ClearCache()
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	validatorCount := uint64(32)

	validators := make([]*zondpb.Validator, validatorCount)
	balances := make([]uint64, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &zondpb.Validator{
			PublicKey:             bytesutil.ToBytes(uint64(i), 2592),
			WithdrawalCredentials: make([]byte, 32),
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance:      params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	atts := []*zondpb.PendingAttestation{{
		Data:              util.HydrateAttestationData(&zondpb.AttestationData{}),
		InclusionDelay:    1,
		ParticipationBits: bitfield.NewBitlist(validatorCount / uint64(params.BeaconConfig().SlotsPerEpoch)),
	}}
	headState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(8))
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetBalances(balances))
	require.NoError(t, headState.AppendCurrentEpochAttestations(atts[0]))
	require.NoError(t, headState.AppendPreviousEpochAttestations(atts[0]))

	b := util.NewBeaconBlock()
	b.Block.Slot = 8
	util.SaveBlock(t, ctx, beaconDB, b)
	bRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, beaconDB.SaveStateSummary(ctx, &zondpb.StateSummary{Root: bRoot[:]}))
	require.NoError(t, beaconDB.SaveStateSummary(ctx, &zondpb.StateSummary{Root: params.BeaconConfig().ZeroHash[:]}))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, bRoot))
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveState(ctx, headState, bRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, params.BeaconConfig().ZeroHash))

	m := &mock.ChainService{State: headState}
	offset := int64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		BeaconDB:    beaconDB,
		HeadFetcher: m,
		StateGen:    stategen.New(beaconDB, doublylinkedtree.New()),
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: qrysmTime.Now().Add(time.Duration(-1*offset) * time.Second),
		},
		CanonicalFetcher: &mock.ChainService{
			CanonicalRoots: map[[32]byte]bool{
				bRoot: true,
			},
		},
		FinalizationFetcher: &mock.ChainService{FinalizedCheckPoint: &zondpb.Checkpoint{Epoch: 100}},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	res, err := bs.GetValidatorParticipation(ctx, &zondpb.GetValidatorParticipationRequest{QueryFilter: &zondpb.GetValidatorParticipationRequest_Epoch{Epoch: 1}})
	require.NoError(t, err)

	wanted := &zondpb.ValidatorParticipation{
		GlobalParticipationRate:          float32(params.BeaconConfig().EffectiveBalanceIncrement) / float32(validatorCount*params.BeaconConfig().MaxEffectiveBalance),
		VotedEther:                       params.BeaconConfig().EffectiveBalanceIncrement,
		EligibleEther:                    validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochActiveGwei:           validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochAttestingGwei:        params.BeaconConfig().EffectiveBalanceIncrement,
		CurrentEpochTargetAttestingGwei:  params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochActiveGwei:          validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochAttestingGwei:       params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochTargetAttestingGwei: params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochHeadAttestingGwei:   params.BeaconConfig().EffectiveBalanceIncrement,
	}
	assert.DeepEqual(t, true, res.Finalized, "Incorrect validator participation respond")
	assert.DeepEqual(t, wanted, res.Participation, "Incorrect validator participation respond")
}
*/

/*
func TestServer_GetValidatorParticipation_OrphanedUntilGenesis(t *testing.T) {
	helpers.ClearCache()
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.BeaconConfig())

	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()
	validatorCount := uint64(100)

	validators := make([]*zondpb.Validator, validatorCount)
	balances := make([]uint64, validatorCount)
	for i := 0; i < len(validators); i++ {
		validators[i] = &zondpb.Validator{
			PublicKey:             bytesutil.ToBytes(uint64(i), 2592),
			WithdrawalCredentials: make([]byte, 32),
			ExitEpoch:             params.BeaconConfig().FarFutureEpoch,
			EffectiveBalance:      params.BeaconConfig().MaxEffectiveBalance,
		}
		balances[i] = params.BeaconConfig().MaxEffectiveBalance
	}

	atts := []*zondpb.PendingAttestation{{
		Data:              util.HydrateAttestationData(&zondpb.AttestationData{}),
		InclusionDelay:    1,
		ParticipationBits: bitfield.NewBitlist((validatorCount / 3) / uint64(params.BeaconConfig().SlotsPerEpoch)),
	}}
	headState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(0))
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetBalances(balances))
	require.NoError(t, headState.AppendCurrentEpochAttestations(atts[0]))
	require.NoError(t, headState.AppendPreviousEpochAttestations(atts[0]))

	b := util.NewBeaconBlock()
	util.SaveBlock(t, ctx, beaconDB, b)
	bRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, bRoot))
	require.NoError(t, err)
	require.NoError(t, beaconDB.SaveState(ctx, headState, bRoot))
	require.NoError(t, beaconDB.SaveState(ctx, headState, params.BeaconConfig().ZeroHash))

	m := &mock.ChainService{State: headState}
	offset := int64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		BeaconDB:    beaconDB,
		HeadFetcher: m,
		StateGen:    stategen.New(beaconDB, doublylinkedtree.New()),
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: qrysmTime.Now().Add(time.Duration(-1*offset) * time.Second),
		},
		CanonicalFetcher: &mock.ChainService{
			CanonicalRoots: map[[32]byte]bool{
				bRoot: true,
			},
		},
		FinalizationFetcher: &mock.ChainService{FinalizedCheckPoint: &zondpb.Checkpoint{Epoch: 100}},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	res, err := bs.GetValidatorParticipation(ctx, &zondpb.GetValidatorParticipationRequest{QueryFilter: &zondpb.GetValidatorParticipationRequest_Epoch{Epoch: 1}})
	require.NoError(t, err)

	wanted := &zondpb.ValidatorParticipation{
		GlobalParticipationRate:          float32(params.BeaconConfig().EffectiveBalanceIncrement) / float32(validatorCount*params.BeaconConfig().MaxEffectiveBalance),
		VotedEther:                       params.BeaconConfig().EffectiveBalanceIncrement,
		EligibleEther:                    validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochActiveGwei:           validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochAttestingGwei:        params.BeaconConfig().EffectiveBalanceIncrement,
		CurrentEpochTargetAttestingGwei:  params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochActiveGwei:          validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochAttestingGwei:       params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochTargetAttestingGwei: params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochHeadAttestingGwei:   params.BeaconConfig().EffectiveBalanceIncrement,
	}
	assert.DeepEqual(t, true, res.Finalized, "Incorrect validator participation respond")
	assert.DeepEqual(t, wanted, res.Participation, "Incorrect validator participation respond")
}
*/

func TestServer_GetValidatorParticipation_CurrentAndPrevEpochWithBits(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.BeaconConfig())
	transition.SkipSlotCache.Disable()

	t.Run("capella", func(t *testing.T) {
		validatorCount := uint64(32)
		genState, _ := util.DeterministicGenesisState(t, validatorCount)
		c, err := altair.NextSyncCommittee(context.Background(), genState)
		require.NoError(t, err)
		require.NoError(t, genState.SetCurrentSyncCommittee(c))

		bits := make([]byte, validatorCount)
		for i := range bits {
			bits[i] = 0xff
		}
		require.NoError(t, genState.SetCurrentParticipationBits(bits))
		require.NoError(t, genState.SetPreviousParticipationBits(bits))
		gb, err := blocks.NewSignedBeaconBlock(util.NewBeaconBlock())
		assert.NoError(t, err)
		runGetValidatorParticipationCurrentAndPrevEpoch(t, genState, gb)
	})
}

func runGetValidatorParticipationCurrentAndPrevEpoch(t *testing.T, genState state.BeaconState, gb interfaces.SignedBeaconBlock) {
	helpers.ClearCache()
	beaconDB := dbTest.SetupDB(t)

	ctx := context.Background()
	validatorCount := uint64(32)

	gsr, err := genState.HashTreeRoot(ctx)
	require.NoError(t, err)
	gb, err = blocktest.SetBlockStateRoot(gb, gsr)
	require.NoError(t, err)
	require.NoError(t, err)
	gRoot, err := gb.Block().HashTreeRoot()
	require.NoError(t, err)

	require.NoError(t, beaconDB.SaveState(ctx, genState, gRoot))
	require.NoError(t, beaconDB.SaveBlock(ctx, gb))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))

	m := &mock.ChainService{State: genState}
	offset := int64(params.BeaconConfig().SlotsPerEpoch.Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		BeaconDB:    beaconDB,
		HeadFetcher: m,
		StateGen:    stategen.New(beaconDB, doublylinkedtree.New()),
		GenesisTimeFetcher: &mock.ChainService{
			Genesis: qrysmTime.Now().Add(time.Duration(-1*offset) * time.Second),
		},
		FinalizationFetcher: &mock.ChainService{FinalizedCheckPoint: &zondpb.Checkpoint{Epoch: 100}},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	res, err := bs.GetValidatorParticipation(ctx, &zondpb.GetValidatorParticipationRequest{QueryFilter: &zondpb.GetValidatorParticipationRequest_Epoch{Epoch: 0}})
	require.NoError(t, err)

	wanted := &zondpb.ValidatorParticipation{
		GlobalParticipationRate:          1,
		VotedEther:                       validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		EligibleEther:                    validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochActiveGwei:           validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochAttestingGwei:        validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochTargetAttestingGwei:  validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochActiveGwei:          validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochAttestingGwei:       validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochTargetAttestingGwei: validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochHeadAttestingGwei:   validatorCount * params.BeaconConfig().MaxEffectiveBalance,
	}
	assert.DeepEqual(t, true, res.Finalized, "Incorrect validator participation respond")
	assert.DeepEqual(t, wanted, res.Participation, "Incorrect validator participation respond")

	res, err = bs.GetValidatorParticipation(ctx, &zondpb.GetValidatorParticipationRequest{QueryFilter: &zondpb.GetValidatorParticipationRequest_Epoch{Epoch: 1}})
	require.NoError(t, err)

	wanted = &zondpb.ValidatorParticipation{
		GlobalParticipationRate:          1,
		VotedEther:                       validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		EligibleEther:                    validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochActiveGwei:           validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		CurrentEpochAttestingGwei:        params.BeaconConfig().EffectiveBalanceIncrement, // Empty because after one epoch, current participation rotates to previous
		CurrentEpochTargetAttestingGwei:  params.BeaconConfig().EffectiveBalanceIncrement,
		PreviousEpochActiveGwei:          validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochAttestingGwei:       validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochTargetAttestingGwei: validatorCount * params.BeaconConfig().MaxEffectiveBalance,
		PreviousEpochHeadAttestingGwei:   validatorCount * params.BeaconConfig().MaxEffectiveBalance,
	}
	assert.DeepEqual(t, true, res.Finalized, "Incorrect validator participation respond")
	assert.DeepEqual(t, wanted, res.Participation, "Incorrect validator participation respond")
}

func TestGetValidatorPerformance_Syncing(t *testing.T) {
	ctx := context.Background()

	bs := &Server{
		CoreService: &core.Service{
			SyncChecker: &mockSync.Sync{IsSyncing: true},
		},
	}

	wanted := "Syncing to latest head, not ready to respond"
	_, err := bs.GetValidatorPerformance(ctx, nil)
	assert.ErrorContains(t, wanted, err)
}

func TestGetValidatorPerformance_OK(t *testing.T) {
	helpers.ClearCache()
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MinimalSpecConfig())

	ctx := context.Background()
	epoch := primitives.Epoch(1)
	headState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch+1))))
	// TODO(rgeraldes24) - remove? I think this logic belongs to < version.Altair
	/*
		atts := make([]*zondpb.PendingAttestation, 3)
		for i := 0; i < len(atts); i++ {
			atts[i] = &zondpb.PendingAttestation{
				Data: &zondpb.AttestationData{
					Target: &zondpb.Checkpoint{Root: make([]byte, 32)},
					Source: &zondpb.Checkpoint{Root: make([]byte, 32)},
				},
				ParticipationBits: bitfield.Bitlist{},
				InclusionDelay:    1,
			}
			//require.NoError(t, headState.AppendPreviousEpochAttestations(atts[i]))
		}
	*/
	defaultBal := params.BeaconConfig().MaxEffectiveBalance
	extraBal := params.BeaconConfig().MaxEffectiveBalance + params.BeaconConfig().GweiPerEth
	balances := []uint64{defaultBal, extraBal, extraBal + params.BeaconConfig().GweiPerEth}
	require.NoError(t, headState.SetBalances(balances))
	publicKey1 := bytesutil.ToBytes2592([]byte{1})
	publicKey2 := bytesutil.ToBytes2592([]byte{2})
	publicKey3 := bytesutil.ToBytes2592([]byte{3})
	validators := []*zondpb.Validator{
		{
			PublicKey:       publicKey1[:],
			ActivationEpoch: 5,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey2[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey3[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
	}
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetInactivityScores([]uint64{0, 0, 0}))
	require.NoError(t, headState.SetBalances([]uint64{100, 101, 102}))
	offset := int64(headState.Slot().Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		CoreService: &core.Service{
			HeadFetcher: &mock.ChainService{
				State: headState,
			},
			GenesisTimeFetcher: &mock.ChainService{Genesis: time.Now().Add(time.Duration(-1*offset) * time.Second)},
			SyncChecker:        &mockSync.Sync{IsSyncing: false},
		},
	}
	want := &zondpb.ValidatorPerformanceResponse{
		PublicKeys:                    [][]byte{publicKey2[:], publicKey3[:]},
		CurrentEffectiveBalances:      []uint64{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance},
		CorrectlyVotedSource:          []bool{false, false},
		CorrectlyVotedTarget:          []bool{false, false},
		CorrectlyVotedHead:            []bool{false, false},
		BalancesBeforeEpochTransition: []uint64{101, 102},
		BalancesAfterEpochTransition:  []uint64{0, 0},
		MissingValidators:             [][]byte{publicKey1[:]},
		InactivityScores:              []uint64{0, 0},
	}

	res, err := bs.GetValidatorPerformance(ctx, &zondpb.ValidatorPerformanceRequest{
		PublicKeys: [][]byte{publicKey1[:], publicKey3[:], publicKey2[:]},
	})
	require.NoError(t, err)
	if !proto.Equal(want, res) {
		t.Errorf("Wanted %v\nReceived %v", want, res)
	}
}

// TODO(rgeraldes24) - might not be needed due to the capella test below
func TestGetValidatorPerformance_Indices(t *testing.T) {
	ctx := context.Background()
	epoch := primitives.Epoch(1)
	defaultBal := params.BeaconConfig().MaxEffectiveBalance
	extraBal := params.BeaconConfig().MaxEffectiveBalance + params.BeaconConfig().GweiPerEth
	headState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch+1))))
	balances := []uint64{defaultBal, extraBal, extraBal + params.BeaconConfig().GweiPerEth}
	require.NoError(t, headState.SetBalances(balances))
	publicKey1 := bytesutil.ToBytes2592([]byte{1})
	publicKey2 := bytesutil.ToBytes2592([]byte{2})
	publicKey3 := bytesutil.ToBytes2592([]byte{3})
	validators := []*zondpb.Validator{
		{
			PublicKey:       publicKey1[:],
			ActivationEpoch: 5,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey2[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey3[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
	}
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetInactivityScores([]uint64{0, 0, 0}))
	offset := int64(headState.Slot().Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		CoreService: &core.Service{
			HeadFetcher: &mock.ChainService{
				// 10 epochs into the future.
				State: headState,
			},
			SyncChecker:        &mockSync.Sync{IsSyncing: false},
			GenesisTimeFetcher: &mock.ChainService{Genesis: time.Now().Add(time.Duration(-1*offset) * time.Second)},
		},
	}
	// TODO (rgeraldes24) old transition logic
	// c := headState.Copy()
	// vp, bp, err := precompute.New(ctx, c)
	// require.NoError(t, err)
	// vp, bp, err = precompute.ProcessAttestations(ctx, c, vp, bp)
	// require.NoError(t, err)
	// _, err = precompute.ProcessRewardsAndPenaltiesPrecompute(c, bp, vp, precompute.AttestationsDelta, precompute.ProposersDelta)
	// require.NoError(t, err)
	want := &zondpb.ValidatorPerformanceResponse{
		PublicKeys:                    [][]byte{publicKey2[:], publicKey3[:]},
		CurrentEffectiveBalances:      []uint64{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance},
		CorrectlyVotedSource:          []bool{false, false},
		CorrectlyVotedTarget:          []bool{false, false},
		CorrectlyVotedHead:            []bool{false, false},
		BalancesBeforeEpochTransition: []uint64{extraBal, extraBal + params.BeaconConfig().GweiPerEth},
		BalancesAfterEpochTransition:  []uint64{32994940360, 33994940360}, // TODO (rgeraldes24) workaround for now
		// BalancesAfterEpochTransition:  []uint64{vp[1].AfterEpochTransitionBalance, vp[2].AfterEpochTransitionBalance},
		MissingValidators: [][]byte{publicKey1[:]},
		InactivityScores:  []uint64{0, 0}, // TODO (rgeraldes24) workaround for now
	}

	res, err := bs.GetValidatorPerformance(ctx, &zondpb.ValidatorPerformanceRequest{
		Indices: []primitives.ValidatorIndex{2, 1, 0},
	})
	require.NoError(t, err)
	if !proto.Equal(want, res) {
		t.Errorf("Wanted %v\nReceived %v", want, res)
	}
}

func TestGetValidatorPerformance_IndicesPubkeys(t *testing.T) {
	ctx := context.Background()
	epoch := primitives.Epoch(1)
	defaultBal := params.BeaconConfig().MaxEffectiveBalance
	extraBal := params.BeaconConfig().MaxEffectiveBalance + params.BeaconConfig().GweiPerEth
	headState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, headState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch+1))))
	balances := []uint64{defaultBal, extraBal, extraBal + params.BeaconConfig().GweiPerEth}
	require.NoError(t, headState.SetBalances(balances))
	publicKey1 := bytesutil.ToBytes2592([]byte{1})
	publicKey2 := bytesutil.ToBytes2592([]byte{2})
	publicKey3 := bytesutil.ToBytes2592([]byte{3})
	validators := []*zondpb.Validator{
		{
			PublicKey:       publicKey1[:],
			ActivationEpoch: 5,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey2[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey3[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
	}
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetInactivityScores([]uint64{0, 0, 0}))

	offset := int64(headState.Slot().Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		CoreService: &core.Service{
			HeadFetcher: &mock.ChainService{
				// 10 epochs into the future.
				State: headState,
			},
			SyncChecker:        &mockSync.Sync{IsSyncing: false},
			GenesisTimeFetcher: &mock.ChainService{Genesis: time.Now().Add(time.Duration(-1*offset) * time.Second)},
		},
	}
	c := headState.Copy()
	vp, bp, err := altair.InitializePrecomputeValidators(ctx, c)
	require.NoError(t, err)
	vp, bp, err = altair.ProcessEpochParticipation(ctx, c, bp, vp)
	require.NoError(t, err)
	c, vp, err = altair.ProcessInactivityScores(ctx, c, vp)
	require.NoError(t, err)
	_, err = altair.ProcessRewardsAndPenaltiesPrecompute(c, bp, vp)
	require.NoError(t, err)

	want := &zondpb.ValidatorPerformanceResponse{
		PublicKeys:                    [][]byte{publicKey2[:], publicKey3[:]},
		CurrentEffectiveBalances:      []uint64{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance},
		CorrectlyVotedSource:          []bool{false, false},
		CorrectlyVotedTarget:          []bool{false, false},
		CorrectlyVotedHead:            []bool{false, false},
		BalancesBeforeEpochTransition: []uint64{extraBal, extraBal + params.BeaconConfig().GweiPerEth},
		BalancesAfterEpochTransition:  []uint64{vp[1].AfterEpochTransitionBalance, vp[2].AfterEpochTransitionBalance},
		InactivityScores:              []uint64{0, 0},
		MissingValidators:             [][]byte{publicKey1[:]},
	}
	// Index 2 and publicKey3 points to the same validator.
	// Should not return duplicates.
	res, err := bs.GetValidatorPerformance(ctx, &zondpb.ValidatorPerformanceRequest{
		PublicKeys: [][]byte{publicKey1[:], publicKey3[:]}, Indices: []primitives.ValidatorIndex{1, 2},
	})
	require.NoError(t, err)
	if !proto.Equal(want, res) {
		t.Errorf("Wanted %v\nReceived %v", want, res)
	}
}

func TestGetValidatorPerformanceCapella_OK(t *testing.T) {
	helpers.ClearCache()
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MinimalSpecConfig())

	ctx := context.Background()
	epoch := primitives.Epoch(1)
	headState, _ := util.DeterministicGenesisState(t, 32)
	require.NoError(t, headState.SetSlot(params.BeaconConfig().SlotsPerEpoch.Mul(uint64(epoch+1))))

	defaultBal := params.BeaconConfig().MaxEffectiveBalance
	extraBal := params.BeaconConfig().MaxEffectiveBalance + params.BeaconConfig().GweiPerEth
	balances := []uint64{defaultBal, extraBal, extraBal + params.BeaconConfig().GweiPerEth}
	require.NoError(t, headState.SetBalances(balances))
	publicKey1 := bytesutil.ToBytes2592([]byte{1})
	publicKey2 := bytesutil.ToBytes2592([]byte{2})
	publicKey3 := bytesutil.ToBytes2592([]byte{3})
	validators := []*zondpb.Validator{
		{
			PublicKey:       publicKey1[:],
			ActivationEpoch: 5,
			ExitEpoch:       params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey2[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
		{
			PublicKey:        publicKey3[:],
			EffectiveBalance: defaultBal,
			ActivationEpoch:  0,
			ExitEpoch:        params.BeaconConfig().FarFutureEpoch,
		},
	}
	require.NoError(t, headState.SetValidators(validators))
	require.NoError(t, headState.SetInactivityScores([]uint64{0, 0, 0}))
	require.NoError(t, headState.SetBalances([]uint64{100, 101, 102}))
	offset := int64(headState.Slot().Mul(params.BeaconConfig().SecondsPerSlot))
	bs := &Server{
		CoreService: &core.Service{
			HeadFetcher: &mock.ChainService{
				State: headState,
			},
			GenesisTimeFetcher: &mock.ChainService{Genesis: time.Now().Add(time.Duration(-1*offset) * time.Second)},
			SyncChecker:        &mockSync.Sync{IsSyncing: false},
		},
	}
	want := &zondpb.ValidatorPerformanceResponse{
		PublicKeys:                    [][]byte{publicKey2[:], publicKey3[:]},
		CurrentEffectiveBalances:      []uint64{params.BeaconConfig().MaxEffectiveBalance, params.BeaconConfig().MaxEffectiveBalance},
		CorrectlyVotedSource:          []bool{false, false},
		CorrectlyVotedTarget:          []bool{false, false},
		CorrectlyVotedHead:            []bool{false, false},
		BalancesBeforeEpochTransition: []uint64{101, 102},
		BalancesAfterEpochTransition:  []uint64{0, 0},
		MissingValidators:             [][]byte{publicKey1[:]},
		InactivityScores:              []uint64{0, 0},
	}

	// TODO(rgeraldes24)
	res, err := bs.GetValidatorPerformance(ctx, &zondpb.ValidatorPerformanceRequest{
		Indices: []primitives.ValidatorIndex{0, 2, 1},
		//PublicKeys: [][]byte{publicKey1[:], publicKey3[:], publicKey2[:]},
	})
	require.NoError(t, err)
	if !proto.Equal(want, res) {
		t.Errorf("Wanted %v\nReceived %v", want, res)
	}
}

func BenchmarkListValidatorBalances(b *testing.B) {
	b.StopTimer()
	beaconDB := dbTest.SetupDB(b)
	ctx := context.Background()

	count := 1000
	_, _, headState := setupValidators(b, beaconDB, count)
	bs := &Server{
		HeadFetcher: &mock.ChainService{
			State: headState,
		},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	req := &zondpb.ListValidatorBalancesRequest{PageSize: 100}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, err := bs.ListValidatorBalances(ctx, req)
		require.NoError(b, err)
	}
}

func setupValidators(t testing.TB, _ db.Database, count int) ([]*zondpb.Validator, []uint64, state.BeaconState) {
	balances := make([]uint64, count)
	validators := make([]*zondpb.Validator, 0, count)
	for i := 0; i < count; i++ {
		pubKey := pubKey(uint64(i))
		balances[i] = uint64(i)
		validators = append(validators, &zondpb.Validator{
			PublicKey:             pubKey,
			WithdrawalCredentials: make([]byte, 32),
		})
	}
	s, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, s.SetValidators(validators))
	require.NoError(t, s.SetBalances(balances))
	return validators, balances, s
}

func TestServer_GetIndividualVotes_RequestFutureSlot(t *testing.T) {
	ds := &Server{GenesisTimeFetcher: &mock.ChainService{}}
	req := &zondpb.IndividualVotesRequest{
		Epoch: slots.ToEpoch(ds.GenesisTimeFetcher.CurrentSlot()) + 1,
	}
	wanted := errNoEpochInfoError
	_, err := ds.GetIndividualVotes(context.Background(), req)
	assert.ErrorContains(t, wanted, err)
}

func TestServer_GetIndividualVotes_ValidatorsDontExist(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MinimalSpecConfig())

	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	var slot primitives.Slot = 0
	validators := uint64(64)
	stateWithValidators, _ := util.DeterministicGenesisState(t, validators)
	beaconState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, beaconState.SetValidators(stateWithValidators.Validators()))
	require.NoError(t, beaconState.SetInactivityScores(make([]uint64, len(stateWithValidators.Validators()))))
	require.NoError(t, beaconState.SetSlot(slot))

	b := util.NewBeaconBlock()
	b.Block.Slot = slot
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	gen := stategen.New(beaconDB, doublylinkedtree.New())
	require.NoError(t, gen.SaveState(ctx, gRoot, beaconState))
	require.NoError(t, beaconDB.SaveState(ctx, beaconState, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	bs := &Server{
		StateGen:           gen,
		GenesisTimeFetcher: &mock.ChainService{},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	// Test non exist public key.
	res, err := bs.GetIndividualVotes(ctx, &zondpb.IndividualVotesRequest{
		PublicKeys: [][]byte{{'a'}},
		Epoch:      0,
	})
	require.NoError(t, err)
	wanted := &zondpb.IndividualVotesRespond{
		IndividualVotes: []*zondpb.IndividualVotesRespond_IndividualVote{
			{PublicKey: []byte{'a'}, ValidatorIndex: primitives.ValidatorIndex(^uint64(0))},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")

	// Test non-existent validator index.
	res, err = bs.GetIndividualVotes(ctx, &zondpb.IndividualVotesRequest{
		Indices: []primitives.ValidatorIndex{100},
		Epoch:   0,
	})
	require.NoError(t, err)
	wanted = &zondpb.IndividualVotesRespond{
		IndividualVotes: []*zondpb.IndividualVotesRespond_IndividualVote{
			{ValidatorIndex: 100},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")

	// Test both.
	res, err = bs.GetIndividualVotes(ctx, &zondpb.IndividualVotesRequest{
		PublicKeys: [][]byte{{'a'}, {'b'}},
		Indices:    []primitives.ValidatorIndex{100, 101},
		Epoch:      0,
	})
	require.NoError(t, err)
	wanted = &zondpb.IndividualVotesRespond{
		IndividualVotes: []*zondpb.IndividualVotesRespond_IndividualVote{
			{PublicKey: []byte{'a'}, ValidatorIndex: primitives.ValidatorIndex(^uint64(0))},
			{PublicKey: []byte{'b'}, ValidatorIndex: primitives.ValidatorIndex(^uint64(0))},
			{ValidatorIndex: 100},
			{ValidatorIndex: 101},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")
}

func TestServer_GetIndividualVotes_Working(t *testing.T) {
	helpers.ClearCache()

	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.MinimalSpecConfig())
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	validators := uint64(32)
	stateWithValidators, _ := util.DeterministicGenesisState(t, validators)
	beaconState, err := util.NewBeaconState()
	require.NoError(t, err)
	require.NoError(t, beaconState.SetValidators(stateWithValidators.Validators()))
	require.NoError(t, beaconState.SetInactivityScores(make([]uint64, len(stateWithValidators.Validators()))))

	bf := bitfield.NewBitlist(validators / uint64(params.BeaconConfig().SlotsPerEpoch))
	att1 := util.NewAttestation()
	att1.ParticipationBits = bf
	att2 := util.NewAttestation()
	att2.ParticipationBits = bf
	rt := [32]byte{'A'}
	att1.Data.Target.Root = rt[:]
	att1.Data.BeaconBlockRoot = rt[:]
	br := beaconState.BlockRoots()
	newRt := [32]byte{'B'}
	br[0] = newRt[:]
	require.NoError(t, beaconState.SetBlockRoots(br))
	att2.Data.Target.Root = rt[:]
	att2.Data.BeaconBlockRoot = newRt[:]
	//err = beaconState.AppendPreviousEpochAttestations(&zondpb.PendingAttestation{
	//	Data: att1.Data, ParticipationBits: bf, InclusionDelay: 1,
	//})
	//require.NoError(t, err)
	//err = beaconState.AppendCurrentEpochAttestations(&zondpb.PendingAttestation{
	//	Data: att2.Data, ParticipationBits: bf, InclusionDelay: 1,
	//})
	require.NoError(t, err)

	b := util.NewBeaconBlock()
	b.Block.Slot = 0
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	gen := stategen.New(beaconDB, doublylinkedtree.New())
	require.NoError(t, gen.SaveState(ctx, gRoot, beaconState))
	require.NoError(t, beaconDB.SaveState(ctx, beaconState, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	bs := &Server{
		StateGen:           gen,
		GenesisTimeFetcher: &mock.ChainService{},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	res, err := bs.GetIndividualVotes(ctx, &zondpb.IndividualVotesRequest{
		Indices: []primitives.ValidatorIndex{0, 1},
		Epoch:   0,
	})
	require.NoError(t, err)
	wanted := &zondpb.IndividualVotesRespond{
		IndividualVotes: []*zondpb.IndividualVotesRespond_IndividualVote{
			{
				ValidatorIndex:                   0,
				PublicKey:                        beaconState.Validators()[0].PublicKey,
				IsActiveInCurrentEpoch:           true,
				IsActiveInPreviousEpoch:          true,
				CurrentEpochEffectiveBalanceGwei: params.BeaconConfig().MaxEffectiveBalance,
			},
			{
				ValidatorIndex:                   1,
				PublicKey:                        beaconState.Validators()[1].PublicKey,
				IsActiveInCurrentEpoch:           true,
				IsActiveInPreviousEpoch:          true,
				CurrentEpochEffectiveBalanceGwei: params.BeaconConfig().MaxEffectiveBalance,
			},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")
}

func TestServer_GetIndividualVotes_WorkingAltair(t *testing.T) {
	helpers.ClearCache()
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	var slot primitives.Slot = 0
	validators := uint64(32)
	beaconState, _ := util.DeterministicGenesisState(t, validators)
	require.NoError(t, beaconState.SetSlot(slot))

	pb, err := beaconState.CurrentEpochParticipation()
	require.NoError(t, err)
	for i := range pb {
		pb[i] = 0xff
	}
	require.NoError(t, beaconState.SetCurrentParticipationBits(pb))
	require.NoError(t, beaconState.SetPreviousParticipationBits(pb))

	b := util.NewBeaconBlock()
	b.Block.Slot = slot
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	gen := stategen.New(beaconDB, doublylinkedtree.New())
	require.NoError(t, gen.SaveState(ctx, gRoot, beaconState))
	require.NoError(t, beaconDB.SaveState(ctx, beaconState, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	bs := &Server{
		StateGen:           gen,
		GenesisTimeFetcher: &mock.ChainService{},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	res, err := bs.GetIndividualVotes(ctx, &zondpb.IndividualVotesRequest{
		Indices: []primitives.ValidatorIndex{0, 1},
		Epoch:   0,
	})
	require.NoError(t, err)
	wanted := &zondpb.IndividualVotesRespond{
		IndividualVotes: []*zondpb.IndividualVotesRespond_IndividualVote{
			{
				ValidatorIndex:                   0,
				PublicKey:                        beaconState.Validators()[0].PublicKey,
				IsActiveInCurrentEpoch:           true,
				IsActiveInPreviousEpoch:          true,
				IsCurrentEpochTargetAttester:     true,
				IsCurrentEpochAttester:           true,
				IsPreviousEpochAttester:          true,
				IsPreviousEpochHeadAttester:      true,
				IsPreviousEpochTargetAttester:    true,
				CurrentEpochEffectiveBalanceGwei: params.BeaconConfig().MaxEffectiveBalance,
			},
			{
				ValidatorIndex:                   1,
				PublicKey:                        beaconState.Validators()[1].PublicKey,
				IsActiveInCurrentEpoch:           true,
				IsActiveInPreviousEpoch:          true,
				IsCurrentEpochTargetAttester:     true,
				IsCurrentEpochAttester:           true,
				IsPreviousEpochAttester:          true,
				IsPreviousEpochHeadAttester:      true,
				IsPreviousEpochTargetAttester:    true,
				CurrentEpochEffectiveBalanceGwei: params.BeaconConfig().MaxEffectiveBalance,
			},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")
}

func TestServer_GetIndividualVotes_CapellaEndOfEpoch(t *testing.T) {
	helpers.ClearCache()
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.BeaconConfig())
	beaconDB := dbTest.SetupDB(t)
	ctx := context.Background()

	validators := uint64(32)
	beaconState, _ := util.DeterministicGenesisState(t, validators)
	startSlot, err := slots.EpochStart(1)
	assert.NoError(t, err)
	require.NoError(t, beaconState.SetSlot(startSlot))

	b := util.NewBeaconBlock()
	b.Block.Slot = startSlot
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err := b.Block.HashTreeRoot()
	require.NoError(t, err)
	gen := stategen.New(beaconDB, doublylinkedtree.New())
	require.NoError(t, gen.SaveState(ctx, gRoot, beaconState))
	require.NoError(t, beaconDB.SaveState(ctx, beaconState, gRoot))
	require.NoError(t, beaconDB.SaveGenesisBlockRoot(ctx, gRoot))
	// Save State at the end of the epoch:
	endSlot, err := slots.EpochEnd(1)
	assert.NoError(t, err)

	beaconState, _ = util.DeterministicGenesisState(t, validators)
	require.NoError(t, beaconState.SetSlot(endSlot))

	pb, err := beaconState.CurrentEpochParticipation()
	require.NoError(t, err)
	for i := range pb {
		pb[i] = 0xff
	}
	require.NoError(t, beaconState.SetCurrentParticipationBits(pb))
	require.NoError(t, beaconState.SetPreviousParticipationBits(pb))

	b.Block.Slot = endSlot
	util.SaveBlock(t, ctx, beaconDB, b)
	gRoot, err = b.Block.HashTreeRoot()
	require.NoError(t, err)

	require.NoError(t, gen.SaveState(ctx, gRoot, beaconState))
	require.NoError(t, beaconDB.SaveState(ctx, beaconState, gRoot))
	bs := &Server{
		StateGen:           gen,
		GenesisTimeFetcher: &mock.ChainService{},
	}
	addDefaultReplayerBuilder(bs, beaconDB)

	res, err := bs.GetIndividualVotes(ctx, &zondpb.IndividualVotesRequest{
		Indices: []primitives.ValidatorIndex{0, 1},
		Epoch:   1,
	})
	require.NoError(t, err)
	wanted := &zondpb.IndividualVotesRespond{
		IndividualVotes: []*zondpb.IndividualVotesRespond_IndividualVote{
			{
				ValidatorIndex:                   0,
				PublicKey:                        beaconState.Validators()[0].PublicKey,
				IsActiveInCurrentEpoch:           true,
				IsActiveInPreviousEpoch:          true,
				IsCurrentEpochTargetAttester:     true,
				IsCurrentEpochAttester:           true,
				IsPreviousEpochAttester:          true,
				IsPreviousEpochHeadAttester:      true,
				IsPreviousEpochTargetAttester:    true,
				CurrentEpochEffectiveBalanceGwei: params.BeaconConfig().MaxEffectiveBalance,
				Epoch:                            1,
			},
			{
				ValidatorIndex:                   1,
				PublicKey:                        beaconState.Validators()[1].PublicKey,
				IsActiveInCurrentEpoch:           true,
				IsActiveInPreviousEpoch:          true,
				IsCurrentEpochTargetAttester:     true,
				IsCurrentEpochAttester:           true,
				IsPreviousEpochAttester:          true,
				IsPreviousEpochHeadAttester:      true,
				IsPreviousEpochTargetAttester:    true,
				CurrentEpochEffectiveBalanceGwei: params.BeaconConfig().MaxEffectiveBalance,
				Epoch:                            1,
			},
		},
	}
	assert.DeepEqual(t, wanted, res, "Unexpected response")
}

func Test_validatorStatus(t *testing.T) {
	tests := []struct {
		name      string
		validator *zondpb.Validator
		epoch     primitives.Epoch
		want      zondpb.ValidatorStatus
	}{
		{
			name:      "Unknown",
			validator: nil,
			epoch:     0,
			want:      zondpb.ValidatorStatus_UNKNOWN_STATUS,
		},
		{
			name: "Deposited",
			validator: &zondpb.Validator{
				ActivationEligibilityEpoch: 1,
			},
			epoch: 0,
			want:  zondpb.ValidatorStatus_DEPOSITED,
		},
		{
			name: "Pending",
			validator: &zondpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            1,
			},
			epoch: 0,
			want:  zondpb.ValidatorStatus_PENDING,
		},
		{
			name: "Active",
			validator: &zondpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            0,
				ExitEpoch:                  params.BeaconConfig().FarFutureEpoch,
			},
			epoch: 0,
			want:  zondpb.ValidatorStatus_ACTIVE,
		},
		{
			name: "Slashed",
			validator: &zondpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            0,
				ExitEpoch:                  5,
				Slashed:                    true,
			},
			epoch: 4,
			want:  zondpb.ValidatorStatus_SLASHING,
		},
		{
			name: "Exiting",
			validator: &zondpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            0,
				ExitEpoch:                  5,
				Slashed:                    false,
			},
			epoch: 4,
			want:  zondpb.ValidatorStatus_EXITING,
		},
		{
			name: "Exiting",
			validator: &zondpb.Validator{
				ActivationEligibilityEpoch: 0,
				ActivationEpoch:            0,
				ExitEpoch:                  3,
				Slashed:                    false,
			},
			epoch: 4,
			want:  zondpb.ValidatorStatus_EXITED,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validatorStatus(tt.validator, tt.epoch); got != tt.want {
				t.Errorf("validatorStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
