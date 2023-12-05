package signing

import (
	"sync"

	"github.com/pkg/errors"
	fssz "github.com/prysmaticlabs/fastssz"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/qrysm/v1alpha1"
)

// ForkVersionByteLength length of fork version byte array.
const ForkVersionByteLength = 4

// DomainByteLength length of domain byte array.
const DomainByteLength = 4

// digestMap maps the fork version and genesis validator root to the
// resultant fork digest.
var digestMapLock sync.RWMutex
var digestMap = make(map[string][32]byte)

// ErrSigFailedToVerify returns when a signature of a block object(ie attestation, slashing, exit... etc)
// failed to verify.
var ErrSigFailedToVerify = errors.New("signature did not verify")

// List of descriptions for different kinds of signatures
const (
	// UnknownSignature represents all signatures other than below types
	UnknownSignature string = "unknown signature"
	// BlockSignature represents the block signature from block proposer
	BlockSignature = "block signature"
	// RandaoSignature represents randao specific signature
	RandaoSignature = "randao signature"
	// SelectionProof represents selection proof
	SelectionProof = "selection proof"
	// AggregatorSignature represents aggregator's signature
	AggregatorSignature = "aggregator signature"
	// AttestationSignature represents aggregated attestation signature
	AttestationSignature = "attestation signature"
	// DilithiumChangeSignature represents signature to DilithiumToExecutionChange
	DilithiumChangeSignature = "dilithiumchange signature"
	// SyncCommitteeSignature represents sync committee signature
	SyncCommitteeSignature = "sync committee signature"
	// SyncSelectionProof represents sync committee selection proof
	SyncSelectionProof = "sync selection proof"
	// ContributionSignature represents sync committee contributor's signature
	ContributionSignature = "sync committee contribution signature"
	// SyncAggregateSignature represents sync committee aggregator's signature
	SyncAggregateSignature = "sync committee aggregator signature"
)

// ComputeDomainAndSign computes the domain and signing root and sign it using the passed in private key.
func ComputeDomainAndSign(st state.ReadOnlyBeaconState, epoch primitives.Epoch, obj fssz.HashRoot, domain [4]byte, key dilithium.DilithiumKey) ([]byte, error) {
	d, err := Domain(st.Fork(), epoch, domain, st.GenesisValidatorsRoot())
	if err != nil {
		return nil, err
	}
	sr, err := ComputeSigningRoot(obj, d)
	if err != nil {
		return nil, err
	}
	return key.Sign(sr[:]).Marshal(), nil
}

// ComputeSigningRoot computes the root of the object by calculating the hash tree root of the signing data with the given domain.
func ComputeSigningRoot(object fssz.HashRoot, domain []byte) ([32]byte, error) {
	return SigningData(object.HashTreeRoot, domain)
}

// SigningData computes the signing data by utilising the provided root function and then
// returning the signing data of the container object.
func SigningData(rootFunc func() ([32]byte, error), domain []byte) ([32]byte, error) {
	objRoot, err := rootFunc()
	if err != nil {
		return [32]byte{}, err
	}
	container := &zondpb.SigningData{
		ObjectRoot: objRoot[:],
		Domain:     domain,
	}
	return container.HashTreeRoot()
}

// ComputeDomainVerifySigningRoot computes domain and verifies signing root of an object given the beacon state, validator index and signature.
func ComputeDomainVerifySigningRoot(st state.ReadOnlyBeaconState, index primitives.ValidatorIndex, epoch primitives.Epoch, obj fssz.HashRoot, domain [4]byte, sig []byte) error {
	v, err := st.ValidatorAtIndex(index)
	if err != nil {
		return err
	}
	d, err := Domain(st.Fork(), epoch, domain, st.GenesisValidatorsRoot())
	if err != nil {
		return err
	}
	return VerifySigningRoot(obj, v.PublicKey, sig, d)
}

// VerifySigningRoot verifies the signing root of an object given its public key, signature and domain.
func VerifySigningRoot(obj fssz.HashRoot, pub, signature, domain []byte) error {
	publicKey, err := dilithium.PublicKeyFromBytes(pub)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to public key")
	}
	sig, err := dilithium.SignatureFromBytes(signature)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to signature")
	}
	root, err := ComputeSigningRoot(obj, domain)
	if err != nil {
		return errors.Wrap(err, "could not compute signing root")
	}
	if !sig.Verify(publicKey, root[:]) {
		return ErrSigFailedToVerify
	}
	return nil
}

