package square_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

func FuzzConstruct(f *testing.F) {
	f.Add([]byte{}, []byte{}, uint8(0), uint8(0))
	f.Add([]byte("tx"), []byte("other"), uint8(1), uint8(1))
	f.Add(bytes.Repeat([]byte{1}, 1000), []byte("tx"), uint8(0), uint8(64))
	blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte{1, 0}), []byte("blob"))
	require.NoError(f, err)
	blobTx, err := tx.MarshalBlobTx([]byte("pfb"), blob)
	require.NoError(f, err)
	f.Add([]byte("tx"), blobTx, uint8(2), uint8(64))
	f.Add(blobTx, []byte("tx"), uint8(2), uint8(64)) // invalid ordering
	f.Add(blobTx[:len(blobTx)-1], []byte{}, uint8(2), uint8(0))

	f.Fuzz(func(t *testing.T, a, b []byte, size, threshold uint8) {
		if len(a)+len(b) > 32*1024 {
			t.Skip()
		}
		txs := make([]square.ClassifiedTx, 0, 2)
		for _, raw := range [][]byte{a, b} {
			if len(raw) > 0 {
				txs = append(txs, square.NewClassifiedTx(raw))
			}
		}
		maxSize := 1 << (size % 5)
		result, err := square.Construct(txs, maxSize, int(threshold))
		if err != nil {
			return // malformed blobs, ordering, capacity, or threshold
		}
		assertFuzzSquare(t, result, maxSize)
		again, err := square.Construct(txs, maxSize, int(threshold))
		require.NoError(t, err)
		require.True(t, result.Equals(again), "construction must be deterministic")
	})
}

// Structured inputs reach blob placement and index calculation even when
// arbitrary transaction bytes rarely decode as valid BlobTx envelopes.
func FuzzConstructBlobRoundTrip(f *testing.F) {
	f.Add([]byte("tx"), []byte("blob"), false, false, uint8(2), uint8(64))
	f.Add(bytes.Repeat([]byte{1}, 473), bytes.Repeat([]byte{2}, 479), false, true, uint8(3), uint8(1))
	f.Add([]byte("tx"), bytes.Repeat([]byte{2}, 459), true, true, uint8(4), uint8(64))
	f.Add([]byte("tx"), bytes.Repeat([]byte{2}, 4096), true, false, uint8(0), uint8(2))
	f.Fuzz(func(t *testing.T, normal, data []byte, withSigner, withFibre bool, size, threshold uint8) {
		if len(normal) == 0 || len(data) == 0 || len(normal)+len(data) > 32*1024 {
			t.Skip()
		}
		version := share.ShareVersionZero
		var signer []byte
		if withSigner {
			version = share.ShareVersionOne
			signer = bytes.Repeat([]byte{1}, share.SignerSize)
		}
		ns := share.MustNewV0Namespace([]byte{1, 0})
		blob, err := share.NewBlob(ns, data, version, signer)
		require.NoError(t, err)
		encoded, err := tx.MarshalBlobTx([]byte("pfb"), blob)
		require.NoError(t, err)
		// Prefix the normal tx so generated protobuf cannot classify it as a BlobTx.
		normal = append([]byte{0xff}, normal...)
		txs := []square.ClassifiedTx{square.NewClassifiedTx(normal), square.NewClassifiedTx(encoded)}
		var systemBlob *share.Blob
		if withFibre {
			commitment := sha256.Sum256(data)
			systemBlob, err = share.NewV2Blob(share.MustNewV0Namespace([]byte{2, 0}), 1, commitment[:], bytes.Repeat([]byte{2}, share.SignerSize))
			require.NoError(t, err)
			fibre, err := square.NewClassifiedFibreTx(&tx.FibreTx{Tx: []byte("fibre"), SystemBlob: systemBlob})
			require.NoError(t, err)
			txs = append(txs, fibre)
		}
		maxSize := 1 << (size % 5)
		result, err := square.Construct(txs, maxSize, 1+int(threshold))
		if maxSize == 16 {
			// 256 shares always accommodate the bounded 32 KiB payload plus
			// compact metadata, subtree alignment and a one-share system blob.
			require.NoError(t, err)
		}
		if err != nil {
			return // bounded squares may not have capacity
		}
		assertFuzzSquare(t, result, maxSize)
		normalRange := share.GetShareRangeForNamespace(result, share.TxNamespace)
		parsed, err := share.ParseTxs(result[normalRange.Start:normalRange.End])
		require.NoError(t, err)
		require.Equal(t, [][]byte{normal}, parsed)
		blobRange, err := square.BlobShareRange(txs, 1, 0, maxSize, 1+int(threshold))
		require.NoError(t, err)
		require.GreaterOrEqual(t, blobRange.Start, 0)
		require.LessOrEqual(t, blobRange.End, len(result))
		blobs, err := share.ParseBlobs(result[blobRange.Start:blobRange.End])
		require.NoError(t, err)
		require.Len(t, blobs, 1)
		require.Equal(t, data, blobs[0].Data())
		require.Equal(t, signer, blobs[0].Signer())
		require.True(t, ns.Equals(blobs[0].Namespace()))
		wrapped, err := result.WrappedPFBs()
		require.NoError(t, err)
		require.Len(t, wrapped, 1)
		index, ok := tx.UnmarshalIndexWrapper(wrapped[0])
		require.True(t, ok)
		require.Equal(t, []uint32{uint32(blobRange.Start)}, index.ShareIndexes)
		require.Equal(t, []byte("pfb"), index.Tx)
		if withFibre {
			r := share.GetShareRangeForNamespace(result, systemBlob.Namespace())
			blobs, err := share.ParseBlobs(result[r.Start:r.End])
			require.NoError(t, err)
			require.Len(t, blobs, 1)
			require.Equal(t, systemBlob.Data(), blobs[0].Data())
			require.Equal(t, systemBlob.Signer(), blobs[0].Signer())
			fibreRange := share.GetShareRangeForNamespace(result, share.PayForFibreNamespace)
			parsed, err := share.ParseTxs(result[fibreRange.Start:fibreRange.End])
			require.NoError(t, err)
			require.Equal(t, [][]byte{[]byte("fibre")}, parsed)
		}
	})
}

func assertFuzzSquare(t *testing.T, result square.Square, maxSize int) {
	t.Helper()
	size, err := result.Size()
	require.NoError(t, err)
	require.Positive(t, size)
	require.LessOrEqual(t, size, maxSize)
	require.Zero(t, size&(size-1))
	require.Len(t, result, size*size)
	for i := range result {
		require.Len(t, result[i].ToBytes(), share.ShareSize)
		if i > 0 {
			require.True(t, result[i-1].Namespace().IsLessOrEqualThan(result[i].Namespace()))
		}
	}
}
