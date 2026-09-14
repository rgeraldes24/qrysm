package params

import (
	"fmt"
	"math/bits"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
)

// MaxActiveValidators returns the largest active validator set that the
// current committee layout and attestation SSZ bounds can represent.
//
// Each epoch has SlotsPerEpoch * MaxCommitteesPerSlot committees, and each
// committee's aggregation bitlist is bounded by MaxValidatorsPerCommittee.
func (b *BeaconChainConfig) MaxActiveValidators() (uint64, error) {
	if b.MaxCommitteesPerSlot == 0 {
		return 0, fmt.Errorf("MAX_COMMITTEES_PER_SLOT must be non-zero")
	}
	if err := b.validateCommitteeSize(); err != nil {
		return 0, err
	}
	if b.SlotsPerEpoch == 0 {
		return 0, fmt.Errorf("SLOTS_PER_EPOCH must be non-zero")
	}

	hi, committeesPerEpoch := bits.Mul64(b.MaxCommitteesPerSlot, uint64(b.SlotsPerEpoch))
	if hi != 0 {
		return 0, fmt.Errorf("MAX_COMMITTEES_PER_SLOT * SLOTS_PER_EPOCH overflows uint64")
	}
	hi, maxActiveValidators := bits.Mul64(committeesPerEpoch, b.MaxValidatorsPerCommittee)
	if hi != 0 {
		return 0, fmt.Errorf("MAX_COMMITTEES_PER_SLOT * SLOTS_PER_EPOCH * MAX_VALIDATORS_PER_COMMITTEE overflows uint64")
	}
	return maxActiveValidators, nil
}

func (b *BeaconChainConfig) validateCommitteeSize() error {
	if b.MaxValidatorsPerCommittee == 0 {
		return fmt.Errorf("MAX_VALIDATORS_PER_COMMITTEE must be non-zero")
	}
	if b.MaxValidatorsPerCommittee > fieldparams.MaxValidatorsPerCommittee {
		return fmt.Errorf("MAX_VALIDATORS_PER_COMMITTEE (%d) must not exceed this binary's SSZ attestation limit (%d)",
			b.MaxValidatorsPerCommittee, fieldparams.MaxValidatorsPerCommittee)
	}
	return nil
}
