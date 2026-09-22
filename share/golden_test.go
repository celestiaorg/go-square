package share

import (
	"bytes"
	"encoding/hex"
	"flag"
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
	commitment := repeatingBytes(FibreCommitmentSize)
	blob, err := NewV2Blob(goldenNamespace, 7, commitment, goldenSigner)
	require.NoError(t, err)

	shares, err := blob.ToShares()
	require.NoError(t, err)
	assertGoldenShares(t, "blob_v2_fibre", shares)

	parsed, err := ParseBlobs(shares)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, blob.Data(), parsed[0].Data())
	assert.Equal(t, goldenSigner, parsed[0].Signer())
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
