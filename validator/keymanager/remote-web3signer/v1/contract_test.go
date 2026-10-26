package v1_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/beacon-chain/core/signing"
	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/network/forks"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	validatorpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1/validator-client"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/theQRL/qrysm/testing/util"
	"github.com/theQRL/qrysm/time/slots"
	v1 "github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1"
	"github.com/theQRL/qrysm/validator/keymanager/remote-web3signer/v1/mock"
)

// vectorsDir holds the remote signer's golden vectors
// (testdata/vectors/README.md).
const vectorsDir = "../testdata/vectors"

// presetConfigs maps a vector preset to the beacon config of its network.
var presetConfigs = map[string]func() *params.BeaconChainConfig{
	"minimal": params.E2ETestConfig,        // end-to-end
	"mainnet": params.E2EMainnetTestConfig, // end-to-end-mainnet
}

// signingFile is a file of testdata/vectors/signing/<preset>/.
type signingFile struct {
	Type   string          `json:"type"`
	Preset string          `json:"preset"`
	Cases  []signingRecord `json:"cases"`
}

// signingRecord is a golden sign request. The fields the client does not use
// (key_index, deterministic_signature, notes, expect) are left out.
type signingRecord struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Preset  string `json:"preset"`
	Network struct {
		ConfigName         string `json:"config_name"`
		GenesisForkVersion string `json:"genesis_fork_version"`
		GVR                string `json:"gvr"`
		SlotsPerEpoch      string `json:"slots_per_epoch"`
	} `json:"network"`
	Pubkey        string          `json:"pubkey"`
	ObjectSSZHex  string          `json:"object_ssz_hex"`
	ObjectRoot    string          `json:"object_root"`
	Domain        string          `json:"domain"`
	SigningRoot   string          `json:"signing_root"`
	RequestJSON   json.RawMessage `json:"request_json"`
	Authoritative *bool           `json:"authoritative"`
	Block         *struct {
		FullSSZHex    string `json:"full_ssz_hex"`
		BlindedSSZHex string `json:"blinded_ssz_hex"`
	} `json:"block"`
}

func loadSigningFiles(t *testing.T, preset string) []signingFile {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(vectorsDir, "signing", preset, "*.json"))
	require.NoError(t, err)
	require.Equal(t, 9, len(paths), "signing files of the %s preset", preset)
	files := make([]signingFile, len(paths))
	for i, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &files[i]), path)
		require.Equal(t, preset, files[i].Preset, path)
	}
	return files
}

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hexutil.Decode(s)
	require.NoError(t, err)
	return b
}

func decodeJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var v map[string]any
	require.NoError(t, json.Unmarshal(raw, &v))
	return v
}

// get walks a dotted path through a decoded JSON object.
func get(t *testing.T, v map[string]any, path string) any {
	t.Helper()
	var cur any = v
	for _, key := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		require.Equal(t, true, ok, "path %s", path)
		cur, ok = obj[key]
		require.Equal(t, true, ok, "path %s", path)
	}
	return cur
}

// mapped is one mapper output and the request body it must equal.
type mapped struct {
	name string
	got  []byte
	want map[string]any
}

