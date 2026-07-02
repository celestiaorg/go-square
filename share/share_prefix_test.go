package share

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file pin the share prefix layout to hardcoded byte
// offsets and values. External consumers (e.g. the Sovereign SDK via the
// celestia-types Rust crate) parse raw shares with these exact offsets, so
// the layout must remain stable over time. If a test in this file fails, the
// change is a breaking change to the share format and must not be released
// without coordinating with downstream consumers.
// See https://github.com/celestiaorg/go-square/issues/267.

// TestSharePrefixConstants pins the constants that define the share prefix
// layout to their expected literal values.
func TestSharePrefixConstants(t *testing.T) {
	testCases := []struct {
		name string
		got  int
		want int
	}{
		{"ShareSize", ShareSize, 512},
		{"NamespaceVersionSize", NamespaceVersionSize, 1},
		{"NamespaceIDSize", NamespaceIDSize, 28},
		{"NamespaceSize", NamespaceSize, 29},
		{"NamespaceVersionZeroPrefixSize", NamespaceVersionZeroPrefixSize, 18},
		{"NamespaceVersionZeroIDSize", NamespaceVersionZeroIDSize, 10},
		{"ShareInfoBytes", ShareInfoBytes, 1},
		{"SequenceLenBytes", SequenceLenBytes, 4},
		{"ShareReservedBytes", ShareReservedBytes, 4},
		{"SignerSize", SignerSize, 20},
		{"MaxShareVersion", MaxShareVersion, 127},
		{"FirstSparseShareContentSize", FirstSparseShareContentSize, 478},
		{"FirstSparseShareContentSizeWithSigner", FirstSparseShareContentSizeWithSigner, 458},
		{"ContinuationSparseShareContentSize", ContinuationSparseShareContentSize, 482},
		{"FirstCompactShareContentSize", FirstCompactShareContentSize, 474},
		{"ContinuationCompactShareContentSize", ContinuationCompactShareContentSize, 478},
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.want, tc.got, tc.name)
	}

	assert.Equal(t, uint8(0), ShareVersionZero)
	assert.Equal(t, uint8(1), ShareVersionOne)
	assert.Equal(t, uint8(2), ShareVersionTwo)
}

// TestShareVersionZeroPrefixLayout pins the byte layout of share version 0
// sparse (blob) shares:
//
//	first share:        | namespace (29 bytes) | info byte | sequence length (4 bytes, big endian) | data (478 bytes) |
//	continuation share: | namespace (29 bytes) | info byte | data (482 bytes) |
func TestShareVersionZeroPrefixLayout(t *testing.T) {
	namespace := MustNewV0Namespace(bytes.Repeat([]byte{0x1}, NamespaceVersionZeroIDSize))
	// 478 bytes fill the first share, 482 bytes fill the second share, and
	// 100 bytes spill into a third share that must be zero-padded.
	data := repeatingBytes(478 + 482 + 100)
	blob, err := NewV0Blob(namespace, data)
	require.NoError(t, err)

	splitter := NewSparseShareSplitter()
	require.NoError(t, splitter.Write(blob))
	shares := splitter.Export()
	require.Len(t, shares, 3)

	first := shares[0].ToBytes()
	require.Len(t, first, 512)
	assert.Equal(t, namespace.Bytes(), first[0:29], "namespace occupies bytes [0, 29)")
	assert.Equal(t, byte(0b00000001), first[29], "info byte: version 0 in the upper 7 bits, sequence start bit set")
	assert.Equal(t, uint32(len(data)), binary.BigEndian.Uint32(first[30:34]), "sequence length occupies bytes [30, 34) big endian")
	assert.Equal(t, data[0:478], first[34:512], "data starts at byte 34")

	second := shares[1].ToBytes()
	assert.Equal(t, namespace.Bytes(), second[0:29])
	assert.Equal(t, byte(0b00000000), second[29], "info byte: version 0, sequence start bit unset")
	assert.Equal(t, data[478:960], second[30:512], "continuation data starts at byte 30")

	third := shares[2].ToBytes()
	assert.Equal(t, namespace.Bytes(), third[0:29])
	assert.Equal(t, byte(0b00000000), third[29])
	assert.Equal(t, data[960:1060], third[30:130])
	assert.Equal(t, bytes.Repeat([]byte{0x00}, 382), third[130:512], "the final share is zero-padded to 512 bytes")
}

