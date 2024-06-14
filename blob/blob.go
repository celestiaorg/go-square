// Package blob provides types and functions for working with blobs, blob
// transactions, and index wrapper transactions.
package blob

import (
	"errors"
	"fmt"
	"sort"

	ns "github.com/celestiaorg/go-square/namespace"
	"google.golang.org/protobuf/proto"
)

// SupportedBlobNamespaceVersions is a list of namespace versions that can be specified by a user for blobs.
var SupportedBlobNamespaceVersions = []uint8{ns.NamespaceVersionZero}

// ProtoBlobTxTypeID is included in each encoded BlobTx to help prevent
// decoding binaries that are not actually BlobTxs.
const ProtoBlobTxTypeID = "BLOB"

// ProtoIndexWrapperTypeID is included in each encoded IndexWrapper to help prevent
// decoding binaries that are not actually IndexWrappers.
const ProtoIndexWrapperTypeID = "INDX"

// MaxShareVersion is the maximum value a share version can be. See: [shares.MaxShareVersion].
const MaxShareVersion = 127

// Blob (stands for binary large object) is a core type that represents data
// to be submitted to the Celestia network alongside an accompanying namespace
// and optional signer (for proving the author of the blob)
type Blob struct {
	namespace    ns.Namespace
	data         []byte
	shareVersion uint8
	signer       string
}

// New creates a new coretypes.Blob from the provided data after performing
// basic stateless checks over it.
func New(ns ns.Namespace, data []byte, shareVersion uint8, signer string) *Blob {
	return &Blob{
		namespace:    ns,
		data:         data,
		shareVersion: shareVersion,
		signer:       signer,
	}
}

// NewFromProto creates a Blob from the proto format and performs
// rudimentary validation checks on the structure
func NewFromProto(pb *BlobProto) (*Blob, error) {
	if pb.ShareVersion > MaxShareVersion {
		return nil, errors.New("share version can not be greater than MaxShareVersion")
	}
	if pb.NamespaceVersion > ns.NamespaceVersionMax {
		return nil, errors.New("namespace version can not be greater than MaxNamespaceVersion")
	}
	if len(pb.Data) == 0 {
		return nil, errors.New("blob data can not be empty")
	}
	ns, err := ns.New(uint8(pb.NamespaceVersion), pb.NamespaceId)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace: %w", err)
	}
	return &Blob{
		namespace:    ns,
		data:         pb.Data,
		shareVersion: uint8(pb.ShareVersion),
		signer:       pb.Signer,
	}, nil
}

// Namespace returns the namespace of the blob
func (b *Blob) Namespace() ns.Namespace {
	return b.namespace
}

// ShareVersion returns the share version of the blob
func (b *Blob) ShareVersion() uint8 {
	return b.shareVersion
}

// Signer returns the signer of the blob
func (b *Blob) Signer() string {
	return b.signer
}

// Data returns the data of the blob
func (b *Blob) Data() []byte {
	return b.data
}

func (b *Blob) ToProto() *BlobProto {
	return &BlobProto{
		NamespaceId:      b.namespace.Bytes(),
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
		if len(b.NamespaceId) != ns.NamespaceIDSize {
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
func Sort(blobs []*Blob) {
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