func encode(t *testing.T, v any, err error) []byte {
	t.Helper()
	require.NoError(t, err)
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// TestSigningVectors runs every golden sign request through the client. It
// decodes object_ssz_hex with qrysm's UnmarshalSSZ, requires qrysm's object
// root, domain and signing root to equal the record's, builds the SignRequest
// the validator client builds, runs the mapper, and requires the JSON to equal
// request_json and to pass the strict contract decoder.
func TestSigningVectors(t *testing.T) {
	if fieldparams.Preset != "mainnet" {
		t.Skip("object_ssz_hex holds mainnet-preset SSZ encodings")
	}
	for _, preset := range []string{"minimal", "mainnet"} {
		t.Run(preset, func(t *testing.T) {
			params.SetupTestConfigCleanup(t)
			cfg := presetConfigs[preset]()
			params.OverrideBeaconConfig(cfg)
			for _, file := range loadSigningFiles(t, preset) {
				require.Equal(t, true, len(file.Cases) > 0, "%s has no cases", file.Type)
				for _, rec := range file.Cases {
					t.Run(file.Type+"/"+rec.Name, func(t *testing.T) {
						require.Equal(t, file.Type, rec.Type)
						require.Equal(t, preset, rec.Preset)
						runSigningVector(t, cfg, rec)
					})
				}
			}
		})
	}
}

func runSigningVector(t *testing.T, cfg *params.BeaconChainConfig, rec signingRecord) {
	// The record's network is the preset's e2e network.
	require.Equal(t, cfg.ConfigName, rec.Network.ConfigName)
	require.Equal(t, hexutil.Encode(cfg.GenesisForkVersion), rec.Network.GenesisForkVersion)
	require.Equal(t, strconv.FormatUint(uint64(cfg.SlotsPerEpoch), 10), rec.Network.SlotsPerEpoch)

	gvr := decodeHex(t, rec.Network.GVR)
	ssz := decodeHex(t, rec.ObjectSSZHex)
	want := decodeJSON(t, rec.RequestJSON)
	req := &validatorpb.SignRequest{
		PublicKey:   decodeHex(t, rec.Pubkey),
		SigningRoot: decodeHex(t, rec.SigningRoot),
	}

	var (
		objectRoot [32]byte
		domainType [4]byte
		outputs    []mapped
		err        error
	)
	switch rec.Type {
	case v1.TypeBlockV2:
		header := &qrysmpb.BeaconBlockHeader{}
		require.NoError(t, header.UnmarshalSSZ(ssz))
		objectRoot, err = header.HashTreeRoot()
		require.NoError(t, err)
		domainType = cfg.DomainBeaconProposer
		req.SigningSlot = header.Slot
		outputs = blockV2Outputs(t, rec, header, req, gvr, want)

	case v1.TypeAttestation:
		data := &qrysmpb.AttestationData{}
		require.NoError(t, data.UnmarshalSSZ(ssz))
		objectRoot, err = data.HashTreeRoot()
		require.NoError(t, err)
		domainType = cfg.DomainBeaconAttester
		req.SigningSlot = data.Slot
		req.Object = &validatorpb.SignRequest_AttestationData{AttestationData: data}
		got, err := v1.GetAttestationSignRequest(req, gvr)
		outputs = []mapped{{"attestation", encode(t, got, err), want}}

	case v1.TypeAggregationSlot:
		var slot primitives.SSZUint64
		require.NoError(t, slot.UnmarshalSSZ(ssz))
		objectRoot, err = slot.HashTreeRoot()
		require.NoError(t, err)
		domainType = cfg.DomainSelectionProof
		req.SigningSlot = primitives.Slot(slot)
		req.Object = &validatorpb.SignRequest_Slot{Slot: primitives.Slot(slot)}
		got, err := v1.GetAggregationSlotSignRequest(req, gvr)
		outputs = []mapped{{"aggregation slot", encode(t, got, err), want}}

	case v1.TypeAggregateAndProof:
		aggregate := &qrysmpb.AggregateAttestationAndProof{}
		require.NoError(t, aggregate.UnmarshalSSZ(ssz))
		objectRoot, err = aggregate.HashTreeRoot()
		require.NoError(t, err)
		domainType = cfg.DomainAggregateAndProof
		req.SigningSlot = aggregate.Aggregate.Data.Slot
		req.Object = &validatorpb.SignRequest_AggregateAttestationAndProof{AggregateAttestationAndProof: aggregate}
		got, err := v1.GetAggregateAndProofSignRequest(req, gvr)
		outputs = []mapped{{"aggregate and proof", encode(t, got, err), want}}

	case v1.TypeVoluntaryExit:
		exit := &qrysmpb.VoluntaryExit{}
		require.NoError(t, exit.UnmarshalSSZ(ssz))
		objectRoot, err = exit.HashTreeRoot()
		require.NoError(t, err)
		domainType = cfg.DomainVoluntaryExit
		// The validator client signs an exit at the start slot of its epoch.
		// The largest epochs have none, so they are mapped at the last slot.
		if req.SigningSlot, err = slots.EpochStart(exit.Epoch); err != nil {
			req.SigningSlot = math.MaxUint64
		}
		req.Object = &validatorpb.SignRequest_Exit{Exit: exit}
		got, err := v1.GetVoluntaryExitSignRequest(req, gvr)
		outputs = []mapped{{"voluntary exit", encode(t, got, err), want}}

	case v1.TypeSyncCommitteeMessage:
		// The object is the block root. The slot is the validator client's
		// signing slot, which only request_json carries.
		require.Equal(t, fieldparams.RootLength, len(ssz))
		objectRoot = [32]byte(ssz)
		domainType = cfg.DomainSyncCommittee
		slot, err := strconv.ParseUint(get(t, want, "sync_committee_message.slot").(string), 10, 64)
		require.NoError(t, err)
		req.SigningSlot = primitives.Slot(slot)
		req.Object = &validatorpb.SignRequest_SyncMessageBlockRoot{SyncMessageBlockRoot: ssz}
		got, err := v1.GetSyncCommitteeMessageSignRequest(req, gvr)
		outputs = []mapped{{"sync committee message", encode(t, got, err), want}}

	case v1.TypeSyncCommitteeSelectionProof:
		data := &qrysmpb.SyncAggregatorSelectionData{}
		require.NoError(t, data.UnmarshalSSZ(ssz))
		objectRoot, err = data.HashTreeRoot()
		require.NoError(t, err)
		domainType = cfg.DomainSyncCommitteeSelectionProof
		req.SigningSlot = data.Slot
		req.Object = &validatorpb.SignRequest_SyncAggregatorSelectionData{SyncAggregatorSelectionData: data}
		got, err := v1.GetSyncCommitteeSelectionProofSignRequest(req, gvr)
		outputs = []mapped{{"sync committee selection proof", encode(t, got, err), want}}

	case v1.TypeSyncCommitteeContributionAndProof:
		contribution := &qrysmpb.ContributionAndProof{}
		require.NoError(t, contribution.UnmarshalSSZ(ssz))
		objectRoot, err = contribution.HashTreeRoot()
		require.NoError(t, err)
		domainType = cfg.DomainContributionAndProof
		req.SigningSlot = contribution.Contribution.Slot
		// object_ssz_hex of a minimal record is the mainnet encoding, with the
		// bitvector padded by zero bytes (authoritative: false). A minimal
		// build holds the unpadded bitvector.
		bits := contribution.Contribution.AggregationBits
		if n := int(cfg.SyncCommitteeSize / cfg.SyncCommitteeSubnetCount / 8); n < len(bits) {
			require.NotNil(t, rec.Authoritative)
			require.Equal(t, false, *rec.Authoritative)
			require.DeepEqual(t, make([]byte, len(bits)-n), []byte(bits[n:]), "padding")
			contribution.Contribution.AggregationBits = bits[:n]
		}
		req.Object = &validatorpb.SignRequest_ContributionAndProof{ContributionAndProof: contribution}
		got, err := v1.GetSyncCommitteeContributionAndProofSignRequest(req, gvr)
		outputs = []mapped{{"contribution and proof", encode(t, got, err), want}}

	case v1.TypeValidatorRegistration:
		registration := &qrysmpb.ValidatorRegistrationV1{}
		require.NoError(t, registration.UnmarshalSSZ(ssz))
		objectRoot, err = registration.HashTreeRoot()
		require.NoError(t, err)
		domainType = cfg.DomainApplicationBuilder
		req.Object = &validatorpb.SignRequest_Registration{Registration: registration}
		got, err := v1.GetValidatorRegistrationSignRequest(req)
		outputs = []mapped{{"validator registration", encode(t, got, err), want}}

	default:
		t.Fatalf("unknown artifact type %s", rec.Type)
	}

	require.Equal(t, rec.ObjectRoot, hexutil.Encode(objectRoot[:]), "object_root")
	domain := signingDomain(t, rec.Type, domainType, req.SigningSlot, gvr)
	require.Equal(t, rec.Domain, hexutil.Encode(domain), "domain")
	signingRoot, err := signing.SigningData(func() ([32]byte, error) { return objectRoot, nil }, domain)
	require.NoError(t, err)
	require.Equal(t, rec.SigningRoot, hexutil.Encode(signingRoot[:]), "signing_root")

	for _, out := range outputs {
		requireRequestJSON(t, out.name, out.want, out.got)
		typ, err := mock.ValidateSignRequestJSON(out.got)
		require.NoError(t, err, out.name)
		require.Equal(t, rec.Type, typ)
	}
	typ, err := mock.ValidateSignRequestJSON(rec.RequestJSON)
	require.NoError(t, err, "request_json")
	require.Equal(t, rec.Type, typ)
}

// signingDomain computes a record's domain the way the validator client does:
// from the fork at the signing slot's epoch for every type but the
// registration, which uses the genesis fork version and a zero root.
func signingDomain(t *testing.T, typ string, domainType [4]byte, slot primitives.Slot, gvr []byte) []byte {
	t.Helper()
	if typ == v1.TypeValidatorRegistration {
		domain, err := signing.ComputeDomain(domainType, nil, nil)
		require.NoError(t, err)
		return domain
	}
	epoch := slots.ToEpoch(slot)
	fork, err := forks.Fork(epoch)
	require.NoError(t, err)
	domain, err := signing.Domain(fork, epoch, domainType, gvr)
	require.NoError(t, err)
	return domain
}

// blockV2Outputs maps a BLOCK_V2 record. The mapper takes a block, which only
// the real_block record carries, in full and blinded form. For the others,
// which hold only the header, it maps a block with the header's slot, proposer
// and parent and state roots, and expects request_json with that block's body
// root.
func blockV2Outputs(
	t *testing.T, rec signingRecord, header *qrysmpb.BeaconBlockHeader, req *validatorpb.SignRequest, gvr []byte, want map[string]any,
) []mapped {
	if rec.Block != nil {
		full := &qrysmpb.BeaconBlockZond{}
		require.NoError(t, full.UnmarshalSSZ(decodeHex(t, rec.Block.FullSSZHex)))
		blinded := &qrysmpb.BlindedBeaconBlockZond{}
		require.NoError(t, blinded.UnmarshalSSZ(decodeHex(t, rec.Block.BlindedSSZHex)))
		for _, root := range []func() ([32]byte, error){full.HashTreeRoot, blinded.HashTreeRoot} {
			r, err := root()
			require.NoError(t, err)
			require.Equal(t, rec.ObjectRoot, hexutil.Encode(r[:]), "block root")
		}
		req.Object = &validatorpb.SignRequest_BlockZond{BlockZond: full}
		gotFull, err := v1.GetBlockV2SignRequest(req, gvr)
		fullJSON := encode(t, gotFull, err)
		req.Object = &validatorpb.SignRequest_BlindedBlockZond{BlindedBlockZond: blinded}
		gotBlinded, err := v1.GetBlockV2SignRequest(req, gvr)
		return []mapped{{"full block", fullJSON, want}, {"blinded block", encode(t, gotBlinded, err), want}}
	}

	block := util.HydrateBeaconBlockZond(&qrysmpb.BeaconBlockZond{
		Slot:          header.Slot,
		ProposerIndex: header.ProposerIndex,
		ParentRoot:    header.ParentRoot,
		StateRoot:     header.StateRoot,
	})
	bodyRoot, err := block.Body.HashTreeRoot()
	require.NoError(t, err)
	get(t, want, "beacon_block.block_header").(map[string]any)["body_root"] = hexutil.Encode(bodyRoot[:])
	req.Object = &validatorpb.SignRequest_BlockZond{BlockZond: block}
	got, err := v1.GetBlockV2SignRequest(req, gvr)
	return []mapped{{"block with the header's fields", encode(t, got, err), want}}
}

// requireRequestJSON requires the mapper's JSON to equal the record's request
// body. Only the fee recipient's letter case may differ: go-qrl writes an
// address in its QIP-55 mixed-case form, request_json in lower case, and the
// contract accepts either case after the Q.
func requireRequestJSON(t *testing.T, name string, want map[string]any, got []byte) {
	t.Helper()
	gotJSON := decodeJSON(t, got)
	lowerFeeRecipient(want)
	lowerFeeRecipient(gotJSON)
	if !reflect.DeepEqual(want, gotJSON) {
		wantRaw, err := json.Marshal(want)
		require.NoError(t, err)
		t.Fatalf("%s: mapper output differs from request_json\nwant: %s\ngot:  %s", name, truncate(wantRaw), truncate(got))
	}
}

func lowerFeeRecipient(body map[string]any) {
	registration, _ := body["validator_registration"].(map[string]any)
	if feeRecipient, ok := registration["fee_recipient"].(string); ok && strings.HasPrefix(feeRecipient, "Q") {
		registration["fee_recipient"] = "Q" + strings.ToLower(feeRecipient[1:])
	}
}

func truncate(b []byte) string {
	const limit = 4000
	if len(b) > limit {
		return string(b[:limit]) + "…"
	}
	return string(b)
}

// specExamples returns the request_json of each minimal spec_example record,
// the contract's request example of its type.
func specExamples(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	examples := map[string]json.RawMessage{}
	for _, file := range loadSigningFiles(t, "minimal") {
		for _, rec := range file.Cases {
			if rec.Name == "spec_example" {
				examples[file.Type] = rec.RequestJSON
			}
		}
	}
	return examples
}

// TestStrictDecoder_RefusesContractViolations checks that the strict decoder
// standing in for the signer refuses the contract's 400 cases.
func TestStrictDecoder_RefusesContractViolations(t *testing.T) {
	examples := specExamples(t)
	edit := func(typ string, change func(body map[string]any)) []byte {
		raw, ok := examples[typ]
		require.Equal(t, true, ok, "no spec_example record for %s", typ)
		body := decodeJSON(t, raw)
		change(body)
		b, err := json.Marshal(body)
		require.NoError(t, err)
		return b
	}
	attestation := func(change func(body map[string]any)) []byte { return edit(v1.TypeAttestation, change) }
	aggregate := func(change func(agg map[string]any)) []byte {
		return edit(v1.TypeAggregateAndProof, func(body map[string]any) {
			change(get(t, body, "aggregate_and_proof.aggregate").(map[string]any))
		})
	}
	registration := func(change func(body map[string]any)) []byte { return edit(v1.TypeValidatorRegistration, change) }

	for _, typ := range []string{v1.TypeAttestation, v1.TypeAggregateAndProof, v1.TypeValidatorRegistration} {
		_, err := mock.ValidateSignRequestJSON(edit(typ, func(map[string]any) {}))
		require.NoError(t, err, typ)
	}
	bad := []struct {
		name string
		body []byte
	}{
		{"unknown field", attestation(func(b map[string]any) { b["extra"] = "x" })},
		{"number for uint64", attestation(func(b map[string]any) { get(t, b, "attestation").(map[string]any)["slot"] = 61 })},
		{"leading zero", attestation(func(b map[string]any) { get(t, b, "attestation").(map[string]any)["slot"] = "061" })},
		{"lower-case type", attestation(func(b map[string]any) { b["type"] = "attestation" })},
		{"missing fork_info", attestation(func(b map[string]any) { delete(b, "fork_info") })},
		{"short root", attestation(func(b map[string]any) { b["signing_root"] = "0x00" })},
		{"both signing root spellings", attestation(func(b map[string]any) { b["signingRoot"] = b["signing_root"] })},
		{"singular signature", aggregate(func(agg map[string]any) {
			agg["signature"] = agg["signatures"].([]any)[0]
			delete(agg, "signatures")
		})},
		{"popcount mismatch", aggregate(func(agg map[string]any) { agg["aggregation_bits"] = "0x0f" })},
		{"0x fee recipient", registration(func(b map[string]any) {
			reg := get(t, b, "validator_registration").(map[string]any)
			reg["fee_recipient"] = "0x" + reg["fee_recipient"].(string)[1:]
		})},
		{"fork_info on registration", registration(func(b map[string]any) { b["fork_info"] = map[string]any{} })},
		{"duplicate key", []byte(`{"type":"AGGREGATION_SLOT","type":"AGGREGATION_SLOT"}`)},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mock.ValidateSignRequestJSON(tt.body)
			require.NotNil(t, err)
		})
	}
}

