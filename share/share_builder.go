package share

import (
	"encoding/binary"
	"errors"
)

type builder struct {
	namespace      Namespace
	shareVersion   uint8
	isFirstShare   bool
	isCompactShare bool
	rawShareData   []byte
}

func newEmptyBuilder() *builder {
	return &builder{
		rawShareData: make([]byte, 0, ShareSize),
	}
}

// newBuilder returns a new share builder.
func newBuilder(ns Namespace, shareVersion uint8, isFirstShare bool) (*builder, error) {
	b := builder{
		namespace:      ns,
		shareVersion:   shareVersion,
		isFirstShare:   isFirstShare,
		isCompactShare: isCompactShare(ns),
	}
	if err := b.init(); err != nil {
		return nil, err
	}
	return &b, nil
}

// init initializes the share builder by populating rawShareData.
func (b *builder) init() error {
	if b.isCompactShare {
		return b.prepareCompactShare()
	}
	return b.prepareSparseShare()
}

func (b *builder) AvailableBytes() int {
	return ShareSize - len(b.rawShareData)
}

func (b *builder) ImportRawShare(rawBytes []byte) *builder {
	b.rawShareData = rawBytes
	return b
}

func (b *builder) AddData(rawData []byte) (rawDataLeftOver []byte) {
	// find the len left in the pending share
	pendingLeft := ShareSize - len(b.rawShareData)

	// if we can simply add the tx to the share without creating a new
	// pending share, do so and return
	if len(rawData) <= pendingLeft {
		b.rawShareData = append(b.rawShareData, rawData...)
		return nil
	}

	// if we can only add a portion of the rawData to the pending share,
	// then we add it and add the pending share to the finalized shares.
	chunk := rawData[:pendingLeft]
	b.rawShareData = append(b.rawShareData, chunk...)

	// We need to finish this share and start a new one
	// so we return the leftover to be written into a new share
	return rawData[pendingLeft:]
}

func (b *builder) Build() (*Share, error) {
	return NewShare(b.rawShareData)
}

// IsEmptyShare returns true if no data has been written to the share
func (b *builder) IsEmptyShare() bool {
	expectedLen := NamespaceSize + ShareInfoBytes
	if b.isCompactShare {
		expectedLen += ShareReservedBytes
	}
	if b.isFirstShare {
		expectedLen += SequenceLenBytes
	}
	return len(b.rawShareData) == expectedLen
}

func (b *builder) ZeroPadIfNecessary() (bytesOfPadding int) {
	b.rawShareData, bytesOfPadding = zeroPadIfNecessary(b.rawShareData, ShareSize)
	return bytesOfPadding
}

// isEmptyReservedBytes returns true if the reserved bytes are empty.
func (b *builder) isEmptyReservedBytes() (bool, error) {
	indexOfReservedBytes := b.indexOfReservedBytes()
	reservedBytes, err := ParseReservedBytes(b.rawShareData[indexOfReservedBytes : indexOfReservedBytes+ShareReservedBytes])
	if err != nil {
		return false, err
	}
	return reservedBytes == 0, nil
}

// indexOfReservedBytes returns the index of the reserved bytes in the share.
func (b *builder) indexOfReservedBytes() int {
	if b.isFirstShare {
		// if the share is the first share, the reserved bytes follow the namespace, info byte, and sequence length
		return NamespaceSize + ShareInfoBytes + SequenceLenBytes
	}
	// if the share is not the first share, the reserved bytes follow the namespace and info byte
	return NamespaceSize + ShareInfoBytes
}

// indexOfInfoBytes returns the index of the InfoBytes.
func (b *builder) indexOfInfoBytes() int {
	// the info byte is immediately after the namespace
	return NamespaceSize
}

// MaybeWriteReservedBytes will be a no-op if the reserved bytes
// have already been populated. If the reserved bytes are empty, it will write
// the location of the next unit of data to the reserved bytes.
func (b *builder) MaybeWriteReservedBytes() error {
	if !b.isCompactShare {
		return errors.New("this is not a compact share")
	}

	empty, err := b.isEmptyReservedBytes()
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}

	byteIndexOfNextUnit := len(b.rawShareData)
	reservedBytes, err := NewReservedBytes(uint32(byteIndexOfNextUnit))
	if err != nil {
		return err
	}

	indexOfReservedBytes := b.indexOfReservedBytes()
	// overwrite the reserved bytes of the pending share
	for i := 0; i < ShareReservedBytes; i++ {
		b.rawShareData[indexOfReservedBytes+i] = reservedBytes[i]
	}
	return nil
}

