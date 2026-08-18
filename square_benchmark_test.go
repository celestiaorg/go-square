package square_test

import (
	"fmt"
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/internal/test"
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
