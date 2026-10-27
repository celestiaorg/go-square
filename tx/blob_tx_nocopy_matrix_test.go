package tx_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
)

// The tests in this file compare UnmarshalBlobTxNoCopy with UnmarshalBlobTx
// on inputs built to sit at the edges of the canonical encoding: length
// prefixes and integers at their varint size boundaries, every field
// mutated in every way, and fuzzed BlobTxs that start out valid.

// varintBoundaries are the largest values that fit in 1, 2, 3 and 4 varint
// bytes. Each boundary and the value after it are tested.
var varintBoundaries = []int{1<<7 - 1, 1<<14 - 1, 1<<21 - 1}

func TestUnmarshalBlobTxNoCopyLengthBoundaries(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, 10)).ID()
	accepted := 0
	check := func(name string, b []byte) {
		t.Run(name, func(t *testing.T) {
			requireSameAsUnmarshalBlobTx(t, b)
			if _, _, err := tx.UnmarshalBlobTx(b); err == nil {
				accepted++
			}
		})
	}
	for _, boundary := range varintBoundaries {
		// Data lengths from far enough below the boundary that the blob
		// message length (data plus up to 36 bytes of other fields) crosses
		// it too, to just past it.
		for dataLen := boundary - 36; dataLen <= boundary+1; dataLen++ {
			data := bytes.Repeat([]byte{byte(dataLen)}, dataLen)
			blob := blobFields(ns, data, 0, 0, nil)
			canonical := blobTxFields([]byte("inner tx"), blob)
			check(fmt.Sprintf("data length %d", dataLen), encodeFields(canonical))
			// The same tx with each length prefix written one byte longer.
			check(fmt.Sprintf("data length %d overlong data length", dataLen),
				encodeFields(blobTxFields([]byte("inner tx"), overlongValue(blob, 1))))
			check(fmt.Sprintf("data length %d overlong blob length", dataLen),
				encodeFields(overlongValue(canonical, 1)))
		}
		for _, txLen := range []int{boundary, boundary + 1} {
			inner := bytes.Repeat([]byte{byte(txLen)}, txLen)
			canonical := blobTxFields(inner, blobFields(ns, []byte("data"), 0, 0, nil))
			check(fmt.Sprintf("tx length %d", txLen), encodeFields(canonical))
			check(fmt.Sprintf("tx length %d overlong tx length", txLen), encodeFields(overlongValue(canonical, 0)))
		}
	}
	require.Positive(t, accepted)
}

func TestUnmarshalBlobTxNoCopyFieldSizeBoundaries(t *testing.T) {
	v2DataLen := share.FibreBlobVersionSize + share.FibreCommitmentSize
	data := bytes.Repeat([]byte{7}, v2DataLen+1)
	accepted := 0
	check := func(name string, b []byte) {
		t.Run(name, func(t *testing.T) {
			requireSameAsUnmarshalBlobTx(t, b)
			if _, _, err := tx.UnmarshalBlobTx(b); err == nil {
				accepted++
			}
		})
	}
	nsID := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, 10)).ID()
	for _, nsLen := range []int{0, 1, share.NamespaceIDSize - 1, share.NamespaceIDSize, share.NamespaceIDSize + 1, 255} {
		ns := bytes.Repeat([]byte{0}, nsLen)
		copy(ns, nsID)
		check(fmt.Sprintf("namespace id length %d", nsLen),
			encodeFields(blobTxFields(nil, blobFields(ns, data[:v2DataLen], 0, 0, nil))))
	}
	for _, shareVersion := range []uint64{0, 1, 2} {
		for _, signerLen := range []int{0, 1, share.SignerSize - 1, share.SignerSize, share.SignerSize + 1} {
			signer := bytes.Repeat([]byte{9}, signerLen)
			check(fmt.Sprintf("share version %d signer length %d", shareVersion, signerLen),
				encodeFields(blobTxFields(nil, blobFields(nsID, data[:v2DataLen], shareVersion, 0, signer))))
		}
		for _, dataLen := range []int{1, v2DataLen - 1, v2DataLen, v2DataLen + 1} {
			check(fmt.Sprintf("share version %d data length %d", shareVersion, dataLen),
				encodeFields(blobTxFields(nil, blobFields(nsID, data[:dataLen], shareVersion, 0, bytes.Repeat([]byte{9}, share.SignerSize)))))
		}
	}
	require.Positive(t, accepted)
}

