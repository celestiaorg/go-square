package share

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"testing"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
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
	require.Contains(t, err.Error(), "share version 2 requires signer of size")

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
	ns := RandomNamespace()
	signer := bytes.Repeat([]byte{1}, SignerSize)
	commitment := bytes.Repeat([]byte{0xFF}, FibreCommitmentSize)
	fibreBlobVersion := uint32(42)

	t.Run("valid V2 blob", func(t *testing.T) {
		blob, err := NewV2Blob(ns, fibreBlobVersion, commitment, signer)
		require.NoError(t, err)
		require.NotNil(t, blob)
		require.Equal(t, ShareVersionTwo, blob.ShareVersion())
		require.Equal(t, ns, blob.Namespace())
		require.Equal(t, signer, blob.Signer())
		require.Len(t, blob.Data(), FibreBlobVersionSize+FibreCommitmentSize)

		// Verify fibre blob version extraction
		gotFibreBlobVersion, err := blob.FibreBlobVersion()
		require.NoError(t, err)
		require.Equal(t, fibreBlobVersion, gotFibreBlobVersion)

		// Verify commitment extraction
		gotCommitment, err := blob.FibreCommitment()
		require.NoError(t, err)
		require.Equal(t, commitment, gotCommitment)
	})

	t.Run("invalid commitment size", func(t *testing.T) {
		invalidCommitment := bytes.Repeat([]byte{0xFF}, 31) // wrong size
		_, err := NewV2Blob(ns, fibreBlobVersion, invalidCommitment, signer)
		require.Error(t, err)
		require.Contains(t, err.Error(), "commitment must be 32 bytes")
	})

	t.Run("invalid signer size", func(t *testing.T) {
		invalidSigner := bytes.Repeat([]byte{1}, 19) // wrong size
		_, err := NewV2Blob(ns, fibreBlobVersion, commitment, invalidSigner)
		require.Error(t, err)
		require.Contains(t, err.Error(), "signer must be 20 bytes")
	})

	t.Run("empty namespace", func(t *testing.T) {
		_, err := NewV2Blob(Namespace{}, fibreBlobVersion, commitment, signer)
		require.Error(t, err)
		require.Contains(t, err.Error(), "namespace can not be empty")
	})
}

func TestFibreBlobVersion(t *testing.T) {
	ns := RandomNamespace()
	signer := bytes.Repeat([]byte{1}, SignerSize)
	commitment := bytes.Repeat([]byte{0xFF}, FibreCommitmentSize)
	fibreBlobVersion := uint32(12345)

	blob, err := NewV2Blob(ns, fibreBlobVersion, commitment, signer)
	require.NoError(t, err)

	t.Run("extract fibre blob version from V2 blob", func(t *testing.T) {
		got, err := blob.FibreBlobVersion()
		require.NoError(t, err)
		require.Equal(t, fibreBlobVersion, got)
	})

	t.Run("fibre blob version from V0 blob fails", func(t *testing.T) {
		v0Blob, err := NewV0Blob(ns, []byte{1, 2, 3})
		require.NoError(t, err)

		_, err = v0Blob.FibreBlobVersion()
		require.Error(t, err)
		require.Contains(t, err.Error(), "fibre blob version is only available for share version 2")
	})

	t.Run("fibre blob version from V1 blob fails", func(t *testing.T) {
		v1Blob, err := NewV1Blob(ns, []byte{1, 2, 3}, signer)
		require.NoError(t, err)

		_, err = v1Blob.FibreBlobVersion()
		require.Error(t, err)
		require.Contains(t, err.Error(), "fibre blob version is only available for share version 2")
	})
}

