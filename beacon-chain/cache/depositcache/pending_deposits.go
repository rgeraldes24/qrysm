package depositcache

import (
	"context"
	"math/big"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"go.opencensus.io/trace"
)

var (
	pendingDepositsCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "beacondb_pending_deposits",
		Help: "The number of pending deposits in the beaconDB in-memory database",
	})
)

// PendingDepositsFetcher specifically outlines a struct that can retrieve deposits
// which have not yet been included in the chain.
type PendingDepositsFetcher interface {
	PendingContainers(ctx context.Context, untilBlk *big.Int) []*qrysmpb.DepositContainer
}

// InsertPendingDeposit into the database. If deposit or block number are nil
// then this method does nothing.
func (dc *DepositCache) InsertPendingDeposit(ctx context.Context, d *qrysmpb.Deposit, blockNum uint64, index int64, depositRoot [32]byte) {
	_, span := trace.StartSpan(ctx, "DepositsCache.InsertPendingDeposit")
	defer span.End()
	if d == nil {
		log.WithFields(logrus.Fields{
			"block":   blockNum,
			"deposit": d,
		}).Debug("Ignoring nil deposit insertion")
		return
	}
	dc.depositsLock.Lock()
	defer dc.depositsLock.Unlock()
	dc.pendingDeposits = append(dc.pendingDeposits,
		&qrysmpb.DepositContainer{Deposit: d, ExecutionBlockHeight: blockNum, Index: index, DepositRoot: depositRoot[:]})
	pendingDepositsCount.Inc()
	span.AddAttributes(trace.Int64Attribute("count", int64(len(dc.pendingDeposits))))
}

// PendingDeposits returns a list of deposits until the given block number
// (inclusive). If no block is specified then this method returns all pending
// deposits.
func (dc *DepositCache) PendingDeposits(ctx context.Context, untilBlk *big.Int) []*qrysmpb.Deposit {
	ctx, span := trace.StartSpan(ctx, "DepositsCache.PendingDeposits")
	defer span.End()

	depositCntrs := dc.PendingContainers(ctx, untilBlk)

	deposits := make([]*qrysmpb.Deposit, 0, len(depositCntrs))
	for _, dep := range depositCntrs {
		deposits = append(deposits, dep.Deposit)
	}

	return deposits
}

// PendingContainers returns a list of deposit containers until the given block number
// (inclusive).
func (dc *DepositCache) PendingContainers(ctx context.Context, untilBlk *big.Int) []*qrysmpb.DepositContainer {
	_, span := trace.StartSpan(ctx, "DepositsCache.PendingDeposits")
	defer span.End()
	dc.depositsLock.RLock()
	defer dc.depositsLock.RUnlock()

	depositCntrs := make([]*qrysmpb.DepositContainer, 0, len(dc.pendingDeposits))
	for _, ctnr := range dc.pendingDeposits {
		if untilBlk == nil || untilBlk.Uint64() >= ctnr.ExecutionBlockHeight {
			depositCntrs = append(depositCntrs, ctnr)
		}
	}
	// Sort the deposits by Merkle index.
	sort.SliceStable(depositCntrs, func(i, j int) bool {
		return depositCntrs[i].Index < depositCntrs[j].Index
	})

	span.AddAttributes(trace.Int64Attribute("count", int64(len(depositCntrs))))

	return depositCntrs
}

// PrunePendingDeposits removes any deposit which is older than the given deposit merkle tree index.
func (dc *DepositCache) PrunePendingDeposits(ctx context.Context, merkleTreeIndex int64) {
	_, span := trace.StartSpan(ctx, "DepositsCache.PrunePendingDeposits")
	defer span.End()

	if merkleTreeIndex == 0 {
		log.Debug("Ignoring 0 deposit removal")
		return
	}

	dc.depositsLock.Lock()
	defer dc.depositsLock.Unlock()

	cleanDeposits := make([]*qrysmpb.DepositContainer, 0, len(dc.pendingDeposits))
	for _, dp := range dc.pendingDeposits {
		if dp.Index >= merkleTreeIndex {
			cleanDeposits = append(cleanDeposits, dp)
		}
	}

	dc.pendingDeposits = cleanDeposits
	pendingDepositsCount.Set(float64(len(dc.pendingDeposits)))
}
