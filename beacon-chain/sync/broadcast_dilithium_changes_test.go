package sync

/*
import (
	"context"
	"testing"
	"time"

	mockChain "github.com/theQRL/qrysm/v4/beacon-chain/blockchain/testing"
	testingdb "github.com/theQRL/qrysm/v4/beacon-chain/db/testing"
	doublylinkedtree "github.com/theQRL/qrysm/v4/beacon-chain/forkchoice/doubly-linked-tree"
	"github.com/theQRL/qrysm/v4/beacon-chain/operations/dilithiumtoexec"
	mockp2p "github.com/theQRL/qrysm/v4/beacon-chain/p2p/testing"
	"github.com/theQRL/qrysm/v4/beacon-chain/state/stategen"
	mockSync "github.com/theQRL/qrysm/v4/beacon-chain/sync/initial-sync/testing"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func TestBroadcastDilithiumChanges(t *testing.T) {
	params.SetupTestConfigCleanup(t)

	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	s := NewService(context.Background(),
		WithP2P(mockp2p.NewTestP2P(t)),
		WithInitialSync(&mockSync.Sync{IsSyncing: false}),
		WithChainService(chainService),
		WithOperationNotifier(chainService.OperationNotifier()),
		WithDilithiumToExecPool(dilithiumtoexec.NewPool()),
	)
	var emptySig [4595]byte
	s.cfg.dilithiumToExecPool.InsertDilithiumToExecChange(&zondpb.SignedDilithiumToExecutionChange{
		Message: &zondpb.DilithiumToExecutionChange{
			ValidatorIndex:      10,
			FromDilithiumPubkey: make([]byte, 2592),
			ToExecutionAddress:  make([]byte, 20),
		},
		Signature: emptySig[:],
	})

	capellaStart := primitives.Slot(0)
	s.broadcastDilithiumChanges(capellaStart)
}

/*
func TestRateDilithiumChanges(t *testing.T) {
	logHook := logTest.NewGlobal()
	params.SetupTestConfigCleanup(t)

	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	p1 := mockp2p.NewTestP2P(t)
	s := NewService(context.Background(),
		WithP2P(p1),
		WithInitialSync(&mockSync.Sync{IsSyncing: false}),
		WithChainService(chainService),
		WithOperationNotifier(chainService.OperationNotifier()),
		WithDilithiumToExecPool(dilithiumtoexec.NewPool()),
	)
	beaconDB := testingdb.SetupDB(t)
	s.cfg.stateGen = stategen.New(beaconDB, doublylinkedtree.New())
	s.cfg.beaconDB = beaconDB
	s.initCaches()
	st, keys := util.DeterministicGenesisState(t, 256)
	s.cfg.chain = &mockChain.ChainService{
		ValidatorsRoot: [32]byte{'A'},
		Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(10)),
		State:          st,
	}

	for i := 0; i < 200; i++ {
		message := &zondpb.DilithiumToExecutionChange{
			ValidatorIndex:      primitives.ValidatorIndex(i),
			FromDilithiumPubkey: keys[i+1].PublicKey().Marshal(),
			ToExecutionAddress:  bytesutil.PadTo([]byte("address"), 20),
		}
		epoch := primitives.Epoch(0)
		domain, err := signing.Domain(st.Fork(), epoch, params.BeaconConfig().DomainDilithiumToExecutionChange, st.GenesisValidatorsRoot())
		assert.NoError(t, err)
		htr, err := signing.SigningData(message.HashTreeRoot, domain)
		assert.NoError(t, err)
		signed := &zondpb.SignedDilithiumToExecutionChange{
			Message:   message,
			Signature: keys[i+1].Sign(htr[:]).Marshal(),
		}

		s.cfg.dilithiumToExecPool.InsertDilithiumToExecChange(signed)
	}

	require.Equal(t, false, p1.BroadcastCalled)
	//slot, err := slots.EpochStart(params.BeaconConfig().CapellaForkEpoch)
	//require.NoError(t, err)
	slot := primitives.Slot(0)
	s.broadcastDilithiumChanges(slot)
	time.Sleep(100 * time.Millisecond) // Need a sleep for the go routine to be ready
	require.Equal(t, true, p1.BroadcastCalled)
	require.LogsDoNotContain(t, logHook, "could not")

	p1.BroadcastCalled = false
	time.Sleep(500 * time.Millisecond) // Need a sleep for the second batch to be broadcast
	require.Equal(t, true, p1.BroadcastCalled)
	require.LogsDoNotContain(t, logHook, "could not")
}

func TestBroadcastDilithiumBatch_changes_slice(t *testing.T) {
	message := &zondpb.DilithiumToExecutionChange{
		FromDilithiumPubkey: make([]byte, 2592),
		ToExecutionAddress:  make([]byte, 20),
	}
	signed := &zondpb.SignedDilithiumToExecutionChange{
		Message:   message,
		Signature: make([]byte, 4595),
	}
	changes := make([]*zondpb.SignedDilithiumToExecutionChange, 200)
	for i := 0; i < len(changes); i++ {
		changes[i] = signed
	}
	p1 := mockp2p.NewTestP2P(t)
	chainService := &mockChain.ChainService{
		Genesis:        time.Now(),
		ValidatorsRoot: [32]byte{'A'},
	}
	s := NewService(context.Background(),
		WithP2P(p1),
		WithInitialSync(&mockSync.Sync{IsSyncing: false}),
		WithChainService(chainService),
		WithOperationNotifier(chainService.OperationNotifier()),
		WithDilithiumToExecPool(dilithiumtoexec.NewPool()),
	)
	beaconDB := testingdb.SetupDB(t)
	s.cfg.stateGen = stategen.New(beaconDB, doublylinkedtree.New())
	s.cfg.beaconDB = beaconDB
	s.initCaches()
	st, _ := util.DeterministicGenesisState(t, 32)
	s.cfg.chain = &mockChain.ChainService{
		ValidatorsRoot: [32]byte{'A'},
		Genesis:        time.Now().Add(-time.Second * time.Duration(params.BeaconConfig().SecondsPerSlot) * time.Duration(10)),
		State:          st,
	}

	s.broadcastDilithiumBatch(s.ctx, &changes)
	require.Equal(t, 200-128, len(changes))
}
*/
