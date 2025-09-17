package share

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"testing"

	v2 "github.com/celestiaorg/go-square/v3/proto/blob/v2"
	"github.com/stretchr/testify/require"
)

func TestProtoEncoding(t *testing.T) {
	signer := make([]byte, 20)
	_, err := rand.Read(signer)
	require.NoError(t, err)
	blob, err := NewBlob(RandomNamespace(), []byte{1, 2, 3, 4, 5}, 1, signer)
	require.NoError(t, err)

	blobBytes, err := blob.Marshal()
	require.NoError(t, err)

	newBlob, err := UnmarshalBlob(blobBytes)
	require.NoError(t, err)

	require.Equal(t, blob, newBlob)
}

func TestJSONEncoding(t *testing.T) {
	signer := make([]byte, 20)
	_, err := rand.Read(signer)
	require.NoError(t, err)
	blob, err := NewBlob(RandomNamespace(), []byte{1, 2, 3, 4, 5}, 1, signer)
	require.NoError(t, err)

	data, err := json.Marshal(blob)
	require.NoError(t, err)
	require.NotNil(t, data)

	b := &Blob{}
	err = json.Unmarshal(data, b)
	require.NoError(t, err)
	require.Equal(t, blob, b)
}

func TestBlobConstructor(t *testing.T) {
	signer := make([]byte, 20)
	_, err := rand.Read(signer)
	require.NoError(t, err)

	ns := RandomNamespace()
	data := []byte{1, 2, 3, 4, 5}

	// test all invalid cases
	_, err = NewBlob(ns, data, 0, signer)
	require.Error(t, err)
	require.Contains(t, err.Error(), "share version 0 does not support signer")

	_, err = NewBlob(ns, nil, 0, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "data can not be empty")

	_, err = NewBlob(ns, data, 1, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "share version 1 requires signer of size")

	_, err = NewBlob(ns, data, 128, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "share version 128 not supported")

	_, err = NewBlob(ns, data, 2, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "share version 2 not supported")

	_, err = NewBlob(Namespace{}, data, 1, signer)
	require.Error(t, err)
	require.Contains(t, err.Error(), "namespace can not be empty")

	ns2, err := NewNamespace(NamespaceVersionMax, ns.ID())
	require.NoError(t, err)
	_, err = NewBlob(ns2, data, 0, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "namespace version must be 0")

	blob, err := NewBlob(ns, data, 0, nil)
	require.NoError(t, err)
	shares, err := blob.ToShares()
	require.NoError(t, err)
	blobList, err := parseSparseShares(shares)
	require.NoError(t, err)
	require.Len(t, blobList, 1)
	require.Equal(t, blob, blobList[0])
}

func TestNewBlobFromProto(t *testing.T) {
	namespace := RandomNamespace()
	testCases := []struct {
		name        string
		proto       *v2.BlobProto
		expectedErr string
	}{
		{
			name: "valid blob",
			proto: &v2.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: uint32(namespace.Version()),
				ShareVersion:     0,
				Data:             []byte{1, 2, 3, 4, 5},
			},
			expectedErr: "",
		},
		{
			name: "invalid namespace version",
			proto: &v2.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 256,
				ShareVersion:     0,
				Data:             []byte{1, 2, 3, 4, 5},
			},
			expectedErr: "namespace version can not be greater than MaxNamespaceVersion",
		},
		{
			name: "empty data",
			proto: &v2.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 0,
				ShareVersion:     0,
				Data:             []byte{},
			},
			expectedErr: "data can not be empty",
		},
		{
			name: "invalid namespace ID length",
			proto: &v2.BlobProto{
				NamespaceId:      []byte{1, 2, 3},
				NamespaceVersion: 0,
				ShareVersion:     0,
				Data:             []byte{1, 2, 3, 4, 5},
			},
			expectedErr: "invalid namespace",
		},
		{
			name: "valid blob with signer",
			proto: &v2.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 0,
				ShareVersion:     1,
				Data:             []byte{1, 2, 3, 4, 5},
				Signer:           bytes.Repeat([]byte{1}, SignerSize),
			},
			expectedErr: "",
		},
		{
			name: "invalid signer length",
			proto: &v2.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 0,
				ShareVersion:     1,
				Data:             []byte{1, 2, 3, 4, 5},
				Signer:           []byte{1, 2, 3},
			},
			expectedErr: "share version 1 requires signer of size",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blob, err := NewBlobFromProto(tc.proto)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, blob)
				require.Equal(t, tc.proto.NamespaceId, blob.Namespace().ID())
				require.Equal(t, uint8(tc.proto.NamespaceVersion), blob.Namespace().Version())
				require.Equal(t, uint8(tc.proto.ShareVersion), blob.ShareVersion())
				require.Equal(t, tc.proto.Data, blob.Data())
				require.Equal(t, tc.proto.Signer, blob.Signer())
			}
		})
	}
}
