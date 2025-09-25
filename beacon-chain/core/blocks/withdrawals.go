package blocks

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/beacon-chain/state"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/interfaces"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/hash"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/encoding/ssz"
	qrlpb "github.com/theQRL/qrysm/proto/qrl/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

const executionToMLDSA87Padding = 12

// ProcessMLDSA87ToExecutionChanges processes a list of ML-DSA-87 Changes and validates them. However,
// the method doesn't immediately verify the signatures in the changes and prefers to extract
// a signature set from them at the end of the transition and then verify them via the
// signature set.
func ProcessMLDSA87ToExecutionChanges(
	st state.BeaconState,
	signed interfaces.ReadOnlySignedBeaconBlock) (state.BeaconState, error) {
	changes, err := signed.Block().Body().MLDSA87ToExecutionChanges()
	if err != nil {
		return nil, errors.Wrap(err, "could not get MLDSA87ToExecutionChanges")
	}
	// Return early if no changes
	if len(changes) == 0 {
		return st, nil
	}
	for _, change := range changes {
		st, err = processMLDSA87ToExecutionChange(st, change)
		if err != nil {
			return nil, errors.Wrap(err, "could not process MLDSA87ToExecutionChange")
		}
	}
	return st, nil
}

// processMLDSA87ToExecutionChange validates a SignedMLDSA87ToExecution message and
// changes the validator's withdrawal address accordingly.
//
// Spec pseudocode definition:
//
// def process_ml_dsa_87_to_execution_change(state: BeaconState, signed_address_change: SignedMLDSA87ToExecutionChange) -> None:
//
//	validator = state.validators[address_change.validator_index]
//
//	assert validator.withdrawal_credentials[:1] == ML_DSA_87_WITHDRAWAL_PREFIX
//	assert validator.withdrawal_credentials[1:] == hash(address_change.from_ml_dsa_87_pubkey)[1:]
//
//	domain = get_domain(state, DOMAIN_ML_DSA_87_TO_EXECUTION_CHANGE)
//	signing_root = compute_signing_root(address_change, domain)
//	assert ml_dsa_87.Verify(address_change.from_dilitium_pubkey, signing_root, signed_address_change.signature)
//
//	validator.withdrawal_credentials = (
//	    EXECUTION_ADDRESS_WITHDRAWAL_PREFIX
//	    + b'\x00' * 11
//	    + address_change.to_execution_address
//	)
func processMLDSA87ToExecutionChange(st state.BeaconState, signed *qrysmpb.SignedMLDSA87ToExecutionChange) (state.BeaconState, error) {
	// Checks that the message passes the validation conditions.
	val, err := ValidateMLDSA87ToExecutionChange(st, signed)
	if err != nil {
		return nil, err
	}

	message := signed.Message
	newCredentials := make([]byte, executionToMLDSA87Padding)
	newCredentials[0] = params.BeaconConfig().QRLAddressWithdrawalPrefixByte
	val.WithdrawalCredentials = append(newCredentials, message.ToExecutionAddress...)
	err = st.UpdateValidatorAtIndex(message.ValidatorIndex, val)
	return st, err
}

// ValidateMLDSA87ToExecutionChange validates the execution change message against the state and returns the
// validator referenced by the message.
func ValidateMLDSA87ToExecutionChange(st state.ReadOnlyBeaconState, signed *qrysmpb.SignedMLDSA87ToExecutionChange) (*qrysmpb.Validator, error) {
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
	if cred[0] != params.BeaconConfig().MLDSA87WithdrawalPrefixByte {
		return nil, errInvalidMLDSA87Prefix
	}

	// hash the public key and verify it matches the withdrawal credentials
	fromPubkey := message.FromMldsa87Pubkey
	hashFn := ssz.NewHasherFunc(hash.CustomSHA256Hasher())
	digest := hashFn.Hash(fromPubkey)
	if !bytes.Equal(digest[1:], cred[1:]) {
		return nil, errInvalidWithdrawalCredentials
	}
	return val, nil
}

