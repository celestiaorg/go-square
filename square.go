// Package square implements the logic to construct the original data square
// based on a list of transactions.
package square

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"math/bits"

	"github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/go-square/v3/tx"
	"golang.org/x/exp/constraints"
)

// Build takes an arbitrary long list of (prioritized) transactions and builds a square that is never
// greater than maxSquareSize. It also returns the ordered list of transactions that are present
// in the square and which have all PFBs trailing regular transactions. Note, this function does
// not check the underlying validity of the transactions.
// Errors should not occur and would reflect a violation in an invariant.
func Build(txs [][]byte, maxSquareSize, subtreeRootThreshold int) (Square, [][]byte, error) {
	builder, err := NewBuilder(maxSquareSize, subtreeRootThreshold)
	if err != nil {
		return nil, nil, err
	}
	normalTxs := make([][]byte, 0, len(txs))
	blobTxs := make([][]byte, 0, len(txs))
	for idx, txBytes := range txs {
		blobTx, isBlobTx, err := tx.UnmarshalBlobTx(txBytes)
		if err != nil && isBlobTx {
			return nil, nil, fmt.Errorf("unmarshalling blob tx at index %d: %w", idx, err)
		}
		if isBlobTx {
			appended, err := builder.AppendBlobTx(blobTx)
			if err != nil {
				return nil, nil, fmt.Errorf("appending blob tx at index %d: %w", idx, err)
			}
			if appended {
				blobTxs = append(blobTxs, txBytes)
			}
		} else if builder.AppendTx(txBytes) {
			normalTxs = append(normalTxs, txBytes)
		}
	}
	square, err := builder.Export()
	return square, append(normalTxs, blobTxs...), err
}

// Construct takes the exact list of ordered transactions and constructs a square, validating that
//   - all blobTxs are ordered after non-blob transactions
//   - the transactions don't collectively exceed the maxSquareSize.
//
// Note that this function does not check the underlying validity of
// the transactions.
func Construct(txs [][]byte, maxSquareSize, subtreeRootThreshold int) (Square, error) {
	builder, err := NewBuilder(maxSquareSize, subtreeRootThreshold, txs...)
	if err != nil {
		return nil, err
	}
	return builder.Export()
}

// TxShareRange returns the range of share indexes that the tx, specified by txIndex, occupies.
// The range is end exclusive.
func TxShareRange(txs [][]byte, txIndex, maxSquareSize, subtreeRootThreshold int) (share.Range, error) {
	builder, err := NewBuilder(maxSquareSize, subtreeRootThreshold, txs...)
	if err != nil {
		return share.Range{}, err
	}

	return builder.FindTxShareRange(txIndex)
}

// BlobShareRange returns the range of share indexes that the blob, identified by txIndex and blobIndex, occupies.
// The range is end exclusive.
func BlobShareRange(txs [][]byte, txIndex, blobIndex, maxSquareSize, subtreeRootThreshold int) (share.Range, error) {
	builder, err := NewBuilder(maxSquareSize, subtreeRootThreshold, txs...)
	if err != nil {
		return share.Range{}, err
	}

	start, err := builder.FindBlobStartingIndex(txIndex, blobIndex)
	if err != nil {
		return share.Range{}, err
	}

	blobLen, err := builder.BlobShareLength(txIndex, blobIndex)
	if err != nil {
		return share.Range{}, err
	}
	end := start + blobLen

	return share.NewRange(start, end), nil
}

// Square is a 2D square of shares with symmetrical sides that are always a power of 2.
type Square []share.Share

// Size returns the size of the sides of a square
func (s Square) Size() (int, error) {
	return Size(len(s))
}

// Size returns the size of the row or column in shares of a square. This
// function is currently a wrapper around the da packages equivalent function to
// avoid breaking the api. In future versions there will not be a copy of this
// code here.
func Size(length int) (int, error) {
	return RoundUpPowerOfTwo(int(math.Ceil(math.Sqrt(float64(length)))))
}

// RoundUpPowerOfTwo returns the next power of two greater than or equal to input.
func RoundUpPowerOfTwo[I constraints.Integer](input I) (I, error) {
	if input <= 1 {
		return 1, nil
	}
	if input&(input-1) == 0 {
		return input, nil
	}
	result := I(1) << bits.Len64(uint64(input))
	if result <= 0 {
		return 0, fmt.Errorf("cannot round up %v: result overflows %T", input, input)
	}
	return result, nil
}

// Equals returns true if two squares are equal
func (s Square) Equals(other Square) bool {
	if len(s) != len(other) {
		return false
	}
	for i := range s {
		if !bytes.Equal(s[i].ToBytes(), other[i].ToBytes()) {
			return false
		}
	}
	return true
}

// WrappedPFBs returns the wrapped PFBs in a square
func (s Square) WrappedPFBs() ([][]byte, error) {
	wpfbShareRange := share.GetShareRangeForNamespace(s, share.PayForBlobNamespace)
	if wpfbShareRange.IsEmpty() {
		return [][]byte{}, nil
	}
	return share.ParseTxs(s[wpfbShareRange.Start:wpfbShareRange.End])
}

func (s Square) IsEmpty() bool {
	return s.Equals(EmptySquare())
}

// ToBytes returns all the shares in the square flattened into a single byte
// slice.
func (s Square) ToBytes() []byte {
	result := []byte{}
	for _, share := range s {
		result = append(result, share.ToBytes()...)
	}
	return result
}

// Hash returns a hash based on the shares in the square.
func (s Square) Hash() [32]byte {
	return sha256.Sum256(s.ToBytes())
}

// EmptySquare returns a 1x1 square with a single tail padding share
func EmptySquare() Square {
	return share.TailPaddingShares(share.MinShareCount)
}

func WriteSquare(
	txWriter, pfbWriter *share.CompactShareSplitter,
	blobWriter *share.SparseShareSplitter,
	nonReservedStart, squareSize int,
) (Square, error) {
	totalShares := squareSize * squareSize
	pfbStartIndex := txWriter.Count()
	paddingStartIndex := pfbStartIndex + pfbWriter.Count()
	if nonReservedStart < paddingStartIndex {
		return nil, fmt.Errorf("nonReservedStart %d is too small to fit all PFBs and txs", nonReservedStart)
	}
	padding := share.ReservedPaddingShares(nonReservedStart - paddingStartIndex)
	endOfLastBlob := nonReservedStart + blobWriter.Count()
	if totalShares < endOfLastBlob {
		return nil, fmt.Errorf("square size %d is too small to fit all blobs", totalShares)
	}

	txShares, err := txWriter.Export()
	if err != nil {
		return nil, fmt.Errorf("failed to export tx shares: %w", err)
	}

	pfbShares, err := pfbWriter.Export()
	if err != nil {
		return nil, fmt.Errorf("failed to export pfb shares: %w", err)
	}

	square := make([]share.Share, totalShares)
	copy(square, txShares)
	copy(square[pfbStartIndex:], pfbShares)
	if blobWriter.Count() > 0 {
		copy(square[paddingStartIndex:], padding)
		copy(square[nonReservedStart:], blobWriter.Export())
	}
	if totalShares > endOfLastBlob {
		copy(square[endOfLastBlob:], share.TailPaddingShares(totalShares-endOfLastBlob))
	}

	return square, nil
}

type PFBDecoder func(txBytes []byte) ([]uint32, error)
