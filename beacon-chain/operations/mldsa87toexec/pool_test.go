package mldsa87toexec

import (
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/beacon-chain/core/time"
	state_native "github.com/theQRL/qrysm/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/hash"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87/common"
	"github.com/theQRL/qrysm/encoding/ssz"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

func TestPendingMLDSA87ToExecChanges(t *testing.T) {
	t.Run("empty pool", func(t *testing.T) {
		pool := NewPool()
		changes, err := pool.PendingMLDSA87ToExecChanges()
		require.NoError(t, err)
		assert.Equal(t, 0, len(changes))
	})
	t.Run("non-empty pool", func(t *testing.T) {
		pool := NewPool()
		pool.InsertMLDSA87ToExecChange(&qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: 0,
			},
		})
		pool.InsertMLDSA87ToExecChange(&qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: 1,
			},
		})
		changes, err := pool.PendingMLDSA87ToExecChanges()
		require.NoError(t, err)
		assert.Equal(t, 2, len(changes))
	})
}

func TestMLDSA87ToExecChangesForInclusion(t *testing.T) {
	spb := &qrysmpb.BeaconStateCapella{
		Fork: &qrysmpb.Fork{
			CurrentVersion:  params.BeaconConfig().GenesisForkVersion,
			PreviousVersion: params.BeaconConfig().GenesisForkVersion,
		},
	}
	numValidators := 2 * params.BeaconConfig().MaxMLDSA87ToExecutionChanges
	validators := make([]*qrysmpb.Validator, numValidators)
	mlDSA87Changes := make([]*qrysmpb.MLDSA87ToExecutionChange, numValidators)
	spb.Balances = make([]uint64, numValidators)
	privKeys := make([]common.SecretKey, numValidators)
	maxEffectiveBalance := params.BeaconConfig().MaxEffectiveBalance
	executionAddress := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13}

	for i := range validators {
		v := &qrysmpb.Validator{}
		v.EffectiveBalance = maxEffectiveBalance
		v.WithdrawableEpoch = params.BeaconConfig().FarFutureEpoch
		v.WithdrawalCredentials = make([]byte, 32)
		priv, err := ml_dsa_87.RandKey()
		require.NoError(t, err)
		privKeys[i] = priv
		pubkey := priv.PublicKey().Marshal()

		message := &qrysmpb.MLDSA87ToExecutionChange{
			ToExecutionAddress: executionAddress,
			ValidatorIndex:     primitives.ValidatorIndex(i),
			FromMldsa87Pubkey:  pubkey,
		}

		hashFn := ssz.NewHasherFunc(hash.CustomSHA256Hasher())
		digest := hashFn.Hash(pubkey)
		digest[0] = params.BeaconConfig().MLDSA87WithdrawalPrefixByte
		copy(v.WithdrawalCredentials, digest[:])
		validators[i] = v
		mlDSA87Changes[i] = message
	}
	spb.Validators = validators
	st, err := state_native.InitializeFromProtoCapella(spb)
	require.NoError(t, err)

	signedChanges := make([]*qrysmpb.SignedMLDSA87ToExecutionChange, numValidators)
	for i, message := range mlDSA87Changes {
		signature, err := signing.ComputeDomainAndSign(st, time.CurrentEpoch(st), message, params.BeaconConfig().DomainMLDSA87ToExecutionChange, privKeys[i])
		require.NoError(t, err)

		signed := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message:   message,
			Signature: signature,
		}
		signedChanges[i] = signed
	}

	t.Run("empty pool", func(t *testing.T) {
		pool := NewPool()
		changes, err := pool.MLDSA87ToExecChangesForInclusion(st)
		require.NoError(t, err)
		assert.Equal(t, 0, len(changes))
	})
	t.Run("Less than MaxMLDSA87ToExecutionChanges in pool", func(t *testing.T) {
		pool := NewPool()
		for i := uint64(0); i < params.BeaconConfig().MaxMLDSA87ToExecutionChanges-1; i++ {
			pool.InsertMLDSA87ToExecChange(signedChanges[i])
		}
		changes, err := pool.MLDSA87ToExecChangesForInclusion(st)
		require.NoError(t, err)
		assert.Equal(t, int(params.BeaconConfig().MaxMLDSA87ToExecutionChanges)-1, len(changes))
	})
	t.Run("MaxMLDSA87ToExecutionChanges in pool", func(t *testing.T) {
		pool := NewPool()
		for i := uint64(0); i < params.BeaconConfig().MaxMLDSA87ToExecutionChanges; i++ {
			pool.InsertMLDSA87ToExecChange(signedChanges[i])
		}
		changes, err := pool.MLDSA87ToExecChangesForInclusion(st)
		require.NoError(t, err)
		assert.Equal(t, int(params.BeaconConfig().MaxMLDSA87ToExecutionChanges), len(changes))
	})
	t.Run("more than MaxMLDSA87ToExecutionChanges in pool", func(t *testing.T) {
		pool := NewPool()
		for i := uint64(0); i < numValidators; i++ {
			pool.InsertMLDSA87ToExecChange(signedChanges[i])
		}
		changes, err := pool.MLDSA87ToExecChangesForInclusion(st)
		require.NoError(t, err)
		// We want FIFO semantics, which means validator with index 16 shouldn't be returned
		assert.Equal(t, int(params.BeaconConfig().MaxMLDSA87ToExecutionChanges), len(changes))
		for _, ch := range changes {
			assert.NotEqual(t, primitives.ValidatorIndex(15), ch.Message.ValidatorIndex)
		}
	})
	t.Run("One Bad change", func(t *testing.T) {
		pool := NewPool()
		saveByte := signedChanges[1].Message.FromMldsa87Pubkey[5]
		signedChanges[1].Message.FromMldsa87Pubkey[5] = 0xff
		for i := uint64(0); i < numValidators; i++ {
			pool.InsertMLDSA87ToExecChange(signedChanges[i])
		}
		changes, err := pool.MLDSA87ToExecChangesForInclusion(st)
		require.NoError(t, err)
		assert.Equal(t, int(params.BeaconConfig().MaxMLDSA87ToExecutionChanges), len(changes))
		assert.Equal(t, primitives.ValidatorIndex(30), changes[1].Message.ValidatorIndex)
		signedChanges[1].Message.FromMldsa87Pubkey[5] = saveByte
	})
	t.Run("One Bad Signature", func(t *testing.T) {
		pool := NewPool()
		copy(signedChanges[30].Signature, signedChanges[31].Signature)
		for i := uint64(0); i < numValidators; i++ {
			pool.InsertMLDSA87ToExecChange(signedChanges[i])
		}
		changes, err := pool.MLDSA87ToExecChangesForInclusion(st)
		require.NoError(t, err)
		assert.Equal(t, int(params.BeaconConfig().MaxMLDSA87ToExecutionChanges), len(changes))
		assert.Equal(t, primitives.ValidatorIndex(30), changes[1].Message.ValidatorIndex)
	})
	t.Run("invalid change not returned", func(t *testing.T) {
		pool := NewPool()
		saveByte := signedChanges[1].Message.FromMldsa87Pubkey[5]
		signedChanges[1].Message.FromMldsa87Pubkey[5] = 0xff
		pool.InsertMLDSA87ToExecChange(signedChanges[1])
		changes, err := pool.MLDSA87ToExecChangesForInclusion(st)
		require.NoError(t, err)
		assert.Equal(t, 0, len(changes))
		signedChanges[1].Message.FromMldsa87Pubkey[5] = saveByte
	})
}

