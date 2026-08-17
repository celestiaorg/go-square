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

// Validate returns an error if the classification is not self-consistent.
func (c ClassifiedTx) Validate() error {
	if len(c.Bytes) == 0 {
		return errors.New("classified tx has no bytes")
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
