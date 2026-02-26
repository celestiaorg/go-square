package square_test

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/celestiaorg/go-square/v3"
	"github.com/celestiaorg/go-square/v3/internal/test"
	"github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/go-square/v3/tx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilderSquareSizeEstimation(t *testing.T) {
	type test struct {
		name               string
		normalTxs          int
		pfbCount, pfbSize  int
		expectedSquareSize int
	}
	tests := []test{
		{"empty block", 0, 0, 0, 1},
		{"one normal tx", 1, 0, 0, 1},
		{"one small pfb small block", 0, 1, 100, 2},
		{"mixed small block", 10, 12, 500, 8},
		{"small block 2", 0, 12, 1000, 8},
		{"mixed medium block 2", 10, 20, 10000, 32},
		{"one large pfb large block", 0, 1, 1000000, 64},
		{"one hundred large pfb large block", 0, 100, 100000, 64},
		{"one hundred large pfb medium block", 100, 100, 100000, 64},
		{"mixed transactions large block", 100, 100, 100000, 64},
		{"mixed transactions large block 2", 1000, 1000, 10000, 64},
		{"mostly transactions large block", 10000, 1000, 100, 64},
		{"only small pfb large block", 0, 10000, 1, 64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txs := generateMixedTxs(tt.normalTxs, tt.pfbCount, 1, tt.pfbSize)
			square, _, err := square.Build(txs, 64, defaultSubtreeRootThreshold)
			require.NoError(t, err)
			size, err := square.Size()
			require.NoError(t, err)
			require.EqualValues(t, tt.expectedSquareSize, size)
		})
	}
}

func TestBuilderRejectsTransactions(t *testing.T) {
	builder, err := square.NewBuilder(2, 64) // 2 x 2 square
	require.NoError(t, err)
	require.False(t, builder.AppendTx(newTx(share.AvailableBytesFromCompactShares(4)+1)))
	require.True(t, builder.AppendTx(newTx(share.AvailableBytesFromCompactShares(4))))
	require.False(t, builder.AppendTx(newTx(1)))
}

func TestBuilderRejectsBlobTransactions(t *testing.T) {
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	testCases := []struct {
		blobSize []int
		added    bool
	}{
		{
			blobSize: []int{share.AvailableBytesFromSparseShares(3) + 1},
			added:    false,
		},
		{
			blobSize: []int{share.AvailableBytesFromSparseShares(3)},
			added:    true,
		},
		{
			blobSize: []int{share.AvailableBytesFromSparseShares(2) + 1, share.AvailableBytesFromSparseShares(1)},
			added:    false,
		},
		{
			blobSize: []int{share.AvailableBytesFromSparseShares(1), share.AvailableBytesFromSparseShares(1)},
			added:    true,
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("case%d", idx), func(t *testing.T) {
			builder, err := square.NewBuilder(2, 64)
			require.NoError(t, err)
			txs := generateBlobTxsWithNamespaces(ns1.Repeat(len(tc.blobSize)), [][]int{tc.blobSize})
			require.Len(t, txs, 1)
			blobTx, isBlobTx, err := tx.UnmarshalBlobTx(txs[0])
			require.NoError(t, err)
			require.True(t, isBlobTx)
			added, err := builder.AppendBlobTx(blobTx)
			require.NoError(t, err)
			require.Equal(t, tc.added, added)
		})
	}
}

func TestBuilderInvalidConstructor(t *testing.T) {
	_, err := square.NewBuilder(-4, 64)
	require.Error(t, err)
	_, err = square.NewBuilder(0, 64)
	require.Error(t, err)
	_, err = square.NewBuilder(13, 64)
	require.Error(t, err)
}

func newTx(length int) []byte {
	return bytes.Repeat([]byte{0}, length-test.DelimLen(uint64(length)))
}

