package square_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

// builderView is the observable state of a builder, used to assert that Fits
// and a rejected append leave it unchanged.
type builderView struct {
	txs, pfbs, pffs, blobs         int
	currentSize                    int
	txShares, pfbShares, pffShares int
	txRem, pfbRem, pffRem          int
	squareHash                     [32]byte
}

func viewBuilder(t *testing.T, b *square.Builder) builderView {
	t.Helper()
	sq, err := b.Export()
	require.NoError(t, err)
	return builderView{
		txs: len(b.Txs), pfbs: len(b.Pfbs), pffs: len(b.PayForFibreTxs), blobs: len(b.Blobs),
		currentSize: b.CurrentSize(),
		txShares:    b.TxCounter.Size(), pfbShares: b.PfbCounter.Size(), pffShares: b.PayForFibreCounter.Size(),
		txRem: b.TxCounter.Remainder(), pfbRem: b.PfbCounter.Remainder(), pffRem: b.PayForFibreCounter.Remainder(),
		squareHash: sq.Hash(),
	}
}

// TestBuilderFits asserts that Fits returns the verdict appending the same
// transaction would, reports invalid transactions as errors, and never changes
// the builder.
func TestBuilderFits(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	blobTxBytes := func(sizes ...int) []byte {
		return generateBlobTxsWithNamespaces(t, ns.Repeat(len(sizes)), [][]int{sizes})[0]
	}
	predecoded := func(sizes ...int) square.ClassifiedTx {
		raw := blobTxBytes(sizes...)
		blobTx, _, err := tx.UnmarshalBlobTx(raw)
		require.NoError(t, err)
		classified, err := square.NewClassifiedBlobTx(raw, blobTx)
		require.NoError(t, err)
		return classified
	}
	fibre := func(raw []byte) square.ClassifiedTx {
		f := newFibreTx(t, ns)
		f.Tx = raw
		return square.ClassifiedTx{Bytes: raw, FibreTx: f}
	}
	emptyBlob := predecoded(10)
	emptyBlob.BlobTx.Blobs[0] = &share.Blob{}
	v0Blob, err := share.NewV0Blob(ns, []byte("not a system blob"))
	require.NoError(t, err)
	wrongVersion := fibre([]byte("raw"))
	wrongVersion.FibreTx.SystemBlob = v0Blob
	noSystemBlob := fibre([]byte("raw"))
	noSystemBlob.FibreTx.SystemBlob = nil

	// All builders are 2x2 = 4 shares.
	fullSquare := func(t *testing.T, b *square.Builder) {
		require.True(t, b.AppendTx(newTx(share.AvailableBytesFromCompactShares(4))))
	}
	twoFibreTxs := func(t *testing.T, b *square.Builder) {
		mustAppendFibreTx(t, b, ns)
		mustAppendFibreTx(t, b, ns)
	}
	threeFibreTxs := func(t *testing.T, b *square.Builder) {
		twoFibreTxs(t, b)
		mustAppendFibreTx(t, b, ns)
	}

	testCases := []struct {
		name    string
		prefill func(t *testing.T, b *square.Builder)
		tx      square.ClassifiedTx
		wantFit bool
		wantErr string
	}{
		{name: "tx fits", tx: square.NewClassifiedTx(newTx(100)), wantFit: true},
		{name: "tx exact capacity", tx: square.NewClassifiedTx(newTx(share.AvailableBytesFromCompactShares(4))), wantFit: true},
		{name: "tx one byte over capacity", tx: square.NewClassifiedTx(newTx(share.AvailableBytesFromCompactShares(4) + 1))},
		{name: "tx on full square", prefill: fullSquare, tx: square.NewClassifiedTx(newTx(10))},
		{name: "tx empty", tx: square.NewClassifiedTx(nil), wantErr: "no bytes"},

		{name: "blob tx fits", tx: square.NewClassifiedTx(blobTxBytes(10)), wantFit: true},
		{name: "blob tx exact capacity", tx: square.NewClassifiedTx(blobTxBytes(share.AvailableBytesFromSparseShares(3))), wantFit: true},
		{name: "blob tx one byte over capacity", tx: square.NewClassifiedTx(blobTxBytes(share.AvailableBytesFromSparseShares(3) + 1))},
		{name: "blob tx on full square", prefill: fullSquare, tx: square.NewClassifiedTx(blobTxBytes(1))},
		{name: "pre-decoded blob tx fits", tx: predecoded(share.AvailableBytesFromSparseShares(1), share.AvailableBytesFromSparseShares(1)), wantFit: true},
		{name: "pre-decoded blob tx over capacity", tx: predecoded(share.AvailableBytesFromSparseShares(2)+1, share.AvailableBytesFromSparseShares(1))},
		{name: "pre-decoded blob tx with empty blob", tx: emptyBlob, wantErr: "nil or empty"},

		{name: "fibre tx fits", tx: fibre([]byte("pff")), wantFit: true},
		{name: "fibre tx exact capacity", tx: fibre(newTx(share.AvailableBytesFromCompactShares(3))), wantFit: true},
		{name: "fibre tx one byte over capacity", tx: fibre(newTx(share.AvailableBytesFromCompactShares(3) + 1))},
		{name: "fibre tx on full square", prefill: fullSquare, tx: fibre([]byte("pff"))},
		// Three fibre txs share one compact share and use three sparse shares.
		{name: "third fibre tx fits exactly", prefill: twoFibreTxs, tx: fibre([]byte("pff")), wantFit: true},
		{name: "fourth fibre tx does not fit", prefill: threeFibreTxs, tx: fibre([]byte("pff"))},
		{name: "fibre tx without system blob", tx: noSystemBlob, wantErr: "no system blob"},
		{name: "fibre tx with wrong share version", tx: wrongVersion, wantErr: "share version"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := square.NewBuilder(2, 64)
			require.NoError(t, err)
			if tc.prefill != nil {
				tc.prefill(t, b)
			}
			before := viewBuilder(t, b)

			fits, err := b.Fits(tc.tx)
			require.Equal(t, tc.wantFit, fits)
			require.Equal(t, before, viewBuilder(t, b), "Fits must not change the builder")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			added := appendClassified(t, b, tc.tx)
			require.Equal(t, fits, added, "Fits and append disagree")
			if !added {
				require.Equal(t, before, viewBuilder(t, b), "a rejected append must not change the builder")
			}
		})
	}
}

// appendClassified appends c with the Append method for its kind.
func appendClassified(t *testing.T, b *square.Builder, c square.ClassifiedTx) bool {
	t.Helper()
	if c.FibreTx != nil {
		added, err := b.AppendFibreTx(c.FibreTx)
		require.NoError(t, err)
		return added
	}
	blobTx, isBlobTx, err := tx.UnmarshalBlobTx(c.Bytes)
	if !isBlobTx {
		return b.AppendTx(c.Bytes)
	}
	require.NoError(t, err)
	added, err := b.AppendBlobTx(blobTx)
	require.NoError(t, err)
	return added
}
