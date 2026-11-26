package square_test

import (
	"bytes"
	"crypto/sha256"
	"slices"
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/inclusion"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/celestiaorg/nmt"
	"github.com/stretchr/testify/require"
)

// FuzzSquareLayout checks the positions that square construction reports
// against the shares it actually wrote. Proofs and commitments depend on these
// positions:
//   - every tx's share range (Builder.FindTxShareRange and TxShareRange, used for
//     tx inclusion proofs) matches the byte offsets of the tx in its compact
//     share sequence,
//   - every PFB blob's range (FindBlobStartingIndex, BlobShareLength,
//     BlobShareRange) matches the index in its wrapped PFB and holds exactly that
//     blob,
//   - every blob starts where the non-interactive default rules
//     (BlobSharesUsedNonInteractiveDefaults) place it, and
//   - the subtree roots of each blob's commitment are aligned subtrees of a
//     square row, with the same roots as computed from the blob alone.
func FuzzSquareLayout(f *testing.F) {
	blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte{1, 0}), []byte("raw blob"))
	require.NoError(f, err)
	rawBlobTx, err := tx.MarshalBlobTx([]byte{0xff, 1}, blob)
	require.NoError(f, err)
	for _, ops := range fuzzBuilderSeeds() {
		for _, size := range []uint8{1, 3, 5} {
			f.Add(ops, rawBlobTx, size, uint8(63))
			f.Add(ops, []byte("normal"), size, uint8(0))
		}
	}
	f.Add([]byte{}, []byte{}, uint8(0), uint8(0))
	f.Fuzz(func(t *testing.T, ops, raw []byte, size, threshold uint8) {
		if len(ops) > maxFuzzOps*fuzzOpSize || len(raw) > maxFuzzRawSize {
			t.Skip()
		}
		checkSquareLayout(t, ops, raw, size, threshold)
	})
}

// TestSquareLayoutSeeds asserts that the seeds build non-trivial squares, so the
// layout checks are not vacuous when the target runs as a unit test.
func TestSquareLayoutSeeds(t *testing.T) {
	blobs, wide := 0, 0
	for _, ops := range fuzzBuilderSeeds() {
		for _, threshold := range []uint8{0, 63} {
			n, w := checkSquareLayout(t, ops, nil, 5, threshold)
			blobs += n
			wide += w
		}
	}
	require.Positive(t, blobs)
	require.Positive(t, wide, "no blob had a subtree wider than one share")
}

