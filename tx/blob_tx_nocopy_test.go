package tx_test

import (
	"bytes"
	"fmt"
	"testing"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// requireSameAsUnmarshalBlobTx checks that UnmarshalBlobTxNoCopy returns what
// UnmarshalBlobTx returns for b.
func requireSameAsUnmarshalBlobTx(t *testing.T, b []byte) {
	t.Helper()
	want, wantIsBlob, wantErr := tx.UnmarshalBlobTx(b)
	got, gotIsBlob, gotErr := tx.UnmarshalBlobTxNoCopy(b)
	require.Equal(t, wantIsBlob, gotIsBlob, "isBlob for %x", b)
	require.Equal(t, wantErr, gotErr, "error for %x", b)
	require.Equal(t, want, got, "BlobTx for %x", b)
}

func TestUnmarshalBlobTxNoCopyMatchesUnmarshalBlobTx(t *testing.T) {
	for _, tc := range noCopyCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			requireSameAsUnmarshalBlobTx(t, tc.input)
			_, _, err := tx.UnmarshalBlobTxNoCopy(tc.input)
			require.Equal(t, tc.valid, err == nil)
		})
	}
}

func FuzzUnmarshalBlobTxNoCopy(f *testing.F) {
	for _, tc := range noCopyCases(f) {
		f.Add(tc.input)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		requireSameAsUnmarshalBlobTx(t, b)
	})
}

func TestUnmarshalBlobTxNoCopyAliasesInput(t *testing.T) {
	input := marshalBlobTx(t, []byte("inner tx"), v1Blob(t, 100))

	blobTx, isBlob, err := tx.UnmarshalBlobTxNoCopy(input)
	require.NoError(t, err)
	require.True(t, isBlob)

	for _, s := range [][]byte{blobTx.Tx, blobTx.Blobs[0].Data(), blobTx.Blobs[0].Signer()} {
		i := bytes.Index(input, s)
		require.GreaterOrEqual(t, i, 0)
		require.Same(t, &input[i], &s[0])
		// Appending must not overwrite the rest of the input.
		require.Equal(t, len(s), cap(s))
	}
}

type noCopyCase struct {
	name  string
	input []byte
	valid bool
}

