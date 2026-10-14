// Package bytesutil defines helper methods for converting integers to byte slices.
package bytesutil

// FromBytes2592 is a convenience method for converting a fixed-size byte array
// to a byte slice.
func FromBytes2592(x [2592]byte) []byte {
	return x[:]
}

// FromBytes2592Array is a convenience method for converting an array of
// fixed-size byte arrays to an array of byte slices.
func FromBytes2592Array(x [][2592]byte) [][]byte {
	y := make([][]byte, len(x))
	for i := range x {
		y[i] = x[i][:]
	}
	return y
}

// Trunc truncates the byte slices to 6 bytes.
func Trunc(x []byte) []byte {
	if len(x) > 6 {
		return x[:6]
	}
	return x
}

// SafeCopyBytes will copy and return a non-nil byte slice, otherwise it returns nil.
func SafeCopyBytes(cp []byte) []byte {
	if cp != nil {
		if len(cp) == 32 {
			copied := [32]byte(cp)
			return copied[:]
		}
		copied := make([]byte, len(cp))
		copy(copied, cp)
		return copied
	}
	return nil
}

// SafeCopy2dBytes will copy and return a non-nil 2d byte slice, otherwise it returns nil.
func SafeCopy2dBytes(ary [][]byte) [][]byte {
	if ary != nil {
		copied := make([][]byte, len(ary))
		for i, a := range ary {
			copied[i] = SafeCopyBytes(a)
		}
		return copied
	}
	return nil
}

// PadTo pads a byte slice to the given size. If the byte slice is larger than the given size, the
// original slice is returned.
func PadTo(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	return append(b, make([]byte, size-len(b))...)
}

// ReverseByteOrder Switch the endianness of a byte slice by reversing its order.
// this function does not modify the actual input bytes.
func ReverseByteOrder(input []byte) []byte {
	b := make([]byte, len(input))
	copy(b, input)
	for i := 0; i < len(b)/2; i++ {
		b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
	}
	return b
}
