package v1_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
	"github.com/theQRL/qrysm/time/slots"
	v1 "github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1/mock"
)

const contractDir = "../testdata/contract"

// fixture is a contract example decoded into generic JSON values.
type fixture map[string]any

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(contractDir, name))
	require.NoError(t, err)
	var f fixture
	require.NoError(t, json.Unmarshal(raw, &f))
	return f
}

// get walks a dotted path through the fixture.
func (f fixture) get(t *testing.T, path string) any {
	t.Helper()
	var cur any = map[string]any(f)
	for _, key := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		require.Equal(t, true, ok, "path %s", path)
		cur, ok = obj[key]
		require.Equal(t, true, ok, "path %s", path)
	}
	return cur
}

func (f fixture) str(t *testing.T, path string) string {
	t.Helper()
	s, ok := f.get(t, path).(string)
	require.Equal(t, true, ok, "path %s is not a string", path)
	return s
}

func (f fixture) uint(t *testing.T, path string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(f.str(t, path), 10, 64)
	require.NoError(t, err)
	return v
}

func (f fixture) bytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := hexutil.Decode(f.str(t, path))
	require.NoError(t, err)
	return b
}

func (f fixture) sigs(t *testing.T, path string) [][]byte {
	t.Helper()
	list, ok := f.get(t, path).([]any)
	require.Equal(t, true, ok, "path %s is not a list", path)
	out := make([][]byte, len(list))
	for i, s := range list {
		b, err := hexutil.Decode(s.(string))
		require.NoError(t, err)
		out[i] = b
	}
	return out
}

func (f fixture) attestationData(t *testing.T, path string) *qrysmpb.AttestationData {
	return &qrysmpb.AttestationData{
		Slot:            primitives.Slot(f.uint(t, path+".slot")),
		CommitteeIndex:  primitives.CommitteeIndex(f.uint(t, path+".index")),
		BeaconBlockRoot: f.bytes(t, path+".beacon_block_root"),
		Source: &qrysmpb.Checkpoint{
			Epoch: primitives.Epoch(f.uint(t, path+".source.epoch")),
			Root:  f.bytes(t, path+".source.root"),
		},
		Target: &qrysmpb.Checkpoint{
			Epoch: primitives.Epoch(f.uint(t, path+".target.epoch")),
			Root:  f.bytes(t, path+".target.root"),
		},
	}
}

// canonical lowercases hex strings and Q addresses so the comparison is
// semantic: the contract accepts either case on input.
func canonical(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = canonical(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = canonical(val)
		}
		return out
	case string:
		if strings.HasPrefix(x, "0x") || (strings.HasPrefix(x, "Q") && len(x) == 129) {
			return x[:1] + strings.ToLower(x[1:])
		}
		return x
	default:
		return v
	}
}

func requireContractJSON(t *testing.T, want fixture, got any) {
	t.Helper()
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	var gotJSON any
	require.NoError(t, json.Unmarshal(encoded, &gotJSON))
	if !reflect.DeepEqual(canonical(map[string]any(want)), canonical(gotJSON)) {
		wantJSON, err := json.MarshalIndent(want, "", "  ")
		require.NoError(t, err)
		t.Fatalf("mapper output differs from the contract example\nwant: %s\ngot:  %s", truncate(wantJSON), truncate(encoded))
	}
}

func truncate(b []byte) string {
	const limit = 4000
	if len(b) > limit {
		return string(b[:limit]) + "…"
	}
	return string(b)
}

func signRequest(f fixture, t *testing.T, slot primitives.Slot) *validatorpb.SignRequest {
	return &validatorpb.SignRequest{
		PublicKey:   make([]byte, 2592),
		SigningRoot: f.bytes(t, "signing_root"),
		SigningSlot: slot,
	}
}

func setupE2EConfig(t *testing.T) []byte {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.E2ETestConfig())
	gvr, err := hexutil.Decode("0x7a1e2c4d5b6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f809")
	require.NoError(t, err)
	return gvr
}

