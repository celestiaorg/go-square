package shares

import (
	"bytes"
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
