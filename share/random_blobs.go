package share

import (
	"bytes"
	crand "crypto/rand"
	"math/rand"
)

// GenerateV0Blobs is a test utility producing v0 share formatted blobs with the
// requested size and namespaces.
func GenerateV0Blobs(sizes []int, sameNamespace bool) ([]*Blob, error) {
	blobs := make([]*Blob, 0, len(sizes))
	for _, size := range sizes {
		size := rawTxSize(FirstSparseShareContentSize * size)
		blob := generateRandomBlob(size)
		if !sameNamespace {
			ns := RandomBlobNamespace()
			var err error
			blob, err = NewV0Blob(ns, blob.Data())
			if err != nil {
				return nil, err
			}
		}

		blobs = append(blobs, blob)
	}
	return blobs, nil
}

func generateRandomBlobWithNamespace(namespace Namespace, size int) *Blob {
	data := make([]byte, size)
	_, err := crand.Read(data)
	if err != nil {
		panic(err)
	}
	blob, err := NewV0Blob(namespace, data)
	if err != nil {
		panic(err)
	}
	return blob
}

func generateRandomBlob(dataSize int) *Blob {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{0x1}, NamespaceVersionZeroIDSize))
	return generateRandomBlobWithNamespace(ns, dataSize)
}

func generateRandomlySizedBlobs(count, maxBlobSize int) []*Blob {
	blobs := make([]*Blob, count)
	for i := 0; i < count; i++ {
		blobs[i] = generateRandomBlob(rand.Intn(maxBlobSize-1) + 1)
		if len(blobs[i].Data()) == 0 {
			i--
		}
	}

	// this is just to let us use assert.Equal
	if count == 0 {
		blobs = nil
	}

	SortBlobs(blobs)
	return blobs
}
