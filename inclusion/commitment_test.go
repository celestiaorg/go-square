package inclusion_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"runtime"
	"testing"

	"github.com/celestiaorg/go-square/v3/inclusion"
	"github.com/celestiaorg/go-square/v3/internal/test"
	"github.com/celestiaorg/go-square/v3/share"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerkleMountainRangeSizes(t *testing.T) {
	type test struct {
		totalSize  uint64
		squareSize uint64
		expected   []uint64
	}
	tests := []test{
		{
			totalSize:  11,
			squareSize: 4,
			expected:   []uint64{4, 4, 2, 1},
		},
		{
			totalSize:  2,
			squareSize: 64,
			expected:   []uint64{2},
		},
		{
			totalSize:  64,
			squareSize: 8,
			expected:   []uint64{8, 8, 8, 8, 8, 8, 8, 8},
		},
		// Height
		// 3              x                               x
		//              /    \                         /    \
		//             /      \                       /      \
		//            /        \                     /        \
		//           /          \                   /          \
		// 2        x            x                 x            x
		//        /   \        /   \             /   \        /   \
		// 1     x     x      x     x           x     x      x     x         x
		//      / \   / \    / \   / \         / \   / \    / \   / \      /   \
		// 0   0   1 2   3  4   5 6   7       8   9 10  11 12 13 14  15   16   17    18
		{
			totalSize:  19,
			squareSize: 8,
			expected:   []uint64{8, 8, 2, 1},
		},
	}
	for _, tt := range tests {
		res, err := inclusion.MerkleMountainRangeSizes(tt.totalSize, tt.squareSize)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, res)
	}
}

// TestCreateCommitment will fail if a change is made to share encoding or how
// the commitment is calculated. If this is the case, the expected commitment
// bytes will need to be updated.
func TestGenerateSubtreeRootsEquivalence(t *testing.T) {
	blobSizes := []int{
		share.AvailableBytesFromSparseShares(2),
		share.AvailableBytesFromSparseShares(16),
		share.AvailableBytesFromSparseShares(64),
	}

	for _, size := range blobSizes {
		blobs := test.GenerateBlobs(size)
		require.Len(t, blobs, 1)
		blob := blobs[0]

		original, err := inclusion.GenerateSubtreeRoots(blob, defaultSubtreeRootThreshold)
		require.NoError(t, err)

		optimized, err := inclusion.GenerateSubtreeRootsReusedNMT(blob, defaultSubtreeRootThreshold)
		require.NoError(t, err)

		parallel, err := inclusion.GenerateSubtreeRootsParallel(blob, defaultSubtreeRootThreshold)
		require.NoError(t, err)

		assert.Equal(t, original, optimized, "Optimized results should be identical for blob size %d", size)
		assert.Equal(t, original, parallel, "Parallel results should be identical for blob size %d", size)
	}
}

