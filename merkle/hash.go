package merkle

import (
	"crypto/sha256"
)

// TODO: make these have a large predefined capacity
var (
	leafPrefix  = []byte{0}
	innerPrefix = []byte{1}
)

// returns empty sha256 hash
func emptyHash() []byte {
	return hash([]byte{})
}

// returns sha256(0x00 || leaf)
func leafHash(leaf []byte) []byte {
	return hash(append(leafPrefix, leaf...))
}

// returns sha256(0x01 || left || right)
func innerHash(left []byte, right []byte) []byte {
	return hash(append(innerPrefix, append(left, right...)...))
}

func hash(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}
