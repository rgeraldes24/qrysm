// Package accounts defines a new model for accounts management in Qrysm, using best
// practices for user security and UX. Validator keys are kept in an imported (local)
// wallet as ML-DSA-87 seeds encrypted in QRL keystore v1 files (argon2id, AES-256-GCM).
package accounts
