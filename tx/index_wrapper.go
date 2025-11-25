package tx

import (
	"google.golang.org/protobuf/proto"

	v2 "github.com/celestiaorg/go-square/v4/proto/blob/v2"
)

const (
	// ProtoIndexWrapperTypeID is included in each encoded IndexWrapper to help prevent
	// decoding binaries that are not actually IndexWrappers.
	ProtoIndexWrapperTypeID = "INDX"
)

// UnmarshalIndexWrapper attempts to unmarshal the provided transaction into an
// IndexWrapper transaction. It returns true if the provided transaction is an
// IndexWrapper transaction. An IndexWrapper transaction is a transaction that contains
// a MsgPayForBlob that has been wrapped with a share index.
//
// NOTE: protobuf sometimes does not throw an error if the transaction passed is
// not a IndexWrapper, since the protobuf definition for MsgPayForBlob is
// kept in the app, we cannot perform further checks without creating an import
// cycle.
func UnmarshalIndexWrapper(tx []byte) (*v2.IndexWrapper, bool) {
	indexWrapper := v2.IndexWrapper{}
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
	wTx := NewIndexWrapper(tx, shareIndexes...)
	return proto.Marshal(wTx)
}

// NewIndexWrapper creates a new IndexWrapper transaction.
func NewIndexWrapper(tx []byte, shareIndexes ...uint32) *v2.IndexWrapper {
	return &v2.IndexWrapper{
		Tx:           tx,
		ShareIndexes: shareIndexes,
		TypeId:       ProtoIndexWrapperTypeID,
	}
}
