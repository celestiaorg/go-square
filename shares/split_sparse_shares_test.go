package shares

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/namespace"
	"github.com/stretchr/testify/assert"
)

// TestSparseShareSplitter tests that the spare share splitter can split blobs
// with different namespaces.
func TestSparseShareSplitter(t *testing.T) {
	ns1 := namespace.MustNewV0(bytes.Repeat([]byte{1}, namespace.NamespaceVersionZeroIDSize))
	ns2 := namespace.MustNewV0(bytes.Repeat([]byte{2}, namespace.NamespaceVersionZeroIDSize))

	blob1 := blob.New(ns1, []byte("data1"), ShareVersionZero)
	blob2 := blob.New(ns2, []byte("data2"), ShareVersionZero)
	sss := NewSparseShareSplitter()

	err := sss.Write(blob1)
	assert.NoError(t, err)

	err = sss.Write(blob2)
	assert.NoError(t, err)

	got := sss.Export()
	assert.Len(t, got, 2)
}

func TestWriteNamespacePaddingShares(t *testing.T) {
	ns1 := namespace.MustNewV0(bytes.Repeat([]byte{1}, namespace.NamespaceVersionZeroIDSize))
	blob1 := newBlob(ns1, ShareVersionZero)

	sss := NewSparseShareSplitter()

	err := sss.Write(blob1)
	assert.NoError(t, err)
	err = sss.WriteNamespacePaddingShares(1)
	assert.NoError(t, err)

	// got is expected to be [blob1, padding]
	got := sss.Export()
	assert.Len(t, got, 2)

	// verify that the second share is padding
	isPadding, err := got[1].IsPadding()
	assert.NoError(t, err)
	assert.True(t, isPadding)

	// verify that the padding share has the same share version as blob1
	info, err := got[1].InfoByte()
	assert.NoError(t, err)
	assert.Equal(t, info.Version(), ShareVersionZero)
}

func newBlob(ns namespace.Namespace, shareVersion uint8) *blob.Blob {
	return blob.New(ns, []byte("data"), shareVersion)
}
