package square_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/internal/test"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

// Kinds of transaction fuzzTxs generates. fuzzRaw is a slice of fuzzer bytes,
// which may be a normal tx, a valid or malformed BlobTx, or empty.
const (
	fuzzNormal = iota
	fuzzBlob
	fuzzFibre
	fuzzRaw
)

const (
	fuzzOpSize     = 4
	maxFuzzOps     = 64
	maxFuzzRawSize = 4096
)

type fuzzTx struct {
	kind   int
	revert bool
	c      square.ClassifiedTx
}

// fuzzTxs decodes at most maxFuzzOps transactions from ops. Each op is four
// bytes: a head byte (kind in the low two bits, revert flag in the top bit), a
// little-endian size parameter, and a selector for namespaces, blob count and
// versions. Generated payloads are bounded to a few KiB per blob.
func fuzzTxs(t *testing.T, ops, raw []byte) []fuzzTx {
	t.Helper()
	var txs []fuzzTx
	for i := 0; i+fuzzOpSize <= len(ops) && len(txs) < maxFuzzOps; i += fuzzOpSize {
		head, sel := ops[i], ops[i+3]
		param := int(binary.LittleEndian.Uint16(ops[i+1:]))
		ft := fuzzTx{kind: int(head % 4), revert: head&0x80 != 0}
		switch ft.kind {
		case fuzzNormal:
			// The 0xff prefix is an invalid protobuf tag, so this is never a BlobTx.
			ft.c = square.NewClassifiedTx(fuzzPayload(0xff, i, 1+param%1200))
		case fuzzBlob:
			ft.c = square.NewClassifiedTx(fuzzBlobTx(t, i, param, sel))
		case fuzzFibre:
			ft.c = fuzzFibreTx(t, i, param, sel)
		case fuzzRaw:
			n := min(param%600, len(raw))
			ft.c = square.NewClassifiedTx(raw[:n:n])
			raw = raw[n:]
		}
		txs = append(txs, ft)
	}
	return txs
}

func fuzzPayload(prefix byte, seed, size int) []byte {
	b := make([]byte, size)
	b[0] = prefix
	for j := 1; j < size; j++ {
		b[j] = byte(seed*31 + j)
	}
	return b
}

func fuzzBlobTx(t *testing.T, seed, param int, sel byte) []byte {
	t.Helper()
	blobs := make([]*share.Blob, 1+int(sel%3))
	for j := range blobs {
		ns := share.MustNewV0Namespace([]byte{1, (sel>>2 + byte(j)) % 4})
		data := fuzzPayload(byte(j), seed+j, 1+(param*(j+1)+j*131)%2048)
		var err error
		if sel&0x40 != 0 {
			blobs[j], err = share.NewV1Blob(ns, data, bytes.Repeat([]byte{byte(j + 1)}, share.SignerSize))
		} else {
			blobs[j], err = share.NewV0Blob(ns, data)
		}
		require.NoError(t, err)
	}
	raw, err := tx.MarshalBlobTx(fuzzPayload(0xff, seed, 1+param%300), blobs...)
	require.NoError(t, err)
	return raw
}

// fuzzFibreTx uses namespaces disjoint from fuzzBlobTx: Export orders blobs of
// one namespace by append order, which Construct's tx ordering would change.
func fuzzFibreTx(t *testing.T, seed, param int, sel byte) square.ClassifiedTx {
	t.Helper()
	commitment := sha256.Sum256([]byte{byte(seed), byte(param), sel})
	// The 0xfe prefix is an invalid protobuf tag, so this is never a BlobTx.
	c, err := test.BuildFibreClassifiedTx(fuzzPayload(0xfe, seed, 1+param%300),
		share.MustNewV0Namespace([]byte{0xf0, sel % 3}), commitment[:],
		bytes.Repeat([]byte{0xf}, share.SignerSize), uint32(sel))
	require.NoError(t, err)
	return c
}

// fuzzCategory is the section of the square c belongs to: 0 normal txs, 1 PFBs,
// 2 pay-for-fibre txs.
func fuzzCategory(c square.ClassifiedTx) int {
	switch {
	case c.FibreTx != nil:
		return 2
	case tx.HasBlobTxTypeID(c.Bytes):
		return 1
	default:
		return 0
	}
}

func fuzzBuilderSeeds() [][]byte {
	op := func(kind byte, revert bool, param uint16, sel byte) []byte {
		if revert {
			kind |= 0x80
		}
		return []byte{kind, byte(param), byte(param >> 8), sel}
	}
	return [][]byte{
		slices.Concat(op(fuzzNormal, false, 10, 0), op(fuzzBlob, false, 100, 0), op(fuzzFibre, false, 5, 0), op(fuzzRaw, false, 599, 0)),
		slices.Concat(op(fuzzBlob, false, 470, 0x41), op(fuzzNormal, true, 900, 0), op(fuzzFibre, true, 7, 1),
			op(fuzzBlob, true, 2000, 2), op(fuzzNormal, false, 1199, 0), op(fuzzFibre, false, 9, 2), op(fuzzRaw, false, 3, 0)),
		bytes.Repeat(op(fuzzBlob, false, 1500, 0x46), 16),
		bytes.Repeat(slices.Concat(op(fuzzNormal, false, 400, 0), op(fuzzFibre, true, 1, 0), op(fuzzBlob, false, 700, 5)), 8),
	}
}

