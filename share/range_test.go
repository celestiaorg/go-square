package share_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-square/v4/internal/test"
	"github.com/celestiaorg/go-square/v4/share"
)

func TestGetShareRangeForNamespace(t *testing.T) {
	blobs := test.GenerateBlobs(100, 200, 300, 400)
	share.SortBlobs(blobs)
	writer := share.NewSparseShareSplitter()
	for _, blob := range blobs {
		err := writer.Write(blob)
		require.NoError(t, err)
	}
	shares := writer.Export()
	firstNamespace := shares[0].Namespace()
	lastNamespace := shares[len(shares)-1].Namespace()
	ns := share.RandomBlobNamespace()

	testCases := []struct {
		name          string
		shares        []share.Share
		namespace     share.Namespace
		expectedRange share.Range
	}{
		{
			name:          "Empty shares",
			shares:        []share.Share{},
			namespace:     ns,
			expectedRange: share.EmptyRange(),
		},
		{
			name:          "Namespace not found",
			shares:        shares,
			namespace:     ns,
			expectedRange: share.EmptyRange(),
		},
		{
			name:          "Namespace found",
			shares:        shares,
			namespace:     firstNamespace,
			expectedRange: share.NewRange(0, 1),
		},
		{
			name:          "Namespace at end",
			shares:        shares,
			namespace:     lastNamespace,
			expectedRange: share.NewRange(3, 4),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := share.GetShareRangeForNamespace(tc.shares, tc.namespace)
			assert.Equal(t, tc.expectedRange, result)
		})
	}
}
