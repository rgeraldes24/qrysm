package builder

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/common/hexutil"
	types "github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/math"
	v1 "github.com/theQRL/qrysm/proto/engine/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

var errInvalidUint256 = errors.New("invalid Uint256")
var errDecodeUint256 = errors.New("unable to decode into Uint256")

// Uint256 a wrapper representation of big.Int
type Uint256 struct {
	*big.Int
}

// sszBytesToUint256 creates a Uint256 from a ssz-style (little-endian byte slice) representation.
func sszBytesToUint256(b []byte) (Uint256, error) {
	bi := bytesutil.LittleEndianBytesToBigInt(b)
	if !math.IsValidUint256(bi) {
		return Uint256{}, errors.Wrapf(errDecodeUint256, "value=%s", b)
	}
	return Uint256{Int: bi}, nil
}

// SSZBytes creates an ssz-style (little-endian byte slice) representation of the Uint256.
func (s Uint256) SSZBytes() []byte {
	if !math.IsValidUint256(s.Int) {
		return []byte{}
	}
	return bytesutil.PadTo(bytesutil.ReverseByteOrder(s.Int.Bytes()), 32)
}

// UnmarshalJSON takes in a byte array and unmarshals the value in Uint256
func (s *Uint256) UnmarshalJSON(t []byte) error {
	end := len(t)
	if len(t) < 2 {
		return errors.Errorf("provided Uint256 json string is too short: %s", string(t))
	}
	if t[0] != '"' || t[end-1] != '"' {
		return errors.Errorf("provided Uint256 json string is malformed: %s", string(t))
	}
	return s.UnmarshalText(t[1 : end-1])
}

// UnmarshalText takes in a byte array and unmarshals the text in Uint256
func (s *Uint256) UnmarshalText(t []byte) error {
	if s.Int == nil {
		s.Int = big.NewInt(0)
	}
	z, ok := s.SetString(string(t), 10)
	if !ok {
		return errors.Wrapf(errDecodeUint256, "value=%s", t)
	}
	if !math.IsValidUint256(z) {
		return errors.Wrapf(errDecodeUint256, "value=%s", t)
	}
	s.Int = z
	return nil
}

// MarshalJSON returns a json byte representation of Uint256.
func (s Uint256) MarshalJSON() ([]byte, error) {
	t, err := s.MarshalText()
	if err != nil {
		return nil, err
	}
	t = append([]byte{'"'}, t...)
	t = append(t, '"')
	return t, nil
}

// MarshalText returns a text byte representation of Uint256.
func (s Uint256) MarshalText() ([]byte, error) {
	if !math.IsValidUint256(s.Int) {
		return nil, errors.Wrapf(errInvalidUint256, "value=%s", s.Int)
	}
	return []byte(s.String()), nil
}

// Uint64String is a custom type that allows marshalling from text to uint64 and vice versa.
type Uint64String uint64

// UnmarshalText takes a byte array and unmarshals the text in Uint64String.
func (s *Uint64String) UnmarshalText(t []byte) error {
	u, err := strconv.ParseUint(string(t), 10, 64)
	*s = Uint64String(u)
	return err
}

// MarshalText returns a byte representation of the text from Uint64String.
func (s Uint64String) MarshalText() ([]byte, error) {
	return fmt.Appendf(nil, "%d", s), nil
}

// VersionResponse is a JSON representation of a field in the builder API header response.
type VersionResponse struct {
	Version string `json:"version"`
}

