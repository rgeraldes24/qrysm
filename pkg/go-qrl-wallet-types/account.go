package types

import (
	"context"

	"github.com/google/uuid"
	zondTypes "github.com/theQRL/qrysm/pkg/go-qrl-types"
)

type AccountIDProvider interface {
	ID() uuid.UUID
}

type AccountNameProvider interface {
	Name() string
}

type AccountPublicKeyProvider interface {
	PublicKey() zondTypes.PublicKey
}

type AccountPathProvider interface {
	Path() string
}

type AccountWalletProvider interface {
	Wallet() Wallet
}

type AccountLocker interface {
	Lock(ctx context.Context) error

	Unlock(ctx context.Context, passphrase []byte) error

	IsUnlocked(ctx context.Context) (bool, error)
}

type AccountSigner interface {
	Sign(ctx context.Context, data []byte) (zondTypes.Signature, error)
}

type AccountProtectingSigner interface {
	SignGeneric(ctx context.Context, data []byte, domain []byte) (zondTypes.Signature, error)

	SignBeaconProposal(ctx context.Context,
		slot uint64,
		proposerIndex uint64,
		parentRoot []byte,
		stateRoot []byte,
		bodyRoot []byte,
		domain []byte) (zondTypes.Signature, error)

	SignBeaconAttestation(ctx context.Context,
		slot uint64,
		committeeIndex uint64,
		blockRoot []byte,
		sourceEpoch uint64,
		sourceRoot []byte,
		targetEpoch uint64,
		targetRoot []byte,
		domain []byte) (zondTypes.Signature, error)
}

type AccountProtectingMultiSigner interface {
	SignBeaconAttestations(ctx context.Context,
		slot uint64,
		accounts []Account,
		committeeIndices []uint64,
		blockRoot []byte,
		sourceEpoch uint64,
		sourceRoot []byte,
		targetEpoch uint64,
		targetRoot []byte,
		domain []byte) ([]zondTypes.Signature, error)
}

type AccountCompositePublicKeyProvider interface {
	CompositePublicKey() zondTypes.PublicKey
}

type AccountSigningThresholdProvider interface {
	SigningThreshold() uint32
}

type AccountVerificationVectorProvider interface {
	VerificationVector() []zondTypes.PublicKey
}

type AccountParticipantsProvider interface {
	Participants() map[uint64]string
}

type AccountPrivateKeyProvider interface {
	PrivateKey(ctx context.Context) (zondTypes.PrivateKey, error)
}

type Account interface {
	AccountIDProvider
	AccountNameProvider
	AccountPublicKeyProvider
}

type DistributedAccount interface {
	AccountIDProvider
	AccountNameProvider
	AccountCompositePublicKeyProvider
	AccountSigningThresholdProvider
	AccountParticipantsProvider
}

type AccountMetadataProvider interface {
	WalletID() uuid.UUID

	ID() uuid.UUID

	Name() string
}
