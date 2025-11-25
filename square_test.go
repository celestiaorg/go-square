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
	t.Run("nil handler returns error", func(t *testing.T) {
		txs := sendTxs[:5]
		_, err := square.Construct(txs, defaultMaxSquareSize, defaultSubtreeRootThreshold, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "handler must not be nil")
	})
	t.Run("normal transactions after PFB transactions", func(t *testing.T) {
		txs := sendTxs[:5]
		txs = append(txs, append(pfbTxs, txs...)...)
		_, err := square.Construct(txs, defaultMaxSquareSize, defaultSubtreeRootThreshold, square.NoOpPayForFibreHandler())
		require.Error(t, err)
	})
	t.Run("not enough space to append transactions", func(t *testing.T) {
		_, err := square.Construct(sendTxs, 2, defaultSubtreeRootThreshold, square.NoOpPayForFibreHandler())
		require.Error(t, err)
		_, err = square.Construct(pfbTxs, 2, defaultSubtreeRootThreshold, square.NoOpPayForFibreHandler())
		require.Error(t, err)
	})
	t.Run("construction should fail if a single PFB tx contains a blob that is too large to fit in the square", func(t *testing.T) {
		pfbTxs := test.GenerateBlobTxs(1, 1, 2*mebibyte)
		_, err := square.Construct(pfbTxs, 64, defaultSubtreeRootThreshold, square.NoOpPayForFibreHandler())
		require.Error(t, err)
	})
	t.Run("validates transaction ordering: normal -> blob -> pay-for-fibre", func(t *testing.T) {
		// Create a mock handler that identifies specific tx bytes as PayForFibre
		normalTx := sendTxs[0]
		blobTx := pfbTxs[0]
		payForFibreTx := []byte("pay-for-fibre-tx")

		mockHandler := &mockPayForFibreHandler{
			payForFibreTxs: map[string]bool{
				string(payForFibreTx): true,
			},
		}

		// Valid ordering: normal -> blob -> pay-for-fibre
		validTxs := [][]byte{normalTx, blobTx, payForFibreTx}
		_, err := square.Construct(validTxs, defaultMaxSquareSize, defaultSubtreeRootThreshold, mockHandler)
		require.NoError(t, err)

		// Invalid: blob after pay-for-fibre (blob must come before pay-for-fibre if both exist)
		invalidTxs1 := [][]byte{normalTx, payForFibreTx, blobTx}
		_, err = square.Construct(invalidTxs1, defaultMaxSquareSize, defaultSubtreeRootThreshold, mockHandler)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be appended after pay-for-fibre tx")

		// Invalid: blob before normal
		invalidTxs2 := [][]byte{blobTx, normalTx}
		_, err = square.Construct(invalidTxs2, defaultMaxSquareSize, defaultSubtreeRootThreshold, mockHandler)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be appended after blob tx")

		// Invalid: normal after pay-for-fibre (will report blob tx error first since normal tx comes after blob tx)
		invalidTxs3 := [][]byte{blobTx, payForFibreTx, normalTx}
		_, err = square.Construct(invalidTxs3, defaultMaxSquareSize, defaultSubtreeRootThreshold, mockHandler)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot be appended after blob tx")
	})
}

// mockPayForFibreHandler is a test helper that identifies specific transactions as PayForFibre
type mockPayForFibreHandler struct {
	payForFibreTxs map[string]bool
}

func (m *mockPayForFibreHandler) IsPayForFibreTx(tx []byte) bool {
	return m.payForFibreTxs[string(tx)]
}

