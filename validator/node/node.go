// Package node is the main process which handles the lifecycle of
// the runtime services in a validator client process, gracefully shutting
// everything down upon close.
package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	fastssz "github.com/prysmaticlabs/fastssz"
	"github.com/sirupsen/logrus"
	"github.com/theQRL/go-zond/common"
	"github.com/theQRL/go-zond/common/hexutil"
	"github.com/theQRL/qrysm/api/gateway"
	"github.com/theQRL/qrysm/api/gateway/apimiddleware"
	"github.com/theQRL/qrysm/async/event"
	"github.com/theQRL/qrysm/cmd"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/config/features"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	validatorServiceConfig "github.com/theQRL/qrysm/config/validator/service"
	"github.com/theQRL/qrysm/consensus-types/validator"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/io/file"
	"github.com/theQRL/qrysm/monitoring/backup"
	"github.com/theQRL/qrysm/monitoring/prometheus"
	tracing2 "github.com/theQRL/qrysm/monitoring/tracing"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	zondpbservice "github.com/theQRL/qrysm/proto/zond/service"
	"github.com/theQRL/qrysm/runtime"
	"github.com/theQRL/qrysm/runtime/debug"
	"github.com/theQRL/qrysm/runtime/prereqs"
	"github.com/theQRL/qrysm/runtime/version"
	"github.com/theQRL/qrysm/validator/accounts/wallet"
	"github.com/theQRL/qrysm/validator/client"
	"github.com/theQRL/qrysm/validator/db/iface"
	"github.com/theQRL/qrysm/validator/db/kv"
	g "github.com/theQRL/qrysm/validator/graffiti"
	"github.com/theQRL/qrysm/validator/keymanager/local"
	"github.com/theQRL/qrysm/validator/rpc"
	validatormiddleware "github.com/theQRL/qrysm/validator/rpc/apimiddleware"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"
)

// ValidatorClient defines an instance of a Zond validator that manages
// the entire lifecycle of services attached to it participating in proof of stake.
type ValidatorClient struct {
	cliCtx            *cli.Context
	ctx               context.Context
	cancel            context.CancelFunc
	db                *kv.Store
	services          *runtime.ServiceRegistry // Lifecycle and service store.
	lock              sync.RWMutex
	wallet            *wallet.Wallet
	walletInitialized *event.Feed
	stop              chan struct{} // Channel to wait for termination notifications.
}

// NewValidatorClient creates a new instance of the Qrysm validator client.
func NewValidatorClient(cliCtx *cli.Context) (*ValidatorClient, error) {
	// TODO(#9883) - Maybe we can pass in a new validator client config instead of the cliCTX to abstract away the use of flags here .
	if err := tracing2.Setup(
		"validator", // service name
		cliCtx.String(cmd.TracingProcessNameFlag.Name),
		cliCtx.String(cmd.TracingEndpointFlag.Name),
		cliCtx.Float64(cmd.TraceSampleFractionFlag.Name),
		cliCtx.Bool(cmd.EnableTracingFlag.Name),
	); err != nil {
		return nil, err
	}

	verbosity := cliCtx.String(cmd.VerbosityFlag.Name)
	level, err := logrus.ParseLevel(verbosity)
	if err != nil {
		return nil, err
	}
	logrus.SetLevel(level)

	// Warn if user's platform is not supported
	prereqs.WarnIfPlatformNotSupported(cliCtx.Context)

	registry := runtime.NewServiceRegistry()
	ctx, cancel := context.WithCancel(cliCtx.Context)
	validatorClient := &ValidatorClient{
		cliCtx:            cliCtx,
		ctx:               ctx,
		cancel:            cancel,
		services:          registry,
		walletInitialized: new(event.Feed),
		stop:              make(chan struct{}),
	}

	if err := features.ConfigureValidator(cliCtx); err != nil {
		return nil, err
	}
	if err := cmd.ConfigureValidator(cliCtx); err != nil {
		return nil, err
	}

	if cliCtx.IsSet(cmd.ChainConfigFileFlag.Name) {
		chainConfigFileName := cliCtx.String(cmd.ChainConfigFileFlag.Name)
		if err := params.LoadChainConfigFile(chainConfigFileName, nil); err != nil {
			return nil, err
		}
	}

	configureFastSSZHashingAlgorithm()

	if err := validatorClient.initializeFromCLI(cliCtx); err != nil {
		return nil, err
	}

	return validatorClient, nil
}

