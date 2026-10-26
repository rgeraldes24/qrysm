package accounts

import (
	"testing"
	"time"

	"github.com/theQRL/qrysm/cmd/validator/accounts/exittest"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/testing/require"
)

// TestAccountsExit_RealRemoteSigner runs the same exits against a real
// go-remotesigner: with --voluntary-exit-signing-enabled, without it, and with
// it but without the network's chain config given to the command. It runs only
// when REMOTESIGNER_BINARY names the signer binary, for example the one
// `make build-consensus` builds in a go-remotesigner checkout:
//
//	REMOTESIGNER_BINARY=/path/to/go-remotesigner/bin/remotesigner-consensus \
//		go test ./cmd/validator/accounts -run TestAccountsExit_RealRemoteSigner
func TestAccountsExit_RealRemoteSigner(t *testing.T) {
	binary := exittest.RealRemoteSignerBinary(t)
	key, err := ml_dsa_87.RandKey()
	require.NoError(t, err)
	pubKey := key.PublicKey().Marshal()
	genesisTime := time.Unix(time.Now().Add(-time.Hour).Unix(), 0)
	// The signer, the beacon node and the command run on one network, whose
	// GENESIS_FORK_VERSION is not mainnet's: the chain config, in qrysm's own
	// format, given to the signer and to the command, with the beacon node's
	// genesis time and validators root.
	network := exittest.WriteChainConfig(t, exittest.DevnetForkVersion)
	for _, tc := range remoteSignerExitCases {
		t.Run(tc.name, func(t *testing.T) {
			beacon := exittest.StartBeaconNode(t, pubKey, genesisTime, exittest.DevnetForkVersion)
			signerURL, tokenFile := exittest.StartRealRemoteSigner(t, binary, key, network, genesisTime, tc.exitsEnabled)
			config := ""
			if tc.withChainConfig {
				config = network
			}
			err := runRemoteSignerExit(t, beacon, signerURL, tokenFile, pubKey, config)
			assertRemoteSignerExit(t, beacon, err, tc)
		})
	}
}
