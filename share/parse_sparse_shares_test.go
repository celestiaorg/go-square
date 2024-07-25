package share

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseSparseShares(t *testing.T) {
	type test struct {
		name      string
		blobSize  int
		blobCount int
	}

	// each test is ran twice, once using blobSize as an exact size, and again
	// using it as a cap for randomly sized leaves
	tests := []test{
		{
			name:      "single small blob",
			blobSize:  10,
			blobCount: 1,
		},
		{
			name:      "ten small blobs",
			blobSize:  10,
			blobCount: 10,
		},
		{
			name:      "single big blob",
			blobSize:  ContinuationSparseShareContentSize * 4,
			blobCount: 1,
		},
		{
			name:      "many big blobs",
			blobSize:  ContinuationSparseShareContentSize * 4,
			blobCount: 10,
		},
		{
			name:      "single exact size blob",
			blobSize:  FirstSparseShareContentSize,
			blobCount: 1,
		},
	}

	for _, tc := range tests {
		// run the tests with identically sized blobs
		t.Run(fmt.Sprintf("%s identically sized ", tc.name), func(t *testing.T) {
			blobs := make([]*Blob, tc.blobCount)
			for i := 0; i < tc.blobCount; i++ {
				blobs[i] = generateRandomBlob(tc.blobSize)
			}

			SortBlobs(blobs)

			shares, err := splitBlobs(blobs...)
			require.NoError(t, err)
			parsedBlobs, err := parseSparseShares(shares, SupportedShareVersions)
			if err != nil {
				t.Error(err)
			}

			// check that the namespaces and data are the same
			for i := 0; i < len(blobs); i++ {
				assert.Equal(t, blobs[i].Namespace(), parsedBlobs[i].Namespace(), "parsed blob namespace does not match")
				assert.Equal(t, blobs[i].Data(), parsedBlobs[i].Data(), "parsed blob data does not match")
			}
		})

		// run the same tests using randomly sized blobs with caps of tc.blobSize
		t.Run(fmt.Sprintf("%s randomly sized", tc.name), func(t *testing.T) {
			blobs := generateRandomlySizedBlobs(tc.blobCount, tc.blobSize)
			shares, err := splitBlobs(blobs...)
			require.NoError(t, err)
			parsedBlobs, err := parseSparseShares(shares, SupportedShareVersions)
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

func Test_parseSparseSharesErrors(t *testing.T) {
	type testCase struct {
		name   string
		shares []Share
	}

	unsupportedShareVersion := 5
	infoByte, _ := NewInfoByte(uint8(unsupportedShareVersion), true)

	rawShare := RandomNamespace().Bytes()
	rawShare = append(rawShare, byte(infoByte))
	rawShare = append(rawShare, bytes.Repeat([]byte{0}, ShareSize-len(rawShare))...)
	share, err := NewShare(rawShare)
	if err != nil {
		t.Fatal(err)
	}

	tests := []testCase{
		{
			"share with unsupported share version",
			[]Share{*share},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			_, err := parseSparseShares(tt.shares, SupportedShareVersions)
			assert.Error(t, err)
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
	pblobs, err := parseSparseShares(shares, SupportedShareVersions)
	require.NoError(t, err)
	require.Equal(t, blobs, pblobs)
}

func Test_parseShareVersionOne(t *testing.T) {
	v1blob, err := NewV1Blob(MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize)), []byte("data"), bytes.Repeat([]byte{1}, SignerSize))
	require.NoError(t, err)
	v1shares, err := splitBlobs(v1blob)
	require.NoError(t, err)

	parsedBlobs, err := parseSparseShares(v1shares, SupportedShareVersions)
	require.NoError(t, err)
	require.Equal(t, v1blob, parsedBlobs[0])
	require.Len(t, parsedBlobs, 1)
}

func generateRandomBlobWithNamespace(namespace Namespace, size int) *Blob {
	data := make([]byte, size)
	_, err := crand.Read(data)
	if err != nil {
		panic(err)
	}
	blob, err := NewV0Blob(namespace, data)
	if err != nil {
		panic(err)
	}
	return blob
}

func generateRandomBlob(dataSize int) *Blob {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{0x1}, NamespaceVersionZeroIDSize))
	return generateRandomBlobWithNamespace(ns, dataSize)
}

func generateRandomlySizedBlobs(count, maxBlobSize int) []*Blob {
	blobs := make([]*Blob, count)
	for i := 0; i < count; i++ {
		blobs[i] = generateRandomBlob(rand.Intn(maxBlobSize-1) + 1)
		if len(blobs[i].Data()) == 0 {
			i--
		}
	}

	// this is just to let us use assert.Equal
	if count == 0 {
		blobs = nil
	}

	SortBlobs(blobs)
	return blobs
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
