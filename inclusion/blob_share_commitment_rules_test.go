package inclusion_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/celestiaorg/go-square/v4/inclusion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultSubtreeRootThreshold = 64
	defaultMaxSquareSize        = 128
)

func TestBlobSharesUsedNonInteractiveDefaults(t *testing.T) {
	defaultSquareSize := 128
	type test struct {
		cursor, expected int
		blobLens         []int
		indexes          []uint32
		expectError      bool
	}
	tests := []test{
		{2, 1, []int{1}, []uint32{2}, false},
		{2, 1, []int{1}, []uint32{2}, false},
		{3, 6, []int{3, 3}, []uint32{3, 6}, false},
		{0, 8, []int{8}, []uint32{0}, false},
		{1, 6, []int{3, 3}, []uint32{1, 4}, false},
		{1, 32, []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}, false},
		{3, 12, []int{5, 7}, []uint32{3, 8}, false},
		{0, 20, []int{5, 5, 5, 5}, []uint32{0, 5, 10, 15}, false},
		{0, 10, []int{10}, []uint32{0}, false},
		{0, 0, []int{0}, []uint32{0}, false}, // This case has blobLen=0, which should work
		{1, 20, []int{10, 10}, []uint32{1, 11}, false},
		{0, 1000, []int{1000}, []uint32{0}, false},
		{0, defaultSquareSize + 1, []int{defaultSquareSize + 1}, []uint32{0}, false},
		{1, 385, []int{128, 128, 128}, []uint32{2, 130, 258}, false},
		{1024, 32, []int{32}, []uint32{1024}, false},
	}
	for i, tt := range tests {
		res, indexes, err := inclusion.BlobSharesUsedNonInteractiveDefaults(tt.cursor, defaultSubtreeRootThreshold, tt.blobLens...)
		test := fmt.Sprintf("test %d: cursor %d", i, tt.cursor)
		if tt.expectError {
			assert.Error(t, err, test)
		} else {
			assert.NoError(t, err, test)
			assert.Equal(t, tt.expected, res, test)
			assert.Equal(t, tt.indexes, indexes, test)
		}
	}
}

