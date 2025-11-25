//go:build !minimal

package field_params

import (
	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
	walletcommon "github.com/theQRL/go-qrllib/wallet/common"
)

const (
	Preset                                = "mainnet"
	BlockRootsLength                      = 1024                               // SLOTS_PER_HISTORICAL_ROOT
	StateRootsLength                      = 1024                               // SLOTS_PER_HISTORICAL_ROOT
	RandaoMixesLength                     = 65536                              // EPOCHS_PER_HISTORICAL_VECTOR
	HistoricalRootsLength                 = 16777216                           // HISTORICAL_ROOTS_LIMIT
	ValidatorRegistryLimit                = 1099511627776                      // VALIDATOR_REGISTRY_LIMIT
	ExecutionDataVotesLength              = 2                                  // SLOTS_PER_EXECUTION_VOTING_PERIOD
	PreviousEpochAttestationsLength       = 16384                              // MAX_ATTESTATIONS * SLOTS_PER_EPOCH
	CurrentEpochAttestationsLength        = 16384                              // MAX_ATTESTATIONS * SLOTS_PER_EPOCH
	SlashingsLength                       = 1024                               // EPOCHS_PER_SLASHINGS_VECTOR
	SyncCommitteeLength                   = 16                                 // SYNC_COMMITTEE_SIZE  // TODO (cyyber) : Original value 512, new value needs to be decided
	RootLength                            = 32                                 // RootLength defines the byte length of a Merkle root.
	MLDSA87SeedLength                     = walletcommon.SeedSize              // MLDSA87SeedLength defines the byte length of a ML-DSA-87 seed.
	MLDSA87SignatureLength                = cryptomldsa87.CryptoBytes          // MLDSA87SignatureLength defines the byte length of a ML-DSA-87 signature.
	MLDSA87PubkeyLength                   = cryptomldsa87.CryptoPublicKeyBytes // MLDSA87PubkeyLength defines the byte length of a ML-DSA-87 public key.
	MaxTxsPerPayloadLength                = 1048576                            // MaxTxsPerPayloadLength defines the maximum number of transactions that can be included in a payload.
	MaxBytesPerTxLength                   = 1073741824                         // MaxBytesPerTxLength defines the maximum number of bytes that can be included in a transaction.
	FeeRecipientLength                    = 20                                 // FeeRecipientLength defines the byte length of a fee recipient.
	LogsBloomLength                       = 256                                // LogsBloomLength defines the byte length of a logs bloom.
	VersionLength                         = 4                                  // VersionLength defines the byte length of a fork version number.
	SlotsPerEpoch                         = 128                                // SlotsPerEpoch defines the number of slots per epoch.
	SyncCommitteeAggregationBytesLength   = 2                                  // SyncCommitteeAggregationBytesLength defines the length of sync committee aggregate bytes. // TODO (cyyber) : Original value 16, new value needs to be decided
	SyncAggregateSyncCommitteeBytesLength = 2                                  // SyncAggregateSyncCommitteeBytesLength defines the length of sync committee bytes in a sync aggregate. // TODO (cyyber) : Original value 64, new value needs to be decided
	MaxWithdrawalsPerPayload              = 16                                 // MaxWithdrawalsPerPayloadLength defines the maximum number of withdrawals that can be included in a payload.
)
