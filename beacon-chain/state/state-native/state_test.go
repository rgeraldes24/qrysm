package state_native

import (
	"strconv"
	"sync"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stateutil"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func TestValidatorMap_DistinctCopy(t *testing.T) {
	count := uint64(100)
	vals := make([]*zondpb.Validator, 0, count)
	for i := uint64(1); i < count; i++ {
		var someRoot [32]byte
		var someKey [dilithium.CryptoPublicKeyBytes]byte
		copy(someRoot[:], strconv.Itoa(int(i)))
		copy(someKey[:], strconv.Itoa(int(i)))
		vals = append(vals, &zondpb.Validator{
			PublicKey:                  someKey[:],
			WithdrawalCredentials:      someRoot[:],
			EffectiveBalance:           params.BeaconConfig().MaxEffectiveBalance,
			Slashed:                    false,
			ActivationEligibilityEpoch: 1,
			ActivationEpoch:            1,
			ExitEpoch:                  1,
			WithdrawableEpoch:          1,
		})
	}
	handler := stateutil.NewValMapHandler(vals)
	newHandler := handler.Copy()
	wantedPubkey := strconv.Itoa(22)
	handler.Set(bytesutil.ToBytes2592([]byte(wantedPubkey)), 27)
	val1, _ := handler.Get(bytesutil.ToBytes2592([]byte(wantedPubkey)))
	val2, _ := newHandler.Get(bytesutil.ToBytes2592([]byte(wantedPubkey)))
	assert.NotEqual(t, val1, val2, "Values are supposed to be unequal due to copy")
}

func TestBeaconState_NoDeadlock_Capella(t *testing.T) {
	count := uint64(100)
	vals := make([]*zondpb.Validator, 0, count)
	for i := uint64(1); i < count; i++ {
		var someRoot [32]byte
		var someKey [dilithium.CryptoPublicKeyBytes]byte
		copy(someRoot[:], strconv.Itoa(int(i)))
		copy(someKey[:], strconv.Itoa(int(i)))
		vals = append(vals, &zondpb.Validator{
			PublicKey:                  someKey[:],
			WithdrawalCredentials:      someRoot[:],
			EffectiveBalance:           params.BeaconConfig().MaxEffectiveBalance,
			Slashed:                    false,
			ActivationEligibilityEpoch: 1,
			ActivationEpoch:            1,
			ExitEpoch:                  1,
			WithdrawableEpoch:          1,
		})
	}
	st, err := InitializeFromProtoUnsafeCapella(&zondpb.BeaconState{
		Validators: vals,
	})
	assert.NoError(t, err)
	s, ok := st.(*BeaconState)
	require.Equal(t, true, ok)

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		// Continuously lock and unlock the state
		// by acquiring the lock.
		for i := 0; i < 1000; i++ {
			for _, f := range s.stateFieldLeaves {
				f.Lock()
				if f.Empty() {
					f.InsertFieldLayer(make([][]*[32]byte, 10))
				}
				f.Unlock()
				f.FieldReference().AddRef()
			}
		}
		wg.Done()
	}()
	// Constantly read from the offending portion
	// of the code to ensure there is no possible
	// recursive read locking.
	for i := 0; i < 1000; i++ {
		go func() {
			_ = st.FieldReferencesCount()
		}()
	}
	// Test will not terminate in the event of a deadlock.
	wg.Wait()
}

/*
func TestBeaconState_AppendBalanceWithTrie(t *testing.T) {

	newState := generateState(t)
	st, ok := newState.(*BeaconState)
	require.Equal(t, true, ok)
	_, err := st.HashTreeRoot(context.Background())
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			assert.NoError(t, st.UpdateBalancesAtIndex(primitives.ValidatorIndex(i), 1000))
		}
		if i%3 == 0 {
			assert.NoError(t, st.AppendBalance(1000))
		}
	}
	_, err = st.HashTreeRoot(context.Background())
	assert.NoError(t, err)
	newRt := bytesutil.ToBytes32(st.merkleLayers[0][types.Balances])
	wantedRt, err := stateutil.Uint64ListRootWithRegistryLimit(st.Balances())
	assert.NoError(t, err)
	assert.Equal(t, wantedRt, newRt, "state roots are unequal")
}
*/

/*
func TestBeaconState_ModifyPreviousParticipationBits(t *testing.T) {
	st, err := InitializeFromProtoUnsafeCapella(&zondpb.BeaconState{})
	assert.NoError(t, err)
	assert.ErrorContains(t, "ModifyPreviousParticipationBits is not supported", st.ModifyPreviousParticipationBits(func(val []byte) ([]byte, error) {
		return nil, nil
	}))
}
*/

/*
func TestBeaconState_ModifyCurrentParticipationBits(t *testing.T) {
	st, err := InitializeFromProtoUnsafeCapella(&zondpb.BeaconState{})
	assert.NoError(t, err)
	assert.ErrorContains(t, "ModifyCurrentParticipationBits is not supported", st.ModifyCurrentParticipationBits(func(val []byte) ([]byte, error) {
		return nil, nil
	}))
}
*/

