package share

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mebibyte = 1024 * 1024

func Test_parseSparseShares(t *testing.T) {
	type test struct {
		name          string
		blobSize      int
		blobCount     int
		sameNamespace bool
	}

	// each test is ran twice, once using blobSize as an exact size, and again
	// using it as a cap for randomly sized leaves
	tests := []test{
		{
			name:          "single small blob",
			blobSize:      10,
			blobCount:     1,
			sameNamespace: true,
		},
		{
			name:          "ten small blobs",
			blobSize:      10,
			blobCount:     10,
			sameNamespace: true,
		},
		{
			name:          "single big blob",
			blobSize:      ContinuationSparseShareContentSize * 4,
			blobCount:     1,
			sameNamespace: true,
		},
		{
			name:          "many big blobs",
			blobSize:      ContinuationSparseShareContentSize * 4,
			blobCount:     10,
			sameNamespace: true,
		},
		{
			name:          "single exact size blob",
			blobSize:      FirstSparseShareContentSize,
			blobCount:     1,
			sameNamespace: true,
		},
		{
			name:          "blobs with different namespaces",
			blobSize:      FirstSparseShareContentSize,
			blobCount:     5,
			sameNamespace: false,
		},
	}

	for _, tc := range tests {
		// run the tests with identically sized blobs
		t.Run(fmt.Sprintf("%s identically sized ", tc.name), func(t *testing.T) {
			sizes := make([]int, tc.blobCount)
			for i := range sizes {
				sizes[i] = tc.blobSize
			}
			blobs, err := GenerateV0Blobs(sizes, tc.sameNamespace)
			if err != nil {
				t.Error(err)
			}

			SortBlobs(blobs)

			shares, err := splitBlobs(blobs...)
			require.NoError(t, err)
			parsedBlobs, err := parseSparseShares(shares)
			if err != nil {
				t.Error(err)
			}

			// check that the namespaces and data are the same
			for i := 0; i < len(blobs); i++ {
				assert.Equal(t, blobs[i].Namespace(), parsedBlobs[i].Namespace(), "parsed blob namespace does not match")
				assert.Equal(t, blobs[i].Data(), parsedBlobs[i].Data(), "parsed blob data does not match")
			}

			if !tc.sameNamespace {
				// compare namespaces in case they should not be the same
				for i := 0; i < len(blobs); i++ {
					for j := i + 1; j < len(blobs); j++ {
						require.False(t, parsedBlobs[i].Namespace().Equals(parsedBlobs[j].Namespace()))
					}
				}
			}
		})

		// run the same tests using randomly sized blobs with caps of tc.blobSize
		t.Run(fmt.Sprintf("%s randomly sized", tc.name), func(t *testing.T) {
			blobs := generateRandomlySizedBlobs(tc.blobCount, tc.blobSize)
			shares, err := splitBlobs(blobs...)
			require.NoError(t, err)
			parsedBlobs, err := parseSparseShares(shares)
			if err != nil {
				t.Error(err)
			}

			// check that the namespaces and data are the same
			for i := 0; i < len(blobs); i++ {
				assert.Equal(t, blobs[i].Namespace(), parsedBlobs[i].Namespace())
				assert.Equal(t, blobs[i].Data(), parsedBlobs[i].Data())
			}
		})
	}
}

func Test_parseSparseSharesWithNamespacedPadding(t *testing.T) {
	sss := NewSparseShareSplitter()
	randomSmallBlob := generateRandomBlob(ContinuationSparseShareContentSize / 2)
	randomLargeBlob := generateRandomBlob(ContinuationSparseShareContentSize * 4)
	blobs := []*Blob{
		randomSmallBlob,
		randomLargeBlob,
	}
	SortBlobs(blobs)

	err := sss.Write(blobs[0])
	require.NoError(t, err)

	err = sss.WriteNamespacePaddingShares(4)
	require.NoError(t, err)

	err = sss.Write(blobs[1])
	require.NoError(t, err)

	err = sss.WriteNamespacePaddingShares(10)
	require.NoError(t, err)

	shares := sss.Export()
	pblobs, err := parseSparseShares(shares)
	require.NoError(t, err)
	require.Equal(t, blobs, pblobs)
}

func Test_parseShareVersionOne(t *testing.T) {
	namespace := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	data := bytes.Repeat([]byte{1}, mebibyte)
	signer := bytes.Repeat([]byte{1}, SignerSize)

	v1blob, err := NewV1Blob(namespace, data, signer)
	require.NoError(t, err)

	v1shares, err := splitBlobs(v1blob)
	require.NoError(t, err)

	parsedBlobs, err := parseSparseShares(v1shares)
	require.NoError(t, err)
	require.Len(t, parsedBlobs, 1)
	got := parsedBlobs[0]
	require.Equal(t, v1blob, got)
}

// The blob in this test was taken from Mocha block height 7,132,999.
// https://www.mintscan.io/celestia-testnet/tx/5E0A7F8FAAF15FA8C098F43B0AC6E033E9614AFE89182A1668E4230961A2BB3D
func Test_parseSparseSharesWithMochaBlob(t *testing.T) {
	namespace, err := NewV0Namespace([]byte("sov-mini-e"))
	require.NoError(t, err)

	// base64 encode the namespace bytes and ensure they match the namespace shown on Mintscan.
	namespaceBase64 := base64.StdEncoding.EncodeToString(namespace.Bytes())
	require.Equal(t, "AAAAAAAAAAAAAAAAAAAAAAAAAHNvdi1taW5pLWU=", namespaceBase64)

	// TODO: get the real blob data.
	data := []byte("data")
	// TODO: get the real signer.
	signer := []byte("signer")

	_, err = NewV1Blob(namespace, data, signer)
}

func splitBlobs(blobs ...*Blob) ([]Share, error) {
	writer := NewSparseShareSplitter()
	for _, blob := range blobs {
		if err := writer.Write(blob); err != nil {
			return nil, err
		}
	}
	return writer.Export(), nil
}
