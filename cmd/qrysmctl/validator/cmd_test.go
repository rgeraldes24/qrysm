package validator

import (
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/cmd"
	"github.com/theQRL/qrysm/cmd/validator/accounts/exittest"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/testing/require"
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

// devnetForkVersion is a GENESIS_FORK_VERSION other than mainnet's, the one of
// the qrl-package devnets.
var devnetForkVersion = []byte{0x10, 0x00, 0x00, 0x38}

// qrysmctl validator exit, run as the binary runs it, signs an exit through
// the remote signer for the network of --chain-config-file: the signer refuses
// a fork_info for another network.
func TestExitCommand_RemoteSignerOnChainConfigNetwork(t *testing.T) {
	key, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	pubKey := key.PublicKey().Marshal()
	beacon := exittest.StartBeaconNode(t, pubKey, time.Now().Add(-time.Hour), devnetForkVersion)
	signerURL, tokenFile := exittest.StartRemoteSigner(t, key, devnetForkVersion, true)

	err = exittest.RunCommand(t, &cli.App{Commands: Commands}, []string{
		"qrysmctl", "validator", "exit",
		"--" + cmd.AcceptTosFlag.Name,
		"--" + flags.ForceExitFlag.Name,
		"--" + cmd.ChainConfigFileFlag.Name, exittest.WriteChainConfig(t, devnetForkVersion),
		"--" + flags.Web3SignerURLFlag.Name, signerURL,
		"--" + flags.Web3SignerPublicValidatorKeysFlag.Name, hexutil.Encode(pubKey),
		"--" + flags.Web3SignerAuthTokenFileFlag.Name, tokenFile,
		"--" + flags.BeaconRPCProviderFlag.Name, beacon.Addr,
		"--" + flags.VoluntaryExitPublicKeysFlag.Name, hexutil.Encode(pubKey),
	})
	require.NoError(t, err)
	require.Equal(t, 1, beacon.ProposedExits())
}