// TestShareVersionOnePrefixLayout pins the byte layout of share version 1
// sparse (blob) shares:
//
//	first share:        | namespace (29 bytes) | info byte | sequence length (4 bytes, big endian) | signer (20 bytes) | data (458 bytes) |
//	continuation share: | namespace (29 bytes) | info byte | data (482 bytes) |
func TestShareVersionOnePrefixLayout(t *testing.T) {
	namespace := MustNewV0Namespace(bytes.Repeat([]byte{0x2}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAB}, 20)
	// 458 bytes fill the first share and 50 bytes spill into a second share.
	data := repeatingBytes(458 + 50)
	blob, err := NewV1Blob(namespace, data, signer)
	require.NoError(t, err)

	splitter := NewSparseShareSplitter()
	require.NoError(t, splitter.Write(blob))
	shares := splitter.Export()
	require.Len(t, shares, 2)

	first := shares[0].ToBytes()
	require.Len(t, first, 512)
	assert.Equal(t, namespace.Bytes(), first[0:29], "namespace occupies bytes [0, 29)")
	assert.Equal(t, byte(0b00000011), first[29], "info byte: version 1 in the upper 7 bits, sequence start bit set")
	assert.Equal(t, uint32(len(data)), binary.BigEndian.Uint32(first[30:34]), "sequence length occupies bytes [30, 34) big endian")
	assert.Equal(t, signer, first[34:54], "signer occupies bytes [34, 54)")
	assert.Equal(t, data[0:458], first[54:512], "data starts at byte 54")

	second := shares[1].ToBytes()
	assert.Equal(t, namespace.Bytes(), second[0:29])
	assert.Equal(t, byte(0b00000010), second[29], "info byte: version 1, sequence start bit unset")
	assert.Equal(t, data[458:508], second[30:80], "continuation data starts at byte 30, no signer")
	assert.Equal(t, bytes.Repeat([]byte{0x00}, 432), second[80:512], "the final share is zero-padded to 512 bytes")
}

// TestShareVersionTwoPrefixLayout pins the byte layout of the share version 2
// (Fibre system blob) share:
//
//	| namespace (29 bytes) | info byte | sequence length (4 bytes, big endian) | signer (20 bytes) | fibre blob version (4 bytes, big endian) | fibre commitment (32 bytes) | padding |
func TestShareVersionTwoPrefixLayout(t *testing.T) {
	namespace := MustNewV0Namespace(bytes.Repeat([]byte{0x3}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xCD}, 20)
	commitment := bytes.Repeat([]byte{0xEF}, 32)
	fibreBlobVersion := uint32(7)
	blob, err := NewV2Blob(namespace, fibreBlobVersion, commitment, signer)
	require.NoError(t, err)

	splitter := NewSparseShareSplitter()
	require.NoError(t, splitter.Write(blob))
	shares := splitter.Export()
	require.Len(t, shares, 1)

	raw := shares[0].ToBytes()
	require.Len(t, raw, 512)
	assert.Equal(t, namespace.Bytes(), raw[0:29], "namespace occupies bytes [0, 29)")
	assert.Equal(t, byte(0b00000101), raw[29], "info byte: version 2 in the upper 7 bits, sequence start bit set")
	assert.Equal(t, uint32(36), binary.BigEndian.Uint32(raw[30:34]), "sequence length is fibre blob version size + commitment size")
	assert.Equal(t, signer, raw[34:54], "signer occupies bytes [34, 54)")
	assert.Equal(t, fibreBlobVersion, binary.BigEndian.Uint32(raw[54:58]), "fibre blob version occupies bytes [54, 58) big endian")
	assert.Equal(t, commitment, raw[58:90], "fibre commitment occupies bytes [58, 90)")
	assert.Equal(t, bytes.Repeat([]byte{0x00}, 422), raw[90:512], "the share is zero-padded to 512 bytes")
}