func TestBuilderFindTxShareRange(t *testing.T) {
	blockTxs := generateOrderedTxs(5, 5, 1000, 10)
	require.Len(t, blockTxs, 10)

	builder, err := square.NewBuilder(128, 64, blockTxs...)
	require.NoError(t, err)

	dataSquare, err := builder.Export()
	require.NoError(t, err)
	dataSquareSize, err := dataSquare.Size()
	require.NoError(t, err)
	size := dataSquareSize * dataSquareSize

	var lastEnd int
	for idx, txBytes := range blockTxs {
		blobTx, isBlobTx, err := tx.UnmarshalBlobTx(txBytes)
		if isBlobTx {
			require.NoError(t, err)
			txBytes = blobTx.Tx
		}
		shareRange, err := builder.FindTxShareRange(idx)
		require.NoError(t, err)
		if idx == 5 {
			// normal txs and PFBs use a different namespace so there
			// can't be any overlap in the index
			require.Greater(t, shareRange.Start, lastEnd-1)
		} else {
			require.GreaterOrEqual(t, shareRange.Start, lastEnd-1)
		}
		require.LessOrEqual(t, shareRange.End, size)
		txShares := dataSquare[shareRange.Start : shareRange.End+1]
		parsedShares, err := rawData(txShares)
		require.NoError(t, err)
		require.True(t, bytes.Contains(parsedShares, txBytes))
		lastEnd = shareRange.End
	}
}

func rawData(shares []share.Share) ([]byte, error) {
	var data []byte
	for _, share := range shares {
		data = append(data, share.RawData()...)
	}
	return data, nil
}

