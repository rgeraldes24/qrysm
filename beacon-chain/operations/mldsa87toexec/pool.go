package mldsa87toexec

import (
	"math"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/beacon-chain/state"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	doublylinkedlist "github.com/theQRL/qrysm/container/doubly-linked-list"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// We recycle the ML-DSA-87 changes pool to avoid the backing map growing without
// bound. The cycling operation is expensive because it copies all elements, so
// we only do it when the map is smaller than this upper bound.
const mlDSA87ChangesPoolThreshold = 2000

var (
	mlDSA87ToExecMessageInPoolTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ml_dsa_87_to_exec_message_pool_total",
		Help: "The number of saved ml-dsa-87 to exec messages in the operation pool.",
	})
)

// PoolManager maintains pending and seen ML-DSA-87-to-execution-change objects.
// This pool is used by proposers to insert ML-DSA-87-to-execution-change objects into new blocks.
type PoolManager interface {
	PendingMLDSA87ToExecChanges() ([]*qrysmpb.SignedMLDSA87ToExecutionChange, error)
	MLDSA87ToExecChangesForInclusion(beaconState state.ReadOnlyBeaconState) ([]*qrysmpb.SignedMLDSA87ToExecutionChange, error)
	InsertMLDSA87ToExecChange(change *qrysmpb.SignedMLDSA87ToExecutionChange)
	MarkIncluded(change *qrysmpb.SignedMLDSA87ToExecutionChange)
	ValidatorExists(idx primitives.ValidatorIndex) bool
}

// Pool is a concrete implementation of PoolManager.
type Pool struct {
	lock    sync.RWMutex
	pending doublylinkedlist.List[*qrysmpb.SignedMLDSA87ToExecutionChange]
	m       map[primitives.ValidatorIndex]*doublylinkedlist.Node[*qrysmpb.SignedMLDSA87ToExecutionChange]
}

// NewPool returns an initialized pool.
func NewPool() *Pool {
	return &Pool{
		pending: doublylinkedlist.List[*qrysmpb.SignedMLDSA87ToExecutionChange]{},
		m:       make(map[primitives.ValidatorIndex]*doublylinkedlist.Node[*qrysmpb.SignedMLDSA87ToExecutionChange]),
	}
}

// Copies the internal map and returns a new one.
func (p *Pool) cycleMap() {
	newMap := make(map[primitives.ValidatorIndex]*doublylinkedlist.Node[*qrysmpb.SignedMLDSA87ToExecutionChange])
	for k, v := range p.m {
		newMap[k] = v
	}
	p.m = newMap
}

// PendingMLDSA87ToExecChanges returns all objects from the pool.
func (p *Pool) PendingMLDSA87ToExecChanges() ([]*qrysmpb.SignedMLDSA87ToExecutionChange, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	result := make([]*qrysmpb.SignedMLDSA87ToExecutionChange, p.pending.Len())
	node := p.pending.First()
	var err error
	for i := 0; node != nil; i++ {
		result[i], err = node.Value()
		if err != nil {
			return nil, err
		}
		node, err = node.Next()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// MLDSA87ToExecChangesForInclusion returns objects that are ready for inclusion.
// This method will not return more than the block enforced MaxMLDSA87ToExecutionChanges.
func (p *Pool) MLDSA87ToExecChangesForInclusion(st state.ReadOnlyBeaconState) ([]*qrysmpb.SignedMLDSA87ToExecutionChange, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	length := int(math.Min(float64(params.BeaconConfig().MaxMLDSA87ToExecutionChanges), float64(p.pending.Len())))
	result := make([]*qrysmpb.SignedMLDSA87ToExecutionChange, 0, length)
	node := p.pending.Last()
	for node != nil && len(result) < length {
		change, err := node.Value()
		if err != nil {
			return nil, err
		}
		_, err = blocks.ValidateMLDSA87ToExecutionChange(st, change)
		if err != nil {
			logrus.WithError(err).Warning("removing invalid MLDSA87ToExecutionChange from pool")
			// MarkIncluded removes the invalid change from the pool
			p.lock.RUnlock()
			p.MarkIncluded(change)
			p.lock.RLock()
		} else {
			result = append(result, change)
		}
		node, err = node.Prev()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// InsertMLDSA87ToExecChange inserts an object into the pool.
func (p *Pool) InsertMLDSA87ToExecChange(change *qrysmpb.SignedMLDSA87ToExecutionChange) {
	p.lock.Lock()
	defer p.lock.Unlock()

	_, exists := p.m[change.Message.ValidatorIndex]
	if exists {
		return
	}

	p.pending.Append(doublylinkedlist.NewNode(change))
	p.m[change.Message.ValidatorIndex] = p.pending.Last()

	mlDSA87ToExecMessageInPoolTotal.Inc()
}

// MarkIncluded is used when an object has been included in a beacon block. Every block seen by this
// node should call this method to include the object. This will remove the object from the pool.
func (p *Pool) MarkIncluded(change *qrysmpb.SignedMLDSA87ToExecutionChange) {
	p.lock.Lock()
	defer p.lock.Unlock()

	node := p.m[change.Message.ValidatorIndex]
	if node == nil {
		return
	}

	delete(p.m, change.Message.ValidatorIndex)
	p.pending.Remove(node)
	if p.numPending() == mlDSA87ChangesPoolThreshold {
		p.cycleMap()
	}

	mlDSA87ToExecMessageInPoolTotal.Dec()
}

// ValidatorExists checks if the ml-dsa-87 to execution change object exists
// for that particular validator.
func (p *Pool) ValidatorExists(idx primitives.ValidatorIndex) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	node := p.m[idx]

	return node != nil
}

// numPending returns the number of pending ml-dsa-87 to execution changes in the pool
func (p *Pool) numPending() int {
	return p.pending.Len()
}
