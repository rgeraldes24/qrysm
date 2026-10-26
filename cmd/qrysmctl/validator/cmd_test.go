package validator

import (
	"testing"

	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/urfave/cli/v2"
)

// The exit command signs through a remote signer when given the validator
// client's remote signer flags.
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
	} {
		require.Equal(t, true, names[f.Names()[0]], "exit command lacks --%s", f.Names()[0])
	}
}