func TestNextShareIndex(t *testing.T) {
	type test struct {
		name                        string
		cursor, blobLen, squareSize int
		expectedIndex               int
		expectError                 bool
	}
	tests := []test{
		{
			name:          "whole row blobLen 4",
			cursor:        0,
			blobLen:       4,
			squareSize:    4,
			expectedIndex: 0,
			expectError:   false,
		},
		{
			name:          "half row blobLen 2 cursor 1",
			cursor:        1,
			blobLen:       2,
			squareSize:    4,
			expectedIndex: 1,
			expectError:   false,
		},
		{
			name:          "half row blobLen 2 cursor 2",
			cursor:        2,
			blobLen:       2,
			squareSize:    4,
			expectedIndex: 2,
			expectError:   false,
		},
		{
			name:          "half row blobLen 4 cursor 3",
			cursor:        3,
			blobLen:       4,
			squareSize:    8,
			expectedIndex: 3,
			expectError:   false,
		},
		{
			name:          "blobLen 5 cursor 3 size 8",
			cursor:        3,
			blobLen:       5,
			squareSize:    8,
			expectedIndex: 3,
			expectError:   false,
		},
		{
			name:          "blobLen 2 cursor 3 square size 8",
			cursor:        3,
			blobLen:       2,
			squareSize:    8,
			expectedIndex: 3,
			expectError:   false,
		},
		{
			name:          "cursor 3 blobLen 5 size 8",
			cursor:        3,
			blobLen:       5,
			squareSize:    8,
			expectedIndex: 3,
			expectError:   false,
		},
		{
			name:          "bloblen 12 cursor 1 size 16",
			cursor:        1,
			blobLen:       12,
			squareSize:    16,
			expectedIndex: 1,
			expectError:   false,
		},
		{
			name:          "edge case where there are many blobs with a single size",
			cursor:        10291,
			blobLen:       1,
			squareSize:    128,
			expectedIndex: 10291,
			expectError:   false,
		},
		{
			name:          "second row blobLen 2 cursor 11 square size 8",
			cursor:        11,
			blobLen:       2,
			squareSize:    8,
			expectedIndex: 11,
			expectError:   false,
		},
		{
			name:          "blob share commitment rules for reduced padding diagram",
			cursor:        11,
			blobLen:       11,
			squareSize:    8,
			expectedIndex: 11,
			expectError:   false,
		},
		{
			name:          "at threshold",
			cursor:        11,
			blobLen:       defaultSubtreeRootThreshold,
			squareSize:    defaultSubtreeRootThreshold, // defaultSubtreeRootThreshold (64) is already a power of 2
			expectedIndex: 11,
			expectError:   false,
		},
		{
			name:          "one over the threshold",
			cursor:        64,
			blobLen:       defaultSubtreeRootThreshold + 1,
			squareSize:    128,
			expectedIndex: 64,
			expectError:   false,
		},
		{
			name:          "one under the threshold",
			cursor:        64,
			blobLen:       defaultSubtreeRootThreshold - 1,
			squareSize:    128,
			expectedIndex: 64,
			expectError:   false,
		},
		{
			name:          "one under the threshold small square size",
			cursor:        1,
			blobLen:       defaultSubtreeRootThreshold - 1,
			squareSize:    16,
			expectedIndex: 1,
			expectError:   false,
		},
		{
			name:          "max padding for square size 128",
			cursor:        1,
			blobLen:       16256,
			squareSize:    128,
			expectedIndex: 128,
			expectError:   false,
		},
		{
			name:          "half max padding for square size 128",
			cursor:        1,
			blobLen:       8192,
			squareSize:    128,
			expectedIndex: 128,
			expectError:   false,
		},
		{
			name:          "quarter max padding for square size 128",
			cursor:        1,
			blobLen:       4096,
			squareSize:    128,
			expectedIndex: 64,
			expectError:   false,
		},
		{
			name:          "round up to 128 subtree size",
			cursor:        1,
			blobLen:       8193,
			squareSize:    128,
			expectedIndex: 128,
			expectError:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := inclusion.NextShareIndex(tt.cursor, tt.blobLen, defaultSubtreeRootThreshold)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedIndex, res)
			}
		})
	}
}

func TestRoundUpByMultipleOf(t *testing.T) {
	type test struct {
		cursor, v     int
		expectedIndex int
		expectError   bool
	}
	tests := []test{
		{
			cursor:        1,
			v:             2,
			expectedIndex: 2,
			expectError:   false,
		},
		{
			cursor:        2,
			v:             2,
			expectedIndex: 2,
			expectError:   false,
		},
		{
			cursor:        0,
			v:             2,
			expectedIndex: 0,
			expectError:   false,
		},
		{
			cursor:        5,
			v:             2,
			expectedIndex: 6,
			expectError:   false,
		},
		{
			cursor:        8,
			v:             16,
			expectedIndex: 16,
			expectError:   false,
		},
		{
			cursor:        33,
			v:             1,
			expectedIndex: 33,
			expectError:   false,
		},
		{
			cursor:        32,
			v:             16,
			expectedIndex: 32,
			expectError:   false,
		},
		{
			cursor:        33,
			v:             16,
			expectedIndex: 48,
			expectError:   false,
		},
		{
			cursor:        10,
			v:             0,
			expectedIndex: 0,
			expectError:   true,
		},
	}
	for i, tt := range tests {
		t.Run(
			fmt.Sprintf(
				"test %d: %d cursor %d v %d expectedIndex",
				i,
				tt.cursor,
				tt.v,
				tt.expectedIndex,
			),
			func(t *testing.T) {
				res, err := inclusion.RoundUpByMultipleOf(tt.cursor, tt.v)
				if tt.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expectedIndex, res)
				}
			})
	}
}

