package tx_test

import (
	"bytes"
	"fmt"
	"math"
	"runtime"
	"slices"
	"testing"
	"unicode/utf8"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// refIndexWrapper is an IndexWrapper decoded by refDecodeIndexWrapper.
type refIndexWrapper struct {
	tx           []byte
	shareIndexes []uint32
	typeID       string
	// unknown holds the fields that are not IndexWrapper fields of the
	// expected wire type, in input order, with canonical tags.
	unknown []byte
}

// refDecodeIndexWrapper is an independent field-by-field IndexWrapper decoder
// following the proto3 rules: the last singular field wins, repeated uint32
// accepts packed and unpacked encodings and truncates to 32 bits, strings must
// be valid UTF-8, and a known field with another wire type is an unknown field.
func refDecodeIndexWrapper(b []byte) (refIndexWrapper, bool) {
	var w refIndexWrapper
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 || num > protowire.MaxValidNumber {
			return w, false
		}
		m := protowire.ConsumeFieldValue(num, typ, b[n:])
		if m < 0 {
			return w, false
		}
		value := b[n : n+m]
		b = b[n+m:]
		switch {
		case num == 1 && typ == protowire.BytesType:
			w.tx, _ = protowire.ConsumeBytes(value)
		case num == 2 && typ == protowire.VarintType:
			v, _ := protowire.ConsumeVarint(value)
			w.shareIndexes = append(w.shareIndexes, uint32(v))
		case num == 2 && typ == protowire.BytesType:
			packed, _ := protowire.ConsumeBytes(value)
			for len(packed) > 0 {
				v, k := protowire.ConsumeVarint(packed)
				if k < 0 {
					return w, false
				}
				w.shareIndexes = append(w.shareIndexes, uint32(v))
				packed = packed[k:]
			}
		case num == 3 && typ == protowire.BytesType:
			s, _ := protowire.ConsumeBytes(value)
			if !utf8.Valid(s) {
				return w, false
			}
			w.typeID = string(s)
		default:
			// The decoder keeps the value bytes but re-encodes the tag.
			w.unknown = append(protowire.AppendTag(w.unknown, num, typ), value...)
		}
	}
	return w, true
}

// checkUnmarshalIndexWrapper checks UnmarshalIndexWrapper on arbitrary bytes
// against refDecodeIndexWrapper and, for accepted inputs, the round trips that
// hold. Non-canonical encodings (field order, unpacked or repeated fields,
// overlong varints) mean the input bytes themselves need not round-trip.
func checkUnmarshalIndexWrapper(t *testing.T, b []byte) {
	t.Helper()
	got, ok := tx.UnmarshalIndexWrapper(b)
	require.NotNil(t, got)
	ref, refOK := refDecodeIndexWrapper(b)
	var generated v4.IndexWrapper
	require.Equal(t, refOK, proto.Unmarshal(b, &generated) == nil, "decodability for %x", b)
	require.Equal(t, refOK && ref.typeID == tx.ProtoIndexWrapperTypeID, ok, "verdict for %x", b)
	if !refOK {
		return // the partially decoded value is unspecified
	}
	require.True(t, bytes.Equal(ref.tx, got.Tx), "tx for %x", b)
	require.Equal(t, len(ref.shareIndexes), len(got.ShareIndexes), "share index count for %x", b)
	require.True(t, slices.Equal(ref.shareIndexes, got.ShareIndexes), "share indexes for %x", b)
	require.Equal(t, ref.typeID, got.TypeId, "type id for %x", b)
	require.True(t, bytes.Equal(ref.unknown, got.ProtoReflect().GetUnknown()), "unknown fields for %x", b)
	if !ok {
		return
	}

	// The decoded value is bounded by the input: no amplification.
	require.LessOrEqual(t, len(got.Tx), len(b))
	require.LessOrEqual(t, len(got.ShareIndexes), len(b))
	// Square construction relies on a tx never being both kinds of envelope.
	require.False(t, tx.HasBlobTxTypeID(b), "both IndexWrapper and BlobTx: %x", b)

	// Marshal then unmarshal returns an equal value, and re-encoding is stable.
	marshal := proto.MarshalOptions{Deterministic: true}
	enc, err := marshal.Marshal(got)
	require.NoError(t, err)
	again, ok := tx.UnmarshalIndexWrapper(enc)
	require.True(t, ok)
	require.True(t, proto.Equal(got, again), "re-decoded value differs for %x", b)
	enc2, err := marshal.Marshal(again)
	require.NoError(t, err)
	require.Equal(t, enc, enc2)

	// MarshalIndexWrapper keeps the known fields and drops unknown ones.
	canonical, err := tx.MarshalIndexWrapper(got.Tx, got.ShareIndexes...)
	require.NoError(t, err)
	back, ok := tx.UnmarshalIndexWrapper(canonical)
	require.True(t, ok)
	require.True(t, bytes.Equal(got.Tx, back.Tx))
	require.True(t, slices.Equal(got.ShareIndexes, back.ShareIndexes))
	require.Empty(t, back.ProtoReflect().GetUnknown())
	if len(ref.unknown) == 0 {
		require.Equal(t, enc, canonical)
	}
}

