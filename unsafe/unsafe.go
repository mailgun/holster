package unsafe

import (
	"unsafe"
)

// BytesToString converts a byte slice to a string without allocation. The
// returned string reuses the slice byte array. Since Go strings are immutable,
// the bytes passed to BytesToString must not be modified afterwards.
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToBytes converts a string to a byte slice without allocation. The
// returned byte slice reuses the underlying string byte array. Since Go
// strings are immutable, the bytes returned by StringToBytes must not be
// modified.
func StringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
