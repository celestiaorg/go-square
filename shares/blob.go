package shares

import (
	"errors"
	"fmt"
	"sort"

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

// NewFromProto creates a Blob from the proto format and performs
// rudimentary validation checks on the structure
func NewBlobFromProto(pb *BlobProto) (*Blob, error) {
	if pb.NamespaceVersion > NamespaceVersionMax {
		return nil, errors.New("namespace version can not be greater than MaxNamespaceVersion")
	}
	if len(pb.Data) == 0 {
		return nil, errors.New("blob data can not be empty")
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

func (b *Blob) ToProto() *BlobProto {
	return &BlobProto{
		NamespaceId:      b.namespace.ID(),
		NamespaceVersion: uint32(b.namespace.Version()),
		ShareVersion:     uint32(b.shareVersion),
		Data:             b.data,
		Signer:           b.signer,
	}
}

func (b *Blob) Compare(other *Blob) int {
	return b.namespace.Compare(other.namespace)
}

// UnmarshalBlobTx attempts to unmarshal a transaction into blob transaction. If an
// error is thrown, false is returned.
func UnmarshalBlobTx(tx []byte) (*BlobTx, bool) {
	bTx := BlobTx{}
	err := proto.Unmarshal(tx, &bTx)
	if err != nil {
		return &bTx, false
	}
	// perform some quick basic checks to prevent false positives
	if bTx.TypeId != ProtoBlobTxTypeID {
		return &bTx, false
	}
	if len(bTx.Blobs) == 0 {
		return &bTx, false
	}
	for _, b := range bTx.Blobs {
		if len(b.NamespaceId) != NamespaceIDSize {
			return &bTx, false
		}
	}
	return &bTx, true
}

// MarshalBlobTx creates a BlobTx using a normal transaction and some number of
// blobs.
//
// NOTE: Any checks on the blobs or the transaction must be performed in the
// application
func MarshalBlobTx(tx []byte, blobs ...*Blob) ([]byte, error) {
	if len(blobs) == 0 {
		return nil, errors.New("at least one blob must be provided")
	}
	bTx := &BlobTx{
		Tx:     tx,
		Blobs:  blobsToProto(blobs),
		TypeId: ProtoBlobTxTypeID,
	}
	return proto.Marshal(bTx)
}

func blobsToProto(blobs []*Blob) []*BlobProto {
	pb := make([]*BlobProto, len(blobs))
	for i, b := range blobs {
		pb[i] = b.ToProto()
	}
	return pb
}

// Sort sorts the blobs by their namespace.
func SortBlobs(blobs []*Blob) {
	sort.SliceStable(blobs, func(i, j int) bool {
		return blobs[i].Compare(blobs[j]) < 0
	})
}

// UnmarshalIndexWrapper attempts to unmarshal the provided transaction into an
// IndexWrapper transaction. It returns true if the provided transaction is an
// IndexWrapper transaction. An IndexWrapper transaction is a transaction that contains
// a MsgPayForBlob that has been wrapped with a share index.
//
// NOTE: protobuf sometimes does not throw an error if the transaction passed is
// not a IndexWrapper, since the protobuf definition for MsgPayForBlob is
// kept in the app, we cannot perform further checks without creating an import
// cycle.
func UnmarshalIndexWrapper(tx []byte) (*IndexWrapper, bool) {
	indexWrapper := IndexWrapper{}
	// attempt to unmarshal into an IndexWrapper transaction
	err := proto.Unmarshal(tx, &indexWrapper)
	if err != nil {
		return &indexWrapper, false
	}
	if indexWrapper.TypeId != ProtoIndexWrapperTypeID {
		return &indexWrapper, false
	}
	return &indexWrapper, true
}

// MarshalIndexWrapper creates a wrapped Tx that includes the original transaction
// and the share index of the start of its blob.
//
// NOTE: must be unwrapped to be a viable sdk.Tx
func MarshalIndexWrapper(tx []byte, shareIndexes ...uint32) ([]byte, error) {
	wTx := IndexWrapper{
		Tx:           tx,
		ShareIndexes: shareIndexes,
		TypeId:       ProtoIndexWrapperTypeID,
	}
	return proto.Marshal(&wTx)
}