func TestUnmarshalBlobTxNoCopyIntegerLimits(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, 10)).ID()
	signer := bytes.Repeat([]byte{9}, share.SignerSize)
	values := []uint64{
		0, 1, 2, 3, 127, 128, 255, 256,
		math.MaxUint32 - 1, math.MaxUint32, math.MaxUint32 + 1, 1<<32 | 1, 1<<32 | 2,
		math.MaxInt64, 1 << 63, math.MaxUint64,
	}
	for _, v := range values {
		// Write the field even when it is zero, which proto.Marshal does not.
		varint := protowire.AppendVarint(nil, v)
		for _, tc := range []struct {
			name   string
			fields []rawField
		}{
			{"share version", []rawField{
				bytesField(1, ns), bytesField(2, []byte("data")),
				{protowire.AppendTag(nil, 3, protowire.VarintType), varint},
				bytesField(5, signer),
			}},
			{"share version without signer", []rawField{
				bytesField(1, ns), bytesField(2, []byte("data")),
				{protowire.AppendTag(nil, 3, protowire.VarintType), varint},
			}},
			{"namespace version", []rawField{
				bytesField(1, ns), bytesField(2, []byte("data")),
				{protowire.AppendTag(nil, 4, protowire.VarintType), varint},
			}},
		} {
			t.Run(fmt.Sprintf("%s %d", tc.name, v), func(t *testing.T) {
				requireSameAsUnmarshalBlobTx(t, encodeFields(blobTxFields(nil, tc.fields)))
			})
		}
		// Length prefixes that claim v bytes.
		for _, field := range []struct {
			name string
			num  protowire.Number
		}{{"tx length", 1}, {"blob length", 2}} {
			t.Run(fmt.Sprintf("%s %d", field.name, v), func(t *testing.T) {
				b := protowire.AppendTag(nil, field.num, protowire.BytesType)
				b = append(append(b, varint...), "data"...)
				requireSameAsUnmarshalBlobTx(t, b)
			})
		}
	}
}

// TestUnmarshalBlobTxNoCopyFieldMutations applies every single field
// mutation, and every pair of them, to valid BlobTxs.
func TestUnmarshalBlobTxNoCopyFieldMutations(t *testing.T) {
	ns0 := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, 10)).ID()
	ns1 := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, 10)).ID()
	signer := bytes.Repeat([]byte{9}, share.SignerSize)
	v2Data := bytes.Repeat([]byte{7}, share.FibreBlobVersionSize+share.FibreCommitmentSize)
	bases := []struct {
		name   string
		fields []rawField
		pairs  bool
	}{
		{"tx with v0 and v1 blobs", blobTxFields([]byte("inner tx"),
			blobFields(ns0, []byte("blob zero"), 0, 0, nil),
			blobFields(ns1, []byte("blob one"), 1, 0, signer)), true},
		{"v2 blob without tx", blobTxFields(nil, blobFields(ns0, v2Data, 2, 0, signer)), false},
	}
	for _, base := range bases {
		t.Run(base.name, func(t *testing.T) {
			_, _, err := tx.UnmarshalBlobTxNoCopy(encodeFields(base.fields))
			require.NoError(t, err)
			mutations := blobTxMutations(base.fields)
			accepted := 0
			apply := func(ms ...mutation) {
				fields := base.fields
				for _, m := range ms {
					var ok bool
					if fields, ok = m.apply(fields); !ok {
						return
					}
				}
				b := encodeFields(fields)
				want, wantIsBlob, wantErr := tx.UnmarshalBlobTx(b)
				got, gotIsBlob, gotErr := tx.UnmarshalBlobTxNoCopy(b)
				if wantIsBlob != gotIsBlob || !reflect.DeepEqual(wantErr, gotErr) || !reflect.DeepEqual(want, got) {
					var names []string
					for _, m := range ms {
						names = append(names, m.name)
					}
					t.Fatalf("mutations %q on %x: UnmarshalBlobTx = (%v, %v, %v), UnmarshalBlobTxNoCopy = (%v, %v, %v)",
						names, b, want, wantIsBlob, wantErr, got, gotIsBlob, gotErr)
				}
				if wantErr == nil {
					accepted++
				}
			}
			for _, a := range mutations {
				apply(a)
				if base.pairs {
					for _, b := range mutations {
						apply(a, b)
					}
				}
			}
			// Some mutations keep the tx valid, such as swapping two blobs.
			require.Positive(t, accepted)
		})
	}
}