// TestRandaoRevealRequest_Contract checks the RANDAO reveal request body
// against the contract: the slot, the commitment and the fork at the slot.
func TestRandaoRevealRequest_Contract(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	params.OverrideBeaconConfig(params.E2ETestConfig())
	gvr := decodeHex(t, "0x9471e8fb19e0faae953e9d46889f207192bcec9b14293d7ba418ae4dc5e3ac8f")
	commitment := decodeHex(t, "0x"+strings.Repeat("3c", fieldparams.RandaoCommitmentLength))

	got, err := v1.GetRandaoRevealRequest(61, commitment, gvr)
	body := encode(t, got, err)
	want := fmt.Sprintf(`{
		"slot": "61",
		"randao_commitment": "0x%s",
		"fork_info": {
			"fork": {"previous_version": "0x000000fd", "current_version": "0x000000fd", "epoch": "0"},
			"genesis_validators_root": "0x9471e8fb19e0faae953e9d46889f207192bcec9b14293d7ba418ae4dc5e3ac8f"
		}
	}`, strings.Repeat("3c", fieldparams.RandaoCommitmentLength))
	requireRequestJSON(t, "randao reveal request", decodeJSON(t, []byte(want)), body)
	require.NoError(t, mock.ValidateRandaoRevealRequestJSON(body))
}
