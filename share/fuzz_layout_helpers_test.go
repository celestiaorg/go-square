package share_test

import (
	"bytes"
	"math"
	"slices"
	"testing"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/stretchr/testify/require"
)

const maxFuzzRangeShares = 64

// FuzzGetShareRangeForNamespace compares GetShareRangeForNamespace with a
// linear scan on namespace-sorted shares, as found in a data square, and checks
// that unsorted input yields an in-bounds range without panicking.
func FuzzGetShareRangeForNamespace(f *testing.F) {
	f.Add([]byte{}, uint8(0), true)
	f.Add([]byte{0}, uint8(0), true)
	f.Add([]byte{0, 1, 1, 2, 3}, uint8(1), true)
	f.Add([]byte{3, 0, 3, 1}, uint8(3), false)
	f.Add([]byte{0, 0, 0}, uint8(5), true)
	f.Add([]byte{4, 5, 6, 4}, uint8(6), true)
	f.Fuzz(func(t *testing.T, layout []byte, query uint8, sorted bool) {
		if len(layout) > maxFuzzRangeShares {
			t.Skip()
		}
		shares := make([]share.Share, len(layout))
		for i, b := range layout {
			shares[i] = rangeTestShare(t, b)
		}
		if sorted {
			slices.SortStableFunc(shares, func(a, b share.Share) int {
				return a.Namespace().Compare(b.Namespace())
			})
		}
		ns := rangeTestNamespace(query)
		got := share.GetShareRangeForNamespace(shares, ns)
		require.True(t, got.IsEmpty() || (0 <= got.Start && got.Start < got.End && got.End <= len(shares)),
			"range %+v out of bounds for %d shares", got, len(shares))
		if !sorted {
			if !got.IsEmpty() {
				require.True(t, ns.Equals(shares[got.Start].Namespace()))
			}
			return
		}
		want := share.EmptyRange()
		for i, s := range shares {
			if s.Namespace().Equals(ns) {
				if want.IsEmpty() {
					want = share.NewRange(i, i)
				}
				want.End = i + 1
			}
		}
		require.Equal(t, want, got)
		if !got.IsEmpty() {
			shifted := got
			shifted.Add(7)
			require.Equal(t, share.NewRange(got.Start+7, got.End+7), shifted)
		}
	})
}

// rangeTestNamespace maps a byte onto eight namespaces that span the primary
// reserved, user and secondary reserved ranges.
func rangeTestNamespace(b byte) share.Namespace {
	switch b % 8 {
	case 0:
		return share.TxNamespace
	case 1:
		return share.PayForBlobNamespace
	case 2:
		return share.PrimaryReservedPaddingNamespace
	case 3:
		return share.MustNewV0Namespace([]byte{1})
	case 4:
		return share.MustNewV0Namespace([]byte{1, 0})
	case 5:
		return share.MustNewV0Namespace(bytes.Repeat([]byte{0xff}, share.NamespaceVersionZeroIDSize))
	case 6:
		return share.TailPaddingNamespace
	default:
		return share.ParitySharesNamespace
	}
}

func rangeTestShare(t *testing.T, b byte) share.Share {
	t.Helper()
	raw := make([]byte, share.ShareSize)
	copy(raw, rangeTestNamespace(b).Bytes())
	raw[share.NamespaceSize] = b // arbitrary info byte; the range ignores it
	s, err := share.NewShare(raw)
	require.NoError(t, err)
	return s
}