// TestSquareBlobPositions ensures that the share commitment rules which dictate the padding
// between blobs is followed as well as the ordering of blobs by namespace.
func TestSquareBlobPostions(t *testing.T) {
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	ns2 := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, share.NamespaceVersionZeroIDSize))
	ns3 := share.MustNewV0Namespace(bytes.Repeat([]byte{3}, share.NamespaceVersionZeroIDSize))

	type testCase struct {
		squareSize      int
		blobTxs         [][]byte
		expectedIndexes [][]uint32
	}
	tests := []testCase{
		{
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1},
				[][]int{{1}},
			),
			expectedIndexes: [][]uint32{{1}},
		},
		{
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns1},
				test.Repeat([]int{100}, 2),
			),
			expectedIndexes: [][]uint32{{2}, {3}},
		},
		{
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns1, ns1, ns1, ns1, ns1, ns1, ns1, ns1},
				test.Repeat([]int{100}, 9),
			),
			expectedIndexes: [][]uint32{{7}, {8}, {9}, {10}, {11}, {12}, {13}, {14}, {15}},
		},
		{
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns1, ns1},
				[][]int{{10000}, {10000}, {1000000}},
			),
			expectedIndexes: [][]uint32{},
		},
		{
			squareSize: 64,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns1, ns1},
				[][]int{{1000}, {10000}, {10000}},
			),
			expectedIndexes: [][]uint32{{3}, {6}, {27}},
		},
		{
			squareSize: 32,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns2, ns1, ns1},
				[][]int{{100}, {100}, {100}},
			),
			expectedIndexes: [][]uint32{{5}, {3}, {4}},
		},
		{
			squareSize: 16,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns2, ns1},
				[][]int{{100}, {900}, {900}}, // 1, 2, 2 shares respectively
			),
			expectedIndexes: [][]uint32{{3}, {6}, {4}},
		},
		{
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns3, ns3, ns2},
				[][]int{{100}, {1000, 1000}, {420}},
			),
			expectedIndexes: [][]uint32{{3}, {5, 8}, {4}},
		},
		{
			// no blob txs should make it in the square
			squareSize: 1,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns2, ns3},
				[][]int{{1000}, {1000}, {1000}},
			),
			expectedIndexes: [][]uint32{},
		},
		{
			// only two blob txs should make it in the square (after reordering)
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns3, ns2, ns1},
				[][]int{{2000}, {2000}, {5000}},
			),
			expectedIndexes: [][]uint32{{7}, {2}},
		},
		{
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns3, ns3, ns2, ns1},
				[][]int{{1800, 1000}, {22000}, {1800}},
			),
			// should be ns1 and {ns3, ns3} as ns2 is too large
			expectedIndexes: [][]uint32{{6, 10}, {2}},
		},
		{
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns3, ns3, ns1, ns2, ns2},
				[][]int{{100}, {1400, 900, 200, 200}, {420}},
			),
			expectedIndexes: [][]uint32{{3}, {7, 10, 4, 5}, {6}},
		},
		{
			squareSize: 4,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns3, ns3, ns1, ns2, ns2},
				[][]int{{100}, {900, 1400, 200, 200}, {420}},
			),
			expectedIndexes: [][]uint32{{3}, {7, 9, 4, 5}, {6}},
		},
		{
			squareSize: 16,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns1},
				[][]int{{100}, {share.AvailableBytesFromSparseShares(64)}},
			),
			// There should be one share padding between the two blobs
			expectedIndexes: [][]uint32{{2}, {3}},
		},
		{
			squareSize: 16,
			blobTxs: generateBlobTxsWithNamespaces(
				[]share.Namespace{ns1, ns1},
				[][]int{{100}, {share.AvailableBytesFromSparseShares(64) + 1}},
			),
			// There should be one share padding between the two blobs
			expectedIndexes: [][]uint32{{2}, {4}},
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			builder, err := square.NewBuilder(tt.squareSize, defaultSubtreeRootThreshold)
			require.NoError(t, err)
			for _, txBytes := range tt.blobTxs {
				blobTx, isBlobTx, err := tx.UnmarshalBlobTx(txBytes)
				require.NoError(t, err)
				require.True(t, isBlobTx)
				_, err = builder.AppendBlobTx(blobTx)
				require.NoError(t, err)
			}
			square, err := builder.Export()
			require.NoError(t, err)
			txs, err := share.ParseTxs(square)
			require.NoError(t, err)
			for j, rawTx := range txs {
				wrappedPFB, isWrappedPFB := tx.UnmarshalIndexWrapper(rawTx)
				assert.True(t, isWrappedPFB)
				assert.Equal(t, tt.expectedIndexes[j], wrappedPFB.ShareIndexes, j)
			}
		})
	}
}

func generateMixedTxs(normalTxCount, pfbCount, blobsPerPfb, blobSize int) [][]byte {
	return shuffle(generateOrderedTxs(normalTxCount, pfbCount, blobsPerPfb, blobSize))
}

func generateOrderedTxs(normalTxCount, pfbCount, blobsPerPfb, blobSize int) [][]byte {
	pfbTxs := test.GenerateBlobTxs(pfbCount, blobsPerPfb, blobSize)
	normieTxs := test.GenerateTxs(200, 400, normalTxCount)
	return append(normieTxs, pfbTxs...)
}

func shuffle(slice [][]byte) [][]byte {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
	return slice
}

func generateBlobTxsWithNamespaces(namespaces []share.Namespace, blobSizes [][]int) [][]byte {
	txs := make([][]byte, len(blobSizes))
	counter := 0
	for i := 0; i < len(txs); i++ {
		n := namespaces[counter : counter+len(blobSizes[i])]
		txs[i] = test.GenerateBlobTxWithNamespace(n, blobSizes[i], share.ShareVersionZero)
		counter += len(blobSizes[i])
	}
	return txs
}

type block struct {
	Txs [][]byte `protobuf:"bytes,1,rep,name=txs,proto3" json:"txs,omitempty"`
}