func FuzzBuilderParity(f *testing.F) {
	blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte{1, 0}), []byte("raw blob"))
	require.NoError(f, err)
	rawBlobTx, err := tx.MarshalBlobTx([]byte{0xff, 1}, blob)
	require.NoError(f, err)
	for _, ops := range fuzzBuilderSeeds() {
		for _, size := range []uint8{0, 2, 3} {
			f.Add(ops, rawBlobTx, size, uint8(64))
			f.Add(ops, []byte("normal"), size, uint8(1))
		}
	}
	f.Add(fuzzBuilderSeeds()[0], rawBlobTx[:len(rawBlobTx)-1], uint8(3), uint8(0))
	f.Fuzz(func(t *testing.T, ops, raw []byte, size, threshold uint8) {
		if len(ops) > maxFuzzOps*fuzzOpSize || len(raw) > maxFuzzRawSize {
			t.Skip()
		}
		checkBuilderParity(t, ops, raw, size, threshold)
	})
}

// TestBuilderParitySeeds asserts the seeds reach the accept and revert paths
// for every kind of transaction, not only rejections.
func TestBuilderParitySeeds(t *testing.T) {
	blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte{1, 0}), []byte("raw blob"))
	require.NoError(t, err)
	rawBlobTx, err := tx.MarshalBlobTx([]byte{0xff, 1}, blob)
	require.NoError(t, err)
	var kept, reverted [4]int
	for _, ops := range fuzzBuilderSeeds() {
		k, r := checkBuilderParity(t, ops, rawBlobTx, 3, 64)
		for i := range kept {
			kept[i] += k[i]
			reverted[i] += r[i]
		}
	}
	for kind := range kept {
		require.Positive(t, kept[kind], "kind %d never kept", kind)
	}
	for _, kind := range []int{fuzzNormal, fuzzBlob, fuzzFibre} {
		require.Positive(t, reverted[kind], "kind %d never reverted", kind)
	}
}

// checkBuilderParity appends the transactions decoded from ops one at a time,
// asserting that
//   - Fits reports an error only for transactions the Append methods reject,
//     and otherwise reports exactly whether the append succeeds,
//   - a rejected append leaves Export unchanged,
//   - a revert right after a successful append restores the previous Export,
//   - the final Export equals Construct over the kept transactions.
//
// PrepareProposal relies on the first property to filter transactions, and on
// the last for ProcessProposal to rebuild the same square.
func checkBuilderParity(t *testing.T, ops, raw []byte, size, threshold uint8) (kept, reverted [4]int) {
	t.Helper()
	maxSize := 1 << (size % 4)
	b, err := square.NewBuilder(maxSize, int(threshold))
	require.NoError(t, err)
	var keptTxs [3][]square.ClassifiedTx
	for i, ft := range fuzzTxs(t, ops, raw) {
		before, beforeSize := exportBytes(t, b), b.CurrentSize()
		fits, fitsErr := b.Fits(ft.c)
		category, added, appendErr := appendFuzzTx(b, ft.c)
		if fitsErr != nil {
			require.False(t, added, "tx %d: Fits failed (%v) but append succeeded", i, fitsErr)
		} else {
			require.NoError(t, appendErr, "tx %d: Fits accepted the tx but append failed", i)
			require.Equal(t, fits, added, "tx %d: Fits and append disagree", i)
		}
		if !added {
			require.Equal(t, before, exportBytes(t, b), "tx %d: a rejected append changed the square", i)
			require.Equal(t, beforeSize, b.CurrentSize(), "tx %d: a rejected append changed the size", i)
			continue
		}
		if ft.revert {
			require.NoError(t, revertFuzzTx(b, category), "tx %d", i)
			require.Equal(t, before, exportBytes(t, b), "tx %d: revert did not restore the square", i)
			require.Equal(t, beforeSize, b.CurrentSize(), "tx %d: revert did not restore the size", i)
			reverted[ft.kind]++
			continue
		}
		keptTxs[category] = append(keptTxs[category], ft.c)
		kept[ft.kind]++
	}
	got, err := b.Export()
	require.NoError(t, err)
	want, err := square.Construct(slices.Concat(keptTxs[0], keptTxs[1], keptTxs[2]), maxSize, int(threshold))
	require.NoError(t, err, "Construct rejected the transactions the builder kept")
	require.True(t, got.Equals(want), "builder square differs from Construct")
	return kept, reverted
}

// appendFuzzTx appends c with the Append method for its kind, decoding a blob
// tx with tx.UnmarshalBlobTx as an application holding raw bytes would.
func appendFuzzTx(b *square.Builder, c square.ClassifiedTx) (int, bool, error) {
	category := fuzzCategory(c)
	switch category {
	case 2:
		added, err := b.AppendFibreTx(c.FibreTx)
		return category, added, err
	case 1:
		blobTx, _, err := tx.UnmarshalBlobTx(c.Bytes)
		if err != nil {
			return category, false, err
		}
		added, err := b.AppendBlobTx(blobTx)
		return category, added, err
	default:
		return category, b.AppendTx(c.Bytes), nil
	}
}

func revertFuzzTx(b *square.Builder, category int) error {
	switch category {
	case 2:
		return b.RevertLastPayForFibreTx()
	case 1:
		return b.RevertLastBlobTx()
	default:
		return b.RevertLastTx()
	}
}

func exportBytes(t *testing.T, b *square.Builder) []byte {
	t.Helper()
	sq, err := b.Export()
	require.NoError(t, err)
	return sq.ToBytes()
}
