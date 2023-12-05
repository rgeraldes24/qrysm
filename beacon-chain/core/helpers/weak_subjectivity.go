package helpers

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/math"
	v1alpha1 "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/time/slots"
)

// ComputeWeakSubjectivityPeriod returns weak subjectivity period for the active validator count and finalized epoch.
func ComputeWeakSubjectivityPeriod(ctx context.Context, st state.ReadOnlyBeaconState, cfg *params.BeaconChainConfig) (primitives.Epoch, error) {
	// Weak subjectivity period cannot be smaller than withdrawal delay.
	wsp := uint64(cfg.MinValidatorWithdrawabilityDelay)

	// Cardinality of active validator set.
	N, err := ActiveValidatorCount(ctx, st, time.CurrentEpoch(st))
	if err != nil {
		return 0, fmt.Errorf("cannot obtain active valiadtor count: %w", err)
	}
	if N == 0 {
		return 0, errors.New("no active validators found")
	}

	// Average effective balance in the given validator set, in Ether.
	t, err := TotalActiveBalance(st)
	if err != nil {
		return 0, fmt.Errorf("cannot find total active balance of validators: %w", err)
	}
	t = t / N / cfg.GweiPerEth

	// Maximum effective balance per validator.
	T := cfg.MaxEffectiveBalance / cfg.GweiPerEth

	// Validator churn limit.
	delta, err := ValidatorChurnLimit(N)
	if err != nil {
		return 0, fmt.Errorf("cannot obtain active validator churn limit: %w", err)
	}

	// Balance top-ups.
	Delta := uint64(cfg.SlotsPerEpoch.Mul(cfg.MaxDeposits))

	if delta == 0 || Delta == 0 {
		return 0, errors.New("either validator churn limit or balance top-ups is zero")
	}

	// Safety decay, maximum tolerable loss of safety margin of FFG finality.
	D := cfg.SafetyDecay

	if T*(200+3*D) < t*(200+12*D) {
		epochsForValidatorSetChurn := N * (t*(200+12*D) - T*(200+3*D)) / (600 * delta * (2*t + T))
		epochsForBalanceTopUps := N * (200 + 3*D) / (600 * Delta)
		wsp += math.Max(epochsForValidatorSetChurn, epochsForBalanceTopUps)
	} else {
		wsp += 3 * N * D * t / (200 * Delta * (T - t))
	}

	return primitives.Epoch(wsp), nil
}

// IsWithinWeakSubjectivityPeriod verifies if a given weak subjectivity checkpoint is not stale i.e.
// the current node is so far beyond, that a given state and checkpoint are not for the latest weak
// subjectivity point. Provided checkpoint still can be used to double-check that node's block root
// at a given epoch matches that of the checkpoint.
func IsWithinWeakSubjectivityPeriod(
	ctx context.Context, currentEpoch primitives.Epoch, wsState state.ReadOnlyBeaconState, wsStateRoot [fieldparams.RootLength]byte, wsEpoch primitives.Epoch, cfg *params.BeaconChainConfig) (bool, error) {
	// Make sure that incoming objects are not nil.
	if wsState == nil || wsState.IsNil() || wsState.LatestBlockHeader() == nil {
		return false, errors.New("invalid weak subjectivity state or checkpoint")
	}

	// Assert that state and checkpoint have the same root and epoch.
	if bytesutil.ToBytes32(wsState.LatestBlockHeader().StateRoot) != wsStateRoot {
		return false, fmt.Errorf("state (%#x) and checkpoint (%#x) roots do not match",
			wsState.LatestBlockHeader().StateRoot, wsStateRoot)
	}
	if slots.ToEpoch(wsState.Slot()) != wsEpoch {
		return false, fmt.Errorf("state (%v) and checkpoint (%v) epochs do not match",
			slots.ToEpoch(wsState.Slot()), wsEpoch)
	}

	// Compare given epoch to state epoch + weak subjectivity period.
	wsPeriod, err := ComputeWeakSubjectivityPeriod(ctx, wsState, cfg)
	if err != nil {
		return false, fmt.Errorf("cannot compute weak subjectivity period: %w", err)
	}
	wsStateEpoch := slots.ToEpoch(wsState.Slot())

	return currentEpoch <= wsStateEpoch+wsPeriod, nil
}

// LatestWeakSubjectivityEpoch returns epoch of the most recent weak subjectivity checkpoint known to a node.
//
// Within the weak subjectivity period, if two conflicting blocks are finalized, 1/3 - D (D := safety decay)
// of validators will get slashed. Therefore, it is safe to assume that any finalized checkpoint within that
// period is protected by this safety margin.
func LatestWeakSubjectivityEpoch(ctx context.Context, st state.ReadOnlyBeaconState, cfg *params.BeaconChainConfig) (primitives.Epoch, error) {
	wsPeriod, err := ComputeWeakSubjectivityPeriod(ctx, st, cfg)
	if err != nil {
		return 0, err
	}

	finalizedEpoch := st.FinalizedCheckpointEpoch()
	return finalizedEpoch - (finalizedEpoch % wsPeriod), nil
}

// ParseWeakSubjectivityInputString parses "blocks_root:epoch_number" string into a checkpoint.
func ParseWeakSubjectivityInputString(wsCheckpointString string) (*v1alpha1.Checkpoint, error) {
	if wsCheckpointString == "" {
		return nil, nil
	}

	// Weak subjectivity input string must contain ":" to separate epoch and block root.
	if !strings.Contains(wsCheckpointString, ":") {
		return nil, fmt.Errorf("%s did not contain column", wsCheckpointString)
	}

	// Strip prefix "0x" if it's part of the input string.
	wsCheckpointString = strings.TrimPrefix(wsCheckpointString, "0x")

	// Get the hexadecimal block root from input string.
	s := strings.Split(wsCheckpointString, ":")
	if len(s) != 2 {
		return nil, errors.New("weak subjectivity checkpoint input should be in `block_root:epoch_number` format")
	}

	bRoot, err := hex.DecodeString(s[0])
	if err != nil {
		return nil, err
	}
	if len(bRoot) != 32 {
		return nil, errors.New("block root is not length of 32")
	}

	// Get the epoch number from input string.
	epoch, err := strconv.ParseUint(s[1], 10, 64)
	if err != nil {
		return nil, err
	}

	return &v1alpha1.Checkpoint{
		Epoch: primitives.Epoch(epoch),
		Root:  bRoot,
	}, nil
}
