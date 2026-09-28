package share_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/stretchr/testify/require"
)

// Pin the different length and padding policies before sharing sequence assembly.
func TestParseBlobsSequenceCompatibility(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	blob, err := share.NewV0Blob(ns, bytes.Repeat([]byte{7}, share.FirstSparseShareContentSize+1))
	require.NoError(t, err)
	writer := share.NewSparseShareSplitter()
	require.NoError(t, writer.Write(blob))
	shares := writer.Export()
	require.Len(t, shares, 2)
	padding, err := share.NamespacePaddingShare(ns, share.ShareVersionZero)
	require.NoError(t, err)

	withVersion := func(s share.Share, version uint8) share.Share {
		data := bytes.Clone(s.ToBytes())
		info, err := share.NewInfoByte(version, s.IsSequenceStart())
		require.NoError(t, err)
		data[share.NamespaceSize] = byte(info)
		result, err := share.NewShare(data)
		require.NoError(t, err)
		return result
	}
	otherNamespace := bytes.Clone(shares[1].ToBytes())
	otherNamespace[share.NamespaceSize-1]++
	wrongNamespace, err := share.NewShare(otherNamespace)
	require.NoError(t, err)

	testCases := []struct {
		name      string
		shares    []share.Share
		want      []*share.Blob
		wantErr   bool
		sharesErr bool
	}{
		{name: "empty"},
		{name: "complete", shares: shares, want: []*share.Blob{blob}},
		{name: "consecutive same namespace", shares: []share.Share{shares[0], shares[1], shares[0], shares[1]}, want: []*share.Blob{blob, blob}},
		{name: "padding only", shares: []share.Share{padding, share.ReservedPaddingShare(), share.TailPaddingShare()}},
		{name: "padding between sequences", shares: []share.Share{padding, shares[0], shares[1], share.TailPaddingShare()}, want: []*share.Blob{blob}},
		{name: "padding within sequence", shares: []share.Share{shares[0], padding, share.ReservedPaddingShare(), share.TailPaddingShare(), shares[1]}, want: []*share.Blob{blob}, sharesErr: true},
		{name: "surplus continuation", shares: []share.Share{shares[0], shares[1], shares[1]}, want: []*share.Blob{blob}, sharesErr: true},
		{name: "supported continuation version differs", shares: []share.Share{shares[0], withVersion(shares[1], share.ShareVersionOne)}, want: []*share.Blob{blob}},
		{name: "truncated", shares: shares[:1], wantErr: true, sharesErr: true},
		{name: "leading continuation", shares: shares[1:], wantErr: true, sharesErr: true},
		{name: "namespace mismatch", shares: []share.Share{shares[0], wrongNamespace}, wantErr: true, sharesErr: true},
		{name: "unsupported continuation version", shares: []share.Share{shares[0], withVersion(shares[1], 127)}, wantErr: true},
		{name: "unsupported padding version", shares: []share.Share{withVersion(padding, 127)}, wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := share.ParseBlobs(tc.shares)
			if tc.wantErr {
				require.Error(t, err)
				require.Empty(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
			for _, ignorePadding := range []bool{false, true} {
				_, err := share.ParseShares(tc.shares, ignorePadding)
				if tc.sharesErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestSequenceRawDataTruncated(t *testing.T) {
	for _, ns := range []share.Namespace{share.TxNamespace, share.PayForBlobNamespace, share.PayForFibreNamespace} {
		writer := share.NewCompactShareSplitter(ns, share.ShareVersionZero)
		require.NoError(t, writer.WriteTx(bytes.Repeat([]byte{1}, share.ShareSize)))
		shares, err := writer.Export()
		require.NoError(t, err)
		sequence := share.Sequence{Namespace: ns, Shares: shares[:1]}
		_, err = sequence.RawData()
		require.ErrorContains(t, err, "greater than the number of bytes")
	}
}
