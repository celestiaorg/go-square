package tx_test

import (
	"bytes"
	"testing"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// oldHasBlobTxTypeID is the implementation HasBlobTxTypeID replaced.
func oldHasBlobTxTypeID(b []byte) bool {
	var blobTx v4.BlobTx
	return proto.Unmarshal(b, &blobTx) == nil && blobTx.TypeId == tx.ProtoBlobTxTypeID
}

type envelopeCase struct {
	name  string
	input []byte
	want  bool
}

func TestHasBlobTxTypeIDMatchesBlobTxDecode(t *testing.T) {
	for _, tc := range envelopeCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			_, isBlob, _ := tx.UnmarshalBlobTx(tc.input)
			require.Equal(t, tc.want, tx.HasBlobTxTypeID(tc.input), "new")
			require.Equal(t, tc.want, oldHasBlobTxTypeID(tc.input), "old")
			require.Equal(t, tc.want, isBlob, "UnmarshalBlobTx")
		})
	}
}

func FuzzHasBlobTxTypeID(f *testing.F) {
	for _, tc := range envelopeCases(f) {
		f.Add(tc.input)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		got := tx.HasBlobTxTypeID(b)
		old := oldHasBlobTxTypeID(b)
		_, isBlob, _ := tx.UnmarshalBlobTx(b)
		if got != old || got != isBlob {
			t.Fatalf("new=%v old=%v UnmarshalBlobTx=%v for %x", got, old, isBlob, b)
		}
	})
}

func BenchmarkHasBlobTxTypeID(b *testing.B) {
	blobTx := marshalBlobTx(b, bytes.Repeat([]byte{1}, 300), v0Blob(b, 8500))
	inputs := []struct {
		name  string
		input []byte
	}{
		{"blob_tx_8.8KB", blobTx},
		{"tx_raw_5KB", txRaw(bytes.Repeat([]byte{2}, 4900), bytes.Repeat([]byte{3}, 100), bytes.Repeat([]byte{4}, 64))},
		{"tx_raw_300B", txRaw(bytes.Repeat([]byte{2}, 200), bytes.Repeat([]byte{3}, 30), bytes.Repeat([]byte{4}, 64))},
	}
	impls := []struct {
		name string
		fn   func([]byte) bool
	}{
		{"old", oldHasBlobTxTypeID},
		{"new", tx.HasBlobTxTypeID},
	}
	for _, in := range inputs {
		for _, impl := range impls {
			b.Run(in.name+"/"+impl.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					impl.fn(in.input)
				}
			})
		}
	}
}

