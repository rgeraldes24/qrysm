package precompute

import (
	"github.com/pkg/errors"
	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/time/slots"
)

var errNilState = errors.New("nil state")

// UnrealizedCheckpoints returns the justification and finalization checkpoints of the
// given state as if it was progressed with empty slots until the next epoch. It
// also returns the total active balance during the epoch.
func UnrealizedCheckpoints(st state.BeaconState) (*zondpb.Checkpoint, *zondpb.Checkpoint, error) {
	if st == nil || st.IsNil() {
		return nil, nil, errNilState
	}

	if slots.ToEpoch(st.Slot()) <= params.BeaconConfig().GenesisEpoch+1 {
		jc := st.CurrentJustifiedCheckpoint()
		fc := st.FinalizedCheckpoint()
		return jc, fc, nil
	}

	activeBalance, prevTarget, currentTarget, err := st.UnrealizedCheckpointBalances()
	if err != nil {
		return nil, nil, err
	}

	justification := processJustificationBits(st, activeBalance, prevTarget, currentTarget)
	jc, fc, err := computeCheckpoints(st, justification)
	return jc, fc, err
}

// ProcessJustificationAndFinalizationPreCompute processes justification and finalization during
// epoch processing. This is where a beacon node can justify and finalize a new epoch.
// Note: this is an optimized version by passing in precomputed total and attesting balances.
func ProcessJustificationAndFinalizationPreCompute(state state.BeaconState, pBal *Balance) (state.BeaconState, error) {
	canProcessSlot, err := slots.EpochStart(2 /*epoch*/)
	if err != nil {
		return nil, err
	}
	if state.Slot() <= canProcessSlot {
		return state, nil
	}

	newBits := processJustificationBits(state, pBal.ActiveCurrentEpoch, pBal.PrevEpochTargetAttested, pBal.CurrentEpochTargetAttested)

	return weighJustificationAndFinalization(state, newBits)
}

// processJustificationBits processes the justification bits during epoch processing.
func processJustificationBits(state state.BeaconState, totalActiveBalance, prevEpochTargetBalance, currEpochTargetBalance uint64) bitfield.Bitvector4 {
	newBits := state.JustificationBits()
	newBits.Shift(1)
	// If 2/3 or more of total balance attested in the previous epoch.
	if 3*prevEpochTargetBalance >= 2*totalActiveBalance {
		newBits.SetBitAt(1, true)
	}

	if 3*currEpochTargetBalance >= 2*totalActiveBalance {
		newBits.SetBitAt(0, true)
	}

	return newBits
}

// weighJustificationAndFinalization processes justification and finalization during
// epoch processing. This is where a beacon node can justify and finalize a new epoch.
func weighJustificationAndFinalization(state state.BeaconState, newBits bitfield.Bitvector4) (state.BeaconState, error) {
	jc, fc, err := computeCheckpoints(state, newBits)
	if err != nil {
		return nil, err
	}

	if err := state.SetPreviousJustifiedCheckpoint(state.CurrentJustifiedCheckpoint()); err != nil {
		return nil, err
	}

	if err := state.SetCurrentJustifiedCheckpoint(jc); err != nil {
		return nil, err
	}

	if err := state.SetJustificationBits(newBits); err != nil {
		return nil, err
	}

	if err := state.SetFinalizedCheckpoint(fc); err != nil {
		return nil, err
	}
	return state, nil
}

// computeCheckpoints computes the new Justification and Finalization
// checkpoints at epoch transition
func computeCheckpoints(state state.BeaconState, newBits bitfield.Bitvector4) (*zondpb.Checkpoint, *zondpb.Checkpoint, error) {
	prevEpoch := time.PrevEpoch(state)
	currentEpoch := time.CurrentEpoch(state)
	oldPrevJustifiedCheckpoint := state.PreviousJustifiedCheckpoint()
	oldCurrJustifiedCheckpoint := state.CurrentJustifiedCheckpoint()

	justifiedCheckpoint := state.CurrentJustifiedCheckpoint()
	finalizedCheckpoint := state.FinalizedCheckpoint()

	// If 2/3 or more of the total balance attested in the current epoch.
	if newBits.BitAt(0) && currentEpoch >= justifiedCheckpoint.Epoch {
		blockRoot, err := helpers.BlockRoot(state, currentEpoch)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not get block root for current epoch %d", currentEpoch)
		}
		justifiedCheckpoint.Epoch = currentEpoch
		justifiedCheckpoint.Root = blockRoot
	} else if newBits.BitAt(1) && prevEpoch >= justifiedCheckpoint.Epoch {
		// If 2/3 or more of total balance attested in the previous epoch.
		blockRoot, err := helpers.BlockRoot(state, prevEpoch)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not get block root for previous epoch %d", prevEpoch)
		}
		justifiedCheckpoint.Epoch = prevEpoch
		justifiedCheckpoint.Root = blockRoot
	}

	// Process finalization according to Ethereum Beacon Chain specification.
	if len(newBits) == 0 {
		return nil, nil, errors.New("empty justification bits")
	}
	justification := newBits.Bytes()[0]

	// 2nd/3rd/4th (0b1110) most recent epochs are justified, the 2nd using the 4th as source.
	if justification&0x0E == 0x0E && (oldPrevJustifiedCheckpoint.Epoch+3) == currentEpoch {
		finalizedCheckpoint = oldPrevJustifiedCheckpoint
	}

	// 2nd/3rd (0b0110) most recent epochs are justified, the 2nd using the 3rd as source.
	if justification&0x06 == 0x06 && (oldPrevJustifiedCheckpoint.Epoch+2) == currentEpoch {
		finalizedCheckpoint = oldPrevJustifiedCheckpoint
	}

	// 1st/2nd/3rd (0b0111) most recent epochs are justified, the 1st using the 3rd as source.
	if justification&0x07 == 0x07 && (oldCurrJustifiedCheckpoint.Epoch+2) == currentEpoch {
		finalizedCheckpoint = oldCurrJustifiedCheckpoint
	}

	// The 1st/2nd (0b0011) most recent epochs are justified, the 1st using the 2nd as source
	if justification&0x03 == 0x03 && (oldCurrJustifiedCheckpoint.Epoch+1) == currentEpoch {
		finalizedCheckpoint = oldCurrJustifiedCheckpoint
	}
	return justifiedCheckpoint, finalizedCheckpoint, nil
}
