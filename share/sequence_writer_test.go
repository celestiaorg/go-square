package share

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSequenceWriterFlushIncompleteShare(t *testing.T) {
	pending, err := newBuilder(TxNamespace, ShareVersionZero, true)
	require.NoError(t, err)
	writer := sequenceWriter{pending: pending}
	payload := bytes.Repeat([]byte{0xab}, FirstCompactShareContentSize)

	require.NoError(t, writer.write(payload[:100]))
	require.ErrorContains(t, writer.flush(), "share data must be 512 bytes")
	require.Empty(t, writer.shares, "a failed flush must not append an invalid share")

	// The rejected flush must retain the pending payload, allowing the caller
	// to complete the share without losing or duplicating any bytes.
	require.NoError(t, writer.write(payload[100:]))
	require.Len(t, writer.shares, 1)
	require.Equal(t, payload, writer.shares[0].RawData())
	require.True(t, writer.shares[0].IsSequenceStart())

	// The next write must use a continuation header and its larger capacity.
	continuation := bytes.Repeat([]byte{0xcd}, ContinuationCompactShareContentSize)
	require.NoError(t, writer.write(continuation))
	require.Len(t, writer.shares, 2)
	require.Equal(t, continuation, writer.shares[1].RawData())
	require.False(t, writer.shares[1].IsSequenceStart())
	require.True(t, writer.shares[1].Namespace().Equals(TxNamespace))
}