// WriteSequenceLen writes the sequence length to the first share.
func (b *builder) WriteSequenceLen(sequenceLen uint32) error {
	if b == nil {
		return errors.New("the builder object is not initialized (is nil)")
	}
	if !b.isFirstShare {
		return errors.New("not the first share")
	}
	sequenceLenBuf := make([]byte, SequenceLenBytes)
	binary.BigEndian.PutUint32(sequenceLenBuf, sequenceLen)

	for i := 0; i < SequenceLenBytes; i++ {
		b.rawShareData[NamespaceSize+ShareInfoBytes+i] = sequenceLenBuf[i]
	}

	return nil
}

// WriteSigner writes the signer's information to the share.
func (b *builder) WriteSigner(signer []byte) {
	// write the signer if it is the first share and the share version is 1 or 2
	if b == nil || !b.isFirstShare || (b.shareVersion != ShareVersionOne && b.shareVersion != ShareVersionTwo) {
		return
	}
	// NOTE: we don't check whether previous data has already been expected
	// like the sequence length (we just assume it has)
	b.rawShareData = append(b.rawShareData, signer...)
}

// WriteFibreBlobVersion writes the Fibre blob version (uint32) to the share.
// This is only used for share version 2.
func (b *builder) WriteFibreBlobVersion(fibreBlobVersion uint32) {
	if b == nil || !b.isFirstShare || b.shareVersion != ShareVersionTwo {
		return
	}
	fibreBlobVersionBuf := make([]byte, FibreBlobVersionSize)
	binary.BigEndian.PutUint32(fibreBlobVersionBuf, fibreBlobVersion)
	b.rawShareData = append(b.rawShareData, fibreBlobVersionBuf...)
}

// WriteCommitment writes the commitment to the share.
// This is only used for share version 2.
func (b *builder) WriteCommitment(commitment []byte) {
	if b == nil || !b.isFirstShare || b.shareVersion != ShareVersionTwo {
		return
	}
	if len(commitment) != FibreCommitmentSize {
		return
	}
	b.rawShareData = append(b.rawShareData, commitment...)
}

// FlipSequenceStart flips the sequence start indicator of the share provided
func (b *builder) FlipSequenceStart() {
	infoByteIndex := b.indexOfInfoBytes()

	// the sequence start indicator is the last bit of the info byte so flip the
	// last bit
	b.rawShareData[infoByteIndex] ^= 0x01
}

func (b *builder) prepareCompactShare() error {
	shareData := make([]byte, 0, ShareSize)
	infoByte, err := NewInfoByte(b.shareVersion, b.isFirstShare)
	if err != nil {
		return err
	}
	placeholderSequenceLen := make([]byte, SequenceLenBytes)
	placeholderReservedBytes := make([]byte, ShareReservedBytes)

	shareData = append(shareData, b.namespace.Bytes()...)
	shareData = append(shareData, byte(infoByte))

	if b.isFirstShare {
		shareData = append(shareData, placeholderSequenceLen...)
	}

	shareData = append(shareData, placeholderReservedBytes...)

	b.rawShareData = shareData

	return nil
}

func (b *builder) prepareSparseShare() error {
	shareData := make([]byte, 0, ShareSize)
	infoByte, err := NewInfoByte(b.shareVersion, b.isFirstShare)
	if err != nil {
		return err
	}
	placeholderSequenceLen := make([]byte, SequenceLenBytes)

	shareData = append(shareData, b.namespace.Bytes()...)
	shareData = append(shareData, byte(infoByte))

	if b.isFirstShare {
		shareData = append(shareData, placeholderSequenceLen...)
	}

	b.rawShareData = shareData
	return nil
}

func isCompactShare(ns Namespace) bool {
	return ns.IsTx() || ns.IsPayForBlob() || ns.IsPayForFibre()
}
