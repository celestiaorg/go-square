package share_test

import (
	"fmt"
	"testing"

	"github.com/celestiaorg/go-square/v2/internal/test"
	"github.com/celestiaorg/go-square/v2/share"
)

func BenchmarkBlobsToShares(b *testing.B) {
	sizes := []int{256, 256 * 8, 256 * 64}
	numBlobs := []int{1, 8, 64}
	for _, size := range sizes {
		for _, numBlobs := range numBlobs {
			b.Run(fmt.Sprintf("ShareEncoding%dBlobs%dBytes", numBlobs, size), func(b *testing.B) {
				b.ReportAllocs()
				blobs := test.GenerateBlobs(test.Repeat(size, numBlobs)...)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Convert blob to shares
					writer := share.NewSparseShareSplitter()
					for _, blob := range blobs {
						if err := writer.Write(blob); err != nil {
							b.Fatal("Failed to write blob into shares:", err)
						}
					}
					_ = writer.Export()
				}
			})
		}
	}
}

func BenchmarkSharesToBlobs(b *testing.B) {
	sizes := []int{256, 256 * 8, 256 * 64}
	numBlobs := []int{1, 8, 64}
	for _, size := range sizes {
		for _, numBlobs := range numBlobs {
			b.Run(fmt.Sprintf("ShareDecoding%dBlobs%dBytes", numBlobs, size), func(b *testing.B) {
				b.ReportAllocs()
				blobs := test.GenerateBlobs(test.Repeat(size, numBlobs)...)
				writer := share.NewSparseShareSplitter()
				for _, blob := range blobs {
					if err := writer.Write(blob); err != nil {
						b.Fatal("Failed to write blob into shares:", err)
					}
				}
				s := writer.Export()

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					// Convert shares back to blob
					_, err := share.ParseBlobs(s)
					if err != nil {
						b.Fatal("Failed to reconstruct blob from shares:", err)
					}
				}
			})
		}
	}
}
