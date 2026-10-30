package share

import (
	"bytes"
	"encoding/json"
	"testing"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/stretchr/testify/require"
)

// maxDecodeInput bounds the encoded input of the decoding targets below.
const maxDecodeInput = 8 * 1024

func decodeSeedBlobs(tb testing.TB) []*Blob {
	tb.Helper()
	ns := MustNewV0Namespace([]byte("decode"))
	signer := bytes.Repeat([]byte{7}, SignerSize)
	v0, err := NewV0Blob(ns, []byte("data"))
	require.NoError(tb, err)
	v1, err := NewV1Blob(ns, repeatingBytes(479), signer)
	require.NoError(tb, err)
	v2, err := NewV2Blob(ns, 3, bytes.Repeat([]byte{9}, FibreCommitmentSize), signer)
	require.NoError(tb, err)
	return []*Blob{v0, v1, v2}
}

// FuzzUnmarshalBlob asserts that arbitrary protobuf bytes never panic
// UnmarshalBlob and that every accepted blob round-trips through both its
// protobuf and JSON encodings.
func FuzzUnmarshalBlob(f *testing.F) {
	for _, blob := range decodeSeedBlobs(f) {
		encoded, err := blob.Marshal()
		require.NoError(f, err)
		f.Add(encoded)
		f.Add(encoded[:len(encoded)-1])
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxDecodeInput {
			t.Skip()
		}
		blob, err := UnmarshalBlob(data)
		if err != nil {
			return
		}
		requireBlobRoundTrip(t, blob)
	})
}

// FuzzNewBlobFromProto reaches the field validation directly, since few
// arbitrary byte strings decode to a BlobProto with consistent fields.
func FuzzNewBlobFromProto(f *testing.F) {
	for _, blob := range decodeSeedBlobs(f) {
		f.Add(uint32(blob.Namespace().Version()), blob.Namespace().ID(), uint32(blob.ShareVersion()), blob.Data(), blob.Signer())
	}
	f.Add(uint32(255), []byte{}, uint32(0), []byte("data"), []byte{})
	f.Add(uint32(0), make([]byte, NamespaceIDSize), uint32(1<<31), []byte("data"), []byte(nil))
	f.Fuzz(func(t *testing.T, nsVersion uint32, nsID []byte, shareVersion uint32, data, signer []byte) {
		if len(nsID)+len(data)+len(signer) > maxDecodeInput {
			t.Skip()
		}
		pb := &v4.BlobProto{NamespaceVersion: nsVersion, NamespaceId: nsID, ShareVersion: shareVersion, Data: data, Signer: signer}
		blob, err := NewBlobFromProto(pb)
		if err != nil {
			return
		}
		require.Equal(t, nsVersion, uint32(blob.Namespace().Version()))
		require.Equal(t, nsID, blob.Namespace().ID())
		require.Equal(t, shareVersion, uint32(blob.ShareVersion()))
		require.Equal(t, data, blob.Data())
		require.True(t, bytes.Equal(signer, blob.Signer()))
		requireBlobRoundTrip(t, blob)
	})
}

