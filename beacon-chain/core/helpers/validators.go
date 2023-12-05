package helpers

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/v4/beacon-chain/cache"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/hash"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/time/slots"
	"go.opencensus.io/trace"
)

var CommitteeCacheInProgressHit = promauto.NewCounter(prometheus.CounterOpts{
	Name: "committee_cache_in_progress_hit",
	Help: "The number of committee requests that are present in the cache.",
})

// IsActiveValidator returns the boolean value on whether the validator
// is active or not.
func IsActiveValidator(validator *zondpb.Validator, epoch primitives.Epoch) bool {
	return checkValidatorActiveStatus(validator.ActivationEpoch, validator.ExitEpoch, epoch)
}

// IsActiveValidatorUsingTrie checks if a read only validator is active.
func IsActiveValidatorUsingTrie(validator state.ReadOnlyValidator, epoch primitives.Epoch) bool {
	return checkValidatorActiveStatus(validator.ActivationEpoch(), validator.ExitEpoch(), epoch)
}

// IsActiveNonSlashedValidatorUsingTrie checks if a read only validator is active and not slashed
func IsActiveNonSlashedValidatorUsingTrie(validator state.ReadOnlyValidator, epoch primitives.Epoch) bool {
	active := checkValidatorActiveStatus(validator.ActivationEpoch(), validator.ExitEpoch(), epoch)
	return active && !validator.Slashed()
}

func checkValidatorActiveStatus(activationEpoch, exitEpoch, epoch primitives.Epoch) bool {
	return activationEpoch <= epoch && epoch < exitEpoch
}

// IsSlashableValidator returns the boolean value on whether the validator
// is slashable or not.
func IsSlashableValidator(activationEpoch, withdrawableEpoch primitives.Epoch, slashed bool, epoch primitives.Epoch) bool {
	return checkValidatorSlashable(activationEpoch, withdrawableEpoch, slashed, epoch)
}

// IsSlashableValidatorUsingTrie checks if a read only validator is slashable.
func IsSlashableValidatorUsingTrie(val state.ReadOnlyValidator, epoch primitives.Epoch) bool {
	return checkValidatorSlashable(val.ActivationEpoch(), val.WithdrawableEpoch(), val.Slashed(), epoch)
}

func checkValidatorSlashable(activationEpoch, withdrawableEpoch primitives.Epoch, slashed bool, epoch primitives.Epoch) bool {
	active := activationEpoch <= epoch
	beforeWithdrawable := epoch < withdrawableEpoch
	return beforeWithdrawable && active && !slashed
}

