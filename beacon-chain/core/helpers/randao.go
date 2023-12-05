package helpers

import (
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/crypto/hash"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
)

// Seed returns the randao seed used for shuffling of a given epoch.
func Seed(state state.ReadOnlyBeaconState, epoch primitives.Epoch, domain [dilithium.DomainByteLength]byte) ([32]byte, error) {
	// See https://github.com/ethereum/consensus-specs/pull/1296 for
	// rationale on why offset has to look down by 1.
	lookAheadEpoch := epoch + params.BeaconConfig().EpochsPerHistoricalVector -
		params.BeaconConfig().MinSeedLookahead - 1

	randaoMix, err := RandaoMix(state, lookAheadEpoch)
	if err != nil {
		return [32]byte{}, err
	}
	seed := append(domain[:], bytesutil.Bytes8(uint64(epoch))...)
	seed = append(seed, randaoMix...)

	seed32 := hash.Hash(seed)

	return seed32, nil
}

// RandaoMix returns the randao mix (xor'ed seed)
// of a given slot. It is used to shuffle validators.
func RandaoMix(state state.ReadOnlyBeaconState, epoch primitives.Epoch) ([]byte, error) {
	return state.RandaoMixAtIndex(uint64(epoch % params.BeaconConfig().EpochsPerHistoricalVector))
}
