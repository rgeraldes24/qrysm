package mock

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
	"regexp"
	"sort"
	"strings"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
)

// This file checks request bodies against the remote signer contract
// (go-remotesigner api/openapi/consensus.yaml, version 1.0.0) the way the
// signer decodes them: unknown fields, duplicate keys and JSON numbers are
// rejected, every fixed-size value has its exact length, and aggregate
// signature lists match their aggregation bits. Tests use it to stand in for
// the signer's strict decoder.

// ValidateSignRequestJSON checks a sign request body and returns its type.
func ValidateSignRequestJSON(body []byte) (string, error) {
	v, err := parseStrict(body)
	if err != nil {
		return "", err
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return "", fmt.Errorf("body is not a JSON object")
	}
	typ, ok := obj["type"].(string)
	if !ok {
		return "", fmt.Errorf("type: missing or not a string")
	}
	schema, ok := signRequestSchemas[typ]
	if !ok {
		return "", fmt.Errorf("type: unknown artifact type %q", typ)
	}
	if _, both := obj["signingRoot"]; both {
		if _, ok := obj["signing_root"]; ok {
			return "", fmt.Errorf("both signing_root and signingRoot are present")
		}
	}
	if err := schema.check("", obj); err != nil {
		return "", err
	}
	return typ, nil
}

// ValidateRandaoRevealRequestJSON checks a RANDAO reveal request body.
func ValidateRandaoRevealRequestJSON(body []byte) error {
	v, err := parseStrict(body)
	if err != nil {
		return err
	}
	return randaoRevealSchema.check("", v)
}

// parseStrict decodes JSON into maps, slices, strings, json.Numbers, bools and
// nils, rejecting duplicate object keys and trailing data.
func parseStrict(body []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	v, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after the JSON value")
	}
	return v, nil
}

func parseValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := map[string]any{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key := keyTok.(string)
				if _, dup := obj[key]; dup {
					return nil, fmt.Errorf("duplicate key %q", key)
				}
				val, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				obj[key] = val
			}
			_, err := dec.Token() // '}'
			return obj, err
		case '[':
			arr := []any{}
			for dec.More() {
				val, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			_, err := dec.Token() // ']'
			return arr, err
		}
		return nil, fmt.Errorf("unexpected delimiter %v", t)
	default:
		return t, nil
	}
}

type node interface {
	check(path string, v any) error
}

type field struct {
	node     node
	optional bool
}

type object map[string]field

func (o object) check(path string, v any) error {
	obj, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: not an object", path)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := o[k]; !ok {
			return fmt.Errorf("%s: unknown field %q", path, k)
		}
	}
	for name, f := range o {
		val, ok := obj[name]
		if !ok {
			if f.optional {
				continue
			}
			return fmt.Errorf("%s: missing field %q", path, name)
		}
		if err := f.node.check(path+"."+name, val); err != nil {
			return err
		}
	}
	return nil
}

