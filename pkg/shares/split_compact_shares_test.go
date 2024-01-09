package shares

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/celestiaorg/go-square/pkg/namespace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCount(t *testing.T) {
	type testCase struct {
		transactions   [][]byte
		wantShareCount int
	}
	testCases := []testCase{
		{transactions: [][]byte{}, wantShareCount: 0},
		{transactions: [][]byte{{0}}, wantShareCount: 1},
		{transactions: [][]byte{bytes.Repeat([]byte{1}, 100)}, wantShareCount: 1},
		// Test with 1 byte over 1 share
		{transactions: [][]byte{bytes.Repeat([]byte{1}, RawTxSize(FirstCompactShareContentSize+1))}, wantShareCount: 2},
		{transactions: [][]byte{generateTx(1)}, wantShareCount: 1},
		{transactions: [][]byte{generateTx(2)}, wantShareCount: 2},
		{transactions: [][]byte{generateTx(20)}, wantShareCount: 20},
	}
	for _, tc := range testCases {
		css := NewCompactShareSplitter(namespace.TxNamespace, ShareVersionZero)
		for _, transaction := range tc.transactions {
			err := css.WriteTx(transaction)
			require.NoError(t, err)
		}
		got := css.Count()
		if got != tc.wantShareCount {
			t.Errorf("count got %d want %d", got, tc.wantShareCount)
		}
	}

	css := NewCompactShareSplitter(namespace.TxNamespace, ShareVersionZero)
	assert.Equal(t, 0, css.Count())
}

// generateTx generates a transaction that occupies exactly numShares number of
// shares.
func generateTx(numShares int) []byte {
	if numShares == 0 {
		return []byte{}
	}
	if numShares == 1 {
		return bytes.Repeat([]byte{1}, RawTxSize(FirstCompactShareContentSize))
	}
	return bytes.Repeat([]byte{2}, RawTxSize(FirstCompactShareContentSize+(numShares-1)*ContinuationCompactShareContentSize))
}

func TestExport_write(t *testing.T) {
	type testCase struct {
		name       string
		want       []Share
		writeBytes [][]byte
	}

	oneShare, _ := zeroPadIfNecessary(
		append(
			namespace.TxNamespace.Bytes(),
			[]byte{
				0x1,                // info byte
				0x0, 0x0, 0x0, 0x1, // sequence len
				0x0, 0x0, 0x0, 0x26, // reserved bytes
				0xf, // data
			}...,
		),
		ShareSize)

	firstShare := fillShare(Share{data: append(
		namespace.TxNamespace.Bytes(),
		[]byte{
			0x1,                // info byte
			0x0, 0x0, 0x2, 0x0, // sequence len
			0x0, 0x0, 0x0, 0x26, // reserved bytes
		}...,
	)}, 0xf)

	continuationShare, _ := zeroPadIfNecessary(
		append(
			namespace.TxNamespace.Bytes(),
			append(
				[]byte{
					0x0,                // info byte
					0x0, 0x0, 0x0, 0x0, // reserved bytes
				}, bytes.Repeat([]byte{0xf}, namespace.NamespaceSize+ShareInfoBytes+SequenceLenBytes+CompactShareReservedBytes)..., // data
			)...,
		),
		ShareSize)

	testCases := []testCase{
		{
			name: "empty",
			want: []Share{},
		},
		{
			name: "one share with small sequence len",
			want: []Share{
				{data: oneShare},
			},
			writeBytes: [][]byte{{0xf}},
		},
		{
			name: "two shares with big sequence len",
			want: []Share{
				firstShare,
				{data: continuationShare},
			},
			writeBytes: [][]byte{bytes.Repeat([]byte{0xf}, 512)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			css := NewCompactShareSplitter(namespace.TxNamespace, ShareVersionZero)
			for _, bytes := range tc.writeBytes {
				err := css.write(bytes)
				require.NoError(t, err)
			}
			got, err := css.Export()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)

			shares, err := css.Export()
			require.NoError(t, err)
			assert.Equal(t, got, shares)
			assert.Len(t, got, css.Count())
		})
	}
}

