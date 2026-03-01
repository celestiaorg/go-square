package square_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/internal/test"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mebibyte                    = 1_048_576 // one mebibyte in bytes
	defaultMaxSquareSize        = 128
	defaultSubtreeRootThreshold = 64
)

func TestSquareConstruction(t *testing.T) {
	sendTxs := test.GenerateTxs(250, 250, 250)
	pfbTxs := test.GenerateBlobTxs(10_000, 1, 1024)
	t.Run("normal transactions after PFB transactions", func(t *testing.T) {
		txs := sendTxs[:5]
		txs = append(txs, append(pfbTxs, txs...)...)
		_, err := square.Construct(txs, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.Error(t, err)
	})
	t.Run("not enough space to append transactions", func(t *testing.T) {
		_, err := square.Construct(sendTxs, 2, defaultSubtreeRootThreshold)
		require.Error(t, err)
		_, err = square.Construct(pfbTxs, 2, defaultSubtreeRootThreshold)
		require.Error(t, err)
	})
	t.Run("construction should fail if a single PFB tx contains a blob that is too large to fit in the square", func(t *testing.T) {
		pfbTxs := test.GenerateBlobTxs(1, 1, 2*mebibyte)
		_, err := square.Construct(pfbTxs, 64, defaultSubtreeRootThreshold)
		require.Error(t, err)
	})
	t.Run("validates transaction ordering: normal -> blob -> pay-for-fibre", func(t *testing.T) {
		normalTx := sendTxs[0]
		blobTx := pfbTxs[0]
		payForFibreTx := newFibreTxBytes(t)

		// Valid ordering: normal -> blob -> pay-for-fibre
		validTxs := [][]byte{normalTx, blobTx, payForFibreTx}
		_, err := square.Construct(validTxs, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.NoError(t, err)

		// Invalid: blob after pay-for-fibre (blob must come before pay-for-fibre if both exist)
		invalidTxs1 := [][]byte{normalTx, payForFibreTx, blobTx}
		_, err = square.Construct(invalidTxs1, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be appended after pay-for-fibre tx")

		// Invalid: blob before normal
		invalidTxs2 := [][]byte{blobTx, normalTx}
		_, err = square.Construct(invalidTxs2, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be appended after blob tx")

		// Invalid: normal after pay-for-fibre (will report blob tx error first since normal tx comes after blob tx)
		invalidTxs3 := [][]byte{blobTx, payForFibreTx, normalTx}
		_, err = square.Construct(invalidTxs3, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be appended after blob tx")
	})
}

func newFibreTxBytes(t *testing.T) []byte {
	t.Helper()
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAA}, share.SignerSize)
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	systemBlob, err := share.NewV2Blob(ns, 1, commitment, signer)
	require.NoError(t, err)
	fibreTxBytes, err := tx.MarshalFibreTx([]byte("pay-for-fibre-sdk-tx"), systemBlob)
	require.NoError(t, err)
	return fibreTxBytes
}

func TestBuild(t *testing.T) {
	t.Run("empty input returns empty square", func(t *testing.T) {
		s, txs, err := square.Build(nil, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.NoError(t, err)
		require.True(t, s.IsEmpty())
		require.Empty(t, txs)
	})
	t.Run("normal txs only", func(t *testing.T) {
		normalTxs := test.GenerateTxs(100, 200, 3)
		s, includedTxs, err := square.Build(normalTxs, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.NoError(t, err)
		require.False(t, s.IsEmpty())
		require.Len(t, includedTxs, 3)
		// all included txs should match the input
		for i, tx := range includedTxs {
			require.Equal(t, normalTxs[i], tx)
		}
	})
	t.Run("blob txs only", func(t *testing.T) {
		blobTxs := test.GenerateBlobTxs(3, 1, 1024)
		s, includedTxs, err := square.Build(blobTxs, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.NoError(t, err)
		require.False(t, s.IsEmpty())
		require.Len(t, includedTxs, 3)
	})
	t.Run("fibre txs only", func(t *testing.T) {
		fibreTxs := [][]byte{newFibreTxBytes(t), newFibreTxBytes(t)}
		s, includedTxs, err := square.Build(fibreTxs, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.NoError(t, err)
		require.False(t, s.IsEmpty())
		require.Len(t, includedTxs, 2)
	})
	t.Run("mixed tx types in any order", func(t *testing.T) {
		normalTx := test.GenerateTxs(100, 100, 1)[0]
		blobTx := test.GenerateBlobTxs(1, 1, 1024)[0]
		fibreTx := newFibreTxBytes(t)

		// Build accepts txs in any order (unlike Construct)
		input := [][]byte{fibreTx, normalTx, blobTx}
		s, includedTxs, err := square.Build(input, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.NoError(t, err)
		require.False(t, s.IsEmpty())
		require.Len(t, includedTxs, 3)

		// Return order is always: normal, blob, fibre
		require.Equal(t, normalTx, includedTxs[0], "normal tx should be first")
		require.Equal(t, blobTx, includedTxs[1], "blob tx should be second")
		require.Equal(t, fibreTx, includedTxs[2], "fibre tx should be third")
	})
	t.Run("drops txs that do not fit", func(t *testing.T) {
		// Use a tiny square (2x2 = 4 shares) to force dropping
		smallSquareSize := 2
		largeTxs := test.GenerateTxs(2000, 2000, 3)
		s, includedTxs, err := square.Build(largeTxs, smallSquareSize, defaultSubtreeRootThreshold)
		require.NoError(t, err)
		// Some txs should be dropped because they don't fit
		require.Less(t, len(includedTxs), len(largeTxs))
		_ = s
	})
	t.Run("square contains all three namespace types", func(t *testing.T) {
		normalTx := test.GenerateTxs(100, 100, 1)[0]
		blobTx := test.GenerateBlobTxs(1, 1, 1024)[0]
		fibreTx := newFibreTxBytes(t)

		s, _, err := square.Build([][]byte{normalTx, blobTx, fibreTx}, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.NoError(t, err)

		txRange := share.GetShareRangeForNamespace(s, share.TxNamespace)
		pfbRange := share.GetShareRangeForNamespace(s, share.PayForBlobNamespace)
		pffRange := share.GetShareRangeForNamespace(s, share.PayForFibreNamespace)

		require.False(t, txRange.IsEmpty(), "should have tx shares")
		require.False(t, pfbRange.IsEmpty(), "should have pfb shares")
		require.False(t, pffRange.IsEmpty(), "should have pay-for-fibre shares")

		// Verify ordering: tx < pfb < pff
		require.LessOrEqual(t, txRange.End, pfbRange.Start)
		require.LessOrEqual(t, pfbRange.End, pffRange.Start)
	})
}

func TestValidateTxOrdering(t *testing.T) {
	// Create test transactions
	normalTx1 := newTx(100)
	normalTx2 := newTx(100)
	blobTx1 := test.GenerateBlobTxs(1, 1, 1024)[0]
	blobTx2 := test.GenerateBlobTxs(1, 1, 1024)[0]
	payForFibreTx1 := newFibreTxBytes(t)
	payForFibreTx2 := newFibreTxBytes(t)

	tests := []struct {
		name          string
		txs           [][]byte
		wantError     bool
		errorContains string
	}{
		{
			name:      "empty list - valid",
			txs:       [][]byte{},
			wantError: false,
		},
		{
			name:      "only normal txs - valid",
			txs:       [][]byte{normalTx1, normalTx2},
			wantError: false,
		},
		{
			name:      "only blob txs - valid",
			txs:       [][]byte{blobTx1, blobTx2},
			wantError: false,
		},
		{
			name:      "only pay-for-fibre txs - valid",
			txs:       [][]byte{payForFibreTx1, payForFibreTx2},
			wantError: false,
		},
		{
			name:      "normal -> blob - valid",
			txs:       [][]byte{normalTx1, normalTx2, blobTx1, blobTx2},
			wantError: false,
		},
		{
			name:      "normal -> blob -> pay-for-fibre - valid",
			txs:       [][]byte{normalTx1, blobTx1, payForFibreTx1},
			wantError: false,
		},
		{
			name:      "blob -> pay-for-fibre - valid",
			txs:       [][]byte{blobTx1, payForFibreTx1},
			wantError: false,
		},
		{
			name:      "normal -> pay-for-fibre (no blob) - valid",
			txs:       [][]byte{normalTx1, payForFibreTx1},
			wantError: false,
		},
		{
			name:          "pay-for-fibre -> blob - invalid (blob must come before pay-for-fibre)",
			txs:           [][]byte{payForFibreTx1, blobTx1},
			wantError:     true,
			errorContains: "cannot be appended after pay-for-fibre tx",
		},
		{
			name:          "blob -> normal - invalid",
			txs:           [][]byte{blobTx1, normalTx1},
			wantError:     true,
			errorContains: "cannot be appended after blob tx",
		},
		{
			name:          "blob -> pay-for-fibre -> normal - invalid",
			txs:           [][]byte{blobTx1, payForFibreTx1, normalTx1},
			wantError:     true,
			errorContains: "cannot be appended after",
		},
		{
			name:          "normal -> blob -> pay-for-fibre -> normal - invalid",
			txs:           [][]byte{normalTx1, blobTx1, payForFibreTx1, normalTx2},
			wantError:     true,
			errorContains: "cannot be appended after",
		},
		{
			name:          "blob -> blob -> pay-for-fibre -> blob - invalid",
			txs:           [][]byte{blobTx1, blobTx2, payForFibreTx1, blobTx1},
			wantError:     true,
			errorContains: "cannot be appended after pay-for-fibre tx",
		},
		{
			name:      "normal -> normal -> blob -> blob -> pay-for-fibre -> pay-for-fibre - valid",
			txs:       [][]byte{normalTx1, normalTx2, blobTx1, blobTx2, payForFibreTx1, payForFibreTx2},
			wantError: false,
		},
		{
			name:          "normal -> pay-for-fibre -> blob - invalid (blob after pay-for-fibre)",
			txs:           [][]byte{normalTx1, payForFibreTx1, blobTx1},
			wantError:     true,
			errorContains: "cannot be appended after pay-for-fibre tx",
		},
		{
			name:          "pay-for-fibre -> normal - invalid (normal after pay-for-fibre)",
			txs:           [][]byte{payForFibreTx1, normalTx1},
			wantError:     true,
			errorContains: "cannot be appended after pay-for-fibre tx",
		},
		{
			name:          "multiple sequences mixed - invalid (normal after blob)",
			txs:           [][]byte{normalTx1, blobTx1, payForFibreTx1, normalTx2, blobTx2, payForFibreTx2},
			wantError:     true,
			errorContains: "cannot be appended after blob tx",
		},
		{
			name:      "normal -> pay-for-fibre (no blob) with multiple pay-for-fibre - valid",
			txs:       [][]byte{normalTx1, payForFibreTx1, payForFibreTx2},
			wantError: false,
		},
		{
			name:      "single normal tx - valid",
			txs:       [][]byte{normalTx1},
			wantError: false,
		},
		{
			name:      "single blob tx - valid",
			txs:       [][]byte{blobTx1},
			wantError: false,
		},
		{
			name:      "single pay-for-fibre tx (no blob) - valid",
			txs:       [][]byte{payForFibreTx1},
			wantError: false,
		},
		{
			name:      "normal -> pay-for-fibre -> pay-for-fibre (no blob) - valid",
			txs:       [][]byte{normalTx1, payForFibreTx1, payForFibreTx2},
			wantError: false,
		},
		{
			name:      "blob -> pay-for-fibre -> pay-for-fibre - valid",
			txs:       [][]byte{blobTx1, payForFibreTx1, payForFibreTx2},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test through Construct which calls validateTxOrdering
			// Use a large square size to avoid space-related errors
			_, err := square.Construct(tt.txs, defaultMaxSquareSize, defaultSubtreeRootThreshold)

			if tt.wantError {
				require.Error(t, err, "expected error but got none")
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains, "error message should contain expected text")
				}
			} else {
				require.NoError(t, err, "expected no error but got: %v", err)
			}
		})
	}
}

func TestSquareTxShareRange(t *testing.T) {
	type test struct {
		name      string
		txs       [][]byte
		index     int
		wantStart int
		wantEnd   int
		expectErr bool
	}

	txOne := []byte{0x1}
	txTwo := bytes.Repeat([]byte{2}, 600)
	txThree := bytes.Repeat([]byte{3}, 1000)

	testCases := []test{
		{
			name:      "txOne occupies shares 0 to 0",
			txs:       [][]byte{txOne},
			index:     0,
			wantStart: 0,
			wantEnd:   1,
			expectErr: false,
		},
		{
			name:      "txTwo occupies shares 0 to 1",
			txs:       [][]byte{txTwo},
			index:     0,
			wantStart: 0,
			wantEnd:   2,
			expectErr: false,
		},
		{
			name:      "txThree occupies shares 0 to 2",
			txs:       [][]byte{txThree},
			index:     0,
			wantStart: 0,
			wantEnd:   3,
			expectErr: false,
		},
		{
			name:      "txThree occupies shares 1 to 3",
			txs:       [][]byte{txOne, txTwo, txThree},
			index:     2,
			wantStart: 1,
			wantEnd:   4,
			expectErr: false,
		},
		{
			name:      "invalid index",
			txs:       [][]byte{txOne, txTwo, txThree},
			index:     3,
			wantStart: 0,
			wantEnd:   0,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shareRange, err := square.TxShareRange(tc.txs, tc.index, 128, 64)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantStart, shareRange.Start)
			require.Equal(t, tc.wantEnd, shareRange.End)
		})
	}
}

func TestSquareTxShareRangeWithPayForFibre(t *testing.T) {
	normalTx := newTx(100)
	payForFibreTx := newFibreTxBytes(t)

	// Build a tx list: normal tx, then PayForFibre tx
	txs := [][]byte{normalTx, payForFibreTx}

	// Verify the normal tx share range is valid
	normalRange, err := square.TxShareRange(txs, 0, 128, 64)
	require.NoError(t, err)
	require.Equal(t, 0, normalRange.Start)
	require.Greater(t, normalRange.End, normalRange.Start)

	// Verify the PayForFibre tx share range is valid (index 1)
	pffRange, err := square.TxShareRange(txs, 1, 128, 64)
	require.NoError(t, err)
	require.Greater(t, pffRange.End, pffRange.Start)

	// PayForFibre tx shares should start at or after normal tx shares end
	// (they are in different namespaces so no overlap)
	require.GreaterOrEqual(t, pffRange.Start, normalRange.End-1)

	// Out of bounds index should error
	_, err = square.TxShareRange(txs, 2, 128, 64)
	require.Error(t, err)
}

func TestSquareBlobShareRange(t *testing.T) {
	txs := test.GenerateBlobTxs(10, 1, 1024)

	builder, err := square.NewBuilder(defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.NoError(t, err)
	for idx, txBytes := range txs {
		blobTx, isBlobTx, err := tx.UnmarshalBlobTx(txBytes)
		require.NoError(t, err)
		require.True(t, isBlobTx)
		added, err := builder.AppendBlobTx(blobTx)
		require.NoError(t, err)
		require.True(t, added, "not enough space to append blob tx at index %d", idx)
	}

	dataSquare, err := builder.Export()
	require.NoError(t, err)

	for pfbIdx, txBytes := range txs {
		blobTx, isBlobTx, err := tx.UnmarshalBlobTx(txBytes)
		require.NoError(t, err)
		require.True(t, isBlobTx)
		for blobIdx := range blobTx.Blobs {
			shareRange, err := square.BlobShareRange(txs, pfbIdx, blobIdx, defaultMaxSquareSize, defaultSubtreeRootThreshold)
			require.NoError(t, err)
			require.LessOrEqual(t, shareRange.End, len(dataSquare))
			blobShares := dataSquare[shareRange.Start:shareRange.End]
			blobSharesBytes, err := rawData(blobShares)
			require.NoError(t, err)
			require.True(t, bytes.Contains(blobSharesBytes, blobTx.Blobs[blobIdx].Data()))
		}
	}

	// error on out of bounds cases
	_, err = square.BlobShareRange(txs, -1, 0, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.Error(t, err)

	_, err = square.BlobShareRange(txs, 0, -1, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.Error(t, err)

	_, err = square.BlobShareRange(txs, 10, 0, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.Error(t, err)

	_, err = square.BlobShareRange(txs, 0, 10, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.Error(t, err)
}

func TestSize(t *testing.T) {
	type test struct {
		input  int
		expect int
	}
	tests := []test{
		{input: 0, expect: share.MinSquareSize},
		{input: 1, expect: share.MinSquareSize},
		{input: 64, expect: 8},
		{input: 100, expect: 16},
		{input: 1000, expect: 32},
		{input: defaultMaxSquareSize * defaultMaxSquareSize, expect: defaultMaxSquareSize},
		{input: defaultMaxSquareSize*defaultMaxSquareSize + 1, expect: defaultMaxSquareSize * 2},
	}
	for i, tt := range tests {
		res := square.Size(tt.input)
		assert.Equal(t, tt.expect, res, i)
		assert.True(t, square.IsPowerOfTwo(res))
	}
}

func TestWriteSquare(t *testing.T) {
	t.Run("writes transactions in correct order", func(t *testing.T) {
		// Create writers for each transaction type
		txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
		pfbWriter := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
		payForFibreWriter := share.NewCompactShareSplitter(share.PayForFibreNamespace, share.ShareVersionZero)
		blobWriter := share.NewSparseShareSplitter()

		// Add some transactions
		tx1 := newTx(100)
		tx2 := newTx(100)
		pfbTx := newTx(100)
		payForFibreTx := newTx(100)

		require.NoError(t, txWriter.WriteTx(tx1))
		require.NoError(t, txWriter.WriteTx(tx2))
		require.NoError(t, pfbWriter.WriteTx(pfbTx))
		require.NoError(t, payForFibreWriter.WriteTx(payForFibreTx))

		// Calculate nonReservedStart (after all compact shares)
		nonReservedStart := txWriter.Count() + pfbWriter.Count() + payForFibreWriter.Count()
		squareSize := 8 // 8x8 square

		// Write the square
		s, err := square.WriteSquare(txWriter, pfbWriter, payForFibreWriter, blobWriter, nonReservedStart, squareSize)
		require.NoError(t, err)
		require.NotNil(t, s)

		// Verify the order: TxNamespace → PayForBlobNamespace → PayForFibreNamespace
		txShareRange := share.GetShareRangeForNamespace(s, share.TxNamespace)
		pfbShareRange := share.GetShareRangeForNamespace(s, share.PayForBlobNamespace)
		payForFibreShareRange := share.GetShareRangeForNamespace(s, share.PayForFibreNamespace)

		require.False(t, txShareRange.IsEmpty())
		require.False(t, pfbShareRange.IsEmpty())
		require.False(t, payForFibreShareRange.IsEmpty())

		// Verify ordering
		require.LessOrEqual(t, txShareRange.End, pfbShareRange.Start, "TxNamespace should come before PayForBlobNamespace")
		require.LessOrEqual(t, pfbShareRange.End, payForFibreShareRange.Start, "PayForBlobNamespace should come before PayForFibreNamespace")

		// Verify transactions can be parsed from their respective namespaces
		txShares := s[txShareRange.Start:txShareRange.End]
		txTxs, err := share.ParseTxs(txShares)
		require.NoError(t, err)
		require.Len(t, txTxs, 2)

		pfbShares := s[pfbShareRange.Start:pfbShareRange.End]
		pfbTxs, err := share.ParseTxs(pfbShares)
		require.NoError(t, err)
		require.Len(t, pfbTxs, 1)

		payForFibreShares := s[payForFibreShareRange.Start:payForFibreShareRange.End]
		payForFibreTxs, err := share.ParseTxs(payForFibreShares)
		require.NoError(t, err)
		require.Len(t, payForFibreTxs, 1)
	})

	t.Run("handles empty pay-for-fibre writer", func(t *testing.T) {
		txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
		pfbWriter := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
		payForFibreWriter := share.NewCompactShareSplitter(share.PayForFibreNamespace, share.ShareVersionZero)
		blobWriter := share.NewSparseShareSplitter()

		tx1 := newTx(100)
		require.NoError(t, txWriter.WriteTx(tx1))

		nonReservedStart := txWriter.Count() + pfbWriter.Count() + payForFibreWriter.Count()
		squareSize := 8

		s, err := square.WriteSquare(txWriter, pfbWriter, payForFibreWriter, blobWriter, nonReservedStart, squareSize)
		require.NoError(t, err)
		require.NotNil(t, s)

		// PayForFibreNamespace should be empty
		payForFibreShareRange := share.GetShareRangeForNamespace(s, share.PayForFibreNamespace)
		require.True(t, payForFibreShareRange.IsEmpty(), "PayForFibreNamespace should be empty when no pay-for-fibre transactions")
	})

	t.Run("handles empty pfb writer", func(t *testing.T) {
		txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
		pfbWriter := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
		payForFibreWriter := share.NewCompactShareSplitter(share.PayForFibreNamespace, share.ShareVersionZero)
		blobWriter := share.NewSparseShareSplitter()

		tx1 := newTx(100)
		payForFibreTx := newTx(100)

		require.NoError(t, txWriter.WriteTx(tx1))
		require.NoError(t, payForFibreWriter.WriteTx(payForFibreTx))

		nonReservedStart := txWriter.Count() + pfbWriter.Count() + payForFibreWriter.Count()
		squareSize := 8

		s, err := square.WriteSquare(txWriter, pfbWriter, payForFibreWriter, blobWriter, nonReservedStart, squareSize)
		require.NoError(t, err)
		require.NotNil(t, s)

		// PayForBlobNamespace should be empty
		pfbShareRange := share.GetShareRangeForNamespace(s, share.PayForBlobNamespace)
		require.True(t, pfbShareRange.IsEmpty(), "PayForBlobNamespace should be empty when no PFB transactions")
	})

	t.Run("handles blobs", func(t *testing.T) {
		txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
		pfbWriter := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
		payForFibreWriter := share.NewCompactShareSplitter(share.PayForFibreNamespace, share.ShareVersionZero)
		blobWriter := share.NewSparseShareSplitter()

		tx1 := newTx(100)
		require.NoError(t, txWriter.WriteTx(tx1))

		// Add a blob
		ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
		blob, err := share.NewBlob(ns1, []byte("test blob data"), share.ShareVersionZero, nil)
		require.NoError(t, err)
		require.NoError(t, blobWriter.Write(blob))

		nonReservedStart := txWriter.Count() + pfbWriter.Count() + payForFibreWriter.Count()
		squareSize := 8

		s, err := square.WriteSquare(txWriter, pfbWriter, payForFibreWriter, blobWriter, nonReservedStart, squareSize)
		require.NoError(t, err)
		require.NotNil(t, s)

		// Verify blob is present
		blobShareRange := share.GetShareRangeForNamespace(s, ns1)
		require.False(t, blobShareRange.IsEmpty(), "Blob should be present in the square")
	})

	t.Run("returns error when nonReservedStart is too small", func(t *testing.T) {
		txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
		pfbWriter := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
		payForFibreWriter := share.NewCompactShareSplitter(share.PayForFibreNamespace, share.ShareVersionZero)
		blobWriter := share.NewSparseShareSplitter()

		tx1 := newTx(100)
		pfbTx := newTx(100)
		payForFibreTx := newTx(100)

		require.NoError(t, txWriter.WriteTx(tx1))
		require.NoError(t, pfbWriter.WriteTx(pfbTx))
		require.NoError(t, payForFibreWriter.WriteTx(payForFibreTx))

		// Set nonReservedStart too small (before all compact shares end)
		nonReservedStart := txWriter.Count() + pfbWriter.Count() // Missing payForFibreWriter.Count()
		squareSize := 8

		_, err := square.WriteSquare(txWriter, pfbWriter, payForFibreWriter, blobWriter, nonReservedStart, squareSize)
		require.Error(t, err)
		require.Contains(t, err.Error(), "nonReservedStart")
		require.Contains(t, err.Error(), "PayForFibre")
	})

	t.Run("returns error when square size is too small for blobs", func(t *testing.T) {
		txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
		pfbWriter := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
		payForFibreWriter := share.NewCompactShareSplitter(share.PayForFibreNamespace, share.ShareVersionZero)
		blobWriter := share.NewSparseShareSplitter()

		tx1 := newTx(100)
		require.NoError(t, txWriter.WriteTx(tx1))

		// Add a blob that's too large for the square
		ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
		largeBlobData := bytes.Repeat([]byte{1}, 10000) // Large blob
		blob, err := share.NewBlob(ns1, largeBlobData, share.ShareVersionZero, nil)
		require.NoError(t, err)
		require.NoError(t, blobWriter.Write(blob))

		nonReservedStart := txWriter.Count() + pfbWriter.Count() + payForFibreWriter.Count()
		squareSize := 2 // 2x2 square, too small

		_, err = square.WriteSquare(txWriter, pfbWriter, payForFibreWriter, blobWriter, nonReservedStart, squareSize)
		require.Error(t, err)
		require.Contains(t, err.Error(), "square size")
		require.Contains(t, err.Error(), "too small")
	})

	t.Run("correctly calculates start indices", func(t *testing.T) {
		txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
		pfbWriter := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
		payForFibreWriter := share.NewCompactShareSplitter(share.PayForFibreNamespace, share.ShareVersionZero)
		blobWriter := share.NewSparseShareSplitter()

		tx1 := newTx(100)
		pfbTx := newTx(100)
		payForFibreTx := newTx(100)

		require.NoError(t, txWriter.WriteTx(tx1))
		require.NoError(t, pfbWriter.WriteTx(pfbTx))
		require.NoError(t, payForFibreWriter.WriteTx(payForFibreTx))

		nonReservedStart := txWriter.Count() + pfbWriter.Count() + payForFibreWriter.Count()
		squareSize := 8

		s, err := square.WriteSquare(txWriter, pfbWriter, payForFibreWriter, blobWriter, nonReservedStart, squareSize)
		require.NoError(t, err)

		// Verify start indices match expectations
		txShareRange := share.GetShareRangeForNamespace(s, share.TxNamespace)
		pfbShareRange := share.GetShareRangeForNamespace(s, share.PayForBlobNamespace)
		payForFibreShareRange := share.GetShareRangeForNamespace(s, share.PayForFibreNamespace)

		require.Equal(t, 0, txShareRange.Start, "TxNamespace should start at 0")
		require.Equal(t, txWriter.Count(), pfbShareRange.Start, "PayForBlobNamespace should start after TxNamespace")
		require.Equal(t, txWriter.Count()+pfbWriter.Count(), payForFibreShareRange.Start, "PayForFibreNamespace should start after PayForBlobNamespace")
	})
}
