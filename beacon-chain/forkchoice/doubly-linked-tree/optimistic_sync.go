package doublylinkedtree

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/time/slots"
)

func (s *Store) setOptimisticToInvalid(ctx context.Context, root, parentRoot, lastValidHash [32]byte) ([][32]byte, error) {
	invalidRoots := make([][32]byte, 0)
	node, ok := s.nodeByRoot[root]
	if !ok {
		node, ok = s.nodeByRoot[parentRoot]
		if !ok || node == nil {
			return invalidRoots, errors.Wrap(ErrNilNode, "could not set node to invalid")
		}
		// return early if the parent is LVH
		if node.payloadHash == lastValidHash {
			return invalidRoots, nil
		}
	} else {
		if node == nil {
			return invalidRoots, errors.Wrap(ErrNilNode, "could not set node to invalid")
		}
		if node.parent == nil || node.parent.root != parentRoot {
			return invalidRoots, errInvalidParentRoot
		}
	}
	firstInvalid := node
	for ; firstInvalid.parent != nil && firstInvalid.parent.payloadHash != lastValidHash; firstInvalid = firstInvalid.parent {
		if ctx.Err() != nil {
			return invalidRoots, ctx.Err()
		}
	}
	// Deal with the case that the last valid payload is in a different fork
	// This means we are dealing with an EE that does not follow the spec
	if firstInvalid.parent == nil {
		// return early if the invalid node was not imported
		if node.root == parentRoot {
			return invalidRoots, nil
		}
		firstInvalid = node
	}
	invalidRoots, err := s.removeNode(ctx, firstInvalid)
	if err != nil {
		return invalidRoots, err
	}
	// NewPayload invalidation need not be followed by Head immediately. Update
	// the store's optimistic status now, without using bestDescendant links
	// that may still point into the removed subtree.
	justifiedNode := s.nodeByRoot[s.justifiedCheckpoint.Root]
	if justifiedNode == nil && s.justifiedCheckpoint.Epoch == params.BeaconConfig().GenesisEpoch {
		justifiedNode = s.treeRootNode
	}
	currentEpoch := slots.EpochsSinceGenesis(time.Unix(int64(s.genesisTime), 0))
	s.allTipsAreInvalid = !justifiedNode.hasViableTip(s.justifiedCheckpoint.Epoch, currentEpoch)
	return invalidRoots, nil
}

// hasViableTip searches the surviving tree after invalidation. Cached best
// descendants have not yet been recomputed, and an internal node is eligible
// only if one of its actual leaves is eligible.
func (n *Node) hasViableTip(justifiedEpoch, currentEpoch primitives.Epoch) bool {
	if n == nil {
		return false
	}
	if len(n.children) == 0 {
		return n.viableForHead(justifiedEpoch, currentEpoch)
	}
	for _, child := range n.children {
		if child.hasViableTip(justifiedEpoch, currentEpoch) {
			return true
		}
	}
	return false
}

// removeNode removes the node with the given root and all of its children
// from the Fork Choice Store.
func (s *Store) removeNode(ctx context.Context, node *Node) ([][32]byte, error) {
	invalidRoots := make([][32]byte, 0)

	if node == nil {
		return invalidRoots, errors.Wrap(ErrNilNode, "could not remove node")
	}
	if !node.optimistic || node.parent == nil {
		return invalidRoots, errInvalidOptimisticStatus
	}

	children := node.parent.children
	if len(children) == 1 {
		node.parent.children = []*Node{}
	} else {
		for i, n := range children {
			if n == node {
				if i != len(children)-1 {
					children[i] = children[len(children)-1]
				}
				node.parent.children = children[:len(children)-1]
				break
			}
		}
	}
	invalidRoots, err := s.removeNodeAndChildren(ctx, node, invalidRoots)
	if err != nil {
		return invalidRoots, err
	}
	s.recomputeUnrealizedCheckpoints()
	return invalidRoots, nil
}

// removeNodeAndChildren removes `node` and all of its descendant from the Store
func (s *Store) removeNodeAndChildren(ctx context.Context, node *Node, invalidRoots [][32]byte) ([][32]byte, error) {
	var err error
	for _, child := range node.children {
		if ctx.Err() != nil {
			return invalidRoots, ctx.Err()
		}
		if invalidRoots, err = s.removeNodeAndChildren(ctx, child, invalidRoots); err != nil {
			return invalidRoots, err
		}
	}
	invalidRoots = append(invalidRoots, node.root)
	if node.root == s.proposerBoostRoot {
		s.proposerBoostRoot = [32]byte{}
	}
	if node.root == s.previousProposerBoostRoot {
		s.previousProposerBoostRoot = params.BeaconConfig().ZeroHash
		s.previousProposerBoostScore = 0
	}
	delete(s.nodeByRoot, node.root)
	delete(s.nodeByPayload, node.payloadHash)
	return invalidRoots, nil
}