func FuzzUnmarshalIndexWrapper(f *testing.F) {
	for _, tc := range indexWrapperCases(f) {
		f.Add(tc.input)
	}
	f.Fuzz(checkUnmarshalIndexWrapper)
}

func TestUnmarshalIndexWrapperCases(t *testing.T) {
	for _, tc := range indexWrapperCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := tx.UnmarshalIndexWrapper(tc.input)
			require.Equal(t, tc.want, ok)
			checkUnmarshalIndexWrapper(t, tc.input)
		})
	}
}

// TestUnmarshalIndexWrapperAllocationBounded checks that inputs claiming huge
// lengths or element counts allocate in proportion to their actual size.
func TestUnmarshalIndexWrapperAllocationBounded(t *testing.T) {
	for _, tc := range indexWrapperCases(t) {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		for range 10 {
			tx.UnmarshalIndexWrapper(tc.input)
		}
		runtime.ReadMemStats(&after)
		perCall := (after.TotalAlloc - before.TotalAlloc) / 10
		require.LessOrEqual(t, perCall, uint64(16*len(tc.input)+64<<10), "%s: %d bytes allocated for a %d byte input", tc.name, perCall, len(tc.input))
	}
}

type indexWrapperCase struct {
	name  string
	input []byte
	want  bool
}

