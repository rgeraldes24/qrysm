package local

import (
	"context"
	"testing"

	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/crypto/randao"
	"github.com/theQRL/qrysm/testing/require"
)

func TestLocalKeymanager_RandaoReveal_IgnoresSlot(t *testing.T) {
	secretKey, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	copy(pubKey[:], secretKey.PublicKey().Marshal())

	lock.Lock()
	mlDSA87KeysCache[pubKey] = secretKey
	lock.Unlock()
	t.Cleanup(func() {
		lock.Lock()
		delete(mlDSA87KeysCache, pubKey)
		lock.Unlock()
		resetRandaoOnions()
	})

	commitment := randao.Commitment(secretKey.Marshal(), randao.DefaultLayers)
	km := &Keymanager{}
	ctx := context.Background()

	var first [field_params.RandaoRevealLength]byte
	for i, slot := range []primitives.Slot{0, 1, 12345, primitives.Slot(^uint64(0))} {
		reveal, err := km.RandaoReveal(ctx, pubKey, commitment, slot)
		require.NoError(t, err)
		require.Equal(t, true, randao.Verify(reveal, commitment))
		if i == 0 {
			first = reveal
			continue
		}
		require.Equal(t, first, reveal, "reveal must not depend on the slot")
	}
}

func TestLocalKeymanager_RandaoReveal_UnknownKey(t *testing.T) {
	var pubKey [field_params.MLDSA87PubkeyLength]byte
	pubKey[0] = 0xBB
	km := &Keymanager{}
	_, err := km.RandaoReveal(context.Background(), pubKey, [field_params.RandaoCommitmentLength]byte{}, 1)
	require.ErrorContains(t, "no signing key found in keys cache", err)
}
