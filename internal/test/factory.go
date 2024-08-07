package test

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	"github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/go-square/v2/tx"
)

var DefaultTestNamespace = share.MustNewV0Namespace([]byte("test"))

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

func GenerateBlobTxWithNamespace(namespaces []share.Namespace, blobSizes []int, version uint8) []byte {
	blobs := make([]*share.Blob, len(blobSizes))
	if len(namespaces) != len(blobSizes) {
		panic("number of namespaces should match number of blob sizes")
	}
	var err error
	var signer []byte
	if version == share.ShareVersionOne {
		signer = RandomBytes(share.SignerSize)
	}
	for i, size := range blobSizes {
		blobs[i], err = share.NewBlob(namespaces[i], RandomBytes(size), version, signer)
		if err != nil {
			panic(err)
		}
	}
	blobTx, err := tx.MarshalBlobTx(MockPFB(toUint32(blobSizes)), blobs...)
	if err != nil {
		panic(err)
	}
	return blobTx
}

func GenerateBlobTx(blobSizes []int) []byte {
	return GenerateBlobTxWithNamespace(Repeat(DefaultTestNamespace, len(blobSizes)), blobSizes, share.DefaultShareVersion)
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

func GenerateBlobs(blobSizes ...int) []*share.Blob {
	blobs := make([]*share.Blob, len(blobSizes))
	var err error
	for i, size := range blobSizes {
		blobs[i], err = share.NewBlob(share.RandomBlobNamespace(), RandomBytes(size), share.ShareVersionZero, nil)
		if err != nil {
			panic(err)
		}
	}
	return blobs
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

// DelimLen calculates the length of the delimiter for a given unit size
func DelimLen(size uint64) int {
	lenBuf := make([]byte, binary.MaxVarintLen64)
	return binary.PutUvarint(lenBuf, size)
}
