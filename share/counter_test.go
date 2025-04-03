package share_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/celestiaorg/go-square/v2/share"
	"github.com/stretchr/testify/require"
)

func TestCounterMatchesCompactShareSplitter(t *testing.T) {
	testCases := []struct {
		txs [][]byte
	}{
		{txs: [][]byte{}},
		{txs: [][]byte{newTx(120)}},
		{txs: [][]byte{newTx(share.FirstCompactShareContentSize - 2)}},
		{txs: [][]byte{newTx(share.FirstCompactShareContentSize - 1)}},
		{txs: [][]byte{newTx(share.FirstCompactShareContentSize)}},
		{txs: [][]byte{newTx(share.FirstCompactShareContentSize + 1)}},
		{txs: [][]byte{newTx(share.FirstCompactShareContentSize), newTx(share.ContinuationCompactShareContentSize - 4)}},
		{txs: newTxs(1000, 100)},
		{txs: newTxs(100, 1000)},
		{txs: newTxs(8931, 77)},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("case%d", idx), func(t *testing.T) {
			writer := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
			counter := share.NewCompactShareCounter()

			sum := 0
			for _, tx := range tc.txs {
				require.NoError(t, writer.WriteTx(tx))
				diff := counter.Add(len(tx))
				require.Equal(t, writer.Count()-sum, diff)
				sum = writer.Count()
				require.Equal(t, sum, counter.Size())
			}
			shares, err := writer.Export()
			require.NoError(t, err)
			require.Equal(t, len(shares), sum)
			require.Equal(t, len(shares), counter.Size())
		})
	}

	writer := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
	counter := share.NewCompactShareCounter()
	require.Equal(t, counter.Size(), 0)
	require.Equal(t, writer.Count(), counter.Size())
}

func TestCompactShareCounterRevert(t *testing.T) {
	counter := share.NewCompactShareCounter()
	require.Equal(t, counter.Size(), 0)
	counter.Add(share.FirstCompactShareContentSize - 2)
	counter.Add(1)
	require.Equal(t, counter.Size(), 2)
	counter.Revert()
	require.Equal(t, counter.Size(), 1)
}

func newTx(length int) []byte {
	return bytes.Repeat([]byte("a"), length)
}

func newTxs(n int, length int) [][]byte {
	txs := make([][]byte, n)
	for i := 0; i < n; i++ {
		txs[i] = newTx(length)
	}
	return txs
}
