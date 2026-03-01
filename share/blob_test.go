package share

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"testing"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/stretchr/testify/assert"
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

	_, err = NewBlob(ns, data, 2, signer)
	require.Error(t, err)
	require.Contains(t, err.Error(), "share version 2 requires data of size")

	_, err = NewBlob(ns, data, 3, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "share version 3 not supported")

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
		proto       *v4.BlobProto
		expectedErr string
	}{
		{
			name: "valid blob",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: uint32(namespace.Version()),
				ShareVersion:     0,
				Data:             []byte{1, 2, 3, 4, 5},
			},
			expectedErr: "",
		},
		{
			name: "invalid namespace version",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 256,
				ShareVersion:     0,
				Data:             []byte{1, 2, 3, 4, 5},
			},
			expectedErr: "namespace version can not be greater than MaxNamespaceVersion",
		},
		{
			name: "empty data",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 0,
				ShareVersion:     0,
				Data:             []byte{},
			},
			expectedErr: "data can not be empty",
		},
		{
			name: "invalid namespace ID length",
			proto: &v4.BlobProto{
				NamespaceId:      []byte{1, 2, 3},
				NamespaceVersion: 0,
				ShareVersion:     0,
				Data:             []byte{1, 2, 3, 4, 5},
			},
			expectedErr: "invalid namespace",
		},
		{
			name: "valid blob with signer",
			proto: &v4.BlobProto{
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
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 0,
				ShareVersion:     1,
				Data:             []byte{1, 2, 3, 4, 5},
				Signer:           []byte{1, 2, 3},
			},
			expectedErr: "share version 1 requires signer of size",
		},
		{
			name: "valid v2 blob",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: uint32(namespace.Version()),
				ShareVersion:     2,
				Data:             makeV2Data(1, bytes.Repeat([]byte{0xAA}, FibreCommitmentSize)),
				Signer:           bytes.Repeat([]byte{1}, SignerSize),
			},
			expectedErr: "",
		},
		{
			name: "v2 blob with wrong data size",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: uint32(namespace.Version()),
				ShareVersion:     2,
				Data:             []byte{1, 2, 3},
				Signer:           bytes.Repeat([]byte{1}, SignerSize),
			},
			expectedErr: "share version 2 requires data of size",
		},
		{
			name: "v2 blob missing signer",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: uint32(namespace.Version()),
				ShareVersion:     2,
				Data:             makeV2Data(1, bytes.Repeat([]byte{0xAA}, FibreCommitmentSize)),
				Signer:           nil,
			},
			expectedErr: "share version 2 requires signer of size",
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

func TestNewV2Blob(t *testing.T) {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAA}, SignerSize)
	commitment := bytes.Repeat([]byte{0xBB}, FibreCommitmentSize)
	fibreBlobVersion := uint32(42)

	blob, err := NewV2Blob(ns, fibreBlobVersion, commitment, signer)
	require.NoError(t, err)
	require.Equal(t, ShareVersionTwo, blob.ShareVersion())
	require.Equal(t, ns, blob.Namespace())
	require.Equal(t, signer, blob.Signer())

	// Verify FibreBlobVersion
	v, err := blob.FibreBlobVersion()
	require.NoError(t, err)
	require.Equal(t, fibreBlobVersion, v)

	// Verify FibreCommitment
	c, err := blob.FibreCommitment()
	require.NoError(t, err)
	require.Equal(t, commitment, c)
}

func TestNewV2BlobInvalidCommitmentSize(t *testing.T) {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAA}, SignerSize)

	_, err := NewV2Blob(ns, 1, []byte{1, 2, 3}, signer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commitment must be")
}

func TestFibreBlobVersionOnNonV2Blob(t *testing.T) {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	blob, err := NewV0Blob(ns, []byte("data"))
	require.NoError(t, err)

	_, err = blob.FibreBlobVersion()
	require.Error(t, err)

	_, err = blob.FibreCommitment()
	require.Error(t, err)
}

func TestV2BlobToShares(t *testing.T) {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAA}, SignerSize)
	commitment := bytes.Repeat([]byte{0xBB}, FibreCommitmentSize)

	blob, err := NewV2Blob(ns, 1, commitment, signer)
	require.NoError(t, err)

	shares, err := blob.ToShares()
	require.NoError(t, err)
	require.Len(t, shares, 1) // V2 blob fits in one share

	// Verify round-trip
	blobList, err := parseSparseShares(shares)
	require.NoError(t, err)
	require.Len(t, blobList, 1)
	require.Equal(t, blob.ShareVersion(), blobList[0].ShareVersion())
	require.Equal(t, blob.Data(), blobList[0].Data())
}

func TestV2BlobProtoRoundTrip(t *testing.T) {
	ns := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAA}, SignerSize)
	commitment := bytes.Repeat([]byte{0xBB}, FibreCommitmentSize)

	blob, err := NewV2Blob(ns, 1, commitment, signer)
	require.NoError(t, err)

	// Marshal and unmarshal
	blobBytes, err := blob.Marshal()
	require.NoError(t, err)

	newBlob, err := UnmarshalBlob(blobBytes)
	require.NoError(t, err)
	require.Equal(t, blob, newBlob)
}

// makeV2Data creates v2 blob data from a fibre blob version and commitment.
func makeV2Data(fibreBlobVersion uint32, commitment []byte) []byte {
	data := make([]byte, FibreBlobVersionSize+FibreCommitmentSize)
	binary.BigEndian.PutUint32(data[:FibreBlobVersionSize], fibreBlobVersion)
	copy(data[FibreBlobVersionSize:], commitment)
	return data
}
