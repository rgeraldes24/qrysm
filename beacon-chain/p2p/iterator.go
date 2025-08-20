package p2p

import (
	"context"

	"github.com/theQRL/go-zond/p2p/qnode"
)

// filterNodes wraps an iterator such that Next only returns nodes for which
// the 'check' function returns true. This custom implementation also
// checks for context deadlines so that in the event the parent context has
// expired, we do exit from the search rather than  perform more network
// lookups for additional peers.
func filterNodes(ctx context.Context, it qnode.Iterator, check func(*qnode.Node) bool) qnode.Iterator {
	return &filterIter{ctx, it, check}
}

type filterIter struct {
	context.Context
	qnode.Iterator
	check func(*qnode.Node) bool
}

// Next looks up for the next valid node according to our
// filter criteria.
func (f *filterIter) Next() bool {
	for f.Iterator.Next() {
		if f.Context.Err() != nil {
			return false
		}
		if f.check(f.Node()) {
			return true
		}
	}
	return false
}
