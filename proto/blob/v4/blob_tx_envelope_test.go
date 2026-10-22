package v4_test

import (
	"testing"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// TestBlobTxEnvelopeMatchesBlobTx pins that every envelope field has the same
// number, kind and cardinality in BlobTx, so both decode the same wire bytes.
func TestBlobTxEnvelopeMatchesBlobTx(t *testing.T) {
	envelope := (&v4.BlobTxEnvelope{}).ProtoReflect().Descriptor()
	blobTx := (&v4.BlobTx{}).ProtoReflect().Descriptor()
	require.Equal(t, 2, envelope.Fields().Len())
	for i := 0; i < envelope.Fields().Len(); i++ {
		field := envelope.Fields().Get(i)
		want := blobTx.Fields().ByNumber(field.Number())
		require.NotNil(t, want, "field %d", field.Number())
		require.Equal(t, want.Kind(), field.Kind(), "field %d", field.Number())
		require.Equal(t, want.Cardinality(), field.Cardinality(), "field %d", field.Number())
	}
	// Blob payloads must still be parsed as messages, without keeping any field.
	require.Zero(t, (&v4.BlobEnvelope{}).ProtoReflect().Descriptor().Fields().Len())
}

func TestBlobTxEnvelopeDecodesBlobTx(t *testing.T) {
	raw, err := proto.Marshal(&v4.BlobTx{
		Tx:     []byte("tx"),
		Blobs:  []*v4.BlobProto{{NamespaceId: []byte{1}, Data: []byte("a")}, {NamespaceId: []byte{2}, Data: []byte("b")}},
		TypeId: "BLOB",
	})
	require.NoError(t, err)

	var env v4.BlobTxEnvelope
	require.NoError(t, proto.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(raw, &env))
	require.Equal(t, "BLOB", env.GetTypeId())
	require.Len(t, env.GetBlobs(), 2)
	require.Empty(t, env.ProtoReflect().GetUnknown())
	require.Contains(t, env.String(), "BLOB")

	// Re-encoding the envelope yields a BlobTx with the same blob count and type id.
	reencoded, err := proto.Marshal(&env)
	require.NoError(t, err)
	var back v4.BlobTx
	require.NoError(t, proto.Unmarshal(reencoded, &back))
	require.Equal(t, "BLOB", back.GetTypeId())
	require.Len(t, back.GetBlobs(), 2)

	env.Reset()
	require.True(t, proto.Equal(&v4.BlobTxEnvelope{}, &env))
}

func TestBlobTxEnvelopeGeneratedAccessors(t *testing.T) {
	var nilEnv *v4.BlobTxEnvelope
	require.Empty(t, nilEnv.GetTypeId())
	require.Nil(t, nilEnv.GetBlobs())

	for _, msg := range []interface {
		proto.Message
		Reset()
		String() string
		Descriptor() ([]byte, []int)
	}{&v4.BlobTxEnvelope{TypeId: "BLOB"}, &v4.BlobEnvelope{}} {
		desc, path := msg.Descriptor()
		require.NotEmpty(t, desc)
		require.Len(t, path, 1)
		require.NotPanics(t, func() { _ = msg.String() })
		msg.Reset()
		require.Zero(t, proto.Size(msg))
	}
}