// FromProtoZond converts a proto execution payload type for zond to our
// builder compatible payload type.
func FromProtoZond(payload *v1.ExecutionPayloadZond) (ExecutionPayloadZond, error) {
	bFee, err := sszBytesToUint256(payload.BaseFeePerGas)
	if err != nil {
		return ExecutionPayloadZond{}, err
	}
	txs := make([]hexutil.Bytes, len(payload.Transactions))
	for i := range payload.Transactions {
		txs[i] = bytesutil.SafeCopyBytes(payload.Transactions[i])
	}
	withdrawals := make([]Withdrawal, len(payload.Withdrawals))
	for i, w := range payload.Withdrawals {
		withdrawals[i] = Withdrawal{
			Index:          Uint256{Int: big.NewInt(0).SetUint64(w.Index)},
			ValidatorIndex: Uint256{Int: big.NewInt(0).SetUint64(uint64(w.ValidatorIndex))},
			Address:        bytesutil.SafeCopyBytes(w.Address),
			Amount:         Uint256{Int: big.NewInt(0).SetUint64(w.Amount)},
		}
	}
	return ExecutionPayloadZond{
		ParentHash:    bytesutil.SafeCopyBytes(payload.ParentHash),
		FeeRecipient:  bytesutil.SafeCopyBytes(payload.FeeRecipient),
		StateRoot:     bytesutil.SafeCopyBytes(payload.StateRoot),
		ReceiptsRoot:  bytesutil.SafeCopyBytes(payload.ReceiptsRoot),
		LogsBloom:     bytesutil.SafeCopyBytes(payload.LogsBloom),
		PrevRandao:    bytesutil.SafeCopyBytes(payload.PrevRandao),
		BlockNumber:   Uint64String(payload.BlockNumber),
		GasLimit:      Uint64String(payload.GasLimit),
		GasUsed:       Uint64String(payload.GasUsed),
		Timestamp:     Uint64String(payload.Timestamp),
		ExtraData:     bytesutil.SafeCopyBytes(payload.ExtraData),
		BaseFeePerGas: bFee,
		BlockHash:     bytesutil.SafeCopyBytes(payload.BlockHash),
		Transactions:  txs,
		Withdrawals:   withdrawals,
	}, nil
}

// ExecHeaderResponseZond is the response of builder API /qrl/v1/builder/header/{slot}/{parent_hash}/{pubkey} for Zond.
type ExecHeaderResponseZond struct {
	Data struct {
		Signature hexutil.Bytes   `json:"signature"`
		Message   *BuilderBidZond `json:"message"`
	} `json:"data"`
}

// ToProto returns a SignedBuilderBidZond Proto from ExecHeaderResponseZond.
//
// The response comes from a relay the node does not control, so every
// pointer and big.Int it may have left unset is checked here and reported as
// an error: a panic in this path unwinds through GetBeaconBlock and costs the
// proposal, whereas an error lets the proposer fall back to the local payload.
func (ehr *ExecHeaderResponseZond) ToProto() (*qrysmpb.SignedBuilderBidZond, error) {
	if ehr.Data.Message == nil {
		return nil, errors.New("builder header response has no message")
	}
	bb, err := ehr.Data.Message.ToProto()
	if err != nil {
		return nil, err
	}
	return &qrysmpb.SignedBuilderBidZond{
		Message:   bb,
		Signature: bytesutil.SafeCopyBytes(ehr.Data.Signature),
	}, nil
}

// ToProto returns a BuilderBidZond Proto.
func (bb *BuilderBidZond) ToProto() (*qrysmpb.BuilderBidZond, error) {
	if bb.Header == nil {
		return nil, errors.New("builder bid has no header")
	}
	if bb.Value.Int == nil {
		return nil, errors.New("builder bid has no value")
	}
	header, err := bb.Header.ToProto()
	if err != nil {
		return nil, err
	}
	return &qrysmpb.BuilderBidZond{
		Header: header,
		Value:  bytesutil.SafeCopyBytes(bb.Value.SSZBytes()),
		Pubkey: bytesutil.SafeCopyBytes(bb.Pubkey),
	}, nil
}

// ToProto returns an ExecutionPayloadHeaderZond Proto
func (h *ExecutionPayloadHeaderZond) ToProto() (*v1.ExecutionPayloadHeaderZond, error) {
	if h.BaseFeePerGas.Int == nil {
		return nil, errors.New("execution payload header has no base_fee_per_gas")
	}
	return &v1.ExecutionPayloadHeaderZond{
		ParentHash:       bytesutil.SafeCopyBytes(h.ParentHash),
		FeeRecipient:     bytesutil.SafeCopyBytes(h.FeeRecipient),
		StateRoot:        bytesutil.SafeCopyBytes(h.StateRoot),
		ReceiptsRoot:     bytesutil.SafeCopyBytes(h.ReceiptsRoot),
		LogsBloom:        bytesutil.SafeCopyBytes(h.LogsBloom),
		PrevRandao:       bytesutil.SafeCopyBytes(h.PrevRandao),
		BlockNumber:      uint64(h.BlockNumber),
		GasLimit:         uint64(h.GasLimit),
		GasUsed:          uint64(h.GasUsed),
		Timestamp:        uint64(h.Timestamp),
		ExtraData:        bytesutil.SafeCopyBytes(h.ExtraData),
		BaseFeePerGas:    bytesutil.SafeCopyBytes(h.BaseFeePerGas.SSZBytes()),
		BlockHash:        bytesutil.SafeCopyBytes(h.BlockHash),
		TransactionsRoot: bytesutil.SafeCopyBytes(h.TransactionsRoot),
		WithdrawalsRoot:  bytesutil.SafeCopyBytes(h.WithdrawalsRoot),
	}, nil
}

