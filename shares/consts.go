package shares

import "github.com/celestiaorg/go-square/namespace"

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
	FirstCompactShareContentSize = ShareSize - namespace.NamespaceSize - ShareInfoBytes - SequenceLenBytes - ShareReservedBytes

	// ContinuationCompactShareContentSize is the number of bytes usable for
	// data in a continuation compact share of a sequence.
	ContinuationCompactShareContentSize = ShareSize - namespace.NamespaceSize - ShareInfoBytes - ShareReservedBytes

	// FirstSparseShareContentSize is the number of bytes usable for data in the
	// first sparse share of a sequence.
	FirstSparseShareContentSize = ShareSize - namespace.NamespaceSize - ShareInfoBytes - SequenceLenBytes

	// ContinuationSparseShareContentSize is the number of bytes usable for data
	// in a continuation sparse share of a sequence.
	ContinuationSparseShareContentSize = ShareSize - namespace.NamespaceSize - ShareInfoBytes

	// MinSquareSize is the smallest original square width.
	MinSquareSize = 1

	// MinShareCount is the minimum number of shares allowed in the original
	// data square.
	MinShareCount = MinSquareSize * MinSquareSize

	// MaxShareVersion is the maximum value a share version can be.
	MaxShareVersion = 127

	// ProtoBlobTxTypeID is included in each encoded BlobTx to help prevent
	// decoding binaries that are not actually BlobTxs.
	ProtoBlobTxTypeID = "BLOB"

	// ProtoIndexWrapperTypeID is included in each encoded IndexWrapper to help prevent
	// decoding binaries that are not actually IndexWrappers.
	ProtoIndexWrapperTypeID = "INDX"

	// SignerSize is the size of the signer in bytes.
	SignerSize = 20
)

// SupportedShareVersions is a list of supported share versions.
var SupportedShareVersions = []uint8{ShareVersionZero, ShareVersionOne}
