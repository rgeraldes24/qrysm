package stateutil

import (
	"bytes"
	"encoding/binary"

	"github.com/pkg/errors"
	params "github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/encoding/ssz"
	zondpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

// ExecutionNodeDataRootWithHasher returns the hash tree root of input `executionNodeData`.
func ExecutionNodeDataRootWithHasher(executionNodeData *zondpb.ExecutionNodeData) ([32]byte, error) {
	if executionNodeData == nil {
		return [32]byte{}, errors.New("nil eth1 data")
	}

	fieldRoots := make([][32]byte, 3)
	for i := 0; i < len(fieldRoots); i++ {
		fieldRoots[i] = [32]byte{}
	}

	if len(executionNodeData.DepositRoot) > 0 {
		fieldRoots[0] = bytesutil.ToBytes32(executionNodeData.DepositRoot)
	}

	executionNodeDataCountBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(executionNodeDataCountBuf, executionNodeData.DepositCount)
	fieldRoots[1] = bytesutil.ToBytes32(executionNodeDataCountBuf)
	if len(executionNodeData.BlockHash) > 0 {
		fieldRoots[2] = bytesutil.ToBytes32(executionNodeData.BlockHash)
	}
	root, err := ssz.BitwiseMerkleize(fieldRoots, uint64(len(fieldRoots)), uint64(len(fieldRoots)))
	if err != nil {
		return [32]byte{}, err
	}
	return root, nil
}

// ExecutionNodeDatasRoot returns the hash tree root of input `executionNodeDatas`.
func ExecutionNodeDatasRoot(executionNodeDatas []*zondpb.ExecutionNodeData) ([32]byte, error) {
	eth1VotesRoots := make([][32]byte, 0, len(executionNodeDatas))
	for i := 0; i < len(executionNodeDatas); i++ {
		eth1, err := ExecutionNodeDataRootWithHasher(executionNodeDatas[i])
		if err != nil {
			return [32]byte{}, errors.Wrap(err, "could not compute executionNodeData merkleization")
		}
		eth1VotesRoots = append(eth1VotesRoots, eth1)
	}

	eth1VotesRootsRoot, err := ssz.BitwiseMerkleize(eth1VotesRoots, uint64(len(eth1VotesRoots)), params.BeaconConfig().ExecutionNodeDataVotesLength())
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute executionNodeData votes merkleization")
	}
	eth1VotesRootBuf := new(bytes.Buffer)
	if err := binary.Write(eth1VotesRootBuf, binary.LittleEndian, uint64(len(executionNodeDatas))); err != nil {
		return [32]byte{}, errors.Wrap(err, "could not marshal executionNodeData votes length")
	}
	// We need to mix in the length of the slice.
	eth1VotesRootBufRoot := make([]byte, 32)
	copy(eth1VotesRootBufRoot, eth1VotesRootBuf.Bytes())
	root := ssz.MixInLength(eth1VotesRootsRoot, eth1VotesRootBufRoot)

	return root, nil
}
