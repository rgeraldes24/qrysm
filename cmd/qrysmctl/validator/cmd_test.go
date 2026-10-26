package validator

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/cmd"
	"github.com/theQRL/qrysm/cmd/validator/accounts/exittest"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/urfave/cli/v2"
)

// The exit command signs through a remote signer when given the validator
// client's remote signer flags, for the network of --chain-config-file.
func TestExitCommand_RemoteSignerFlags(t *testing.T) {
	var exit *cli.Command
	for _, c := range Commands[0].Subcommands {
		if c.Name == "exit" {
			exit = c
		}
	}
	require.NotNil(t, exit)
	names := map[string]bool{}
	for _, f := range exit.Flags {
		for _, name := range f.Names() {
			names[name] = true
		}
	}
	for _, f := range []cli.Flag{
		flags.Web3SignerURLFlag,
		flags.Web3SignerPublicValidatorKeysFlag,
		flags.Web3SignerKeyFileFlag,
		flags.Web3SignerAuthTokenFileFlag,
		flags.Web3SignerTLSCAFileFlag,
		flags.Web3SignerTLSClientCertFileFlag,
		flags.Web3SignerTLSClientKeyFileFlag,
		flags.Web3SignerTimeoutFlag,
		cmd.ChainConfigFileFlag,
	} {
		require.Equal(t, true, names[f.Names()[0]], "exit command lacks --%s", f.Names()[0])
	}
}

// runExit runs qrysmctl validator exit, as the binary runs it, exiting the
// beacon node's validator through the remote signer at signerURL, with
// --chain-config-file unless chainConfig is empty.
func runExit(t *testing.T, beacon *exittest.BeaconNode, signerURL, tokenFile string, pubKey []byte, chainConfig string) error {
	t.Helper()
	args := []string{
		"qrysmctl", "validator", "exit",
		"--" + cmd.AcceptTosFlag.Name,
		"--" + flags.ForceExitFlag.Name,
		"--" + flags.Web3SignerURLFlag.Name, signerURL,
		"--" + flags.Web3SignerPublicValidatorKeysFlag.Name, hexutil.Encode(pubKey),
		"--" + flags.Web3SignerAuthTokenFileFlag.Name, tokenFile,
		"--" + flags.BeaconRPCProviderFlag.Name, beacon.Addr,
		"--" + flags.VoluntaryExitPublicKeysFlag.Name, hexutil.Encode(pubKey),
	}
	if chainConfig != "" {
		args = append(args, "--"+cmd.ChainConfigFileFlag.Name, chainConfig)
	}
	return exittest.RunCommand(t, &cli.App{Commands: Commands}, args)
}

// assertExit checks the outcome of runExit on a network other than mainnet:
// with the network's chain config the exit reached the beacon node; without
// it the exit was made for mainnet, the signer rejected it, and the command
// failed naming the chain config with nothing submitted.
func assertExit(t *testing.T, beacon *exittest.BeaconNode, err error, withChainConfig bool) {
	t.Helper()
	if withChainConfig {
		require.NoError(t, err)
		require.Equal(t, 1, beacon.ProposedExits())
		return
	}
	t.Logf("exit rejected: %v", err)
	require.Equal(t, true, errors.Is(err, keymanager.ErrVoluntaryExitNetworkMismatch), "got %v", err)
	require.ErrorContains(t, "FORK_INFO_MISMATCH", err)
	require.ErrorContains(t, "--chain-config-file", err)
	require.Equal(t, 0, beacon.ProposedExits())
}

// qrysmctl validator exit signs an exit through the remote signer for the
// network of --chain-config-file. Without it the command makes the exit for
// mainnet: the signer, on another network, rejects it, and the command stops
// with an error naming the chain config.
func TestExitCommand_RemoteSignerOnChainConfigNetwork(t *testing.T) {
	key, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	pubKey := key.PublicKey().Marshal()
	chainConfig := exittest.WriteChainConfig(t, exittest.DevnetForkVersion)
	for _, withChainConfig := range []bool{true, false} {
		t.Run(fmt.Sprintf("chain config %v", withChainConfig), func(t *testing.T) {
			beacon := exittest.StartBeaconNode(t, pubKey, time.Now().Add(-time.Hour), exittest.DevnetForkVersion)
			signerURL, tokenFile := exittest.StartRemoteSigner(t, key, exittest.DevnetForkVersion, true)
			config := ""
			if withChainConfig {
				config = chainConfig
			}
			err := runExit(t, beacon, signerURL, tokenFile, pubKey, config)
			assertExit(t, beacon, err, withChainConfig)
		})
	}
}

// TestExitCommand_RealRemoteSigner runs the same exits against a real
// go-remotesigner started with --voluntary-exit-signing-enabled. It runs only
// when REMOTESIGNER_BINARY names the signer binary:
//
//	REMOTESIGNER_BINARY=/path/to/go-remotesigner/bin/remotesigner-consensus \
//		go test ./cmd/qrysmctl/validator -run TestExitCommand_RealRemoteSigner
func TestExitCommand_RealRemoteSigner(t *testing.T) {
	binary := exittest.RealRemoteSignerBinary(t)
	key, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	pubKey := key.PublicKey().Marshal()
	genesisTime := time.Unix(time.Now().Add(-time.Hour).Unix(), 0)
	network := exittest.WriteChainConfig(t, exittest.DevnetForkVersion)
	for _, withChainConfig := range []bool{true, false} {
		t.Run(fmt.Sprintf("chain config %v", withChainConfig), func(t *testing.T) {
			beacon := exittest.StartBeaconNode(t, pubKey, genesisTime, exittest.DevnetForkVersion)
			signerURL, tokenFile := exittest.StartRealRemoteSigner(t, binary, key, network, genesisTime, true)
			config := ""
			if withChainConfig {
				config = network
			}
			err := runExit(t, beacon, signerURL, tokenFile, pubKey, config)
			assertExit(t, beacon, err, withChainConfig)
		})
	}
}
