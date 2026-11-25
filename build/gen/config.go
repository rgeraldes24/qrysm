package main

// This file is the configuration of the code generation: the proto packages
// and how they are compiled, the SSZ targets, and the mainnet/minimal SSZ
// size substitutions applied to the .proto files.

// sszMainnet and sszMinimal are the values substituted, per configuration, for
// the placeholders (e.g. ssz-size:"block_roots.size") of the .proto files.
var (
	sszMainnet = map[string]string{
		"block_roots.size":                    "1024,32",  // SLOTS_PER_HISTORICAL_ROOT, [32]byte
		"state_roots.size":                    "1024,32",  // SLOTS_PER_HISTORICAL_ROOT, [32]byte
		"execution_data_votes.size":           "512",      // SLOTS_PER_EXECUTION_VOTING_PERIOD = EPOCHS_PER_EXECUTION_VOTING_PERIOD (4) * SLOTS_PER_EPOCH (128)
		"randao_mixes.size":                   "65536,32", // EPOCHS_PER_HISTORICAL_VECTOR, [32]byte
		"previous_epoch_attestations.max":     "512",      // MAX_ATTESTATIONS * SLOTS_PER_EPOCH
		"current_epoch_attestations.max":      "512",      // MAX_ATTESTATIONS * SLOTS_PER_EPOCH
		"slashings.size":                      "512",      // EPOCHS_PER_SLASHINGS_VECTOR
		"sync_committee_bits.size":            "128",      // SYNC_COMMITTEE_SIZE
		"sync_committee_bytes.size":           "16",
		"sync_committee_bits.type":            "github.com/theQRL/go-bitfield.Bitvector128",
		"sync_committee_aggregate_bytes.size": "16",
		"sync_committee_aggregate_bits.type":  "github.com/theQRL/go-bitfield.Bitvector128",
		"withdrawal.size":                     "16",
		"logs_bloom.size":                     "256",
		"extra_data.size":                     "32",
	}

	sszMinimal = map[string]string{
		"block_roots.size":                    "64,32",
		"state_roots.size":                    "64,32",
		"execution_data_votes.size":           "32",
		"randao_mixes.size":                   "64,32",
		"previous_epoch_attestations.max":     "1024",
		"current_epoch_attestations.max":      "1024",
		"slashings.size":                      "64",
		"sync_committee_bits.size":            "16",
		"sync_committee_bytes.size":           "2",
		"sync_committee_bits.type":            "github.com/theQRL/go-bitfield.Bitvector16",
		"sync_committee_aggregate_bytes.size": "2",
		"sync_committee_aggregate_bits.type":  "github.com/theQRL/go-bitfield.Bitvector16",
		"withdrawal.size":                     "4",
		"logs_bloom.size":                     "256",
		"extra_data.size":                     "32",
	}
)

// protoPkg describes how one proto package is compiled to Go.
type protoPkg struct {
	// mode is the protoc-gen-go flavour generating the *.pb.go files
	// (cast, cast_grpc or stock).
	mode string
	// gatewayOpts are the options of the grpc-gateway plugin generating the
	// *.pb.gw.go files, nil when the package has no gateway.
	gatewayOpts []string
}

// gateway reports whether the package has grpc-gateway (*.pb.gw.go) files.
func (p protoPkg) gateway() bool {
	return p.gatewayOpts != nil
}

var (
	defaultGatewayOptions = []string{"logtostderr=true", "allow_repeated_fields_in_body=true"}

	// protoPkgs are the proto package directories, mapped to how they are
	// compiled. Every .proto file of a package is compiled.
	protoPkgs = map[string]protoPkg{
		"proto/engine/v1":                       {mode: modeCastGRPC},
		"proto/qrl/ext":                         {mode: modeCast},
		"proto/qrl/service":                     {mode: modeCastGRPC, gatewayOpts: []string{"logtostderr=true", "allow_repeated_fields_in_body=true", "allow_delete_body=true"}},
		"proto/qrl/v1":                          {mode: modeCastGRPC, gatewayOpts: defaultGatewayOptions},
		"proto/qrysm/v1alpha1":                  {mode: modeCastGRPC, gatewayOpts: defaultGatewayOptions},
		"proto/qrysm/v1alpha1/validator-client": {mode: modeCastGRPC, gatewayOpts: defaultGatewayOptions},
		"proto/testing":                         {mode: modeCast},
	}
)