func noCopyCases(tb testing.TB) []noCopyCase {
	lenField := func(num protowire.Number, payload []byte) []byte {
		return protowire.AppendBytes(protowire.AppendTag(nil, num, protowire.BytesType), payload)
	}
	varintField := func(num protowire.Number, v uint64) []byte {
		return protowire.AppendVarint(protowire.AppendTag(nil, num, protowire.VarintType), v)
	}
	// nonMinimalLenField writes payload's length with a redundant 0x80 byte.
	nonMinimalLenField := func(num protowire.Number, payload []byte) []byte {
		b := protowire.AppendTag(nil, num, protowire.BytesType)
		b = append(b, byte(len(payload))|0x80, 0x00)
		return append(b, payload...)
	}
	cat := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
	marshal := func(m *v4.BlobTx) []byte {
		b, err := proto.Marshal(m)
		require.NoError(tb, err)
		return b
	}

	ns := bytes.Repeat([]byte{0}, share.NamespaceVersionZeroPrefixSize)
	ns = append(ns, bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize)...)
	signer := bytes.Repeat([]byte{9}, share.SignerSize)
	data := []byte("blob data")
	typeID := lenField(3, []byte(tx.ProtoBlobTxTypeID))
	innerTx := lenField(1, []byte("inner tx"))
	blob := func(fields ...[]byte) []byte { return lenField(2, cat(fields...)) }
	v0 := blob(lenField(1, ns), lenField(2, data))

	v2Blob, err := share.NewV2Blob(share.MustNewV0Namespace(bytes.Repeat([]byte{3}, 10)), 1, bytes.Repeat([]byte{4}, share.FibreCommitmentSize), signer)
	require.NoError(tb, err)

	cases := []noCopyCase{
		{"canonical v0 blob", cat(innerTx, v0, typeID), true},
		{"canonical v1 blob", cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(3, 1), lenField(5, signer)), typeID), true},
		{"canonical v2 blob", marshalBlobTx(tb, []byte("inner tx"), v2Blob), true},
		{"canonical v0, v1 and v0 blobs", marshalBlobTx(tb, []byte("inner tx"), v0Blob(tb, 10), v1Blob(tb, 600), v0Blob(tb, 3000)), true},
		{"canonical 2 KiB length prefix", marshalBlobTx(tb, bytes.Repeat([]byte{5}, 2048), v0Blob(tb, 2048)), true},
		{"no inner tx", cat(v0, typeID), true},
		{"empty inner tx field", cat(lenField(1, nil), v0, typeID), false},
		{"inner tx twice", cat(innerTx, innerTx, v0, typeID), false},
		{"inner tx after blobs", cat(v0, innerTx, typeID), false},
		{"type id before blobs", cat(innerTx, typeID, v0), false},
		{"type id twice", cat(innerTx, v0, typeID, typeID), false},
		{"no type id", cat(innerTx, v0), false},
		{"wrong type id", cat(innerTx, v0, lenField(3, []byte("BLOC"))), false},
		{"no blobs", cat(innerTx, typeID), false},
		{"trailing byte", cat(innerTx, v0, typeID, []byte{0}), false},
		{"unknown wrapper field", cat(innerTx, v0, typeID, varintField(15, 1)), false},
		{"non-minimal inner tx length", cat(nonMinimalLenField(1, []byte("inner tx")), v0, typeID), false},
		{"non-minimal blob length", cat(innerTx, nonMinimalLenField(2, cat(lenField(1, ns), lenField(2, data))), typeID), false},
		{"non-minimal type id length", cat(innerTx, v0, nonMinimalLenField(3, []byte(tx.ProtoBlobTxTypeID))), false},
		{"non-minimal namespace length", cat(innerTx, blob(nonMinimalLenField(1, ns), lenField(2, data)), typeID), false},
		{"non-minimal share version", cat(innerTx, blob(lenField(1, ns), lenField(2, data), append(protowire.AppendTag(nil, 3, protowire.VarintType), 0x81, 0x00), lenField(5, signer)), typeID), false},
		{"empty blob", cat(innerTx, blob(), typeID), false},
		{"blob without data", cat(innerTx, blob(lenField(1, ns)), typeID), false},
		{"blob without namespace", cat(innerTx, blob(lenField(2, data)), typeID), false},
		{"empty namespace field", cat(innerTx, blob(lenField(1, nil), lenField(2, data)), typeID), false},
		{"empty data field", cat(innerTx, blob(lenField(1, ns), lenField(2, nil)), typeID), false},
		{"explicit zero share version", cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(3, 0)), typeID), false},
		{"explicit zero namespace version", cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(4, 0)), typeID), false},
		{"empty signer field", cat(innerTx, blob(lenField(1, ns), lenField(2, data), lenField(5, nil)), typeID), false},
		{"share version above uint32", cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(3, 1<<32|1), lenField(5, signer)), typeID), false},
		{"unsupported share version", cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(3, 3), lenField(5, signer)), typeID), false},
		{"namespace version 255", cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(4, 255)), typeID), false},
		{"v0 blob with signer", cat(innerTx, blob(lenField(1, ns), lenField(2, data), lenField(5, signer)), typeID), false},
		{"v1 blob without signer", cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(3, 1)), typeID), false},
		{"blob fields out of order", cat(innerTx, blob(lenField(2, data), lenField(1, ns)), typeID), false},
		{"blob field twice", cat(innerTx, blob(lenField(1, ns), lenField(2, data), lenField(2, data)), typeID), false},
		{"unknown blob field", cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(15, 1)), typeID), false},
		{"blob length past end", cat(innerTx, []byte{0x12, 0x7f}, lenField(1, ns)), false},
		{"nested blob tx", marshalBlobTx(tb, cat(innerTx, v0, typeID), v0Blob(tb, 10)), false},
		{"proto.Marshal with unknown blob field", marshal(&v4.BlobTx{Tx: []byte("inner tx"), Blobs: []*v4.BlobProto{func() *v4.BlobProto {
			pb := &v4.BlobProto{NamespaceId: ns[1:], Data: data}
			pb.ProtoReflect().SetUnknown(varintField(15, 1))
			return pb
		}()}, TypeId: tx.ProtoBlobTxTypeID}), false},
	}
	for _, tc := range envelopeCases(tb) {
		_, _, err := tx.UnmarshalBlobTx(tc.input)
		cases = append(cases, noCopyCase{"envelope case " + tc.name, tc.input, err == nil})
	}
	// Truncate a canonical BlobTx at every length.
	canonical := cat(innerTx, blob(lenField(1, ns), lenField(2, data), varintField(3, 1), lenField(5, signer)), typeID)
	for i := range canonical {
		cases = append(cases, noCopyCase{fmt.Sprintf("truncated to %d bytes", i), canonical[:i], false})
	}
	return cases
}

func BenchmarkUnmarshalBlobTxNoCopy(b *testing.B) {
	// About 32 MiB of blob data per block.
	shapes := []struct {
		name         string
		blobsPerTx   int
		blobSize     int
		txsPerBlock  int
		innerTxBytes int
	}{
		{"16x2MiB", 1, 2 << 20, 16, 300},
		{"327x100KiB", 1, 100 << 10, 327, 300},
		{"3276x10x1KiB", 10, 1 << 10, 3276, 300},
	}
	impls := []struct {
		name string
		fn   func([]byte) (*tx.BlobTx, bool, error)
	}{
		{"old", tx.UnmarshalBlobTx},
		{"new", tx.UnmarshalBlobTxNoCopy},
	}
	for _, s := range shapes {
		blobs := make([]*share.Blob, s.blobsPerTx)
		for i := range blobs {
			blobs[i] = v1Blob(b, s.blobSize)
		}
		txs := make([][]byte, s.txsPerBlock)
		total := 0
		for i := range txs {
			txs[i] = marshalBlobTx(b, bytes.Repeat([]byte{1}, s.innerTxBytes), blobs...)
			total += len(txs[i])
		}
		for _, impl := range impls {
			b.Run("shape="+s.name+"/impl="+impl.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(total))
				for b.Loop() {
					for _, t := range txs {
						if _, _, err := impl.fn(t); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}
