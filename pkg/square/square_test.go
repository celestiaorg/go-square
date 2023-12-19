package square_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/pkg/shares"
	"github.com/celestiaorg/go-square/pkg/square"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mebibyte                    = 1_048_576 // one mebibyte in bytes
	defaultMaxSquareSize        = 128
	defaultSubtreeRootThreshold = 64
)

// func TestSquareConstruction(t *testing.T) {
// 	rand := tmrand.NewRand()
// 	signer, err := testnode.NewOfflineSigner()
// 	require.NoError(t, err)
// 	sendTxs := blobfactory.GenerateManyRawSendTxs(signer, 250)
// 	pfbTxs := blobfactory.RandBlobTxs(signer, rand, 10000, 1, 1024)
// 	t.Run("normal transactions after PFB transactions", func(t *testing.T) {
// 		txs := append(sendTxs[:5], append(pfbTxs, sendTxs[5:]...)...)
// 		_, err := square.Construct(txs, LatestVersion, defaultMaxSquareSize)
// 		require.Error(t, err)
// 	})
// 	t.Run("not enough space to append transactions", func(t *testing.T) {
// 		_, err := square.Construct(sendTxs, LatestVersion, 2)
// 		require.Error(t, err)
// 		_, err = square.Construct(pfbTxs, LatestVersion, 2)
// 		require.Error(t, err)
// 	})
// 	t.Run("construction should fail if a single PFB tx contains a blob that is too large to fit in the square", func(t *testing.T) {
// 		pfbTxs := blobfactory.RandBlobTxs(signer, rand, 1, 1, 2*mebibyte)
// 		_, err := square.Construct(pfbTxs, LatestVersion, 64)
// 		require.Error(t, err)
// 	})
// }

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

// func TestSquareBlobShareRange(t *testing.T) {
// 	signer, err := testnode.NewOfflineSigner()
// 	require.NoError(t, err)
// 	txs := blobfactory.RandBlobTxsRandomlySized(signer, tmrand.NewRand(), 10, 1000, 10).ToSliceOfBytes()

// 	builder, err := square.NewBuilder(defaultMaxSquareSize, defaultSubtreeRootThreshold, txs...)
// 	require.NoError(t, err)

// 	dataSquare, err := builder.Export()
// 	require.NoError(t, err)

// 	for pfbIdx, tx := range txs {
// 		blobTx, isBlobTx := blob.UnmarshalBlobTx(tx)
// 		require.True(t, isBlobTx)
// 		for blobIdx := range blobTx.Blobs {
// 			shareRange, err := square.BlobShareRange(txs, pfbIdx, blobIdx, defaultMaxSquareSize, defaultSubtreeRootThreshold)
// 			require.NoError(t, err)
// 			require.LessOrEqual(t, shareRange.End, len(dataSquare))
// 			blobShares := dataSquare[shareRange.Start:shareRange.End]
// 			blobSharesBytes, err := rawData(blobShares)
// 			require.NoError(t, err)
// 			require.True(t, bytes.Contains(blobSharesBytes, blobTx.Blobs[blobIdx].Data))
// 		}
// 	}

// 	// error on out of bounds cases
// 	_, err = square.BlobShareRange(txs, -1, 0, defaultMaxSquareSize, defaultSubtreeRootThreshold)
// 	require.Error(t, err)

// 	_, err = square.BlobShareRange(txs, 0, -1, defaultMaxSquareSize, defaultSubtreeRootThreshold)
// 	require.Error(t, err)

// 	_, err = square.BlobShareRange(txs, 10, 0, defaultMaxSquareSize, defaultSubtreeRootThreshold)
// 	require.Error(t, err)

// 	_, err = square.BlobShareRange(txs, 0, 10, defaultMaxSquareSize, defaultSubtreeRootThreshold)
// 	require.Error(t, err)
// }

