package remote_web3signer

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/async/event"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	qrlpbservice "github.com/theQRL/qrysm/proto/qrl/service"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/validator/accounts/petnames"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/internal"
)

// SetupConfig includes configuration values for initializing.
// a keymanager, such as passwords, the wallet, and more.
// Web3Signer contains one public keys option. Either through a URL or a static key list.
type SetupConfig struct {
	BaseEndpoint          string
	GenesisValidatorsRoot []byte

	// Either URL or keylist must be set.
	// If the URL is set, the keymanager will fetch the public keys from the URL.
	// caution: this option is susceptible to slashing if the web3signer's validator keys are shared across validators
	PublicKeysURL string

	// Either URL or keylist must be set.
	// a static list of public keys to be passed by the user to determine what accounts should sign.
	// This will provide a layer of safety against slashing if the web3signer is shared across validators.
	ProvidedPublicKeys [][field_params.MLDSA87PubkeyLength]byte
}

// Keymanager defines the web3signer keymanager.
type Keymanager struct {
	client                internal.HttpSignerClient
	genesisValidatorsRoot []byte
	publicKeysURL         string
	providedPublicKeys    [][field_params.MLDSA87PubkeyLength]byte
	accountsChangedFeed   *event.Feed
	publicKeysUrlCalled   bool
}

// NewKeymanager instantiates a new web3signer key manager.
func NewKeymanager(_ context.Context, cfg *SetupConfig) (*Keymanager, error) {
	if cfg.BaseEndpoint == "" {
		return nil, fmt.Errorf("invalid setup config: BaseEndpoint is empty")
	}
	client, err := internal.NewApiClient(cfg.BaseEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "could not create apiClient")
	}
	return &Keymanager{
		client:                internal.HttpSignerClient(client),
		genesisValidatorsRoot: cfg.GenesisValidatorsRoot,
		accountsChangedFeed:   new(event.Feed),
		publicKeysURL:         cfg.PublicKeysURL,
		providedPublicKeys:    cfg.ProvidedPublicKeys,
		publicKeysUrlCalled:   false,
	}, nil
}

// FetchValidatingPublicKeys fetches the validating public keys
// from the remote server or from the provided keys if there are no existing public keys set
// or provides the existing keys in the keymanager.
func (km *Keymanager) FetchValidatingPublicKeys(ctx context.Context) ([][field_params.MLDSA87PubkeyLength]byte, error) {
	if km.publicKeysURL != "" && !km.publicKeysUrlCalled {
		providedPublicKeys, err := km.client.GetPublicKeys(ctx, km.publicKeysURL)
		if err != nil {
			erroredResponsesTotal.Inc()
			return nil, errors.Wrapf(err, "could not get public keys from remote server url: %v", km.publicKeysURL)
		}
		// makes sure that if the public keys are deleted the validator does not call URL again.
		km.publicKeysUrlCalled = true
		km.providedPublicKeys = providedPublicKeys
	}
	return km.providedPublicKeys, nil
}

// Sign signs the message by using a remote web3signer server.
func (km *Keymanager) Sign(ctx context.Context, request *validatorpb.SignRequest) (ml_dsa_87.Signature, error) {
	signRequest, err := getSignRequestJson(request)
	if err != nil {
		erroredResponsesTotal.Inc()
		return nil, err
	}

	signRequestsTotal.Inc()

	return km.client.Sign(ctx, hexutil.Encode(request.PublicKey), signRequest)
}

// getSignRequestJson returns a json request based on the SignRequest type.
func getSignRequestJson(request *validatorpb.SignRequest) (internal.SignRequestJson, error) {
	if request == nil {
		return nil, errors.New("nil sign request provided")
	}

	type signRequest struct {
		Type        string        `json:"type,omitempty"`
		SigningRoot hexutil.Bytes `json:"signingRoot"`
	}

	if len(request.SigningRoot) != 32 {
		return nil, fmt.Errorf("invalid signing root length %d", len(request.SigningRoot))
	}

	var typ string
	switch request.Object.(type) {
	case *validatorpb.SignRequest_AttestationData:
		typ = "ATTESTATION"
		attestationSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_AggregateAttestationAndProof:
		typ = "AGGREGATE_AND_PROOF"
		aggregateAndProofSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_Slot:
		typ = "AGGREGATION_SLOT"
		aggregationSlotSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_BlockCapella:
		typ = "BLOCK"
		blockCapellaSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_BlindedBlockCapella:
		typ = "BLINDED_BLOCK"
		blindedBlockCapellaSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_Epoch:
		typ = "RANDAO_REVEAL"
		randaoRevealSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_Exit:
		typ = "VOLUNTARY_EXIT"
		voluntaryExitSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_SyncMessageBlockRoot:
		typ = "SYNC_COMMITTEE_MESSAGE"
		syncCommitteeMessageSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_SyncAggregatorSelectionData:
		typ = "SYNC_COMMITTEE_SELECTION_PROOF"
		syncCommitteeSelectionProofSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_ContributionAndProof:
		typ = "SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF"
		syncCommitteeContributionAndProofSignRequestsTotal.Inc()
	case *validatorpb.SignRequest_Registration:
		typ = "VALIDATOR_REGISTRATION"
		validatorRegistrationSignRequestsTotal.Inc()
	default:
		return nil, fmt.Errorf("web3signer sign request type %T not supported", request.Object)
	}

	return json.Marshal(signRequest{
		Type:        typ,
		SigningRoot: request.SigningRoot,
	})
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
func (km *Keymanager) DeleteKeystores(context.Context, [][]byte) ([]*qrlpbservice.DeletedKeystoreStatus, error) {
	return nil, errors.New("Wrong wallet type: web3-signer. Only Imported or Derived wallets can delete accounts")
}

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
	//TODO: add config options, may require refactor again
	validatingPubKeys, err := km.FetchValidatingPublicKeys(ctx)
	if err != nil {
		return errors.Wrap(err, "could not fetch validating public keys")
	}
	if len(validatingPubKeys) == 1 {
		fmt.Print("Showing 1 validator account\n")
	} else if len(validatingPubKeys) == 0 {
		fmt.Print("No accounts found\n")
		return nil
	} else {
		fmt.Printf("Showing %d validator accounts\n", len(validatingPubKeys))
	}
	DisplayRemotePublicKeys(validatingPubKeys)
	return nil
}

// DisplayRemotePublicKeys prints remote public keys to stdout.
func DisplayRemotePublicKeys(validatingPubKeys [][field_params.MLDSA87PubkeyLength]byte) {
	au := aurora.NewAurora(true)
	for i := 0; i < len(validatingPubKeys); i++ {
		fmt.Println("")
		fmt.Printf(
			"%s\n", au.BrightGreen(petnames.DeterministicName(validatingPubKeys[i][:], "-")).Bold(),
		)
		// Retrieve the validating key account metadata.
		fmt.Printf("%s %#x\n", au.BrightCyan("[validating public key]").Bold(), validatingPubKeys[i])
		fmt.Println(" ")
	}
}
