package merkle

import (
	"crypto/sha256"
	shash "hash"
	"sync"
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
	return hash(leafPrefix, leaf)
}

// returns sha256(0x01 || left || right)
func innerHash(left, right []byte) []byte {
	return hash(innerPrefix, left, right)
}

var sha256Pool = &sync.Pool{New: func() any { return sha256.New() }}

func hash(slices ...[]byte) []byte {
	h := sha256Pool.Get().(shash.Hash)
	defer func() {
		h.Reset()
		sha256Pool.Put(h)
	}()

	for _, slice := range slices {
		h.Write(slice)
	}

	return h.Sum(nil)
}
