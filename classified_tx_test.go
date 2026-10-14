package square_test

import (
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

// systemBlob returns a share version two blob suitable for a FibreTx.
func systemBlob(t *testing.T) *share.Blob {
	t.Helper()
	ns := share.MustNewV0Namespace([]byte("fibre"))
	blob, err := share.NewV2Blob(ns, 0, make([]byte, share.FibreCommitmentSize), make([]byte, share.SignerSize))
	require.NoError(t, err)
	return blob
}

func TestClassifiedTxValidate(t *testing.T) {
	raw := []byte("raw tx bytes")
	blob := systemBlob(t)

	v0Blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte("fibre")), []byte("data"))
	require.NoError(t, err)

	blobTxBytes, err := tx.MarshalBlobTx([]byte("inner tx"), v0Blob)
	require.NoError(t, err)
	decodedBlobTx, isBlobTx, err := tx.UnmarshalBlobTx(blobTxBytes)
	require.NoError(t, err)
	require.True(t, isBlobTx)

	// A blob tx whose inner tx is itself a blob tx. UnmarshalBlobTx rejects
	// these as nested, so a pre-decoded one must be rejected too.
	nestedBlobTxBytes, err := tx.MarshalBlobTx(blobTxBytes, v0Blob)
	require.NoError(t, err)
	_, isBlobTx, err = tx.UnmarshalBlobTx(nestedBlobTxBytes)
	require.True(t, isBlobTx)
	require.ErrorIs(t, err, tx.ErrNestedBlobTx)

	testCases := []struct {
		name    string
		ctx     square.ClassifiedTx
		wantErr string
	}{
		{
			name: "normal tx",
			ctx:  square.ClassifiedTx{Bytes: raw},
		},
		{
			name: "fibre tx",
			ctx:  square.ClassifiedTx{Bytes: raw, FibreTx: &tx.FibreTx{Tx: raw, SystemBlob: blob}},
		},
		{
			name:    "empty bytes",
			ctx:     square.ClassifiedTx{},
			wantErr: "has no bytes",
		},
		{
			name:    "fibre tx with nil system blob",
			ctx:     square.ClassifiedTx{Bytes: raw, FibreTx: &tx.FibreTx{Tx: raw}},
			wantErr: "system blob",
		},
		{
			name:    "fibre tx bytes differ from classified bytes",
			ctx:     square.ClassifiedTx{Bytes: raw, FibreTx: &tx.FibreTx{Tx: []byte("other"), SystemBlob: blob}},
			wantErr: "differ from classified tx bytes",
		},
		{
			name:    "system blob is not share version two",
			ctx:     square.ClassifiedTx{Bytes: raw, FibreTx: &tx.FibreTx{Tx: raw, SystemBlob: v0Blob}},
			wantErr: "share version",
		},
		{
			name: "pre-decoded blob tx",
			ctx:  square.ClassifiedTx{Bytes: blobTxBytes, BlobTx: decodedBlobTx},
		},
		{
			name: "pre-decoded blob tx and fibre tx",
			ctx: square.ClassifiedTx{
				Bytes:   blobTxBytes,
				BlobTx:  decodedBlobTx,
				FibreTx: &tx.FibreTx{Tx: blobTxBytes, SystemBlob: blob},
			},
			wantErr: "both blob tx and fibre tx",
		},
		{
			name:    "pre-decoded blob tx with no blobs",
			ctx:     square.ClassifiedTx{Bytes: blobTxBytes, BlobTx: &tx.BlobTx{Tx: []byte("inner tx")}},
			wantErr: "blob tx has no blobs",
		},
		{
			name:    "pre-decoded blob tx with a nil blob",
			ctx:     square.ClassifiedTx{Bytes: blobTxBytes, BlobTx: &tx.BlobTx{Tx: []byte("inner tx"), Blobs: []*share.Blob{nil}}},
			wantErr: "blob 0 is nil or empty",
		},
		{
			name:    "pre-decoded blob tx with an empty blob",
			ctx:     square.ClassifiedTx{Bytes: blobTxBytes, BlobTx: &tx.BlobTx{Tx: []byte("inner tx"), Blobs: []*share.Blob{{}}}},
			wantErr: "blob 0 is nil or empty",
		},
		{
			name:    "pre-decoded blob tx with a nil blob after a valid one",
			ctx:     square.ClassifiedTx{Bytes: blobTxBytes, BlobTx: &tx.BlobTx{Tx: []byte("inner tx"), Blobs: []*share.Blob{v0Blob, nil}}},
			wantErr: "blob 1 is nil or empty",
		},
		{
			name:    "pre-decoded nested blob tx",
			ctx:     square.ClassifiedTx{Bytes: nestedBlobTxBytes, BlobTx: &tx.BlobTx{Tx: blobTxBytes, Blobs: []*share.Blob{v0Blob}}},
			wantErr: "wraps another blob tx",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ctx.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestValidateRejectsBlobTxClassifiedAsFibre(t *testing.T) {
	v0Blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte("blobns")), []byte("data"))
	require.NoError(t, err)
	blobTxBytes, err := tx.MarshalBlobTx([]byte("inner tx"), v0Blob)
	require.NoError(t, err)

	classified := square.ClassifiedTx{
		Bytes:   blobTxBytes,
		FibreTx: &tx.FibreTx{Tx: blobTxBytes, SystemBlob: systemBlob(t)},
	}
	require.ErrorContains(t, classified.Validate(), "blob tx")
}

