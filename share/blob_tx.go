package share

import (
	"errors"

	v1 "github.com/celestiaorg/go-square/proto/blob/v1"
	"google.golang.org/protobuf/proto"
)

const (
	// ProtoBlobTxTypeID is included in each encoded BlobTx to help prevent
	// decoding binaries that are not actually BlobTxs.
	ProtoBlobTxTypeID = "BLOB"
)

type BlobTx struct {
	Tx    []byte
	Blobs []*Blob
}

// UnmarshalBlobTx attempts to unmarshal a transaction into blob transaction. It returns a boolean 
// If the bytes are of type BlobTx and an error if there is a problem with decoding
func UnmarshalBlobTx(tx []byte) (*BlobTx, bool, error) {
	bTx := v1.BlobTx{}
	err := proto.Unmarshal(tx, &bTx)
	if err != nil {
		return nil, false, err
	}
	// perform some quick basic checks to prevent false positives
	if bTx.TypeId != ProtoBlobTxTypeID {
		return nil, false, errors.New("invalid type id")
	}
	if len(bTx.Blobs) == 0 {
		return nil, true, errors.New("no blobs provided")
	}
	blobs := make([]*Blob, len(bTx.Blobs))
	for i, b := range bTx.Blobs {
		blobs[i], err = NewBlobFromProto(b)
		if err != nil {
			return nil, true, err
		}
	}
	return &BlobTx{
		Tx:    bTx.Tx,
		Blobs: blobs,
	}, true, nil
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
	bTx := &v1.BlobTx{
		Tx:     tx,
		Blobs:  blobsToProto(blobs),
		TypeId: ProtoBlobTxTypeID,
	}
	return proto.Marshal(bTx)
}

func blobsToProto(blobs []*Blob) []*v1.BlobProto {
	pb := make([]*v1.BlobProto, len(blobs))
	for i, b := range blobs {
		pb[i] = &v1.BlobProto{
			NamespaceId:      b.Namespace().ID(),
			NamespaceVersion: uint32(b.Namespace().Version()),
			ShareVersion:     uint32(b.ShareVersion()),
			Signer:           b.Signer(),
			Data:             b.Data(),
		}
	}
	return pb
}
