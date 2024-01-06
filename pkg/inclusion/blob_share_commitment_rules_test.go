package inclusion_test

import (
	"fmt"
	"testing"

	"github.com/celestiaorg/go-square/pkg/inclusion"
	"github.com/celestiaorg/go-square/pkg/shares"
	"github.com/stretchr/testify/assert"
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
	}
	tests := []test{
		{2, 1, []int{1}, []uint32{2}},
		{2, 1, []int{1}, []uint32{2}},
		{3, 6, []int{3, 3}, []uint32{3, 6}},
		{0, 8, []int{8}, []uint32{0}},
		{1, 6, []int{3, 3}, []uint32{1, 4}},
		{1, 32, []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}},
		{3, 12, []int{5, 7}, []uint32{3, 8}},
		{0, 20, []int{5, 5, 5, 5}, []uint32{0, 5, 10, 15}},
		{0, 10, []int{10}, []uint32{0}},
		{1, 20, []int{10, 10}, []uint32{1, 11}},
		{0, 1000, []int{1000}, []uint32{0}},
		{0, defaultSquareSize + 1, []int{defaultSquareSize + 1}, []uint32{0}},
		{1, 385, []int{128, 128, 128}, []uint32{2, 130, 258}},
		{1024, 32, []int{32}, []uint32{1024}},
	}
	for i, tt := range tests {
		res, indexes := inclusion.BlobSharesUsedNonInteractiveDefaults(tt.cursor, defaultSubtreeRootThreshold, tt.blobLens...)
		test := fmt.Sprintf("test %d: cursor %d", i, tt.cursor)
		assert.Equal(t, tt.expected, res, test)
		assert.Equal(t, tt.indexes, indexes, test)
	}
}

func TestNextShareIndex(t *testing.T) {
	type test struct {
		name                        string
		cursor, blobLen, squareSize int
		expectedIndex               int
	}
	tests := []test{
		{
			name:          "whole row blobLen 4",
			cursor:        0,
			blobLen:       4,
			squareSize:    4,
			expectedIndex: 0,
		},
		{
			name:          "half row blobLen 2 cursor 1",
			cursor:        1,
			blobLen:       2,
			squareSize:    4,
			expectedIndex: 1,
		},
		{
			name:          "half row blobLen 2 cursor 2",
			cursor:        2,
			blobLen:       2,
			squareSize:    4,
			expectedIndex: 2,
		},
		{
			name:          "half row blobLen 4 cursor 3",
			cursor:        3,
			blobLen:       4,
			squareSize:    8,
			expectedIndex: 3,
		},
		{
			name:          "blobLen 5 cursor 3 size 8",
			cursor:        3,
			blobLen:       5,
			squareSize:    8,
			expectedIndex: 3,
		},
		{
			name:          "blobLen 2 cursor 3 square size 8",
			cursor:        3,
			blobLen:       2,
			squareSize:    8,
			expectedIndex: 3,
		},
		{
			name:          "cursor 3 blobLen 5 size 8",
			cursor:        3,
			blobLen:       5,
			squareSize:    8,
			expectedIndex: 3,
		},
		{
			name:          "bloblen 12 cursor 1 size 16",
			cursor:        1,
			blobLen:       12,
			squareSize:    16,
			expectedIndex: 1,
		},
		{
			name:          "edge case where there are many blobs with a single size",
			cursor:        10291,
			blobLen:       1,
			squareSize:    128,
			expectedIndex: 10291,
		},
		{
			name:          "second row blobLen 2 cursor 11 square size 8",
			cursor:        11,
			blobLen:       2,
			squareSize:    8,
			expectedIndex: 11,
		},
		{
			name:          "blob share commitment rules for reduced padding diagram",
			cursor:        11,
			blobLen:       11,
			squareSize:    8,
			expectedIndex: 11,
		},
		{
			name:          "at threshold",
			cursor:        11,
			blobLen:       defaultSubtreeRootThreshold,
			squareSize:    shares.RoundUpPowerOfTwo(defaultSubtreeRootThreshold),
			expectedIndex: 11,
		},
		{
			name:          "one over the threshold",
			cursor:        64,
			blobLen:       defaultSubtreeRootThreshold + 1,
			squareSize:    128,
			expectedIndex: 64,
		},
		{
			name:          "one under the threshold",
			cursor:        64,
			blobLen:       defaultSubtreeRootThreshold - 1,
			squareSize:    128,
			expectedIndex: 64,
		},
		{
			name:          "one under the threshold small square size",
			cursor:        1,
			blobLen:       defaultSubtreeRootThreshold - 1,
			squareSize:    16,
			expectedIndex: 1,
		},
		{
			name:          "max padding for square size 128",
			cursor:        1,
			blobLen:       16256,
			squareSize:    128,
			expectedIndex: 128,
		},
		{
			name:          "half max padding for square size 128",
			cursor:        1,
			blobLen:       8192,
			squareSize:    128,
			expectedIndex: 128,
		},
		{
			name:          "quarter max padding for square size 128",
			cursor:        1,
			blobLen:       4096,
			squareSize:    128,
			expectedIndex: 64,
		},
		{
			name:          "round up to 128 subtree size",
			cursor:        1,
			blobLen:       8193,
			squareSize:    128,
			expectedIndex: 128,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := inclusion.NextShareIndex(tt.cursor, tt.blobLen, defaultSubtreeRootThreshold)
			assert.Equal(t, tt.expectedIndex, res)
		})
	}
}

func TestRoundUpByMultipleOf(t *testing.T) {
	type test struct {
		cursor, v     int
		expectedIndex int
	}
	tests := []test{
		{
			cursor:        1,
			v:             2,
			expectedIndex: 2,
		},
		{
			cursor:        2,
			v:             2,
			expectedIndex: 2,
		},
		{
			cursor:        0,
			v:             2,
			expectedIndex: 0,
		},
		{
			cursor:        5,
			v:             2,
			expectedIndex: 6,
		},
		{
			cursor:        8,
			v:             16,
			expectedIndex: 16,
		},
		{
			cursor:        33,
			v:             1,
			expectedIndex: 33,
		},
		{
			cursor:        32,
			v:             16,
			expectedIndex: 32,
		},
		{
			cursor:        33,
			v:             16,
			expectedIndex: 48,
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
				res := inclusion.RoundUpByMultipleOf(tt.cursor, tt.v)
				assert.Equal(t, tt.expectedIndex, res)
			})
	}
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
			got := inclusion.BlobMinSquareSize(tc.shareCount)
			assert.Equal(t, tc.want, got)
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
			got := inclusion.SubTreeWidth(tc.shareCount, defaultSubtreeRootThreshold)
			assert.Equal(t, tc.want, got, i)
		})
	}
}