// Start every service in the validator client.
func (c *ValidatorClient) Start() {
	c.lock.Lock()

	log.WithFields(logrus.Fields{
		"version": version.Version(),
	}).Info("Starting validator node")

	c.services.StartAll()

	stop := c.stop
	c.lock.Unlock()

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got interrupt, shutting down...")
		debug.Exit(c.cliCtx) // Ensure trace and CPU profile data are flushed.
		go c.Close()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.WithField("times", i-1).Info("Already shutting down, interrupt more to panic.")
			}
		}
		panic("Panic closing the validator client")
	}()

	// Wait for stop channel to be closed.
	<-stop
}

// Close handles graceful shutdown of the system.
func (c *ValidatorClient) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.services.StopAll()
	log.Info("Stopping Qrysm validator")
	c.cancel()
	close(c.stop)
}

func (c *ValidatorClient) initializeFromCLI(cliCtx *cli.Context) error {
	var err error
	dataDir := cliCtx.String(flags.WalletDirFlag.Name)
	if !cliCtx.IsSet(flags.InteropNumValidators.Name) {
		// Custom Check For Web3Signer
		/*
			if cliCtx.IsSet(flags.Web3SignerURLFlag.Name) {
				c.wallet = wallet.NewWalletForWeb3Signer()
			} else {
				w, err := wallet.OpenWalletOrElseCli(cliCtx, func(cliCtx *cli.Context) (*wallet.Wallet, error) {
					return nil, wallet.ErrNoWalletFound
				})
				if err != nil {
					return errors.Wrap(err, "could not open wallet")
				}
				c.wallet = w
				// TODO(#9883) - Remove this when we have a better way to handle this.
				log.WithFields(logrus.Fields{
					"wallet":          w.AccountsDir(),
					"keymanager-kind": w.KeymanagerKind().String(),
				}).Info("Opened validator wallet")
				dataDir = c.wallet.AccountsDir()
			}
		*/

		w, err := wallet.OpenWalletOrElseCli(cliCtx, func(cliCtx *cli.Context) (*wallet.Wallet, error) {
			return nil, wallet.ErrNoWalletFound
		})
		if err != nil {
			return errors.Wrap(err, "could not open wallet")
		}
		c.wallet = w
		// TODO(#9883) - Remove this when we have a better way to handle this.
		log.WithFields(logrus.Fields{
			"wallet":          w.AccountsDir(),
			"keymanager-kind": w.KeymanagerKind().String(),
		}).Info("Opened validator wallet")
		dataDir = c.wallet.AccountsDir()
	}
	if cliCtx.String(cmd.DataDirFlag.Name) != cmd.DefaultDataDir() {
		dataDir = cliCtx.String(cmd.DataDirFlag.Name)
	}
	clearFlag := cliCtx.Bool(cmd.ClearDB.Name)
	forceClearFlag := cliCtx.Bool(cmd.ForceClearDB.Name)
	if clearFlag || forceClearFlag {
		if dataDir == "" && c.wallet != nil {
			dataDir = c.wallet.AccountsDir()
			if dataDir == "" {
				// skipcq: RVV-A0003
				log.Fatal(
					"Could not determine your system'c HOME path, please specify a --datadir you wish " +
						"to use for your validator data",
				)
			}
		}
		if err := clearDB(cliCtx.Context, dataDir, forceClearFlag); err != nil {
			return err
		}
	} else {
		dataFile := filepath.Join(dataDir, kv.ProtectionDbFileName)
		if !file.FileExists(dataFile) {
			log.Warnf("Slashing protection file %s is missing.\n"+
				"If you changed your --wallet-dir or --datadir, please copy your previous \"validator.db\" file into your current --datadir.\n"+
				"Disregard this warning if this is the first time you are running this set of keys.", dataFile)
		}
	}
	log.WithField("databasePath", dataDir).Info("Checking DB")

	valDB, err := kv.NewKVStore(cliCtx.Context, dataDir, &kv.Config{
		PubKeys: nil,
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize db")
	}
	c.db = valDB
	if err := valDB.RunUpMigrations(cliCtx.Context); err != nil {
		return errors.Wrap(err, "could not run database migration")
	}

	if !cliCtx.Bool(cmd.DisableMonitoringFlag.Name) {
		if err := c.registerPrometheusService(cliCtx); err != nil {
			return err
		}
	}
	if err := c.registerValidatorService(cliCtx); err != nil {
		return err
	}
	if cliCtx.Bool(flags.EnableRPCFlag.Name) {
		if err := c.registerRPCService(cliCtx); err != nil {
			return err
		}
		if err := c.registerRPCGatewayService(cliCtx); err != nil {
			return err
		}
	}
	return nil
}

func (c *ValidatorClient) registerPrometheusService(cliCtx *cli.Context) error {
	var additionalHandlers []prometheus.Handler
	if cliCtx.IsSet(cmd.EnableBackupWebhookFlag.Name) {
		additionalHandlers = append(
			additionalHandlers,
			prometheus.Handler{
				Path:    "/db/backup",
				Handler: backup.BackupHandler(c.db, cliCtx.String(cmd.BackupWebhookOutputDir.Name)),
			},
		)
	}
	service := prometheus.NewService(
		fmt.Sprintf("%s:%d", c.cliCtx.String(cmd.MonitoringHostFlag.Name), c.cliCtx.Int(flags.MonitoringPortFlag.Name)),
		c.services,
		additionalHandlers...,
	)
	logrus.AddHook(prometheus.NewLogrusCollector())
	return c.services.RegisterService(service)
}

func (c *ValidatorClient) registerValidatorService(cliCtx *cli.Context) error {
	endpoint := c.cliCtx.String(flags.BeaconRPCProviderFlag.Name)
	dataDir := c.cliCtx.String(cmd.DataDirFlag.Name)
	logValidatorBalances := !c.cliCtx.Bool(flags.DisablePenaltyRewardLogFlag.Name)
	emitAccountMetrics := !c.cliCtx.Bool(flags.DisableAccountMetricsFlag.Name)
	cert := c.cliCtx.String(flags.CertFlag.Name)
	graffiti := c.cliCtx.String(flags.GraffitiFlag.Name)
	maxCallRecvMsgSize := c.cliCtx.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)
	grpcRetries := c.cliCtx.Uint(flags.GrpcRetriesFlag.Name)
	grpcRetryDelay := c.cliCtx.Duration(flags.GrpcRetryDelayFlag.Name)
	var interopKeysConfig *local.InteropKeymanagerConfig
	if c.cliCtx.IsSet(flags.InteropNumValidators.Name) {
		interopKeysConfig = &local.InteropKeymanagerConfig{
			Offset:           cliCtx.Uint64(flags.InteropStartIndex.Name),
			NumValidatorKeys: cliCtx.Uint64(flags.InteropNumValidators.Name),
		}
	}

	gStruct := &g.Graffiti{}
	var err error
	if c.cliCtx.IsSet(flags.GraffitiFileFlag.Name) {
		n := c.cliCtx.String(flags.GraffitiFileFlag.Name)
		gStruct, err = g.ParseGraffitiFile(n)
		if err != nil {
			log.WithError(err).Warn("Could not parse graffiti file")
		}
	}

	/*
		wsc, err := Web3SignerConfig(c.cliCtx)
		if err != nil {
			return err
		}
	*/

	bpc, err := proposerSettings(c.cliCtx, c.db)
	if err != nil {
		return err
	}

	v, err := client.NewValidatorService(c.cliCtx.Context, &client.Config{
		Endpoint:                   endpoint,
		DataDir:                    dataDir,
		LogValidatorBalances:       logValidatorBalances,
		EmitAccountMetrics:         emitAccountMetrics,
		CertFlag:                   cert,
		GraffitiFlag:               g.ParseHexGraffiti(graffiti),
		GrpcMaxCallRecvMsgSizeFlag: maxCallRecvMsgSize,
		GrpcRetriesFlag:            grpcRetries,
		GrpcRetryDelay:             grpcRetryDelay,
		GrpcHeadersFlag:            c.cliCtx.String(flags.GrpcHeadersFlag.Name),
		ValDB:                      c.db,
		InteropKeysConfig:          interopKeysConfig,
		Wallet:                     c.wallet,
		WalletInitializedFeed:      c.walletInitialized,
		GraffitiStruct:             gStruct,
		// Web3SignerConfig:           wsc,
		ProposerSettings:  bpc,
		BeaconApiTimeout:  time.Second * 30,
		BeaconApiEndpoint: c.cliCtx.String(flags.BeaconRESTApiProviderFlag.Name),
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize validator service")
	}

	return c.services.RegisterService(v)
}

/*
func Web3SignerConfig(cliCtx *cli.Context) (*remoteweb3signer.SetupConfig, error) {
	var web3signerConfig *remoteweb3signer.SetupConfig
	if cliCtx.IsSet(flags.Web3SignerURLFlag.Name) {
		urlStr := cliCtx.String(flags.Web3SignerURLFlag.Name)
		u, err := url.ParseRequestURI(urlStr)
		if err != nil {
			return nil, errors.Wrapf(err, "web3signer url %s is invalid", urlStr)
		}
		if u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("web3signer url must be in the format of http(s)://host:port url used: %v", urlStr)
		}
		web3signerConfig = &remoteweb3signer.SetupConfig{
			BaseEndpoint:          u.String(),
			GenesisValidatorsRoot: nil,
		}
		if cliCtx.IsSet(flags.WalletPasswordFileFlag.Name) {
			log.Warnf("%s was provided while using web3signer and will be ignored", flags.WalletPasswordFileFlag.Name)
		}

		if publicKeysSlice := cliCtx.StringSlice(flags.Web3SignerPublicValidatorKeysFlag.Name); len(publicKeysSlice) > 0 {
			pks := make([]string, 0)
			if len(publicKeysSlice) == 1 {
				pURL, err := url.ParseRequestURI(publicKeysSlice[0])
				if err == nil && pURL.Scheme != "" && pURL.Host != "" {
					web3signerConfig.PublicKeysURL = publicKeysSlice[0]
				} else {
					pks = strings.Split(publicKeysSlice[0], ",")
				}
			} else if len(publicKeysSlice) > 1 {
				pks = publicKeysSlice
			}
			if len(pks) > 0 {
				pks = slice.Unique[string](pks)
				var validatorKeys [][field_params.DilithiumPubkeyLength]byte
				for _, key := range pks {
					decodedKey, decodeErr := hexutil.Decode(key)
					if decodeErr != nil {
						return nil, errors.Wrapf(decodeErr, "could not decode public key for web3signer: %s", key)
					}
					validatorKeys = append(validatorKeys, bytesutil.ToBytes2592(decodedKey))
				}
				web3signerConfig.ProvidedPublicKeys = validatorKeys
			}
		}
	}
	return web3signerConfig, nil
}
*/

func proposerSettings(cliCtx *cli.Context, db iface.ValidatorDB) (*validatorServiceConfig.ProposerSettings, error) {
	var fileConfig *validatorpb.ProposerSettingsPayload

	if cliCtx.IsSet(flags.ProposerSettingsFlag.Name) && cliCtx.IsSet(flags.ProposerSettingsURLFlag.Name) {
		return nil, errors.New("cannot specify both " + flags.ProposerSettingsFlag.Name + " and " + flags.ProposerSettingsURLFlag.Name)
	}
	builderConfigFromFlag, err := BuilderSettingsFromFlags(cliCtx)
	if err != nil {
		return nil, err
	}
	// is overridden by file and URL flags
	if cliCtx.IsSet(flags.SuggestedFeeRecipientFlag.Name) &&
		!cliCtx.IsSet(flags.ProposerSettingsFlag.Name) &&
		!cliCtx.IsSet(flags.ProposerSettingsURLFlag.Name) {
		suggestedFee := cliCtx.String(flags.SuggestedFeeRecipientFlag.Name)
		fileConfig = &validatorpb.ProposerSettingsPayload{
			ProposerConfig: nil,
			DefaultConfig: &validatorpb.ProposerOptionPayload{
				FeeRecipient: suggestedFee,
				Builder:      builderConfigFromFlag.ToPayload(),
			},
		}
	}

	if cliCtx.IsSet(flags.ProposerSettingsFlag.Name) {
		if err := unmarshalFromFile(cliCtx.Context, cliCtx.String(flags.ProposerSettingsFlag.Name), &fileConfig); err != nil {
			return nil, err
		}
	}
	if cliCtx.IsSet(flags.ProposerSettingsURLFlag.Name) {
		if err := unmarshalFromURL(cliCtx.Context, cliCtx.String(flags.ProposerSettingsURLFlag.Name), &fileConfig); err != nil {
			return nil, err
		}
	}

	// this condition triggers if SuggestedFeeRecipientFlag,ProposerSettingsFlag or ProposerSettingsURLFlag did not create any settings
	if fileConfig == nil {
		// Checks the db or enable builder settings before starting the node without proposer settings
		// starting the node without proposer settings, will skip API calls for push proposer settings and register validator
		return handleNoProposerSettingsFlagsProvided(cliCtx, db, builderConfigFromFlag)
	}
	// convert file config to proposer config for internal use
	vpSettings := &validatorServiceConfig.ProposerSettings{}

	// default fileConfig is mandatory
	if fileConfig.DefaultConfig == nil {
		return nil, errors.New("default fileConfig is required, proposer settings file is either empty or an incorrect format")
	}
	recipient, err := common.NewAddressFromString(fileConfig.DefaultConfig.FeeRecipient)
	if err != nil {
		return nil, errors.New("default fileConfig fee recipient is not a valid zond address")
	}
	psExists, err := db.ProposerSettingsExists(cliCtx.Context)
	if err != nil {
		return nil, err
	}
	if err := warnNonChecksummedAddress(fileConfig.DefaultConfig.FeeRecipient); err != nil {
		return nil, err
	}
	vpSettings.DefaultConfig = &validatorServiceConfig.ProposerOption{
		FeeRecipientConfig: &validatorServiceConfig.FeeRecipientConfig{
			FeeRecipient: recipient,
		},
		BuilderConfig: validatorServiceConfig.ToBuilderConfig(fileConfig.DefaultConfig.Builder),
	}

	if builderConfigFromFlag != nil {
		config := builderConfigFromFlag.Clone()
		if config.GasLimit == validator.Uint64(params.BeaconConfig().DefaultBuilderGasLimit) && vpSettings.DefaultConfig.BuilderConfig != nil {
			config.GasLimit = vpSettings.DefaultConfig.BuilderConfig.GasLimit
		}
		vpSettings.DefaultConfig.BuilderConfig = config
	} else if vpSettings.DefaultConfig.BuilderConfig != nil {
		vpSettings.DefaultConfig.BuilderConfig.GasLimit = reviewGasLimit(vpSettings.DefaultConfig.BuilderConfig.GasLimit)
	}

	if psExists {
		// if settings exist update the default
		if err := db.UpdateProposerSettingsDefault(cliCtx.Context, vpSettings.DefaultConfig); err != nil {
			return nil, err
		}
	}

	if fileConfig.ProposerConfig != nil && len(fileConfig.ProposerConfig) != 0 {
		vpSettings.ProposeConfig = make(map[[field_params.DilithiumPubkeyLength]byte]*validatorServiceConfig.ProposerOption)
		for key, option := range fileConfig.ProposerConfig {
			decodedKey, err := hexutil.Decode(key)
			if err != nil {
				return nil, errors.Wrapf(err, "could not decode public key %s", key)
			}
			if len(decodedKey) != field_params.DilithiumPubkeyLength {
				return nil, fmt.Errorf("%v  is not a dilithium public key", key)
			}
			if err := verifyOption(key, option); err != nil {
				return nil, err
			}
			currentBuilderConfig := validatorServiceConfig.ToBuilderConfig(option.Builder)
			if builderConfigFromFlag != nil {
				config := builderConfigFromFlag.Clone()
				if config.GasLimit == validator.Uint64(params.BeaconConfig().DefaultBuilderGasLimit) && currentBuilderConfig != nil {
					config.GasLimit = currentBuilderConfig.GasLimit
				}
				currentBuilderConfig = config
			} else if currentBuilderConfig != nil {
				currentBuilderConfig.GasLimit = reviewGasLimit(currentBuilderConfig.GasLimit)
			}
			recipient, err := common.NewAddressFromString(option.FeeRecipient)
			if err != nil {
				return nil, err
			}
			o := &validatorServiceConfig.ProposerOption{
				FeeRecipientConfig: &validatorServiceConfig.FeeRecipientConfig{
					FeeRecipient: recipient,
				},
				BuilderConfig: currentBuilderConfig,
			}
			pubkeyB := bytesutil.ToBytes2592(decodedKey)
			vpSettings.ProposeConfig[pubkeyB] = o
		}
		if psExists {
			// override the existing saved settings if providing values via fileConfig.ProposerConfig
			if err := db.SaveProposerSettings(cliCtx.Context, vpSettings); err != nil {
				return nil, err
			}
		}
	}
	if !psExists {
		// if no proposer settings ever existed in the db just save the settings
		if err := db.SaveProposerSettings(cliCtx.Context, vpSettings); err != nil {
			return nil, err
		}
	}
	return vpSettings, nil
}

func verifyOption(key string, option *validatorpb.ProposerOptionPayload) error {
	if option == nil {
		return fmt.Errorf("fee recipient is required for proposer %s", key)
	}
	if !common.IsAddress(option.FeeRecipient) {
		return errors.New("fee recipient is not a valid zond address")
	}
	if err := warnNonChecksummedAddress(option.FeeRecipient); err != nil {
		return err
	}
	return nil
}

func handleNoProposerSettingsFlagsProvided(cliCtx *cli.Context,
	db iface.ValidatorDB,
	builderConfigFromFlag *validatorServiceConfig.BuilderConfig) (*validatorServiceConfig.ProposerSettings, error) {
	log.Info("no proposer settings files have been provided, attempting to load from db.")
	// checks db if proposer settings exist if none is provided.
	settings, err := db.ProposerSettings(cliCtx.Context)
	if err == nil {
		// process any overrides to builder settings
		overrideBuilderSettings(settings, builderConfigFromFlag)
		// if settings are empty
		log.Info("successfully loaded proposer settings from db.")
		return settings, nil
	} else {
		log.WithError(err).Warn("no proposer settings will be loaded from the db")
	}

	if cliCtx.Bool(flags.EnableBuilderFlag.Name) {
		// if there are no proposer settings provided, create a default where fee recipient is not populated, this will be skipped for validator registration on validators that don't have a fee recipient set.
		// skip saving to DB if only builder settings are provided until a trigger like keymanager API updates with fee recipient values
		return &validatorServiceConfig.ProposerSettings{
			DefaultConfig: &validatorServiceConfig.ProposerOption{
				BuilderConfig: builderConfigFromFlag,
			},
		}, nil
	}
	return nil, nil
}

func overrideBuilderSettings(settings *validatorServiceConfig.ProposerSettings, builderConfigFromFlag *validatorServiceConfig.BuilderConfig) {
	// override the db settings with the results based on whether the --enable-builder flag is provided.
	if builderConfigFromFlag == nil {
		log.Infof("proposer settings loaded from db. validator registration to builder is not enabled, please use the --%s flag if you wish to use a builder.", flags.EnableBuilderFlag.Name)
	}
	if settings.ProposeConfig != nil {
		for key := range settings.ProposeConfig {
			settings.ProposeConfig[key].BuilderConfig = builderConfigFromFlag
		}
	}
	if settings.DefaultConfig != nil {
		settings.DefaultConfig.BuilderConfig = builderConfigFromFlag
	}
}

func BuilderSettingsFromFlags(cliCtx *cli.Context) (*validatorServiceConfig.BuilderConfig, error) {
	if cliCtx.Bool(flags.EnableBuilderFlag.Name) {
		gasLimit := validator.Uint64(params.BeaconConfig().DefaultBuilderGasLimit)
		sgl := cliCtx.String(flags.BuilderGasLimitFlag.Name)

		if sgl != "" {
			gl, err := strconv.ParseUint(sgl, 10, 64)
			if err != nil {
				return nil, errors.New("Gas Limit is not a uint64")
			}
			gasLimit = reviewGasLimit(validator.Uint64(gl))
		}
		return &validatorServiceConfig.BuilderConfig{
			Enabled:  true,
			GasLimit: gasLimit,
		}, nil
	}
	return nil, nil
}

func warnNonChecksummedAddress(feeRecipient string) error {
	mixedcaseAddress, err := common.NewMixedcaseAddressFromString(feeRecipient)
	if err != nil {
		return errors.Wrapf(err, "could not decode fee recipient %s", feeRecipient)
	}
	if !mixedcaseAddress.ValidChecksum() {
		log.Warnf("Fee recipient %s is not a checksum Zond address. "+
			"The checksummed address is %s and will be used as the fee recipient. "+
			"We recommend using a mixed-case address (checksum) "+
			"to prevent spelling mistakes in your fee recipient Zond address", feeRecipient, mixedcaseAddress.Address().Hex())
	}
	return nil
}

func reviewGasLimit(gasLimit validator.Uint64) validator.Uint64 {
	// sets gas limit to default if not defined or set to 0
	if gasLimit == 0 {
		return validator.Uint64(params.BeaconConfig().DefaultBuilderGasLimit)
	}
	// TODO(10810): add in warning for ranges
	return gasLimit
}

func (c *ValidatorClient) registerRPCService(cliCtx *cli.Context) error {
	var vs *client.ValidatorService
	if err := c.services.FetchService(&vs); err != nil {
		return err
	}
	validatorGatewayHost := cliCtx.String(flags.GRPCGatewayHost.Name)
	validatorGatewayPort := cliCtx.Int(flags.GRPCGatewayPort.Name)
	validatorMonitoringHost := cliCtx.String(cmd.MonitoringHostFlag.Name)
	validatorMonitoringPort := cliCtx.Int(flags.MonitoringPortFlag.Name)
	rpcHost := cliCtx.String(flags.RPCHost.Name)
	rpcPort := cliCtx.Int(flags.RPCPort.Name)
	nodeGatewayEndpoint := cliCtx.String(flags.BeaconRPCGatewayProviderFlag.Name)
	beaconClientEndpoint := cliCtx.String(flags.BeaconRPCProviderFlag.Name)
	maxCallRecvMsgSize := c.cliCtx.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)
	grpcRetries := c.cliCtx.Uint(flags.GrpcRetriesFlag.Name)
	grpcRetryDelay := c.cliCtx.Duration(flags.GrpcRetryDelayFlag.Name)
	walletDir := cliCtx.String(flags.WalletDirFlag.Name)
	grpcHeaders := c.cliCtx.String(flags.GrpcHeadersFlag.Name)
	clientCert := c.cliCtx.String(flags.CertFlag.Name)
	server := rpc.NewServer(cliCtx.Context, &rpc.Config{
		ValDB:                    c.db,
		Host:                     rpcHost,
		Port:                     fmt.Sprintf("%d", rpcPort),
		WalletInitializedFeed:    c.walletInitialized,
		ValidatorService:         vs,
		SyncChecker:              vs,
		GenesisFetcher:           vs,
		NodeGatewayEndpoint:      nodeGatewayEndpoint,
		WalletDir:                walletDir,
		Wallet:                   c.wallet,
		ValidatorGatewayHost:     validatorGatewayHost,
		ValidatorGatewayPort:     validatorGatewayPort,
		ValidatorMonitoringHost:  validatorMonitoringHost,
		ValidatorMonitoringPort:  validatorMonitoringPort,
		BeaconClientEndpoint:     beaconClientEndpoint,
		ClientMaxCallRecvMsgSize: maxCallRecvMsgSize,
		ClientGrpcRetries:        grpcRetries,
		ClientGrpcRetryDelay:     grpcRetryDelay,
		ClientGrpcHeaders:        strings.Split(grpcHeaders, ","),
		ClientWithCert:           clientCert,
	})
	return c.services.RegisterService(server)
}