// FuzzUnmarshalJSON asserts that arbitrary JSON never panics the Blob,
// Namespace or Share decoders, and that every accepted value round-trips.
func FuzzUnmarshalJSON(f *testing.F) {
	// Seeds are encoded without the MarshalJSON methods under test, so that a
	// broken encoder cannot also break the seeds.
	for _, blob := range decodeSeedBlobs(f) {
		encoded, err := json.Marshal(&v4.BlobProto{
			NamespaceId:      blob.Namespace().ID(),
			NamespaceVersion: uint32(blob.Namespace().Version()),
			ShareVersion:     uint32(blob.ShareVersion()),
			Data:             blob.Data(),
			Signer:           blob.Signer(),
		})
		require.NoError(f, err)
		f.Add(encoded)
		encoded, err = json.Marshal(blob.Namespace().Bytes())
		require.NoError(f, err)
		f.Add(encoded)
		shares, err := blob.ToShares()
		require.NoError(f, err)
		encoded, err = json.Marshal(shares[0].ToBytes())
		require.NoError(f, err)
		f.Add(encoded)
	}
	f.Add([]byte(`null`))
	f.Add([]byte(`{"namespace_id":"","data":"AA=="}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxDecodeInput {
			t.Skip()
		}
		var blob Blob
		if err := json.Unmarshal(data, &blob); err == nil && !blob.IsEmpty() {
			requireBlobRoundTrip(t, &blob)
		}

		var ns Namespace
		if err := ns.UnmarshalJSON(data); err == nil {
			encoded, err := json.Marshal(ns)
			require.NoError(t, err)
			var again Namespace
			require.NoError(t, json.Unmarshal(encoded, &again))
			require.True(t, ns.Equals(again))
			fromBytes, err := NewNamespaceFromBytes(ns.Bytes())
			require.NoError(t, err)
			require.True(t, ns.Equals(fromBytes))
		}

		var sh Share
		if err := sh.UnmarshalJSON(data); err == nil {
			require.Len(t, sh.ToBytes(), ShareSize)
			encoded, err := json.Marshal(sh)
			require.NoError(t, err)
			var again Share
			require.NoError(t, json.Unmarshal(encoded, &again))
			require.Equal(t, sh.ToBytes(), again.ToBytes())
			_, err = NewShare(sh.ToBytes())
			require.NoError(t, err)
		}
	})
}

// TestDecodeSeedsRoundTrip asserts the seeds take the accept path of every
// decoder above.
func TestDecodeSeedsRoundTrip(t *testing.T) {
	for _, blob := range decodeSeedBlobs(t) {
		encoded, err := blob.Marshal()
		require.NoError(t, err)
		decoded, err := UnmarshalBlob(encoded)
		require.NoError(t, err)
		requireBlobRoundTrip(t, decoded)

		encoded, err = json.Marshal(blob)
		require.NoError(t, err)
		var fromJSON Blob
		require.NoError(t, json.Unmarshal(encoded, &fromJSON))
		requireBlobEqual(t, blob, &fromJSON)

		encoded, err = json.Marshal(blob.Namespace())
		require.NoError(t, err)
		var ns Namespace
		require.NoError(t, ns.UnmarshalJSON(encoded))
		require.True(t, blob.Namespace().Equals(ns))

		shares, err := blob.ToShares()
		require.NoError(t, err)
		encoded, err = json.Marshal(shares[0])
		require.NoError(t, err)
		var sh Share
		require.NoError(t, sh.UnmarshalJSON(encoded))
		require.Equal(t, shares[0].ToBytes(), sh.ToBytes())
	}
}

func requireBlobRoundTrip(t *testing.T, blob *Blob) {
	t.Helper()
	encoded, err := blob.Marshal()
	require.NoError(t, err)
	decoded, err := UnmarshalBlob(encoded)
	require.NoError(t, err, "a valid blob's encoding must decode")
	requireBlobEqual(t, blob, decoded)
	reencoded, err := decoded.Marshal()
	require.NoError(t, err)
	require.Equal(t, encoded, reencoded, "blob encoding must be stable")

	encoded, err = json.Marshal(blob)
	require.NoError(t, err)
	var fromJSON Blob
	require.NoError(t, json.Unmarshal(encoded, &fromJSON), "a valid blob's JSON must decode")
	requireBlobEqual(t, blob, &fromJSON)
}

func requireBlobEqual(t *testing.T, want, got *Blob) {
	t.Helper()
	require.True(t, want.Namespace().Equals(got.Namespace()), "namespace")
	require.Equal(t, want.ShareVersion(), got.ShareVersion(), "share version")
	require.Equal(t, want.Data(), got.Data(), "data")
	require.True(t, bytes.Equal(want.Signer(), got.Signer()), "signer")
	require.Equal(t, want.Hash(), got.Hash(), "hash")
}