func TestInsertMLDSA87ToExecChange(t *testing.T) {
	t.Run("empty pool", func(t *testing.T) {
		pool := NewPool()
		change := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			},
		}
		pool.InsertMLDSA87ToExecChange(change)
		require.Equal(t, 1, pool.pending.Len())
		require.Equal(t, 1, len(pool.m))
		n, ok := pool.m[0]
		require.Equal(t, true, ok)
		v, err := n.Value()
		require.NoError(t, err)
		assert.DeepEqual(t, change, v)
	})
	t.Run("item in pool", func(t *testing.T) {
		pool := NewPool()
		old := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			},
		}
		change := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(1),
			},
		}
		pool.InsertMLDSA87ToExecChange(old)
		pool.InsertMLDSA87ToExecChange(change)
		require.Equal(t, 2, pool.pending.Len())
		require.Equal(t, 2, len(pool.m))
		n, ok := pool.m[0]
		require.Equal(t, true, ok)
		v, err := n.Value()
		require.NoError(t, err)
		assert.DeepEqual(t, old, v)
		n, ok = pool.m[1]
		require.Equal(t, true, ok)
		v, err = n.Value()
		require.NoError(t, err)
		assert.DeepEqual(t, change, v)
	})
	t.Run("validator index already exists", func(t *testing.T) {
		pool := NewPool()
		old := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			},
			Signature: []byte("old"),
		}
		change := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			},
			Signature: []byte("change"),
		}
		pool.InsertMLDSA87ToExecChange(old)
		pool.InsertMLDSA87ToExecChange(change)
		assert.Equal(t, 1, pool.pending.Len())
		require.Equal(t, 1, len(pool.m))
		n, ok := pool.m[0]
		require.Equal(t, true, ok)
		v, err := n.Value()
		require.NoError(t, err)
		assert.DeepEqual(t, old, v)
	})
}

