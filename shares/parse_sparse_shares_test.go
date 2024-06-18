package shares

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"testing"

	"github.com/celestiaorg/go-square/blob"
	ns "github.com/celestiaorg/go-square/namespace"
	"github.com/celestiaorg/nmt/namespace"
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
			blobs := make([]*blob.Blob, tc.blobCount)
			for i := 0; i < tc.blobCount; i++ {
				blobs[i] = generateRandomBlob(tc.blobSize)
			}

			blob.Sort(blobs)

			shares, err := SplitBlobs(blobs...)
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
			blobs := GenerateRandomlySizedBlobs(tc.blobCount, tc.blobSize)
			shares, err := SplitBlobs(blobs...)
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

	rawShare := []byte{}
	rawShare = append(rawShare, namespace.ID{1, 1, 1, 1, 1, 1, 1, 1}...)
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
	blobs := []*blob.Blob{
		randomSmallBlob,
		randomLargeBlob,
	}
	blob.Sort(blobs)

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

func generateRandomBlobWithNamespace(namespace ns.Namespace, size int) *blob.Blob {
	data := make([]byte, size)
	_, err := crand.Read(data)
	if err != nil {
		panic(err)
	}
	return blob.New(namespace, data, ShareVersionZero, nil)
}

func generateRandomBlob(dataSize int) *blob.Blob {
	ns := ns.MustNewV0(bytes.Repeat([]byte{0x1}, ns.NamespaceVersionZeroIDSize))
	return generateRandomBlobWithNamespace(ns, dataSize)
}

func GenerateRandomlySizedBlobs(count, maxBlobSize int) []*blob.Blob {
	blobs := make([]*blob.Blob, count)
	for i := 0; i < count; i++ {
		blobs[i] = generateRandomBlob(rand.Intn(maxBlobSize))
		if len(blobs[i].Data()) == 0 {
			i--
		}
	}

	// this is just to let us use assert.Equal
	if count == 0 {
		blobs = nil
	}

	blob.Sort(blobs)
	return blobs
}
