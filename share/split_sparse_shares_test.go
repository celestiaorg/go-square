package share

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSparseShareSplitter tests that the spare share splitter can split blobs
// with different namespaces.
func TestSparseShareSplitter(t *testing.T) {
	ns1 := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	ns2 := MustNewV0Namespace(bytes.Repeat([]byte{2}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{1}, SignerSize)

	blob1, err := NewV0Blob(ns1, []byte("data1"))
	require.NoError(t, err)
	blob2, err := NewV1Blob(ns2, []byte("data2"), signer)
	require.NoError(t, err)
	sss := NewSparseShareSplitter()

	err = sss.Write(blob1)
	assert.NoError(t, err)

	err = sss.Write(blob2)
	assert.NoError(t, err)

	got := sss.Export()
	assert.Len(t, got, 2)

	assert.Equal(t, ShareVersionZero, got[0].Version())
	assert.Equal(t, ShareVersionOne, got[1].Version())
	assert.Equal(t, signer, GetSigner(got[1]))
	assert.Nil(t, GetSigner(got[0])) // this is v0 so should not have any signer attached
}

func TestWriteNamespacePaddingShares(t *testing.T) {
	ns1 := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	blob1, err := NewV0Blob(ns1, []byte("data1"))
	require.NoError(t, err)

	sss := NewSparseShareSplitter()

	err = sss.Write(blob1)
	assert.NoError(t, err)
	err = sss.WriteNamespacePaddingShares(1)
	assert.NoError(t, err)

	// got is expected to be [blob1, padding]
	got := sss.Export()
	assert.Len(t, got, 2)

	// verify that the second share is padding
	assert.True(t, got[1].IsPadding())

	// verify that the padding share has the same share version as blob1
	version := got[1].Version()
	assert.Equal(t, version, ShareVersionZero)
}

func TestSparseShareSplitterV2Blob(t *testing.T) {
	ns1 := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	ns2 := MustNewV0Namespace(bytes.Repeat([]byte{2}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAA}, SignerSize)
	commitment := bytes.Repeat([]byte{0xBB}, FibreCommitmentSize)
	fibreBlobVersion := uint32(123)

	blob1, err := NewV0Blob(ns1, []byte("data1"))
	require.NoError(t, err)
	blob2, err := NewV2Blob(ns2, fibreBlobVersion, commitment, signer)
	require.NoError(t, err)

	sss := NewSparseShareSplitter()

	err = sss.Write(blob1)
	assert.NoError(t, err)

	err = sss.Write(blob2)
	assert.NoError(t, err)

	got := sss.Export()
	assert.Len(t, got, 2)

	// Verify share versions
	assert.Equal(t, ShareVersionZero, got[0].Version())
	assert.Equal(t, ShareVersionTwo, got[1].Version())

	// Verify signer is present in V2 share
	assert.Equal(t, signer, GetSigner(got[1]))
	assert.Nil(t, GetSigner(got[0])) // V0 share should not have signer

	// Parse shares back to verify round-trip
	blobList, err := parseSparseShares(got)
	require.NoError(t, err)
	require.Len(t, blobList, 2)

	// Verify V2 blob round-trip
	v2Blob := blobList[1]
	require.Equal(t, ShareVersionTwo, v2Blob.ShareVersion())
	require.Equal(t, ns2, v2Blob.Namespace())
	require.Equal(t, signer, v2Blob.Signer())

	// Verify fibre blob version and commitment extraction
	rv, err := v2Blob.FibreBlobVersion()
	require.NoError(t, err)
	require.Equal(t, fibreBlobVersion, rv)

	comm, err := v2Blob.FibreCommitment()
	require.NoError(t, err)
	require.Equal(t, commitment, comm)
}

func TestSparseShareSplitterV2BlobSingleShare(t *testing.T) {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{3}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xCC}, SignerSize)
	commitment := bytes.Repeat([]byte{0xDD}, FibreCommitmentSize)
	fibreBlobVersion := uint32(456)

	blob, err := NewV2Blob(ns, fibreBlobVersion, commitment, signer)
	require.NoError(t, err)

	sss := NewSparseShareSplitter()
	err = sss.Write(blob)
	assert.NoError(t, err)

	got := sss.Export()
	assert.Len(t, got, 1) // V2 blob should fit in a single share

	// Verify share version
	assert.Equal(t, ShareVersionTwo, got[0].Version())

	// Verify sequence length is FibreBlobVersionSize + FibreCommitmentSize (36 bytes)
	sequenceLen := got[0].SequenceLen()
	assert.Equal(t, uint32(FibreBlobVersionSize+FibreCommitmentSize), sequenceLen)

	// Verify signer is present
	assert.Equal(t, signer, GetSigner(got[0]))

	// Parse back to verify round-trip
	blobList, err := parseSparseShares(got)
	require.NoError(t, err)
	require.Len(t, blobList, 1)

	parsedBlob := blobList[0]
	require.Equal(t, blob.ShareVersion(), parsedBlob.ShareVersion())
	require.Equal(t, blob.Namespace(), parsedBlob.Namespace())
	require.Equal(t, blob.Signer(), parsedBlob.Signer())

	// Verify fibre blob version and commitment
	rv, err := parsedBlob.FibreBlobVersion()
	require.NoError(t, err)
	require.Equal(t, fibreBlobVersion, rv)

	comm, err := parsedBlob.FibreCommitment()
	require.NoError(t, err)
	require.Equal(t, commitment, comm)
}

func TestSparseShareSplitterV2BlobInvalidData(t *testing.T) {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{4}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xEE}, SignerSize)

	// Create blob with wrong data size (not 36 bytes)
	wrongData := []byte{1, 2, 3}

	_, err := NewBlob(ns, wrongData, ShareVersionTwo, signer)
	require.Error(t, err) // Should fail validation

	// Test with correct data size but try to write through splitter
	// (This should be caught by NewBlob validation, but test the splitter too)
	validData := make([]byte, FibreBlobVersionSize+FibreCommitmentSize)
	binary.BigEndian.PutUint32(validData[0:FibreBlobVersionSize], uint32(789))
	copy(validData[FibreBlobVersionSize:], bytes.Repeat([]byte{0xFF}, FibreCommitmentSize))

	validBlob, err := NewBlob(ns, validData, ShareVersionTwo, signer)
	require.NoError(t, err)

	sss := NewSparseShareSplitter()
	err = sss.Write(validBlob)
	assert.NoError(t, err)
}