// BuilderBidZond is field of ExecHeaderResponseZond.
type BuilderBidZond struct {
	Header *ExecutionPayloadHeaderZond `json:"header"`
	Value  Uint256                     `json:"value"`
	Pubkey hexutil.Bytes               `json:"pubkey"`
}

// ExecutionPayloadHeaderZond is a field in BuilderBidZond.
type ExecutionPayloadHeaderZond struct {
	ParentHash       hexutil.Bytes  `json:"parent_hash"`
	FeeRecipient     hexutil.BytesQ `json:"fee_recipient"`
	StateRoot        hexutil.Bytes  `json:"state_root"`
	ReceiptsRoot     hexutil.Bytes  `json:"receipts_root"`
	LogsBloom        hexutil.Bytes  `json:"logs_bloom"`
	PrevRandao       hexutil.Bytes  `json:"prev_randao"`
	BlockNumber      Uint64String   `json:"block_number"`
	GasLimit         Uint64String   `json:"gas_limit"`
	GasUsed          Uint64String   `json:"gas_used"`
	Timestamp        Uint64String   `json:"timestamp"`
	ExtraData        hexutil.Bytes  `json:"extra_data"`
	BaseFeePerGas    Uint256        `json:"base_fee_per_gas"`
	BlockHash        hexutil.Bytes  `json:"block_hash"`
	TransactionsRoot hexutil.Bytes  `json:"transactions_root"`
	WithdrawalsRoot  hexutil.Bytes  `json:"withdrawals_root"`
	*v1.ExecutionPayloadHeaderZond
}

// MarshalJSON returns a JSON byte representation of ExecutionPayloadHeaderZond.
func (h *ExecutionPayloadHeaderZond) MarshalJSON() ([]byte, error) {
	type MarshalCaller ExecutionPayloadHeaderZond
	baseFeePerGas, err := sszBytesToUint256(h.ExecutionPayloadHeaderZond.BaseFeePerGas)
	if err != nil {
		return []byte{}, errors.Wrapf(err, "invalid BaseFeePerGas")
	}
	return json.Marshal(&MarshalCaller{
		ParentHash:       h.ExecutionPayloadHeaderZond.ParentHash,
		FeeRecipient:     h.ExecutionPayloadHeaderZond.FeeRecipient,
		StateRoot:        h.ExecutionPayloadHeaderZond.StateRoot,
		ReceiptsRoot:     h.ExecutionPayloadHeaderZond.ReceiptsRoot,
		LogsBloom:        h.ExecutionPayloadHeaderZond.LogsBloom,
		PrevRandao:       h.ExecutionPayloadHeaderZond.PrevRandao,
		BlockNumber:      Uint64String(h.ExecutionPayloadHeaderZond.BlockNumber),
		GasLimit:         Uint64String(h.ExecutionPayloadHeaderZond.GasLimit),
		GasUsed:          Uint64String(h.ExecutionPayloadHeaderZond.GasUsed),
		Timestamp:        Uint64String(h.ExecutionPayloadHeaderZond.Timestamp),
		ExtraData:        h.ExecutionPayloadHeaderZond.ExtraData,
		BaseFeePerGas:    baseFeePerGas,
		BlockHash:        h.ExecutionPayloadHeaderZond.BlockHash,
		TransactionsRoot: h.ExecutionPayloadHeaderZond.TransactionsRoot,
		WithdrawalsRoot:  h.ExecutionPayloadHeaderZond.WithdrawalsRoot,
	})
}

// UnmarshalJSON takes a JSON byte array and sets ExecutionPayloadHeaderZond.
func (h *ExecutionPayloadHeaderZond) UnmarshalJSON(b []byte) error {
	type UnmarshalCaller ExecutionPayloadHeaderZond
	uc := &UnmarshalCaller{}
	if err := json.Unmarshal(b, uc); err != nil {
		return err
	}
	ep := ExecutionPayloadHeaderZond(*uc)
	*h = ep
	var err error
	h.ExecutionPayloadHeaderZond, err = h.ToProto()
	return err
}

// ExecPayloadResponseZond is the builder API /qrl/v1/builder/blinded_blocks for Zond.
type ExecPayloadResponseZond struct {
	Version string               `json:"version"`
	Data    ExecutionPayloadZond `json:"data"`
}

