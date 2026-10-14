/*
Package local defines an implementation of an on-disk keystore.json approach towards
defining validator accounts in Qrysm. A validating ML-DSA-87 seed is encrypted using a
passphrase and its resulting encrypted file is stored as a keystore.json file under a
unique, human-readable, account namespace. This local keymanager approach relies on
storing account information on-disk, making it trivial to import, backup and list all
associated accounts for a user.

Keystores use the QRL keystore v1 format (pkg/go-qrl-wallet-encryptor-keystore):
the key is derived from the password with argon2id and the seed is encrypted with
AES-256-GCM. Passwords utilized for key encryptions are strings of arbitrary unicode
characters. The password is first converted to its NFKD representation, stripped of
control codes as described in EIP-2335, and finally the password is UTF-8 encoded.
*/
package local
