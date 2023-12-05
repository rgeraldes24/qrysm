package blocks

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/time"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"github.com/theQRL/qrysm/v4/time/slots"
	"google.golang.org/protobuf/proto"
)

type slashValidatorFunc func(ctx context.Context, st state.BeaconState, vid primitives.ValidatorIndex, penaltyQuotient, proposerRewardQuotient uint64) (state.BeaconState, error)

// ProcessProposerSlashings is one of the operations performed
// on each processed beacon block to slash proposers based on
// slashing conditions if any slashable events occurred.
func ProcessProposerSlashings(
	ctx context.Context,
	beaconState state.BeaconState,
	slashings []*zondpb.ProposerSlashing,
	slashFunc slashValidatorFunc,
) (state.BeaconState, error) {
	var err error
	for _, slashing := range slashings {
		beaconState, err = ProcessProposerSlashing(ctx, beaconState, slashing, slashFunc)
		if err != nil {
			return nil, err
		}
	}
	return beaconState, nil
}

// ProcessProposerSlashing processes individual proposer slashing.
func ProcessProposerSlashing(
	ctx context.Context,
	beaconState state.BeaconState,
	slashing *zondpb.ProposerSlashing,
	slashFunc slashValidatorFunc,
) (state.BeaconState, error) {
	var err error
	if slashing == nil {
		return nil, errors.New("nil proposer slashings in block body")
	}
	if err = VerifyProposerSlashing(beaconState, slashing); err != nil {
		return nil, errors.Wrap(err, "could not verify proposer slashing")
	}
	cfg := params.BeaconConfig()
	var slashingQuotient uint64
	switch {
	case beaconState.Version() == version.Capella:
		slashingQuotient = cfg.MinSlashingPenaltyQuotient
	default:
		return nil, errors.New("unknown state version")
	}
	beaconState, err = slashFunc(ctx, beaconState, slashing.Header_1.Header.ProposerIndex, slashingQuotient, cfg.ProposerRewardQuotient)
	if err != nil {
		return nil, errors.Wrapf(err, "could not slash proposer index %d", slashing.Header_1.Header.ProposerIndex)
	}
	return beaconState, nil
}

// VerifyProposerSlashing verifies that the data provided from slashing is valid.
func VerifyProposerSlashing(
	beaconState state.ReadOnlyBeaconState,
	slashing *zondpb.ProposerSlashing,
) error {
	if slashing.Header_1 == nil || slashing.Header_1.Header == nil || slashing.Header_2 == nil || slashing.Header_2.Header == nil {
		return errors.New("nil header cannot be verified")
	}
	hSlot := slashing.Header_1.Header.Slot
	if hSlot != slashing.Header_2.Header.Slot {
		return fmt.Errorf("mismatched header slots, received %d == %d", slashing.Header_1.Header.Slot, slashing.Header_2.Header.Slot)
	}
	pIdx := slashing.Header_1.Header.ProposerIndex
	if pIdx != slashing.Header_2.Header.ProposerIndex {
		return fmt.Errorf("mismatched indices, received %d == %d", slashing.Header_1.Header.ProposerIndex, slashing.Header_2.Header.ProposerIndex)
	}
	if proto.Equal(slashing.Header_1.Header, slashing.Header_2.Header) {
		return errors.New("expected slashing headers to differ")
	}
	proposer, err := beaconState.ValidatorAtIndexReadOnly(slashing.Header_1.Header.ProposerIndex)
	if err != nil {
		return err
	}
	if !helpers.IsSlashableValidatorUsingTrie(proposer, time.CurrentEpoch(beaconState)) {
		return fmt.Errorf("validator with key %#x is not slashable", proposer.PublicKey())
	}
	headers := []*zondpb.SignedBeaconBlockHeader{slashing.Header_1, slashing.Header_2}
	for _, header := range headers {
		if err := signing.ComputeDomainVerifySigningRoot(beaconState, pIdx, slots.ToEpoch(hSlot),
			header.Header, params.BeaconConfig().DomainBeaconProposer, header.Signature); err != nil {
			return errors.Wrap(err, "could not verify beacon block header")
		}
	}
	return nil
}