func checkSquareLayout(t *testing.T, ops, raw []byte, size, threshold uint8) (checkedBlobs, wideSubtrees int) {
	t.Helper()
	maxSize := 1 << (size % 6)
	thr := 1 + int(threshold%128)
	generated := fuzzTxs(t, ops, raw)
	txs := make([]square.ClassifiedTx, 0, len(generated))
	for _, ft := range generated {
		txs = append(txs, ft.c)
	}
	// Construct requires normal txs, then PFBs, then pay-for-fibre txs.
	slices.SortStableFunc(txs, func(a, b square.ClassifiedTx) int { return fuzzCategory(a) - fuzzCategory(b) })

	sq, err := square.Construct(txs, maxSize, thr)
	if err != nil {
		return 0, 0 // capacity, or a malformed raw tx
	}
	assertFuzzSquare(t, sq, maxSize)
	width, err := sq.Size()
	require.NoError(t, err)
	minSquare, err := inclusion.BlobMinSquareSize(len(sq))
	require.NoError(t, err)
	require.Equal(t, width, minSquare)
	require.Equal(t, len(txs) == 0, sq.IsEmpty())
	require.Equal(t, sha256.Sum256(sq.ToBytes()), sq.Hash())

	b, err := square.NewBuilder(maxSize, thr, txs...)
	require.NoError(t, err)
	exported, err := b.Export()
	require.NoError(t, err)
	require.True(t, exported.Equals(sq))
	require.Equal(t, thr, b.SubtreeRootThreshold())
	require.Equal(t, len(txs), b.NumTxs())

	wrapped, err := sq.WrappedPFBs()
	require.NoError(t, err)
	require.Equal(t, b.NumPFBs(), len(wrapped))

	// The bytes each tx has in the square, in square order.
	sections := [3][][]byte{}
	pfbs := 0
	for _, c := range txs {
		switch fuzzCategory(c) {
		case 0:
			sections[0] = append(sections[0], c.Bytes)
		case 1:
			sections[1] = append(sections[1], wrapped[pfbs])
			pfbs++
		case 2:
			sections[2] = append(sections[2], c.FibreTx.Tx)
		}
	}
	require.Equal(t, len(wrapped), pfbs)

	txIndex := 0
	for i, ns := range []share.Namespace{share.TxNamespace, share.PayForBlobNamespace, share.PayForFibreNamespace} {
		r := share.GetShareRangeForNamespace(sq, ns)
		stream := compactStream(sq[r.Start:r.End])
		offset := 0
		for _, txBytes := range sections[i] {
			delimited, err := share.MarshalDelimitedTx(txBytes)
			require.NoError(t, err)
			require.Equal(t, delimited, stream[offset:offset+len(delimited)], "tx %d is not where its offset says", txIndex)
			want := share.NewRange(r.Start+compactShareOf(offset), r.Start+compactShareOf(offset+len(delimited)-1)+1)
			got, err := b.FindTxShareRange(txIndex)
			require.NoError(t, err)
			require.Equal(t, want, got, "share range of tx %d (namespace %s)", txIndex, ns)
			if txIndex == 0 || txIndex == len(txs)-1 {
				got, err := square.TxShareRange(txs, txIndex, maxSize, thr)
				require.NoError(t, err)
				require.Equal(t, want, got)
			}
			if i == 1 {
				iw, err := b.GetWrappedPFB(txIndex)
				require.NoError(t, err)
				decoded, ok := tx.UnmarshalIndexWrapper(txBytes)
				require.True(t, ok)
				require.Equal(t, decoded.Tx, iw.Tx)
				require.Equal(t, decoded.ShareIndexes, iw.ShareIndexes)
			}
			offset += len(delimited)
			txIndex++
		}
	}
	_, err = b.FindTxShareRange(len(txs))
	require.Error(t, err)

	// Check every PFB blob against its wrapped PFB and its commitment.
	pfbIndex := len(sections[0])
	for _, c := range txs {
		if fuzzCategory(c) != 1 {
			continue
		}
		blobTx, _, err := tx.UnmarshalBlobTx(c.Bytes)
		require.NoError(t, err)
		iw, ok := tx.UnmarshalIndexWrapper(wrapped[pfbIndex-len(sections[0])])
		require.True(t, ok)
		require.Len(t, iw.ShareIndexes, len(blobTx.Blobs))
		for j, blob := range blobTx.Blobs {
			start, err := b.FindBlobStartingIndex(pfbIndex, j)
			require.NoError(t, err)
			length, err := b.BlobShareLength(pfbIndex, j)
			require.NoError(t, err)
			require.Equal(t, int(iw.ShareIndexes[j]), start)
			require.Equal(t, share.SparseSharesNeeded(uint32(len(blob.Data())), blob.HasSigner()), length)
			r, err := square.BlobShareRange(txs, pfbIndex, j, maxSize, thr)
			require.NoError(t, err)
			require.Equal(t, share.NewRange(start, start+length), r)
			if checkBlobInSquare(t, sq, width, thr, blob, start, length) > 1 {
				wideSubtrees++
			}
			checkedBlobs++
		}
		_, err = b.FindBlobStartingIndex(pfbIndex, len(blobTx.Blobs))
		require.Error(t, err)
		pfbIndex++
	}
	checkBlobStarts(t, sq, thr)
	return checkedBlobs, wideSubtrees
}

