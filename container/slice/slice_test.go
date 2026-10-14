package slice_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/theQRL/qrysm/consensus-types/primitives"
	"github.com/theQRL/qrysm/container/slice"
	"github.com/theQRL/qrysm/testing/require"
)

func TestIntersectionUint64(t *testing.T) {
	testCases := []struct {
		setA []uint64
		setB []uint64
		setC []uint64
		out  []uint64
	}{
		{[]uint64{2, 3, 5}, []uint64{3}, []uint64{3}, []uint64{3}},
		{[]uint64{2, 3, 5}, []uint64{3, 5}, []uint64{5}, []uint64{5}},
		{[]uint64{2, 3, 5}, []uint64{3, 5}, []uint64{3, 5}, []uint64{3, 5}},
		{[]uint64{2, 3, 5}, []uint64{5, 3, 2}, []uint64{3, 2, 5}, []uint64{2, 3, 5}},
		{[]uint64{3, 2, 5}, []uint64{5, 3, 2}, []uint64{3, 2, 5}, []uint64{2, 3, 5}},
		{[]uint64{3, 3, 5}, []uint64{5, 3, 2}, []uint64{3, 2, 5}, []uint64{3, 5}},
		{[]uint64{2, 3, 5}, []uint64{2, 3, 5}, []uint64{2, 3, 5}, []uint64{2, 3, 5}},
		{[]uint64{2, 3, 5}, []uint64{}, []uint64{}, []uint64{}},
		{[]uint64{2, 3, 5}, []uint64{2, 3, 5}, []uint64{}, []uint64{}},
		{[]uint64{2, 3}, []uint64{2, 3, 5}, []uint64{5}, []uint64{}},
		{[]uint64{2, 2, 2}, []uint64{2, 2, 2}, []uint64{}, []uint64{}},
		{[]uint64{}, []uint64{2, 3, 5}, []uint64{}, []uint64{}},
		{[]uint64{}, []uint64{}, []uint64{}, []uint64{}},
		{[]uint64{1}, []uint64{1}, []uint64{}, []uint64{}},
		{[]uint64{1, 1, 1}, []uint64{1, 1}, []uint64{1, 2, 3}, []uint64{1}},
	}
	for _, tt := range testCases {
		setA := slices.Clone(tt.setA)
		setB := slices.Clone(tt.setB)
		setC := slices.Clone(tt.setC)
		result := slice.IntersectionUint64(setA, setB, setC)
		slices.Sort(result)
		if !reflect.DeepEqual(result, tt.out) {
			t.Errorf("got %d, want %d", result, tt.out)
		}
		if !reflect.DeepEqual(setA, tt.setA) {
			t.Errorf("slice modified, got %v, want %v", setA, tt.setA)
		}
		if !reflect.DeepEqual(setB, tt.setB) {
			t.Errorf("slice modified, got %v, want %v", setB, tt.setB)
		}
		if !reflect.DeepEqual(setC, tt.setC) {
			t.Errorf("slice modified, got %v, want %v", setC, tt.setC)
		}
	}
}

func TestUnionUint64(t *testing.T) {
	testCases := []struct {
		setA []uint64
		setB []uint64
		out  []uint64
	}{
		{[]uint64{2, 3, 5}, []uint64{4, 6}, []uint64{2, 3, 5, 4, 6}},
		{[]uint64{2, 3, 5}, []uint64{3, 5}, []uint64{2, 3, 5}},
		{[]uint64{2, 3, 5}, []uint64{2, 3, 5}, []uint64{2, 3, 5}},
		{[]uint64{2, 3, 5}, []uint64{}, []uint64{2, 3, 5}},
		{[]uint64{}, []uint64{2, 3, 5}, []uint64{2, 3, 5}},
		{[]uint64{}, []uint64{}, []uint64{}},
		{[]uint64{1}, []uint64{1}, []uint64{1}},
	}
	for _, tt := range testCases {
		result := slice.UnionUint64(tt.setA, tt.setB)
		if !reflect.DeepEqual(result, tt.out) {
			t.Errorf("got %d, want %d", result, tt.out)
		}

	}
	items := [][]uint64{
		{3, 4, 5},
		{6, 7, 8},
		{9, 10, 11},
	}
	variadicResult := slice.UnionUint64(items...)
	want := []uint64{3, 4, 5, 6, 7, 8, 9, 10, 11}
	if !reflect.DeepEqual(want, variadicResult) {
		t.Errorf("Received %v, wanted %v", variadicResult, want)
	}
}

func TestCleanUint64(t *testing.T) {
	testCases := []struct {
		in  []uint64
		out []uint64
	}{
		{[]uint64{2, 4, 4, 6, 6}, []uint64{2, 4, 6}},
		{[]uint64{3, 5, 5}, []uint64{3, 5}},
		{[]uint64{2, 2, 2}, []uint64{2}},
		{[]uint64{1, 4, 5, 9, 9}, []uint64{1, 4, 5, 9}},
		{[]uint64{}, []uint64{}},
		{[]uint64{1}, []uint64{1}},
	}
	for _, tt := range testCases {
		result := slice.SetUint64(tt.in)
		if !reflect.DeepEqual(result, tt.out) {
			t.Errorf("got %d, want %d", result, tt.out)
		}
	}
}