// TestBigBlock indirectly verifies that the worst case share padding
// calculation is computed using the rules on celestia-app v1.x. This test does
// so by using a big_block.json file generated via celestia-app v1.x and then
// verifies the share index of a particular blob. Note: if worst case share
// padding is modified then we expect this test to fail and need a new valid
// testdata/big_block.json.
//
// https://github.com/celestiaorg/go-square/issues/47
func TestBigBlock(t *testing.T) {
	bigBlock := block{}
	err := json.Unmarshal([]byte(bigBlockJSON), &bigBlock)
	require.NoError(t, err)

	builder, err := square.NewBuilder(defaultMaxSquareSize, defaultSubtreeRootThreshold, bigBlock.Txs...)
	require.NoError(t, err)
	assert.Len(t, builder.Blobs, 84)
	assert.Len(t, builder.Pfbs, 25)

	index, err := builder.FindBlobStartingIndex(0, 0)
	require.NoError(t, err)
	assert.Equal(t, 2234, index)
}

//go:embed "internal/testdata/big_block.json"
var bigBlockJSON string

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected bool
	}{
		{-1, false}, // negative numbers should return false
		{0, false},  // zero should return false
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{8, true},
		{16, true},
		{32, true},
		{64, true},
		{128, true},
		{256, true},
		{512, true},
		{1024, true},
	}
	for _, test := range tests {
		assert.Equal(t, test.expected, square.IsPowerOfTwo(test.input))
	}
}

func TestBuilderRevertLastTx(t *testing.T) {
	builder, err := square.NewBuilder(8, 64)
	require.NoError(t, err)

	// Test reverting when there are no transactions
	err = builder.RevertLastTx()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no transactions to revert")

	// Add a transaction and verify it was added
	tx1 := newTx(100)
	require.True(t, builder.AppendTx(tx1))
	require.Len(t, builder.Txs, 1)
	require.Equal(t, tx1, builder.Txs[0])
	initialSize := builder.CurrentSize()
	require.Greater(t, initialSize, 0)

	// Revert the transaction
	err = builder.RevertLastTx()
	require.NoError(t, err)
	require.Len(t, builder.Txs, 0)
	require.Equal(t, 0, builder.CurrentSize())

	// Test reverting when there are no transactions left
	err = builder.RevertLastTx()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no transactions to revert")

	// Add multiple transactions and revert only the last one
	tx2 := newTx(50)
	tx3 := newTx(75)
	require.True(t, builder.AppendTx(tx2))
	sizeAfterOneTx := builder.CurrentSize()
	require.True(t, builder.AppendTx(tx3))
	require.Len(t, builder.Txs, 2)

	err = builder.RevertLastTx()
	require.NoError(t, err)
	require.Len(t, builder.Txs, 1)
	require.Equal(t, tx2, builder.Txs[0])
	require.Equal(t, sizeAfterOneTx, builder.CurrentSize())
	require.Greater(t, builder.CurrentSize(), 0)
}

func TestBuilderRevertLastBlobTx(t *testing.T) {
	builder, err := square.NewBuilder(64, 64)
	require.NoError(t, err)

	// Test reverting when there are no blob transactions
	err = builder.RevertLastBlobTx()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no blob transactions to revert")

	// Add a blob transaction and verify it was added
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	blobTxs := generateBlobTxsWithNamespaces([]share.Namespace{ns1}, [][]int{{100}})
	require.Len(t, blobTxs, 1)

	blobTx, isBlobTx, err := tx.UnmarshalBlobTx(blobTxs[0])
	require.NoError(t, err)
	require.True(t, isBlobTx)

	added, err := builder.AppendBlobTx(blobTx)
	require.NoError(t, err)
	require.True(t, added)
	require.Len(t, builder.Pfbs, 1)
	require.Len(t, builder.Blobs, 1)
	initialSize := builder.CurrentSize()
	require.Greater(t, initialSize, 0)

	// Revert the blob transaction
	err = builder.RevertLastBlobTx()
	require.NoError(t, err)
	require.Len(t, builder.Pfbs, 0)
	require.Len(t, builder.Blobs, 0)
	require.Equal(t, 0, builder.CurrentSize())

	// Test reverting when there are no blob transactions left
	err = builder.RevertLastBlobTx()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no blob transactions to revert")
}

