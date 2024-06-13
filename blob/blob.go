// Package blob provides types and functions for working with blobs, blob
// transactions, and index wrapper transactions.
package blob

import (
	"bytes"
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

// New creates a new coretypes.Blob from the provided data after performing
// basic stateless checks over it.
func New(ns ns.Namespace, blob []byte, shareVersion uint8) *Blob {
	return &Blob{
		NamespaceId:      ns.ID(),
		Data:             blob,
		ShareVersion:     uint32(shareVersion),
		NamespaceVersion: uint32(ns.Version()),
	}
}

// Namespace returns the namespace of the blob
func (b *Blob) Namespace() (ns.Namespace, error) {
	return ns.NewFromBytes(b.RawNamespace())
}

// RawNamespace returns the namespace of the blob
func (b *Blob) RawNamespace() []byte {
	namespace := make([]byte, ns.NamespaceSize)
	namespace[ns.VersionIndex] = uint8(b.NamespaceVersion)
	copy(namespace[ns.NamespaceVersionSize:], b.NamespaceId)
	return namespace
}

// Validate runs a stateless validity check on the form of the struct.
func (b *Blob) Validate() error {
	if b == nil {
		return errors.New("nil blob")
	}
	if len(b.NamespaceId) != ns.NamespaceIDSize {
		return fmt.Errorf("namespace id must be %d bytes", ns.NamespaceIDSize)
	}
	if b.ShareVersion > MaxShareVersion {
		return errors.New("share version can not be greater than MaxShareVersion")
	}
	if b.NamespaceVersion > ns.NamespaceVersionMax {
		return errors.New("namespace version can not be greater than MaxNamespaceVersion")
	}
	if len(b.Data) == 0 {
		return errors.New("blob data can not be empty")
	}
	return nil
}

func (b *Blob) Compare(other *Blob) int {
	return bytes.Compare(b.RawNamespace(), other.RawNamespace())
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
	bTx := &BlobTx{
		Tx:     tx,
		Blobs:  blobs,
		TypeId: ProtoBlobTxTypeID,
	}
	return proto.Marshal(bTx)
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
