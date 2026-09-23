package share

import (
	"bytes"
	"encoding/hex"
	"flag"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file pin the exact bytes produced by the share encoders.
// Fixtures live under testdata/golden, one hex-encoded share per line.
// Regenerate with:
//
//	go test ./share -run 'TestGolden' -update
//
// A diff in a fixture is a wire-format change. Do not regenerate fixtures to
// make a refactor pass; the refactor must reproduce them byte for byte.

var updateGolden = flag.Bool("update", false, "rewrite golden fixtures under testdata/golden")

func goldenPath(name string) string {
	return filepath.Join("testdata", "golden", name+".hex")
}

func encodeGolden(shares []Share) []byte {
	var buf bytes.Buffer
	for _, s := range shares {
		buf.WriteString(hex.EncodeToString(s.ToBytes()))
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func decodeGolden(raw []byte) []string {
	trimmed := strings.TrimRight(string(raw), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// assertGoldenShares compares got against the fixture named name. With
// -update the fixture is rewritten first.
func assertGoldenShares(t *testing.T, name string, got []Share) {
	t.Helper()
	path := goldenPath(name)
	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, encodeGolden(got), 0o644))
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "missing fixture %s; run: go test ./share -run TestGolden -update", path)
	want := decodeGolden(raw)
	require.Len(t, got, len(want), "%s: share count differs", name)
	for i := range want {
		assert.Equal(t, want[i], hex.EncodeToString(got[i].ToBytes()), "%s: share %d differs", name, i)
	}
}

var (
	goldenNamespace = MustNewV0Namespace(bytes.Repeat([]byte{0x1}, NamespaceVersionZeroIDSize))
	goldenSigner    = repeatingBytes(SignerSize)
)

func TestGoldenBlobShares(t *testing.T) {
	testCases := []struct {
		name         string
		shareVersion uint8
		dataLen      int
	}{
		{"blob_v0_100", ShareVersionZero, 100},
		{"blob_v0_478_exact_first_share", ShareVersionZero, 478},
		{"blob_v0_479_two_shares", ShareVersionZero, 479},
		{"blob_v0_960_exact_two_shares", ShareVersionZero, 960},
		{"blob_v0_961_three_shares", ShareVersionZero, 961},
		{"blob_v1_100", ShareVersionOne, 100},
		{"blob_v1_458_exact_first_share", ShareVersionOne, 458},
		{"blob_v1_459_two_shares", ShareVersionOne, 459},
		{"blob_v1_940_exact_two_shares", ShareVersionOne, 940},
		{"blob_v1_941_three_shares", ShareVersionOne, 941},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var signer []byte
			if tc.shareVersion == ShareVersionOne {
				signer = goldenSigner
			}
			blob, err := NewBlob(goldenNamespace, repeatingBytes(tc.dataLen), tc.shareVersion, signer)
			require.NoError(t, err)

			shares, err := blob.ToShares()
			require.NoError(t, err)
			assertGoldenShares(t, tc.name, shares)

			// The encoder and decoder must agree on the pinned bytes.
			parsed, err := ParseBlobs(shares)
			require.NoError(t, err)
			require.Len(t, parsed, 1)
			assert.Equal(t, blob.Data(), parsed[0].Data())
			assert.Equal(t, blob.Signer(), parsed[0].Signer())
			assert.Equal(t, blob.ShareVersion(), parsed[0].ShareVersion())
			assert.True(t, blob.Namespace().Equals(parsed[0].Namespace()))
			assert.Equal(t, len(shares), SparseSharesNeeded(uint32(tc.dataLen), tc.shareVersion == ShareVersionOne))
		})
	}
}

func TestGoldenFibreBlobShares(t *testing.T) {
	testCases := []struct {
		name         string
		fibreVersion uint32
	}{
		{"blob_v2_fibre", 7},
		{"blob_v2_fibre_version_max", math.MaxUint32},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			commitment := repeatingBytes(FibreCommitmentSize)
			blob, err := NewV2Blob(goldenNamespace, tc.fibreVersion, commitment, goldenSigner)
			require.NoError(t, err)

			shares, err := blob.ToShares()
			require.NoError(t, err)
			assertGoldenShares(t, tc.name, shares)

			parsed, err := ParseBlobs(shares)
			require.NoError(t, err)
			require.Len(t, parsed, 1)
			assert.Equal(t, blob.Data(), parsed[0].Data())
			assert.Equal(t, goldenSigner, parsed[0].Signer())
		})
	}
}

func TestGoldenPaddingShares(t *testing.T) {
	nsPadV0, err := NamespacePaddingShare(goldenNamespace, ShareVersionZero)
	require.NoError(t, err)
	nsPadV1, err := NamespacePaddingShare(goldenNamespace, ShareVersionOne)
	require.NoError(t, err)

	testCases := []struct {
		name   string
		shares []Share
	}{
		{"padding_namespace_v0", []Share{nsPadV0}},
		{"padding_namespace_v1", []Share{nsPadV1}},
		{"padding_reserved", ReservedPaddingShares(2)},
		{"padding_tail", TailPaddingShares(2)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertGoldenShares(t, tc.name, tc.shares)
			for _, s := range tc.shares {
				assert.True(t, s.IsPadding())
			}
		})
	}
}

