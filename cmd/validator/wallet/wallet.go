package wallet

import (
	"github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/cmd"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/config/features"
	"github.com/theQRL/qrysm/runtime/tos"
	"github.com/urfave/cli/v2"
)

var log = logrus.WithField("prefix", "wallet")

// Commands for wallets for Qrysm validators.
var Commands = &cli.Command{
	Name:     "wallet",
	Category: "wallet",
	Usage:    "defines commands for interacting with QRL validator wallets",
	Subcommands: []*cli.Command{
		{
			Name:  "create",
			Usage: "creates a new wallet with an on-disk (imported) keymanager",
			Flags: cmd.WrapFlags([]cli.Flag{
				flags.WalletDirFlag,
				flags.KeymanagerKindFlag,
				// flags.RemoteSignerCertPathFlag,
				// flags.RemoteSignerKeyPathFlag,
				// flags.RemoteSignerCACertPathFlag,
				flags.WalletPasswordFileFlag,
				features.Mainnet,
				cmd.AcceptTosFlag,
			}),
			Before: func(cliCtx *cli.Context) error {
				if err := cmd.LoadFlagsFromConfig(cliCtx, cliCtx.Command.Flags); err != nil {
					return err
				}
				if err := tos.VerifyTosAcceptedOrPrompt(cliCtx); err != nil {
					return err
				}
				return features.ConfigureValidator(cliCtx)
			},
			Action: func(cliCtx *cli.Context) error {
				if err := walletCreate(cliCtx); err != nil {
					log.WithError(err).Fatal("Could not create a wallet")
				}
				return nil
			},
		},
	},
}
