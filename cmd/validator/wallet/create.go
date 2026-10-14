package wallet

import (
	"fmt"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/cmd/validator/flags"
	"github.com/theQRL/qrysm/io/prompt"
	"github.com/theQRL/qrysm/validator/accounts"
	"github.com/theQRL/qrysm/validator/accounts/userprompt"
	"github.com/theQRL/qrysm/validator/accounts/wallet"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/urfave/cli/v2"
)

func walletCreate(c *cli.Context) error {
	keymanagerKind, err := inputKeymanagerKind(c)
	if err != nil {
		return err
	}

	opts, err := ConstructCLIManagerOpts(c, keymanagerKind)
	if err != nil {
		return err
	}

	acc, err := accounts.NewCLIManager(opts...)
	if err != nil {
		return err
	}
	if _, err := acc.WalletCreate(c.Context); err != nil {
		return errors.Wrap(err, "could not create wallet")
	}
	return nil
}

// ConstructCLIManagerOpts prompts the user for wallet creation input.
func ConstructCLIManagerOpts(cliCtx *cli.Context, keymanagerKind keymanager.Kind) ([]accounts.Option, error) {
	cliOpts := []accounts.Option{}
	// Get wallet dir and check that no wallet exists at the location.
	walletDir, err := userprompt.InputDirectory(cliCtx, userprompt.WalletDirPromptText, flags.WalletDirFlag)
	if err != nil {
		return []accounts.Option{}, err
	}
	dirExists, err := wallet.Exists(walletDir)
	if err != nil {
		return []accounts.Option{}, err
	}
	if dirExists {
		return []accounts.Option{}, errors.New("a wallet already exists at this location. Please input an" +
			" alternative location for the new wallet or remove the current wallet")
	}

	walletPassword, err := prompt.InputPassword(
		cliCtx,
		flags.WalletPasswordFileFlag,
		wallet.NewWalletPasswordPromptText,
		wallet.ConfirmPasswordPromptText,
		true, /* Should confirm password */
		prompt.ValidatePasswordInput,
	)
	if err != nil {
		return []accounts.Option{}, err
	}
	cliOpts = append(cliOpts, accounts.WithWalletDir(walletDir))
	cliOpts = append(cliOpts, accounts.WithWalletPassword(walletPassword))
	cliOpts = append(cliOpts, accounts.WithKeymanagerType(keymanagerKind))
	return cliOpts, nil
}

func inputKeymanagerKind(cliCtx *cli.Context) (keymanager.Kind, error) {
	if cliCtx.IsSet(flags.KeymanagerKindFlag.Name) {
		return keymanager.ParseKind(cliCtx.String(flags.KeymanagerKindFlag.Name))
	}
	promptSelect := promptui.Select{
		Label: "Select a type of wallet",
		Items: []string{
			wallet.KeymanagerKindSelections[keymanager.Local],
		},
	}
	selection, _, err := promptSelect.Run()
	if err != nil {
		return keymanager.Local, fmt.Errorf("could not select wallet type: %w", userprompt.FormatPromptError(err))
	}
	return keymanager.Kind(selection), nil
}

// CreateAndSaveWalletCli from user input with a desired keymanager. If a
// wallet already exists in the path, it suggests the user alternatives
// such as how to edit their existing wallet configuration.
func CreateAndSaveWalletCli(cliCtx *cli.Context) (*wallet.Wallet, error) {
	keymanagerKind, err := inputKeymanagerKind(cliCtx)
	if err != nil {
		return nil, err
	}
	opts, err := ConstructCLIManagerOpts(cliCtx, keymanagerKind)
	if err != nil {
		return nil, err
	}
	acc, err := accounts.NewCLIManager(opts...)
	if err != nil {
		return nil, err
	}
	w, err := acc.WalletCreate(cliCtx.Context)
	if err != nil {
		return nil, errors.Wrap(err, "could not create wallet")
	}
	return w, nil
}
