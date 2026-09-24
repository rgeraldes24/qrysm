package slashings

import (
	"context"
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/operations/slashings/mock"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
)

var (
	_ = PoolManager(&Pool{})
	_ = PoolInserter(&Pool{})
	_ = PoolManager(&mock.PoolMock{})
	_ = PoolInserter(&mock.PoolMock{})
)

func TestPool_validatorSlashingPreconditionCheck_requiresLock(t *testing.T) {
	p := &Pool{}
	_, err := p.validatorSlashingPreconditionCheck(nil, 0, false)
	require.ErrorContains(t, "caller must hold read/write lock", err)
}

func TestPool_RecoverSlashing(t *testing.T) {
	for _, kind := range []string{"proposer", "attester"} {
		for _, status := range []string{"slashable", "already slashed", "withdrawn", "bad signature"} {
			t.Run(kind+"/"+status, func(t *testing.T) {
				ctx := context.Background()
				st, keys := util.DeterministicGenesisStateZond(t, 64)
				p := NewPool()
				// An unrelated inclusion marker must survive recovery.
				p.included[1] = true
				var recover func() error
				var pending func() int
				if kind == "proposer" {
					proof, err := util.GenerateProposerSlashingForValidator(st, keys[0], 0)
					require.NoError(t, err)
					require.NoError(t, p.InsertProposerSlashing(ctx, st, proof))
					p.MarkIncludedProposerSlashing(proof)
					require.ErrorContains(t, "cannot be slashed", p.InsertProposerSlashing(ctx, st, proof))
					if status == "bad signature" {
						proof.Header_1.Signature[0] ^= 1
					}
					recover = func() error { return p.RecoverProposerSlashing(ctx, st, proof) }
					pending = func() int { return len(p.PendingProposerSlashings(ctx, st, true)) }
				} else {
					proof, err := util.GenerateAttesterSlashingForValidator(st, keys[0], 0)
					require.NoError(t, err)
					require.NoError(t, p.InsertAttesterSlashing(ctx, st, proof))
					p.MarkIncludedAttesterSlashing(proof)
					require.ErrorContains(t, "already recently included", p.InsertAttesterSlashing(ctx, st, proof))
					if status == "bad signature" {
						proof.Attestation_1.Signatures[0][0] ^= 1
					}
					recover = func() error { return p.RecoverAttesterSlashing(ctx, st, proof) }
					pending = func() int { return len(p.PendingAttesterSlashings(ctx, st, true)) }
				}
				v, err := st.ValidatorAtIndex(0)
				require.NoError(t, err)
				if status == "already slashed" {
					v.Slashed = true
				} else if status == "withdrawn" {
					v.WithdrawableEpoch = 0
				}
				require.NoError(t, st.UpdateValidatorAtIndex(0, v))
				if status == "slashable" {
					require.NoError(t, recover())
					require.NoError(t, recover(), "recovery retries must be idempotent")
					require.Equal(t, 1, pending())
					require.Equal(t, false, p.included[0])
				} else {
					require.NotNil(t, recover())
					require.Equal(t, 0, pending())
					require.Equal(t, true, p.included[0])
				}
				require.Equal(t, true, p.included[1])
			})
		}
	}
}

func TestPool_RecoverAttesterSlashing_PartlyCanonical(t *testing.T) {
	ctx := context.Background()
	st, keys := util.DeterministicGenesisStateZond(t, 64)
	proof := validAttesterSlashingForValIdx(t, st, keys, 0, 1)
	p := NewPool()
	p.MarkIncludedAttesterSlashing(proof)
	// Validator 1 is also slashed on the replacement branch. Only validator 0
	// should lose its inclusion marker and return to the pending pool.
	v, err := st.ValidatorAtIndex(1)
	require.NoError(t, err)
	v.Slashed = true
	require.NoError(t, st.UpdateValidatorAtIndex(1, v))
	require.NoError(t, p.RecoverAttesterSlashing(ctx, st, proof))
	require.NoError(t, p.RecoverAttesterSlashing(ctx, st, proof))
	require.Equal(t, false, p.included[0])
	require.Equal(t, true, p.included[1])
	require.Equal(t, 1, len(p.pendingAttesterSlashing))
	require.Equal(t, primitives.ValidatorIndex(0), p.pendingAttesterSlashing[0].validatorToSlash)
	require.Equal(t, 1, len(p.PendingAttesterSlashings(ctx, st, true)))
}
