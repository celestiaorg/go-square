package test

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/cosmos/btcutil/bech32"
)

var DefaultTestNamespace = share.MustNewV0Namespace([]byte("test"))

func GenerateTxs(minSize, maxSize, numTxs int) ([][]byte, error) {
	txs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		var err error
		txs[i], err = GenerateRandomTx(minSize, maxSize)
		if err != nil {
			return nil, err
		}
	}
	return txs, nil
}

func GenerateRandomTx(minSize, maxSize int) ([]byte, error) {
	size := minSize
	if maxSize > minSize {
		size = rand.Intn(maxSize-minSize) + minSize
	}
	return RandomBytes(size)
}

func RandomBytes(size int) ([]byte, error) {
	b := make([]byte, size)
	_, err := crand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to read random bytes: %w", err)
	}
	return b, nil
}

func GenerateBlobTxWithNamespace(namespaces []share.Namespace, blobSizes []int, version uint8) ([]byte, error) {
	blobs := make([]*share.Blob, len(blobSizes))
	if len(namespaces) != len(blobSizes) {
		return nil, fmt.Errorf("number of namespaces should match number of blob sizes")
	}
	var signer []byte
	if version == share.ShareVersionOne {
		var err error
		signer, err = RandomBytes(share.SignerSize)
		if err != nil {
			return nil, err
		}
	}
	for i, size := range blobSizes {
		data, err := RandomBytes(size)
		if err != nil {
			return nil, err
		}
		blobs[i], err = share.NewBlob(namespaces[i], data, version, signer)
		if err != nil {
			return nil, err
		}
	}
	mockPFB, err := MockPFB(toUint32(blobSizes))
	if err != nil {
		return nil, err
	}
	blobTx, err := tx.MarshalBlobTx(mockPFB, blobs...)
	if err != nil {
		return nil, err
	}
	return blobTx, nil
}

func GenerateBlobTx(blobSizes []int) ([]byte, error) {
	return GenerateBlobTxWithNamespace(Repeat(DefaultTestNamespace, len(blobSizes)), blobSizes, share.DefaultShareVersion)
}

func GenerateBlobTxs(numTxs, blobsPerPfb, blobSize int) ([][]byte, error) {
	blobSizes := make([]int, blobsPerPfb)
	for i := range blobSizes {
		blobSizes[i] = blobSize
	}
	txs := make([][]byte, numTxs)
	for i := 0; i < numTxs; i++ {
		var err error
		txs[i], err = GenerateBlobTx(blobSizes)
		if err != nil {
			return nil, err
		}
	}
	return txs, nil
}

func GenerateBlobs(blobSizes ...int) ([]*share.Blob, error) {
	blobs := make([]*share.Blob, len(blobSizes))
	for i, size := range blobSizes {
		data, err := RandomBytes(size)
		if err != nil {
			return nil, err
		}
		blobs[i], err = share.NewBlob(share.RandomBlobNamespace(), data, share.ShareVersionZero, nil)
		if err != nil {
			return nil, err
		}
	}
	return blobs, nil
}

const mockPFBExtraBytes = 329

func MockPFB(blobSizes []uint32) ([]byte, error) {
	if len(blobSizes) == 0 {
		return nil, fmt.Errorf("must have at least one blob")
	}
	tx := make([]byte, len(blobSizes)*4)
	for i, size := range blobSizes {
		binary.BigEndian.PutUint32(tx[i*4:], uint32(size))
	}

	randomPrefix, err := RandomBytes(mockPFBExtraBytes)
	if err != nil {
		return nil, err
	}
	return append(randomPrefix, tx...), nil
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

// BuildFibreClassifiedTx returns a ClassifiedTx for a pay-for-fibre
// transaction. The transaction bytes are opaque to go-square: classification is
// the caller's job, so tests need not encode a Cosmos SDK transaction to
// exercise the fibre path.
func BuildFibreClassifiedTx(txBytes []byte, ns share.Namespace, commitment, signer []byte, blobVersion uint32) (square.ClassifiedTx, error) {
	systemBlob, err := share.NewV2Blob(ns, blobVersion, commitment, signer)
	if err != nil {
		return square.ClassifiedTx{}, fmt.Errorf("creating system blob: %w", err)
	}
	return square.NewClassifiedFibreTx(&tx.FibreTx{Tx: txBytes, SystemBlob: systemBlob})
}

// ClassifyNormalTxs wraps raw transactions as ClassifiedTx values that are not
// pay-for-fibre transactions. Blob txs may be passed here: go-square recognises
// its own BlobTx format without help.
func ClassifyNormalTxs(txs [][]byte) []square.ClassifiedTx {
	classified := make([]square.ClassifiedTx, len(txs))
	for i, txBytes := range txs {
		classified[i] = square.NewClassifiedTx(txBytes)
	}
	return classified
}

// EncodeBech32 encodes raw bytes as a bech32 string with the given
// human-readable prefix (e.g. "celestia").
func EncodeBech32(hrp string, data []byte) (string, error) {
	encoded, err := bech32.EncodeFromBase256(hrp, data)
	if err != nil {
		return "", fmt.Errorf("failed to encode bech32: %w", err)
	}
	return encoded, nil
}
