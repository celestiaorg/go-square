package square

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/celestiaorg/go-square/v2/inclusion"
	v1 "github.com/celestiaorg/go-square/v2/proto/blob/v1"
	"github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/go-square/v2/tx"
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/proto"
)

type Builder struct {
	// maxSquareSize is the maximum number of rows (or columns) in the original data square
	maxSquareSize int
	// currentSize is an overestimate for the number of shares used by this builder.
	currentSize int

	// here we keep track of the pending data to go in a square
	Txs   [][]byte
	Pfbs  []*v1.IndexWrapper
	Blobs []*Element

	// for compact shares we use a counter to track the amount of shares needed
	TxCounter  *share.CompactShareCounter
	PfbCounter *share.CompactShareCounter

	// for reverting the last addition
	lastTxSizeChange     int
	lastBlobTxSizeChange int
	// track if a revert has already occurred to prevent multiple reverts
	txReverted     bool
	blobTxReverted bool

	done                 bool
	subtreeRootThreshold int
}

func NewBuilder(maxSquareSize int, subtreeRootThreshold int, txs ...[]byte) (*Builder, error) {
	if maxSquareSize <= 0 {
		return nil, errors.New("max square size must be strictly positive")
	}
	if !IsPowerOfTwo(maxSquareSize) {
		return nil, errors.New("max square size must be a power of two")
	}
	builder := &Builder{
		maxSquareSize:        maxSquareSize,
		subtreeRootThreshold: subtreeRootThreshold,
		Blobs:                make([]*Element, 0),
		Pfbs:                 make([]*v1.IndexWrapper, 0),
		Txs:                  make([][]byte, 0),
		TxCounter:            share.NewCompactShareCounter(),
		PfbCounter:           share.NewCompactShareCounter(),
	}
	seenFirstBlobTx := false
	for idx, txBytes := range txs {
		blobTx, isBlobTx, err := tx.UnmarshalBlobTx(txBytes)
		if err != nil && isBlobTx {
			return nil, fmt.Errorf("unmarshalling blob tx at index %d: %w", idx, err)
		}
		if isBlobTx {
			seenFirstBlobTx = true
			if !builder.AppendBlobTx(blobTx) {
				return nil, fmt.Errorf("not enough space to append blob tx at index %d", idx)
			}
		} else {
			if seenFirstBlobTx {
				return nil, fmt.Errorf("normal tx at index %d can not be appended after blob tx", idx)
			}
			if !builder.AppendTx(txBytes) {
				return nil, fmt.Errorf("not enough space to append tx at index %d", idx)
			}
		}
	}
	return builder, nil
}

// AppendTx attempts to allocate the transaction to the square. It returns false if there is not
// enough space in the square to fit the transaction.
func (b *Builder) AppendTx(tx []byte) bool {
	lenChange := b.TxCounter.Add(len(tx))
	if b.canFit(lenChange) {
		b.Txs = append(b.Txs, tx)
		b.currentSize += lenChange
		b.lastTxSizeChange = lenChange
		b.txReverted = false // reset revert flag
		b.done = false
		return true
	}
	b.TxCounter.Revert()
	return false
}

// RevertLastTx reverts the last transaction that was appended to the builder.
// It returns an error if there are no transactions to revert or if this method
// has been called consecutively without adding a tx in between calls.
func (b *Builder) RevertLastTx() error {
	if len(b.Txs) == 0 {
		return errors.New("no transactions to revert")
	}
	if b.txReverted {
		return errors.New("cannot revert: last transaction has already been reverted")
	}

	b.Txs = b.Txs[:len(b.Txs)-1]
	b.TxCounter.Revert()
	b.currentSize -= b.lastTxSizeChange
	b.txReverted = true
	b.done = false

	return nil
}

// AppendBlobTx attempts to allocate the blob transaction to the square. It returns false if there is not
// enough space in the square to fit the transaction.
func (b *Builder) AppendBlobTx(blobTx *tx.BlobTx) bool {
	iw := tx.NewIndexWrapper(blobTx.Tx, worstCaseShareIndexes(len(blobTx.Blobs))...)
	size := proto.Size(iw)
	pfbShareDiff := b.PfbCounter.Add(size)

	// create a new blob element for each blob and track the worst-case share count
	blobElements := make([]*Element, len(blobTx.Blobs))
	maxBlobShareCount := 0
	for idx, blob := range blobTx.Blobs {
		blobElements[idx] = newElement(blob, len(b.Pfbs), idx, b.subtreeRootThreshold)
		maxBlobShareCount += blobElements[idx].maxShareOffset()
	}

	totalSizeChange := pfbShareDiff + maxBlobShareCount
	if b.canFit(totalSizeChange) {
		b.Blobs = append(b.Blobs, blobElements...)
		b.Pfbs = append(b.Pfbs, iw)
		b.currentSize += totalSizeChange
		b.lastBlobTxSizeChange = totalSizeChange
		b.blobTxReverted = false // reset revert flag
		b.done = false
		return true
	}
	b.PfbCounter.Revert()
	return false
}

