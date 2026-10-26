package accounts

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

// runRemoteSignerExit runs validator accounts voluntary-exit, as the
// validator binary runs it, exiting the beacon node's validator through the
// remote signer at signerURL. chainConfig is passed with the global
// --chain-config-file flag, before the accounts command, unless it is empty.
func runRemoteSignerExit(
	t *testing.T, beacon *exittest.BeaconNode, signerURL, tokenFile string, pubKey []byte, chainConfig string,
) error {
	t.Helper()
	app := &cli.App{
		Flags:    []cli.Flag{cmd.ChainConfigFileFlag},
		Commands: []*cli.Command{Commands},
	}
	args := []string{"validator"}
	if chainConfig != "" {
		args = append(args, "--"+cmd.ChainConfigFileFlag.Name, chainConfig)
	}
	key := hexutil.Encode(pubKey)
	args = append(args, "accounts", "voluntary-exit",
		"--"+cmd.AcceptTosFlag.Name,
		"--"+flags.ForceExitFlag.Name,
		"--"+flags.Web3SignerURLFlag.Name, signerURL,
		"--"+flags.Web3SignerPublicValidatorKeysFlag.Name, key,
		"--"+flags.Web3SignerAuthTokenFileFlag.Name, tokenFile,
		"--"+flags.BeaconRPCProviderFlag.Name, beacon.Addr,
		"--"+flags.VoluntaryExitPublicKeysFlag.Name, key,
	)
	return exittest.RunCommand(t, app, args)
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

// The exit is signed for the network of the global --chain-config-file, here
// one whose GENESIS_FORK_VERSION is not mainnet's.
func TestAccountsExit_RemoteSigner(t *testing.T) {
	key, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	pubKey := key.PublicKey().Marshal()
	chainConfig := exittest.WriteChainConfig(t, exittest.DevnetForkVersion)
	for _, exitsEnabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("exit signing enabled %v", exitsEnabled), func(t *testing.T) {
			beacon := exittest.StartBeaconNode(t, pubKey, time.Now().Add(-time.Hour), exittest.DevnetForkVersion)
			signerURL, tokenFile := exittest.StartRemoteSigner(t, key, exittest.DevnetForkVersion, exitsEnabled)
			err := runRemoteSignerExit(t, beacon, signerURL, tokenFile, pubKey, chainConfig)
			assertRemoteSignerExit(t, beacon, err, exitsEnabled)
		})
	}
}
