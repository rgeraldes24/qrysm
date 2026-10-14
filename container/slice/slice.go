package slice

import (
	"slices"
	"strings"

	"github.com/theQRL/qrysm/consensus-types/primitives"
)

// IntersectionUint64 of any number of uint64 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func IntersectionUint64(s ...[]uint64) []uint64 {
	if len(s) == 0 {
		return []uint64{}
	}
	if len(s) == 1 {
		return s[0]
	}
	intersect := make([]uint64, 0)
	m := make(map[uint64]int)
	for _, k := range s[0] {
		m[k] = 1
	}
	for i, num := 1, len(s); i < num; i++ {
		for _, k := range s[i] {
			// Increment and check only if item is present in both, and no increment has happened yet.
			if _, found := m[k]; found && i == m[k] {
				m[k]++
				if m[k] == num {
					intersect = append(intersect, k)
				}
			}
		}
	}
	return intersect
}

// UnionUint64 of any number of uint64 slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func UnionUint64(s ...[]uint64) []uint64 {
	if len(s) == 0 {
		return []uint64{}
	}
	if len(s) == 1 {
		return s[0]
	}
	set := s[0]
	m := make(map[uint64]bool)
	for i := 1; i < len(s); i++ {
		a := s[i-1]
		b := s[i]
		for j := range a {
			m[a[j]] = true
		}
		for j := range b {
			if _, found := m[b[j]]; !found {
				set = append(set, b[j])
			}
		}
	}
	return set
}

// SetUint64 returns a slice with only unique
// values from the provided list of indices.
func SetUint64(a []uint64) []uint64 {
	// Remove duplicates indices.
	intMap := make(map[uint64]bool, len(a))
	cleanedIndices := make([]uint64, 0, len(a))
	for _, idx := range a {
		if intMap[idx] {
			continue
		}
		intMap[idx] = true
		cleanedIndices = append(cleanedIndices, idx)
	}
	return cleanedIndices
}

// IntersectionByteSlices returns the common elements between sets of byte slices.
func IntersectionByteSlices(s ...[][]byte) [][]byte {
	if len(s) == 0 {
		return [][]byte{}
	}
	if len(s) == 1 {
		return s[0]
	}
	inter := make([][]byte, 0)
	m := make(map[string]int)
	for _, k := range s[0] {
		m[string(k)] = 1
	}
	for i, num := 1, len(s); i < num; i++ {
		for _, k := range s[i] {
			if _, found := m[string(k)]; found && i == m[string(k)] {
				m[string(k)]++
				if m[string(k)] == num {
					inter = append(inter, k)
				}
			}
		}
	}
	return inter
}

// SplitCommaSeparated values from the list. Example: []string{"a,b", "c,d"} becomes []string{"a", "b", "c", "d"}.
func SplitCommaSeparated(arr []string) []string {
	var result []string
	for _, val := range arr {
		result = append(result, strings.Split(val, ",")...)
	}
	return result
}

// SplitOffset returns the start index of a given list splits into chunks,
// it computes (listsize * index) / chunks.
//
// Spec pseudocode definition:
// def get_split_offset(list_size: int, chunks: int, index: int) -> int:
//
//	"""
//	Returns a value such that for a list L, chunk count k and index i,
//	split(L, k)[i] == L[get_split_offset(len(L), k, i): get_split_offset(len(L), k, i+1)]
//	"""
//	return (list_size * index) // chunks
func SplitOffset(listSize, chunks, index uint64) uint64 {
	return (listSize * index) / chunks
}

// IntersectionSlot of any number of types.Slot slices with time
// complexity of approximately O(n) leveraging a map to
// check for element existence off by a constant factor
// of underlying map efficiency.
func IntersectionSlot(s ...[]primitives.Slot) []primitives.Slot {
	if len(s) == 0 {
		return []primitives.Slot{}
	}
	if len(s) == 1 {
		return s[0]
	}
	intersect := make([]primitives.Slot, 0)
	m := make(map[primitives.Slot]int)
	for _, k := range s[0] {
		m[k] = 1
	}
	for i, num := 1, len(s); i < num; i++ {
		for _, k := range s[i] {
			// Increment and check only if item is present in both, and no increment has happened yet.
			if _, found := m[k]; found && i == m[k] {
				m[k]++
				if m[k] == num {
					intersect = append(intersect, k)
				}
			}
		}
	}
	return intersect
}

// NotSlot returns the types.Slot in slice b that are
// not in slice a with time complexity of approximately
// O(n) leveraging a map to check for element existence
// off by a constant factor of underlying map efficiency.
func NotSlot(a, b []primitives.Slot) []primitives.Slot {
	set := make([]primitives.Slot, 0)
	m := make(map[primitives.Slot]bool)

	for i := range a {
		m[a[i]] = true
	}
	for i := range b {
		if _, found := m[b[i]]; !found {
			set = append(set, b[i])
		}
	}
	return set
}

// IsInSlots returns true if a is in b and False otherwise.
func IsInSlots(a primitives.Slot, b []primitives.Slot) bool {
	return slices.Contains(b, a)
}

// Unique returns an array with duplicates filtered based on the type given
func Unique[T comparable](a []T) []T {
	if len(a) <= 1 {
		return a
	}
	found := map[T]bool{}
	result := make([]T, len(a))
	end := 0
	for i := range a {
		if !found[a[i]] {
			found[a[i]] = true
			result[end] = a[i]
			end += 1
		}
	}
	return result[:end]
}

// Reverse reverses any slice in place
// Taken from https://github.com/faiface/generics/blob/8cf65f0b43803410724d8c671cb4d328543ba07d/examples/sliceutils/sliceutils.go
func Reverse[E any](s []E) []E {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