func TestConstructReportsClassificationErrorBeforeOrderingError(t *testing.T) {
	v0Blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte("blobns")), []byte("data"))
	require.NoError(t, err)
	blobTxBytes, err := tx.MarshalBlobTx([]byte("inner tx"), v0Blob)
	require.NoError(t, err)

	txs := []square.ClassifiedTx{square.NewClassifiedTx(blobTxBytes), {}}
	_, err = square.Construct(txs, 64, 64)
	require.ErrorContains(t, err, "classified tx at index 1: classified tx has no bytes")
}

func TestConstructRejectsInvalidClassification(t *testing.T) {
	raw := []byte("raw tx bytes")
	blob := systemBlob(t)

	testCases := []struct {
		name    string
		txs     []square.ClassifiedTx
		wantErr string
	}{
		{
			name:    "fibre tx bytes do not match",
			txs:     []square.ClassifiedTx{{Bytes: raw, FibreTx: &tx.FibreTx{Tx: []byte("other"), SystemBlob: blob}}},
			wantErr: "differ from classified tx bytes",
		},
		{
			name:    "fibre tx without system blob",
			txs:     []square.ClassifiedTx{{Bytes: raw, FibreTx: &tx.FibreTx{Tx: raw}}},
			wantErr: "system blob",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := square.Construct(tc.txs, 64, 64)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestConstructOrdersNormalThenFibre(t *testing.T) {
	normal := square.NewClassifiedTx([]byte("normal tx"))
	fibre, err := square.NewClassifiedFibreTx(&tx.FibreTx{Tx: []byte("fibre tx"), SystemBlob: systemBlob(t)})
	require.NoError(t, err)

	_, err = square.Construct([]square.ClassifiedTx{normal, fibre}, 64, 64)
	require.NoError(t, err)

	_, err = square.Construct([]square.ClassifiedTx{fibre, normal}, 64, 64)
	require.ErrorContains(t, err, "cannot be appended after pay-for-fibre tx")
}

func TestNewClassifiedTx(t *testing.T) {
	raw := []byte("raw tx bytes")
	ctx := square.NewClassifiedTx(raw)
	require.Equal(t, raw, ctx.Bytes)
	require.Nil(t, ctx.FibreTx)
	require.NoError(t, ctx.Validate())
}

func TestNewClassifiedFibreTx(t *testing.T) {
	raw := []byte("raw tx bytes")
	blob := systemBlob(t)

	t.Run("valid", func(t *testing.T) {
		ctx, err := square.NewClassifiedFibreTx(&tx.FibreTx{Tx: raw, SystemBlob: blob})
		require.NoError(t, err)
		require.Equal(t, raw, ctx.Bytes)
		require.NotNil(t, ctx.FibreTx)
	})

	t.Run("nil fibre tx", func(t *testing.T) {
		_, err := square.NewClassifiedFibreTx(nil)
		require.ErrorContains(t, err, "nil")
	})

	t.Run("nil system blob", func(t *testing.T) {
		_, err := square.NewClassifiedFibreTx(&tx.FibreTx{Tx: raw})
		require.ErrorContains(t, err, "system blob")
	})
}

func TestNewClassifiedBlobTx(t *testing.T) {
	v0Blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte("blobns")), []byte("data"))
	require.NoError(t, err)
	blobTxBytes, err := tx.MarshalBlobTx([]byte("inner tx"), v0Blob)
	require.NoError(t, err)
	decoded, isBlobTx, err := tx.UnmarshalBlobTx(blobTxBytes)
	require.NoError(t, err)
	require.True(t, isBlobTx)

	t.Run("valid", func(t *testing.T) {
		classified, err := square.NewClassifiedBlobTx(blobTxBytes, decoded)
		require.NoError(t, err)
		require.Equal(t, blobTxBytes, classified.Bytes)
		require.Same(t, decoded, classified.BlobTx)
		require.Nil(t, classified.FibreTx)
		require.NoError(t, classified.Validate())
	})

	t.Run("nil blob tx", func(t *testing.T) {
		_, err := square.NewClassifiedBlobTx(blobTxBytes, nil)
		require.ErrorContains(t, err, "nil blob tx")
	})

	t.Run("no bytes", func(t *testing.T) {
		_, err := square.NewClassifiedBlobTx(nil, decoded)
		require.ErrorContains(t, err, "has no bytes")
	})

	t.Run("no blobs", func(t *testing.T) {
		_, err := square.NewClassifiedBlobTx(blobTxBytes, &tx.BlobTx{Tx: []byte("inner tx")})
		require.ErrorContains(t, err, "blob tx has no blobs")
	})

	t.Run("nil blob", func(t *testing.T) {
		_, err := square.NewClassifiedBlobTx(blobTxBytes, &tx.BlobTx{Tx: []byte("inner tx"), Blobs: []*share.Blob{nil}})
		require.ErrorContains(t, err, "blob 0 is nil or empty")
	})

	t.Run("nested blob tx", func(t *testing.T) {
		nested, err := tx.MarshalBlobTx(blobTxBytes, v0Blob)
		require.NoError(t, err)
		_, err = square.NewClassifiedBlobTx(nested, &tx.BlobTx{Tx: blobTxBytes, Blobs: []*share.Blob{v0Blob}})
		require.ErrorIs(t, err, tx.ErrNestedBlobTx)
	})
}
