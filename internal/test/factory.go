package test

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/namespace"
	"github.com/celestiaorg/go-square/shares"
)

var DefaultTestNamespace = namespace.MustNewV0([]byte("test"))

func GenerateTxs(minSize, maxSize, numTxs int) [][]byte {
	txs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		txs[i] = GenerateRandomTx(minSize, maxSize)
	}
	return txs
}

func GenerateRandomTx(minSize, maxSize int) []byte {
	size := minSize
	if maxSize > minSize {
		size = rand.Intn(maxSize-minSize) + minSize
	}
	return RandomBytes(size)
}

func RandomBytes(size int) []byte {
	b := make([]byte, size)
	_, err := crand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func GenerateBlobTxWithNamespace(namespaces []namespace.Namespace, blobSizes []int) []byte {
	blobs := make([]*blob.Blob, len(blobSizes))
	if len(namespaces) != len(blobSizes) {
		panic("number of namespaces should match number of blob sizes")
	}
	for i, size := range blobSizes {
		blobs[i] = blob.New(namespaces[i], RandomBytes(size), shares.DefaultShareVersion)
	}
	blobTx, err := blob.MarshalBlobTx(MockPFB(toUint32(blobSizes)), blobs...)
	if err != nil {
		panic(err)
	}
	return blobTx
}

func GenerateBlobTx(blobSizes []int) []byte {
	return GenerateBlobTxWithNamespace(Repeat(DefaultTestNamespace, len(blobSizes)), blobSizes)
}

func GenerateBlobTxs(numTxs, blobsPerPfb, blobSize int) [][]byte {
	blobSizes := make([]int, blobsPerPfb)
	for i := range blobSizes {
		blobSizes[i] = blobSize
	}
	txs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		txs[i] = GenerateBlobTx(blobSizes)
	}
	return txs
}

const mockPFBExtraBytes = 329

func MockPFB(blobSizes []uint32) []byte {
	if len(blobSizes) == 0 {
		panic("must have at least one blob")
	}
	tx := make([]byte, len(blobSizes)*4)
	for i, size := range blobSizes {
		binary.BigEndian.PutUint32(tx[i*4:], uint32(size))
	}

	return append(RandomBytes(mockPFBExtraBytes), tx...)
}

func DecodeMockPFB(pfb []byte) ([]uint32, error) {
	if len(pfb) < mockPFBExtraBytes+4 {
		return nil, fmt.Errorf("must have a length of at least %d bytes, got %d", mockPFBExtraBytes+4, len(pfb))
	}
	pfb = pfb[mockPFBExtraBytes:]
	blobSizes := make([]uint32, len(pfb)/4)
	for i := 0; i < len(blobSizes); i++ {
		blobSizes[i] = binary.BigEndian.Uint32(pfb[i*4 : (i+1)*4])
	}
	return blobSizes, nil
}

func toUint32(arr []int) []uint32 {
	output := make([]uint32, len(arr))
	for i, value := range arr {
		output[i] = uint32(value)
	}
	return output
}

func Repeat[T any](s T, count int) []T {
	ss := make([]T, count)
	for i := 0; i < count; i++ {
		ss[i] = s
	}
	return ss
}
