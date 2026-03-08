package test_test

import (
	"testing"

	"github.com/celestiaorg/go-square/v4/internal/test"
	"github.com/stretchr/testify/require"
)

func TestPFBParity(t *testing.T) {
	blobSizes := []uint32{20, 30, 10}
	pfb, err := test.MockPFB(blobSizes)
	require.NoError(t, err)
	output, err := test.DecodeMockPFB(pfb)
	require.NoError(t, err)
	require.Equal(t, blobSizes, output)

	_, err = test.MockPFB(nil)
	require.Error(t, err)

	randomBytes, err := test.RandomBytes(20)
	require.NoError(t, err)
	_, err = test.DecodeMockPFB(randomBytes)
	require.Error(t, err)
}