// func TestSquareDeconstruct(t *testing.T) {
// 	rand := tmrand.NewRand()
// 	encCfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)
// 	t.Run("ConstructDeconstructParity", func(t *testing.T) {
// 		// 8192 -> square size 128
// 		for _, numTxs := range []int{2, 128, 1024, 8192} {
// 			t.Run(fmt.Sprintf("%d", numTxs), func(t *testing.T) {
// 				signer, err := testnode.NewOfflineSigner()
// 				require.NoError(t, err)
// 				txs := generateOrderedTxs(signer, rand, numTxs/2, numTxs/2, 1, 800)
// 				dataSquare, err := square.Construct(txs, LatestVersion, defaultMaxSquareSize)
// 				require.NoError(t, err)
// 				recomputedTxs, err := square.Deconstruct(dataSquare, encCfg.TxConfig.TxDecoder())
// 				require.NoError(t, err)
// 				require.Equal(t, txs, recomputedTxs)
// 			})
// 		}
// 	})
// 	t.Run("NoPFBs", func(t *testing.T) {
// 		const numTxs = 10
// 		signer, err := testnode.NewOfflineSigner()
// 		require.NoError(t, err)
// 		txs := coretypes.Txs(blobfactory.GenerateManyRawSendTxs(signer, numTxs)).ToSliceOfBytes()
// 		dataSquare, err := square.Construct(txs, LatestVersion, defaultMaxSquareSize)
// 		require.NoError(t, err)
// 		recomputedTxs, err := square.Deconstruct(dataSquare, encCfg.TxConfig.TxDecoder())
// 		require.NoError(t, err)
// 		require.Equal(t, txs, recomputedTxs)
// 	})
// 	t.Run("PFBsOnly", func(t *testing.T) {
// 		signer, err := testnode.NewOfflineSigner()
// 		require.NoError(t, err)
// 		txs := blobfactory.RandBlobTxs(signer, rand, 100, 1, 1024).ToSliceOfBytes()
// 		dataSquare, err := square.Construct(txs, LatestVersion, defaultMaxSquareSize)
// 		require.NoError(t, err)
// 		recomputedTxs, err := square.Deconstruct(dataSquare, encCfg.TxConfig.TxDecoder())
// 		require.NoError(t, err)
// 		require.Equal(t, txs, recomputedTxs)
// 	})
// 	t.Run("EmptySquare", func(t *testing.T) {
// 		tx, err := square.Deconstruct(square.EmptySquare(), encCfg.TxConfig.TxDecoder())
// 		require.NoError(t, err)
// 		require.Equal(t, coretypes.Txs{}, tx)
// 	})
// }

// func TestSquareShareCommitments(t *testing.T) {
// 	const numTxs = 10
// 	rand := tmrand.NewRand()
// 	signer, err := testnode.NewOfflineSigner()
// 	require.NoError(t, err)
// 	txs := generateOrderedTxs(signer, rand, numTxs, numTxs, 3, 800)
// 	builder, err := square.NewBuilder(defaultMaxSquareSize, LatestVersion, txs...)
// 	require.NoError(t, err)

// 	dataSquare, err := builder.Export()
// 	require.NoError(t, err)

// 	cacher := inclusion.NewSubtreeCacher(uint64(dataSquare.Size()))
// 	eds, err := rsmt2d.ComputeExtendedDataSquare(shares.ToBytes(dataSquare), DefaultCodec(), cacher.Constructor)
// 	require.NoError(t, err)
// 	dah, err := da.NewDataAvailabilityHeader(eds)
// 	require.NoError(t, err)
// 	decoder := encoding.MakeConfig(app.ModuleEncodingRegisters...).TxConfig.TxDecoder()

// 	for pfbIndex := 0; pfbIndex < numTxs; pfbIndex++ {
// 		wpfb, err := builder.GetWrappedPFB(pfbIndex + numTxs)
// 		require.NoError(t, err)
// 		tx, err := decoder(wpfb.Tx)
// 		require.NoError(t, err)

// 		pfb, ok := tx.GetMsgs()[0].(*blobtypes.MsgPayForBlobs)
// 		require.True(t, ok)

// 		for blobIndex, shareIndex := range wpfb.ShareIndexes {
// 			commitment, err := inclusion.GetCommitment(cacher, dah, int(shareIndex), shares.SparseSharesNeeded(pfb.BlobSizes[blobIndex]), DefaultSubtreeRootThreshold)
// 			require.NoError(t, err)
// 			require.Equal(t, pfb.ShareCommitments[blobIndex], commitment)
// 		}
// 	}
// }

func TestSize(t *testing.T) {
	type test struct {
		input  int
		expect int
	}
	tests := []test{
		{input: 0, expect: shares.MinSquareSize},
		{input: 1, expect: shares.MinSquareSize},
		{input: 64, expect: 8},
		{input: 100, expect: 16},
		{input: 1000, expect: 32},
		{input: defaultMaxSquareSize * defaultMaxSquareSize, expect: defaultMaxSquareSize},
		{input: defaultMaxSquareSize*defaultMaxSquareSize + 1, expect: defaultMaxSquareSize * 2},
	}
	for i, tt := range tests {
		res := square.Size(tt.input)
		assert.Equal(t, tt.expect, res, i)
		assert.True(t, shares.IsPowerOfTwo(res))
	}
}