// RevertLastBlobTx reverts the last blob transaction that was appended to the builder.
// It returns an error if there are no blob transactions to revert or if this method
// has been called consecutively without adding a tx in between calls.
func (b *Builder) RevertLastBlobTx() error {
	if len(b.Pfbs) == 0 {
		return errors.New("no blob transactions to revert")
	}
	if b.blobTxReverted {
		return errors.New("cannot revert: last blob transaction has already been reverted")
	}

	lastPfbIndex := len(b.Pfbs) - 1

	var remainingBlobs []*Element
	for _, blob := range b.Blobs {
		if blob.PfbIndex != lastPfbIndex {
			remainingBlobs = append(remainingBlobs, blob)
		}
	}

	b.Blobs = remainingBlobs
	b.Pfbs = b.Pfbs[:len(b.Pfbs)-1]
	b.PfbCounter.Revert()
	b.currentSize -= b.lastBlobTxSizeChange
	b.blobTxReverted = true
	b.done = false

	return nil
}

// Export constructs the square.
func (b *Builder) Export() (Square, error) {
	// if there are no transactions, return an empty square
	if b.IsEmpty() {
		return EmptySquare(), nil
	}

	// calculate the square size.
	// NOTE: A future optimization could be to recalculate the currentSize based on the actual
	// interblob padding used when the blobs are correctly ordered instead of using worst case padding.
	ss := inclusion.BlobMinSquareSize(b.currentSize)

	// Sort the blobs by shares. This uses SliceStable to preserve the order
	// of blobs within a namespace because b.Blobs are already ordered by tx
	// priority.
	sort.SliceStable(b.Blobs, func(i, j int) bool {
		ns1 := b.Blobs[i].Blob.Namespace().Bytes()
		ns2 := b.Blobs[j].Blob.Namespace().Bytes()
		return bytes.Compare(ns1, ns2) < 0
	})

	// write all the regular transactions into compact shares
	txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
	for _, tx := range b.Txs {
		if err := txWriter.WriteTx(tx); err != nil {
			return nil, fmt.Errorf("writing tx into compact shares: %w", err)
		}
	}

	// begin to iteratively add blobs to the sparse share splitter calculating the actual padding
	nonReservedStart := b.TxCounter.Size() + b.PfbCounter.Size()
	cursor := nonReservedStart
	endOfLastBlob := nonReservedStart
	blobWriter := share.NewSparseShareSplitter()
	for i, element := range b.Blobs {
		// NextShareIndex returned where the next blob should start so as to comply with the share commitment rules
		// We fill out the remaining
		cursor = inclusion.NextShareIndex(cursor, element.NumShares, b.subtreeRootThreshold)
		if i == 0 {
			nonReservedStart = cursor
		}

		// defensively check that the actual padding never exceeds the max padding initially allocated for it
		padding := cursor - endOfLastBlob
		if padding > element.MaxPadding {
			return nil, fmt.Errorf("blob has %d padding shares, but %d was the max possible", padding, element.MaxPadding)
		}

		// record the starting share index of the blob in the PFB that paid for it
		b.Pfbs[element.PfbIndex].ShareIndexes[element.BlobIndex] = uint32(cursor)
		// If this is not the first blob, we add padding by writing padded shares to the previous blob
		// (which could be of a different namespace)
		if i > 0 {
			if err := blobWriter.WriteNamespacePaddingShares(padding); err != nil {
				return nil, fmt.Errorf("writing padding into sparse shares: %w", err)
			}
		}
		// Finally write the blob itself
		if err := blobWriter.Write(element.Blob); err != nil {
			return nil, fmt.Errorf("writing blob into sparse shares: %w", err)
		}
		// increment the cursor by the size of the blob
		cursor += element.NumShares
		endOfLastBlob = cursor
	}

	// write all the pay for blob transactions into compact shares. We need to do this after allocating the blobs to their
	// appropriate shares as the starting index of each blob needs to be included in the PFB transaction
	pfbWriter := share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero)
	for _, iw := range b.Pfbs {
		iwBytes, err := proto.Marshal(iw)
		if err != nil {
			return nil, fmt.Errorf("marshaling pay for blob tx: %w", err)
		}
		if err := pfbWriter.WriteTx(iwBytes); err != nil {
			return nil, fmt.Errorf("writing pay for blob tx into compact shares: %w", err)
		}
	}

	// defensively check that the counter is always greater in share count than the pfbTxWriter.
	if b.PfbCounter.Size() < pfbWriter.Count() {
		return nil, fmt.Errorf("pfbCounter.Size() < pfbTxWriter.Count(): %d < %d", b.PfbCounter.Size(), pfbWriter.Count())
	}

	// Write out the square
	square, err := WriteSquare(txWriter, pfbWriter, blobWriter, nonReservedStart, ss)
	if err != nil {
		return nil, fmt.Errorf("writing square: %w", err)
	}

	b.done = true

	return square, nil
}

