package square_test

import (
	"bytes"
	"encoding/hex"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file pins the full share bytes of a square that exercises every
// region: Tx, PayForBlob, and PayForFibre reserved namespaces, v0 and v1
// blobs in two namespaces, a v2 Fibre system blob, namespace padding,
// reserved padding, and tail padding. Regenerate with:
//
//	go test . -run TestGoldenMixedSquare -update
//
// A diff is a wire-format change in square construction.

var updateSquareGolden = flag.Bool("update", false, "rewrite golden fixtures under testdata/golden")

func repeating(n int, seed byte) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte(int(seed)+i%251) + 1
	}
	return data
}

// buildMixedSquare constructs the square deterministically. Keep the inputs
// fixed forever; changing them invalidates the fixture.
func buildMixedSquare(t *testing.T) square.Square {
	t.Helper()
	builder, err := square.NewBuilder(16, 64)
	require.NoError(t, err)

	// Two normal txs: one small, one spanning the first compact share boundary.
	require.True(t, builder.AppendTx(repeating(100, 0x10)))
	require.True(t, builder.AppendTx(repeating(600, 0x20)))

	nsA := share.MustNewV0Namespace(bytes.Repeat([]byte{0xA}, share.NamespaceVersionZeroIDSize))
	nsB := share.MustNewV0Namespace(bytes.Repeat([]byte{0xB}, share.NamespaceVersionZeroIDSize))
	signer := repeating(share.SignerSize, 0x30)

	// Blob tx 1: two v0 blobs in different namespaces; the second one spans shares.
	blobA, err := share.NewV0Blob(nsA, repeating(300, 0x40))
	require.NoError(t, err)
	blobB, err := share.NewV0Blob(nsB, repeating(1200, 0x50))
	require.NoError(t, err)
	added, err := builder.AppendBlobTx(&tx.BlobTx{Tx: []byte("pfb-one"), Blobs: []*share.Blob{blobA, blobB}})
	require.NoError(t, err)
	require.True(t, added)

	// Blob tx 2: one v1 blob with a signer, in namespace A so it sorts before B.
	blobA1, err := share.NewV1Blob(nsA, repeating(700, 0x60), signer)
	require.NoError(t, err)
	added, err = builder.AppendBlobTx(&tx.BlobTx{Tx: []byte("pfb-two"), Blobs: []*share.Blob{blobA1}})
	require.NoError(t, err)
	require.True(t, added)

	// Fibre tx: PayForFibre compact share plus a v2 system blob in namespace B.
	systemBlob, err := share.NewV2Blob(nsB, 1, repeating(share.FibreCommitmentSize, 0x70), signer)
	require.NoError(t, err)
	added, err = builder.AppendFibreTx(&tx.FibreTx{Tx: []byte("pay-for-fibre"), SystemBlob: systemBlob})
	require.NoError(t, err)
	require.True(t, added)

	sq, err := builder.Export()
	require.NoError(t, err)
	return sq
}

func TestGoldenMixedSquare(t *testing.T) {
	sq := buildMixedSquare(t)

	path := filepath.Join("testdata", "golden", "mixed_square.hex")
	if *updateSquareGolden {
		var buf bytes.Buffer
		for _, s := range sq {
			buf.WriteString(hex.EncodeToString(s.ToBytes()))
			buf.WriteByte('\n')
		}
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "missing fixture %s; run: go test . -run TestGoldenMixedSquare -update", path)
	want := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")

	require.Len(t, sq, len(want), "square share count differs")
	for i := range want {
		assert.Equal(t, want[i], hex.EncodeToString(sq[i].ToBytes()), "share %d differs", i)
	}

	// Structural sanity so a reader can see what the fixture contains.
	size, err := sq.Size()
	require.NoError(t, err)
	assert.Equal(t, 4, size)
	assert.False(t, share.GetShareRangeForNamespace(sq, share.TxNamespace).IsEmpty())
	assert.False(t, share.GetShareRangeForNamespace(sq, share.PayForBlobNamespace).IsEmpty())
	assert.False(t, share.GetShareRangeForNamespace(sq, share.PayForFibreNamespace).IsEmpty())
	assert.False(t, share.GetShareRangeForNamespace(sq, share.TailPaddingNamespace).IsEmpty())

	// The pinned bytes must also decode: 2 txs, 2 wrapped PFBs, 4 blobs.
	txRange := share.GetShareRangeForNamespace(sq, share.TxNamespace)
	txs, err := share.ParseTxs(sq[txRange.Start:txRange.End])
	require.NoError(t, err)
	assert.Len(t, txs, 2)
	pfbs, err := sq.WrappedPFBs()
	require.NoError(t, err)
	assert.Len(t, pfbs, 2)
	blobStart := share.GetShareRangeForNamespace(sq, share.PayForFibreNamespace).End
	blobs, err := share.ParseBlobs(sq[blobStart:])
	require.NoError(t, err)
	assert.Len(t, blobs, 4)
}