// ActiveValidatorIndices filters out active validators based on validator status
// and returns their indices in a list.
//
// WARNING: This method allocates a new copy of the validator index set and is
// considered to be very memory expensive. Avoid using this unless you really
// need the active validator indices for some specific reason.
func ActiveValidatorIndices(ctx context.Context, s state.ReadOnlyBeaconState, epoch primitives.Epoch) ([]primitives.ValidatorIndex, error) {
	seed, err := Seed(s, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return nil, errors.Wrap(err, "could not get seed")
	}
	activeIndices, err := committeeCache.ActiveIndices(ctx, seed)
	if err != nil {
		return nil, errors.Wrap(err, "could not interface with committee cache")
	}
	if activeIndices != nil {
		return activeIndices, nil
	}

	if err := committeeCache.MarkInProgress(seed); err != nil {
		if errors.Is(err, cache.ErrAlreadyInProgress) {
			activeIndices, err := committeeCache.ActiveIndices(ctx, seed)
			if err != nil {
				return nil, err
			}
			if activeIndices == nil {
				return nil, errors.New("nil active indices")
			}
			CommitteeCacheInProgressHit.Inc()
			return activeIndices, nil
		}
		return nil, errors.Wrap(err, "could not mark committee cache as in progress")
	}
	defer func() {
		if err := committeeCache.MarkNotInProgress(seed); err != nil {
			log.WithError(err).Error("Could not mark cache not in progress")
		}
	}()

	var indices []primitives.ValidatorIndex
	if err := s.ReadFromEveryValidator(func(idx int, val state.ReadOnlyValidator) error {
		if IsActiveValidatorUsingTrie(val, epoch) {
			indices = append(indices, primitives.ValidatorIndex(idx))
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := UpdateCommitteeCache(ctx, s, epoch); err != nil {
		return nil, errors.Wrap(err, "could not update committee cache")
	}

	return indices, nil
}

// ActiveValidatorCount returns the number of active validators in the state
// at the given epoch.
func ActiveValidatorCount(ctx context.Context, s state.ReadOnlyBeaconState, epoch primitives.Epoch) (uint64, error) {
	seed, err := Seed(s, epoch, params.BeaconConfig().DomainBeaconAttester)
	if err != nil {
		return 0, errors.Wrap(err, "could not get seed")
	}
	activeCount, err := committeeCache.ActiveIndicesCount(ctx, seed)
	if err != nil {
		return 0, errors.Wrap(err, "could not interface with committee cache")
	}
	if activeCount != 0 && s.Slot() != 0 {
		return uint64(activeCount), nil
	}

	if err := committeeCache.MarkInProgress(seed); err != nil {
		if errors.Is(err, cache.ErrAlreadyInProgress) {
			activeCount, err := committeeCache.ActiveIndicesCount(ctx, seed)
			if err != nil {
				return 0, err
			}
			CommitteeCacheInProgressHit.Inc()
			return uint64(activeCount), nil
		}
		return 0, errors.Wrap(err, "could not mark committee cache as in progress")
	}
	defer func() {
		if err := committeeCache.MarkNotInProgress(seed); err != nil {
			log.WithError(err).Error("Could not mark cache not in progress")
		}
	}()

	count := uint64(0)
	if err := s.ReadFromEveryValidator(func(idx int, val state.ReadOnlyValidator) error {
		if IsActiveValidatorUsingTrie(val, epoch) {
			count++
		}
		return nil
	}); err != nil {
		return 0, err
	}

	if err := UpdateCommitteeCache(ctx, s, epoch); err != nil {
		return 0, errors.Wrap(err, "could not update committee cache")
	}

	return count, nil
}

// ActivationExitEpoch takes in epoch number and returns when
// the validator is eligible for activation and exit.
func ActivationExitEpoch(epoch primitives.Epoch) primitives.Epoch {
	return epoch + 1 + params.BeaconConfig().MaxSeedLookahead
}

// ValidatorChurnLimit returns the number of validators that are allowed to
// enter and exit validator pool for an epoch.
func ValidatorChurnLimit(activeValidatorCount uint64) (uint64, error) {
	churnLimit := activeValidatorCount / params.BeaconConfig().ChurnLimitQuotient
	if churnLimit < params.BeaconConfig().MinPerEpochChurnLimit {
		churnLimit = params.BeaconConfig().MinPerEpochChurnLimit
	}
	return churnLimit, nil
}

// BeaconProposerIndex returns proposer index of a current slot.
func BeaconProposerIndex(ctx context.Context, state state.ReadOnlyBeaconState) (primitives.ValidatorIndex, error) {
	e := time.CurrentEpoch(state)
	// The cache uses the state root of the previous epoch - minimum_seed_lookahead last slot as key. (e.g. Starting epoch 1, slot 32, the key would be block root at slot 31)
	// For simplicity, the node will skip caching of genesis epoch.
	if e > params.BeaconConfig().GenesisEpoch+params.BeaconConfig().MinSeedLookahead {
		wantedEpoch := time.PrevEpoch(state)
		s, err := slots.EpochEnd(wantedEpoch)
		if err != nil {
			return 0, err
		}
		r, err := StateRootAtSlot(state, s)
		if err != nil {
			return 0, err
		}
		if r != nil && !bytes.Equal(r, params.BeaconConfig().ZeroHash[:]) {
			proposerIndices, err := proposerIndicesCache.ProposerIndices(bytesutil.ToBytes32(r))
			if err != nil {
				return 0, errors.Wrap(err, "could not interface with committee cache")
			}
			if proposerIndices != nil {
				if len(proposerIndices) != int(params.BeaconConfig().SlotsPerEpoch) {
					return 0, errors.Errorf("length of proposer indices is not equal %d to slots per epoch", len(proposerIndices))
				}
				return proposerIndices[state.Slot()%params.BeaconConfig().SlotsPerEpoch], nil
			}
			if err := UpdateProposerIndicesInCache(ctx, state, time.CurrentEpoch(state)); err != nil {
				return 0, errors.Wrap(err, "could not update committee cache")
			}
		}
	}

	seed, err := Seed(state, e, params.BeaconConfig().DomainBeaconProposer)
	if err != nil {
		return 0, errors.Wrap(err, "could not generate seed")
	}

	seedWithSlot := append(seed[:], bytesutil.Bytes8(uint64(state.Slot()))...)
	seedWithSlotHash := hash.Hash(seedWithSlot)

	indices, err := ActiveValidatorIndices(ctx, state, e)
	if err != nil {
		return 0, errors.Wrap(err, "could not get active indices")
	}

	return ComputeProposerIndex(state, indices, seedWithSlotHash)
}

// ComputeProposerIndex returns the index sampled by effective balance, which is used to calculate proposer.
func ComputeProposerIndex(bState state.ReadOnlyValidators, activeIndices []primitives.ValidatorIndex, seed [32]byte) (primitives.ValidatorIndex, error) {
	length := uint64(len(activeIndices))
	if length == 0 {
		return 0, errors.New("empty active indices list")
	}
	maxRandomByte := uint64(1<<8 - 1)
	hashFunc := hash.CustomSHA256Hasher()

	for i := uint64(0); ; i++ {
		candidateIndex, err := ComputeShuffledIndex(primitives.ValidatorIndex(i%length), length, seed, true /* shuffle */)
		if err != nil {
			return 0, err
		}
		candidateIndex = activeIndices[candidateIndex]
		if uint64(candidateIndex) >= uint64(bState.NumValidators()) {
			return 0, errors.New("active index out of range")
		}
		b := append(seed[:], bytesutil.Bytes8(i/32)...)
		randomByte := hashFunc(b)[i%32]
		v, err := bState.ValidatorAtIndexReadOnly(candidateIndex)
		if err != nil {
			return 0, err
		}
		effectiveBal := v.EffectiveBalance()

		if effectiveBal*maxRandomByte >= params.BeaconConfig().MaxEffectiveBalance*uint64(randomByte) {
			return candidateIndex, nil
		}
	}
}

// IsEligibleForActivationQueue checks if the validator is eligible to
// be placed into the activation queue.
func IsEligibleForActivationQueue(validator *zondpb.Validator) bool {
	return isEligibileForActivationQueue(validator.ActivationEligibilityEpoch, validator.EffectiveBalance)
}

// IsEligibleForActivationQueueUsingTrie checks if the read-only validator is eligible to
// be placed into the activation queue.
func IsEligibleForActivationQueueUsingTrie(validator state.ReadOnlyValidator) bool {
	return isEligibileForActivationQueue(validator.ActivationEligibilityEpoch(), validator.EffectiveBalance())
}

// isEligibleForActivationQueue carries out the logic for IsEligibleForActivationQueue*
func isEligibileForActivationQueue(activationEligibilityEpoch primitives.Epoch, effectiveBalance uint64) bool {
	return activationEligibilityEpoch == params.BeaconConfig().FarFutureEpoch &&
		effectiveBalance == params.BeaconConfig().MaxEffectiveBalance
}

// IsEligibleForActivation checks if the validator is eligible for activation.
func IsEligibleForActivation(state state.ReadOnlyCheckpoint, validator *zondpb.Validator) bool {
	finalizedEpoch := state.FinalizedCheckpointEpoch()
	return isEligibleForActivation(validator.ActivationEligibilityEpoch, validator.ActivationEpoch, finalizedEpoch)
}

// IsEligibleForActivationUsingTrie checks if the validator is eligible for activation.
func IsEligibleForActivationUsingTrie(state state.ReadOnlyCheckpoint, validator state.ReadOnlyValidator) bool {
	cpt := state.FinalizedCheckpoint()
	if cpt == nil {
		return false
	}
	return isEligibleForActivation(validator.ActivationEligibilityEpoch(), validator.ActivationEpoch(), cpt.Epoch)
}

// isEligibleForActivation carries out the logic for IsEligibleForActivation*
func isEligibleForActivation(activationEligibilityEpoch, activationEpoch, finalizedEpoch primitives.Epoch) bool {
	return activationEligibilityEpoch <= finalizedEpoch &&
		activationEpoch == params.BeaconConfig().FarFutureEpoch
}

// LastActivatedValidatorIndex provides the last activated validator given a state
func LastActivatedValidatorIndex(ctx context.Context, st state.ReadOnlyBeaconState) (primitives.ValidatorIndex, error) {
	_, span := trace.StartSpan(ctx, "helpers.LastActivatedValidatorIndex")
	defer span.End()
	var lastActivatedvalidatorIndex primitives.ValidatorIndex
	// linear search because status are not sorted
	for j := st.NumValidators() - 1; j >= 0; j-- {
		val, err := st.ValidatorAtIndexReadOnly(primitives.ValidatorIndex(j))
		if err != nil {
			return 0, err
		}
		if IsActiveValidatorUsingTrie(val, time.CurrentEpoch(st)) {
			lastActivatedvalidatorIndex = primitives.ValidatorIndex(j)
			break
		}
	}
	return lastActivatedvalidatorIndex, nil
}
