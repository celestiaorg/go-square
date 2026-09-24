package share

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"

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
		{transactions: [][]byte{bytes.Repeat([]byte{1}, rawTxSize(FirstCompactShareContentSize+1))}, wantShareCount: 2},
		{transactions: [][]byte{generateTx(1)}, wantShareCount: 1},
		{transactions: [][]byte{generateTx(2)}, wantShareCount: 2},
		{transactions: [][]byte{generateTx(20)}, wantShareCount: 20},
	}
	for _, tc := range testCases {
		css := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
		for _, transaction := range tc.transactions {
			err := css.WriteTx(transaction)
			require.NoError(t, err)
		}
		got := css.Count()
		if got != tc.wantShareCount {
			t.Errorf("count got %d want %d", got, tc.wantShareCount)
		}
	}

	css := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
	assert.Equal(t, 0, css.Count())
}

// generateTx generates a transaction that occupies exactly numShares number of
// shares.
func generateTx(numShares int) []byte {
	if numShares == 0 {
		return []byte{}
	}
	if numShares == 1 {
		return bytes.Repeat([]byte{1}, rawTxSize(FirstCompactShareContentSize))
	}
	return bytes.Repeat([]byte{2}, rawTxSize(FirstCompactShareContentSize+(numShares-1)*ContinuationCompactShareContentSize))
}

