package signing

import (
	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	zond "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// Domain returns the domain version for Dilithium private key to sign and verify.
func Domain(fork *zond.Fork, epoch primitives.Epoch, domainType [dilithium.DomainByteLength]byte, genesisRoot []byte) ([]byte, error) {
	if fork == nil {
		return []byte{}, errors.New("nil fork or domain type")
	}
	var forkVersion []byte
	if epoch < fork.Epoch {
		forkVersion = fork.PreviousVersion
	} else {
		forkVersion = fork.CurrentVersion
	}
	if len(forkVersion) != 4 {
		return []byte{}, errors.New("fork version length is not 4 byte")
	}
	var forkVersionArray [4]byte
	copy(forkVersionArray[:], forkVersion[:4])
	return ComputeDomain(domainType, forkVersionArray[:], genesisRoot)
}