func (m *mockPayForFibreHandler) CreateSystemBlob(_ []byte) (*share.Blob, error) {
	ns := share.MustNewV0Namespace([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	return share.NewBlob(ns, []byte("system blob"), share.ShareVersionZero, nil)
}

func TestValidateTxOrdering(t *testing.T) {
	// Create test transactions
	normalTx1 := newTx(100)
	normalTx2 := newTx(100)
	blobTx1 := test.GenerateBlobTxs(1, 1, 1024)[0]
	blobTx2 := test.GenerateBlobTxs(1, 1, 1024)[0]
	payForFibreTx1 := []byte("pay-for-fibre-tx-1")
	payForFibreTx2 := []byte("pay-for-fibre-tx-2")

	mockHandler := &mockPayForFibreHandler{
		payForFibreTxs: map[string]bool{
			string(payForFibreTx1): true,
			string(payForFibreTx2): true,
		},
	}

	tests := []struct {
		name          string
		txs           [][]byte
		handler       square.PayForFibreHandler
		wantError     bool
		errorContains string
	}{
		{
			name:      "empty list - valid",
			txs:       [][]byte{},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "only normal txs - valid",
			txs:       [][]byte{normalTx1, normalTx2},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "only blob txs - valid",
			txs:       [][]byte{blobTx1, blobTx2},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "only pay-for-fibre txs - valid",
			txs:       [][]byte{payForFibreTx1, payForFibreTx2},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "normal -> blob - valid",
			txs:       [][]byte{normalTx1, normalTx2, blobTx1, blobTx2},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "normal -> blob -> pay-for-fibre - valid",
			txs:       [][]byte{normalTx1, blobTx1, payForFibreTx1},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "blob -> pay-for-fibre - valid",
			txs:       [][]byte{blobTx1, payForFibreTx1},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "normal -> pay-for-fibre (no blob) - valid",
			txs:       [][]byte{normalTx1, payForFibreTx1},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:          "pay-for-fibre -> blob - invalid (blob must come before pay-for-fibre)",
			txs:           [][]byte{payForFibreTx1, blobTx1},
			handler:       mockHandler,
			wantError:     true,
			errorContains: "cannot be appended after pay-for-fibre tx",
		},
		{
			name:          "blob -> normal - invalid",
			txs:           [][]byte{blobTx1, normalTx1},
			handler:       mockHandler,
			wantError:     true,
			errorContains: "cannot be appended after blob tx",
		},
		{
			name:          "blob -> pay-for-fibre -> normal - invalid",
			txs:           [][]byte{blobTx1, payForFibreTx1, normalTx1},
			handler:       mockHandler,
			wantError:     true,
			errorContains: "cannot be appended after",
		},
		{
			name:          "normal -> blob -> pay-for-fibre -> normal - invalid",
			txs:           [][]byte{normalTx1, blobTx1, payForFibreTx1, normalTx2},
			handler:       mockHandler,
			wantError:     true,
			errorContains: "cannot be appended after",
		},
		{
			name:          "blob -> blob -> pay-for-fibre -> blob - invalid",
			txs:           [][]byte{blobTx1, blobTx2, payForFibreTx1, blobTx1},
			handler:       mockHandler,
			wantError:     true,
			errorContains: "cannot be appended after pay-for-fibre tx",
		},
		{
			name:      "normal -> normal -> blob -> blob -> pay-for-fibre -> pay-for-fibre - valid",
			txs:       [][]byte{normalTx1, normalTx2, blobTx1, blobTx2, payForFibreTx1, payForFibreTx2},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:          "normal -> pay-for-fibre -> blob - invalid (blob after pay-for-fibre)",
			txs:           [][]byte{normalTx1, payForFibreTx1, blobTx1},
			handler:       mockHandler,
			wantError:     true,
			errorContains: "cannot be appended after pay-for-fibre tx",
		},
		{
			name:          "pay-for-fibre -> normal - invalid (normal after pay-for-fibre)",
			txs:           [][]byte{payForFibreTx1, normalTx1},
			handler:       mockHandler,
			wantError:     true,
			errorContains: "cannot be appended after pay-for-fibre tx",
		},
		{
			name:          "multiple sequences mixed - invalid (normal after blob)",
			txs:           [][]byte{normalTx1, blobTx1, payForFibreTx1, normalTx2, blobTx2, payForFibreTx2},
			handler:       mockHandler,
			wantError:     true,
			errorContains: "cannot be appended after blob tx",
		},
		{
			name:      "normal -> pay-for-fibre (no blob) with multiple pay-for-fibre - valid",
			txs:       [][]byte{normalTx1, payForFibreTx1, payForFibreTx2},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "single normal tx - valid",
			txs:       [][]byte{normalTx1},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "single blob tx - valid",
			txs:       [][]byte{blobTx1},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "single pay-for-fibre tx (no blob) - valid",
			txs:       [][]byte{payForFibreTx1},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "normal -> pay-for-fibre -> pay-for-fibre (no blob) - valid",
			txs:       [][]byte{normalTx1, payForFibreTx1, payForFibreTx2},
			handler:   mockHandler,
			wantError: false,
		},
		{
			name:      "blob -> pay-for-fibre -> pay-for-fibre - valid",
			txs:       [][]byte{blobTx1, payForFibreTx1, payForFibreTx2},
			handler:   mockHandler,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test through Construct which calls validateTxOrdering
			// Use a large square size to avoid space-related errors
			_, err := square.Construct(tt.txs, defaultMaxSquareSize, defaultSubtreeRootThreshold, tt.handler)

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

func TestSquareBlobShareRange(t *testing.T) {
	txs := test.GenerateBlobTxs(10, 1, 1024)

	builder, err := square.NewBuilder(defaultMaxSquareSize, defaultSubtreeRootThreshold, txs...)
	require.NoError(t, err)

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
