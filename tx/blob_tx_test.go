package tx_test

import (
	"testing"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestUnmarshalBlobTxRejectsUnrecognizedFields(t *testing.T) {
	clean := cleanBlobTx(t)

	tests := []struct {
		name   string
		mutate func(*testing.T, []byte) []byte
	}{
		{
			name: "known field with wrong wire type",
			mutate: func(_ *testing.T, blobTx []byte) []byte {
				return append(blobTx, 0x10, 0x33)
			},
		},
		{
			name: "unknown wrapper field",
			mutate: func(_ *testing.T, blobTx []byte) []byte {
				return append(blobTx, 0x78, 0x01)
			},
		},
		{
			name: "unknown blob field",
			mutate: func(t *testing.T, blobTx []byte) []byte {
				var wrapper v4.BlobTx
				require.NoError(t, proto.Unmarshal(blobTx, &wrapper))
				wrapper.Blobs[0].ProtoReflect().SetUnknown([]byte{0x78, 0x01})
				encoded, err := proto.Marshal(&wrapper)
				require.NoError(t, err)
				return encoded
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blobTx, isBlobTx, err := tx.UnmarshalBlobTx(tt.mutate(t, append([]byte(nil), clean...)))

			require.ErrorIs(t, err, tx.ErrNonCanonicalBlobTx)
			require.True(t, isBlobTx)
			require.Nil(t, blobTx)
		})
	}
}

func TestUnmarshalBlobTxRejectsNestedWrapper(t *testing.T) {
	inner := cleanBlobTx(t)
	blob := newBlob(t)
	outer, err := tx.MarshalBlobTx(inner, blob)
	require.NoError(t, err)

	blobTx, isBlobTx, err := tx.UnmarshalBlobTx(outer)

	require.ErrorIs(t, err, tx.ErrNestedBlobTx)
	require.True(t, isBlobTx)
	require.Nil(t, blobTx)
}

func cleanBlobTx(t *testing.T) []byte {
	t.Helper()

	blobTx, err := tx.MarshalBlobTx([]byte("inner tx"), newBlob(t))
	require.NoError(t, err)
	return blobTx
}

func newBlob(t *testing.T) *share.Blob {
	t.Helper()

	blob, err := share.NewBlob(
		share.RandomBlobNamespace(),
		[]byte("blob data"),
		share.ShareVersionZero,
		nil,
	)
	require.NoError(t, err)
	return blob
}
