package share_test

import (
	"fmt"
	"testing"

	"github.com/celestiaorg/go-square/internal/test"
	"github.com/celestiaorg/go-square/share"
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
					_, err := share.SplitBlobs(blobs...)
					if err != nil {
						b.Fatal("Failed to split blob into shares:", err)
					}
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
				s, err := share.SplitBlobs(blobs...)
				if err != nil {
					b.Fatal("Failed to split blob into shares:", err)
				}

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
