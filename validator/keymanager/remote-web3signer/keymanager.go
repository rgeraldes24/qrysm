// Package remote_web3signer is a keymanager backed by a remote signer that
// implements the Web3Signer-style consensus signing API, adapted to QRL:
// ML-DSA-87 keys, per-participant signature lists and a RANDAO hash-onion
// reveal endpoint.
package remote_web3signer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/async/event"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	qrlpbservice "github.com/theQRL/qrysm/proto/qrl/service"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/validator/accounts/petnames"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/internal"
	web3signerv1 "github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1"
)

// defaultRetryDelay is the wait before the single RANDAO reveal retry when a
// 429 or 503 response carries no Retry-After header. A variable for tests.
var defaultRetryDelay = 500 * time.Millisecond

// ErrInvalidSignature is returned when the remote signer's signature does not
// verify against the requested public key and signing root.
var ErrInvalidSignature = errors.New("remote signer returned a signature that does not verify")

// SetupConfig includes configuration values for initializing a remote signer
// keymanager. The validating keys come from exactly one of PublicKeysURL,
// ProvidedPublicKeys or KeyFile.
type SetupConfig struct {
	BaseEndpoint          string
	GenesisValidatorsRoot []byte

	// If the URL is set, the keymanager will fetch the public keys from the URL.
	// caution: this option is susceptible to slashing if the web3signer's validator keys are shared across validators
	PublicKeysURL string

	// a static list of public keys to be passed by the user to determine what accounts should sign.
	// This will provide a layer of safety against slashing if the web3signer is shared across validators.
	ProvidedPublicKeys [][field_params.MLDSA87PubkeyLength]byte

	// KeyFile names a file of public keys, one per line (see ParseKeyFile).
	// NewKeymanager reads it, and re-reads it on change when
	// ListenForChanges is set.
	KeyFile          string
	ListenForChanges bool

	// AuthTokenFile holds the signer's sign-scope bearer token. Optional.
	AuthTokenFile string
	// TLSCAFile replaces the system roots when verifying the signer's
	// certificate. Optional.
	TLSCAFile string
	// TLSClientCertFile and TLSClientKeyFile enable mutual TLS. Optional,
	// both or neither.
	TLSClientCertFile string
	TLSClientKeyFile  string
	// Timeout bounds each request to the signer. Zero means 10 seconds.
	Timeout time.Duration
}

// Keymanager defines the web3signer keymanager.
type Keymanager struct {
	client                internal.HttpSignerClient
	genesisValidatorsRoot []byte
	publicKeysURL         string
	accountsChangedFeed   *event.Feed
	signerURL             string // the base endpoint without user information
	keyFile               string // absolute; empty without a key file
	stopWatching          func() // stops the key file watcher; nil without one

	// updateLock serializes changes to the validating keys, each with its
	// key file read or write and its notification, so that subscribers see
	// them in order and a reload never undoes a newer change.
	updateLock          sync.Mutex
	lock                sync.RWMutex
	providedPublicKeys  [][field_params.MLDSA87PubkeyLength]byte
	publicKeysUrlCalled bool
}

// NewKeymanager instantiates a new web3signer key manager. With a key file
// and ListenForChanges, the file is watched until ctx is done or Close is
// called.
func NewKeymanager(ctx context.Context, cfg *SetupConfig) (*Keymanager, error) {
	// The errors name the field, never its value: the URL may carry
	// credentials.
	if cfg.BaseEndpoint == "" {
		return nil, errors.New("invalid setup config: remote signer base endpoint is empty")
	}
	if !bytesutil.IsValidRoot(cfg.GenesisValidatorsRoot) {
		return nil, fmt.Errorf("invalid setup config: genesis validators root is invalid (%d bytes)", len(cfg.GenesisValidatorsRoot))
	}
	client, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	km := &Keymanager{
		client:                client,
		genesisValidatorsRoot: cfg.GenesisValidatorsRoot,
		accountsChangedFeed:   new(event.Feed),
		signerURL:             signerURLForDisplay(cfg.BaseEndpoint),
		publicKeysURL:         cfg.PublicKeysURL,
		providedPublicKeys:    cfg.ProvidedPublicKeys,
	}
	if cfg.KeyFile == "" {
		return km, nil
	}
	if cfg.PublicKeysURL != "" || len(cfg.ProvidedPublicKeys) != 0 {
		return nil, errors.New("invalid setup config: a key file cannot be combined with a public keys url or list")
	}
	km.keyFile, err = filepath.Abs(cfg.KeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not resolve the remote signer key file path")
	}
	if km.providedPublicKeys, err = ReadKeyFile(km.keyFile); err != nil {
		return nil, err
	}
	log.WithFields(logrus.Fields{
		"keyFile": km.keyFile,
		"keys":    len(km.providedPublicKeys),
	}).Info("Loaded validating public keys from the remote signer key file")
	if cfg.ListenForChanges {
		if err := km.watchKeyFile(ctx); err != nil {
			return nil, err
		}
	}
	return km, nil
}