func TestBuilderRevertLastBlobTxWithMultipleBlobs(t *testing.T) {
	builder, err := square.NewBuilder(64, 64)
	require.NoError(t, err)

	// Create a blob transaction with multiple blobs
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	ns2 := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, share.NamespaceVersionZeroIDSize))
	blobTxs := generateBlobTxsWithNamespaces([]share.Namespace{ns1, ns2}, [][]int{{100, 150}})
	require.Len(t, blobTxs, 1)

	blobTx, isBlobTx, err := tx.UnmarshalBlobTx(blobTxs[0])
	require.NoError(t, err)
	require.True(t, isBlobTx)
	require.Len(t, blobTx.Blobs, 2)

	added, err := builder.AppendBlobTx(blobTx)
	require.NoError(t, err)
	require.True(t, added)
	require.Len(t, builder.Pfbs, 1)
	require.Len(t, builder.Blobs, 2) // Should have 2 blobs

	// Add another single blob transaction
	blobTxs2 := generateBlobTxsWithNamespaces([]share.Namespace{ns1}, [][]int{{200}})
	blobTx2, isBlobTx2, err := tx.UnmarshalBlobTx(blobTxs2[0])
	require.NoError(t, err)
	require.True(t, isBlobTx2)

	added, err = builder.AppendBlobTx(blobTx2)
	require.NoError(t, err)
	require.True(t, added)
	require.Len(t, builder.Pfbs, 2)
	require.Len(t, builder.Blobs, 3) // Should have 3 blobs total

	// Revert the last blob transaction (which had 1 blob)
	err = builder.RevertLastBlobTx()
	require.NoError(t, err)
	require.Len(t, builder.Pfbs, 1)
	require.Len(t, builder.Blobs, 2) // Should be back to 2 blobs

	// Try to revert the first blob transaction - this should return error
	// because the last blob tx has already been reverted
	err = builder.RevertLastBlobTx()
	require.Error(t, err)
	require.Contains(t, err.Error(), "already been reverted")
	require.Len(t, builder.Pfbs, 1)  // Should remain at 1
	require.Len(t, builder.Blobs, 2) // Should remain at 2 blobs
}

func TestBuilderRevertMixed(t *testing.T) {
	builder, err := square.NewBuilder(64, 64)
	require.NoError(t, err)

	// Add a regular transaction
	tx1 := newTx(100)
	require.True(t, builder.AppendTx(tx1))

	// Add a blob transaction
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	blobTxs := generateBlobTxsWithNamespaces([]share.Namespace{ns1}, [][]int{{100}})
	blobTx, isBlobTx, err := tx.UnmarshalBlobTx(blobTxs[0])
	require.NoError(t, err)
	require.True(t, isBlobTx)
	added, err := builder.AppendBlobTx(blobTx)
	require.NoError(t, err)
	require.True(t, added)

	// Verify state
	require.Len(t, builder.Txs, 1)
	require.Len(t, builder.Pfbs, 1)
	require.Len(t, builder.Blobs, 1)

	// Revert blob transaction - should not affect regular transaction
	err = builder.RevertLastBlobTx()
	require.NoError(t, err)
	require.Len(t, builder.Txs, 1)
	require.Len(t, builder.Pfbs, 0)
	require.Len(t, builder.Blobs, 0)

	// Regular transaction should still be there
	require.Equal(t, tx1, builder.Txs[0])

	// Should not be able to revert blob tx when there are none
	err = builder.RevertLastBlobTx()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no blob transactions to revert")

	// Should still be able to revert the regular transaction
	err = builder.RevertLastTx()
	require.NoError(t, err)
	require.Len(t, builder.Txs, 0)
}