// goldenTxs returns deterministic transactions. Transaction i is filled with
// the byte i+1 so that transactions are distinguishable from each other and
// from zero padding.
func goldenTxs(sizes []int) [][]byte {
	txs := make([][]byte, len(sizes))
	for i, size := range sizes {
		txs[i] = bytes.Repeat([]byte{byte(i + 1)}, size)
	}
	return txs
}

func splitGoldenTxs(t *testing.T, ns Namespace, txs [][]byte) []Share {
	t.Helper()
	splitter := NewCompactShareSplitter(ns, ShareVersionZero)
	for _, tx := range txs {
		require.NoError(t, splitter.WriteTx(tx))
	}
	shares, err := splitter.Export()
	require.NoError(t, err)
	return shares
}

// reservedBytesOf reads the 4-byte reserved field from each share. The
// field follows the sequence length in the first share and the info byte in
// continuation shares.
func reservedBytesOf(t *testing.T, shares []Share) []uint32 {
	t.Helper()
	got := make([]uint32, len(shares))
	for i, s := range shares {
		start := NamespaceSize + ShareInfoBytes
		if i == 0 {
			start += SequenceLenBytes
		}
		var err error
		got[i], err = ParseReservedBytes(s.ToBytes()[start : start+ShareReservedBytes])
		require.NoError(t, err)
	}
	return got
}

func manySmallTxSizes(count, size int) []int {
	sizes := make([]int, count)
	for i := range sizes {
		sizes[i] = size
	}
	return sizes
}

func TestGoldenTxShares(t *testing.T) {
	// Length delimiters are varints: 1 byte for sizes < 128, 2 bytes for
	// sizes < 16384. FirstCompactShareContentSize is 474 and
	// ContinuationCompactShareContentSize is 478.
	testCases := []struct {
		name  string
		ns    Namespace
		sizes []int
	}{
		{"tx_single_100", TxNamespace, []int{100}},
		{"tx_472_exact_first_share", TxNamespace, []int{472}}, // 2 + 472 = 474
		{"tx_472_then_100_next_share_start", TxNamespace, []int{472, 100}},
		{"tx_471_then_100_delimiter_on_last_byte", TxNamespace, []int{471, 100}}, // 2 + 471 = 473, delimiter of tx 2 lands on share byte 511
		{"tx_1000_spanning_three_shares", TxNamespace, []int{1000}},
		{"tx_1000_then_100_mid_third_share", TxNamespace, []int{1000, 100}},
		{"tx_949_then_100_reserved_511", TxNamespace, []int{949, 100}}, // 2 + 949 = 951 = 474 + 477; tx 2's delimiter lands on byte 511 of share 2, the last byte
		{"tx_950_exact_two_shares", TxNamespace, []int{950}},           // 2 + 950 = 952 = 474 + 478, no padding in the last share
		{"tx_many_small_50x20", TxNamespace, manySmallTxSizes(50, 20)},
		{"tx_varint_boundary_127_128", TxNamespace, []int{127, 128}},
		{"pfb_single_100", PayForBlobNamespace, []int{100}},
		{"pfb_many_small_50x20", PayForBlobNamespace, manySmallTxSizes(50, 20)},
		{"pff_single_100", PayForFibreNamespace, []int{100}},
		{"pff_many_small_50x20", PayForFibreNamespace, manySmallTxSizes(50, 20)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			txs := goldenTxs(tc.sizes)
			shares := splitGoldenTxs(t, tc.ns, txs)
			assertGoldenShares(t, tc.name, shares)

			parsed, err := ParseTxs(shares)
			require.NoError(t, err)
			assert.Equal(t, txs, parsed)
			assertTxShareSuffixes(t, txs, shares)

			total := 0
			for _, tx := range txs {
				total += delimLen(uint64(len(tx))) + len(tx)
			}
			assert.Equal(t, len(shares), CompactSharesNeeded(uint32(total)))
			assert.Equal(t, uint32(total), shares[0].SequenceLen())
		})
	}
}

// TestGoldenReservedBytes pins the reserved-bytes value per share in
// human-readable form so a fixture diff can be explained. Offsets: the first
// compact share's data starts at byte 38 (29 namespace + 1 info + 4 sequence
// length + 4 reserved); a continuation share's data starts at byte 34.
func TestGoldenReservedBytes(t *testing.T) {
	testCases := []struct {
		sizes []int
		want  []uint32
	}{
		{[]int{100}, []uint32{38}},
		{[]int{472}, []uint32{38}},
		{[]int{472, 100}, []uint32{38, 34}},
		{[]int{471, 100}, []uint32{38, 0}},
		{[]int{1000}, []uint32{38, 0, 0}},
		{[]int{1000, 100}, []uint32{38, 0, 84}}, // 34 + (1002 - 474 - 478) = 84
		{[]int{949, 100}, []uint32{38, 511, 0}},
	}
	for _, tc := range testCases {
		shares := splitGoldenTxs(t, TxNamespace, goldenTxs(tc.sizes))
		assert.Equal(t, tc.want, reservedBytesOf(t, shares), "sizes=%v", tc.sizes)
	}
}
