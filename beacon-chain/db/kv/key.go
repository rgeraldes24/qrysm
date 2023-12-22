package kv

import "bytes"

// NOTE(rgeraldes24) - not used
/*
func hasCapellaKey(enc []byte) bool {
	if len(capellaKey) >= len(enc) {
		return false
	}
	return bytes.Equal(enc[:len(capellaKey)], capellaKey)
}
*/

func hasCapellaBlindKey(enc []byte) bool {
	if len(capellaBlindKey) >= len(enc) {
		return false
	}
	return bytes.Equal(enc[:len(capellaBlindKey)], capellaBlindKey)
}
