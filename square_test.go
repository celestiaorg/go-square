package square_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v3"
	"github.com/celestiaorg/go-square/v3/internal/test"
	"github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/go-square/v3/tx"
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
		res, err := square.Size(tt.input)
		require.NoError(t, err, i)
		assert.Equal(t, tt.expect, res, i)
		assert.True(t, square.IsPowerOfTwo(res))
	}
}