/*
func TestCopyAllTries(t *testing.T) {
	newState := generateState(t)
	_, err := newState.HashTreeRoot(context.Background())
	assert.NoError(t, err)

	assert.NoError(t, newState.UpdateBalancesAtIndex(0, 10000))
	assert.NoError(t, newState.UpdateBlockRootAtIndex(0, [32]byte{'a'}))

	_, err = newState.HashTreeRoot(context.Background())
	assert.NoError(t, err)

	st, ok := newState.(*BeaconState)
	require.Equal(t, true, ok)

	obj := st.stateFieldLeaves[types.Balances]

	fieldAddr := fmt.Sprintf("%p", obj)

	nState, ok := st.Copy().(*BeaconState)
	require.Equal(t, true, ok)

	obj = nState.stateFieldLeaves[types.Balances]

	newFieldAddr := fmt.Sprintf("%p", obj)
	assert.Equal(t, fieldAddr, newFieldAddr)
	assert.Equal(t, 2, int(obj.FieldReference().Refs()))

	nState.CopyAllTries()

	obj = nState.stateFieldLeaves[types.Balances]
	updatedFieldAddr := fmt.Sprintf("%p", obj)

	assert.NotEqual(t, fieldAddr, updatedFieldAddr)
	assert.Equal(t, 1, int(obj.FieldReference().Refs()))

	assert.NoError(t, nState.UpdateBalancesAtIndex(20, 10000))

	_, err = nState.HashTreeRoot(context.Background())
	assert.NoError(t, err)

	rt, err := st.stateFieldLeaves[types.Balances].TrieRoot()
	assert.NoError(t, err)

	newRt, err := nState.stateFieldLeaves[types.Balances].TrieRoot()
	assert.NoError(t, err)
	assert.NotEqual(t, rt, newRt)
}
*/

func generateState(t *testing.T) state.BeaconState {
	count := uint64(100)
	vals := make([]*zondpb.Validator, 0, count)
	bals := make([]uint64, 0, count)
	for i := uint64(1); i < count; i++ {
		var someRoot [32]byte
		var someKey [dilithium.CryptoPublicKeyBytes]byte
		copy(someRoot[:], strconv.Itoa(int(i)))
		copy(someKey[:], strconv.Itoa(int(i)))
		vals = append(vals, &zondpb.Validator{
			PublicKey:                  someKey[:],
			WithdrawalCredentials:      someRoot[:],
			EffectiveBalance:           params.BeaconConfig().MaxEffectiveBalance,
			Slashed:                    false,
			ActivationEligibilityEpoch: 1,
			ActivationEpoch:            1,
			ExitEpoch:                  1,
			WithdrawableEpoch:          1,
		})
		bals = append(bals, params.BeaconConfig().MaxEffectiveBalance)
	}
	zeroHash := params.BeaconConfig().ZeroHash
	mockblockRoots := make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	for i := 0; i < len(mockblockRoots); i++ {
		mockblockRoots[i] = zeroHash[:]
	}

	mockstateRoots := make([][]byte, params.BeaconConfig().SlotsPerHistoricalRoot)
	for i := 0; i < len(mockstateRoots); i++ {
		mockstateRoots[i] = zeroHash[:]
	}
	mockrandaoMixes := make([][]byte, params.BeaconConfig().EpochsPerHistoricalVector)
	for i := 0; i < len(mockrandaoMixes); i++ {
		mockrandaoMixes[i] = zeroHash[:]
	}
	newState, err := InitializeFromProtoCapella(&zondpb.BeaconState{
		Slot:                  1,
		GenesisValidatorsRoot: make([]byte, 32),
		Fork: &zondpb.Fork{
			PreviousVersion: make([]byte, 4),
			CurrentVersion:  make([]byte, 4),
			Epoch:           0,
		},
		LatestBlockHeader: &zondpb.BeaconBlockHeader{
			ParentRoot: make([]byte, fieldparams.RootLength),
			StateRoot:  make([]byte, fieldparams.RootLength),
			BodyRoot:   make([]byte, fieldparams.RootLength),
		},
		Validators: vals,
		Balances:   bals,
		Zond1Data: &zondpb.Zond1Data{
			DepositRoot: make([]byte, 32),
			BlockHash:   make([]byte, 32),
		},
		BlockRoots:                  mockblockRoots,
		StateRoots:                  mockstateRoots,
		RandaoMixes:                 mockrandaoMixes,
		JustificationBits:           bitfield.NewBitvector4(),
		PreviousJustifiedCheckpoint: &zondpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
		CurrentJustifiedCheckpoint:  &zondpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
		FinalizedCheckpoint:         &zondpb.Checkpoint{Root: make([]byte, fieldparams.RootLength)},
		Slashings:                   make([]uint64, params.BeaconConfig().EpochsPerSlashingsVector),
	})
	assert.NoError(t, err)
	return newState
}
