package bytesutil_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/theQRL/qrysm/encoding/bytesutil"
	"github.com/theQRL/qrysm/testing/assert"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		a []byte
		b []byte
	}{
		{[]byte{'A', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O'},
			[]byte{'A', 'C', 'D', 'E', 'F', 'G'}},
		{[]byte{'A', 'C', 'D', 'E', 'F'},
			[]byte{'A', 'C', 'D', 'E', 'F'}},
		{[]byte{}, []byte{}},
	}
	for _, tt := range tests {
		b := bytesutil.Trunc(tt.a)
		assert.DeepEqual(t, tt.b, b)
	}
}

func TestSafeCopy2dBytes(t *testing.T) {
	tests := []struct {
		name  string
		input [][]byte
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name:  "correct copy",
			input: [][]byte{{0x1}, {0x2}},
		},
		{
			name:  "empty",
			input: [][]byte{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bytesutil.SafeCopy2dBytes(tt.input); !reflect.DeepEqual(got, tt.input) {
				t.Errorf("SafeCopy2dBytes() = %v, want %v", got, tt.input)
			}
		})
	}
}

func TestBytesInvalidInputs(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Test panicked: %v", r)
		}
	}()
	rawBytes := bytesutil.ToBytes(100, -10)
	assert.DeepEqual(t, []byte{}, rawBytes)

	_, err := bytesutil.HighestBitIndexAt([]byte{'A', 'B', 'C'}, -5)
	assert.ErrorContains(t, "index is negative", err)

	// There should be no panic
	_ = bytesutil.ClearBit([]byte{'C', 'D', 'E'}, -7)
	res := bytesutil.FromBytes4([]byte{})
	assert.Equal(t, res, uint64(0))
	newRes := bytesutil.FromBytes2([]byte{})
	assert.Equal(t, newRes, uint16(0))
	res = bytesutil.FromBytes8([]byte{})
	assert.Equal(t, res, uint64(0))

	intRes := bytesutil.ToLowInt64([]byte{})
	assert.Equal(t, intRes, int64(0))
}

func TestReverseByteOrder(t *testing.T) {
	input := []byte{0, 1, 2, 3, 4, 5}
	expectedResult := []byte{5, 4, 3, 2, 1, 0}
	output := bytesutil.ReverseByteOrder(input)

	// check that the input is not modified and the output is reversed
	assert.Equal(t, bytes.Equal(input, []byte{0, 1, 2, 3, 4, 5}), true)
	assert.Equal(t, bytes.Equal(expectedResult, output), true)
}

func BenchmarkToBytes32(b *testing.B) {
	x := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}
	for b.Loop() {
		bytesutil.ToBytes32(x)
	}
}

func TestSafeCopyBytes_Copy(t *testing.T) {
	slice := make([]byte, 32)
	slice[0] = 'A'

	copiedSlice := bytesutil.SafeCopyBytes(slice)

	assert.NotEqual(t, fmt.Sprintf("%p", slice), fmt.Sprintf("%p", copiedSlice))
	assert.Equal(t, slice[0], copiedSlice[0])
	slice[1] = 'B'

	assert.NotEqual(t, slice[1], copiedSlice[1])
}

func BenchmarkSafeCopyBytes(b *testing.B) {
	dSlice := make([][]byte, 900000)
	for i := range 900000 {
		slice := make([]byte, 32)
		slice[0] = 'A'
		dSlice[i] = slice
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.Run("Copy Bytes", func(b *testing.B) {
		cSlice := bytesutil.SafeCopy2dBytes(dSlice)
		a := cSlice
		_ = a
	})
}