// ExecutionPayloadZond is a field of ExecPayloadResponseZond.
type ExecutionPayloadZond struct {
	ParentHash    hexutil.Bytes   `json:"parent_hash"`
	FeeRecipient  hexutil.BytesQ  `json:"fee_recipient"`
	StateRoot     hexutil.Bytes   `json:"state_root"`
	ReceiptsRoot  hexutil.Bytes   `json:"receipts_root"`
	LogsBloom     hexutil.Bytes   `json:"logs_bloom"`
	PrevRandao    hexutil.Bytes   `json:"prev_randao"`
	BlockNumber   Uint64String    `json:"block_number"`
	GasLimit      Uint64String    `json:"gas_limit"`
	GasUsed       Uint64String    `json:"gas_used"`
	Timestamp     Uint64String    `json:"timestamp"`
	ExtraData     hexutil.Bytes   `json:"extra_data"`
	BaseFeePerGas Uint256         `json:"base_fee_per_gas"`
	BlockHash     hexutil.Bytes   `json:"block_hash"`
	Transactions  []hexutil.Bytes `json:"transactions"`
	Withdrawals   []Withdrawal    `json:"withdrawals"`
}

// ToProto returns an ExecutionPayloadZond Proto.
func (r *ExecPayloadResponseZond) ToProto() (*v1.ExecutionPayloadZond, error) {
	return r.Data.ToProto()
}

// ToProto returns an ExecutionPayloadZond Proto.
//
// Like the header response, this is relay-supplied data: unset numeric
// fields are rejected rather than dereferenced.
func (p *ExecutionPayloadZond) ToProto() (*v1.ExecutionPayloadZond, error) {
	if p.BaseFeePerGas.Int == nil {
		return nil, errors.New("execution payload has no base_fee_per_gas")
	}
	txs := make([][]byte, len(p.Transactions))
	for i := range p.Transactions {
		txs[i] = bytesutil.SafeCopyBytes(p.Transactions[i])
	}
	withdrawals := make([]*v1.Withdrawal, len(p.Withdrawals))
	for i, w := range p.Withdrawals {
		if w.Index.Int == nil || w.ValidatorIndex.Int == nil || w.Amount.Int == nil {
			return nil, errors.Errorf("withdrawal %d is missing index, validator_index or amount", i)
		}
		withdrawals[i] = &v1.Withdrawal{
			Index:          w.Index.Uint64(),
			ValidatorIndex: types.ValidatorIndex(w.ValidatorIndex.Uint64()),
			Address:        bytesutil.SafeCopyBytes(w.Address),
			Amount:         w.Amount.Uint64(),
		}
	}
	return &v1.ExecutionPayloadZond{
		ParentHash:    bytesutil.SafeCopyBytes(p.ParentHash),
		FeeRecipient:  bytesutil.SafeCopyBytes(p.FeeRecipient),
		StateRoot:     bytesutil.SafeCopyBytes(p.StateRoot),
		ReceiptsRoot:  bytesutil.SafeCopyBytes(p.ReceiptsRoot),
		LogsBloom:     bytesutil.SafeCopyBytes(p.LogsBloom),
		PrevRandao:    bytesutil.SafeCopyBytes(p.PrevRandao),
		BlockNumber:   uint64(p.BlockNumber),
		GasLimit:      uint64(p.GasLimit),
		GasUsed:       uint64(p.GasUsed),
		Timestamp:     uint64(p.Timestamp),
		ExtraData:     bytesutil.SafeCopyBytes(p.ExtraData),
		BaseFeePerGas: bytesutil.SafeCopyBytes(p.BaseFeePerGas.SSZBytes()),
		BlockHash:     bytesutil.SafeCopyBytes(p.BlockHash),
		Transactions:  txs,
		Withdrawals:   withdrawals,
	}, nil
}

// Withdrawal is a field of ExecutionPayloadZond.
type Withdrawal struct {
	Index          Uint256        `json:"index"`
	ValidatorIndex Uint256        `json:"validator_index"`
	Address        hexutil.BytesQ `json:"address"`
	Amount         Uint256        `json:"amount"`
}

// ErrorMessage is a JSON representation of the builder API's returned error message.
type ErrorMessage struct {
	Code        int      `json:"code"`
	Message     string   `json:"message"`
	Stacktraces []string `json:"stacktraces,omitempty"`
}
