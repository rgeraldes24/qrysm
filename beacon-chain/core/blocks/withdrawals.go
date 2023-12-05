package blocks

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	fieldparams "github.com/theQRL/qrysm/v4/config/fieldparams"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/crypto/hash"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	"github.com/theQRL/qrysm/v4/encoding/ssz"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
	zondpbv1 "github.com/theQRL/qrysm/v4/proto/zond/v1"
)

const executionToDilithiumPadding = 12

// ProcessDilithiumToExecutionChanges processes a list of Dilithium Changes and validates them. However,
// the method doesn't immediately verify the signatures in the changes and prefers to extract
// a signature set from them at the end of the transition and then verify them via the
// signature set.
func ProcessDilithiumToExecutionChanges(
	st state.BeaconState,
	signed interfaces.ReadOnlySignedBeaconBlock) (state.BeaconState, error) {
	changes, err := signed.Block().Body().DilithiumToExecutionChanges()
	if err != nil {
		return nil, errors.Wrap(err, "could not get DilithiumToExecutionChanges")
	}
	// Return early if no changes
	if len(changes) == 0 {
		return st, nil
	}
	for _, change := range changes {
		st, err = processDilithiumToExecutionChange(st, change)
		if err != nil {
			return nil, errors.Wrap(err, "could not process DilithiumToExecutionChange")
		}
	}
	return st, nil
}

// processDilithiumToExecutionChange validates a SignedDilithiumToExecution message and
// changes the validator's withdrawal address accordingly.
func processDilithiumToExecutionChange(st state.BeaconState, signed *zondpb.SignedDilithiumToExecutionChange) (state.BeaconState, error) {
	// Checks that the message passes the validation conditions.
	val, err := ValidateDilithiumToExecutionChange(st, signed)
	if err != nil {
		return nil, err
	}

	message := signed.Message
	newCredentials := make([]byte, executionToDilithiumPadding)
	newCredentials[0] = params.BeaconConfig().ZOND1AddressWithdrawalPrefixByte
	val.WithdrawalCredentials = append(newCredentials, message.ToExecutionAddress...)
	err = st.UpdateValidatorAtIndex(message.ValidatorIndex, val)
	return st, err
}

// ValidateDilithiumToExecutionChange validates the execution change message against the state and returns the
// validator referenced by the message.
func ValidateDilithiumToExecutionChange(st state.ReadOnlyBeaconState, signed *zondpb.SignedDilithiumToExecutionChange) (*zondpb.Validator, error) {
	if signed == nil {
		return nil, errNilSignedWithdrawalMessage
	}
	message := signed.Message
	if message == nil {
		return nil, errNilWithdrawalMessage
	}

	val, err := st.ValidatorAtIndex(message.ValidatorIndex)
	if err != nil {
		return nil, err
	}
	cred := val.WithdrawalCredentials
	if cred[0] != params.BeaconConfig().DilithiumWithdrawalPrefixByte {
		return nil, errInvalidDilithiumPrefix
	}

	// hash the public key and verify it matches the withdrawal credentials
	fromPubkey := message.FromDilithiumPubkey
	hashFn := ssz.NewHasherFunc(hash.CustomSHA256Hasher())
	digest := hashFn.Hash(fromPubkey)
	if !bytes.Equal(digest[1:], cred[1:]) {
		return nil, errInvalidWithdrawalCredentials
	}
	return val, nil
}

