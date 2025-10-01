package inclusion_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"runtime"
	"testing"
	"time"

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

func hashConcatenatedData(data [][]byte) []byte {
	var total []byte
	for _, d := range data {
		total = append(total, d...)
	}
	finalHash := sha256.Sum256(total)
	return finalHash[:]
}

// TestCreateParallelCommitments compares results of
// parallel and non-parallel versions of the algorithm with different configurations.
func TestCreateParallelCommitments(t *testing.T) {
	t.Run("empty blob", func(t *testing.T) {
		result, err := inclusion.CreateParallelCommitments([]*share.Blob{}, hashConcatenatedData, defaultSubtreeRootThreshold, 4)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("equivalence with sequential commitments (random)", func(t *testing.T) {
		var (
			workers     = runtime.NumCPU()
			rng         = rand.New(rand.NewSource(time.Now().UnixNano()))
			minBlobSize = 512
			maxBlobSize = 1024 * 1024
			maxBlobNum  = 10
			minBlobNum  = 2
			testCount   = 100
		)

		for i := 0; i < testCount; i++ {
			numBlobs := rng.Intn(maxBlobNum-minBlobNum) + minBlobNum
			blobSizes := make([]int, numBlobs)
			maxSize := 0
			for j := range blobSizes {
				blobSizes[j] = rng.Intn(maxBlobSize-minBlobSize) + minBlobSize
				if blobSizes[j] > maxSize {
					maxSize = blobSizes[j]
				}
			}
			t.Run(fmt.Sprintf("test_%d_blobs_%d_max_size_%d", numBlobs, i, maxSize), func(t *testing.T) {
				blobs := test.GenerateBlobs(blobSizes...)

				sequential, err := inclusion.CreateCommitments(blobs, hashConcatenatedData, defaultSubtreeRootThreshold)
				require.NoError(t, err)

				parallel, err := inclusion.CreateParallelCommitments(blobs, hashConcatenatedData, defaultSubtreeRootThreshold, workers)
				require.NoError(t, err)

				assert.Equal(t, sequential, parallel,
					"Parallel results with %d workers should match sequential for %d blobs",
					workers, numBlobs)
			})
		}
	})

	t.Run("equivalence with sequential commitments (fixed)", func(t *testing.T) {
		genBlobSizes := func(size, count int) []int {
			blobSizes := make([]int, count)
			for i := range blobSizes {
				blobSizes[i] = size
			}
			return blobSizes
		}
		testCases := []struct {
			name      string
			blobSizes []int
		}{
			{
				name:      "16*512bytes",
				blobSizes: genBlobSizes(512, 16),
			},
			{
				name:      "16x8MB",
				blobSizes: genBlobSizes(8*1024*1024, 16),
			},
			{
				name:      "mixed_sizes",
				blobSizes: []int{512, 8192, 32768, 131072},
			},
		}
		workers := runtime.NumCPU()
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				blobs := test.GenerateBlobs(tc.blobSizes...)

				sequential, err := inclusion.CreateCommitments(blobs, hashConcatenatedData, defaultSubtreeRootThreshold)
				require.NoError(t, err)

				parallel, err := inclusion.CreateParallelCommitments(blobs, hashConcatenatedData, defaultSubtreeRootThreshold, workers)
				require.NoError(t, err)

				assert.Equal(t, sequential, parallel,
					"Parallel results should match sequential for %s", tc.name)
			})
		}
	})

	t.Run("invalid worker count", func(t *testing.T) {
		blobs := test.GenerateBlobs(1024)

		_, err := inclusion.CreateParallelCommitments(blobs, hashConcatenatedData, defaultSubtreeRootThreshold, 0)
		require.Error(t, err)

		_, err = inclusion.CreateParallelCommitments(blobs, hashConcatenatedData, defaultSubtreeRootThreshold, -1)
		require.Error(t, err)
	})
}

// BenchmarkCommitmentsComparison directly compares CreateCommitment vs CreateParallelCommitments.
func BenchmarkCommitmentsComparison(b *testing.B) {
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
	emptyHash := func([][]byte) []byte {
		return nil
	}
	for _, scenario := range scenarios {
		blobSizes := make([]int, scenario.numBlobs)
		for i := range blobSizes {
			blobSizes[i] = scenario.bytesPerBlob
		}
		blobs := test.GenerateBlobs(blobSizes...)
		b.Run(fmt.Sprintf("%s_Sequential", scenario.description), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for _, blob := range blobs {
					_, err := inclusion.CreateCommitment(blob, emptyHash, defaultSubtreeRootThreshold)
					require.NoError(b, err)
				}
			}
		})
		b.Run(fmt.Sprintf("%s_Parallel", scenario.description), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := inclusion.CreateParallelCommitments(blobs, emptyHash, defaultSubtreeRootThreshold, runtime.NumCPU())
				require.NoError(b, err)
			}
		})
	}
}
