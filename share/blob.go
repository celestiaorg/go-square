package share

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	v2 "github.com/celestiaorg/go-square/v3/proto/blob/v2"
	"google.golang.org/protobuf/proto"
)

// Blob (stands for binary large object) is a core type that represents data
// to be submitted to the Celestia network alongside an accompanying namespace
// and optional signer (for proving the signer of the blob)
type Blob struct {
	namespace    Namespace
	data         []byte
	shareVersion uint8
	signer       []byte
}

// New creates a new coretypes.Blob from the provided data after performing
// basic stateless checks over it.
func NewBlob(ns Namespace, data []byte, shareVersion uint8, signer []byte) (*Blob, error) {
	if len(data) == 0 {
		return nil, errors.New("data can not be empty")
	}
	if ns.IsEmpty() {
		return nil, errors.New("namespace can not be empty")
	}
	if ns.Version() != NamespaceVersionZero {
		return nil, fmt.Errorf("namespace version must be %d got %d", NamespaceVersionZero, ns.Version())
	}
	switch shareVersion {
	case ShareVersionZero:
		if signer != nil {
			return nil, errors.New("share version 0 does not support signer")
		}
	case ShareVersionOne:
		if len(signer) != SignerSize {
			return nil, fmt.Errorf("share version 1 requires signer of size %d bytes", SignerSize)
		}
	case ShareVersionTwo:
		if len(signer) != SignerSize {
			return nil, fmt.Errorf("share version 2 requires signer of size %d bytes", SignerSize)
		}
		// Share version 2 data must contain fibre_blob_version (4 bytes) + commitment (32 bytes)
		expectedDataSize := FibreBlobVersionSize + FibreCommitmentSize
		if len(data) != expectedDataSize {
			return nil, fmt.Errorf("share version 2 requires data of size %d bytes (fibre_blob_version + commitment), got %d", expectedDataSize, len(data))
		}
	// Note that we don't specifically check that shareVersion is less than 128 as this is caught
	// by the default case
	default:
		return nil, fmt.Errorf("share version %d not supported. Please use 0, 1, or 2", shareVersion)
	}
	return &Blob{
		namespace:    ns,
		data:         data,
		shareVersion: shareVersion,
		signer:       signer,
	}, nil
}

// NewV0Blob creates a new blob with share version 0
func NewV0Blob(ns Namespace, data []byte) (*Blob, error) {
	return NewBlob(ns, data, 0, nil)
}

// NewV1Blob creates a new blob with share version 1
func NewV1Blob(ns Namespace, data []byte, signer []byte) (*Blob, error) {
	return NewBlob(ns, data, 1, signer)
}

// NewV2Blob creates a new blob with share version 2 (for Fibre system-level blobs).
// The data must contain fibre_blob_version (4 bytes) + commitment (32 bytes).
// The signer must be 20 bytes (the signer address from MsgPayForFibre).
func NewV2Blob(ns Namespace, fibreBlobVersion uint32, commitment []byte, signer []byte) (*Blob, error) {
	if len(commitment) != FibreCommitmentSize {
		return nil, fmt.Errorf("commitment must be %d bytes, got %d", FibreCommitmentSize, len(commitment))
	}
	if len(signer) != SignerSize {
		return nil, fmt.Errorf("signer must be %d bytes, got %d", SignerSize, len(signer))
	}
	// Encode fibre_blob_version as big-endian uint32
	data := make([]byte, FibreBlobVersionSize+FibreCommitmentSize)
	binary.BigEndian.PutUint32(data[0:FibreBlobVersionSize], fibreBlobVersion)
	copy(data[FibreBlobVersionSize:], commitment)
	return NewBlob(ns, data, ShareVersionTwo, signer)
}

// UnmarshalBlob unmarshals a blob from the proto encoded bytes
func UnmarshalBlob(blob []byte) (*Blob, error) {
	pb := &v2.BlobProto{}
	err := proto.Unmarshal(blob, pb)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal blob: %w", err)
	}
	return NewBlobFromProto(pb)
}

// Marshal marshals the blob to the proto encoded bytes
func (b *Blob) Marshal() ([]byte, error) {
	pb := &v2.BlobProto{
		NamespaceId:      b.namespace.ID(),
		NamespaceVersion: uint32(b.namespace.Version()),
		ShareVersion:     uint32(b.shareVersion),
		Data:             b.data,
		Signer:           b.signer,
	}
	return proto.Marshal(pb)
}

