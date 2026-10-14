package accounts

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/validator/accounts/wallet"
	"github.com/theQRL/qrysm/validator/keymanager"
	"github.com/theQRL/qrysm/validator/keymanager/local"
)

// WalletCreate creates wallet specified by configuration options.
func (acm *AccountsCLIManager) WalletCreate(ctx context.Context) (*wallet.Wallet, error) {
	w := wallet.New(&wallet.Config{
		WalletDir:      acm.walletDir,
		KeymanagerKind: acm.keymanagerKind,
		WalletPassword: acm.walletPassword,
	})
	var err error
	switch w.KeymanagerKind() {
	case keymanager.Local:
		if err := w.SaveWallet(); err != nil {
			return nil, errors.Wrap(err, "could not initialize wallet: could not save wallet to disk")
		}
		accountsKeystore, err := local.CreateEmptyKeyStoreRepresentationForNewWallet(ctx, w.Password())
		if err != nil {
			return nil, err
		}
		encodedAccounts, err := json.MarshalIndent(accountsKeystore, "", "\t")
		if err != nil {
			return nil, err
		}
		if _, err = w.WriteFileAtPath(ctx, local.AccountsPath, local.AccountsKeystoreFileName, encodedAccounts); err != nil {
			return nil, err
		}
		log.WithField("--wallet-dir", acm.walletDir).Info(
			"Successfully created wallet with ability to import keystores",
		)
	/*
		case keymanager.Web3Signer:
			return nil, errors.New("web3signer keymanager does not require persistent wallets.")
	*/
	default:
		return nil, errors.Wrapf(err, errKeymanagerNotSupported, w.KeymanagerKind())
	}
	return w, nil
}
