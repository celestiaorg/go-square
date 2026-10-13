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
	// it are asserting that it is what tx.UnmarshalBlobTx(Bytes) returns --
	// the same assertion Builder.AppendBlobTx already takes on its argument,
	// so this grants no trust that the builder did not already grant, only
	// through Construct as well.
	//
	// Validate cannot confirm the assertion: FibreTx is cross-checked against
	// Bytes because FibreTx.Tx is the raw transaction, whereas BlobTx.Tx is
	// the inner SDK transaction, so checking it would mean re-marshalling --
	// exactly the work this field exists to avoid.
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
		// tx.UnmarshalBlobTx rejects a BlobTx with no blobs, so a pre-decoded
		// one must be rejected here too.
		if len(c.BlobTx.Blobs) == 0 {
			return errors.New("blob tx has no blobs")
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
	if _, isBlobTx, _ := tx.UnmarshalBlobTx(c.Bytes); isBlobTx {
		return errors.New("tx classified as fibre is a blob tx")
	}
	return nil
}

// blobTx returns the decoded blob tx for c, using the caller's pre-decoded
// form when present and unmarshalling Bytes otherwise. The results match
// tx.UnmarshalBlobTx: the second value reports whether Bytes is a blob tx at
// all, and an error alongside it means it is one but could not be decoded.
//
// Both square construction paths go through here so that the fallback cannot
// drift between them.
func (c ClassifiedTx) blobTx() (*tx.BlobTx, bool, error) {
	if c.BlobTx != nil {
		return c.BlobTx, true, nil
	}
	return tx.UnmarshalBlobTx(c.Bytes)
}
