package share

import (
	"errors"
	"fmt"
	"sort"

	v1 "github.com/celestiaorg/go-square/v2/proto/blob/v1"
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
	if shareVersion == 0 && signer != nil {
		return nil, errors.New("share version 0 does not support signer")
	}
	if shareVersion == 1 && len(signer) != SignerSize {
		return nil, fmt.Errorf("share version 1 requires signer of size %d bytes", SignerSize)
	}
	if shareVersion > MaxShareVersion {
		return nil, fmt.Errorf("share version can not be greater than MaxShareVersion %d", MaxShareVersion)
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

// UnmarshalBlob unmarshals a blob from the proto encoded bytes
func UnmarshalBlob(blob []byte) (*Blob, error) {
	pb := &v1.BlobProto{}
	err := proto.Unmarshal(blob, pb)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal blob: %w", err)
	}
	return NewBlobFromProto(pb)
}

// Marshal marshals the blob to the proto encoded bytes
func (b *Blob) Marshal() ([]byte, error) {
	pb := &v1.BlobProto{
		NamespaceId:      b.namespace.ID(),
		NamespaceVersion: uint32(b.namespace.Version()),
		ShareVersion:     uint32(b.shareVersion),
		Data:             b.data,
		Signer:           b.signer,
	}
	return proto.Marshal(pb)
}

// NewBlobFromProto creates a new blob from the proto generated type
func NewBlobFromProto(pb *v1.BlobProto) (*Blob, error) {
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
// construction that can only occur if using the nil value
func (b *Blob) IsEmpty() bool {
	return b.namespace.IsEmpty()
}

// Sort sorts the blobs by their namespace.
func SortBlobs(blobs []*Blob) {
	sort.SliceStable(blobs, func(i, j int) bool {
		return blobs[i].Compare(blobs[j]) < 0
	})
}
