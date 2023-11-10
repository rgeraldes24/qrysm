package helpers

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/container/trie"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// UpdateGenesisZond1Data updates zond1 data for genesis state.
func UpdateGenesisZond1Data(state state.BeaconState, deposits []*zondpb.Deposit, zond1Data *zondpb.Zond1Data) (state.BeaconState, error) {
	if zond1Data == nil {
		return nil, errors.New("no zond1data provided for genesis state")
	}

	leaves := make([][]byte, 0, len(deposits))
	for _, deposit := range deposits {
		if deposit == nil || deposit.Data == nil {
			return nil, fmt.Errorf("nil deposit or deposit with nil data cannot be processed: %v", deposit)
		}
		hash, err := deposit.Data.HashTreeRoot()
		if err != nil {
			return nil, err
		}
		leaves = append(leaves, hash[:])
	}
	var t *trie.SparseMerkleTrie
	var err error
	if len(leaves) > 0 {
		t, err = trie.GenerateTrieFromItems(leaves, params.BeaconConfig().DepositContractTreeDepth)
		if err != nil {
			return nil, err
		}
	} else {
		t, err = trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
		if err != nil {
			return nil, err
		}
	}

	depositRoot, err := t.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	zond1Data.DepositRoot = depositRoot[:]
	err = state.SetZond1Data(zond1Data)
	if err != nil {
		return nil, err
	}
	return state, nil
}