// FuzzNamespacePaddingShares checks the padding constructors: padding is
// recognised as padding by every parser, carries the requested namespace and
// version, and is never parsed as a transaction or blob.
func FuzzNamespacePaddingShares(f *testing.F) {
	f.Add(uint8(0), uint8(0), 0)
	f.Add(uint8(3), uint8(1), 2)
	f.Add(uint8(0), uint8(2), 1)
	f.Add(uint8(6), uint8(0), 3)
	f.Add(uint8(4), uint8(127), 1)
	f.Add(uint8(4), uint8(128), 1)
	f.Add(uint8(1), uint8(0), -1)
	f.Fuzz(func(t *testing.T, nsSel, version uint8, n int) {
		if n > 16 {
			t.Skip()
		}
		ns := rangeTestNamespace(nsSel)
		shares, err := share.NamespacePaddingShares(ns, version, n)
		if n < 0 || (n > 0 && version > share.MaxShareVersion) {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Len(t, shares, n)
		for _, s := range shares {
			require.Len(t, s.ToBytes(), share.ShareSize)
			require.True(t, ns.Equals(s.Namespace()))
			require.Equal(t, version, s.Version())
			require.True(t, s.IsSequenceStart())
			require.Zero(t, s.SequenceLen())
			require.True(t, s.IsPadding())
			require.Equal(t, make([]byte, len(s.RawData())), s.RawData(), "padding must be zero")
		}
		sequences, err := share.ParseShares(shares, true)
		require.NoError(t, err)
		require.Empty(t, sequences)
		sequences, err = share.ParseShares(shares, false)
		require.NoError(t, err)
		require.Len(t, sequences, n)
		if version == share.ShareVersionZero && (ns.IsTx() || ns.IsPayForBlob() || ns.IsPayForFibre()) {
			txs, err := share.ParseTxs(shares)
			require.NoError(t, err)
			require.Empty(t, txs)
		}
		if slices.Contains(share.SupportedShareVersions, version) {
			blobs, err := share.ParseBlobs(shares)
			require.NoError(t, err)
			require.Empty(t, blobs)
		}
	})
}

func TestReservedAndTailPadding(t *testing.T) {
	for _, tc := range []struct {
		ns    share.Namespace
		one   share.Share
		many  func(int) []share.Share
		count int
	}{
		{share.PrimaryReservedPaddingNamespace, share.ReservedPaddingShare(), share.ReservedPaddingShares, 3},
		{share.TailPaddingNamespace, share.TailPaddingShare(), share.TailPaddingShares, 3},
	} {
		want, err := share.NamespacePaddingShare(tc.ns, share.ShareVersionZero)
		require.NoError(t, err)
		require.Equal(t, want.ToBytes(), tc.one.ToBytes())
		got := tc.many(tc.count)
		require.Len(t, got, tc.count)
		for _, s := range got {
			require.Equal(t, want.ToBytes(), s.ToBytes())
		}
		require.Empty(t, tc.many(0))
	}
	_, err := share.NamespacePaddingShare(share.Namespace{}, share.ShareVersionZero)
	require.Error(t, err, "the zero namespace cannot form a 512-byte share")
}

// FuzzShareCountArithmetic checks that the share counters agree with a ceiling
// division reference for every uint32 sequence length, and that the
// AvailableBytes helpers are their exact inverses.
func FuzzShareCountArithmetic(f *testing.F) {
	for _, v := range []uint32{0, 1, 473, 474, 475, 477, 478, 479, 952, 953, 1<<31 - 1, math.MaxUint32} {
		f.Add(v, int32(v%1000))
	}
	f.Add(uint32(0), int32(-1))
	f.Add(uint32(0), int32(math.MaxInt32))
	f.Fuzz(func(t *testing.T, seqLen uint32, n int32) {
		require.Equal(t, refSharesNeeded(seqLen, share.FirstCompactShareContentSize, share.ContinuationCompactShareContentSize),
			share.CompactSharesNeeded(seqLen))
		require.Equal(t, refSharesNeeded(seqLen, share.FirstSparseShareContentSize, share.ContinuationSparseShareContentSize),
			share.SparseSharesNeeded(seqLen, false))
		require.Equal(t, refSharesNeeded(seqLen, share.FirstSparseShareContentSizeWithSigner, share.ContinuationSparseShareContentSize),
			share.SparseSharesNeeded(seqLen, true))

		compact := share.AvailableBytesFromCompactShares(int(n))
		sparse := share.AvailableBytesFromSparseShares(int(n))
		if n <= 0 {
			require.Zero(t, compact)
			require.Zero(t, sparse)
			return
		}
		require.Equal(t, share.FirstCompactShareContentSize+(int(n)-1)*share.ContinuationCompactShareContentSize, compact)
		require.Equal(t, share.FirstSparseShareContentSize+(int(n)-1)*share.ContinuationSparseShareContentSize, sparse)
		if compact < math.MaxUint32 {
			require.Equal(t, int(n), share.CompactSharesNeeded(uint32(compact)), "capacity of %d compact shares", n)
			require.Equal(t, int(n)+1, share.CompactSharesNeeded(uint32(compact)+1))
		}
		if sparse < math.MaxUint32 {
			require.Equal(t, int(n), share.SparseSharesNeeded(uint32(sparse), false), "capacity of %d sparse shares", n)
			require.Equal(t, int(n)+1, share.SparseSharesNeeded(uint32(sparse)+1, false))
		}
	})
}

// refSharesNeeded is ceil-division arithmetic in uint64, so it cannot overflow.
func refSharesNeeded(seqLen uint32, first, continuation int) int {
	l := uint64(seqLen)
	switch {
	case l == 0:
		return 0
	case l <= uint64(first):
		return 1
	default:
		return int(1 + (l-uint64(first)+uint64(continuation)-1)/uint64(continuation))
	}
}

func TestParseInfoByteExhaustive(t *testing.T) {
	for b := 0; b <= math.MaxUint8; b++ {
		ib, err := share.ParseInfoByte(byte(b))
		require.NoError(t, err, "every byte is a valid info byte")
		require.Equal(t, byte(b), byte(ib))
		require.Equal(t, uint8(b>>1), ib.Version())
		require.Equal(t, b&1 == 1, ib.IsSequenceStart())
		again, err := share.NewInfoByte(ib.Version(), ib.IsSequenceStart())
		require.NoError(t, err)
		require.Equal(t, ib, again)
	}
}

func TestFromBytes(t *testing.T) {
	good := bytes.Repeat([]byte{1}, share.ShareSize)
	shares, err := share.FromBytes([][]byte{good, good})
	require.NoError(t, err)
	require.Equal(t, [][]byte{good, good}, share.ToBytes(shares))
	empty, err := share.FromBytes(nil)
	require.NoError(t, err)
	require.Empty(t, empty)
	for _, bad := range [][]byte{nil, good[:share.ShareSize-1], append(bytes.Clone(good), 0)} {
		_, err := share.FromBytes([][]byte{good, bad})
		require.Error(t, err)
	}
}