// TestContractExamples maps one qrysm SignRequest per contract example and
// requires the produced JSON to equal the example, and to pass the strict
// contract decoder.
func TestContractExamples(t *testing.T) {
	gvr := setupE2EConfig(t)

	tests := []struct {
		file  string
		build func(t *testing.T, f fixture) any
	}{
		{
			file: "block_v2.json",
			build: func(t *testing.T, f fixture) any {
				block := util.HydrateBeaconBlockZond(&qrysmpb.BeaconBlockZond{
					Slot:          primitives.Slot(f.uint(t, "beacon_block.block_header.slot")),
					ProposerIndex: primitives.ValidatorIndex(f.uint(t, "beacon_block.block_header.proposer_index")),
					ParentRoot:    f.bytes(t, "beacon_block.block_header.parent_root"),
					StateRoot:     f.bytes(t, "beacon_block.block_header.state_root"),
				})
				// The example's body_root is a placeholder; a real block's body
				// root is its body's hash tree root.
				bodyRoot, err := block.Body.HashTreeRoot()
				require.NoError(t, err)
				f.get(t, "beacon_block.block_header").(map[string]any)["body_root"] = hexutil.Encode(bodyRoot[:])
				req := signRequest(f, t, block.Slot)
				req.Object = &validatorpb.SignRequest_BlockZond{BlockZond: block}
				got, err := v1.GetBlockV2SignRequest(req, gvr)
				require.NoError(t, err)
				return got
			},
		},
		{
			file: "block_v2.json",
			build: func(t *testing.T, f fixture) any {
				block := util.HydrateBlindedBeaconBlockZond(&qrysmpb.BlindedBeaconBlockZond{
					Slot:          primitives.Slot(f.uint(t, "beacon_block.block_header.slot")),
					ProposerIndex: primitives.ValidatorIndex(f.uint(t, "beacon_block.block_header.proposer_index")),
					ParentRoot:    f.bytes(t, "beacon_block.block_header.parent_root"),
					StateRoot:     f.bytes(t, "beacon_block.block_header.state_root"),
				})
				bodyRoot, err := block.Body.HashTreeRoot()
				require.NoError(t, err)
				f.get(t, "beacon_block.block_header").(map[string]any)["body_root"] = hexutil.Encode(bodyRoot[:])
				req := signRequest(f, t, block.Slot)
				req.Object = &validatorpb.SignRequest_BlindedBlockZond{BlindedBlockZond: block}
				got, err := v1.GetBlockV2SignRequest(req, gvr)
				require.NoError(t, err)
				return got
			},
		},
		{
			file: "attestation.json",
			build: func(t *testing.T, f fixture) any {
				data := f.attestationData(t, "attestation")
				req := signRequest(f, t, data.Slot)
				req.Object = &validatorpb.SignRequest_AttestationData{AttestationData: data}
				got, err := v1.GetAttestationSignRequest(req, gvr)
				require.NoError(t, err)
				return got
			},
		},
		{
			file: "aggregation_slot.json",
			build: func(t *testing.T, f fixture) any {
				slot := primitives.Slot(f.uint(t, "aggregation_slot.slot"))
				req := signRequest(f, t, slot)
				req.Object = &validatorpb.SignRequest_Slot{Slot: slot}
				got, err := v1.GetAggregationSlotSignRequest(req, gvr)
				require.NoError(t, err)
				return got
			},
		},
		{
			file: "aggregate_and_proof.json",
			build: func(t *testing.T, f fixture) any {
				data := f.attestationData(t, "aggregate_and_proof.aggregate.data")
				req := signRequest(f, t, data.Slot)
				req.Object = &validatorpb.SignRequest_AggregateAttestationAndProof{
					AggregateAttestationAndProof: &qrysmpb.AggregateAttestationAndProof{
						AggregatorIndex: primitives.ValidatorIndex(f.uint(t, "aggregate_and_proof.aggregator_index")),
						Aggregate: &qrysmpb.Attestation{
							AggregationBits: bitfield.Bitlist(f.bytes(t, "aggregate_and_proof.aggregate.aggregation_bits")),
							Data:            data,
							Signatures:      f.sigs(t, "aggregate_and_proof.aggregate.signatures"),
						},
						SelectionProof: f.bytes(t, "aggregate_and_proof.selection_proof"),
					},
				}
				got, err := v1.GetAggregateAndProofSignRequest(req, gvr)
				require.NoError(t, err)
				return got
			},
		},
		{
			file: "voluntary_exit.json",
			build: func(t *testing.T, f fixture) any {
				exit := &qrysmpb.VoluntaryExit{
					Epoch:          primitives.Epoch(f.uint(t, "voluntary_exit.epoch")),
					ValidatorIndex: primitives.ValidatorIndex(f.uint(t, "voluntary_exit.validator_index")),
				}
				startSlot, err := slots.EpochStart(exit.Epoch)
				require.NoError(t, err)
				req := signRequest(f, t, startSlot)
				req.Object = &validatorpb.SignRequest_Exit{Exit: exit}
				got, err := v1.GetVoluntaryExitSignRequest(req, gvr)
				require.NoError(t, err)
				return got
			},
		},
		{
			file: "sync_committee_message.json",
			build: func(t *testing.T, f fixture) any {
				req := signRequest(f, t, primitives.Slot(f.uint(t, "sync_committee_message.slot")))
				req.Object = &validatorpb.SignRequest_SyncMessageBlockRoot{
					SyncMessageBlockRoot: f.bytes(t, "sync_committee_message.beacon_block_root"),
				}
				got, err := v1.GetSyncCommitteeMessageSignRequest(req, gvr)
				require.NoError(t, err)
				return got
			},
		},
		{
			file: "sync_committee_selection_proof.json",
			build: func(t *testing.T, f fixture) any {
				slot := primitives.Slot(f.uint(t, "sync_aggregator_selection_data.slot"))
				req := signRequest(f, t, slot)
				req.Object = &validatorpb.SignRequest_SyncAggregatorSelectionData{
					SyncAggregatorSelectionData: &qrysmpb.SyncAggregatorSelectionData{
						Slot:              slot,
						SubcommitteeIndex: f.uint(t, "sync_aggregator_selection_data.subcommittee_index"),
					},
				}
				got, err := v1.GetSyncCommitteeSelectionProofSignRequest(req, gvr)
				require.NoError(t, err)
				return got
			},
		},
		{file: "sync_committee_contribution_and_proof_minimal.json", build: buildContribution(gvr)},
		{file: "sync_committee_contribution_and_proof_mainnet.json", build: buildContribution(gvr)},
		{
			file: "validator_registration.json",
			build: func(t *testing.T, f fixture) any {
				feeRecipient, err := hexutil.Decode("0x" + f.str(t, "validator_registration.fee_recipient")[1:])
				require.NoError(t, err)
				req := signRequest(f, t, 0)
				req.Object = &validatorpb.SignRequest_Registration{
					Registration: &qrysmpb.ValidatorRegistrationV1{
						FeeRecipient: feeRecipient,
						GasLimit:     f.uint(t, "validator_registration.gas_limit"),
						Timestamp:    f.uint(t, "validator_registration.timestamp"),
						Pubkey:       f.bytes(t, "validator_registration.pubkey"),
					},
				}
				got, err := v1.GetValidatorRegistrationSignRequest(req)
				require.NoError(t, err)
				return got
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			f := loadFixture(t, tt.file)
			got := tt.build(t, f)
			requireContractJSON(t, f, got)

			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			typ, err := mock.ValidateSignRequestJSON(encoded)
			require.NoError(t, err)
			require.Equal(t, f.str(t, "type"), typ)
		})
	}
}

