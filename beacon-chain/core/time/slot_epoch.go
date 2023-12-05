package time

import (
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/time/slots"
)

// CurrentEpoch returns the current epoch number calculated from
// the slot number stored in beacon state.
func CurrentEpoch(state state.ReadOnlyBeaconState) primitives.Epoch {
	return slots.ToEpoch(state.Slot())
}

// PrevEpoch returns the previous epoch number calculated from
// the slot number stored in beacon state. It also checks for
// underflow condition.
func PrevEpoch(state state.ReadOnlyBeaconState) primitives.Epoch {
	currentEpoch := CurrentEpoch(state)
	if currentEpoch == 0 {
		return 0
	}
	return currentEpoch - 1
}

// NextEpoch returns the next epoch number calculated from
// the slot number stored in beacon state.
func NextEpoch(state state.ReadOnlyBeaconState) primitives.Epoch {
	return slots.ToEpoch(state.Slot()) + 1
}

// CanProcessEpoch checks the eligibility to process epoch.
// The epoch can be processed at the end of the last slot of every epoch.
func CanProcessEpoch(state state.ReadOnlyBeaconState) bool {
	return (state.Slot()+1)%params.BeaconConfig().SlotsPerEpoch == 0
}