// TestConsecutiveRevertCalls demonstrates that consecutive revert calls are prevented
// by returning errors
func TestConsecutiveRevertCalls(t *testing.T) {
	builder, err := square.NewBuilder(64, 64)
	require.NoError(t, err)

	// Add two transactions to demonstrate the footgun
	tx1 := make([]byte, 50)
	tx2 := make([]byte, 50)

	require.True(t, builder.AppendTx(tx1))
	require.True(t, builder.AppendTx(tx2))
	require.Len(t, builder.Txs, 2)

	sizeAfterTwoTxs := builder.CurrentSize()
	require.Greater(t, sizeAfterTwoTxs, 0)

	// First revert should work
	err = builder.RevertLastTx()
	require.NoError(t, err)
	require.Len(t, builder.Txs, 1)

	sizeAfterFirstRevert := builder.CurrentSize()
	t.Logf("Size after first revert: %d (was %d)", sizeAfterFirstRevert, sizeAfterTwoTxs)

	// Second revert should now return error due to counter limitation prevention
	err = builder.RevertLastTx()
	require.Error(t, err, "Second revert should return error due to counter limitation")
	require.Contains(t, err.Error(), "already been reverted")
	require.Len(t, builder.Txs, 1) // Should remain at 1

	// Size should remain consistent
	sizeAfterSecondRevert := builder.CurrentSize()

	t.Logf("Size after second revert: %d (was %d)", sizeAfterSecondRevert, sizeAfterFirstRevert)

	// Size should remain the same since the revert was prevented
	require.Equal(t, sizeAfterFirstRevert, sizeAfterSecondRevert)
}

// TestMultipleRevertBlobTxs demonstrates that consecutive revert calls are prevented for blob transactions
func TestMultipleRevertBlobTxs(t *testing.T) {
	builder, err := square.NewBuilder(64, 64)
	require.NoError(t, err)

	// Add two blob transactions
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	ns2 := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, share.NamespaceVersionZeroIDSize))

	blobTxs1 := generateBlobTxsWithNamespaces([]share.Namespace{ns1}, [][]int{{100}})
	blobTx1, _, err := tx.UnmarshalBlobTx(blobTxs1[0])
	require.NoError(t, err)

	blobTxs2 := generateBlobTxsWithNamespaces([]share.Namespace{ns2}, [][]int{{100}})
	blobTx2, _, err := tx.UnmarshalBlobTx(blobTxs2[0])
	require.NoError(t, err)

	added, err := builder.AppendBlobTx(blobTx1)
	require.NoError(t, err)
	require.True(t, added)
	added, err = builder.AppendBlobTx(blobTx2)
	require.NoError(t, err)
	require.True(t, added)
	require.Len(t, builder.Pfbs, 2)

	sizeAfterTwoBlobTxs := builder.CurrentSize()
	require.Greater(t, sizeAfterTwoBlobTxs, 0)

	// First revert should work
	err = builder.RevertLastBlobTx()
	require.NoError(t, err)
	require.Len(t, builder.Pfbs, 1)

	sizeAfterFirstRevert := builder.CurrentSize()
	t.Logf("Size after first revert: %d (was %d)", sizeAfterFirstRevert, sizeAfterTwoBlobTxs)

	// Second revert should now return error due to counter limitation prevention
	err = builder.RevertLastBlobTx()
	require.Error(t, err, "Second revert should return error due to counter limitation")
	require.Contains(t, err.Error(), "already been reverted")
	require.Len(t, builder.Pfbs, 1)  // Should remain at 1
	require.Len(t, builder.Blobs, 1) // Should remain at 1 (only blobTx1's blob)

	// Size should remain consistent
	sizeAfterSecondRevert := builder.CurrentSize()

	t.Logf("Size after second revert: %d (was %d)", sizeAfterSecondRevert, sizeAfterFirstRevert)

	// Size should remain the same since the revert was prevented
	require.Equal(t, sizeAfterFirstRevert, sizeAfterSecondRevert)
}

