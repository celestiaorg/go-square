package square

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
)

// ClassifiedTx is a transaction whose kind the caller has already determined.
//
// go-square only decodes formats it owns (BlobTx, IndexWrapper, Blob).
// Recognizing a MsgPayForFibre transaction requires the Cosmos SDK's schema,
// so the caller classifies the transaction and passes the result in.
type ClassifiedTx struct {
	// Bytes is the raw transaction exactly as it appears in the block.
	Bytes []byte

	// FibreTx is non-nil if and only if Bytes is a MsgPayForFibre transaction,
	// in which case it carries the system blob the caller synthesized. When set,
	// FibreTx.Tx must equal Bytes.
	FibreTx *tx.FibreTx

	// BlobTx, when non-nil, is the decoded form of Bytes that the caller has
	// already unmarshalled. It is an optimization only: leaving it nil makes
	// the square decode Bytes itself, with identical results. Callers that set
	// it are asserting that it is what tx.UnmarshalBlobTx(Bytes) returns, or
	// equally tx.UnmarshalBlobTxNoCopy(Bytes), since the two agree --
	// the same assertion Builder.AppendBlobTx already takes on its argument,
	// so this grants no trust that the builder did not already grant, only
	// through Construct as well.
	//
	// Validate cannot confirm the assertion: FibreTx is cross-checked against
	// Bytes because FibreTx.Tx is the raw transaction, whereas BlobTx.Tx is
	// the inner SDK transaction, so checking it would mean re-marshalling --
	// exactly the work this field exists to avoid. What Validate does enforce
	// is every invariant tx.UnmarshalBlobTx guarantees about the decode
	// itself, which costs O(1) per blob and copies nothing: at least one
	// blob, no nil or empty blob, and no nested BlobTx. A pre-decode that
	// fails one of these is not something UnmarshalBlobTx could have
	// returned, so it is rejected rather than trusted.
	BlobTx *tx.BlobTx
}

// NewClassifiedTx returns a ClassifiedTx for a transaction that is not a
// MsgPayForFibre transaction (blob transactions belong here too).
func NewClassifiedTx(txBytes []byte) ClassifiedTx {
	return ClassifiedTx{Bytes: txBytes}
}

// NewClassifiedFibreTx returns a ClassifiedTx for a MsgPayForFibre transaction
// whose system blob the caller has already synthesized.
func NewClassifiedFibreTx(fibreTx *tx.FibreTx) (ClassifiedTx, error) {
	if fibreTx == nil {
		return ClassifiedTx{}, errors.New("nil fibre tx")
	}
	classified := ClassifiedTx{Bytes: fibreTx.Tx, FibreTx: fibreTx}
	if err := classified.Validate(); err != nil {
		return ClassifiedTx{}, err
	}
	return classified, nil
}

// NewClassifiedBlobTx returns a ClassifiedTx for a blob transaction the caller
// has already decoded. blobTx must be what tx.UnmarshalBlobTx(txBytes)
// returns; see the BlobTx field.
func NewClassifiedBlobTx(txBytes []byte, blobTx *tx.BlobTx) (ClassifiedTx, error) {
	if blobTx == nil {
		return ClassifiedTx{}, errors.New("nil blob tx")
	}
	classified := ClassifiedTx{Bytes: txBytes, BlobTx: blobTx}
	if err := classified.Validate(); err != nil {
		return ClassifiedTx{}, err
	}
	return classified, nil
}

// Validate returns an error if the classification is not self-consistent.
func (c ClassifiedTx) Validate() error {
	if len(c.Bytes) == 0 {
		return errors.New("classified tx has no bytes")
	}
	if c.BlobTx != nil {
		if c.FibreTx != nil {
			return errors.New("tx classified as both blob tx and fibre tx")
		}
		if err := c.validateBlobTx(); err != nil {
			return err
		}
	}
	if c.FibreTx == nil {
		return nil
	}
	if c.FibreTx.SystemBlob == nil {
		return errors.New("fibre tx has no system blob")
	}
	if !bytes.Equal(c.FibreTx.Tx, c.Bytes) {
		return fmt.Errorf("fibre tx bytes (%d bytes) differ from classified tx bytes (%d bytes)", len(c.FibreTx.Tx), len(c.Bytes))
	}
	if got := c.FibreTx.SystemBlob.ShareVersion(); got != share.ShareVersionTwo {
		return fmt.Errorf("system blobs must use share version %d, got %d", share.ShareVersionTwo, got)
	}
	// HasBlobTxTypeID is UnmarshalBlobTx's isBlob verdict without decoding
	// the blobs.
	if tx.HasBlobTxTypeID(c.Bytes) {
		return errors.New("tx classified as fibre is a blob tx")
	}
	return nil
}

// validateBlobTx rejects a pre-decoded blob tx that tx.UnmarshalBlobTx could
// not have returned. It cannot prove the decode belongs to Bytes -- that needs
// a re-marshal, which is the cost this field exists to avoid -- but every
// invariant UnmarshalBlobTx guarantees about the decoded value itself is
// checked here, at O(1) per blob and copying no blob data.
func (c ClassifiedTx) validateBlobTx() error {
	// UnmarshalBlobTx rejects a BlobTx with no blobs.
	if len(c.BlobTx.Blobs) == 0 {
		return errors.New("blob tx has no blobs")
	}
	// UnmarshalBlobTx rejects a BlobTx whose inner tx is itself a BlobTx. The
	// bytes path and the pre-decoded path would otherwise disagree on whether
	// the same transaction is valid, which for a square under construction is
	// a disagreement about the data root.
	if tx.HasBlobTxTypeID(c.BlobTx.Tx) {
		return fmt.Errorf("blob tx wraps another blob tx: %w", tx.ErrNestedBlobTx)
	}
	// UnmarshalBlobTx builds blobs with share.NewBlobFromProto, which never
	// yields a nil or empty one. Without this check a nil blob is a nil
	// pointer dereference in the builder rather than an error, and Construct
	// runs on a consensus path.
	for i, blob := range c.BlobTx.Blobs {
		if blob == nil || blob.IsEmpty() {
			return fmt.Errorf("blob tx blob %d is nil or empty", i)
		}
	}
	return nil
}

// blobTx returns the decoded blob tx for c, using the caller's pre-decoded
// form when present and unmarshalling Bytes otherwise. The results match
// tx.UnmarshalBlobTx: the second value reports whether Bytes is a blob tx at
// all, and an error alongside it means it is one but could not be decoded.
//
// The fallback decodes with tx.UnmarshalBlobTxNoCopy, so the result aliases
// Bytes rather than copying the inner tx and the blob data. Square
// construction only reads the result and Bytes is the transaction exactly as
// it appears in the block, which nothing modifies.
//
// Both square construction paths go through here so that the fallback cannot
// drift between them.
func (c ClassifiedTx) blobTx() (*tx.BlobTx, bool, error) {
	if c.BlobTx != nil {
		return c.BlobTx, true, nil
	}
	return tx.UnmarshalBlobTxNoCopy(c.Bytes)
}
