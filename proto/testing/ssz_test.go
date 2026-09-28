package testing

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/theQRL/go-bitfield"
	engine "github.com/theQRL/qrysm/proto/engine/v1"
	v1 "github.com/theQRL/qrysm/proto/qrl/v1"
	alpha "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
)

type sszCodec interface {
	MarshalSSZ() ([]byte, error)
	UnmarshalSSZ([]byte) error
	HashTreeRoot() ([32]byte, error)
}

func TestSSZUnmarshal_InvalidTransactionOffset(t *testing.T) {
	// A payload with an empty extra-data field, one empty transaction, and no withdrawals.
	encoded := make([]byte, 560)
	binary.LittleEndian.PutUint32(encoded[480:484], 556)
	binary.LittleEndian.PutUint32(encoded[548:552], 556)
	binary.LittleEndian.PutUint32(encoded[552:556], 560)
	binary.LittleEndian.PutUint32(encoded[556:], 4)
	var payload engine.ExecutionPayloadZond
	if err := payload.UnmarshalSSZ(encoded); err != nil {
		t.Fatal(err)
	}
	malformed := bytes.Clone(encoded)
	// Claim thousands of offsets while providing only four transaction bytes.
	binary.LittleEndian.PutUint32(malformed[556:], 4096*4)
	var decodeErr error
	allocations := testing.AllocsPerRun(2, func() {
		decodeErr = payload.UnmarshalSSZ(malformed)
	})
	if decodeErr == nil {
		t.Fatal("invalid transaction offset accepted")
	}
	if allocations != 0 {
		t.Fatalf("invalid offset allocated before rejection: %v allocations", allocations)
	}
	if err := payload.UnmarshalSSZ(encoded); err != nil {
		t.Fatal(err)
	}
	if len(payload.Transactions) != 1 || len(payload.Transactions[0]) != 0 {
		t.Fatal("valid retry decoded incorrect transactions")
	}
}

func TestSSZBitvectorPadding(t *testing.T) {
	for _, value := range []byte{0, 0x0f, 0x10, 0x80, 0xff} {
		t.Run(fmt.Sprintf("%02x", value), func(t *testing.T) {
			obj := &alpha.MetaDataV1{Attnets: make([]byte, 8), Syncnets: bitfield.Bitvector4{value}}
			_, hashErr := obj.HashTreeRoot()
			_, marshalErr := obj.MarshalSSZ()
			var decoded alpha.MetaDataV1
			decodeErr := decoded.UnmarshalSSZ(append(make([]byte, 16), value))
			if value > 0x0f {
				if hashErr == nil || marshalErr == nil || decodeErr == nil {
					t.Fatalf("nonzero padding accepted: hash %v, marshal %v, decode %v", hashErr, marshalErr, decodeErr)
				}
			} else if hashErr != nil || marshalErr != nil || decodeErr != nil {
				t.Fatalf("valid bitvector rejected: hash %v, marshal %v, decode %v", hashErr, marshalErr, decodeErr)
			}
		})
	}
}

func alphaAttestation(bits bitfield.Bitlist) sszCodec {
	return &alpha.Attestation{
		AggregationBits: bits,
		Data: &alpha.AttestationData{
			BeaconBlockRoot: bytes.Repeat([]byte{1}, 32),
			Source:          &alpha.Checkpoint{Root: bytes.Repeat([]byte{2}, 32)},
			Target:          &alpha.Checkpoint{Root: bytes.Repeat([]byte{3}, 32)},
		},
	}
}

func v1Attestation(bits bitfield.Bitlist) sszCodec {
	return &v1.Attestation{
		AggregationBits: bits,
		Data: &v1.AttestationData{
			BeaconBlockRoot: bytes.Repeat([]byte{1}, 32),
			Source:          &v1.Checkpoint{Root: bytes.Repeat([]byte{2}, 32)},
			Target:          &v1.Checkpoint{Root: bytes.Repeat([]byte{3}, 32)},
		},
	}
}

func TestSSZUnmarshal_ReusedReceiver(t *testing.T) {
	factories := map[string]func(byte) sszCodec{
		"withdrawal": func(seed byte) sszCodec {
			return &engine.Withdrawal{Address: bytes.Repeat([]byte{seed}, 64), Amount: uint64(seed)}
		},
		"alpha attestation": func(seed byte) sszCodec {
			return alphaAttestation(bitfield.NewBitlist(uint64(seed)))
		},
		"v1 attestation": func(seed byte) sszCodec {
			return v1Attestation(bitfield.NewBitlist(uint64(seed)))
		},
		"execution payload": func(seed byte) sszCodec {
			return &engine.ExecutionPayloadZond{
				ParentHash: bytes.Repeat([]byte{seed}, 32), FeeRecipient: make([]byte, 64),
				StateRoot: make([]byte, 32), ReceiptsRoot: make([]byte, 32),
				LogsBloom: make([]byte, 256), PrevRandao: make([]byte, 32),
				ExtraData: bytes.Repeat([]byte{seed}, int(seed)), BaseFeePerGas: make([]byte, 32),
				BlockHash: make([]byte, 32), Transactions: [][]byte{{seed, seed}},
				Withdrawals: []*engine.Withdrawal{{Address: bytes.Repeat([]byte{seed}, 64)}},
			}
		},
	}
	for name, factory := range factories {
		t.Run(name, func(t *testing.T) {
			receiver := factory(1)
			for _, seed := range []byte{2, 3, 1} {
				input := factory(seed)
				encoded, err := input.MarshalSSZ()
				if err != nil {
					t.Fatal(err)
				}
				wantRoot, err := input.HashTreeRoot()
				if err != nil {
					t.Fatal(err)
				}
				if err := receiver.UnmarshalSSZ(encoded); err != nil {
					t.Fatal(err)
				}
				got, err := receiver.MarshalSSZ()
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(encoded, got) {
					t.Fatal("reused receiver retained previous SSZ data")
				}
				gotRoot, err := receiver.HashTreeRoot()
				if err != nil {
					t.Fatal(err)
				}
				if gotRoot != wantRoot {
					t.Fatal("reused receiver has a different root")
				}
			}
		})
	}
}

func TestSSZBitlistBounds(t *testing.T) {
	for name, factory := range map[string]func(bitfield.Bitlist) sszCodec{"alpha": alphaAttestation, "v1": v1Attestation} {
		t.Run(name, func(t *testing.T) {
			cases := []struct {
				name  string
				bits  bitfield.Bitlist
				valid bool
			}{
				{"nil", nil, false}, {"missing delimiter", bitfield.Bitlist{0}, false},
				{"trailing zero", bitfield.Bitlist{1, 0}, false},
			}
			for _, size := range []uint64{0, 1, 31, 32, 33, 255, 256} {
				cases = append(cases, struct {
					name  string
					bits  bitfield.Bitlist
					valid bool
				}{fmt.Sprint(size), bitfield.NewBitlist(size), size <= 32})
			}
			for _, tt := range cases {
				t.Run(tt.name, func(t *testing.T) {
					obj := factory(tt.bits)
					_, hashErr := obj.HashTreeRoot()
					encoded, marshalErr := obj.MarshalSSZ()
					if !tt.valid {
						if hashErr == nil || marshalErr == nil {
							t.Fatalf("invalid bitlist accepted: hash %v, marshal %v", hashErr, marshalErr)
						}
						return
					}
					if hashErr != nil || marshalErr != nil {
						t.Fatalf("valid bitlist rejected: hash %v, marshal %v", hashErr, marshalErr)
					}
					if err := factory(nil).UnmarshalSSZ(encoded); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
