package inclusion

import (
	"math"

	"github.com/celestiaorg/go-square/pkg/shares"
	"golang.org/x/exp/constraints"
)

// BlobSharesUsedNonInteractiveDefaults returns the number of shares used by a given set
// of blobs share lengths. It follows the blob share commitment rules and
// returns the share indexes for each blob.
func BlobSharesUsedNonInteractiveDefaults(cursor, subtreeRootThreshold int, blobShareLens ...int) (sharesUsed int, indexes []uint32) {
	start := cursor
	indexes = make([]uint32, len(blobShareLens))
	for i, blobLen := range blobShareLens {
		cursor = NextShareIndex(cursor, blobLen, subtreeRootThreshold)
		indexes[i] = uint32(cursor)
		cursor += blobLen
	}
	return cursor - start, indexes
}

// NextShareIndex determines the next index in a square that can be used. It
// follows the blob share commitment rules defined in ADR-013. Assumes that all
// args are non negative, that squareSize is a power of two and that the blob can
// fit in the square. The cursor is expected to be the index after the end of
// the previous blob.
//
// See https://github.com/celestiaorg/celestia-app/blob/main/specs/src/specs/data_square_layout.md
// for more information.
func NextShareIndex(cursor, blobShareLen, subtreeRootThreshold int) int {
	// Calculate the subtreewidth. This is the width of the first mountain in the
	// merkle mountain range that makes up the blob share commitment (given the
	// subtreeRootThreshold and the BlobMinSquareSize).
	treeWidth := SubTreeWidth(blobShareLen, subtreeRootThreshold)
	// We round up the cursor to the next multiple of this value i.e. if the cursor
	// was at 13 and the tree width was 4, we return 16.
	return RoundUpByMultipleOf(cursor, treeWidth)
}

// RoundUpByMultipleOf rounds cursor up to the next multiple of v. If cursor is divisible
// by v, then it returns cursor
func RoundUpByMultipleOf(cursor, v int) int {
	if cursor%v == 0 {
		return cursor
	}
	return ((cursor / v) + 1) * v
}

// BlobMinSquareSize returns the minimum square size that can contain shareCount
// number of shares.
func BlobMinSquareSize(shareCount int) int {
	return shares.RoundUpPowerOfTwo(int(math.Ceil(math.Sqrt(float64(shareCount)))))
}

// SubTreeWidth determines the maximum number of leaves per subtree in the share
// commitment over a given blob. The input should be the total number of shares
// used by that blob. The reasoning behind this algorithm is discussed in depth
// in ADR013
// (celestia-app/docs/architecture/adr-013-non-interative-default-rules-for-zero-padding).
func SubTreeWidth(shareCount, subtreeRootThreshold int) int {
	// per ADR013, we use a predetermined threshold to determine width of sub
	// trees used to create share commitments
	s := (shareCount / subtreeRootThreshold)

	// round up if the width is not an exact multiple of the threshold
	if shareCount%subtreeRootThreshold != 0 {
		s++
	}

	// use a power of two equal to or larger than the multiple of the subtree
	// root threshold
	s = shares.RoundUpPowerOfTwo(s)

	// use the minimum of the subtree width and the min square size, this
	// gurarantees that a valid value is returned
	return min(s, BlobMinSquareSize(shareCount))
}

func min[T constraints.Integer](i, j T) T {
	if i < j {
		return i
	}
	return j
}
