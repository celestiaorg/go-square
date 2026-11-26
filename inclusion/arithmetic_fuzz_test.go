package inclusion_test

import (
	"encoding/binary"
	"math"
	"math/bits"
	"testing"

	"github.com/celestiaorg/go-square/v4/inclusion"
	"github.com/stretchr/testify/require"
)

// maxFuzzShareCount bounds share counts to what a uint32 blob length can
// produce (sequence lengths are uint32, and a share holds at least one byte).
const maxFuzzShareCount = math.MaxUint32

// FuzzBlobLayoutArithmetic checks the helpers that place blobs in the square
// and size their commitments against integer reference implementations. A
// wrong value here moves a blob or changes its commitment, so it is a consensus
// difference between nodes. Any int input must also be handled without a panic.
func FuzzBlobLayoutArithmetic(f *testing.F) {
	for _, c := range []struct{ shares, threshold, cursor int64 }{
		{0, 1, 0},
		{1, 64, 0},
		{2, 1, 1},
		{11, 2, 13},
		{128, 64, 3},
		{129, 64, 1000},
		{4096, 64, 511},
		{1 << 20, 64, 1 << 18},
		{maxFuzzShareCount, 64, 1 << 31},
		{-1, 64, -1},
		{5, 0, 3},
		{5, -1, 3},
		{math.MaxInt64, 1, math.MaxInt64},
		{math.MinInt64, math.MinInt64, math.MinInt64},
	} {
		f.Add(c.shares, c.threshold, c.cursor, []byte{1, 0, 0, 0, 8, 1, 0, 0, 255, 255, 0, 0})
	}
	f.Fuzz(func(t *testing.T, shareCount, threshold, cursor int64, blobLens []byte) {
		if len(blobLens) > 4*32 {
			t.Skip()
		}
		n, thr, cur := int(shareCount), int(threshold), int(cursor)

		width, widthErr := inclusion.SubTreeWidth(n, thr)
		minSquare, minSquareErr := inclusion.BlobMinSquareSize(n)
		next, nextErr := inclusion.NextShareIndex(cur, n, thr)
		_, _ = inclusion.RoundUpByMultipleOf(cur, thr)
		if thr <= 0 {
			require.Error(t, widthErr)
			require.Error(t, nextErr)
		}
		inDomain := 0 <= n && n <= maxFuzzShareCount && 0 < thr && thr <= maxFuzzShareCount
		if !inDomain {
			return // only the absence of panics is required outside the domain
		}

		require.NoError(t, minSquareErr)
		require.Equal(t, refMinSquareSize(uint64(n)), uint64(minSquare), "BlobMinSquareSize(%d)", n)
		require.NoError(t, widthErr)
		require.Equal(t, refSubTreeWidth(uint64(n), uint64(thr)), uint64(width), "SubTreeWidth(%d, %d)", n, thr)
		require.True(t, width >= 1 && width&(width-1) == 0)
		require.LessOrEqual(t, width, minSquare, "a subtree must fit in one row of the blob's minimum square")

		if 0 <= cur && cur <= 1<<40 {
			require.NoError(t, nextErr)
			require.Equal(t, refRoundUp(uint64(cur), uint64(width)), uint64(next), "NextShareIndex(%d, %d, %d)", cur, n, thr)
			require.Zero(t, next%width)
			require.Less(t, next-cur, width)
			require.GreaterOrEqual(t, next, cur)
			got, err := inclusion.RoundUpByMultipleOf(cur, width)
			require.NoError(t, err)
			require.Equal(t, next, got)
			checkNonInteractiveDefaults(t, cur, thr, blobLens)
		}
	})
}

// checkNonInteractiveDefaults places blobs one after another with the reference
// rules and compares the indexes and total with BlobSharesUsedNonInteractiveDefaults.
func checkNonInteractiveDefaults(t *testing.T, cursor, threshold int, raw []byte) {
	t.Helper()
	var lens []int
	for i := 0; i+4 <= len(raw); i += 4 {
		lens = append(lens, int(binary.LittleEndian.Uint32(raw[i:])%(1<<20)))
	}
	used, indexes, err := inclusion.BlobSharesUsedNonInteractiveDefaults(cursor, threshold, lens...)
	require.NoError(t, err)
	require.Len(t, indexes, len(lens))
	end := uint64(cursor)
	for i, l := range lens {
		w := refSubTreeWidth(uint64(l), uint64(threshold))
		start := refRoundUp(end, w)
		require.Equal(t, uint32(start), indexes[i], "blob %d", i)
		end = start + uint64(l)
	}
	require.Equal(t, end-uint64(cursor), uint64(used))
}