func (c *ValidatorClient) registerRPCGatewayService(cliCtx *cli.Context) error {
	gatewayHost := cliCtx.String(flags.GRPCGatewayHost.Name)
	if gatewayHost != flags.DefaultGatewayHost {
		log.WithField("web-host", gatewayHost).Warn(
			"You are using a non-default web host. Web traffic is served by HTTP, so be wary of " +
				"changing this parameter if you are exposing this host to the Internet!",
		)
	}
	gatewayPort := cliCtx.Int(flags.GRPCGatewayPort.Name)
	rpcHost := cliCtx.String(flags.RPCHost.Name)
	rpcPort := cliCtx.Int(flags.RPCPort.Name)
	rpcAddr := net.JoinHostPort(rpcHost, fmt.Sprintf("%d", rpcPort))
	gatewayAddress := net.JoinHostPort(gatewayHost, fmt.Sprintf("%d", gatewayPort))
	timeout := cliCtx.Int(cmd.ApiTimeoutFlag.Name)
	var allowedOrigins []string
	if cliCtx.IsSet(flags.GPRCGatewayCorsDomain.Name) {
		allowedOrigins = strings.Split(cliCtx.String(flags.GPRCGatewayCorsDomain.Name), ",")
	} else {
		allowedOrigins = strings.Split(flags.GPRCGatewayCorsDomain.Value, ",")
	}
	maxCallSize := cliCtx.Uint64(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)

	registrations := []gateway.PbHandlerRegistration{
		zondpbservice.RegisterKeyManagementHandler,
	}
	gwmux := gwruntime.NewServeMux(
		gwruntime.WithMarshalerOption(gwruntime.MIMEWildcard, &gwruntime.HTTPBodyMarshaler{
			Marshaler: &gwruntime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					EmitUnpopulated: true,
					UseProtoNames:   true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					DiscardUnknown: true,
				},
			},
		}),
		gwruntime.WithMarshalerOption(
			"text/event-stream", &gwruntime.EventSourceJSONPb{},
		),
		gwruntime.WithForwardResponseOption(gateway.HttpResponseModifier),
	)
	muxHandler := func(apiMware *apimiddleware.ApiProxyMiddleware, h http.HandlerFunc, w http.ResponseWriter, req *http.Request) {
		// The validator gateway handler requires this special logic as it serves two kinds of APIs, namely
		// the standard validator keymanager API under the /zond namespace, and the Qrysm internal
		// validator API under the /api namespace. Finally, it also serves requests to host the validator web UI.
		if strings.HasPrefix(req.URL.Path, "/api/zond/") {
			req.URL.Path = strings.Replace(req.URL.Path, "/api", "", 1)
			// If the prefix has /zond/, we handle it with the standard API gateway middleware.
			apiMware.ServeHTTP(w, req)
		} else if strings.HasPrefix(req.URL.Path, "/api") {
			req.URL.Path = strings.Replace(req.URL.Path, "/api", "", 1)
			// Else, we handle with the Qrysm API gateway without a middleware.
			h(w, req)
		}
	}

	pbHandler := &gateway.PbMux{
		Registrations: registrations,
		Patterns:      []string{"/internal/zond/v1/"},
		Mux:           gwmux,
	}
	opts := []gateway.Option{
		gateway.WithRouter(mux.NewRouter()),
		gateway.WithRemoteAddr(rpcAddr),
		gateway.WithGatewayAddr(gatewayAddress),
		gateway.WithMaxCallRecvMsgSize(maxCallSize),
		gateway.WithPbHandlers([]*gateway.PbMux{pbHandler}),
		gateway.WithAllowedOrigins(allowedOrigins),
		gateway.WithApiMiddleware(&validatormiddleware.ValidatorEndpointFactory{}),
		gateway.WithMuxHandler(muxHandler),
		gateway.WithTimeout(uint64(timeout)),
	}
	gw, err := gateway.New(cliCtx.Context, opts...)
	if err != nil {
		return err
	}
	return c.services.RegisterService(gw)
}