func TestIntersectionByteSlices(t *testing.T) {
	testCases := []struct {
		name   string
		input  [][][]byte
		result [][]byte
	}{
		{
			name: "intersect with empty set",
			input: [][][]byte{
				{
					{1, 2, 3},
					{4, 5},
				},
				{
					{1, 2},
					{4, 5},
				},
				{},
			},
			result: [][]byte{},
		},
		{
			name: "ensure duplicate elements are removed in the resulting set",
			input: [][][]byte{
				{
					{1, 2, 3},
					{4, 5},
					{4, 5},
				},
				{
					{1, 2},
					{4, 5},
					{4, 5},
				},
				{
					{4, 5},
					{4, 5},
				},
			},
			result: [][]byte{{4, 5}},
		},
		{
			name: "ensure no intersection returns an empty set",
			input: [][][]byte{
				{
					{1, 2, 3},
					{4, 5},
				},
				{
					{1, 2},
				},
				{
					{1, 2},
				},
			},
			result: [][]byte{},
		},
		{
			name: "intersection between A and A should return A",
			input: [][][]byte{
				{
					{1, 2},
				},
				{
					{1, 2},
				},
				{
					{1, 2},
				},
			},
			result: [][]byte{{1, 2}},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result := slice.IntersectionByteSlices(tt.input...)
			if !reflect.DeepEqual(result, tt.result) {
				t.Errorf("IntersectionByteSlices(%v)=%v, wanted: %v",
					tt.input, result, tt.result)
			}
		})
	}
	t.Run("properly handle duplicates", func(t *testing.T) {
		input := [][][]byte{
			{{1, 2}, {1, 2}},
			{{1, 2}, {1, 2}},
			{},
		}
		result := slice.IntersectionByteSlices(input...)
		if !reflect.DeepEqual(result, [][]byte{}) {
			t.Errorf("IntersectionByteSlices(%v)=%v, wanted: %v", input, result, [][]byte{})
		}
	})
}