// TestCompactSharePrefixLayout pins the byte layout of compact (transaction)
// shares:
//
//	first share:        | namespace (29 bytes) | info byte | sequence length (4 bytes, big endian) | reserved bytes (4 bytes, big endian) | data (474 bytes) |
//	continuation share: | namespace (29 bytes) | info byte | reserved bytes (4 bytes, big endian) | data (478 bytes) |
func TestCompactSharePrefixLayout(t *testing.T) {
	tx := repeatingBytes(700)
	splitter := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
	require.NoError(t, splitter.WriteTx(tx))
	shares, err := splitter.Export()
	require.NoError(t, err)
	require.Len(t, shares, 2)

	first := shares[0].ToBytes()
	require.Len(t, first, 512)
	assert.Equal(t, TxNamespace.Bytes(), first[0:29], "namespace occupies bytes [0, 29)")
	assert.Equal(t, byte(0b00000001), first[29], "info byte: version 0 in the upper 7 bits, sequence start bit set")
	// The sequence length is the length of the transaction plus the length of
	// its varint length delimiter (2 bytes for a 700 byte transaction).
	assert.Equal(t, uint32(702), binary.BigEndian.Uint32(first[30:34]), "sequence length occupies bytes [30, 34) big endian")
	assert.Equal(t, uint32(38), binary.BigEndian.Uint32(first[34:38]), "reserved bytes occupy bytes [34, 38) and point at the first transaction")

	second := shares[1].ToBytes()
	assert.Equal(t, TxNamespace.Bytes(), second[0:29])
	assert.Equal(t, byte(0b00000000), second[29], "info byte: version 0, sequence start bit unset")
	assert.Equal(t, uint32(0), binary.BigEndian.Uint32(second[30:34]), "reserved bytes occupy bytes [30, 34) and are zero when no transaction starts in the share")
}

// TestPaddingSharePrefixLayout pins the byte layout of padding shares:
// sequence start bit set, sequence length zero, and an all-zero payload.
func TestPaddingSharePrefixLayout(t *testing.T) {
	namespace := MustNewV0Namespace(bytes.Repeat([]byte{0x4}, NamespaceVersionZeroIDSize))

	namespacePadding, err := NamespacePaddingShare(namespace, ShareVersionZero)
	require.NoError(t, err)
	raw := namespacePadding.ToBytes()
	require.Len(t, raw, 512)
	assert.Equal(t, namespace.Bytes(), raw[0:29], "namespace padding shares carry the namespace of the preceding blob")
	assert.Equal(t, byte(0b00000001), raw[29], "info byte: version 0, sequence start bit set")
	assert.Equal(t, bytes.Repeat([]byte{0x00}, 482), raw[30:512], "sequence length and payload are all zeros")

	tailPadding := TailPaddingShare()
	raw = tailPadding.ToBytes()
	expectedTailPaddingNamespace := append(bytes.Repeat([]byte{0xFF}, 28), 0xFE)
	assert.Equal(t, expectedTailPaddingNamespace, raw[0:29], "tail padding namespace is 28 0xFF bytes followed by 0xFE")
	assert.Equal(t, byte(0b00000001), raw[29])
	assert.Equal(t, bytes.Repeat([]byte{0x00}, 482), raw[30:512])
}

// TestReservedNamespaces pins the byte representation of the namespaces that
// external consumers match on.
func TestReservedNamespaces(t *testing.T) {
	assert.Equal(t, append(bytes.Repeat([]byte{0x00}, 28), 0x01), TxNamespace.Bytes())
	assert.Equal(t, append(bytes.Repeat([]byte{0x00}, 28), 0x04), PayForBlobNamespace.Bytes())
	assert.Equal(t, append(bytes.Repeat([]byte{0x00}, 28), 0xFF), PrimaryReservedPaddingNamespace.Bytes())
	assert.Equal(t, append(bytes.Repeat([]byte{0xFF}, 28), 0xFE), TailPaddingNamespace.Bytes())
	assert.Equal(t, bytes.Repeat([]byte{0xFF}, 29), ParitySharesNamespace.Bytes())
}

// TestSparseSharesNeededGolden pins the blob length to share count
// arithmetic. External consumers reimplement this arithmetic from the
// 478/458/482 content size constants to compute proof ranges.
func TestSparseSharesNeededGolden(t *testing.T) {
	testCases := []struct {
		sequenceLen    uint32
		containsSigner bool
		want           int
	}{
		{0, false, 0},
		{1, false, 1},
		{478, false, 1},
		{479, false, 2},
		{960, false, 2},  // 478 + 482
		{961, false, 3},  // 478 + 482 + 1
		{1442, false, 3}, // 478 + 2*482
		{1, true, 1},
		{458, true, 1},
		{459, true, 2},
		{940, true, 2}, // 458 + 482
		{941, true, 3}, // 458 + 482 + 1
	}
	for _, tc := range testCases {
		assert.Equal(t, tc.want, SparseSharesNeeded(tc.sequenceLen, tc.containsSigner), "sequenceLen=%d containsSigner=%v", tc.sequenceLen, tc.containsSigner)
	}
}

// repeatingBytes returns n bytes that cycle from 0x01 to 0xFF. It avoids 0x00
// so that data bytes are distinguishable from zero padding.
func repeatingBytes(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(i%255) + 1
	}
	return data
}