func envelopeCases(tb testing.TB) []envelopeCase {
	valid := marshalBlobTx(tb, []byte("inner tx"), v0Blob(tb, 100))
	three := marshalBlobTx(tb, []byte("inner tx"), v0Blob(tb, 10), v1Blob(tb, 600), v0Blob(tb, 3000))
	indexWrapper, err := tx.MarshalIndexWrapper([]byte("inner tx"), 0, 5)
	require.NoError(tb, err)

	field := func(num protowire.Number, typ protowire.Type, payload ...byte) []byte {
		return append(protowire.AppendTag(nil, num, typ), payload...)
	}
	lenField := func(num protowire.Number, payload []byte) []byte {
		return protowire.AppendBytes(protowire.AppendTag(nil, num, protowire.BytesType), payload)
	}
	cat := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
	blobTag := lenField(3, []byte("BLOB"))
	group := cat(field(9, protowire.StartGroupType), field(1, protowire.VarintType, 7), field(9, protowire.EndGroupType))
	bigNum := protowire.AppendVarint(nil, uint64(protowire.MaxValidNumber+1)<<3|uint64(protowire.VarintType))

	return []envelopeCase{
		{"empty", nil, false},
		{"valid one v0 blob", valid, true},
		{"valid three blobs v0 and v1", three, true},
		{"nested blob tx", marshalBlobTx(tb, valid, v0Blob(tb, 10)), true},
		{"index wrapper", indexWrapper, false},
		{"tx raw look-alike", txRaw([]byte("body"), []byte("auth"), []byte("sig")), false},
		{"tx raw with BLOB signature", txRaw([]byte("body"), nil, []byte("BLOB")), true},
		{"tx raw with BLOB signature and non-message auth info", txRaw([]byte("body"), []byte{0xff}, []byte("BLOB")), false},
		{"type id BLOB without blobs", blobTag, true},
		{"wrong type id", lenField(3, []byte("BLOC")), false},
		{"type id last occurrence wins", cat(blobTag, lenField(3, []byte("XXXX"))), false},
		{"type id earlier occurrence overwritten", cat(lenField(3, []byte("XXXX")), blobTag), true},
		{"varint type id then BLOB", cat(field(3, protowire.VarintType, 1), blobTag), true},
		{"BLOB then varint type id", cat(blobTag, field(3, protowire.VarintType, 1)), true},
		{"invalid UTF-8 type id then BLOB", cat(lenField(3, []byte{0xff, 0xfe}), blobTag), false},
		{"varint field 2", cat(field(2, protowire.VarintType, 1), blobTag), true},
		{"fixed64 field 1", cat(field(1, protowire.Fixed64Type, 1, 2, 3, 4, 5, 6, 7, 8), blobTag), true},
		{"unknown field in blob", cat(lenField(2, field(15, protowire.VarintType, 1)), blobTag), true},
		{"wrong wire type namespace in blob", cat(lenField(2, field(1, protowire.VarintType, 1)), blobTag), true},
		{"bytes share version in blob", cat(lenField(2, lenField(3, []byte("x"))), blobTag), true},
		{"group in blob", cat(lenField(2, group), blobTag), true},
		{"group in wrapper", cat(group, blobTag), true},
		{"unterminated group in blob", cat(lenField(2, field(9, protowire.StartGroupType)), blobTag), false},
		{"mismatched end group in blob", cat(lenField(2, cat(field(9, protowire.StartGroupType), field(8, protowire.EndGroupType))), blobTag), false},
		{"lone end group", cat(field(9, protowire.EndGroupType), blobTag), false},
		{"invalid wire type", cat(field(9, 6), blobTag), false},
		{"non-message blob", cat(lenField(2, []byte{0xff}), blobTag), false},
		{"overflowing varint in blob", cat(lenField(2, field(3, protowire.VarintType, bytes.Repeat([]byte{0xff}, 10)...)), blobTag), false},
		{"truncated", valid[:len(valid)-2], false},
		{"truncated blob", cat(protowire.AppendTag(nil, 2, protowire.BytesType), protowire.AppendVarint(nil, 50), []byte("short")), false},
		{"field number 0", cat(field(0, protowire.VarintType, 1), blobTag), false},
		{"field number above max", cat(bigNum, []byte{1}, blobTag), false},
	}
}

func marshalBlobTx(tb testing.TB, inner []byte, blobs ...*share.Blob) []byte {
	b, err := tx.MarshalBlobTx(inner, blobs...)
	require.NoError(tb, err)
	return b
}

func v0Blob(tb testing.TB, size int) *share.Blob {
	blob, err := share.NewV0Blob(share.MustNewV0Namespace(bytes.Repeat([]byte{1}, 10)), bytes.Repeat([]byte{7}, size))
	require.NoError(tb, err)
	return blob
}

func v1Blob(tb testing.TB, size int) *share.Blob {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, 10))
	blob, err := share.NewV1Blob(ns, bytes.Repeat([]byte{8}, size), bytes.Repeat([]byte{9}, share.SignerSize))
	require.NoError(tb, err)
	return blob
}

// txRaw encodes an SDK TxRaw: body_bytes = 1, auth_info_bytes = 2, signatures = 3.
func txRaw(body, authInfo, sig []byte) []byte {
	b := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), body)
	b = protowire.AppendBytes(protowire.AppendTag(b, 2, protowire.BytesType), authInfo)
	return protowire.AppendBytes(protowire.AppendTag(b, 3, protowire.BytesType), sig)
}
