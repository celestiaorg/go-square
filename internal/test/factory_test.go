package test_test

import (
	"testing"

	"github.com/celestiaorg/go-square/internal/test"
	"github.com/stretchr/testify/require"
)

func TestPFBParity(t *testing.T) {
	blobSizes := []uint32{20, 30, 10}
	pfb := test.MockPFB(blobSizes)
	output, err := test.DecodeMockPFB(pfb)
	require.NoError(t, err)
	require.Equal(t, blobSizes, output)

	require.Panics(t, func() { test.MockPFB(nil) })

	_, err = test.DecodeMockPFB(test.RandomBytes(20))
	require.Error(t, err)
}
