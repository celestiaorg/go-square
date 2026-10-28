package square_test

import (
	"fmt"
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/internal/test"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

func BenchmarkSquareConstruct(b *testing.B) {
	for _, txCount := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("txCount=%d", txCount), func(b *testing.B) {
			b.ReportAllocs()
			txs := test.ClassifyNormalTxs(generateOrderedTxs(b, txCount/2, txCount/2, 1, 1024))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := square.Construct(txs, defaultMaxSquareSize, defaultSubtreeRootThreshold)
				require.NoError(b, err)
			}
		})
	}
}

// BenchmarkConstructPreDecodedBlobTx measures what ClassifiedTx.BlobTx saves:
// Construct unmarshals every blob tx twice, once to validate the ordering and
// once to append it, without copying the blobs. The "decoded" case hands it
// the decode the caller already has.
func BenchmarkConstructPreDecodedBlobTx(b *testing.B) {
	const (
		blobSize = 1024 * 1024
		txCount  = 4
	)

	rawTxs, err := test.GenerateBlobTxs(txCount, 1, blobSize)
	require.NoError(b, err)

	raw := make([]square.ClassifiedTx, len(rawTxs))
	decoded := make([]square.ClassifiedTx, len(rawTxs))
	for i, rawTx := range rawTxs {
		raw[i] = square.NewClassifiedTx(rawTx)
		blobTx, isBlobTx, err := tx.UnmarshalBlobTx(rawTx)
		require.NoError(b, err)
		require.True(b, isBlobTx)
		decoded[i], err = square.NewClassifiedBlobTx(rawTx, blobTx)
		require.NoError(b, err)
	}

	// The two inputs must produce the same square; measuring otherwise would
	// be measuring different work.
	fromRaw, err := square.Construct(raw, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.NoError(b, err)
	fromDecoded, err := square.Construct(decoded, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.NoError(b, err)
	require.Equal(b, fromRaw, fromDecoded)

	for _, bc := range []struct {
		name string
		txs  []square.ClassifiedTx
	}{
		{name: "raw", txs: raw},
		{name: "decoded", txs: decoded},
	} {
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := square.Construct(bc.txs, defaultMaxSquareSize, defaultSubtreeRootThreshold)
				require.NoError(b, err)
			}
		})
	}
}