func indexWrapperCases(tb testing.TB) []indexWrapperCase {
	tb.Helper()
	mustMarshal := func(inner []byte, indexes ...uint32) []byte {
		b, err := tx.MarshalIndexWrapper(inner, indexes...)
		require.NoError(tb, err)
		return b
	}
	typeID := bytesField(3, []byte(tx.ProtoIndexWrapperTypeID))
	inner := bytesField(1, []byte("inner tx"))
	packed := func(values ...uint64) rawField {
		var b []byte
		for _, v := range values {
			b = protowire.AppendVarint(b, v)
		}
		return bytesField(2, b)
	}
	// claimsLength is a field whose length prefix exceeds the bytes that follow.
	claimsLength := func(num protowire.Number, n uint64) rawField {
		return rawField{protowire.AppendTag(nil, num, protowire.BytesType), protowire.AppendVarint(nil, n)}
	}
	valid := mustMarshal([]byte("inner tx"), 0, 5, math.MaxUint32)
	blobTx, err := proto.Marshal(&v4.BlobTx{Tx: []byte("inner tx"), TypeId: tx.ProtoBlobTxTypeID})
	require.NoError(tb, err)
	packed64K := bytesField(2, bytes.Repeat([]byte{1}, 1<<16))

	cases := make([]indexWrapperCase, 0, 40)
	cases = append(cases, []indexWrapperCase{
		{"empty", nil, false},
		{"valid", valid, true},
		{"valid without tx or indexes", mustMarshal(nil), true},
		{"valid many indexes", mustMarshal([]byte{1}, slices.Repeat([]uint32{math.MaxUint32, 0, 1 << 20}, 100)...), true},
		{"truncated", valid[:len(valid)-1], false},
		{"truncated type id length", valid[:len(valid)-len(tx.ProtoIndexWrapperTypeID)-1], false},
		{"BlobTx", blobTx, false},
		{"wrong type id", encodeFields([]rawField{inner, bytesField(3, []byte("BLOB"))}), false},
		{"short type id", encodeFields([]rawField{inner, bytesField(3, []byte("IND"))}), false},
		{"long type id", encodeFields([]rawField{inner, bytesField(3, []byte("INDXX"))}), false},
		{"invalid UTF-8 type id", encodeFields([]rawField{inner, bytesField(3, []byte{0xff, 'N', 'D', 'X'})}), false},
		{"type id then wrong type id", encodeFields([]rawField{typeID, bytesField(3, []byte("BLOB"))}), false},
		{"wrong type id then type id", encodeFields([]rawField{bytesField(3, []byte("BLOB")), typeID}), true},
		{"type id as varint", encodeFields([]rawField{inner, varintField(3, 4)}), false},
		{"reordered fields", encodeFields([]rawField{typeID, packed(7), inner}), true},
		{"unpacked indexes", encodeFields([]rawField{inner, varintField(2, 1), varintField(2, 2), typeID}), true},
		{"mixed packed and unpacked", encodeFields([]rawField{packed(1, 2), varintField(2, 3), packed(4), typeID}), true},
		{"64-bit index truncated", encodeFields([]rawField{packed(math.MaxUint64, 1<<32+7), typeID}), true},
		{"overlong index varint", encodeFields([]rawField{bytesField(2, overlongVarint(5)), typeID}), true},
		{"11-byte index varint", encodeFields([]rawField{bytesField(2, append(bytes.Repeat([]byte{0x80}, 10), 1)), typeID}), false},
		{"unterminated packed varint", encodeFields([]rawField{bytesField(2, []byte{1, 0x80}), typeID}), false},
		{"packed length 2^62", encodeFields([]rawField{typeID, claimsLength(2, 1<<62)}), false},
		{"packed length 2^31", encodeFields([]rawField{typeID, claimsLength(2, 1<<31), {value: []byte{1, 2, 3}}}), false},
		{"tx length 2^63", encodeFields([]rawField{typeID, claimsLength(1, 1<<63)}), false},
		{"packed 64 KiB indexes", encodeFields([]rawField{packed64K, typeID}), true},
		{"tx as varint", encodeFields([]rawField{varintField(1, 1), typeID}), true},
		{"indexes as fixed32", encodeFields([]rawField{{protowire.AppendTag(nil, 2, protowire.Fixed32Type), []byte{1, 0, 0, 0}}, typeID}), true},
		{"unknown field", encodeFields([]rawField{inner, varintField(99, 1), typeID}), true},
		{"overlong unknown tag", encodeFields([]rawField{{overlongVarint(uint64(protowire.EncodeTag(4, protowire.VarintType))), []byte{0x30}}, typeID}), true},
		{"unknown group", encodeFields([]rawField{{protowire.AppendTag(nil, 4, protowire.StartGroupType), protowire.AppendTag(nil, 4, protowire.EndGroupType)}, typeID}), true},
		{"stray end group", encodeFields([]rawField{{protowire.AppendTag(nil, 4, protowire.EndGroupType), nil}, typeID}), false},
		{"field number 0", encodeFields([]rawField{varintField(0, 1), typeID}), false},
		{"overlong tag", append(overlongVarint(uint64(protowire.EncodeTag(3, protowire.BytesType))), valid[len(valid)-len(tx.ProtoIndexWrapperTypeID)-1:]...), true},
		{"field number above max", append(protowire.AppendVarint(nil, uint64(protowire.MaxValidNumber+1)<<3), 0), false},
	}...)
	// Overlong length prefixes and values for each field of a valid wrapper.
	fields, ok := splitFields(valid)
	require.True(tb, ok)
	for i := range fields {
		cases = append(cases, indexWrapperCase{fmt.Sprint("overlong value ", i), encodeFields(overlongValue(fields, i)), true})
	}
	return cases
}