// FindBlobStartingIndex returns the starting share index of the blob in the square. It takes
// the index of the pfb in the tx set and the index of the blob within the PFB.
func (b *Builder) FindBlobStartingIndex(pfbIndex, blobIndex int) (int, error) {
	if pfbIndex < len(b.Txs) {
		return 0, fmt.Errorf("pfbIndex %d does not match a pfb", pfbIndex)
	}
	pfbIndex -= len(b.Txs)
	if pfbIndex >= len(b.Pfbs) {
		return 0, fmt.Errorf("pfbIndex %d out of range", pfbIndex)
	}
	if blobIndex < 0 {
		return 0, fmt.Errorf("blobIndex %d must not be negative", blobIndex)
	}

	// The share indexes of each blob needs to be computed thus we need to ensure
	// that we have called Export() before we can return the share index of a blob
	if !b.done {
		_, err := b.Export()
		if err != nil {
			return 0, fmt.Errorf("building square: %w", err)
		}
	}

	if blobIndex >= len(b.Pfbs[pfbIndex].ShareIndexes) {
		return 0, fmt.Errorf("blobIndex %d out of range", blobIndex)
	}

	return int(b.Pfbs[pfbIndex].ShareIndexes[blobIndex]), nil
}

// BlobShareLength returns the amount of shares a blob takes up in the square. It takes
// the index of the pfb in the tx set and the index of the blob within the PFB.
// TODO: we could look in to creating a map to avoid O(n) lookup when we expect large
// numbers of blobs
func (b *Builder) BlobShareLength(pfbIndex, blobIndex int) (int, error) {
	if pfbIndex < len(b.Txs) {
		return 0, fmt.Errorf("pfbIndex %d does not match a pfb", pfbIndex)
	}
	pfbIndex -= len(b.Txs)
	if pfbIndex >= len(b.Pfbs) {
		return 0, fmt.Errorf("pfbIndex %d out of range", pfbIndex)
	}
	if blobIndex < 0 {
		return 0, fmt.Errorf("blobIndex %d must not be negative", blobIndex)
	}

	for _, blob := range b.Blobs {
		if blob.PfbIndex == pfbIndex && blob.BlobIndex == blobIndex {
			return blob.NumShares, nil
		}
	}
	return 0, fmt.Errorf("blob not found")
}

// FindTxShareRange returns the range of shares occupied by the tx at txIndex.
// The indexes are both inclusive.
func (b *Builder) FindTxShareRange(txIndex int) (share.Range, error) {
	// the square must be built before we can find the share range as we need to compute
	// the wrapped indexes for the PFBs. NOTE: If a tx isn't a PFB, we could theoretically
	// calculate the index without having to build the entire square.
	if !b.done {
		_, err := b.Export()
		if err != nil {
			return share.Range{}, fmt.Errorf("building square: %w", err)
		}
	}
	if txIndex < 0 {
		return share.Range{}, fmt.Errorf("txIndex %d must not be negative", txIndex)
	}

	if txIndex >= len(b.Txs)+len(b.Pfbs) {
		return share.Range{}, fmt.Errorf("txIndex %d out of range", txIndex)
	}

	txWriter := share.NewCompactShareCounter()
	pfbWriter := share.NewCompactShareCounter()
	for i := 0; i < txIndex; i++ {
		if i < len(b.Txs) {
			_ = txWriter.Add(len(b.Txs[i]))
		} else {
			size := proto.Size(b.Pfbs[i-len(b.Txs)])
			_ = pfbWriter.Add(size)
		}
	}

	start := txWriter.Size() + pfbWriter.Size() - 1

	// the chosen tx is a regular tx
	if txIndex < len(b.Txs) {
		// If the remainder is 0, it means the tx will begin with the next share
		// so we need to increment the start index.
		if txWriter.Remainder() == 0 {
			start++
		}
		_ = txWriter.Add(len(b.Txs[txIndex]))
	} else { // the chosen tx is a PFB
		// If the remainder is 0, it means the tx will begin with the next share
		// so we need to increment the start index.
		if pfbWriter.Remainder() == 0 {
			start++
		}
		size := proto.Size(b.Pfbs[txIndex-len(b.Txs)])
		_ = pfbWriter.Add(size)
	}
	end := txWriter.Size() + pfbWriter.Size()

	return share.NewRange(start, end), nil
}

