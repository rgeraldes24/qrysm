package accounts

import (
	"context"
	"io"
	"strings"

	"github.com/pkg/errors"
	grpcutil "github.com/theQRL/qrysm/api/grpc"
	"github.com/theQRL/qrysm/cmd"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/validator/accounts"
	"github.com/theQRL/qrysm/validator/accounts/wallet"
	"github.com/theQRL/qrysm/validator/client"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/local"
	"github.com/theQRL/qrysm/validator/node"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func AccountsExit(c *cli.Context, r io.Reader) error {
	var w *wallet.Wallet
	var km keymanager.IKeymanager
	var err error
	dialOpts := client.ConstructDialOptions(
		c.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name),
		c.String(flags.CertFlag.Name),
		c.Uint(flags.GrpcRetriesFlag.Name),
		c.Duration(flags.GrpcRetryDelayFlag.Name),
	)
	grpcHeaders := strings.Split(c.String(flags.GrpcHeadersFlag.Name), ",")
	beaconRPCProvider := c.String(flags.BeaconRPCProviderFlag.Name)
	if !c.IsSet(flags.Web3SignerURLFlag.Name) && !c.IsSet(flags.WalletDirFlag.Name) && !c.IsSet(flags.InteropNumValidators.Name) {
		return errors.Errorf("No validators found, please provide a qrysm wallet directory via flag --%s "+
			"or a remote signer URL via flag --%s with its public keys via flag --%s or --%s",
			flags.WalletDirFlag.Name,
			flags.Web3SignerURLFlag.Name,
			flags.Web3SignerKeyFileFlag.Name,
			flags.Web3SignerPublicValidatorKeysFlag.Name,
		)
	}
	if c.IsSet(flags.InteropNumValidators.Name) {
		km, err = local.NewInteropKeymanager(c.Context, c.Uint64(flags.InteropStartIndex.Name), c.Uint64(flags.InteropNumValidators.Name))
		if err != nil {
			return errors.Wrap(err, "could not generate interop keys for key manager")
		}
		w = &wallet.Wallet{}
	} else if c.IsSet(flags.Web3SignerURLFlag.Name) {
		// The exits are signed by the remote signer, which only signs them
		// when it runs with --voluntary-exit-signing-enabled.
		config, err := node.Web3SignerConfig(c)
		if err != nil {
			return errors.Wrap(err, "could not configure the remote signer")
		}
		config.GenesisValidatorsRoot, err = genesisValidatorsRoot(c.Context, beaconRPCProvider, dialOpts, grpcHeaders)
		if err != nil {
			return err
		}
		w, km, err = walletWithWeb3SignerKeymanager(c, config)
		if err != nil {
			return err
		}
	} else {
		w, km, err = walletWithKeymanager(c)
		if err != nil {
			return err
		}
	}

	opts := []accounts.Option{
		accounts.WithWallet(w),
		accounts.WithKeymanager(km),
		accounts.WithGRPCDialOpts(dialOpts),
		accounts.WithBeaconRPCProvider(beaconRPCProvider),
		accounts.WithBeaconRESTApiProvider(c.String(flags.BeaconRESTApiProviderFlag.Name)),
		accounts.WithGRPCHeaders(grpcHeaders),
		accounts.WithExitJSONOutputPath(c.String(flags.VoluntaryExitJSONOutputPath.Name)),
	}
	// Get full set of public keys from the keymanager.
	validatingPublicKeys, err := km.FetchValidatingPublicKeys(c.Context)
	if err != nil {
		return err
	}
	if len(validatingPublicKeys) == 0 {
		return errors.New("wallet is empty, no accounts to delete")
	}
	// Filter keys either from CLI flag or from interactive session.
	rawPubKey, formattedPubKeys, err := accounts.FilterExitAccountsFromUserInput(c, r, validatingPublicKeys, c.Bool(flags.ForceExitFlag.Name))
	if err != nil {
		return errors.Wrap(err, "could not filter public keys for deletion")
	}
	opts = append(opts, accounts.WithRawPubKeys(rawPubKey))
	opts = append(opts, accounts.WithFormattedPubKeys(formattedPubKeys))
	acc, err := accounts.NewCLIManager(opts...)
	if err != nil {
		return err
	}
	return acc.Exit(c.Context)
}

// genesisValidatorsRoot asks the beacon node for the chain's genesis
// validators root, which the remote signer keymanager puts in the fork_info of
// every request.
func genesisValidatorsRoot(
	ctx context.Context, beaconRPCProvider string, dialOpts []grpc.DialOption, grpcHeaders []string,
) ([]byte, error) {
	ctx = grpcutil.AppendHeaders(ctx, grpcHeaders)
	conn, err := grpc.DialContext(ctx, beaconRPCProvider, dialOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "could not dial endpoint %s", beaconRPCProvider)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.WithError(err).Error("Failed to close connection")
		}
	}()
	resp, err := qrysmpb.NewNodeClient(conn).GetGenesis(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get genesis info")
	}
	return resp.GenesisValidatorsRoot, nil
}