func TestWriteAndExportIdempotence(t *testing.T) {
	type testCase struct {
		name    string
		txs     [][]byte
		wantLen int
	}
	testCases := []testCase{
		{
			name:    "one tx that occupies exactly one share",
			txs:     [][]byte{generateTx(1)},
			wantLen: 1,
		},
		{
			name:    "one tx that occupies exactly two shares",
			txs:     [][]byte{generateTx(2)},
			wantLen: 2,
		},
		{
			name:    "one tx that occupies exactly three shares",
			txs:     [][]byte{generateTx(3)},
			wantLen: 3,
		},
		{
			name: "two txs that occupy exactly two shares",
			txs: [][]byte{
				bytes.Repeat([]byte{0xf}, RawTxSize(FirstCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, RawTxSize(ContinuationCompactShareContentSize)),
			},
			wantLen: 2,
		},
		{
			name: "three txs that occupy exactly three shares",
			txs: [][]byte{
				bytes.Repeat([]byte{0xf}, RawTxSize(FirstCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, RawTxSize(ContinuationCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, RawTxSize(ContinuationCompactShareContentSize)),
			},
			wantLen: 3,
		},
		{
			name: "four txs that occupy three full shares and one partial share",
			txs: [][]byte{
				bytes.Repeat([]byte{0xf}, RawTxSize(FirstCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, RawTxSize(ContinuationCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, RawTxSize(ContinuationCompactShareContentSize)),
				{0xf},
			},
			wantLen: 4,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			css := NewCompactShareSplitter(namespace.TxNamespace, ShareVersionZero)

			for _, tx := range tc.txs {
				err := css.WriteTx(tx)
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantLen, css.Count())
			shares, err := css.Export()
			require.NoError(t, err)
			assert.Equal(t, tc.wantLen, len(shares))
		})
	}
}

func TestExport(t *testing.T) {
	type testCase struct {
		name             string
		txs              [][]byte
		want             map[[sha256.Size]byte]Range
		shareRangeOffset int
	}

	txOne := []byte{0x1}
	txTwo := []byte(bytes.Repeat([]byte{2}, 600))
	txThree := []byte(bytes.Repeat([]byte{3}, 1000))
	exactlyOneShare := []byte(bytes.Repeat([]byte{4}, RawTxSize(FirstCompactShareContentSize)))
	exactlyTwoShares := []byte(bytes.Repeat([]byte{5}, RawTxSize(FirstCompactShareContentSize+ContinuationCompactShareContentSize)))

	testCases := []testCase{
		{
			name: "empty",
			txs:  [][]byte{},
			want: map[[sha256.Size]byte]Range{},
		},
		{
			name: "txOne occupies shares 0 to 0",
			txs: [][]byte{
				txOne,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(txOne): {0, 1},
			},
		},
		{
			name: "txTwo occupies shares 0 to 1",
			txs: [][]byte{
				txTwo,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(txTwo): {0, 2},
			},
		},
		{
			name: "txThree occupies shares 0 to 2",
			txs: [][]byte{
				txThree,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(txThree): {0, 3},
			},
		},
		{
			name: "txOne occupies shares 0 to 0, txTwo occupies shares 0 to 1, txThree occupies shares 1 to 3",
			txs: [][]byte{
				txOne,
				txTwo,
				txThree,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(txOne):   {0, 1},
				sha256.Sum256(txTwo):   {0, 2},
				sha256.Sum256(txThree): {1, 4},
			},
		},

		{
			name: "exactly one share occupies shares 0 to 0",
			txs: [][]byte{
				exactlyOneShare,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(exactlyOneShare): {0, 1},
			},
		},
		{
			name: "exactly two shares occupies shares 0 to 1",
			txs: [][]byte{
				exactlyTwoShares,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(exactlyTwoShares): {0, 2},
			},
		},
		{
			name: "two shares followed by one share",
			txs: [][]byte{
				exactlyTwoShares,
				exactlyOneShare,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(exactlyTwoShares): {0, 2},
				sha256.Sum256(exactlyOneShare):  {2, 3},
			},
		},
		{
			name: "one share followed by two shares",
			txs: [][]byte{
				exactlyOneShare,
				exactlyTwoShares,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(exactlyOneShare):  {0, 1},
				sha256.Sum256(exactlyTwoShares): {1, 3},
			},
		},
		{
			name: "one share followed by two shares offset by 10",
			txs: [][]byte{
				exactlyOneShare,
				exactlyTwoShares,
			},
			want: map[[sha256.Size]byte]Range{
				sha256.Sum256(exactlyOneShare):  {10, 11},
				sha256.Sum256(exactlyTwoShares): {11, 13},
			},
			shareRangeOffset: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			css := NewCompactShareSplitter(namespace.TxNamespace, ShareVersionZero)

			for _, tx := range tc.txs {
				err := css.WriteTx(tx)
				require.NoError(t, err)
			}

			got := css.ShareRanges(tc.shareRangeOffset)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWriteAfterExport(t *testing.T) {
	a := bytes.Repeat([]byte{0xf}, RawTxSize(FirstCompactShareContentSize))
	b := bytes.Repeat([]byte{0xf}, RawTxSize(ContinuationCompactShareContentSize*2))
	c := bytes.Repeat([]byte{0xf}, RawTxSize(ContinuationCompactShareContentSize))
	d := []byte{0xf}

	css := NewCompactShareSplitter(namespace.TxNamespace, ShareVersionZero)
	shares, err := css.Export()
	require.NoError(t, err)
	assert.Equal(t, 0, len(shares))

	err = css.WriteTx(a)
	require.NoError(t, err)

	shares, err = css.Export()
	require.NoError(t, err)
	assert.Equal(t, 1, len(shares))

	err = css.WriteTx(b)
	require.NoError(t, err)

	shares, err = css.Export()
	require.NoError(t, err)
	assert.Equal(t, 3, len(shares))

	err = css.WriteTx(c)
	require.NoError(t, err)

	shares, err = css.Export()
	require.NoError(t, err)
	assert.Equal(t, 4, len(shares))

	err = css.WriteTx(d)
	require.NoError(t, err)

	shares, err = css.Export()
	require.NoError(t, err)
	assert.Equal(t, 5, len(shares))

	shares, err = css.Export()
	require.NoError(t, err)
	assert.Equal(t, 5, len(shares))
}