func TestMarkIncluded(t *testing.T) {
	t.Run("one element in pool", func(t *testing.T) {
		pool := NewPool()
		change := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		pool.InsertMLDSA87ToExecChange(change)
		pool.MarkIncluded(change)
		assert.Equal(t, 0, pool.pending.Len())
		_, ok := pool.m[0]
		assert.Equal(t, false, ok)
	})
	t.Run("first of multiple elements", func(t *testing.T) {
		pool := NewPool()
		first := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		second := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(1),
			}}
		third := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(2),
			}}
		pool.InsertMLDSA87ToExecChange(first)
		pool.InsertMLDSA87ToExecChange(second)
		pool.InsertMLDSA87ToExecChange(third)
		pool.MarkIncluded(first)
		require.Equal(t, 2, pool.pending.Len())
		_, ok := pool.m[0]
		assert.Equal(t, false, ok)
	})
	t.Run("last of multiple elements", func(t *testing.T) {
		pool := NewPool()
		first := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		second := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(1),
			}}
		third := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(2),
			}}
		pool.InsertMLDSA87ToExecChange(first)
		pool.InsertMLDSA87ToExecChange(second)
		pool.InsertMLDSA87ToExecChange(third)
		pool.MarkIncluded(third)
		require.Equal(t, 2, pool.pending.Len())
		_, ok := pool.m[2]
		assert.Equal(t, false, ok)
	})
	t.Run("in the middle of multiple elements", func(t *testing.T) {
		pool := NewPool()
		first := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		second := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(1),
			}}
		third := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(2),
			}}
		pool.InsertMLDSA87ToExecChange(first)
		pool.InsertMLDSA87ToExecChange(second)
		pool.InsertMLDSA87ToExecChange(third)
		pool.MarkIncluded(second)
		require.Equal(t, 2, pool.pending.Len())
		_, ok := pool.m[1]
		assert.Equal(t, false, ok)
	})
	t.Run("not in pool", func(t *testing.T) {
		pool := NewPool()
		first := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		second := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(1),
			}}
		change := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(2),
			}}
		pool.InsertMLDSA87ToExecChange(first)
		pool.InsertMLDSA87ToExecChange(second)
		pool.MarkIncluded(change)
		require.Equal(t, 2, pool.pending.Len())
		_, ok := pool.m[0]
		require.Equal(t, true, ok)
		assert.NotNil(t, pool.m[0])
		_, ok = pool.m[1]
		require.Equal(t, true, ok)
		assert.NotNil(t, pool.m[1])
	})
}

func TestValidatorExists(t *testing.T) {
	t.Run("no validators in pool", func(t *testing.T) {
		pool := NewPool()
		assert.Equal(t, false, pool.ValidatorExists(0))
	})
	t.Run("validator added to pool", func(t *testing.T) {
		pool := NewPool()
		change := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		pool.InsertMLDSA87ToExecChange(change)
		assert.Equal(t, true, pool.ValidatorExists(0))
	})
	t.Run("multiple validators added to pool", func(t *testing.T) {
		pool := NewPool()
		change := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		pool.InsertMLDSA87ToExecChange(change)
		change = &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(10),
			}}
		pool.InsertMLDSA87ToExecChange(change)
		change = &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(30),
			}}
		pool.InsertMLDSA87ToExecChange(change)

		assert.Equal(t, true, pool.ValidatorExists(0))
		assert.Equal(t, true, pool.ValidatorExists(10))
		assert.Equal(t, true, pool.ValidatorExists(30))
	})
	t.Run("validator added and then removed", func(t *testing.T) {
		pool := NewPool()
		change := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		pool.InsertMLDSA87ToExecChange(change)
		pool.MarkIncluded(change)
		assert.Equal(t, false, pool.ValidatorExists(0))
	})
	t.Run("multiple validators added to pool and removed", func(t *testing.T) {
		pool := NewPool()
		firstChange := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(0),
			}}
		pool.InsertMLDSA87ToExecChange(firstChange)
		secondChange := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(10),
			}}
		pool.InsertMLDSA87ToExecChange(secondChange)
		thirdChange := &qrysmpb.SignedMLDSA87ToExecutionChange{
			Message: &qrysmpb.MLDSA87ToExecutionChange{
				ValidatorIndex: primitives.ValidatorIndex(30),
			}}
		pool.InsertMLDSA87ToExecChange(thirdChange)

		pool.MarkIncluded(firstChange)
		pool.MarkIncluded(thirdChange)

		assert.Equal(t, false, pool.ValidatorExists(0))
		assert.Equal(t, true, pool.ValidatorExists(10))
		assert.Equal(t, false, pool.ValidatorExists(30))
	})
}

func TestPoolCycleMap(t *testing.T) {
	pool := NewPool()
	firstChange := &qrysmpb.SignedMLDSA87ToExecutionChange{
		Message: &qrysmpb.MLDSA87ToExecutionChange{
			ValidatorIndex: primitives.ValidatorIndex(0),
		}}
	pool.InsertMLDSA87ToExecChange(firstChange)
	secondChange := &qrysmpb.SignedMLDSA87ToExecutionChange{
		Message: &qrysmpb.MLDSA87ToExecutionChange{
			ValidatorIndex: primitives.ValidatorIndex(10),
		}}
	pool.InsertMLDSA87ToExecChange(secondChange)
	thirdChange := &qrysmpb.SignedMLDSA87ToExecutionChange{
		Message: &qrysmpb.MLDSA87ToExecutionChange{
			ValidatorIndex: primitives.ValidatorIndex(30),
		}}
	pool.InsertMLDSA87ToExecChange(thirdChange)

	pool.cycleMap()
	require.Equal(t, true, pool.ValidatorExists(0))
	require.Equal(t, true, pool.ValidatorExists(10))
	require.Equal(t, true, pool.ValidatorExists(30))
	require.Equal(t, false, pool.ValidatorExists(20))

}
