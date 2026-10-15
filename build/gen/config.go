package main

// This file is the generation config of build/gen: what used to be read from
// proto/ssz_proto_library.bzl (the SSZ size substitutions) and from the
// go_proto_library, go_proto_compiler and ssz_gen_marshal rules of
// proto/**/BUILD.bazel.

// sszSizes are the values substituted for the placeholders of the templated
// protos (e.g. `ssz_size: "block_roots.size"`), per configuration.
var sszSizes = map[string]map[string]string{
	"mainnet": {
		"block_roots.size":                    "1024,32", // SLOTS_PER_HISTORICAL_ROOT, [32]byte
		"current_epoch_attestations.max":      "512",     // MAX_ATTESTATIONS * SLOTS_PER_EPOCH
		"execution_data_votes.size":           "512",     // SLOTS_PER_EXECUTION_VOTING_PERIOD = EPOCHS_PER_EXECUTION_VOTING_PERIOD (4) * SLOTS_PER_EPOCH (128)
		"extra_data.size":                     "32",
		"logs_bloom.size":                     "256",
		"previous_epoch_attestations.max":     "512",      // MAX_ATTESTATIONS * SLOTS_PER_EPOCH
		"randao_mixes.size":                   "65536,32", // EPOCHS_PER_HISTORICAL_VECTOR, [32]byte
		"slashings.size":                      "512",      // EPOCHS_PER_SLASHINGS_VECTOR
		"state_roots.size":                    "1024,32",  // SLOTS_PER_HISTORICAL_ROOT, [32]byte
		"sync_committee_aggregate_bits.type":  "github.com/theQRL/go-bitfield.Bitvector128",
		"sync_committee_aggregate_bytes.size": "16",
		"sync_committee_bits.size":            "128", // SYNC_COMMITTEE_SIZE
		"sync_committee_bits.type":            "github.com/theQRL/go-bitfield.Bitvector128",
		"sync_committee_bytes.size":           "16",
		"withdrawal.size":                     "16",
	},
	"minimal": {
		"block_roots.size":                    "64,32",
		"current_epoch_attestations.max":      "1024",
		"execution_data_votes.size":           "32",
		"extra_data.size":                     "32",
		"logs_bloom.size":                     "256",
		"previous_epoch_attestations.max":     "1024",
		"randao_mixes.size":                   "64,32",
		"slashings.size":                      "64",
		"state_roots.size":                    "64,32",
		"sync_committee_aggregate_bits.type":  "github.com/theQRL/go-bitfield.Bitvector16",
		"sync_committee_aggregate_bytes.size": "2",
		"sync_committee_bits.size":            "16",
		"sync_committee_bits.type":            "github.com/theQRL/go-bitfield.Bitvector16",
		"sync_committee_bytes.size":           "2",
		"withdrawal.size":                     "4",
	},
}

// defaultGatewayOptions are the options of go_gen_grpc_gateway, the
// go_proto_compiler in protoc-gen-grpc-gateway/BUILD.bazel of the grpc-gateway
// fork.
var defaultGatewayOptions = []string{"logtostderr=true", "allow_repeated_fields_in_body=true"}

// protoPkgs are the proto package directories, sorted, with the plugin their
// .pb.go files come from and the options of the grpc-gateway plugin when they
// have .pb.gw.go files.
var protoPkgs = []protoPkg{
	{dir: "proto/engine/v1", mode: modeCastGRPC},
	{dir: "proto/qrl/ext", mode: modeStock},
	{dir: "proto/qrl/service", mode: modeCastGRPC, gateway: append(defaultGatewayOptions[:len(defaultGatewayOptions):len(defaultGatewayOptions)], "allow_delete_body=true")},
	{dir: "proto/qrl/v1", mode: modeCastGRPC, gateway: defaultGatewayOptions},
	{dir: "proto/qrysm/v1alpha1", mode: modeCastGRPC, gateway: defaultGatewayOptions},
	{dir: "proto/qrysm/v1alpha1/validator-client", mode: modeCastGRPC, gateway: defaultGatewayOptions},
	{dir: "proto/testing", mode: modeCast},
}

// sszTargets are the packages whose generated.ssz.go sszgen writes: the
// objects to generate code for, the Go packages it reads types from (libInc)
// and the proto packages whose .pb.go files it reads too (protoInc). The
// order of objs is kept as it was in the BUILD files, duplicates included,
// since sszgen's output follows it.
var sszTargets = []sszTarget{
	{
		pkg:    "proto/engine/v1",
		libInc: []string{"consensus-types/primitives"},
		objs: []string{
			"ExecutionPayload", "ExecutionPayloadZond", "ExecutionPayloadHeader", "ExecutionPayloadHeaderZond",
			"Withdrawal",
		},
	},
	{
		pkg:      "proto/qrl/v1",
		libInc:   []string{"consensus-types/primitives"},
		protoInc: []string{"proto/engine/v1"},
		objs: []string{
			"AggregateAttestationAndProof", "Attestation", "AttestationData", "AttesterSlashing", "BeaconBlock",
			"BeaconBlockHeader", "Checkpoint", "Deposit", "DepositData", "ExecutionData", "IndexedAttestation",
			"ProposerSlashing", "SignedAggregateAttestationAndProof", "SignedBeaconBlock", "SignedBeaconBlockZond",
			"SignedBlindedBeaconBlockZond", "SignedBeaconBlockHeader", "SignedVoluntaryExit", "SyncAggregate",
			"SyncCommittee", "Validator", "VoluntaryExit",
		},
	},
	{
		pkg:      "proto/qrysm/v1alpha1",
		libInc:   []string{"consensus-types/primitives"},
		protoInc: []string{"proto/engine/v1"},
		objs: []string{
			"BeaconBlockZond", "BeaconBlockBodyZond", "SignedBeaconBlockZond", "SignedBlindedBeaconBlockZond",
			"BlindedBeaconBlockZond", "BlindedBeaconBlockBodyZond", "SyncAggregate", "SyncCommitteeMessage",
			"SyncCommitteeContribution", "ContributionAndProof", "SignedContributionAndProof",
			"BeaconBlocksByRangeRequest", "QNRForkID", "MetaDataV1", "Status", "AggregateAttestationAndProof",
			"Attestation", "AttestationData", "AttesterSlashing", "BeaconBlockHeader", "Checkpoint", "Deposit",
			"ExecutionData", "IndexedAttestation", "ProposerSlashing", "SignedAggregateAttestationAndProof",
			"SignedBeaconBlockHeader", "SignedVoluntaryExit", "Validator", "VoluntaryExit", "ContributionAndProof",
			"SignedContributionAndProof", "DepositMessage", "Fork", "ForkData", "HistoricalBatch", "Status",
			"BeaconStateZond", "SigningData", "SyncCommittee", "SyncAggregatorSelectionData", "PowBlock",
			"SignedValidatorRegistrationV1", "ValidatorRegistrationV1", "Withdrawal", "BuilderBidZond",
		},
	},
}