// TestUnmarshalBlobTxNoCopyNestedMutations uses every mutation of a valid
// BlobTx as the inner tx of another, so the nested BlobTx check sees inputs
// on both sides of its verdict.
func TestUnmarshalBlobTxNoCopyNestedMutations(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, 10)).ID()
	blob := blobFields(ns, []byte("blob"), 0, 0, nil)
	inner := blobTxFields([]byte("inner tx"), blob)
	nested := 0
	for _, m := range blobTxMutations(inner) {
		fields, ok := m.apply(inner)
		if !ok {
			continue
		}
		b := encodeFields(blobTxFields(encodeFields(fields), blob))
		t.Run(m.name, func(t *testing.T) {
			requireSameAsUnmarshalBlobTx(t, b)
		})
		if _, _, err := tx.UnmarshalBlobTx(b); errors.Is(err, tx.ErrNestedBlobTx) {
			nested++
		}
	}
	require.Positive(t, nested)
}

// FuzzUnmarshalBlobTxNoCopyStructured generates a valid BlobTx from the
// fuzz input and then mutates it, so that most inputs get past the first
// few bytes of the decoder.
func FuzzUnmarshalBlobTxNoCopyStructured(f *testing.F) {
	for i := range 32 {
		f.Add(binary.LittleEndian.AppendUint64(nil, uint64(i)*0x9e3779b97f4a7c15))
	}
	f.Fuzz(func(t *testing.T, seed []byte) {
		r := newFuzzReader(seed)
		b := generateBlobTx(t, r)
		got, isBlob, err := tx.UnmarshalBlobTxNoCopy(b)
		require.NoError(t, err)
		require.True(t, isBlob)
		require.NotNil(t, got)
		requireSameAsUnmarshalBlobTx(t, b)
		for range r.intn(4) {
			b = mutateBytes(b, r)
			requireSameAsUnmarshalBlobTx(t, b)
		}
	})
}

// rawField is one encoded protobuf field: its tag and the bytes after it.
type rawField struct {
	tag, value []byte
}

func encodeFields(fields []rawField) []byte {
	var b []byte
	for _, f := range fields {
		b = append(append(b, f.tag...), f.value...)
	}
	return b
}

// splitFields splits b into fields, or returns false if b is not a
// well-formed protobuf message.
func splitFields(b []byte) ([]rawField, bool) {
	var fields []rawField
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, false
		}
		m := protowire.ConsumeFieldValue(num, typ, b[n:])
		if m < 0 {
			return nil, false
		}
		fields = append(fields, rawField{tag: b[:n:n], value: b[n : n+m : n+m]})
		b = b[n+m:]
	}
	return fields, true
}

func bytesField(num protowire.Number, v []byte) rawField {
	return rawField{protowire.AppendTag(nil, num, protowire.BytesType), protowire.AppendBytes(nil, v)}
}

func varintField(num protowire.Number, v uint64) rawField {
	return rawField{protowire.AppendTag(nil, num, protowire.VarintType), protowire.AppendVarint(nil, v)}
}

// blobFields returns the BlobProto fields proto.Marshal would write.
func blobFields(ns, data []byte, shareVersion, namespaceVersion uint64, signer []byte) []rawField {
	var fields []rawField
	if len(ns) > 0 {
		fields = append(fields, bytesField(1, ns))
	}
	if len(data) > 0 {
		fields = append(fields, bytesField(2, data))
	}
	if shareVersion != 0 {
		fields = append(fields, varintField(3, shareVersion))
	}
	if namespaceVersion != 0 {
		fields = append(fields, varintField(4, namespaceVersion))
	}
	if len(signer) > 0 {
		fields = append(fields, bytesField(5, signer))
	}
	return fields
}

// blobTxFields returns the BlobTx fields proto.Marshal would write.
func blobTxFields(inner []byte, blobs ...[]rawField) []rawField {
	fields := make([]rawField, 0, len(blobs)+2)
	if len(inner) > 0 {
		fields = append(fields, bytesField(1, inner))
	}
	for _, blob := range blobs {
		fields = append(fields, bytesField(2, encodeFields(blob)))
	}
	return append(fields, bytesField(3, []byte(tx.ProtoBlobTxTypeID)))
}

