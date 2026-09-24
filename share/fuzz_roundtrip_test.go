package share

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

// These fuzz targets assert round-trip and share-count invariants using only
// the exported API. Seeds come from the golden cases so the corpus covers
// every interesting share boundary even in the seed-only CI run.

// nonEmpty drops empty slices. An empty transaction encodes as a single
// zero-length delimiter, which the parser treats as padding, so it cannot
// round-trip and is never written by the square builder.
func nonEmpty(parts ...[]byte) [][]byte {
	out := make([][]byte, 0, len(parts))
	for _, p := range parts {
		if len(p) > 0 {
			out = append(out, p)
		}
	}
	return out
}

func FuzzTxRoundTrip(f *testing.F) {
	f.Add([]byte("a"), []byte("bb"), []byte("ccc"))
	f.Add(repeatingBytes(472), repeatingBytes(100), []byte{})
	f.Add(repeatingBytes(471), repeatingBytes(100), []byte{})
	f.Add(repeatingBytes(1000), repeatingBytes(100), repeatingBytes(20))
	f.Add(repeatingBytes(1000), []byte{}, []byte{})
	f.Add(repeatingBytes(2000), repeatingBytes(1000), repeatingBytes(20))
	f.Add(repeatingBytes(949), repeatingBytes(100), []byte{})
	f.Add(repeatingBytes(1427), repeatingBytes(100), []byte{})
	f.Add(repeatingBytes(127), repeatingBytes(128), repeatingBytes(16384))

	f.Fuzz(func(t *testing.T, a, b, c []byte) {
		txs := nonEmpty(a, b, c)
		if len(txs) == 0 {
			t.Skip()
		}

		// build
		splitter := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
		for _, tx := range txs {
			require.NoError(t, splitter.WriteTx(tx))
		}
		shares, err := splitter.Export()
		require.NoError(t, err)

		// assert: full round trip
		parsed, err := ParseTxs(shares)
		require.NoError(t, err)
		require.Equal(t, txs, parsed)

		assertTxShareSuffixes(t, txs, shares)

		// assert: every continuation share is marked as such and shares the namespace
		for i, s := range shares {
			require.Equal(t, i == 0, s.IsSequenceStart(), "share %d", i)
			require.True(t, s.Namespace().Equals(TxNamespace))
			require.Equal(t, ShareVersionZero, s.Version())
		}
	})
}

// assertTxShareSuffixes checks every starting share, including those with no
// transaction start. Compute the expected suffix from input transaction lengths
// and share capacities, independently of the encoder's reserved bytes.
func assertTxShareSuffixes(t *testing.T, txs [][]byte, shares []Share) {
	t.Helper()
	starts := make([]int, len(txs))
	offset := 0
	var delimiter [binary.MaxVarintLen64]byte
	for i, tx := range txs {
		starts[i] = offset
		offset += binary.PutUvarint(delimiter[:], uint64(len(tx))) + len(tx)
	}

	firstTx := 0
	for start := 1; start < len(shares); start++ {
		payloadStart := FirstCompactShareContentSize + (start-1)*ContinuationCompactShareContentSize
		for firstTx < len(txs) && starts[firstTx] < payloadStart {
			firstTx++
		}
		parsed, err := ParseTxs(shares[start:])
		require.NoError(t, err, "parse from share %d", start)
		require.Equal(t, txs[firstTx:], parsed, "parse from share %d", start)
	}
}

func FuzzBlobRoundTrip(f *testing.F) {
	f.Add(repeatingBytes(1), false)
	f.Add(repeatingBytes(478), false)
	f.Add(repeatingBytes(479), false)
	f.Add(repeatingBytes(961), false)
	f.Add(repeatingBytes(458), true)
	f.Add(repeatingBytes(459), true)
	f.Add(repeatingBytes(941), true)

	f.Fuzz(func(t *testing.T, data []byte, withSigner bool) {
		if len(data) == 0 {
			t.Skip()
		}
		shareVersion := ShareVersionZero
		var signer []byte
		if withSigner {
			shareVersion = ShareVersionOne
			signer = goldenSigner
		}
		blob, err := NewBlob(goldenNamespace, data, shareVersion, signer)
		require.NoError(t, err)

		// build
		shares, err := blob.ToShares()
		require.NoError(t, err)

		// assert
		require.Equal(t, SparseSharesNeeded(uint32(len(data)), withSigner), len(shares))
		require.Equal(t, uint32(len(data)), shares[0].SequenceLen())

		parsed, err := ParseBlobs(shares)
		require.NoError(t, err)
		require.Len(t, parsed, 1)
		require.Equal(t, data, parsed[0].Data())
		require.Equal(t, signer, parsed[0].Signer())
		require.Equal(t, shareVersion, parsed[0].ShareVersion())
		require.True(t, goldenNamespace.Equals(parsed[0].Namespace()))

		// assert: the sequence parser agrees with the blob parser
		sequences, err := ParseShares(shares, true)
		require.NoError(t, err)
		require.Len(t, sequences, 1)
		raw, err := sequences[0].RawData()
		require.NoError(t, err)
		// Sequence.RawData strips the signer for v1, so it is the data alone.
		require.Equal(t, data, raw)
	})
}

func FuzzSharesNeeded(f *testing.F) {
	f.Add([]byte("a"), []byte("bb"), []byte("ccc"))
	f.Add(repeatingBytes(472), repeatingBytes(100), []byte{})
	f.Add(repeatingBytes(1000), repeatingBytes(100), repeatingBytes(20))

	f.Fuzz(func(t *testing.T, a, b, c []byte) {
		txs := nonEmpty(a, b, c)
		if len(txs) == 0 {
			t.Skip()
		}

		// build: incrementally, checking the counter against the splitter at each step
		counter := NewCompactShareCounter()
		splitter := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
		total := 0
		for i, tx := range txs {
			diff := counter.Add(len(tx))
			before := splitter.Count()
			require.NoError(t, splitter.WriteTx(tx))
			require.Equal(t, diff, splitter.Count()-before, "tx %d: counter diff disagrees with splitter", i)
			require.Equal(t, counter.Size(), splitter.Count(), "tx %d: counter size disagrees with splitter", i)
			total += delimLen(uint64(len(tx))) + len(tx)
		}
		shares, err := splitter.Export()
		require.NoError(t, err)

		// assert
		require.Equal(t, len(shares), CompactSharesNeeded(uint32(total)))
		require.Equal(t, len(shares), counter.Size())
		require.LessOrEqual(t, total, AvailableBytesFromCompactShares(len(shares)))
		if len(shares) > 1 {
			require.Greater(t, total, AvailableBytesFromCompactShares(len(shares)-1))
		}

		// assert: revert restores the previous state
		lastLen := len(txs[len(txs)-1])
		counter2 := NewCompactShareCounter()
		for _, tx := range txs[:len(txs)-1] {
			counter2.Add(len(tx))
		}
		sizeBefore := counter2.Size()
		counter2.Add(lastLen)
		counter2.Revert()
		require.Equal(t, sizeBefore, counter2.Size())
	})
}
