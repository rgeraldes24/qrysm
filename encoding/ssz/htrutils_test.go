package ssz_test

import (
	"reflect"
	"testing"

	fieldparams "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/encoding/ssz"
	enginev1 "github.com/theQRL/qrysm/proto/engine/v1"
	qrysmpb "github.com/theQRL/qrysm/proto/qrysm/v1alpha1"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

func TestUint64Root(t *testing.T) {
	uintVal := uint64(1234567890)
	expected := [32]byte{210, 2, 150, 73, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	result := ssz.Uint64Root(uintVal)
	assert.Equal(t, expected, result)
}

func TestForkRoot(t *testing.T) {
	testFork := qrysmpb.Fork{
		PreviousVersion: []byte{123},
		CurrentVersion:  []byte{124},
		Epoch:           1234567890,
	}
	expected := [32]byte{19, 46, 77, 103, 92, 175, 247, 33, 100, 64, 17, 111, 199, 145, 69, 38, 217, 112, 6, 16, 149, 201, 225, 144, 192, 228, 197, 172, 157, 78, 114, 140}

	result, err := ssz.ForkRoot(&testFork)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestCheckPointRoot(t *testing.T) {
	testCheckpoint := qrysmpb.Checkpoint{
		Epoch: 1234567890,
		Root:  []byte{222},
	}
	expected := [32]byte{228, 65, 39, 109, 183, 249, 167, 232, 125, 239, 25, 155, 207, 4, 84, 174, 176, 229, 175, 224, 62, 33, 215, 254, 170, 220, 132, 65, 246, 128, 68, 194}

	result, err := ssz.CheckpointRoot(&testCheckpoint)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestByteArrayRootWithLimit(t *testing.T) {
	testHistoricalRoots := [][]byte{{123}, {234}}
	expected := [32]byte{70, 204, 150, 196, 89, 138, 190, 205, 65, 207, 120, 166, 179, 247, 147, 20, 29, 133, 117, 116, 151, 234, 129, 32, 22, 15, 79, 178, 98, 73, 132, 152}

	result, err := ssz.ByteArrayRootWithLimit(testHistoricalRoots, 16777216)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestSlashingsRoot(t *testing.T) {
	testSlashingsRoot := []uint64{123, 234}
	expected := [32]byte{123, 0, 0, 0, 0, 0, 0, 0, 234, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	result, err := ssz.SlashingsRoot(testSlashingsRoot)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestTransactionsRoot(t *testing.T) {
	tests := []struct {
		name    string
		txs     [][]byte
		want    [32]byte
		wantErr bool
	}{
		{
			name: "nil",
			txs:  nil,
			want: [32]byte{127, 254, 36, 30, 166, 1, 135, 253, 176, 24, 123, 250, 34, 222, 53, 209, 249, 190, 215, 171, 6, 29, 148, 1, 253, 71, 227, 74, 84, 251, 237, 225},
		},
		{
			name: "empty",
			txs:  [][]byte{},
			want: [32]byte{127, 254, 36, 30, 166, 1, 135, 253, 176, 24, 123, 250, 34, 222, 53, 209, 249, 190, 215, 171, 6, 29, 148, 1, 253, 71, 227, 74, 84, 251, 237, 225},
		},
		{
			name: "one tx",
			txs:  [][]byte{{1, 2, 3}},
			want: [32]byte{102, 209, 140, 87, 217, 28, 68, 12, 133, 42, 77, 136, 191, 18, 234, 105, 166, 228, 216, 235, 230, 95, 200, 73, 85, 33, 134, 254, 219, 97, 82, 209},
		},
		{
			name: "max txs",
			txs: func() [][]byte {
				var txs [][]byte
				for range fieldparams.MaxTxsPerPayloadLength {
					txs = append(txs, []byte{})
				}
				return txs
			}(),
			want: [32]byte{13, 66, 254, 206, 203, 58, 48, 133, 78, 218, 48, 231, 120, 90, 38, 72, 73, 137, 86, 9, 31, 213, 185, 101, 103, 144, 0, 236, 225, 57, 47, 244},
		},
		{
			name: "exceed max txs",
			txs: func() [][]byte {
				var txs [][]byte
				for range fieldparams.MaxTxsPerPayloadLength + 1 {
					txs = append(txs, []byte{})
				}
				return txs
			}(),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ssz.TransactionsRoot(tt.txs)
			if (err != nil) != tt.wantErr {
				t.Errorf("TransactionsRoot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TransactionsRoot() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPackByChunk_SingleList(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  [][32]byte
	}{
		{
			name:  "nil",
			input: nil,
			want:  [][32]byte{{}},
		},
		{
			name:  "empty",
			input: []byte{},
			want:  [][32]byte{{}},
		},
		{
			name:  "one",
			input: []byte{1},
			want:  [][32]byte{{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		},
		{
			name:  "one, two",
			input: []byte{1, 2},
			want:  [][32]byte{{1, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ssz.PackByChunk([][]byte{tt.input})
			require.NoError(t, err)
			require.DeepSSZEqual(t, tt.want, got)
		})
	}
}

func TestWithdrawalRoot(t *testing.T) {
	tests := []struct {
		name    string
		input   *enginev1.Withdrawal
		want    [32]byte
		wantErr string
	}{
		{
			name:    "missing address",
			input:   &enginev1.Withdrawal{},
			wantErr: "incorrect byte size for --.Address: 0 != 64",
		},
		{
			name: "empty",
			input: &enginev1.Withdrawal{
				Address: make([]byte, fieldparams.FeeRecipientLength),
			},
			want: [32]byte{0x23, 0xd3, 0xe3, 0xd3, 0xc3, 0xbf, 0xbc, 0x0e, 0x8a, 0x0b, 0xd4, 0x33, 0xa9, 0x3a, 0xbb, 0x32, 0x79, 0x63, 0x17, 0x8a, 0x2f, 0x08, 0x19, 0x7f, 0x37, 0x74, 0x2f, 0x75, 0x11, 0x68, 0x92, 0x5d},
		},
		{
			name: "non-empty",
			input: &enginev1.Withdrawal{
				Index:          123,
				ValidatorIndex: 123123,
				Address:        withdrawalAddressForTest(),
				Amount:         50,
			},
			want: [32]byte{0x62, 0xbd, 0xa7, 0x20, 0xb8, 0x4e, 0x62, 0x87, 0x78, 0x3f, 0x06, 0x3b, 0x9f, 0xda, 0xb7, 0x25, 0x8e, 0x6d, 0xf5, 0xd8, 0x69, 0x19, 0x05, 0xe2, 0x4e, 0xe5, 0x8d, 0x8c, 0xbf, 0x0a, 0x13, 0x85},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ssz.WithdrawalRoot(tt.input)
			if tt.wantErr != "" {
				require.ErrorContains(t, tt.wantErr, err)
				return
			}
			require.NoError(t, err)
			generatedRoot, err := tt.input.HashTreeRoot()
			require.NoError(t, err)
			require.DeepSSZEqual(t, generatedRoot, got)
			require.DeepSSZEqual(t, tt.want, got)
		})
	}
}

func TestWithrawalSliceRoot(t *testing.T) {
	tests := []struct {
		name  string
		input []*enginev1.Withdrawal
		want  [32]byte
	}{
		{
			name:  "empty",
			input: make([]*enginev1.Withdrawal, 0),
			want:  [32]byte{0x79, 0x29, 0x30, 0xbb, 0xd5, 0xba, 0xac, 0x43, 0xbc, 0xc7, 0x98, 0xee, 0x49, 0xaa, 0x81, 0x85, 0xef, 0x76, 0xbb, 0x3b, 0x44, 0xba, 0x62, 0xb9, 0x1d, 0x86, 0xae, 0x56, 0x9e, 0x4b, 0xb5, 0x35},
		},
		{
			name: "non-empty",
			input: []*enginev1.Withdrawal{{
				Index:          123,
				ValidatorIndex: 123123,
				Address:        withdrawalAddressForTest(),
				Amount:         50,
			},
			},
			want: [32]byte{0x06, 0x67, 0x1d, 0xa7, 0x6b, 0x30, 0xfb, 0x39, 0x1f, 0x55, 0x9b, 0x86, 0x7c, 0xd0, 0x2c, 0x6f, 0x9b, 0x87, 0x91, 0xff, 0x77, 0x40, 0x24, 0xa2, 0x12, 0xc8, 0x18, 0xcb, 0x45, 0x25, 0x39, 0xc6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ssz.WithdrawalSliceRoot(tt.input, 16)
			require.NoError(t, err)
			require.DeepSSZEqual(t, tt.want, got)
		})
	}
}

func withdrawalAddressForTest() []byte {
	address := make([]byte, fieldparams.FeeRecipientLength)
	copy(address, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0})
	return address
}
