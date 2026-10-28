package share_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/stretchr/testify/require"
)

// FuzzParseShares feeds wire bytes directly to the decoders, without repairing
// namespaces, versions, sequence lengths, reserved offsets, or delimiters.
func FuzzParseShares(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add(make([]byte, share.ShareSize-1))
	f.Add(make([]byte, share.ShareSize))
	f.Add(make([]byte, share.ShareSize+1))
	compact := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
	require.NoError(f, compact.WriteTx(bytes.Repeat([]byte{0xff}, 1000)))
	shares, err := compact.Export()
	require.NoError(f, err)
	f.Add(bytes.Join(share.ToBytes(shares), nil))
	ns := share.MustNewV0Namespace([]byte{1, 0})
	for _, version := range []uint8{share.ShareVersionZero, share.ShareVersionOne, share.ShareVersionTwo} {
		var signer []byte
		data := bytes.Repeat([]byte{0x80}, 1000)
		if version != share.ShareVersionZero {
			signer = bytes.Repeat([]byte{1}, share.SignerSize)
		}
		if version == share.ShareVersionTwo {
			data = data[:share.FibreBlobVersionSize+share.FibreCommitmentSize]
		}
		blob, err := share.NewBlob(ns, data, version, signer)
		require.NoError(f, err)
		shares, err := blob.ToShares()
		require.NoError(f, err)
		wire := bytes.Join(share.ToBytes(shares), nil)
		f.Add(wire)
		f.Add(wire[share.ShareSize:]) // continuation without a sequence start
		malformed := bytes.Clone(wire)
		// A declared length much larger than the supplied sequence.
		copy(malformed[share.NamespaceSize+share.ShareInfoBytes:], []byte{0xff, 0xff, 0xff, 0xff})
		f.Add(malformed)
	}
	f.Add(bytes.Join(share.ToBytes(share.TailPaddingShares(2)), nil))

	f.Fuzz(func(t *testing.T, wire []byte) {
		// Bound work and allocation independently of attacker-declared lengths.
		if len(wire) > 16*share.ShareSize {
			t.Skip()
		}
		original := bytes.Clone(wire)
		shares := make([]share.Share, 0, len(wire)/share.ShareSize)
		for offset := 0; offset < len(wire); offset += share.ShareSize {
			s, err := share.NewShare(wire[offset:min(offset+share.ShareSize, len(wire))])
			if err != nil {
				return // truncated final share
			}
			require.Equal(t, wire[offset:offset+share.ShareSize], s.ToBytes())
			_ = s.CheckVersionSupported()
			_ = s.RawData()
			_, _ = s.RawDataUsingReserved()
			_ = share.GetSigner(s)
			shares = append(shares, s)
		}
		_, _ = share.ParseTxs(shares)
		_, _ = share.ParseBlobs(shares)
		for _, ignorePadding := range []bool{false, true} {
			sequences, _ := share.ParseShares(shares, ignorePadding)
			for _, sequence := range sequences {
				_, _ = sequence.RawData()
			}
		}
		require.Equal(t, original, wire, "parsing must not mutate wire bytes")
	})
}