// VerifyBlockHeaderSigningRoot verifies the signing root of a block header given its public key, signature and domain.
func VerifyBlockHeaderSigningRoot(blkHdr *zondpb.BeaconBlockHeader, pub, signature, domain []byte) error {
	publicKey, err := dilithium.PublicKeyFromBytes(pub)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to public key")
	}
	sig, err := dilithium.SignatureFromBytes(signature)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to signature")
	}
	root, err := SigningData(blkHdr.HashTreeRoot, domain)
	if err != nil {
		return errors.Wrap(err, "could not compute signing root")
	}
	if !sig.Verify(publicKey, root[:]) {
		return ErrSigFailedToVerify
	}
	return nil
}

// VerifyBlockSigningRoot verifies the signing root of a block given its public key, signature and domain.
func VerifyBlockSigningRoot(pub, signature, domain []byte, rootFunc func() ([32]byte, error)) error {
	set, err := BlockSignatureBatch(pub, signature, domain, rootFunc)
	if err != nil {
		return err
	}

	// We assume only one signature batch is returned here.
	sig := set.Signatures[0][0]
	publicKey := set.PublicKeys[0][0]
	root := set.Messages[0]

	rSig, err := dilithium.SignatureFromBytes(sig)
	if err != nil {
		return err
	}
	if !rSig.Verify(publicKey, root[:]) {
		return ErrSigFailedToVerify
	}
	return nil
}

// BlockSignatureBatch retrieves the relevant signature, message and pubkey data from a block and collating it
// into a signature batch object.
func BlockSignatureBatch(pub, signature, domain []byte, rootFunc func() ([32]byte, error)) (*dilithium.SignatureBatch, error) {
	publicKey, err := dilithium.PublicKeyFromBytes(pub)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert bytes to public key")
	}
	// utilize custom block hashing function
	root, err := SigningData(rootFunc, domain)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute signing root")
	}
	desc := BlockSignature
	return &dilithium.SignatureBatch{
		Signatures:   [][][]byte{{signature}},
		PublicKeys:   [][]dilithium.PublicKey{{publicKey}},
		Messages:     [][32]byte{root},
		Descriptions: []string{desc},
	}, nil
}

// ComputeDomain returns the domain version for Dilithium private key to sign and verify with a zeroed 4-byte
// array as the fork version.
func ComputeDomain(domainType [DomainByteLength]byte, forkVersion, genesisValidatorsRoot []byte) ([]byte, error) {
	if forkVersion == nil {
		forkVersion = params.BeaconConfig().GenesisForkVersion
	}
	if genesisValidatorsRoot == nil {
		genesisValidatorsRoot = params.BeaconConfig().ZeroHash[:]
	}
	var forkBytes [ForkVersionByteLength]byte
	copy(forkBytes[:], forkVersion)

	forkDataRoot, err := computeForkDataRoot(forkBytes[:], genesisValidatorsRoot)
	if err != nil {
		return nil, err
	}

	return domain(domainType, forkDataRoot[:]), nil
}

// This returns the dilithium domain given by the domain type and fork data root.
func domain(domainType [DomainByteLength]byte, forkDataRoot []byte) []byte {
	var b []byte
	b = append(b, domainType[:4]...)
	b = append(b, forkDataRoot[:28]...)
	return b
}

// computeForkDataRoot returns the 32byte fork data root for the “current_version“ and “genesis_validators_root“.
// This is used primarily in signature domains to avoid collisions across forks/chains.
func computeForkDataRoot(version, root []byte) ([32]byte, error) {
	digestMapLock.RLock()
	if val, ok := digestMap[string(version)+string(root)]; ok {
		digestMapLock.RUnlock()
		return val, nil
	}
	digestMapLock.RUnlock()
	r, err := (&zondpb.ForkData{
		CurrentVersion:        version,
		GenesisValidatorsRoot: root,
	}).HashTreeRoot()
	if err != nil {
		return [32]byte{}, err
	}
	// Cache result of digest computation
	// as this is a hot path and doesn't need
	// to be constantly computed.
	digestMapLock.Lock()
	digestMap[string(version)+string(root)] = r
	digestMapLock.Unlock()
	return r, nil
}

// ComputeForkDigest returns the fork for the current version and genesis validators root
func ComputeForkDigest(version, genesisValidatorsRoot []byte) ([4]byte, error) {
	dataRoot, err := computeForkDataRoot(version, genesisValidatorsRoot)
	if err != nil {
		return [4]byte{}, err
	}
	return bytesutil.ToBytes4(dataRoot[:]), nil
}
