package tx_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshalFibreTx(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAA}, share.SignerSize)
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	// Construct v2 blob data: fibre_blob_version (4 bytes) + commitment (32 bytes)
	data := make([]byte, share.FibreBlobVersionSize+share.FibreCommitmentSize)
	binary.BigEndian.PutUint32(data[:share.FibreBlobVersionSize], 1)
	copy(data[share.FibreBlobVersionSize:], commitment)
	systemBlob, err := share.NewBlob(ns, data, share.ShareVersionTwo, signer)
	require.NoError(t, err)

	rawTx := []byte("raw-sdk-tx-bytes")

	marshaled, err := tx.MarshalFibreTx(rawTx, systemBlob)
	require.NoError(t, err)

	fibreTx, isFibreTx, err := tx.UnmarshalFibreTx(marshaled)
	require.NoError(t, err)
	require.True(t, isFibreTx)
	require.Equal(t, rawTx, fibreTx.Tx)
	require.Equal(t, systemBlob.Namespace(), fibreTx.SystemBlob.Namespace())
	require.Equal(t, systemBlob.Data(), fibreTx.SystemBlob.Data())
	require.Equal(t, systemBlob.ShareVersion(), fibreTx.SystemBlob.ShareVersion())
	require.Equal(t, systemBlob.Signer(), fibreTx.SystemBlob.Signer())
}

func TestUnmarshalFibreTxRejectsNonFibreTx(t *testing.T) {
	_, isFibreTx, _ := tx.UnmarshalFibreTx([]byte("not-a-fibre-tx"))
	assert.False(t, isFibreTx)
}

func TestUnmarshalFibreTxRejectsBlobTx(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	blob, err := share.NewV0Blob(ns, []byte("blob-data"))
	require.NoError(t, err)

	blobTxBytes, err := tx.MarshalBlobTx([]byte("inner-tx"), blob)
	require.NoError(t, err)

	_, isFibreTx, _ := tx.UnmarshalFibreTx(blobTxBytes)
	assert.False(t, isFibreTx)
}

func TestMarshalFibreTxRejectsNilBlob(t *testing.T) {
	_, err := tx.MarshalFibreTx([]byte("tx"), nil)
	require.Error(t, err)
}

func TestMarshalFibreTxRejectsEmptyBlob(t *testing.T) {
	_, err := tx.MarshalFibreTx([]byte("tx"), &share.Blob{})
	require.Error(t, err)
}
