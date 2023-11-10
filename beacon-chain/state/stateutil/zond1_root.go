package stateutil

import (
	"bytes"
	"encoding/binary"

	"github.com/pkg/errors"
	params "github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/encoding/ssz"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// Zond1DataRootWithHasher returns the hash tree root of input `zond1Data`.
func Zond1DataRootWithHasher(zond1Data *zondpb.Zond1Data) ([32]byte, error) {
	if zond1Data == nil {
		return [32]byte{}, errors.New("nil zond1 data")
	}

	fieldRoots := make([][32]byte, 3)
	for i := 0; i < len(fieldRoots); i++ {
		fieldRoots[i] = [32]byte{}
	}

	if len(zond1Data.DepositRoot) > 0 {
		fieldRoots[0] = bytesutil.ToBytes32(zond1Data.DepositRoot)
	}

	zond1DataCountBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(zond1DataCountBuf, zond1Data.DepositCount)
	fieldRoots[1] = bytesutil.ToBytes32(zond1DataCountBuf)
	if len(zond1Data.BlockHash) > 0 {
		fieldRoots[2] = bytesutil.ToBytes32(zond1Data.BlockHash)
	}
	root, err := ssz.BitwiseMerkleize(fieldRoots, uint64(len(fieldRoots)), uint64(len(fieldRoots)))
	if err != nil {
		return [32]byte{}, err
	}
	return root, nil
}

// Zond1DatasRoot returns the hash tree root of input `zond1Datas`.
func Zond1DatasRoot(zond1Datas []*zondpb.Zond1Data) ([32]byte, error) {
	zond1VotesRoots := make([][32]byte, 0, len(zond1Datas))
	for i := 0; i < len(zond1Datas); i++ {
		zond1, err := Zond1DataRootWithHasher(zond1Datas[i])
		if err != nil {
			return [32]byte{}, errors.Wrap(err, "could not compute zond1data merkleization")
		}
		zond1VotesRoots = append(zond1VotesRoots, zond1)
	}

	zond1VotesRootsRoot, err := ssz.BitwiseMerkleize(zond1VotesRoots, uint64(len(zond1VotesRoots)), params.BeaconConfig().Zond1DataVotesLength())
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "could not compute zond1data votes merkleization")
	}
	zond1VotesRootBuf := new(bytes.Buffer)
	if err := binary.Write(zond1VotesRootBuf, binary.LittleEndian, uint64(len(zond1Datas))); err != nil {
		return [32]byte{}, errors.Wrap(err, "could not marshal zond1data votes length")
	}
	// We need to mix in the length of the slice.
	zond1VotesRootBufRoot := make([]byte, 32)
	copy(zond1VotesRootBufRoot, zond1VotesRootBuf.Bytes())
	root := ssz.MixInLength(zond1VotesRootsRoot, zond1VotesRootBufRoot)

	return root, nil
}
