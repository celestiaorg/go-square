package share

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// CompactShareSplitter will write raw data compactly across a progressively
// increasing set of shares. It is used to lazily split block data such as
// transactions or intermediate state roots into shares.
type CompactShareSplitter struct {
	writer    sequenceWriter
	namespace Namespace
	done      bool
	// A partial share finalized by Export can be reopened by the next WriteTx.
	pendingBeforeExport *builder
	exportPadding       int
	shareVersion        uint8
	// shareRanges is a map from a transaction key to the range of shares it
	// occupies. The range assumes this compact share splitter is the only
	// thing in the data square (e.g. the range for the first tx starts at index
	// 0).
	shareRanges map[[sha256.Size]byte]Range
}

// NewCompactShareSplitter returns a CompactShareSplitter using the provided
// namespace and shareVersion.
func NewCompactShareSplitter(ns Namespace, shareVersion uint8) *CompactShareSplitter {
	sb, err := newBuilder(ns, shareVersion, true)
	if err != nil {
		panic(err)
	}

	return &CompactShareSplitter{
		writer:       sequenceWriter{shares: []Share{}, pending: sb},
		namespace:    ns,
		shareVersion: shareVersion,
		shareRanges:  map[[sha256.Size]byte]Range{},
	}
}

// WriteTx adds the delimited data for the provided tx to the underlying compact
// share splitter.
func (css *CompactShareSplitter) WriteTx(tx []byte) error {
	rawData, err := MarshalDelimitedTx(tx)
	if err != nil {
		return fmt.Errorf("included Tx in mem-pool that can not be encoded %v", tx)
	}

	css.resume()
	startShare := len(css.writer.shares)

	if err := css.write(rawData); err != nil {
		return err
	}
	endShare := css.Count()
	css.shareRanges[sha256.Sum256(tx)] = NewRange(startShare, endShare)

	return nil
}

// write adds the delimited data to the underlying compact shares.
func (css *CompactShareSplitter) write(rawData []byte) error {
	if err := css.writer.pending.MaybeWriteReservedBytes(); err != nil {
		return err
	}

	return css.writer.write(rawData)
}

// resume removes only the padding introduced by Export. Restore the partial
// builder before computing a new transaction's share range, since that
// transaction may start in the previously exported last share.
func (css *CompactShareSplitter) resume() {
	if !css.done {
		return
	}
	if css.pendingBeforeExport != nil {
		pending := css.pendingBeforeExport
		// Copy before writing again: exported Share values may still refer to
		// the padded buffer, which must not become the active write buffer.
		pending.rawShareData = bytes.Clone(pending.rawShareData[:ShareSize-css.exportPadding])
		css.writer.pending = pending
		css.writer.shares = css.writer.shares[:len(css.writer.shares)-1]
		css.pendingBeforeExport = nil
		css.exportPadding = 0
	}
	css.done = false
}

// Export returns the underlying compact shares
func (css *CompactShareSplitter) Export() ([]Share, error) {
	if css.isEmpty() {
		return []Share{}, nil
	}

	// in case Export is called multiple times
	if css.done {
		return css.writer.shares, nil
	}

	pending := css.writer.pending
	bytesOfPadding, err := css.writer.finalize()
	if err != nil {
		return []Share{}, err
	}

	sequenceLen := css.sequenceLen(bytesOfPadding)
	if err := css.writeSequenceLen(sequenceLen); err != nil {
		return []Share{}, err
	}
	if bytesOfPadding > 0 {
		css.pendingBeforeExport = pending
		css.exportPadding = bytesOfPadding
	}
	css.done = true
	return css.writer.shares, nil
}

// ShareRanges returns a map of share ranges to the corresponding tx keys. All
// share ranges in the map of shareRanges will be offset (i.e. incremented) by
// the shareRangeOffset provided. shareRangeOffset should be 0 for the first
// compact share sequence in the data square (transactions) but should be some
// non-zero number for subsequent compact share sequences (e.g. pfb txs).
func (css *CompactShareSplitter) ShareRanges(shareRangeOffset int) map[[sha256.Size]byte]Range {
	// apply the shareRangeOffset to all share ranges
	shareRanges := make(map[[sha256.Size]byte]Range, len(css.shareRanges))

	for k, v := range css.shareRanges {
		shareRanges[k] = Range{
			Start: v.Start + shareRangeOffset,
			End:   v.End + shareRangeOffset,
		}
	}

	return shareRanges
}

// writeSequenceLen writes the sequence length to the first share.
func (css *CompactShareSplitter) writeSequenceLen(sequenceLen uint32) error {
	if css.isEmpty() {
		return nil
	}

	// We may find a more efficient way to write seqLen
	b, err := newBuilder(css.namespace, css.shareVersion, true)
	if err != nil {
		return err
	}
	if err := b.ImportRawShare(css.writer.shares[0].ToBytes()); err != nil {
		return err
	}
	if err := b.WriteSequenceLen(sequenceLen); err != nil {
		return err
	}

	firstShare, err := b.Build()
	if err != nil {
		return err
	}

	// replace existing first share with new first share
	css.writer.shares[0] = firstShare

	return nil
}

// sequenceLen returns the total length in bytes of all units (transactions or
// intermediate state roots) written to this splitter. sequenceLen does not
// include the number of bytes occupied by the namespace ID, the share info
// byte, or the reserved bytes. sequenceLen does include the unit length
// delimiter prefixed to each unit.
func (css *CompactShareSplitter) sequenceLen(bytesOfPadding int) uint32 {
	if len(css.writer.shares) == 0 {
		return 0
	}
	if len(css.writer.shares) == 1 {
		return uint32(FirstCompactShareContentSize) - uint32(bytesOfPadding)
	}

	continuationSharesCount := len(css.writer.shares) - 1
	continuationSharesSequenceLen := continuationSharesCount * ContinuationCompactShareContentSize
	return uint32(FirstCompactShareContentSize + continuationSharesSequenceLen - bytesOfPadding)
}

// isEmpty returns whether this compact share splitter is empty.
func (css *CompactShareSplitter) isEmpty() bool {
	return len(css.writer.shares) == 0 && css.writer.pending.IsEmptyShare()
}

// Count returns the number of shares that would be made if `Export` was invoked
// on this compact share splitter.
func (css *CompactShareSplitter) Count() int {
	if !css.writer.pending.IsEmptyShare() && !css.done {
		// pending share is non-empty, so it will be zero padded and added to shares during export
		return len(css.writer.shares) + 1
	}
	return len(css.writer.shares)
}

// MarshalDelimitedTx prefixes a transaction with the length of the transaction
// encoded as a varint.
func MarshalDelimitedTx(tx []byte) ([]byte, error) {
	lenBuf := make([]byte, binary.MaxVarintLen64)
	length := uint64(len(tx))
	n := binary.PutUvarint(lenBuf, length)
	return append(lenBuf[:n], tx...), nil
}