// overlongVarint encodes v in one more byte than it needs, or in 10 bytes
// if it already needs 10.
func overlongVarint(v uint64) []byte {
	b := protowire.AppendVarint(nil, v)
	if len(b) == binary.MaxVarintLen64 {
		return b
	}
	b[len(b)-1] |= 0x80
	return append(b, 0)
}

// overlongValue returns fields with field i's leading varint (a length
// prefix or an integer value) written one byte longer, if it has one.
func overlongValue(fields []rawField, i int) []rawField {
	out := append([]rawField(nil), fields...)
	if v, n := protowire.ConsumeVarint(out[i].value); n >= 0 {
		out[i].value = append(overlongVarint(v), out[i].value[n:]...)
	}
	return out
}

type mutation struct {
	name  string
	apply func([]rawField) ([]rawField, bool)
}

// fieldMutations returns every mutation of the field at index i of a
// message.
func fieldMutations(i int) []mutation {
	at := func(name string, fn func(fields []rawField) []rawField) mutation {
		return mutation{fmt.Sprintf("%s field %d", name, i), func(fields []rawField) ([]rawField, bool) {
			if i >= len(fields) {
				return nil, false
			}
			return fn(append([]rawField(nil), fields...)), true
		}}
	}
	retag := func(fields []rawField, fn func(num protowire.Number, typ protowire.Type) []byte) []rawField {
		num, typ, _ := protowire.ConsumeTag(fields[i].tag)
		fields[i].tag = fn(num, typ)
		return fields
	}
	mutations := []mutation{
		at("duplicate", func(f []rawField) []rawField { return slices.Insert(f, i, f[i]) }),
		at("drop", func(f []rawField) []rawField { return slices.Delete(f, i, i+1) }),
		at("move to front", func(f []rawField) []rawField {
			v := f[i]
			return slices.Insert(slices.Delete(f, i, i+1), 0, v)
		}),
		at("move to end", func(f []rawField) []rawField {
			v := f[i]
			return append(slices.Delete(f, i, i+1), v)
		}),
		at("swap with next", func(f []rawField) []rawField {
			if i+1 < len(f) {
				f[i], f[i+1] = f[i+1], f[i]
			}
			return f
		}),
		at("overlong tag of", func(f []rawField) []rawField {
			return retag(f, func(num protowire.Number, typ protowire.Type) []byte {
				return overlongVarint(protowire.EncodeTag(num, typ))
			})
		}),
		at("renumber", func(f []rawField) []rawField {
			return retag(f, func(num protowire.Number, typ protowire.Type) []byte {
				return protowire.AppendTag(nil, num%5+1, typ)
			})
		}),
		at("overlong value of", func(f []rawField) []rawField {
			if _, typ, _ := protowire.ConsumeTag(f[i].tag); typ != protowire.BytesType && typ != protowire.VarintType {
				return f
			}
			return overlongValue(f, i)
		}),
		at("zero value of", func(f []rawField) []rawField {
			f[i].value = []byte{0}
			return f
		}),
		at("max value of", func(f []rawField) []rawField {
			f[i].value = protowire.AppendVarint(nil, math.MaxUint64)
			return f
		}),
		at("lengthen prefix of", func(f []rawField) []rawField {
			if v, n := protowire.ConsumeVarint(f[i].value); n >= 0 {
				f[i].value = append(protowire.AppendVarint(nil, v+1), f[i].value[n:]...)
			}
			return f
		}),
		at("shorten prefix of", func(f []rawField) []rawField {
			if v, n := protowire.ConsumeVarint(f[i].value); n >= 0 {
				f[i].value = append(protowire.AppendVarint(nil, v-1), f[i].value[n:]...)
			}
			return f
		}),
	}
	// Keep the value but change the wire type, including the invalid 6
	// and 7.
	for typ := protowire.Type(0); typ < 8; typ++ {
		mutations = append(mutations, at(fmt.Sprintf("wire type %d on", typ), func(f []rawField) []rawField {
			return retag(f, func(num protowire.Number, _ protowire.Type) []byte {
				return protowire.AppendVarint(nil, protowire.EncodeTag(num, typ))
			})
		}))
	}
	return mutations
}

