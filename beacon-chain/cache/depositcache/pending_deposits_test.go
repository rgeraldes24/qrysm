package depositcache

import (
	"context"
	"math/big"
	"testing"

	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
)

var _ PendingDepositsFetcher = (*DepositCache)(nil)

func TestInsertPendingDeposit_OK(t *testing.T) {
	dc := DepositCache{}
	dc.InsertPendingDeposit(context.Background(), &qrysmpb.Deposit{}, 111, 100, [32]byte{})

	assert.Equal(t, 1, len(dc.pendingDeposits), "deposit not inserted")
}

func TestInsertPendingDeposit_ignoresNilDeposit(t *testing.T) {
	dc := DepositCache{}
	dc.InsertPendingDeposit(context.Background(), nil /*deposit*/, 0 /*blockNum*/, 0, [32]byte{})

	assert.Equal(t, 0, len(dc.pendingDeposits))
}

func TestPendingDeposits_OK(t *testing.T) {
	dc := DepositCache{}

	dc.pendingDeposits = []*qrysmpb.DepositContainer{
		{ExecutionBlockHeight: 2, Deposit: &qrysmpb.Deposit{Proof: [][]byte{[]byte("A")}}},
		{ExecutionBlockHeight: 4, Deposit: &qrysmpb.Deposit{Proof: [][]byte{[]byte("B")}}},
		{ExecutionBlockHeight: 6, Deposit: &qrysmpb.Deposit{Proof: [][]byte{[]byte("c")}}},
	}

	deposits := dc.PendingDeposits(context.Background(), big.NewInt(4))
	expected := []*qrysmpb.Deposit{
		{Proof: [][]byte{[]byte("A")}},
		{Proof: [][]byte{[]byte("B")}},
	}
	assert.DeepSSZEqual(t, expected, deposits)

	all := dc.PendingDeposits(context.Background(), nil)
	assert.Equal(t, len(dc.pendingDeposits), len(all), "PendingDeposits(ctx, nil) did not return all deposits")
}

func TestPrunePendingDeposits_ZeroMerkleIndex(t *testing.T) {
	dc := DepositCache{}

	dc.pendingDeposits = []*qrysmpb.DepositContainer{
		{ExecutionBlockHeight: 2, Index: 2},
		{ExecutionBlockHeight: 4, Index: 4},
		{ExecutionBlockHeight: 6, Index: 6},
		{ExecutionBlockHeight: 8, Index: 8},
		{ExecutionBlockHeight: 10, Index: 10},
		{ExecutionBlockHeight: 12, Index: 12},
	}

	dc.PrunePendingDeposits(context.Background(), 0)
	expected := []*qrysmpb.DepositContainer{
		{ExecutionBlockHeight: 2, Index: 2},
		{ExecutionBlockHeight: 4, Index: 4},
		{ExecutionBlockHeight: 6, Index: 6},
		{ExecutionBlockHeight: 8, Index: 8},
		{ExecutionBlockHeight: 10, Index: 10},
		{ExecutionBlockHeight: 12, Index: 12},
	}
	assert.DeepEqual(t, expected, dc.pendingDeposits)
}

func TestPrunePendingDeposits_OK(t *testing.T) {
	dc := DepositCache{}

	dc.pendingDeposits = []*qrysmpb.DepositContainer{
		{ExecutionBlockHeight: 2, Index: 2},
		{ExecutionBlockHeight: 4, Index: 4},
		{ExecutionBlockHeight: 6, Index: 6},
		{ExecutionBlockHeight: 8, Index: 8},
		{ExecutionBlockHeight: 10, Index: 10},
		{ExecutionBlockHeight: 12, Index: 12},
	}

	dc.PrunePendingDeposits(context.Background(), 6)
	expected := []*qrysmpb.DepositContainer{
		{ExecutionBlockHeight: 6, Index: 6},
		{ExecutionBlockHeight: 8, Index: 8},
		{ExecutionBlockHeight: 10, Index: 10},
		{ExecutionBlockHeight: 12, Index: 12},
	}

	assert.DeepEqual(t, expected, dc.pendingDeposits)

	dc.pendingDeposits = []*qrysmpb.DepositContainer{
		{ExecutionBlockHeight: 2, Index: 2},
		{ExecutionBlockHeight: 4, Index: 4},
		{ExecutionBlockHeight: 6, Index: 6},
		{ExecutionBlockHeight: 8, Index: 8},
		{ExecutionBlockHeight: 10, Index: 10},
		{ExecutionBlockHeight: 12, Index: 12},
	}

	dc.PrunePendingDeposits(context.Background(), 10)
	expected = []*qrysmpb.DepositContainer{
		{ExecutionBlockHeight: 10, Index: 10},
		{ExecutionBlockHeight: 12, Index: 12},
	}

	assert.DeepEqual(t, expected, dc.pendingDeposits)
}
