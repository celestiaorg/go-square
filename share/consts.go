package share

import (
	"bytes"
	"math"
)

const (
	// ShareSize is the size of a share in bytes.
	ShareSize = 512

	// ShareInfoBytes is the number of bytes reserved for information. The info
	// byte contains the share version and a sequence start idicator.
	ShareInfoBytes = 1

	// SequenceLenBytes is the number of bytes reserved for the sequence length
	// that is present in the first share of a sequence.
	SequenceLenBytes = 4

	// ShareVersionZero is the first share version format.
	ShareVersionZero = uint8(0)

	// ShareVersionOne is the second share version format.
	// It requires that a signer is included in the first share in the sequence.
	ShareVersionOne = uint8(1)

	// DefaultShareVersion is the defacto share version. Use this if you are
	// unsure of which version to use.
	DefaultShareVersion = ShareVersionZero

	// CompactShareReservedBytes is the number of bytes reserved for the location of
	// the first unit (transaction, ISR) in a compact share.
	// Deprecated: use ShareReservedBytes instead.
	CompactShareReservedBytes = ShareReservedBytes

	// ShareReservedBytes is the number of bytes reserved for the location of
	// the first unit (transaction, ISR) in a compact share.
	ShareReservedBytes = 4

	// FirstCompactShareContentSize is the number of bytes usable for data in
	// the first compact share of a sequence.
	FirstCompactShareContentSize = ShareSize - NamespaceSize - ShareInfoBytes - SequenceLenBytes - ShareReservedBytes

	// ContinuationCompactShareContentSize is the number of bytes usable for
	// data in a continuation compact share of a sequence.
	ContinuationCompactShareContentSize = ShareSize - NamespaceSize - ShareInfoBytes - ShareReservedBytes

	// FirstSparseShareContentSize is the number of bytes usable for data in the
	// first sparse share of a sequence.
	FirstSparseShareContentSize = ShareSize - NamespaceSize - ShareInfoBytes - SequenceLenBytes

	// FirstSparseShareContentSizeWithSigner is the number of bytes usable for data in the
	// first sparse share of a sequence if it contains a signer (a.k.a authored blob).
	FirstSparseShareContentSizeWithSigner = ShareSize - NamespaceSize - ShareInfoBytes - SequenceLenBytes - SignerSize

	// ContinuationSparseShareContentSize is the number of bytes usable for data
	// in a continuation sparse share of a sequence.
	ContinuationSparseShareContentSize = ShareSize - NamespaceSize - ShareInfoBytes

	// MinSquareSize is the smallest original square width.
	MinSquareSize = 1

	// MinShareCount is the minimum number of shares allowed in the original
	// data square.
	MinShareCount = MinSquareSize * MinSquareSize

	// MaxShareVersion is the maximum value a share version can be.
	MaxShareVersion = 127

	// SignerSize is the size of the signer in bytes.
	SignerSize = 20
)

// SupportedShareVersions is a list of supported share versions.
var SupportedShareVersions = []uint8{ShareVersionZero, ShareVersionOne}

const (
	// NamespaceVersionSize is the size of a namespace version in bytes.
	NamespaceVersionSize = 1

	// VersionIndex is the index of the version in the namespace. This should
	// always be the first byte
	VersionIndex = 0

	// NamespaceIDSize is the size of a namespace ID in bytes.
	NamespaceIDSize = 28

	// NamespaceSize is the size of a namespace (version + ID) in bytes.
	NamespaceSize = NamespaceVersionSize + NamespaceIDSize

	// NamespaceVersionZero is the first namespace version.
	NamespaceVersionZero = uint8(0)

	// NamespaceVersionMax is the max namespace version.
	NamespaceVersionMax = math.MaxUint8

	// NamespaceVersionZeroPrefixSize is the number of `0` bytes that are prefixed to
	// namespace IDs for version 0.
	NamespaceVersionZeroPrefixSize = 18

	// NamespaceVersionZeroIDSize is the number of bytes available for
	// user-specified namespace ID in a namespace ID for version 0.
	NamespaceVersionZeroIDSize = NamespaceIDSize - NamespaceVersionZeroPrefixSize
)

var (
	// NamespaceVersionZeroPrefix is the prefix of a namespace ID for version 0.
	NamespaceVersionZeroPrefix = bytes.Repeat([]byte{0}, NamespaceVersionZeroPrefixSize)

	// TxNamespace is the namespace reserved for ordinary Cosmos SDK transactions.
	TxNamespace = primaryReservedNamespace(0x01)

	// IntermediateStateRootsNamespace is the namespace reserved for
	// intermediate state root data.
	IntermediateStateRootsNamespace = primaryReservedNamespace(0x02)

	// PayForBlobNamespace is the namespace reserved for PayForBlobs transactions.
	PayForBlobNamespace = primaryReservedNamespace(0x04)

	// PrimaryReservedPaddingNamespace is the namespace used for padding after all
	// primary reserved namespaces.
	PrimaryReservedPaddingNamespace = primaryReservedNamespace(0xFF)

	// MaxPrimaryReservedNamespace is the highest primary reserved namespace.
	// Namespaces lower than this are reserved for protocol use.
	MaxPrimaryReservedNamespace = primaryReservedNamespace(0xFF)

	// MinSecondaryReservedNamespace is the lowest secondary reserved namespace
	// reserved for protocol use. Namespaces higher than this are reserved for
	// protocol use.
	MinSecondaryReservedNamespace = secondaryReservedNamespace(0x00)

	// TailPaddingNamespace is the namespace reserved for tail padding. All data
	// with this namespace will be ignored.
	TailPaddingNamespace = secondaryReservedNamespace(0xFE)

	// ParitySharesNamespace is the namespace reserved for erasure coded data.
	ParitySharesNamespace = secondaryReservedNamespace(0xFF)

	// SupportedBlobNamespaceVersions is a list of namespace versions that can be specified by a user for blobs.
	SupportedBlobNamespaceVersions = []uint8{NamespaceVersionZero}
)

func primaryReservedNamespace(lastByte byte) Namespace {
	return newNamespace(NamespaceVersionZero, append(bytes.Repeat([]byte{0x00}, NamespaceIDSize-1), lastByte))
}

func secondaryReservedNamespace(lastByte byte) Namespace {
	return newNamespace(NamespaceVersionMax, append(bytes.Repeat([]byte{0xFF}, NamespaceIDSize-1), lastByte))
}