// MarshalJSON converts blob's data to the json encoded bytes
func (b *Blob) MarshalJSON() ([]byte, error) {
	pb := &v2.BlobProto{
		NamespaceId:      b.namespace.ID(),
		NamespaceVersion: uint32(b.namespace.Version()),
		ShareVersion:     uint32(b.shareVersion),
		Data:             b.data,
		Signer:           b.signer,
	}
	return json.Marshal(pb)
}

// UnmarshalJSON converts json encoded data to the blob
func (b *Blob) UnmarshalJSON(bb []byte) error {
	pb := &v2.BlobProto{}
	err := json.Unmarshal(bb, pb)
	if err != nil {
		return err
	}

	blob, err := NewBlobFromProto(pb)
	if err != nil {
		return err
	}

	*b = *blob
	return nil
}

// NewBlobFromProto creates a new blob from the proto generated type
func NewBlobFromProto(pb *v2.BlobProto) (*Blob, error) {
	if pb.NamespaceVersion > NamespaceVersionMax {
		return nil, errors.New("namespace version can not be greater than MaxNamespaceVersion")
	}
	if pb.ShareVersion > MaxShareVersion {
		return nil, fmt.Errorf("share version can not be greater than MaxShareVersion %d", MaxShareVersion)
	}
	ns, err := NewNamespace(uint8(pb.NamespaceVersion), pb.NamespaceId)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}
	return NewBlob(
		ns,
		pb.Data,
		uint8(pb.ShareVersion),
		pb.Signer,
	)
}

// Namespace returns the namespace of the blob
func (b *Blob) Namespace() Namespace {
	return b.namespace
}

// ShareVersion returns the share version of the blob
func (b *Blob) ShareVersion() uint8 {
	return b.shareVersion
}

// Signer returns the signer of the blob
func (b *Blob) Signer() []byte {
	return b.signer
}

// HasSigner returns true if the blob has a signer.
func (b *Blob) HasSigner() bool {
	return b.signer != nil
}

// Data returns the data of the blob
func (b *Blob) Data() []byte {
	return b.data
}

// DataLen returns the length of the data of the blob
func (b *Blob) DataLen() int {
	return len(b.data)
}

// Compare is used to order two blobs based on their namespace
func (b *Blob) Compare(other *Blob) int {
	return b.namespace.Compare(other.namespace)
}

// IsEmpty returns true if the blob is empty. This is an invalid
// construction that can only occur if using the nil value. We
// only check that the data is empty but this also implies that
// all other fields would have their zero value
func (b *Blob) IsEmpty() bool {
	return len(b.data) == 0
}

// Sort sorts the blobs by their namespace.
func SortBlobs(blobs []*Blob) {
	sort.SliceStable(blobs, func(i, j int) bool {
		return blobs[i].Compare(blobs[j]) < 0
	})
}

// ToShares converts blob's data back to shares.
func (b *Blob) ToShares() ([]Share, error) {
	splitter := NewSparseShareSplitter()
	err := splitter.Write(b)
	if err != nil {
		return nil, err
	}
	return splitter.Export(), nil
}

// FibreBlobVersion returns the Fibre blob version for share version 2 blobs.
// Returns 0 and an error if the blob is not share version 2 or if the data is invalid.
func (b *Blob) FibreBlobVersion() (uint32, error) {
	if b.shareVersion != ShareVersionTwo {
		return 0, fmt.Errorf("fibre blob version is only available for share version 2, got version %d", b.shareVersion)
	}
	if len(b.data) < FibreBlobVersionSize {
		return 0, fmt.Errorf("blob data too short to contain fibre blob version: %d bytes", len(b.data))
	}
	return binary.BigEndian.Uint32(b.data[0:FibreBlobVersionSize]), nil
}

// Commitment returns the commitment for share version 2 blobs.
// Returns nil and an error if the blob is not share version 2 or if the data is invalid.
func (b *Blob) Commitment() ([]byte, error) {
	if b.shareVersion != ShareVersionTwo {
		return nil, fmt.Errorf("commitment is only available for share version 2, got version %d", b.shareVersion)
	}
	if len(b.data) != FibreBlobVersionSize+FibreCommitmentSize {
		return nil, fmt.Errorf("blob data has invalid size for share version 2: expected %d bytes, got %d", FibreBlobVersionSize+FibreCommitmentSize, len(b.data))
	}
	commitment := make([]byte, FibreCommitmentSize)
	copy(commitment, b.data[FibreBlobVersionSize:])
	return commitment, nil
}