var uint64Pattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,19})$`)

type uint64String struct{}

func (uint64String) check(path string, v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("%s: uint64 must be a decimal string, got %T", path, v)
	}
	if !uint64Pattern.MatchString(s) || (len(s) == 20 && s > "18446744073709551615") {
		return fmt.Errorf("%s: %q is not a canonical uint64", path, s)
	}
	return nil
}

type enum []string

func (e enum) check(path string, v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("%s: not a string", path)
	}
	for _, want := range e {
		if s == want {
			return nil
		}
	}
	return fmt.Errorf("%s: %q is not one of %v", path, s, []string(e))
}

func decodeHex(path string, v any, prefix string) ([]byte, error) {
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("%s: not a string", path)
	}
	if !strings.HasPrefix(s, prefix) {
		return nil, fmt.Errorf("%s: missing %q prefix", path, prefix)
	}
	b, err := hex.DecodeString(s[len(prefix):])
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	return b, nil
}

// fixedHex is 0x + hex of exactly n bytes, any of the lengths given.
type fixedHex []int

func (f fixedHex) check(path string, v any) error {
	_, err := f.decode(path, v)
	return err
}

func (f fixedHex) decode(path string, v any) ([]byte, error) {
	b, err := decodeHex(path, v, "0x")
	if err != nil {
		return nil, err
	}
	for _, n := range f {
		if len(b) == n {
			return b, nil
		}
	}
	return nil, fmt.Errorf("%s: %d bytes, want %v", path, len(b), []int(f))
}

type qAddress struct{}

func (qAddress) check(path string, v any) error {
	b, err := decodeHex(path, v, "Q")
	if err != nil {
		return err
	}
	if len(b) != fieldparams.FeeRecipientLength {
		return fmt.Errorf("%s: %d bytes, want %d", path, len(b), fieldparams.FeeRecipientLength)
	}
	return nil
}

type signatureList struct{ min, max int }

func (s signatureList) check(path string, v any) error {
	arr, ok := v.([]any)
	if !ok {
		return fmt.Errorf("%s: not an array", path)
	}
	if len(arr) < s.min || len(arr) > s.max {
		return fmt.Errorf("%s: %d signatures, want %d to %d", path, len(arr), s.min, s.max)
	}
	for i, sig := range arr {
		if err := signature.check(fmt.Sprintf("%s[%d]", path, i), sig); err != nil {
			return err
		}
	}
	return nil
}

// aggregate checks that the number of signatures equals the number of set
// aggregation bits. For a bitlist the delimiter bit is excluded.
type aggregate struct {
	object  object
	bits    fixedHex
	bitlist bool
}

func (a aggregate) check(path string, v any) error {
	if err := a.object.check(path, v); err != nil {
		return err
	}
	obj := v.(map[string]any)
	raw, err := a.bits.decode(path+".aggregation_bits", obj["aggregation_bits"])
	if err != nil {
		return err
	}
	set := 0
	for _, b := range raw {
		set += bits.OnesCount8(b)
	}
	if a.bitlist {
		if raw[len(raw)-1] == 0 {
			return fmt.Errorf("%s.aggregation_bits: bitlist has no delimiter bit", path)
		}
		set--
	}
	if n := len(obj["signatures"].([]any)); n != set {
		return fmt.Errorf("%s: %d signatures for %d set aggregation bits", path, n, set)
	}
	return nil
}

var (
	u64       = uint64String{}
	root      = fixedHex{fieldparams.RootLength}
	version   = fixedHex{fieldparams.VersionLength}
	signature = fixedHex{fieldparams.MLDSA87SignatureLength}
	pubkey    = fixedHex{fieldparams.MLDSA87PubkeyLength}

	forkInfo = object{
		"fork": {node: object{
			"previous_version": {node: version},
			"current_version":  {node: version},
			"epoch":            {node: u64},
		}},
		"genesis_validators_root": {node: root},
	}
	checkpoint      = object{"epoch": {node: u64}, "root": {node: root}}
	attestationData = object{
		"slot":              {node: u64},
		"index":             {node: u64},
		"beacon_block_root": {node: root},
		"source":            {node: checkpoint},
		"target":            {node: checkpoint},
	}
	attestation = aggregate{
		object: object{
			"aggregation_bits": {node: fixedHex{1, 2, 3, 4, 5}},
			"data":             {node: attestationData},
			"signatures":       {node: signatureList{0, 32}},
		},
		bits:    fixedHex{1, 2, 3, 4, 5},
		bitlist: true,
	}
	contribution = aggregate{
		object: object{
			"slot":               {node: u64},
			"beacon_block_root":  {node: root},
			"subcommittee_index": {node: u64},
			"aggregation_bits":   {node: fixedHex{2, 16}},
			"signatures":         {node: signatureList{1, 128}},
		},
		bits: fixedHex{2, 16},
	}

	signRequestSchemas = map[string]object{
		"BLOCK_V2": envelope("BLOCK_V2", "beacon_block", object{
			"version": {node: enum{"ZOND"}},
			"block_header": {node: object{
				"slot":           {node: u64},
				"proposer_index": {node: u64},
				"parent_root":    {node: root},
				"state_root":     {node: root},
				"body_root":      {node: root},
			}},
		}),
		"ATTESTATION":      envelope("ATTESTATION", "attestation", attestationData),
		"AGGREGATION_SLOT": envelope("AGGREGATION_SLOT", "aggregation_slot", object{"slot": {node: u64}}),
		"AGGREGATE_AND_PROOF": envelope("AGGREGATE_AND_PROOF", "aggregate_and_proof", object{
			"aggregator_index": {node: u64},
			"aggregate":        {node: attestation},
			"selection_proof":  {node: signature},
		}),
		"VOLUNTARY_EXIT": envelope("VOLUNTARY_EXIT", "voluntary_exit", object{
			"epoch":           {node: u64},
			"validator_index": {node: u64},
		}),
		"SYNC_COMMITTEE_MESSAGE": envelope("SYNC_COMMITTEE_MESSAGE", "sync_committee_message", object{
			"beacon_block_root": {node: root},
			"slot":              {node: u64},
		}),
		"SYNC_COMMITTEE_SELECTION_PROOF": envelope("SYNC_COMMITTEE_SELECTION_PROOF", "sync_aggregator_selection_data", object{
			"slot":               {node: u64},
			"subcommittee_index": {node: u64},
		}),
		"SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF": envelope("SYNC_COMMITTEE_CONTRIBUTION_AND_PROOF", "contribution_and_proof", object{
			"aggregator_index": {node: u64},
			"selection_proof":  {node: signature},
			"contribution":     {node: contribution},
		}),
		"VALIDATOR_REGISTRATION": {
			"type":         {node: enum{"VALIDATOR_REGISTRATION"}},
			"signing_root": {node: root, optional: true},
			"signingRoot":  {node: root, optional: true},
			"validator_registration": {node: object{
				"fee_recipient": {node: qAddress{}},
				"gas_limit":     {node: u64},
				"timestamp":     {node: u64},
				"pubkey":        {node: pubkey},
			}},
		},
	}

	randaoRevealSchema = object{
		"slot":              {node: u64},
		"randao_commitment": {node: root},
		"fork_info":         {node: forkInfo},
	}
)

func envelope(typ, payloadKey string, payload node) object {
	return object{
		"type":         {node: enum{typ}},
		"fork_info":    {node: forkInfo},
		"signing_root": {node: root, optional: true},
		"signingRoot":  {node: root, optional: true},
		payloadKey:     {node: payload},
	}
}