// FuzzMerkleMountainRangeSizes compares the commitment's mountain range with
// the decomposition it must produce: full trees of maxTreeSize, then the binary
// decomposition of the rest, largest first.
func FuzzMerkleMountainRangeSizes(f *testing.F) {
	for _, c := range []struct {
		total uint64
		shift uint8
	}{{0, 0}, {1, 0}, {11, 2}, {11, 3}, {128, 6}, {1000, 4}, {1<<24 - 1, 20}, {1 << 24, 0}} {
		f.Add(c.total, c.shift)
	}
	f.Fuzz(func(t *testing.T, total uint64, shift uint8) {
		// Subtree widths are powers of two (see SubTreeWidth).
		maxTree := uint64(1) << (shift % 21)
		if total/maxTree > 1<<16 {
			t.Skip()
		}
		got, err := inclusion.MerkleMountainRangeSizes(total, maxTree)
		require.NoError(t, err)
		var want []uint64
		for i := uint64(0); i < total/maxTree; i++ {
			want = append(want, maxTree)
		}
		rest := total % maxTree
		for b := 63; b >= 0; b-- {
			if rest&(1<<b) != 0 {
				want = append(want, 1<<b)
			}
		}
		require.Equal(t, want, got)
	})
}

// FuzzRoundPowerOfTwo checks the generic power-of-two helpers for several
// integer types, including overflow of the result type.
func FuzzRoundPowerOfTwo(f *testing.F) {
	for _, v := range []int64{math.MinInt64, -1, 0, 1, 2, 3, 63, 64, 65, 127, 128, 129, 255, 256, 1<<31 + 1, 1 << 62, 1<<62 + 1, math.MaxInt64} {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v int64) {
		checkRoundPowerOfTwo(t, v, math.MaxInt64)
		checkRoundPowerOfTwo(t, int(v), math.MaxInt)
		checkRoundPowerOfTwo(t, int32(v), math.MaxInt32)
		checkRoundPowerOfTwo(t, int8(v), math.MaxInt8)
		checkRoundPowerOfTwo(t, uint8(v), math.MaxUint8)
		checkRoundPowerOfTwo(t, uint32(v), math.MaxUint32)
		checkRoundPowerOfTwo(t, uint64(v), math.MaxUint64)
	})
}

func checkRoundPowerOfTwo[I int | int8 | int32 | int64 | uint8 | uint32 | uint64](t *testing.T, v I, maxValue uint64) {
	t.Helper()
	up, upErr := inclusion.RoundUpPowerOfTwo(v)
	if v <= 1 {
		require.NoError(t, upErr)
		require.Equal(t, I(1), up)
	} else {
		want := uint64(1) << (bits.Len64(uint64(v) - 1))
		if uint64(v) > 1<<63 || want > maxValue {
			require.Error(t, upErr, "RoundUpPowerOfTwo(%v) overflows %T", v, v)
		} else {
			require.NoError(t, upErr)
			require.Equal(t, want, uint64(up), "RoundUpPowerOfTwo(%v)", v)
		}
	}

	down, downErr := inclusion.RoundDownPowerOfTwo(v)
	if v <= 0 {
		require.Error(t, downErr)
		return
	}
	require.NoError(t, downErr)
	require.Equal(t, uint64(1)<<(bits.Len64(uint64(v))-1), uint64(down), "RoundDownPowerOfTwo(%v)", v)
}

// refMinSquareSize is the smallest power of two k with k*k >= n, computed
// without floating point.
func refMinSquareSize(n uint64) uint64 {
	k := uint64(1)
	for k*k < n {
		k <<= 1
	}
	return k
}

// refSubTreeWidth is ADR-013's width: the smallest power of two at least
// ceil(n/threshold), capped at the blob's minimum square size.
func refSubTreeWidth(n, threshold uint64) uint64 {
	s := uint64(1)
	for s*threshold < n {
		s <<= 1
	}
	return min(s, refMinSquareSize(n))
}

func refRoundUp(x, m uint64) uint64 {
	return (x + m - 1) / m * m
}