// signerURLForDisplay returns the signer's base URL without user information,
// query or fragment, which may carry credentials.
func signerURLForDisplay(baseEndpoint string) string {
	u, err := url.Parse(baseEndpoint)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// CheckClientConfig builds the signer client that NewKeymanager would build
// from cfg, reading the auth token and TLS files, so that a bad file, TLS
// files with an http URL or an http public keys URL next to an https signer
// URL can be reported at startup, before the chain starts.
func CheckClientConfig(cfg *SetupConfig) error {
	_, err := newClient(cfg)
	return err
}

func newClient(cfg *SetupConfig) (*internal.ApiClient, error) {
	client, err := internal.NewApiClient(internal.Config{
		BaseEndpoint:      cfg.BaseEndpoint,
		AuthTokenFile:     cfg.AuthTokenFile,
		TLSCAFile:         cfg.TLSCAFile,
		TLSClientCertFile: cfg.TLSClientCertFile,
		TLSClientKeyFile:  cfg.TLSClientKeyFile,
		Timeout:           cfg.Timeout,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not create remote signer client")
	}
	if cfg.PublicKeysURL != "" {
		if err := client.CheckPublicKeysURL(cfg.PublicKeysURL); err != nil {
			return nil, err
		}
	}
	return client, nil
}

// Close stops the key file watcher, if there is one, and waits until it has
// released its file watch. The keymanager still signs afterwards but no
// longer reloads the key file. Close may be called more than once.
func (km *Keymanager) Close() {
	if km.stopWatching != nil {
		km.stopWatching()
	}
}

// FetchValidatingPublicKeys fetches the validating public keys
// from the remote server or from the provided keys if there are no existing public keys set
// or provides the existing keys in the keymanager.
func (km *Keymanager) FetchValidatingPublicKeys(ctx context.Context) ([][field_params.MLDSA87PubkeyLength]byte, error) {
	km.lock.Lock()
	defer km.lock.Unlock()
	if km.publicKeysURL != "" && !km.publicKeysUrlCalled {
		providedPublicKeys, err := km.client.GetPublicKeys(ctx, km.publicKeysURL)
		if err != nil {
			erroredResponsesTotal.Inc()
			return nil, errors.Wrap(err, "could not get public keys from the remote signer public keys url")
		}
		// makes sure that if the public keys are deleted the validator does not call URL again.
		km.publicKeysUrlCalled = true
		km.providedPublicKeys = providedPublicKeys
	}
	keys := make([][field_params.MLDSA87PubkeyLength]byte, len(km.providedPublicKeys))
	copy(keys, km.providedPublicKeys)
	return keys, nil
}

// Sign signs the message by using a remote web3signer server. The returned
// signature is verified against the request's public key and signing root
// before it is handed to the caller.
func (km *Keymanager) Sign(ctx context.Context, request *validatorpb.SignRequest) (ml_dsa_87.Signature, error) {
	signRequest, artifactType, err := getSignRequestJson(request, km.genesisValidatorsRoot)
	if err != nil {
		erroredResponsesTotal.Inc()
		return nil, err
	}
	pubKey, err := ml_dsa_87.PublicKeyFromBytes(request.PublicKey)
	if err != nil {
		erroredResponsesTotal.Inc()
		return nil, errors.Wrap(err, "invalid public key in sign request")
	}

	signRequestsTotal.Inc()
	start := time.Now()
	sig, err := km.client.Sign(ctx, hexutil.Encode(request.PublicKey), signRequest)
	if err != nil {
		erroredResponsesTotal.Inc()
		return nil, err
	}
	if !sig.Verify(pubKey, request.SigningRoot) {
		erroredResponsesTotal.Inc()
		return nil, errors.Wrapf(ErrInvalidSignature, "artifact type %s", artifactType)
	}
	signRequestDurationSeconds.WithLabelValues(artifactType).Observe(time.Since(start).Seconds())
	return sig, nil
}

// RandaoReveal asks the remote signer, which holds the validator's seed, for
// the pre-image of commitment for the proposal at slot. A 429 or 503 response
// is retried once, after its Retry-After delay, if that still fits in ctx.
// The caller checks SHA-256(reveal) == commitment before using it.
func (km *Keymanager) RandaoReveal(
	ctx context.Context,
	publicKey [field_params.MLDSA87PubkeyLength]byte,
	commitment [field_params.RandaoCommitmentLength]byte,
	slot primitives.Slot,
) ([field_params.RandaoRevealLength]byte, error) {
	var reveal [field_params.RandaoRevealLength]byte
	randaoRevealRequestsTotal.Inc()
	request, err := web3signerv1.GetRandaoRevealRequest(slot, commitment[:], km.genesisValidatorsRoot)
	if err != nil {
		erroredResponsesTotal.Inc()
		return reveal, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		erroredResponsesTotal.Inc()
		return reveal, err
	}
	identifier := hexutil.Encode(publicKey[:])
	reveal, err = km.client.RandaoReveal(ctx, identifier, body)
	if delay, ok := retryDelay(ctx, err); ok {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
			reveal, err = km.client.RandaoReveal(ctx, identifier, body)
		}
	}
	if err != nil {
		erroredResponsesTotal.Inc()
		return [field_params.RandaoRevealLength]byte{}, err
	}
	return reveal, nil
}

// retryDelay reports whether err is a 429 or 503 refusal worth one retry, and
// how long to wait first. It declines when the wait would outlast ctx.
func retryDelay(ctx context.Context, err error) (time.Duration, bool) {
	var signerErr *internal.SignerError
	if !errors.As(err, &signerErr) {
		return 0, false
	}
	if !errors.Is(signerErr, internal.ErrRateLimited) && !errors.Is(signerErr, internal.ErrSignerUnavailable) {
		return 0, false
	}
	delay := signerErr.RetryAfter
	if delay == 0 {
		delay = defaultRetryDelay
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
		return 0, false
	}
	return delay, true
}

// getSignRequestJson returns the JSON body and artifact type for a sign
// request, based on its oneof.
func getSignRequestJson(request *validatorpb.SignRequest, genesisValidatorsRoot []byte) (internal.SignRequestJson, string, error) {
	if request == nil {
		return nil, "", errors.New("nil sign request provided")
	}
	if !bytesutil.IsValidRoot(genesisValidatorsRoot) {
		return nil, "", fmt.Errorf("invalid genesis validators root length, genesis root: %v", genesisValidatorsRoot)
	}
	var (
		body any
		typ  string
		err  error
	)
	switch request.Object.(type) {
	case *validatorpb.SignRequest_BlockZond:
		body, err = web3signerv1.GetBlockV2SignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeBlockV2
		blockZondSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_BlindedBlockZond:
		body, err = web3signerv1.GetBlockV2SignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeBlockV2
		blindedBlockZondSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_AttestationData:
		body, err = web3signerv1.GetAttestationSignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeAttestation
		attestationSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_AggregateAttestationAndProof:
		body, err = web3signerv1.GetAggregateAndProofSignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeAggregateAndProof
		aggregateAndProofSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_Slot:
		body, err = web3signerv1.GetAggregationSlotSignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeAggregationSlot
		aggregationSlotSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_Exit:
		body, err = web3signerv1.GetVoluntaryExitSignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeVoluntaryExit
		voluntaryExitSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_SyncMessageBlockRoot:
		body, err = web3signerv1.GetSyncCommitteeMessageSignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeSyncCommitteeMessage
		syncCommitteeMessageSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_SyncAggregatorSelectionData:
		body, err = web3signerv1.GetSyncCommitteeSelectionProofSignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeSyncCommitteeSelectionProof
		syncCommitteeSelectionProofSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_ContributionAndProof:
		body, err = web3signerv1.GetSyncCommitteeContributionAndProofSignRequest(request, genesisValidatorsRoot)
		typ = web3signerv1.TypeSyncCommitteeContributionAndProof
		syncCommitteeContributionAndProofSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_Registration:
		body, err = web3signerv1.GetValidatorRegistrationSignRequest(request)
		typ = web3signerv1.TypeValidatorRegistration
		validatorRegistrationSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_Epoch:
		return nil, "", web3signerv1.ErrRandaoRevealSignRequest
	default:
		return nil, "", fmt.Errorf("web3signer sign request type %T not supported", request.Object)
	}
	if err != nil {
		return nil, "", err
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}
	return encoded, typ, nil
}

// SubscribeAccountChanges returns the event subscription for changes to public keys.
func (km *Keymanager) SubscribeAccountChanges(pubKeysChan chan [][field_params.MLDSA87PubkeyLength]byte) event.Subscription {
	return km.accountsChangedFeed.Subscribe(pubKeysChan)
}

// ExtractKeystores is not supported for the remote-web3signer keymanager type.
func (*Keymanager) ExtractKeystores(
	_ context.Context, _ []ml_dsa_87.PublicKey, _ string,
) ([]*keymanager.Keystore, error) {
	return nil, errors.New("extracting keys is not supported for a web3signer keymanager")
}

// DeleteKeystores is not supported for the remote-web3signer keymanager type.
func (*Keymanager) DeleteKeystores(context.Context, [][]byte) ([]*qrlpbservice.DeletedKeystoreStatus, error) {
	return nil, errors.New("wrong wallet type: web3-signer. Only imported wallets can delete accounts")
}

// ListKeymanagerAccounts prints the remote signer accounts to stdout.
func (km *Keymanager) ListKeymanagerAccounts(ctx context.Context, cfg keymanager.ListKeymanagerAccountConfig) error {
	au := aurora.NewAurora(true)
	fmt.Printf("(keymanager kind) %s\n", au.BrightGreen("web3signer").Bold())
	fmt.Printf(
		"(configuration file path) %s\n",
		au.BrightGreen(filepath.Join(cfg.WalletAccountsDir, cfg.KeymanagerConfigFileName)).Bold(),
	)
	fmt.Println(" ")
	fmt.Printf("%s\n", au.BrightGreen("Setup Configuration").Bold())
	fmt.Println(" ")
	validatingPubKeys, err := km.FetchValidatingPublicKeys(ctx)
	if err != nil {
		return errors.Wrap(err, "could not fetch validating public keys")
	}
	switch len(validatingPubKeys) {
	case 0:
		fmt.Print("No accounts found\n")
		return nil
	case 1:
		fmt.Print("Showing 1 validator account\n")
	default:
		fmt.Printf("Showing %d validator accounts\n", len(validatingPubKeys))
	}
	DisplayRemotePublicKeys(validatingPubKeys)
	return nil
}

// DisplayRemotePublicKeys prints remote public keys to stdout.
func DisplayRemotePublicKeys(validatingPubKeys [][field_params.MLDSA87PubkeyLength]byte) {
	au := aurora.NewAurora(true)
	for i := range validatingPubKeys {
		fmt.Println("")
		fmt.Printf(
			"%s\n", au.BrightGreen(petnames.DeterministicName(validatingPubKeys[i][:], "-")).Bold(),
		)
		// Retrieve the validating key account metadata.
		fmt.Printf("%s %#x\n", au.BrightCyan("[validating public key]").Bold(), validatingPubKeys[i])
		fmt.Println(" ")
	}
}

// ErrKeyFileRequired is returned by AddPublicKeys and DeletePublicKeys when
// the validating keys do not come from a key file. Keys given as a list or by
// a public keys URL are fixed for the life of the validator client: a change
// made in memory only would be undone by a restart, which could make the
// validator client sign again with a key that was deleted and moved to
// another validator client.
var ErrKeyFileRequired = errors.New("the validating keys can only be changed when they come from " +
	"a remote signer key file (--validators-external-signer-key-file)")

// SignerURL returns the remote signer's URL, without user information, as
// the remote-keys API reports it for every key.
func (km *Keymanager) SignerURL() string {
	return km.signerURL
}

// KeysReadOnly reports whether the remote-keys API cannot add or delete
// keys, which is so unless the keys come from a key file.
func (km *Keymanager) KeysReadOnly() bool {
	return km.keyFile == ""
}

// AddPublicKeys adds public keys to the key file and to the validating keys,
// and notifies subscribers. A key that is already present, or repeated in
// pubKeys, is reported as a duplicate. The key file is read again first, so
// that an edit the watcher has not reloaded yet is kept, and is then replaced
// atomically; if that fails, nothing changes and the error is returned. Only
// keys that come from a key file can be added (ErrKeyFileRequired).
func (km *Keymanager) AddPublicKeys(_ context.Context, pubKeys [][field_params.MLDSA87PubkeyLength]byte) ([]*qrlpbservice.ImportedRemoteKeysStatus, error) {
	if km.keyFile == "" {
		return nil, ErrKeyFileRequired
	}
	km.updateLock.Lock()
	defer km.updateLock.Unlock()
	// An invalid file is an error, so that it is not overwritten: the
	// operator may be in the middle of editing it.
	data, keys, err := readKeyFile(km.keyFile)
	if err != nil {
		return nil, err
	}
	present := make(map[[field_params.MLDSA87PubkeyLength]byte]bool, len(keys)+len(pubKeys))
	for _, key := range keys {
		present[key] = true
	}
	statuses := make([]*qrlpbservice.ImportedRemoteKeysStatus, len(pubKeys))
	var added [][field_params.MLDSA87PubkeyLength]byte
	for i, pubKey := range pubKeys {
		if present[pubKey] {
			statuses[i] = &qrlpbservice.ImportedRemoteKeysStatus{
				Status:  qrlpbservice.ImportedRemoteKeysStatus_DUPLICATE,
				Message: fmt.Sprintf("Duplicate pubkey: %v, already in use", hexutil.Encode(pubKey[:])),
			}
			continue
		}
		present[pubKey] = true
		added = append(added, pubKey)
		statuses[i] = &qrlpbservice.ImportedRemoteKeysStatus{
			Status:  qrlpbservice.ImportedRemoteKeysStatus_IMPORTED,
			Message: fmt.Sprintf("Successfully added pubkey: %v", hexutil.Encode(pubKey[:])),
		}
	}
	if len(added) > 0 {
		if err := replaceKeyFile(km.keyFile, appendKeyFileLines(data, added)); err != nil {
			return nil, errors.Wrap(err, "could not write the remote signer key file")
		}
		keys = append(keys, added...)
		log.WithFields(logrus.Fields{
			"keyFile": km.keyFile,
			"added":   len(added),
			"keys":    len(keys),
		}).Info("Added validating public keys to the remote signer key file")
	}
	km.setKeys(keys)
	return statuses, nil
}

// DeletePublicKeys removes public keys from the key file and from the
// validating keys, and notifies subscribers. A key that is not present, or
// repeated in pubKeys, is reported as not found. The key file is read again
// and replaced as in AddPublicKeys; if that fails, nothing changes and the
// error is returned. Only keys that come from a key file can be deleted
// (ErrKeyFileRequired).
func (km *Keymanager) DeletePublicKeys(_ context.Context, pubKeys [][field_params.MLDSA87PubkeyLength]byte) ([]*qrlpbservice.DeletedRemoteKeysStatus, error) {
	if km.keyFile == "" {
		return nil, ErrKeyFileRequired
	}
	km.updateLock.Lock()
	defer km.updateLock.Unlock()
	// An invalid file is an error, so that it is not overwritten: the
	// operator may be in the middle of editing it.
	data, keys, err := readKeyFile(km.keyFile)
	if err != nil {
		return nil, err
	}
	present := make(map[[field_params.MLDSA87PubkeyLength]byte]bool, len(keys))
	for _, key := range keys {
		present[key] = true
	}
	statuses := make([]*qrlpbservice.DeletedRemoteKeysStatus, len(pubKeys))
	deleted := make(map[[field_params.MLDSA87PubkeyLength]byte]bool, len(pubKeys))
	for i, pubKey := range pubKeys {
		if !present[pubKey] || deleted[pubKey] {
			statuses[i] = &qrlpbservice.DeletedRemoteKeysStatus{
				Status:  qrlpbservice.DeletedRemoteKeysStatus_NOT_FOUND,
				Message: fmt.Sprintf("Pubkey: %v not found", hexutil.Encode(pubKey[:])),
			}
			continue
		}
		deleted[pubKey] = true
		statuses[i] = &qrlpbservice.DeletedRemoteKeysStatus{
			Status:  qrlpbservice.DeletedRemoteKeysStatus_DELETED,
			Message: fmt.Sprintf("Successfully deleted pubkey: %v", hexutil.Encode(pubKey[:])),
		}
	}
	if len(deleted) > 0 {
		if err := replaceKeyFile(km.keyFile, removeKeyFileLines(data, deleted)); err != nil {
			return nil, errors.Wrap(err, "could not write the remote signer key file")
		}
		keys = slices.DeleteFunc(keys, func(key [field_params.MLDSA87PubkeyLength]byte) bool { return deleted[key] })
		log.WithFields(logrus.Fields{
			"keyFile": km.keyFile,
			"deleted": len(deleted),
			"keys":    len(keys),
		}).Info("Deleted validating public keys from the remote signer key file")
	}
	km.setKeys(keys)
	return statuses, nil
}