// blobTxMutations returns every field mutation of fields, a BlobTx, and of
// each of its blobs.
func blobTxMutations(fields []rawField) []mutation {
	var mutations []mutation
	for i, f := range fields {
		mutations = append(mutations, fieldMutations(i)...)
		if !bytes.Equal(f.tag, protowire.AppendTag(nil, 2, protowire.BytesType)) {
			continue
		}
		msg, _ := protowire.ConsumeBytes(f.value)
		blob, ok := splitFields(msg)
		if !ok {
			continue
		}
		for j := range blob {
			for _, inner := range fieldMutations(j) {
				mutations = append(mutations, mutation{
					name: fmt.Sprintf("%s of blob field %d", inner.name, i),
					apply: func(fields []rawField) ([]rawField, bool) {
						if i >= len(fields) {
							return nil, false
						}
						msg, n := protowire.ConsumeBytes(fields[i].value)
						if n < 0 {
							return nil, false
						}
						blob, ok := splitFields(msg)
						if !ok {
							return nil, false
						}
						if blob, ok = inner.apply(blob); !ok {
							return nil, false
						}
						out := append([]rawField(nil), fields...)
						out[i].value = protowire.AppendBytes(nil, encodeFields(blob))
						return out, true
					},
				})
			}
		}
	}
	return mutations
}

// fuzzReader draws decisions from the fuzz input, then zeros once it runs
// out, and seeds a PRNG for bulk contents.
type fuzzReader struct {
	b   []byte
	rng *rand.Rand
}

func newFuzzReader(b []byte) *fuzzReader {
	r := &fuzzReader{b: b}
	r.rng = rand.New(rand.NewPCG(r.uint64(), r.uint64()))
	return r
}

func (r *fuzzReader) byte() byte {
	if len(r.b) == 0 {
		return 0
	}
	c := r.b[0]
	r.b = r.b[1:]
	return c
}

func (r *fuzzReader) uint64() uint64 {
	var v uint64
	for range 8 {
		v = v<<8 | uint64(r.byte())
	}
	return v
}

// intn returns a value in [0, n).
func (r *fuzzReader) intn(n int) int {
	return int(uint16(r.byte())<<8|uint16(r.byte())) % n
}

func (r *fuzzReader) bytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.rng.Uint32())
	}
	return b
}

// generateBlobTx returns a valid BlobTx with up to 4 blobs of every share
// version, each with its own namespace, data and signer.
func generateBlobTx(t *testing.T, r *fuzzReader) []byte {
	sizes := []int{1, 127, 128, 129, 300, 1000, 16383, 16384}
	blobs := make([]*share.Blob, 1+r.intn(4))
	for i := range blobs {
		ns, err := share.NewV0Namespace(r.bytes(share.NamespaceVersionZeroIDSize))
		require.NoError(t, err)
		switch r.intn(3) {
		case 0:
			blobs[i], err = share.NewV0Blob(ns, r.bytes(sizes[r.intn(len(sizes))]+r.intn(3)))
		case 1:
			blobs[i], err = share.NewV1Blob(ns, r.bytes(sizes[r.intn(len(sizes))]+r.intn(3)), r.bytes(share.SignerSize))
		default:
			blobs[i], err = share.NewV2Blob(ns, r.rng.Uint32(), r.bytes(share.FibreCommitmentSize), r.bytes(share.SignerSize))
		}
		require.NoError(t, err)
	}
	b, err := tx.MarshalBlobTx(r.bytes(r.intn(400)), blobs...)
	require.NoError(t, err)
	return b
}

// mutateBytes applies one mutation chosen by r: a field mutation when b
// still parses as a BlobTx's fields, otherwise or sometimes a byte-level one.
func mutateBytes(b []byte, r *fuzzReader) []byte {
	if fields, ok := splitFields(b); ok && len(fields) > 0 && r.intn(4) != 0 {
		mutations := blobTxMutations(fields)
		if out, ok := mutations[r.intn(len(mutations))].apply(fields); ok {
			return encodeFields(out)
		}
	}
	if len(b) == 0 {
		return []byte{r.byte()}
	}
	out := append([]byte(nil), b...)
	i := r.intn(len(out))
	switch r.intn(5) {
	case 0:
		out[i] ^= 1 << r.intn(8)
	case 1:
		out[i] = r.byte()
	case 2:
		out = append(out[:i], append([]byte{r.byte()}, out[i:]...)...)
	case 3:
		out = append(out[:i], out[i+1:]...)
	default:
		out = out[:i]
	}
	return out
}