func (b *Builder) GetWrappedPFB(txIndex int) (*v1.IndexWrapper, error) {
	if txIndex < 0 {
		return nil, fmt.Errorf("txIndex %d must not be negative", txIndex)
	}

	if txIndex < len(b.Txs) {
		return nil, fmt.Errorf("txIndex %d does not match a pfb", txIndex)
	}

	if txIndex >= len(b.Txs)+len(b.Pfbs) {
		return nil, fmt.Errorf("txIndex %d out of range", txIndex)
	}

	if !b.done {
		_, err := b.Export()
		if err != nil {
			return nil, fmt.Errorf("building square: %w", err)
		}
	}

	return b.Pfbs[txIndex-len(b.Txs)], nil
}

func (b *Builder) CurrentSize() int {
	return b.currentSize
}

func (b *Builder) SubtreeRootThreshold() int {
	return b.subtreeRootThreshold
}

func (b *Builder) NumPFBs() int {
	return len(b.Pfbs)
}

func (b *Builder) NumTxs() int {
	return len(b.Txs) + len(b.Pfbs)
}

func (b *Builder) canFit(shareNum int) bool {
	return b.currentSize+shareNum <= (b.maxSquareSize * b.maxSquareSize)
}

func (b *Builder) IsEmpty() bool {
	return b.TxCounter.Size() == 0 && b.PfbCounter.Size() == 0
}

type Element struct {
	Blob       *share.Blob
	PfbIndex   int
	BlobIndex  int
	NumShares  int
	MaxPadding int
}

func newElement(blob *share.Blob, pfbIndex, blobIndex, subtreeRootThreshold int) *Element {
	numShares := share.SparseSharesNeeded(uint32(len(blob.Data())))
	return &Element{
		Blob:      blob,
		PfbIndex:  pfbIndex,
		BlobIndex: blobIndex,
		NumShares: numShares,
		//
		// For calculating the maximum possible padding consider the following tree
		// where each leaf corresponds to a share.
		//
		//	Depth       Position
		//	0              0
		//	              / \
		//	             /   \
		//	1           0     1
		//	           /\     /\
		//	2         0  1   2  3
		//	         /\  /\ /\  /\
		//	3       0 1 2 3 4 5 6 7
		//
		// Imagine if, according to the share commitment rules, a transcation took up 11 shares
		// and had the merkle mountain tree commitment of 4,4,2,1. The first part of the share commitment
		// would then be something that spans 4 shares and has a depth of 1. The worst case padding
		// would be if the last transaction had a share at leaf index 0. Thus three padding shares would
		// be needed to start the transaction at index 4 and be aligned with the first commitment.
		// Thus the rule is to take the subtreewidh of the share size and subtract 1.
		//
		// Note that the padding would actually belong to the namespace of the transaction before it, but
		// this makes no difference to the total share size.
		MaxPadding: inclusion.SubTreeWidth(numShares, subtreeRootThreshold) - 1,
	}
}

func (e Element) maxShareOffset() int {
	return e.NumShares + e.MaxPadding
}

// worstCaseShareIndexes returns the largest possible share indexes for a set of
// blobs. Largest possible is "worst" in that protobuf uses varints to encode
// integers, so larger integers can require more bytes to encode.
//
// Note: the implementation of this function assumes that the worst case share
// index is always 128 * 128 to preserve backwards compatibility with
// celestia-app v1.x.
func worstCaseShareIndexes(blobs int) []uint32 {
	// TODO: de-duplicate this constant with celestia-app SquareSizeUpperBound constant.
	// https://github.com/celestiaorg/celestia-app/blob/a93bb625c6dc0ae6c7c357e9991815a68ab33c79/pkg/appconsts/v1/app_consts.go#L5
	squareSizeUpperBound := 128
	worstCaseShareIndex := squareSizeUpperBound * squareSizeUpperBound
	shareIndexes := make([]uint32, blobs)
	for i := range shareIndexes {
		shareIndexes[i] = uint32(worstCaseShareIndex)
	}
	return shareIndexes
}

// IsPowerOfTwo returns true if input is a power of two.
func IsPowerOfTwo[I constraints.Integer](input I) bool {
	return input > 0 && input&(input-1) == 0
}