// sszTarget describes the SSZ code generated for one proto package.
type sszTarget struct {
	pkg         string   // proto package dir, e.g. "proto/qrysm/v1alpha1"
	out         string   // generated file, relative to pkg
	includes    []string // repo-relative package dirs sszgen resolves external types from
	objs        []string // types to generate the SSZ methods of (empty: every struct)
	excludeObjs []string // types to skip
}

// sszTargets are generated in this order.
var sszTargets = []sszTarget{
	{
		pkg:      "proto/engine/v1",
		out:      "generated.ssz.go",
		includes: []string{"consensus-types/primitives"},
		objs: []string{
			"ExecutionPayload",
			"ExecutionPayloadZond",
			"ExecutionPayloadHeader",
			"ExecutionPayloadHeaderZond",
			"Withdrawal",
		},
	},
	{
		pkg:      "proto/qrl/v1",
		out:      "generated.ssz.go",
		includes: []string{"consensus-types/primitives", "proto/engine/v1"},
		objs: []string{
			"AggregateAttestationAndProof",
			"Attestation",
			"AttestationData",
			"AttesterSlashing",
			"BeaconBlock",
			"BeaconBlockHeader",
			"Checkpoint",
			"Deposit",
			"DepositData",
			"ExecutionData",
			"IndexedAttestation",
			"ProposerSlashing",
			"SignedAggregateAttestationAndProof",
			"SignedBeaconBlock",
			"SignedBeaconBlockZond",
			"SignedBlindedBeaconBlockZond",
			"SignedBeaconBlockHeader",
			"SignedVoluntaryExit",
			"SyncAggregate",
			"SyncCommittee",
			"Validator",
			"VoluntaryExit",
		},
	},
	{
		pkg:      "proto/qrysm/v1alpha1",
		out:      "generated.ssz.go",
		includes: []string{"consensus-types/primitives", "proto/engine/v1"},
		// The duplicates (e.g. Status) are harmless, and kept so that the
		// generated code does not change.
		objs: []string{
			"BeaconBlockZond",
			"BeaconBlockBodyZond",
			"SignedBeaconBlockZond",
			"SignedBlindedBeaconBlockZond",
			"BlindedBeaconBlockZond",
			"BlindedBeaconBlockBodyZond",
			"SyncAggregate",
			"SyncCommitteeMessage",
			"SyncCommitteeContribution",
			"ContributionAndProof",
			"SignedContributionAndProof",
			"BeaconBlocksByRangeRequest",
			"QNRForkID",
			"MetaDataV1",
			"Status",
			"AggregateAttestationAndProof",
			"Attestation",
			"AttestationData",
			"AttesterSlashing",
			"BeaconBlockHeader",
			"Checkpoint",
			"Deposit",
			"ExecutionData",
			"IndexedAttestation",
			"ProposerSlashing",
			"SignedAggregateAttestationAndProof",
			"SignedBeaconBlockHeader",
			"SignedVoluntaryExit",
			"Validator",
			"VoluntaryExit",
			"ContributionAndProof",
			"SignedContributionAndProof",
			"DepositMessage",
			"Fork",
			"ForkData",
			"HistoricalBatch",
			"Status",
			"BeaconStateZond",
			"SigningData",
			"SyncCommittee",
			"SyncAggregatorSelectionData",
			"PowBlock",
			"SignedValidatorRegistrationV1",
			"ValidatorRegistrationV1",
			"Withdrawal",
			"BuilderBidZond",
		},
	},
}