func buildContribution(gvr []byte) func(t *testing.T, f fixture) any {
	return func(t *testing.T, f fixture) any {
		const c = "contribution_and_proof.contribution"
		slot := primitives.Slot(f.uint(t, c+".slot"))
		req := signRequest(f, t, slot)
		req.Object = &validatorpb.SignRequest_ContributionAndProof{
			ContributionAndProof: &qrysmpb.ContributionAndProof{
				AggregatorIndex: primitives.ValidatorIndex(f.uint(t, "contribution_and_proof.aggregator_index")),
				Contribution: &qrysmpb.SyncCommitteeContribution{
					Slot:              slot,
					BlockRoot:         f.bytes(t, c+".beacon_block_root"),
					SubcommitteeIndex: f.uint(t, c+".subcommittee_index"),
					AggregationBits:   f.bytes(t, c+".aggregation_bits"),
					Signatures:        f.sigs(t, c+".signatures"),
				},
				SelectionProof: f.bytes(t, "contribution_and_proof.selection_proof"),
			},
		}
		got, err := v1.GetSyncCommitteeContributionAndProofSignRequest(req, gvr)
		require.NoError(t, err)
		return got
	}
}

func TestContractExamples_RandaoRevealRequest(t *testing.T) {
	gvr := setupE2EConfig(t)
	f := loadFixture(t, "randao_reveal_request.json")
	got, err := v1.GetRandaoRevealRequest(
		primitives.Slot(f.uint(t, "slot")), f.bytes(t, "randao_commitment"), gvr,
	)
	require.NoError(t, err)
	requireContractJSON(t, f, got)

	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.NoError(t, mock.ValidateRandaoRevealRequestJSON(encoded))
}

