package blob

import (
	"bytes"
	"errors"
	fmt "fmt"
	math "math"
	"sort"

	"github.com/celestiaorg/go-square/pkg/namespace"
	"google.golang.org/protobuf/proto"
)

// SupportedBlobNamespaceVersions is a list of namespace versions that can be specified by a user for blobs.
var SupportedBlobNamespaceVersions = []uint8{namespace.NamespaceVersionZero}

// ProtoBlobTxTypeID is included in each encoded BlobTx to help prevent
// decoding binaries that are not actually BlobTxs.
const ProtoBlobTxTypeID = "BLOB"

// ProtoIndexWrapperTypeID is included in each encoded IndexWrapper to help prevent
// decoding binaries that are not actually IndexWrappers.
const ProtoIndexWrapperTypeID = "INDX"

// NewBlob creates a new coretypes.Blob from the provided data after performing
// basic stateless checks over it.
func New(ns namespace.Namespace, blob []byte, shareVersion uint8) *Blob {
	return &Blob{
		NamespaceId:      ns.ID,
		Data:             blob,
		ShareVersion:     uint32(shareVersion),
		NamespaceVersion: uint32(ns.Version),
	}
}

// Namespace returns the namespace of the blob
func (b *Blob) Namespace() namespace.Namespace {
	return namespace.Namespace{
		Version: uint8(b.NamespaceVersion),
		ID:      b.NamespaceId,
	}
}

// Validate runs a stateless validity check on the form of the struct.
func (b *Blob) Validate() error {
	if b == nil {
		return errors.New("nil blob")
	}
	if len(b.NamespaceId) != namespace.NamespaceIDSize {
		return fmt.Errorf("namespace id must be %d bytes", namespace.NamespaceIDSize)
	}
	if b.ShareVersion > math.MaxUint8 {
		return errors.New("share version can not be greater than MaxShareVersion")
	}
	if b.NamespaceVersion > namespace.NamespaceVersionMax {
		return errors.New("namespace version can not be greater than MaxNamespaceVersion")
	}
	if len(b.Data) == 0 {
		return errors.New("blob data can not be empty")
	}
	return nil
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
		if len(b.NamespaceId) != namespace.NamespaceIDSize {
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

func Sort(blobs []*Blob) {
	sort.SliceStable(blobs, func(i, j int) bool {
		return bytes.Compare(blobs[i].Namespace().Bytes(), blobs[j].Namespace().Bytes()) < 0
	})
}

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
