package share

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// Reproduces https://github.com/celestiaorg/celestia-node/issues/4490
func TestMochaShares(t *testing.T) {
	jsonData, err := os.ReadFile("./testdata/mocha-shares.json")
	require.NoError(t, err)

	var shares []Share
	err = json.Unmarshal(jsonData, &shares)
	require.NoError(t, err)

	require.Equal(t, 3423, len(shares))
	require.True(t, shares[0].IsSequenceStart())
	require.False(t, shares[1].IsSequenceStart())
	require.False(t, shares[3422].IsSequenceStart())

	wantNamespace, err := NewV0Namespace([]byte{0x72, 0x65, 0x6e, 0x65, 0x72, 0x65, 0x6e, 0x65})
	require.NoError(t, err)
	for _, share := range shares {
		require.Equal(t, wantNamespace, share.Namespace())
	}

	_, err = parseSparseShares(shares)
	require.NoError(t, err)
}