// TestContractExamples_StrictDecoder checks that the strict decoder used as
// the fake signer accepts every example and refuses the contract's 400 cases.
func TestContractExamples_StrictDecoder(t *testing.T) {
	files := []string{
		"block_v2.json", "attestation.json", "aggregation_slot.json", "aggregate_and_proof.json",
		"voluntary_exit.json", "sync_committee_message.json", "sync_committee_selection_proof.json",
		"sync_committee_contribution_and_proof_minimal.json", "sync_committee_contribution_and_proof_mainnet.json",
		"validator_registration.json",
	}
	for _, name := range files {
		raw, err := os.ReadFile(filepath.Join(contractDir, name))
		require.NoError(t, err)
		_, err = mock.ValidateSignRequestJSON(raw)
		require.NoError(t, err, name)
	}

	attestation := func(edit func(f fixture)) []byte {
		f := loadFixture(t, "attestation.json")
		edit(f)
		b, err := json.Marshal(f)
		require.NoError(t, err)
		return b
	}
	aggregate := func(edit func(f fixture)) []byte {
		f := loadFixture(t, "aggregate_and_proof.json")
		edit(f)
		b, err := json.Marshal(f)
		require.NoError(t, err)
		return b
	}
	registration := func(edit func(f fixture)) []byte {
		f := loadFixture(t, "validator_registration.json")
		edit(f)
		b, err := json.Marshal(f)
		require.NoError(t, err)
		return b
	}
	bad := []struct {
		name string
		body []byte
	}{
		{"unknown field", attestation(func(f fixture) { f["extra"] = "x" })},
		{"number for uint64", attestation(func(f fixture) { f.get(t, "attestation").(map[string]any)["slot"] = 61 })},
		{"leading zero", attestation(func(f fixture) { f.get(t, "attestation").(map[string]any)["slot"] = "061" })},
		{"lower-case type", attestation(func(f fixture) { f["type"] = "attestation" })},
		{"missing fork_info", attestation(func(f fixture) { delete(f, "fork_info") })},
		{"short root", attestation(func(f fixture) { f["signing_root"] = "0x00" })},
		{"both signing root spellings", attestation(func(f fixture) { f["signingRoot"] = f["signing_root"] })},
		{"singular signature", aggregate(func(f fixture) {
			agg := f.get(t, "aggregate_and_proof.aggregate").(map[string]any)
			agg["signature"] = agg["signatures"].([]any)[0]
			delete(agg, "signatures")
		})},
		{"popcount mismatch", aggregate(func(f fixture) {
			f.get(t, "aggregate_and_proof.aggregate").(map[string]any)["aggregation_bits"] = "0x0f"
		})},
		{"0x fee recipient", registration(func(f fixture) {
			reg := f.get(t, "validator_registration").(map[string]any)
			reg["fee_recipient"] = "0x" + reg["fee_recipient"].(string)[1:]
		})},
		{"fork_info on registration", registration(func(f fixture) { f["fork_info"] = map[string]any{} })},
		{"duplicate key", []byte(`{"type":"AGGREGATION_SLOT","type":"AGGREGATION_SLOT"}`)},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mock.ValidateSignRequestJSON(tt.body)
			require.NotNil(t, err)
		})
	}
}