// TestRevertAfterNewAdd demonstrates that you can revert again after adding a new transaction
func TestRevertAfterNewAdd(t *testing.T) {
	builder, err := square.NewBuilder(64, 64)
	require.NoError(t, err)

	// Add and revert a transaction
	tx1 := make([]byte, 50)
	require.True(t, builder.AppendTx(tx1))
	err = builder.RevertLastTx()
	require.NoError(t, err)

	// Try to revert again - should return error
	err = builder.RevertLastTx()
	require.Error(t, err)
	// After reverting, there are no transactions left, so we get this error instead of "already been reverted"
	require.Contains(t, err.Error(), "no transactions to revert")

	// Add a new transaction
	tx2 := make([]byte, 50)
	require.True(t, builder.AppendTx(tx2))

	// Now revert should work again
	err = builder.RevertLastTx()
	require.NoError(t, err)
	require.Len(t, builder.Txs, 0)

	// Same test for blob transactions
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	blobTxs1 := generateBlobTxsWithNamespaces([]share.Namespace{ns1}, [][]int{{100}})
	blobTx1, _, err := tx.UnmarshalBlobTx(blobTxs1[0])
	require.NoError(t, err)

	added, err := builder.AppendBlobTx(blobTx1)
	require.NoError(t, err)
	require.True(t, added)
	err = builder.RevertLastBlobTx()
	require.NoError(t, err)

	// Try to revert again - should return error
	err = builder.RevertLastBlobTx()
	require.Error(t, err)
	// After reverting, there are no blob transactions left, so we get this error instead of "already been reverted"
	require.Contains(t, err.Error(), "no blob transactions to revert")

	// Add a new blob transaction
	blobTxs2 := generateBlobTxsWithNamespaces([]share.Namespace{ns1}, [][]int{{200}})
	blobTx2, _, err := tx.UnmarshalBlobTx(blobTxs2[0])
	require.NoError(t, err)

	added, err = builder.AppendBlobTx(blobTx2)
	require.NoError(t, err)
	require.True(t, added)

	// Now revert should work again
	err = builder.RevertLastBlobTx()
	require.NoError(t, err)
	require.Len(t, builder.Pfbs, 0)
}

// TestArabicaSquareHash is a test that verifies that the square built from the
// Arabica block 8122437 has the correct hash. This is an attempt to catch
// future regressions that break square construction.
func TestArabicaSquareHash(t *testing.T) {
	arabicaTxs := loadArabicaTxs(t)

	currentSquare, _, err := square.Build(arabicaTxs, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.NoError(t, err)

	want := [32]uint8([32]uint8{0x18, 0x80, 0xb0, 0xe7, 0x7b, 0x46, 0x84, 0xcb, 0xc, 0xb, 0x33, 0x1b, 0xe3, 0xc9, 0xf9, 0x9f, 0x15, 0x7, 0x93, 0x3e, 0x5, 0xa1, 0x35, 0x2c, 0xdb, 0xaa, 0xba, 0xb3, 0x4e, 0x8f, 0xc0, 0x3f})
	got := currentSquare.Hash()
	require.Equal(t, want, got)
}

// loadArabicaTxs loads the transaction data from Arabica block 8122437.
func loadArabicaTxs(t *testing.T) [][]byte {
	txsJSON, err := os.ReadFile("testdata/arabica_8122437_txs.json")
	require.NoError(t, err)

	var txsBase64 []string
	err = json.Unmarshal(txsJSON, &txsBase64)
	require.NoError(t, err)

	txs := make([][]byte, len(txsBase64))
	for i, txBase64 := range txsBase64 {
		txBytes, err := base64.StdEncoding.DecodeString(txBase64)
		require.NoError(t, err)
		txs[i] = txBytes
	}

	return txs
}