func clearDB(ctx context.Context, dataDir string, force bool) error {
	var err error
	clearDBConfirmed := force

	if !force {
		actionText := "This will delete your validator's historical actions database stored in your data directory. " +
			"This may lead to potential slashing - do you want to proceed? (Y/N)"
		deniedText := "The historical actions database will not be deleted. No changes have been made."
		clearDBConfirmed, err = cmd.ConfirmAction(actionText, deniedText)
		if err != nil {
			return errors.Wrapf(err, "Could not clear DB in dir %s", dataDir)
		}
	}

	if clearDBConfirmed {
		valDB, err := kv.NewKVStore(ctx, dataDir, &kv.Config{})
		if err != nil {
			return errors.Wrapf(err, "Could not create DB in dir %s", dataDir)
		}
		if err := valDB.Close(); err != nil {
			return errors.Wrapf(err, "could not close DB in dir %s", dataDir)
		}

		log.Warning("Removing database")
		if err := valDB.ClearDB(); err != nil {
			return errors.Wrapf(err, "Could not clear DB in dir %s", dataDir)
		}
	}

	return nil
}

func unmarshalFromURL(ctx context.Context, from string, to interface{}) error {
	u, err := url.ParseRequestURI(from)
	if err != nil {
		return err
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL: %s", from)
	}
	req, reqerr := http.NewRequestWithContext(ctx, http.MethodGet, from, nil)
	if reqerr != nil {
		return errors.Wrap(reqerr, "failed to create http request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, resperr := http.DefaultClient.Do(req)
	if resperr != nil {
		return errors.Wrap(resperr, "failed to send http request")
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.WithError(err).Error("failed to close response body")
		}
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("http request to %v failed with status code %d", from, resp.StatusCode)
	}
	if decodeerr := json.NewDecoder(resp.Body).Decode(&to); decodeerr != nil {
		return errors.Wrap(decodeerr, "failed to decode http response")
	}
	return nil
}

func unmarshalFromFile(ctx context.Context, from string, to interface{}) error {
	if ctx == nil {
		return errors.New("node: nil context passed to unmarshalFromFile")
	}
	cleanpath := filepath.Clean(from)
	b, err := os.ReadFile(cleanpath)
	if err != nil {
		return errors.Wrap(err, "failed to open file")
	}

	if err := json.Unmarshal(b, to); err != nil {
		return errors.Wrap(err, "failed to unmarshal json file")
	}

	return nil
}

func configureFastSSZHashingAlgorithm() {
	fastssz.EnableVectorizedHTR = true
}