func TestBlobCommitment(t *testing.T) {
	ns := RandomNamespace()
	signer := bytes.Repeat([]byte{1}, SignerSize)
	commitment := bytes.Repeat([]byte{0xAA}, FibreCommitmentSize)
	fibreBlobVersion := uint32(1)

	blob, err := NewV2Blob(ns, fibreBlobVersion, commitment, signer)
	require.NoError(t, err)

	t.Run("extract commitment from V2 blob", func(t *testing.T) {
		comm, err := blob.FibreCommitment()
		require.NoError(t, err)
		require.Equal(t, commitment, comm)
		require.Len(t, comm, FibreCommitmentSize)
	})

	t.Run("commitment from V0 blob fails", func(t *testing.T) {
		v0Blob, err := NewV0Blob(ns, []byte{1, 2, 3})
		require.NoError(t, err)

		_, err = v0Blob.FibreCommitment()
		require.Error(t, err)
		require.Contains(t, err.Error(), "commitment is only available for share version 2")
	})

	t.Run("commitment from V1 blob fails", func(t *testing.T) {
		v1Blob, err := NewV1Blob(ns, []byte{1, 2, 3}, signer)
		require.NoError(t, err)

		_, err = v1Blob.FibreCommitment()
		require.Error(t, err)
		require.Contains(t, err.Error(), "commitment is only available for share version 2")
	})
}

func TestNewBlob(t *testing.T) {
	ns := RandomNamespace()
	signer := bytes.Repeat([]byte{0x42}, SignerSize)
	commitment := bytes.Repeat([]byte{0xAB}, FibreCommitmentSize)
	fibreBlobVersion := uint32(999)

	t.Run("share version 2 blob validation", func(t *testing.T) {
		// Test with correct data size
		data := make([]byte, FibreBlobVersionSize+FibreCommitmentSize)
		binary.BigEndian.PutUint32(data[0:FibreBlobVersionSize], fibreBlobVersion)
		copy(data[FibreBlobVersionSize:], commitment)

		blob, err := NewBlob(ns, data, ShareVersionTwo, signer)
		require.NoError(t, err)
		require.Equal(t, ShareVersionTwo, blob.ShareVersion())
	})

	t.Run("share version 2 with wrong data size", func(t *testing.T) {
		wrongData := []byte{1, 2, 3} // too small
		_, err := NewBlob(ns, wrongData, ShareVersionTwo, signer)
		require.Error(t, err)
		require.Contains(t, err.Error(), "share version 2 requires data of size 36 bytes")
	})

	t.Run("share version 2 with wrong signer size", func(t *testing.T) {
		wrongSigner := []byte{1, 2, 3} // too small
		data := make([]byte, FibreBlobVersionSize+FibreCommitmentSize)
		_, err := NewBlob(ns, data, ShareVersionTwo, wrongSigner)
		require.Error(t, err)
		require.Contains(t, err.Error(), "share version 2 requires signer of size 20 bytes")
	})
}

func TestV2BlobProtoEncoding(t *testing.T) {
	ns := RandomNamespace()
	signer := bytes.Repeat([]byte{0x77}, SignerSize)
	commitment := bytes.Repeat([]byte{0x88}, FibreCommitmentSize)
	fibreBlobVersion := uint32(54321)

	blob, err := NewV2Blob(ns, fibreBlobVersion, commitment, signer)
	require.NoError(t, err)

	t.Run("marshal and unmarshal V2 blob", func(t *testing.T) {
		blobBytes, err := blob.Marshal()
		require.NoError(t, err)

		unmarshaledBlob, err := UnmarshalBlob(blobBytes)
		require.NoError(t, err)

		require.Equal(t, blob.ShareVersion(), unmarshaledBlob.ShareVersion())
		require.Equal(t, blob.Namespace(), unmarshaledBlob.Namespace())
		require.Equal(t, blob.Signer(), unmarshaledBlob.Signer())
		require.Equal(t, blob.Data(), unmarshaledBlob.Data())

		// Verify round-trip of fibre blob version and commitment
		rv, err := unmarshaledBlob.FibreBlobVersion()
		require.NoError(t, err)
		require.Equal(t, fibreBlobVersion, rv)

		comm, err := unmarshaledBlob.FibreCommitment()
		require.NoError(t, err)
		require.Equal(t, commitment, comm)
	})

	t.Run("JSON encoding V2 blob", func(t *testing.T) {
		data, err := json.Marshal(blob)
		require.NoError(t, err)
		require.NotNil(t, data)

		b := &Blob{}
		err = json.Unmarshal(data, b)
		require.NoError(t, err)

		require.Equal(t, blob.ShareVersion(), b.ShareVersion())
		require.Equal(t, blob.Namespace(), b.Namespace())
		require.Equal(t, blob.Signer(), b.Signer())

		rv, err := b.FibreBlobVersion()
		require.NoError(t, err)
		require.Equal(t, fibreBlobVersion, rv)

		comm, err := b.FibreCommitment()
		require.NoError(t, err)
		require.Equal(t, commitment, comm)
	})
}

