package newseed

import (
	"crypto/sha512"
	"fmt"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/misc"
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/stakingdeposit"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

var (
	newSeedFlags = struct {
		ValidatorStartIndex uint64
		NumValidators       uint64
		Folder              string
		ChainName           string
		ExecutionAddress    string
		Mnemonic            string
	}{}
	log = logrus.WithField("prefix", "deposit")

	// KeystorePassword specifies the keystore password.
	KeystorePassword = &cli.StringFlag{
		Name:  "keystore-password",
		Usage: "The keystore password.",
		Value: "",
	}
)
var Commands = []*cli.Command{
	{
		Name:    "new-seed",
		Aliases: []string{"ns"},
		Usage:   "",
		Action: func(cliCtx *cli.Context) error {
			if err := cliActionNewSeed(cliCtx); err != nil {
				log.WithError(err).Fatal("Could not generate new seed")
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.Uint64Flag{
				Name:        "validator-start-index",
				Usage:       "",
				Destination: &newSeedFlags.ValidatorStartIndex,
				Value:       0,
			},
			&cli.Uint64Flag{
				Name:        "num-validators",
				Usage:       "",
				Destination: &newSeedFlags.NumValidators,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "folder",
				Usage:       "",
				Destination: &newSeedFlags.Folder,
				Value:       "validator_keys",
			},
			&cli.StringFlag{
				Name:        "chain-name",
				Usage:       "",
				Destination: &newSeedFlags.ChainName,
				Value:       "betanet",
			},
			&cli.StringFlag{
				Name:        "execution-address",
				Usage:       "",
				Destination: &newSeedFlags.ExecutionAddress,
				Value:       "",
			},
			&cli.StringFlag{
				Name:        "mnemonic",
				Usage:       "",
				Destination: &newSeedFlags.Mnemonic,
				Value:       "",
			},
		},
	},
}

func cliActionNewSeed(cliCtx *cli.Context) error {
	var keystorePassword string
	if cliCtx.IsSet(KeystorePassword.Name) {
		keystorePassword = cliCtx.String(KeystorePassword.Name)
	} else {
		fmt.Println("Create a password that secures your validator keystore(s). " +
			"You will need to re-enter this to decrypt them when you setup your Zond validators.")
		keystorePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}

		fmt.Println("Re-enter password ")
		reEnterKeystorePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}

		if string(keystorePassword) != string(reEnterKeystorePassword) {
			return fmt.Errorf("password mismatch")
		}
	}

	seed := bytesutil.ToBytes48(pbkdf2.Key([]byte(newSeedFlags.Mnemonic), []byte("mnemonic"), 2048, 48, sha512.New))
	stakingdeposit.GenerateKeys(newSeedFlags.ValidatorStartIndex,
		newSeedFlags.NumValidators, misc.EncodeHex(seed[:]), newSeedFlags.Folder,
		newSeedFlags.ChainName, string(keystorePassword), newSeedFlags.ExecutionAddress)

	return nil
}