func TestExport_write(t *testing.T) {
	type testCase struct {
		name       string
		want       []Share
		writeBytes [][]byte
	}

	oneShare, _ := zeroPadIfNecessary(
		append(
			TxNamespace.Bytes(),
			[]byte{
				0x1,                // info byte
				0x0, 0x0, 0x0, 0x1, // sequence len
				0x0, 0x0, 0x0, 0x26, // reserved bytes
				0xf, // data
			}...,
		),
		ShareSize)

	firstShare := fillShare(Share{data: append(
		TxNamespace.Bytes(),
		[]byte{
			0x1,                // info byte
			0x0, 0x0, 0x2, 0x0, // sequence len
			0x0, 0x0, 0x0, 0x26, // reserved bytes
		}...,
	)}, 0xf)

	continuationShare, _ := zeroPadIfNecessary(
		append(
			TxNamespace.Bytes(),
			append(
				[]byte{
					0x0,                // info byte
					0x0, 0x0, 0x0, 0x0, // reserved bytes
				}, bytes.Repeat([]byte{0xf}, NamespaceSize+ShareInfoBytes+SequenceLenBytes+ShareReservedBytes)..., // data
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
			css := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
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
				bytes.Repeat([]byte{0xf}, rawTxSize(FirstCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, rawTxSize(ContinuationCompactShareContentSize)),
			},
			wantLen: 2,
		},
		{
			name: "three txs that occupy exactly three shares",
			txs: [][]byte{
				bytes.Repeat([]byte{0xf}, rawTxSize(FirstCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, rawTxSize(ContinuationCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, rawTxSize(ContinuationCompactShareContentSize)),
			},
			wantLen: 3,
		},
		{
			name: "four txs that occupy three full shares and one partial share",
			txs: [][]byte{
				bytes.Repeat([]byte{0xf}, rawTxSize(FirstCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, rawTxSize(ContinuationCompactShareContentSize)),
				bytes.Repeat([]byte{0xf}, rawTxSize(ContinuationCompactShareContentSize)),
				{0xf},
			},
			wantLen: 4,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			css := NewCompactShareSplitter(TxNamespace, ShareVersionZero)

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
	txTwo := bytes.Repeat([]byte{2}, 600)
	txThree := bytes.Repeat([]byte{3}, 1000)
	exactlyOneShare := bytes.Repeat([]byte{4}, rawTxSize(FirstCompactShareContentSize))
	exactlyTwoShares := bytes.Repeat([]byte{5}, rawTxSize(FirstCompactShareContentSize+ContinuationCompactShareContentSize))

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
			css := NewCompactShareSplitter(TxNamespace, ShareVersionZero)

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
	a := bytes.Repeat([]byte{0xf}, rawTxSize(FirstCompactShareContentSize))
	b := bytes.Repeat([]byte{0xf}, rawTxSize(ContinuationCompactShareContentSize*2))
	c := bytes.Repeat([]byte{0xf}, rawTxSize(ContinuationCompactShareContentSize))
	d := []byte{0xf}

	css := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
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

// fillShare returns a share filled with filler so that the share length
// is equal to ShareSize.
func fillShare(share Share, filler byte) (paddedShare Share) {
	return Share{data: append(share.data, bytes.Repeat([]byte{filler}, ShareSize-len(share.data))...)}
}

func TestCompactShareSplitterRejectsInvalidVersion(t *testing.T) {
	for _, version := range []uint8{MaxShareVersion + 1, 255} {
		require.PanicsWithError(t, fmt.Sprintf("version %d must be less than or equal to %d", version, MaxShareVersion), func() {
			NewCompactShareSplitter(TxNamespace, version)
		})
	}
}

func TestCompactShareSplitterRejectsNonTransactionNamespace(t *testing.T) {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	splitter := NewCompactShareSplitter(ns, ShareVersionZero)
	require.ErrorContains(t, splitter.WriteTx([]byte("tx")), "this is not a compact share")
	require.Zero(t, splitter.Count())
	require.Empty(t, splitter.ShareRanges(0), "a rejected transaction must not register a share range")
	shares, err := splitter.Export()
	require.NoError(t, err)
	require.Empty(t, shares)
}

func TestCompactShareSplitterEmptySequenceLength(t *testing.T) {
	splitter := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
	require.Zero(t, splitter.sequenceLen(0))
	shares, err := splitter.Export()
	require.NoError(t, err)
	require.Empty(t, shares)
	require.Zero(t, splitter.sequenceLen(0))
}

func TestCompactShareSplitterExportRejectsInvalidShare(t *testing.T) {
	splitter := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
	// Fill the first share exactly so another write starts a continuation.
	require.NoError(t, splitter.WriteTx(bytes.Repeat([]byte{1}, 472)))
	shares, err := splitter.Export()
	require.NoError(t, err)
	require.Len(t, shares, 1)

	// Export returns the underlying slice. A caller can replace an element
	// with an invalid zero-value share; finalizing subsequent writes must
	// report the invalid size rather than panic or return malformed shares.
	shares[0] = Share{}
	require.NoError(t, splitter.WriteTx([]byte("another transaction")))
	shares, err = splitter.Export()
	require.ErrorContains(t, err, "imported share must be 512 bytes, got 0")
	require.Empty(t, shares)
}

func TestWriteAfterExportRoundTrip(t *testing.T) {
	for _, ns := range []Namespace{TxNamespace, PayForBlobNamespace, PayForFibreNamespace} {
		for _, size := range []int{1, 100, 300, 471, 472, 473, 474, 949, 950, 951} {
			t.Run(fmt.Sprintf("%x/%d", ns.Bytes(), size), func(t *testing.T) {
				txs := goldenTxs([]int{size, 50, 1000})
				splitter := NewCompactShareSplitter(ns, ShareVersionZero)
				for i, tx := range txs {
					require.NoError(t, splitter.WriteTx(tx))
					count := splitter.Count()
					got, err := splitter.Export()
					require.NoError(t, err)
					require.Len(t, got, count)
					want := splitGoldenTxs(t, ns, txs[:i+1])
					require.Equal(t, ToBytes(want), ToBytes(got), "intermediate exports must not change encoding")
					parsed, err := ParseTxs(got)
					require.NoError(t, err)
					require.Equal(t, txs[:i+1], parsed)
					assertTxShareSuffixes(t, txs[:i+1], got)
					again, err := splitter.Export()
					require.NoError(t, err)
					require.Equal(t, ToBytes(got), ToBytes(again))

					// Compare ranges with uninterrupted writes, including offsets.
					uninterrupted := NewCompactShareSplitter(ns, ShareVersionZero)
					for _, written := range txs[:i+1] {
						require.NoError(t, uninterrupted.WriteTx(written))
					}
					require.Equal(t, uninterrupted.ShareRanges(7), splitter.ShareRanges(7))
				}
			})
		}
	}
}