func TestCreateCommitment(t *testing.T) {
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{0x1}, share.NamespaceVersionZeroIDSize))

	type test struct {
		name         string
		namespace    share.Namespace
		blob         []byte
		expected     []byte
		expectErr    bool
		shareVersion uint8
		signer       []byte
	}
	tests := []test{
		{
			name:         "blob of 2 shares succeeds",
			namespace:    ns1,
			blob:         bytes.Repeat([]byte{0xFF}, share.AvailableBytesFromSparseShares(2)),
			expected:     []byte{0x31, 0xf5, 0x15, 0x6d, 0x5d, 0xb9, 0xa7, 0xf5, 0xb4, 0x3b, 0x29, 0x7a, 0x14, 0xc0, 0x70, 0xc2, 0xcc, 0x4e, 0xf3, 0xd6, 0x9d, 0x87, 0xed, 0x8, 0xad, 0xdd, 0x21, 0x6d, 0x9b, 0x9f, 0xa1, 0x18},
			shareVersion: share.ShareVersionZero,
		},
		{
			name:         "blob of one share with signer succeeds",
			namespace:    ns1,
			blob:         bytes.Repeat([]byte{0xFF}, share.AvailableBytesFromSparseShares(2)-share.SignerSize),
			expected:     []byte{0x88, 0x3c, 0x74, 0x6, 0x4e, 0x8e, 0x26, 0x27, 0xad, 0x58, 0x8, 0x38, 0x9f, 0x1f, 0x19, 0x24, 0x19, 0x4c, 0x1a, 0xe2, 0x3c, 0x7d, 0xf9, 0x62, 0xc8, 0xd5, 0x6d, 0xf0, 0x62, 0xa9, 0x2b, 0x2b},
			shareVersion: share.ShareVersionOne,
			signer:       bytes.Repeat([]byte{1}, share.SignerSize),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blob, err := share.NewBlob(tt.namespace, tt.blob, tt.shareVersion, tt.signer)
			require.NoError(t, err)
			res, err := inclusion.CreateCommitment(blob, twoLeafMerkleRoot, defaultSubtreeRootThreshold)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func twoLeafMerkleRoot(data [][]byte) []byte {
	if len(data) != 2 {
		panic("data must have exactly 2 elements")
	}
	h1 := sha256.Sum256(data[0])
	h2 := sha256.Sum256(data[1])
	sum := sha256.Sum256(append(h1[:], h2[:]...))
	return sum[:]
}

func simpleMerkleRoot(data [][]byte) []byte {
	// Return a dummy 32-byte value to avoid affecting benchmark
	return make([]byte, 32)
}

func BenchmarkGenerateSubtreeRoots(b *testing.B) {
	benchmarks := []struct {
		name     string
		blobSize int
	}{
		{
			name:     "2 shares",
			blobSize: share.AvailableBytesFromSparseShares(2),
		},
		{
			name:     "16 shares",
			blobSize: share.AvailableBytesFromSparseShares(16),
		},
		{
			name:     "64 shares",
			blobSize: share.AvailableBytesFromSparseShares(64),
		},
		{
			name:     "128 shares",
			blobSize: share.AvailableBytesFromSparseShares(128),
		},
		{
			name:     "256 shares",
			blobSize: share.AvailableBytesFromSparseShares(256),
		},
		{
			name:     "16384 shares",
			blobSize: share.AvailableBytesFromSparseShares(16384),
		},
	}

	for _, bm := range benchmarks {
		blobs := test.GenerateBlobs(bm.blobSize)
		if len(blobs) != 1 {
			b.Fatal("expected exactly one blob")
		}
		blob := blobs[0]

		b.Run(bm.name+"/original", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := inclusion.GenerateSubtreeRoots(blob, defaultSubtreeRootThreshold)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(bm.name+"/reused_nmt", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := inclusion.GenerateSubtreeRootsReusedNMT(blob, defaultSubtreeRootThreshold)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(bm.name+"/parallel", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := inclusion.GenerateSubtreeRootsParallel(blob, defaultSubtreeRootThreshold)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGenerateSubtreeRootsParallelWorkers(b *testing.B) {
	blobSizes := []struct {
		name     string
		blobSize int
	}{
		{
			name:     "64 shares",
			blobSize: share.AvailableBytesFromSparseShares(64),
		},
		{
			name:     "128 shares",
			blobSize: share.AvailableBytesFromSparseShares(128),
		},
		{
			name:     "256 shares",
			blobSize: share.AvailableBytesFromSparseShares(256),
		},
		{
			name:     "512 shares",
			blobSize: share.AvailableBytesFromSparseShares(512),
		},
	}

	workerCounts := []int{1, 2, 4, 8, 16}

	for _, bs := range blobSizes {
		blobs := test.GenerateBlobs(bs.blobSize)
		if len(blobs) != 1 {
			b.Fatal("expected exactly one blob")
		}
		blob := blobs[0]

		for _, workers := range workerCounts {
			b.Run(fmt.Sprintf("%s/%d_workers", bs.name, workers), func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := inclusion.GenerateSubtreeRootsParallelWithWorkers(blob, defaultSubtreeRootThreshold, workers)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}

		// Also benchmark the sequential reused version for comparison
		b.Run(bs.name+"/sequential_reused", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := inclusion.GenerateSubtreeRootsReusedNMT(blob, defaultSubtreeRootThreshold)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCreateCommitment(b *testing.B) {
	benchmarks := []struct {
		name     string
		blobSize int
	}{
		{
			name:     "2 shares",
			blobSize: share.AvailableBytesFromSparseShares(2),
		},
		{
			name:     "4 shares",
			blobSize: share.AvailableBytesFromSparseShares(4),
		},
		{
			name:     "8 shares",
			blobSize: share.AvailableBytesFromSparseShares(8),
		},
		{
			name:     "16 shares",
			blobSize: share.AvailableBytesFromSparseShares(16),
		},
		{
			name:     "32 shares",
			blobSize: share.AvailableBytesFromSparseShares(32),
		},
		{
			name:     "64 shares",
			blobSize: share.AvailableBytesFromSparseShares(64),
		},
		{
			name:     "128 shares",
			blobSize: share.AvailableBytesFromSparseShares(128),
		},
		{
			name:     "256 shares",
			blobSize: share.AvailableBytesFromSparseShares(256),
		},
		{
			name:     "512 shares",
			blobSize: share.AvailableBytesFromSparseShares(512),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			blobs := test.GenerateBlobs(bm.blobSize)
			if len(blobs) != 1 {
				b.Fatal("expected exactly one blob")
			}
			blob := blobs[0]

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := inclusion.CreateCommitment(blob, simpleMerkleRoot, defaultSubtreeRootThreshold)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestCreateCommitmentsEquivalence(t *testing.T) {
	// Test with various combinations of blob counts and sizes
	testCases := []struct {
		name      string
		blobSizes []int
	}{
		{
			name:      "single small blob",
			blobSizes: []int{1024}, // ~2 shares
		},
		{
			name:      "multiple small blobs",
			blobSizes: []int{1024, 2048, 1536},
		},
		{
			name:      "mixed sizes",
			blobSizes: []int{512, 8192, 32768, 1024},
		},
		{
			name:      "large blobs",
			blobSizes: []int{65536, 131072}, // 128 and 256 shares
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blobs := test.GenerateBlobs(tc.blobSizes...)

			// Sequential version
			sequential, err := inclusion.CreateCommitmentsSequential(blobs, simpleMerkleRoot, defaultSubtreeRootThreshold)
			require.NoError(t, err)

			// Parallel version with different worker counts
			for _, workers := range []int{1, 2, 4, 8} {
				parallel, err := inclusion.CreateCommitments(blobs, simpleMerkleRoot, defaultSubtreeRootThreshold, workers)
				require.NoError(t, err)

				assert.Equal(t, sequential, parallel,
					"Parallel results with %d workers should match sequential for %s", workers, tc.name)
			}
		})
	}
}

func TestCreateCommitmentsEmpty(t *testing.T) {
	// Test with empty blob slice
	result, err := inclusion.CreateCommitments([]*share.Blob{}, simpleMerkleRoot, defaultSubtreeRootThreshold, 4)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// BenchmarkCommitmentsComparison directly compares CreateCommitment vs CreateCommitments
func BenchmarkCommitmentsComparison(b *testing.B) {
	// Test scenarios with different blob configurations
	scenarios := []struct {
		numBlobs     int
		bytesPerBlob int
		description  string
	}{
		{1, 1048576, "1x1MB"},    // 1 blob of 1MB
		{10, 104858, "10x100KB"}, // 10 blobs of ~100KB each
		{100, 10486, "100x10KB"}, // 100 blobs of ~10KB each
		{4, 1048576, "4x1MB"},    // 4 blobs of 1MB each
		{16, 262144, "16x256KB"}, // 16 blobs of 256KB each
		{64, 65536, "64x64KB"},   // 64 blobs of 64KB each
		{16, 8388608, "16x8MB"},  // 16 blobs of 8MB each (128MB total)
	}

	for _, scenario := range scenarios {
		// Generate blobs for this scenario
		blobSizes := make([]int, scenario.numBlobs)
		for i := range blobSizes {
			blobSizes[i] = scenario.bytesPerBlob
		}
		blobs := test.GenerateBlobs(blobSizes...)

		//totalMB := float64(scenario.numBlobs * scenario.bytesPerBlob) / (1024 * 1024)

		// Sequential: CreateCommitment for each blob
		b.Run(fmt.Sprintf("%s_Sequential", scenario.description), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, blob := range blobs {
					_, err := inclusion.CreateCommitment(blob, simpleMerkleRoot, defaultSubtreeRootThreshold)
					if err != nil {
						b.Fatal(err)
					}
				}
			}
			//b.ReportMetric(totalMB*1000/b.Elapsed().Seconds(), "MB/s")
		})

		// Parallel: CreateCommitments with 8 workers
		b.Run(fmt.Sprintf("%s_Parallel8", scenario.description), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := inclusion.CreateCommitments(blobs, simpleMerkleRoot, defaultSubtreeRootThreshold, 8)
				if err != nil {
					b.Fatal(err)
				}
			}
			//b.ReportMetric(totalMB*1000/b.Elapsed().Seconds(), "MB/s")
		})

		// For the large 16x8MB scenario, also test with different worker counts
		if scenario.description == "16x8MB" {
			for _, workers := range []int{4, 16, 32, 4 * runtime.NumCPU()} {
				b.Run(fmt.Sprintf("%s_Parallel%d", scenario.description, workers), func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, err := inclusion.CreateCommitments(blobs, simpleMerkleRoot, defaultSubtreeRootThreshold, workers)
						if err != nil {
							b.Fatal(err)
						}
					}
					//b.ReportMetric(totalMB*1000/b.Elapsed().Seconds(), "MB/s")
				})
			}
		}
	}
}
