// Package square implements the logic to construct the original data square
// based on a list of transactions.
package square

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
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
			if builder.AppendBlobTx(blobTx) {
				blobTxs = append(blobTxs, txBytes)
			}
		} else {
			if builder.AppendTx(txBytes) {
				normalTxs = append(normalTxs, txBytes)
			}
		}
	}
	square, err := builder.Export()
	return square, append(normalTxs, blobTxs...), err
}

// validateTxOrdering validates that transactions are ordered correctly:
//  1. Normal transactions
//  2. Pay for blob transactions
//  3. Pay for fibre transactions
//
// Returns an error if the ordering is invalid.
func validateTxOrdering(txs [][]byte, handler PayForFibreHandler) error {
	seenBlobTx := false
	seenPayForFibreTx := false

	for idx, txBytes := range txs {
		_, isBlobTx, err := tx.UnmarshalBlobTx(txBytes)
		if err != nil && isBlobTx {
			return fmt.Errorf("unmarshalling blob tx at index %d: %w", idx, err)
		}

		if isBlobTx {
			seenBlobTx = true
			// Blob txs cannot come after PayForFibre txs
			if seenPayForFibreTx {
				return fmt.Errorf("blob tx at index %d cannot be appended after pay-for-fibre tx", idx)
			}
			continue
		}

		// Check if this is a PayForFibre transaction
		if isPayForFibre := handler.IsPayForFibreTx(txBytes); isPayForFibre {
			seenPayForFibreTx = true
			// PayForFibre txs can exist without blob txs, or come after blob txs
			// The ordering check for blob txs coming before pay-for-fibre txs
			// is handled in the blob tx section above
			continue
		}

		// Normal txs cannot come after blob txs or PayForFibre txs
		if seenBlobTx {
			return fmt.Errorf("normal tx at index %d cannot be appended after blob tx", idx)
		}
		if seenPayForFibreTx {
			return fmt.Errorf("normal tx at index %d cannot be appended after pay-for-fibre tx", idx)
		}
	}

	return nil
}

// Construct takes the exact list of ordered transactions and constructs a square, validating that
//   - transactions are ordered: normal txs, pay for blob txs, pay for fibre txs
//   - the transactions don't collectively exceed the maxSquareSize.
//
// The handler parameter is required and must not be nil. Use NoOpPayForFibreHandler() if
// PayForFibre support is not needed.
//
// Note that this function does not check the underlying validity of
// the transactions.
func Construct(txs [][]byte, maxSquareSize, subtreeRootThreshold int, handler PayForFibreHandler) (Square, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler must not be nil, use NoOpPayForFibreHandler() if PayForFibre support is not needed")
	}

	// Validate transaction ordering
	if err := validateTxOrdering(txs, handler); err != nil {
		return nil, err
	}

	builder, err := NewBuilder(maxSquareSize, subtreeRootThreshold)
	if err != nil {
		return nil, err
	}

	for idx, txBytes := range txs {
		blobTx, isBlobTx, err := tx.UnmarshalBlobTx(txBytes)
		if err != nil && isBlobTx {
			return nil, fmt.Errorf("unmarshalling blob tx at index %d: %w", idx, err)
		}
		if isBlobTx {
			if !builder.AppendBlobTx(blobTx) {
				return nil, fmt.Errorf("not enough space to append blob tx at index %d", idx)
			}
			continue
		}

		// Check if this is a PayForFibre transaction
		if isPayForFibre := handler.IsPayForFibreTx(txBytes); isPayForFibre {
			// Append the PayForFibre transaction
			if !builder.AppendPayForFibreTx(txBytes) {
				return nil, fmt.Errorf("not enough space to append pay-for-fibre tx at index %d", idx)
			}
			// Create and append the system blob
			systemBlob, err := handler.CreateSystemBlob(txBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to create system blob for pay-for-fibre tx at index %d: %w", idx, err)
			}
			if !builder.AppendSystemBlob(systemBlob) {
				return nil, fmt.Errorf("not enough space to append system blob for pay-for-fibre tx at index %d", idx)
			}
			continue
		}

		// Normal transaction
		if !builder.AppendTx(txBytes) {
			return nil, fmt.Errorf("not enough space to append tx at index %d", idx)
		}
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
func (s Square) Size() int {
	return Size(len(s))
}

// Size returns the size of the row or column in shares of a square. This
// function is currently a wrapper around the da packages equivalent function to
// avoid breaking the api. In future versions there will not be a copy of this
// code here.
func Size(length int) int {
	return RoundUpPowerOfTwo(int(math.Ceil(math.Sqrt(float64(length)))))
}

// RoundUpPowerOfTwo returns the next power of two greater than or equal to input.
func RoundUpPowerOfTwo[I constraints.Integer](input I) I {
	var result I = 1
	for result < input {
		result <<= 1
	}
	return result
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
	txWriter *share.CompactShareSplitter,
	pfbWriter *share.CompactShareSplitter,
	payForFibreWriter *share.CompactShareSplitter,
	blobWriter *share.SparseShareSplitter,
	nonReservedStart int,
	squareSize int,
) (Square, error) {
	totalShares := squareSize * squareSize
	pfbStartIndex := txWriter.Count()
	payForFibreStartIndex := pfbStartIndex + pfbWriter.Count()
	paddingStartIndex := payForFibreStartIndex + payForFibreWriter.Count()
	if nonReservedStart < paddingStartIndex {
		return nil, fmt.Errorf("nonReservedStart %d is too small to fit all txs, PayForBlob txs, and PayForFibre txs", nonReservedStart)
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

	payForFibreShares, err := payForFibreWriter.Export()
	if err != nil {
		return nil, fmt.Errorf("failed to export pay-for-fibre shares: %w", err)
	}

	square := make([]share.Share, totalShares)
	copy(square, txShares)
	copy(square[pfbStartIndex:], pfbShares)
	copy(square[payForFibreStartIndex:], payForFibreShares)
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

// PayForFibreHandler handles PayForFibre transaction identification and system blob creation.
// This interface allows go-square to handle PayForFibre transactions without depending on
// application-specific types.
type PayForFibreHandler interface {
	// IsPayForFibreTx returns true if the given transaction bytes represent a PayForFibre transaction.
	IsPayForFibreTx(tx []byte) bool
	// CreateSystemBlob creates a system blob from a PayForFibre transaction.
	// The system blob will be added to the square alongside the PayForFibre transaction.
	CreateSystemBlob(tx []byte) (*share.Blob, error)
}

// noOpPayForFibreHandler is a no-op implementation of PayForFibreHandler that never identifies
// PayForFibre transactions. Use this when PayForFibre support is not needed.
type noOpPayForFibreHandler struct{}

func (noOpPayForFibreHandler) IsPayForFibreTx([]byte) bool {
	return false
}

func (noOpPayForFibreHandler) CreateSystemBlob([]byte) (*share.Blob, error) {
	return nil, fmt.Errorf("no-op handler cannot create system blobs")
}

// NoOpPayForFibreHandler returns a no-op PayForFibreHandler that never identifies PayForFibre transactions.
// Use this when PayForFibre support is not needed.
func NoOpPayForFibreHandler() PayForFibreHandler {
	return noOpPayForFibreHandler{}
}
