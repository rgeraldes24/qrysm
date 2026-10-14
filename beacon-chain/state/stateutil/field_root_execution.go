package stateutil

import (
	"github.com/pkg/errors"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// ExecutionRoot computes the HashTreeRoot Merkleization of
// an ExecutionData struct according to the Simple Serialize (SSZ)
// specification.
func ExecutionRoot(executionData *qrysmpb.ExecutionData) ([32]byte, error) {
	if executionData == nil {
		return [32]byte{}, errors.New("nil execution data")
	}
	return ExecutionDataRootWithHasher(executionData)
}

// ExecutionDataVotesRoot computes the HashTreeRoot Merkleization of
// a list of ExecutionData structs according to the Simple Serialize (SSZ)
// specification.
func ExecutionDataVotesRoot(executionDataVotes []*qrysmpb.ExecutionData) ([32]byte, error) {
	return ExecutionDatasRoot(executionDataVotes)
}