// ProcessWithdrawals processes the validator withdrawals from the provided execution payload
// into the beacon state.
//
// Spec pseudocode definition:
//
// def process_withdrawals(state: BeaconState, payload: ExecutionPayload) -> None:
//
//	expected_withdrawals = get_expected_withdrawals(state)
//	assert len(payload.withdrawals) == len(expected_withdrawals)
//
//	for expected_withdrawal, withdrawal in zip(expected_withdrawals, payload.withdrawals):
//	    assert withdrawal == expected_withdrawal
//	    decrease_balance(state, withdrawal.validator_index, withdrawal.amount)
//
//	# Update the next withdrawal index if this block contained withdrawals
//	if len(expected_withdrawals) != 0:
//	    latest_withdrawal = expected_withdrawals[-1]
//	    state.next_withdrawal_index = WithdrawalIndex(latest_withdrawal.index + 1)
//
//	# Update the next validator index to start the next withdrawal sweep
//	if len(expected_withdrawals) == MAX_WITHDRAWALS_PER_PAYLOAD:
//	    # Next sweep starts after the latest withdrawal's validator index
//	    next_validator_index = ValidatorIndex((expected_withdrawals[-1].validator_index + 1) % len(state.validators))
//	    state.next_withdrawal_validator_index = next_validator_index
//	else:
//	    # Advance sweep by the max length of the sweep if there was not a full set of withdrawals
//	    next_index = state.next_withdrawal_validator_index + MAX_VALIDATORS_PER_WITHDRAWALS_SWEEP
//	    next_validator_index = ValidatorIndex(next_index % len(state.validators))
//	    state.next_withdrawal_validator_index = next_validator_index
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

// MLDSA87ChangesSignatureBatch extracts the relevant signatures from the provided execution change
// messages and transforms them into a signature batch object.
func MLDSA87ChangesSignatureBatch(
	st state.ReadOnlyBeaconState,
	changes []*qrysmpb.SignedMLDSA87ToExecutionChange,
) (*ml_dsa_87.SignatureBatch, error) {
	// Return early if no changes
	if len(changes) == 0 {
		return ml_dsa_87.NewSet(), nil
	}
	batch := &ml_dsa_87.SignatureBatch{
		Signatures:   make([][][]byte, len(changes)),
		PublicKeys:   make([][]ml_dsa_87.PublicKey, len(changes)),
		Messages:     make([][32]byte, len(changes)),
		Descriptions: make([]string, len(changes)),
	}
	c := params.BeaconConfig()
	domain, err := signing.ComputeDomain(c.DomainMLDSA87ToExecutionChange, c.GenesisForkVersion, st.GenesisValidatorsRoot())
	if err != nil {
		return nil, errors.Wrap(err, "could not compute signing domain")
	}
	for i, change := range changes {
		batch.Signatures[i] = append(batch.Signatures[i], change.Signature)
		publicKey, err := ml_dsa_87.PublicKeyFromBytes(change.Message.FromMldsa87Pubkey)
		if err != nil {
			return nil, errors.Wrap(err, "could not convert bytes to public key")
		}
		batch.PublicKeys[i] = append(batch.PublicKeys[i], publicKey)
		htr, err := signing.SigningData(change.Message.HashTreeRoot, domain)
		if err != nil {
			return nil, errors.Wrap(err, "could not compute MLDSA87ToExecutionChange signing data")
		}
		batch.Messages[i] = htr
		batch.Descriptions[i] = signing.MLDSA87ChangeSignature
	}
	return batch, nil
}

// VerifyMLDSA87ChangeSignature checks the signature in the SignedMLDSA87ToExecutionChange message.
// It validates the signature with the Capella fork version if the passed state
// is from a previous fork.
func VerifyMLDSA87ChangeSignature(
	st state.ReadOnlyBeaconState,
	change *qrlpb.SignedMLDSA87ToExecutionChange,
) error {
	c := params.BeaconConfig()
	domain, err := signing.ComputeDomain(c.DomainMLDSA87ToExecutionChange, c.GenesisForkVersion, st.GenesisValidatorsRoot())
	if err != nil {
		return errors.Wrap(err, "could not compute signing domain")
	}
	publicKey := change.Message.FromMldsa87Pubkey
	return signing.VerifySigningRoot(change.Message, publicKey, change.Signature, domain)
}
