package transition

import (
	"context"

	"github.com/pkg/errors"
	b "github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/container/trie"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// PreminedGenesisBeaconState works almost exactly like GenesisBeaconState, except that it assumes that genesis deposits
// are not represented in the deposit contract and are only found in the genesis state validator registry. In order
// to ensure the deposit root and count match the empty deposit contract deployed in a testnet genesis block, the root
// of an empty deposit trie is computed and used as Eth1Data.deposit_root, and the deposit count is set to 0.
func PreminedGenesisBeaconState(ctx context.Context, deposits []*zondpb.Deposit, genesisTime uint64, eth1Data *zondpb.Eth1Data) (state.BeaconState, error) {
	st, err := EmptyGenesisState()
	if err != nil {
		return nil, err
	}

	// Process initial deposits.
	st, err = helpers.UpdateGenesisEth1Data(st, deposits, eth1Data)
	if err != nil {
		return nil, err
	}
	st, err = b.ProcessPreGenesisDeposits(ctx, st, deposits)
	if err != nil {
		return nil, errors.Wrap(err, "could not process validator deposits")
	}

	t, err := trie.NewTrie(params.BeaconConfig().DepositContractTreeDepth)
	if err != nil {
		return nil, err
	}
	dr, err := t.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	if err := st.SetEth1Data(&zondpb.Eth1Data{DepositRoot: dr[:], BlockHash: eth1Data.BlockHash}); err != nil {
		return nil, err
	}
	if err := st.SetEth1DepositIndex(0); err != nil {
		return nil, err
	}
	return OptimizedGenesisBeaconState(genesisTime, st, st.Eth1Data())
}

// IsValidGenesisState gets called whenever there's a deposit event,
// it checks whether there's enough effective balance to trigger and
// if the minimum genesis time arrived already.
//
// Spec pseudocode definition:
//
//	def is_valid_genesis_state(state: BeaconState) -> bool:
//	   if state.genesis_time < MIN_GENESIS_TIME:
//	       return False
//	   if len(get_active_validator_indices(state, GENESIS_EPOCH)) < MIN_GENESIS_ACTIVE_VALIDATOR_COUNT:
//	       return False
//	   return True
//
// This method has been modified from the spec to allow whole states not to be saved
// but instead only cache the relevant information.
func IsValidGenesisState(chainStartDepositCount, currentTime uint64) bool {
	if currentTime < params.BeaconConfig().MinGenesisTime {
		return false
	}
	if chainStartDepositCount < params.BeaconConfig().MinGenesisActiveValidatorCount {
		return false
	}
	return true
}