func TestSplitCommaSeparated(t *testing.T) {
	tests := []struct {
		input  []string
		output []string
	}{
		{
			input:  []string{"a,b", "c,d"},
			output: []string{"a", "b", "c", "d"},
		},
		{
			input:  []string{"a", "b,c,d"},
			output: []string{"a", "b", "c", "d"},
		},
		{
			input:  []string{"a", "b", "c"},
			output: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		if result := slice.SplitCommaSeparated(tt.input); !reflect.DeepEqual(result, tt.output) {
			t.Errorf("SplitCommaSeparated(%v) = %v; wanted %v", tt.input, result, tt.output)
		}
	}
}

func TestSplitOffset_OK(t *testing.T) {
	testCases := []struct {
		listSize uint64
		chunks   uint64
		index    uint64
		offset   uint64
	}{
		{30, 3, 2, 20},
		{1000, 10, 60, 6000},
		{2482, 10, 70, 17374},
		{323, 98, 56, 184},
		{273, 8, 6, 204},
		{3274, 98, 256, 8552},
		{23, 3, 2, 15},
		{23, 3, 9, 69},
	}
	for _, tt := range testCases {
		result := slice.SplitOffset(tt.listSize, tt.chunks, tt.index)
		if result != tt.offset {
			t.Errorf("got %d, want %d", result, tt.offset)
		}

	}
}

func TestIntersectionSlot(t *testing.T) {
	testCases := []struct {
		setA []primitives.Slot
		setB []primitives.Slot
		setC []primitives.Slot
		out  []primitives.Slot
	}{
		{[]primitives.Slot{2, 3, 5}, []primitives.Slot{3}, []primitives.Slot{3}, []primitives.Slot{3}},
		{[]primitives.Slot{2, 3, 5}, []primitives.Slot{3, 5}, []primitives.Slot{5}, []primitives.Slot{5}},
		{[]primitives.Slot{2, 3, 5}, []primitives.Slot{3, 5}, []primitives.Slot{3, 5}, []primitives.Slot{3, 5}},
		{[]primitives.Slot{2, 3, 5}, []primitives.Slot{5, 3, 2}, []primitives.Slot{3, 2, 5}, []primitives.Slot{2, 3, 5}},
		{[]primitives.Slot{3, 2, 5}, []primitives.Slot{5, 3, 2}, []primitives.Slot{3, 2, 5}, []primitives.Slot{2, 3, 5}},
		{[]primitives.Slot{3, 3, 5}, []primitives.Slot{5, 3, 2}, []primitives.Slot{3, 2, 5}, []primitives.Slot{3, 5}},
		{[]primitives.Slot{2, 3, 5}, []primitives.Slot{2, 3, 5}, []primitives.Slot{2, 3, 5}, []primitives.Slot{2, 3, 5}},
		{[]primitives.Slot{2, 3, 5}, []primitives.Slot{}, []primitives.Slot{}, []primitives.Slot{}},
		{[]primitives.Slot{2, 3, 5}, []primitives.Slot{2, 3, 5}, []primitives.Slot{}, []primitives.Slot{}},
		{[]primitives.Slot{2, 3}, []primitives.Slot{2, 3, 5}, []primitives.Slot{5}, []primitives.Slot{}},
		{[]primitives.Slot{2, 2, 2}, []primitives.Slot{2, 2, 2}, []primitives.Slot{}, []primitives.Slot{}},
		{[]primitives.Slot{}, []primitives.Slot{2, 3, 5}, []primitives.Slot{}, []primitives.Slot{}},
		{[]primitives.Slot{}, []primitives.Slot{}, []primitives.Slot{}, []primitives.Slot{}},
		{[]primitives.Slot{1}, []primitives.Slot{1}, []primitives.Slot{}, []primitives.Slot{}},
		{[]primitives.Slot{1, 1, 1}, []primitives.Slot{1, 1}, []primitives.Slot{1, 2, 3}, []primitives.Slot{1}},
	}
	for _, tt := range testCases {
		setA := slices.Clone(tt.setA)
		setB := slices.Clone(tt.setB)
		setC := slices.Clone(tt.setC)
		result := slice.IntersectionSlot(setA, setB, setC)
		slices.Sort(result)
		if !reflect.DeepEqual(result, tt.out) {
			t.Errorf("got %d, want %d", result, tt.out)
		}
		if !reflect.DeepEqual(setA, tt.setA) {
			t.Errorf("slice modified, got %v, want %v", setA, tt.setA)
		}
		if !reflect.DeepEqual(setB, tt.setB) {
			t.Errorf("slice modified, got %v, want %v", setB, tt.setB)
		}
		if !reflect.DeepEqual(setC, tt.setC) {
			t.Errorf("slice modified, got %v, want %v", setC, tt.setC)
		}
	}
}

func TestNotSlot(t *testing.T) {
	testCases := []struct {
		setA []primitives.Slot
		setB []primitives.Slot
		out  []primitives.Slot
	}{
		{[]primitives.Slot{4, 6}, []primitives.Slot{2, 3, 5, 4, 6}, []primitives.Slot{2, 3, 5}},
		{[]primitives.Slot{3, 5}, []primitives.Slot{2, 3, 5}, []primitives.Slot{2}},
		{[]primitives.Slot{2, 3, 5}, []primitives.Slot{2, 3, 5}, []primitives.Slot{}},
		{[]primitives.Slot{2}, []primitives.Slot{2, 3, 5}, []primitives.Slot{3, 5}},
		{[]primitives.Slot{}, []primitives.Slot{2, 3, 5}, []primitives.Slot{2, 3, 5}},
		{[]primitives.Slot{}, []primitives.Slot{}, []primitives.Slot{}},
		{[]primitives.Slot{1}, []primitives.Slot{1}, []primitives.Slot{}},
	}
	for _, tt := range testCases {
		result := slice.NotSlot(tt.setA, tt.setB)
		if !reflect.DeepEqual(result, tt.out) {
			t.Errorf("got %d, want %d", result, tt.out)
		}
	}
}

func TestIsInSlots(t *testing.T) {
	testCases := []struct {
		a      primitives.Slot
		b      []primitives.Slot
		result bool
	}{
		{0, []primitives.Slot{}, false},
		{0, []primitives.Slot{0}, true},
		{4, []primitives.Slot{2, 3, 5, 4, 6}, true},
		{100, []primitives.Slot{2, 3, 5, 4, 6}, false},
	}
	for _, tt := range testCases {
		result := slice.IsInSlots(tt.a, tt.b)
		if result != tt.result {
			t.Errorf("IsIn(%d, %v)=%v, wanted: %v",
				tt.a, tt.b, result, tt.result)
		}
	}
}

func TestUnique(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		result := slice.Unique[string]([]string{"a", "b", "a"})
		require.DeepEqual(t, []string{"a", "b"}, result)
	})
	t.Run("uint64", func(t *testing.T) {
		result := slice.Unique[uint64]([]uint64{1, 2, 1})
		require.DeepEqual(t, []uint64{1, 2}, result)
	})
}

func TestReverse(t *testing.T) {
	tests := []struct {
		value [][32]byte
		want  [][32]byte
	}{
		{[][32]byte{}, [][32]byte{}},
		{[][32]byte{{'A'}, {'B'}, {'C'}, {'D'}},
			[][32]byte{{'D'}, {'C'}, {'B'}, {'A'}}},
		{[][32]byte{{1}, {2}, {3}, {4}},
			[][32]byte{{4}, {3}, {2}, {1}}},
	}
	for _, tt := range tests {
		b := slice.Reverse(tt.value)
		require.DeepEqual(t, tt.want, b)
	}
}