// compactStream concatenates the data of a compact share sequence, after the
// sequence length and reserved bytes.
func compactStream(shares []share.Share) []byte {
	var stream []byte
	for _, s := range shares {
		stream = append(stream, s.RawData()...)
	}
	return stream
}

// compactShareOf returns the share, relative to the start of a compact
// sequence, that holds the byte at offset in the sequence's data.
func compactShareOf(offset int) int {
	if offset < share.FirstCompactShareContentSize {
		return 0
	}
	return 1 + (offset-share.FirstCompactShareContentSize)/share.ContinuationCompactShareContentSize
}

// checkBlobInSquare returns the blob's subtree width.
func checkBlobInSquare(t *testing.T, sq square.Square, width, thr int, blob *share.Blob, start, length int) int {
	t.Helper()
	require.LessOrEqual(t, start+length, len(sq))
	parsed, err := share.ParseBlobs(sq[start : start+length])
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	require.True(t, blob.Namespace().Equals(parsed[0].Namespace()))
	require.Equal(t, blob.Data(), parsed[0].Data())
	require.Equal(t, blob.ShareVersion(), parsed[0].ShareVersion())
	require.Equal(t, blob.Signer(), parsed[0].Signer())

	subtreeWidth, err := inclusion.SubTreeWidth(length, thr)
	require.NoError(t, err)
	sizes, err := inclusion.MerkleMountainRangeSizes(uint64(length), uint64(subtreeWidth))
	require.NoError(t, err)
	want, err := inclusion.GenerateSubtreeRoots(blob, thr)
	require.NoError(t, err)
	require.Len(t, want, len(sizes))
	cursor := start
	for i, treeSize := range sizes {
		// A subtree root is a node of the row's NMT only if the subtree is
		// aligned to its size and lies within one row.
		require.Zero(t, cursor%int(treeSize), "subtree %d of size %d starts at %d", i, treeSize, cursor)
		require.LessOrEqual(t, int(treeSize), width)
		tree := nmt.New(sha256.New(), nmt.NamespaceIDSize(share.NamespaceSize), nmt.IgnoreMaxNamespace(true))
		for _, s := range sq[cursor : cursor+int(treeSize)] {
			require.NoError(t, tree.Push(append(bytes.Clone(s.Namespace().Bytes()), s.ToBytes()...)))
		}
		root, err := tree.Root()
		require.NoError(t, err)
		require.Equal(t, want[i], root, "subtree %d root differs from the commitment's", i)
		cursor += int(treeSize)
	}
	return subtreeWidth
}

// checkBlobStarts finds every blob sequence in the square, including Fibre
// system blobs, and checks that the non-interactive default rules place them
// exactly where they are, with only namespace padding in between.
func checkBlobStarts(t *testing.T, sq square.Square, thr int) {
	t.Helper()
	var starts []uint32
	var lens []int
	for i := range sq {
		s := sq[i]
		if s.Namespace().IsReserved() || !s.IsSequenceStart() || s.IsPadding() {
			continue
		}
		starts = append(starts, uint32(i))
		lens = append(lens, share.SparseSharesNeeded(s.SequenceLen(), s.ContainsSigner()))
	}
	if len(starts) == 0 {
		return
	}
	used, indexes, err := inclusion.BlobSharesUsedNonInteractiveDefaults(int(starts[0]), thr, lens...)
	require.NoError(t, err)
	require.Equal(t, starts, indexes)
	last := len(starts) - 1
	require.Equal(t, int(starts[last])+lens[last]-int(starts[0]), used)
	for k := 1; k < len(starts); k++ {
		for i := int(starts[k-1]) + lens[k-1]; i < int(starts[k]); i++ {
			require.True(t, sq[i].IsPadding(), "share %d between blobs is not padding", i)
			require.True(t, sq[i].Namespace().Equals(sq[starts[k-1]].Namespace()), "padding takes the previous blob's namespace")
		}
	}
}
