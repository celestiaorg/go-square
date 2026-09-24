package share_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/stretchr/testify/require"
)

// Alternating versions and padding exercises the public splitter's lifecycle:
// each blob must start a new sequence without inheriting its predecessor's
// namespace, signer, or partially filled share.
func TestSparseSplitterMixedSequences(t *testing.T) {
	testCases := []struct {
		version uint8
		size    int
		shares  int
	}{
		{share.ShareVersionZero, 478, 1},
		{share.ShareVersionOne, 459, 2},
		{share.ShareVersionTwo, 36, 1},
		{share.ShareVersionZero, 479, 2},
		{share.ShareVersionOne, 458, 1},
		{share.ShareVersionZero, 961, 3},
	}
	splitter := share.NewSparseShareSplitter()
	want := make([]*share.Blob, 0, len(testCases))
	count := 0
	for i, tc := range testCases {
		ns := share.MustNewV0Namespace(bytes.Repeat([]byte{byte(i + 1)}, share.NamespaceVersionZeroIDSize))
		var signer []byte
		if tc.version != share.ShareVersionZero {
			signer = bytes.Repeat([]byte{byte(i + 1)}, share.SignerSize)
		}
		blob, err := share.NewBlob(ns, bytes.Repeat([]byte{byte(i + 1)}, tc.size), tc.version, signer)
		require.NoError(t, err)
		require.NoError(t, splitter.Write(blob))
		want = append(want, blob)
		count += tc.shares
		require.Equal(t, count, splitter.Count())
		require.Len(t, splitter.Export(), count)

		require.NoError(t, splitter.WriteNamespacePaddingShares(1))
		count++
		got := splitter.Export()
		require.Equal(t, count, splitter.Count())
		require.Len(t, got, count)
		require.True(t, got[count-1].IsPadding())
		require.True(t, ns.Equals(got[count-1].Namespace()))
		require.Equal(t, tc.version, got[count-1].Version())

		parsed, err := share.ParseBlobs(got)
		require.NoError(t, err)
		require.Equal(t, want, parsed)
	}
}

func TestShareCountsMaxSequenceLength(t *testing.T) {
	// Expected counts use 64-bit arithmetic independently of the uint32 API.
	const length = uint32(1<<32 - 1)
	for _, withSigner := range []bool{false, true} {
		first := uint64(478)
		if withSigner {
			first = 458
		}
		want := 1 + (uint64(length)-first+481)/482
		require.Equal(t, int(want), share.SparseSharesNeeded(length, withSigner))
	}
	wantCompact := 1 + (uint64(length)-474+477)/478
	require.Equal(t, int(wantCompact), share.CompactSharesNeeded(length))
}