// ProcessWithdrawals processes the validator withdrawals from the provided execution payload
// into the beacon state.
func ProcessWithdrawals(st state.BeaconState, executionData interfaces.ExecutionData) (state.BeaconState, error) {
	expectedWithdrawals, err := st.ExpectedWithdrawals()
	if err != nil {
		return nil, errors.Wrap(err, "could not get expected withdrawals")
	}

	var wdRoot [32]byte
	if executionData.IsBlinded() {
		r, err := executionData.WithdrawalsRoot()
		if err != nil {
			return nil, errors.Wrap(err, "could not get withdrawals root")
		}
		wdRoot = bytesutil.ToBytes32(r)
	} else {
		wds, err := executionData.Withdrawals()
		if err != nil {
			return nil, errors.Wrap(err, "could not get withdrawals")
		}
		wdRoot, err = ssz.WithdrawalSliceRoot(wds, fieldparams.MaxWithdrawalsPerPayload)
		if err != nil {
			return nil, errors.Wrap(err, "could not get withdrawals root")
		}
	}

	expectedRoot, err := ssz.WithdrawalSliceRoot(expectedWithdrawals, fieldparams.MaxWithdrawalsPerPayload)
	if err != nil {
		return nil, errors.Wrap(err, "could not get expected withdrawals root")
	}
	if expectedRoot != wdRoot {
		return nil, fmt.Errorf("expected withdrawals root %#x, got %#x", expectedRoot, wdRoot)
	}

	for _, withdrawal := range expectedWithdrawals {
		err := helpers.DecreaseBalance(st, withdrawal.ValidatorIndex, withdrawal.Amount)
		if err != nil {
			return nil, errors.Wrap(err, "could not decrease balance")
		}
	}
	if len(expectedWithdrawals) > 0 {
		if err := st.SetNextWithdrawalIndex(expectedWithdrawals[len(expectedWithdrawals)-1].Index + 1); err != nil {
			return nil, errors.Wrap(err, "could not set next withdrawal index")
		}
	}
	var nextValidatorIndex primitives.ValidatorIndex
	if uint64(len(expectedWithdrawals)) < params.BeaconConfig().MaxWithdrawalsPerPayload {
		nextValidatorIndex, err = st.NextWithdrawalValidatorIndex()
		if err != nil {
			return nil, errors.Wrap(err, "could not get next withdrawal validator index")
		}
		nextValidatorIndex += primitives.ValidatorIndex(params.BeaconConfig().MaxValidatorsPerWithdrawalsSweep)
		nextValidatorIndex = nextValidatorIndex % primitives.ValidatorIndex(st.NumValidators())
	} else {
		nextValidatorIndex = expectedWithdrawals[len(expectedWithdrawals)-1].ValidatorIndex + 1
		if nextValidatorIndex == primitives.ValidatorIndex(st.NumValidators()) {
			nextValidatorIndex = 0
		}
	}
	if err := st.SetNextWithdrawalValidatorIndex(nextValidatorIndex); err != nil {
		return nil, errors.Wrap(err, "could not set next withdrawal validator index")
	}
	return st, nil
}

// DilithiumChangesSignatureBatch extracts the relevant signatures from the provided execution change
// messages and transforms them into a signature batch object.
func DilithiumChangesSignatureBatch(
	st state.ReadOnlyBeaconState,
	changes []*zondpb.SignedDilithiumToExecutionChange,
) (*dilithium.SignatureBatch, error) {
	// Return early if no changes
	if len(changes) == 0 {
		return dilithium.NewSet(), nil
	}
	batch := &dilithium.SignatureBatch{
		Signatures:   make([][][]byte, len(changes)),
		PublicKeys:   make([][]dilithium.PublicKey, len(changes)),
		Messages:     make([][32]byte, len(changes)),
		Descriptions: make([]string, len(changes)),
	}
	c := params.BeaconConfig()
	domain, err := signing.ComputeDomain(c.DomainDilithiumToExecutionChange, c.GenesisForkVersion, st.GenesisValidatorsRoot())
	if err != nil {
		return nil, errors.Wrap(err, "could not compute signing domain")
	}
	for i, change := range changes {
		batch.Signatures[i] = append(batch.Signatures[i], change.Signature)
		publicKey, err := dilithium.PublicKeyFromBytes(change.Message.FromDilithiumPubkey)
		if err != nil {
			return nil, errors.Wrap(err, "could not convert bytes to public key")
		}
		batch.PublicKeys[i] = append(batch.PublicKeys[i], publicKey)
		htr, err := signing.SigningData(change.Message.HashTreeRoot, domain)
		if err != nil {
			return nil, errors.Wrap(err, "could not compute DilithiumToExecutionChange signing data")
		}
		batch.Messages[i] = htr
		batch.Descriptions[i] = signing.DilithiumChangeSignature
	}
	return batch, nil
}

// VerifyDilithiumChangeSignature checks the signature in the SignedDilithiumToExecutionChange message.
// It validates the signature with the Capella fork version if the passed state
// is from a previous fork.
func VerifyDilithiumChangeSignature(
	st state.ReadOnlyBeaconState,
	change *zondpbv1.SignedDilithiumToExecutionChange,
) error {
	c := params.BeaconConfig()
	domain, err := signing.ComputeDomain(c.DomainDilithiumToExecutionChange, c.GenesisForkVersion, st.GenesisValidatorsRoot())
	if err != nil {
		return errors.Wrap(err, "could not compute signing domain")
	}
	publicKey := change.Message.FromDilithiumPubkey
	return signing.VerifySigningRoot(change.Message, publicKey, change.Signature, domain)
}