func TestRoundUpPowerOfTwo(t *testing.T) {
	type testCase struct {
		input int
		want  int
	}
	testCases := []testCase{
		{input: -1, want: 1},
		{input: 0, want: 1},
		{input: 1, want: 1},
		{input: 2, want: 2},
		{input: 4, want: 4},
		{input: 5, want: 8},
		{input: 8, want: 8},
		{input: 11, want: 16},
		{input: 511, want: 512},
		{input: 1 << 30, want: 1 << 30},
		{input: (1 << 30) + 1, want: 1 << 31},
		{input: math.MaxInt32, want: 1 << 31},
		{input: math.MaxInt32 - 1, want: 1 << 31},
		{input: 1 << 62, want: 1 << 62},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%d", tc.input), func(t *testing.T) {
			got, err := inclusion.RoundUpPowerOfTwo(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRoundUpPowerOfTwoOverflow(t *testing.T) {
	t.Run("int overflow", func(t *testing.T) {
		_, err := inclusion.RoundUpPowerOfTwo(math.MaxInt)
		assert.Error(t, err)
	})
	t.Run("uint64 overflow", func(t *testing.T) {
		_, err := inclusion.RoundUpPowerOfTwo(uint64(math.MaxUint64))
		assert.Error(t, err)
	})
	t.Run("uint64 just over last power of two", func(t *testing.T) {
		_, err := inclusion.RoundUpPowerOfTwo(uint64((1 << 63) + 1))
		assert.Error(t, err)
	})
}

func TestBlobMinSquareSize(t *testing.T) {
	type testCase struct {
		shareCount int
		want       int
	}
	testCases := []testCase{
		{
			shareCount: 0,
			want:       1,
		},
		{
			shareCount: 1,
			want:       1,
		},
		{
			shareCount: 2,
			want:       2,
		},
		{
			shareCount: 3,
			want:       2,
		},
		{
			shareCount: 4,
			want:       2,
		},
		{
			shareCount: 5,
			want:       4,
		},
		{
			shareCount: 16,
			want:       4,
		},
		{
			shareCount: 17,
			want:       8,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("shareCount %d", tc.shareCount), func(t *testing.T) {
			got, err := inclusion.BlobMinSquareSize(tc.shareCount)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// oldBlobMinSquareSize is the float64 formula that BlobMinSquareSize used
// before it switched to integer arithmetic. It is correct for every shareCount
// up to 2^52, where float64 still holds the result of the square root exactly
// enough. It is kept here only as a reference.
func oldBlobMinSquareSize(shareCount int) (int, error) {
	return inclusion.RoundUpPowerOfTwo(int(math.Ceil(math.Sqrt(float64(shareCount)))))
}

// TestBlobMinSquareSizeMatchesFloatFormula checks that BlobMinSquareSize gives
// the same result as the old float64 formula for every input that the old
// formula handled correctly.
func TestBlobMinSquareSizeMatchesFloatFormula(t *testing.T) {
	check := func(n int) {
		want, wantErr := oldBlobMinSquareSize(n)
		got, gotErr := inclusion.BlobMinSquareSize(n)
		if got != want || gotErr != nil || wantErr != nil {
			t.Fatalf("BlobMinSquareSize(%d) = %d, %v; old formula = %d, %v", n, got, gotErr, want, wantErr)
		}
	}

	// Negative inputs.
	for _, n := range []int{math.MinInt, math.MinInt + 1, -1 << 40, -17, -2, -1} {
		check(n)
	}

	// Every input in [0, 2^20].
	for n := 0; n <= 1<<20; n++ {
		check(n)
	}

	// Inputs around perfect squares up to 2^52: every k up to 2^16, then a
	// sample of larger k. A prime step keeps the sample off powers of two,
	// and the last k is always included.
	checkSquare := func(k int64) {
		sq := k * k
		check(int(sq - 1))
		check(int(sq))
		if sq+1 <= 1<<52 {
			check(int(sq + 1))
		}
	}
	for k := int64(1 << 10); k <= 1<<16; k++ {
		checkSquare(k)
	}
	for k := int64(1<<16) + 1; k < 1<<26; k += 1021 {
		checkSquare(k)
	}
	for e := 17; e <= 26; e++ {
		for d := int64(-2); d <= 2; d++ {
			if k := int64(1)<<e + d; k <= 1<<26 {
				checkSquare(k)
			}
		}
	}

	// Inputs around every power of two up to 2^52.
	for e := 0; e <= 52; e++ {
		p := int64(1) << e
		for d := int64(-2); d <= 2; d++ {
			if n := p + d; n >= 0 && n <= 1<<52 {
				check(int(n))
			}
		}
	}
}

// TestBlobMinSquareSizeLargeShareCount checks inputs above 2^52, where the old
// float64 formula returned a square too small to hold the shares.
func TestBlobMinSquareSizeLargeShareCount(t *testing.T) {
	if math.MaxInt < math.MaxInt64 {
		t.Skip("needs a 64-bit int")
	}
	testCases := []struct {
		shareCount int64
		want       int64
	}{
		{shareCount: 1 << 52, want: 1 << 26},
		{shareCount: 1<<52 + 1, want: 1 << 27},
		{shareCount: 1<<54 + 1, want: 1 << 28},
		{shareCount: (1<<27+1)*(1<<27+1) - 1, want: 1 << 28},
		{shareCount: 1 << 62, want: 1 << 31},
		{shareCount: 1<<62 + 1, want: 1 << 32},
		{shareCount: math.MaxInt64, want: 1 << 32},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("shareCount %d", tc.shareCount), func(t *testing.T) {
			got, err := inclusion.BlobMinSquareSize(int(tc.shareCount))
			require.NoError(t, err)
			assert.Equal(t, int(tc.want), got)
		})
	}
}

func TestSubTreeWidth(t *testing.T) {
	type testCase struct {
		shareCount int
		want       int
	}
	testCases := []testCase{
		{
			shareCount: 0,
			want:       1,
		},
		{
			shareCount: 1,
			want:       1,
		},
		{
			shareCount: 2,
			want:       1,
		},
		{
			shareCount: defaultSubtreeRootThreshold,
			want:       1,
		},
		{
			shareCount: defaultSubtreeRootThreshold + 1,
			want:       2,
		},
		{
			shareCount: defaultSubtreeRootThreshold - 1,
			want:       1,
		},
		{
			shareCount: defaultSubtreeRootThreshold * 2,
			want:       2,
		},
		{
			shareCount: (defaultSubtreeRootThreshold * 2) + 1,
			want:       4,
		},
		{
			shareCount: (defaultSubtreeRootThreshold * 3) - 1,
			want:       4,
		},
		{
			shareCount: (defaultSubtreeRootThreshold * 4),
			want:       4,
		},
		{
			shareCount: (defaultSubtreeRootThreshold * 5),
			want:       8,
		},
		{
			shareCount: (defaultSubtreeRootThreshold * defaultMaxSquareSize) - 1,
			want:       128,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("shareCount %d", tc.shareCount), func(t *testing.T) {
			got, err := inclusion.SubTreeWidth(tc.shareCount, defaultSubtreeRootThreshold)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got, i)
		})
	}
}

func TestSubTreeWidthErrors(t *testing.T) {
	testCases := []struct {
		name                 string
		subtreeRootThreshold int
	}{
		{"zero threshold", 0},
		{"negative threshold", -1},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := inclusion.SubTreeWidth(2, tc.subtreeRootThreshold)
			assert.Error(t, err)
		})
	}
}

func TestRoundDownPowerOfTwo(t *testing.T) {
	type testCase struct {
		input int
		want  int
	}
	testCases := []testCase{
		{input: 1, want: 1},
		{input: 2, want: 2},
		{input: 4, want: 4},
		{input: 5, want: 4},
		{input: 8, want: 8},
		{input: 11, want: 8},
		{input: 511, want: 256},
		{input: 1 << 30, want: 1 << 30},
		{input: (1 << 30) + 1, want: 1 << 30},
		{input: math.MaxInt32, want: 1 << 30},
		{input: math.MaxInt32 - 1, want: 1 << 30},
		{input: math.MaxInt32 + 1, want: 1 << 31},
		{input: 1 << 62, want: 1 << 62},
		{input: (1 << 62) + 1, want: 1 << 62},
		{input: math.MaxInt, want: 1 << 62},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("input_%d", tc.input), func(t *testing.T) {
			got, err := inclusion.RoundDownPowerOfTwo(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRoundDownPowerOfTwoErrors(t *testing.T) {
	t.Run("negative input", func(t *testing.T) {
		_, err := inclusion.RoundDownPowerOfTwo(-1)
		assert.Error(t, err)
	})
	t.Run("zero input", func(t *testing.T) {
		_, err := inclusion.RoundDownPowerOfTwo(0)
		assert.Error(t, err)
	})
}
