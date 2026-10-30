package square_test

import (
	"bytes"
	"encoding/binary"
	"slices"
	"sort"
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

const (
	maxClassifiedOps = 32
	mutationSize     = 4
	maxMutations     = 16
)

// FuzzClassifiedTxParity asserts that a block classified with pre-decoded blob
// txs (as ProcessProposal does) builds exactly the square, share ranges and
// errors that the same block classified from raw bytes builds. Batches are
// generated from structured parameters and raw bytes, optionally put into
// canonical order, then mutated: each mutation is four bytes (tx index, a
// little-endian offset, and an XOR mask; a zero mask truncates at the offset).
func FuzzClassifiedTxParity(f *testing.F) {
	blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte{1, 0}), []byte("raw blob"))
	require.NoError(f, err)
	rawBlobTx, err := tx.MarshalBlobTx([]byte{0xff, 1}, blob)
	require.NoError(f, err)
	for _, ops := range fuzzBuilderSeeds() {
		if len(ops) > maxClassifiedOps*fuzzOpSize {
			ops = ops[:maxClassifiedOps*fuzzOpSize]
		}
		f.Add(ops, rawBlobTx, []byte{}, true, uint8(4), uint8(64), uint8(1))
		f.Add(ops, rawBlobTx, []byte{}, false, uint8(4), uint8(64), uint8(0))
		f.Add(ops, rawBlobTx, []byte{1, 40, 0, 0x08, 2, 3, 0, 0}, true, uint8(3), uint8(1), uint8(2))
		f.Add(ops, rawBlobTx, []byte{1, 0xff, 0xff, 0x80, 3, 7, 0, 1}, true, uint8(4), uint8(8), uint8(5))
	}
	f.Fuzz(func(t *testing.T, ops, raw, mutations []byte, canonical bool, size, threshold, index uint8) {
		if len(ops) > maxClassifiedOps*fuzzOpSize || len(raw) > maxFuzzRawSize || len(mutations) > maxMutations*mutationSize {
			t.Skip()
		}
		checkClassifiedParity(t, ops, raw, mutations, canonical, size, threshold, index)
	})
}

// TestClassifiedTxParitySeeds asserts the seeds build squares through both
// paths, not only rejections.
func TestClassifiedTxParitySeeds(t *testing.T) {
	blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte{1, 0}), []byte("raw blob"))
	require.NoError(t, err)
	rawBlobTx, err := tx.MarshalBlobTx([]byte{0xff, 1}, blob)
	require.NoError(t, err)
	built := 0
	for _, ops := range fuzzBuilderSeeds() {
		if checkClassifiedParity(t, ops[:min(len(ops), maxClassifiedOps*fuzzOpSize)], rawBlobTx, nil, true, 4, 64, 1) {
			built++
		}
	}
	require.Equal(t, len(fuzzBuilderSeeds()), built)
}

func checkClassifiedParity(t *testing.T, ops, raw, mutations []byte, canonical bool, size, threshold, index uint8) bool {
	t.Helper()
	txs := fuzzTxs(t, ops, raw)
	if len(txs) == 0 {
		return false
	}
	entries := make([]square.ClassifiedTx, len(txs))
	for i, ft := range txs {
		entries[i] = ft.c
		entries[i].Bytes = bytes.Clone(ft.c.Bytes)
	}
	for m := 0; m+mutationSize <= len(mutations); m += mutationSize {
		e := &entries[int(mutations[m])%len(entries)]
		if len(e.Bytes) == 0 {
			continue
		}
		pos := int(binary.LittleEndian.Uint16(mutations[m+1:])) % len(e.Bytes)
		if mask := mutations[m+3]; mask == 0 {
			e.Bytes = e.Bytes[:pos]
		} else {
			e.Bytes[pos] ^= mask
		}
	}
	for i := range entries {
		if e := &entries[i]; e.FibreTx != nil {
			// Keep FibreTx.Tx equal to Bytes; Validate checks the rest.
			e.FibreTx = &tx.FibreTx{Tx: e.Bytes, SystemBlob: e.FibreTx.SystemBlob}
		}
	}
	if canonical {
		sort.SliceStable(entries, func(i, j int) bool { return fuzzCategory(entries[i]) < fuzzCategory(entries[j]) })
	}

	fromBytes := slices.Clone(entries)
	preDecoded := slices.Clone(entries)
	for i, e := range entries {
		if e.FibreTx != nil {
			continue
		}
		decoded, isBlobTx, err := tx.UnmarshalBlobTx(e.Bytes)
		if !isBlobTx || err != nil {
			continue
		}
		preDecoded[i], err = square.NewClassifiedBlobTx(e.Bytes, decoded)
		require.NoError(t, err, "tx %d: Validate rejected what UnmarshalBlobTx returned", i)
	}

	maxSize, thr := 1<<(size%5), int(threshold)
	want, wantErr := square.Construct(fromBytes, maxSize, thr)
	got, gotErr := square.Construct(preDecoded, maxSize, thr)
	requireSameError(t, wantErr, gotErr, "Construct")
	require.True(t, want.Equals(got), "pre-decoded square differs")
	built := wantErr == nil

	txIndex := int(index) % (len(entries) + 1)
	wantRange, wantErr := square.TxShareRange(fromBytes, txIndex, maxSize, thr)
	gotRange, gotErr := square.TxShareRange(preDecoded, txIndex, maxSize, thr)
	requireSameError(t, wantErr, gotErr, "TxShareRange")
	require.Equal(t, wantRange, gotRange)

	blobIndex := int(index>>6) % 3
	wantRange, wantErr = square.BlobShareRange(fromBytes, txIndex, blobIndex, maxSize, thr)
	gotRange, gotErr = square.BlobShareRange(preDecoded, txIndex, blobIndex, maxSize, thr)
	requireSameError(t, wantErr, gotErr, "BlobShareRange")
	require.Equal(t, wantRange, gotRange)
	return built
}

func requireSameError(t *testing.T, want, got error, what string) {
	t.Helper()
	if want == nil || got == nil {
		require.Equal(t, want, got, "%s: verdicts differ", what)
		return
	}
	require.Equal(t, want.Error(), got.Error(), "%s: errors differ", what)
}
