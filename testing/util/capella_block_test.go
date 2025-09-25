package util

import (
	"testing"

	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/beacon-chain/core/time"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/crypto/hash"
	"github.com/theQRL/qrysm/encoding/ssz"
	"github.com/theQRL/qrysm/testing/require"
)

func TestGenerateMLDSA87ToExecutionChange(t *testing.T) {
	st, keys := DeterministicGenesisStateCapella(t, 64)
	change, err := GenerateMLDSA87ToExecutionChange(st, keys[0], 0)
	require.NoError(t, err)

	message := change.Message
	val, err := st.ValidatorAtIndex(message.ValidatorIndex)
	require.NoError(t, err)

	cred := val.WithdrawalCredentials
	require.DeepEqual(t, cred[0], params.BeaconConfig().MLDSA87WithdrawalPrefixByte)

	fromPubkey := message.FromMldsa87Pubkey
	hashFn := ssz.NewHasherFunc(hash.CustomSHA256Hasher())
	digest := hashFn.Hash(fromPubkey)
	require.DeepEqual(t, digest[1:], digest[1:])

	domain, err := signing.Domain(st.Fork(), time.CurrentEpoch(st), params.BeaconConfig().DomainMLDSA87ToExecutionChange, st.GenesisValidatorsRoot())
	require.NoError(t, err)

	require.NoError(t, signing.VerifySigningRoot(message, fromPubkey, change.Signature, domain))
}
