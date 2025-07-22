package stateutil

import (
	"github.com/pkg/errors"
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// Eth1Root computes the HashTreeRoot Merkleization of
// a BeaconBlockHeader struct according to the eth2
// Simple Serialize specification.
func Eth1Root(executionNodeData *zondpb.ExecutionNodeData) ([32]byte, error) {
	if executionNodeData == nil {
		return [32]byte{}, errors.New("nil eth1 data")
	}
	return ExecutionNodeDataRootWithHasher(executionNodeData)
}

// ExecutionNodeDataVotesRoot computes the HashTreeRoot Merkleization of
// a list of ExecutionNodeData structs according to the eth2
// Simple Serialize specification.
func ExecutionNodeDataVotesRoot(executionNodeDataVotes []*zondpb.ExecutionNodeData) ([32]byte, error) {
	return ExecutionNodeDatasRoot(executionNodeDataVotes)
}
