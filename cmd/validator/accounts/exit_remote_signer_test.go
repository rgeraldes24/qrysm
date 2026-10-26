package accounts

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/cmd/validator/accounts/exittest"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/urfave/cli/v2"
)

// runRemoteSignerExit runs AccountsExit with the flags of accounts
// voluntary-exit, exiting the beacon node's validator through the remote
// signer at signerURL.
func runRemoteSignerExit(t *testing.T, beacon *exittest.BeaconNode, signerURL, tokenFile string, pubKey []byte) error {
	t.Helper()
	var command *cli.Command
	for _, c := range Commands.Subcommands {
		if c.Name == "voluntary-exit" {
			command = c
		}
	}
	require.NotNil(t, command)
	set := flag.NewFlagSet(command.Name, flag.ContinueOnError)
	for _, f := range command.Flags {
		require.NoError(t, f.Apply(set))
	}
	key := hexutil.Encode(pubKey)
	require.NoError(t, set.Parse([]string{
		"--" + flags.Web3SignerURLFlag.Name, signerURL,
		"--" + flags.Web3SignerPublicValidatorKeysFlag.Name, key,
		"--" + flags.Web3SignerAuthTokenFileFlag.Name, tokenFile,
		"--" + flags.BeaconRPCProviderFlag.Name, beacon.Addr,
		"--" + flags.VoluntaryExitPublicKeysFlag.Name, key,
		"--" + flags.ForceExitFlag.Name,
	}))
	return AccountsExit(cli.NewContext(&cli.App{}, set, nil), strings.NewReader(""))
}

// assertRemoteSignerExit checks the outcome of runRemoteSignerExit: one exit
// with a valid signature reached the beacon node when the signer signs exits,
// and otherwise the command failed naming the signer option and nothing was
// submitted.
func assertRemoteSignerExit(t *testing.T, beacon *exittest.BeaconNode, err error, exitsEnabled bool) {
	t.Helper()
	if exitsEnabled {
		require.NoError(t, err)
		require.Equal(t, 1, beacon.ProposedExits())
		return
	}
	t.Logf("exit refused: %v", err)
	require.ErrorContains(t, "remote signer does not sign voluntary exits", err)
	require.ErrorContains(t, "--voluntary-exit-signing-enabled", err)
	require.ErrorContains(t, "ARTIFACT_TYPE_DISABLED", err)
	require.Equal(t, true, errors.Is(err, keymanager.ErrVoluntaryExitSigningDisabled))
	require.Equal(t, 0, beacon.ProposedExits())
}

func TestAccountsExit_RemoteSigner(t *testing.T) {
	key, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	pubKey := key.PublicKey().Marshal()
	forkVersion := params.BeaconConfig().GenesisForkVersion
	for _, exitsEnabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("exit signing enabled %v", exitsEnabled), func(t *testing.T) {
			beacon := exittest.StartBeaconNode(t, pubKey, time.Now().Add(-time.Hour), forkVersion)
			signerURL, tokenFile := exittest.StartRemoteSigner(t, key, forkVersion, exitsEnabled)
			err := runRemoteSignerExit(t, beacon, signerURL, tokenFile, pubKey)
			assertRemoteSignerExit(t, beacon, err, exitsEnabled)
		})
	}
}