func TestV2BlobToShares(t *testing.T) {
	ns := RandomNamespace()
	signer := bytes.Repeat([]byte{0x99}, SignerSize)
	commitment := bytes.Repeat([]byte{0xAA}, FibreCommitmentSize)
	fibreBlobVersion := uint32(100)

	blob, err := NewV2Blob(ns, fibreBlobVersion, commitment, signer)
	require.NoError(t, err)

	t.Run("convert V2 blob to shares and back", func(t *testing.T) {
		shares, err := blob.ToShares()
		require.NoError(t, err)
		require.Len(t, shares, 1) // V2 blob should fit in a single share

		// Verify share version
		require.Equal(t, ShareVersionTwo, shares[0].Version())

		// Parse shares back to blob
		blobList, err := parseSparseShares(shares)
		require.NoError(t, err)
		require.Len(t, blobList, 1)

		parsedBlob := blobList[0]
		require.Equal(t, blob.ShareVersion(), parsedBlob.ShareVersion())
		require.Equal(t, blob.Namespace(), parsedBlob.Namespace())
		require.Equal(t, blob.Signer(), parsedBlob.Signer())

		rv, err := parsedBlob.FibreBlobVersion()
		require.NoError(t, err)
		require.Equal(t, fibreBlobVersion, rv)

		comm, err := parsedBlob.FibreCommitment()
		require.NoError(t, err)
		require.Equal(t, commitment, comm)
	})
}

func TestNewBlobFromProtoV2(t *testing.T) {
	namespace := RandomNamespace()
	signer := bytes.Repeat([]byte{1}, SignerSize)
	commitment := bytes.Repeat([]byte{0xFF}, FibreCommitmentSize)
	fibreBlobVersion := uint32(42)

	// Create data: fibre_blob_version (4 bytes) + commitment (32 bytes)
	data := make([]byte, FibreBlobVersionSize+FibreCommitmentSize)
	binary.BigEndian.PutUint32(data[0:FibreBlobVersionSize], fibreBlobVersion)
	copy(data[FibreBlobVersionSize:], commitment)

	testCases := []struct {
		name        string
		proto       *v4.BlobProto
		expectedErr string
	}{
		{
			name: "valid V2 blob",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: uint32(namespace.Version()),
				ShareVersion:     2,
				Data:             data,
				Signer:           signer,
			},
			expectedErr: "",
		},
		{
			name: "V2 blob with invalid signer length",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 0,
				ShareVersion:     2,
				Data:             data,
				Signer:           []byte{1, 2, 3}, // wrong size
			},
			expectedErr: "share version 2 requires signer of size",
		},
		{
			name: "V2 blob with invalid data size",
			proto: &v4.BlobProto{
				NamespaceId:      namespace.ID(),
				NamespaceVersion: 0,
				ShareVersion:     2,
				Data:             []byte{1, 2, 3}, // wrong size
				Signer:           signer,
			},
			expectedErr: "share version 2 requires data of size 36 bytes",
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
				require.Equal(t, uint8(2), blob.ShareVersion())
				require.Equal(t, tc.proto.NamespaceId, blob.Namespace().ID())
				require.Equal(t, uint8(tc.proto.NamespaceVersion), blob.Namespace().Version())
				require.Equal(t, tc.proto.Data, blob.Data())
				require.Equal(t, tc.proto.Signer, blob.Signer())
			}
		})
	}
}
