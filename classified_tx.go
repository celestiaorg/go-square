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
// go-square decodes formats it owns -- BlobTx, IndexWrapper, Blob -- and never
// decodes formats the Cosmos SDK owns -- Tx, TxBody, Any. Deciding whether a
// transaction is a MsgPayForFibre transaction requires the SDK's schema and the
// application's rules, so the caller makes that decision and passes the result
// in.
//
// This is not a stylistic preference. When go-square classified transactions
// itself it used a different protobuf schema than the SDK, and the two reached
// different answers about the same bytes: one square from the proposer, another
// from every validator re-deriving it. A library that re-derives a
// consensus-critical decision its caller has already made can only agree with
// the caller by coincidence.
type ClassifiedTx struct {
	// Bytes is the raw transaction exactly as it appears in the block.
	Bytes []byte

	// FibreTx is non-nil if and only if Bytes is a MsgPayForFibre transaction,
	// in which case it carries the system blob the caller synthesized. When set,
	// FibreTx.Tx must equal Bytes.
	FibreTx *tx.FibreTx
}

// NewClassifiedTx returns a ClassifiedTx for a transaction that is not a
// MsgPayForFibre transaction. Blob transactions belong here too: go-square
// recognises those itself from its own BlobTx wire format.
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

// Validate reports whether the classification is self-consistent. Construct and
// NewBuilder call it on every element so that a caller mistake fails loudly at
// the boundary instead of silently producing a square that disagrees with the
// caller's own view of the block.
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
	return nil
}
