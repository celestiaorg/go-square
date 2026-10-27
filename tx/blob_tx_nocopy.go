package tx

import (
	"math"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"google.golang.org/protobuf/encoding/protowire"
)

// Single byte tags of the BlobTx and BlobProto fields, in the order
// proto.Marshal writes them.
const (
	tagBlobTxTx     = byte(1<<3 | protowire.BytesType)
	tagBlobTxBlobs  = byte(2<<3 | protowire.BytesType)
	tagBlobTxTypeID = byte(3<<3 | protowire.BytesType)

	tagBlobNamespaceID      = byte(1<<3 | protowire.BytesType)
	tagBlobData             = byte(2<<3 | protowire.BytesType)
	tagBlobShareVersion     = byte(3<<3 | protowire.VarintType)
	tagBlobNamespaceVersion = byte(4<<3 | protowire.VarintType)
	tagBlobSigner           = byte(5<<3 | protowire.BytesType)
)

// canonicalTypeIDField is the type_id field exactly as proto.Marshal writes it.
var canonicalTypeIDField = string([]byte{tagBlobTxTypeID, byte(len(ProtoBlobTxTypeID))}) + ProtoBlobTxTypeID

// UnmarshalBlobTxNoCopy returns the same result as UnmarshalBlobTx for every
// input, but for a valid BlobTx it walks the input once instead of decoding,
// re-marshalling and comparing it, and it does not copy the tx or the blob
// data.
//
// The returned BlobTx's Tx and each blob's Data and Signer alias tx: the
// caller must not modify tx while the result is in use.
func UnmarshalBlobTxNoCopy(tx []byte) (*BlobTx, bool, error) {
	if blobTx, ok := decodeCanonicalBlobTx(tx); ok {
		return blobTx, true, nil
	}
	// tx is not a valid BlobTx. UnmarshalBlobTx gives the verdict and the
	// error, at the cost it had before this fast path existed.
	return UnmarshalBlobTx(tx)
}

// decodeCanonicalBlobTx reports whether b is a valid BlobTx in exactly the
// encoding proto.Marshal produces, which is what UnmarshalBlobTx accepts. It
// returns false for anything else, without saying why.
func decodeCanonicalBlobTx(b []byte) (*BlobTx, bool) {
	var (
		blobTx BlobTx
		ok     bool
	)
	// proto.Marshal writes fields in field number order and omits empty
	// bytes fields.
	if len(b) > 0 && b[0] == tagBlobTxTx {
		blobTx.Tx, b, ok = consumeCanonicalBytes(b[1:])
		if !ok {
			return nil, false
		}
	}
	for len(b) > 0 && b[0] == tagBlobTxBlobs {
		var msg []byte
		msg, b, ok = consumeCanonicalLen(b[1:])
		if !ok {
			return nil, false
		}
		blob, ok := decodeCanonicalBlob(msg)
		if !ok {
			return nil, false
		}
		blobTx.Blobs = append(blobTx.Blobs, blob)
	}
	if string(b) != canonicalTypeIDField || len(blobTx.Blobs) == 0 || hasBlobTxTypeID(blobTx.Tx) {
		return nil, false
	}
	return &blobTx, true
}

// decodeCanonicalBlob decodes a BlobProto in exactly the encoding
// proto.Marshal produces and validates it as UnmarshalBlobTx does.
func decodeCanonicalBlob(b []byte) (*share.Blob, bool) {
	var (
		pb v4.BlobProto
		ok bool
	)
	if len(b) > 0 && b[0] == tagBlobNamespaceID {
		if pb.NamespaceId, b, ok = consumeCanonicalBytes(b[1:]); !ok {
			return nil, false
		}
	}
	if len(b) > 0 && b[0] == tagBlobData {
		if pb.Data, b, ok = consumeCanonicalBytes(b[1:]); !ok {
			return nil, false
		}
	}
	if len(b) > 0 && b[0] == tagBlobShareVersion {
		if pb.ShareVersion, b, ok = consumeCanonicalUint32(b[1:]); !ok {
			return nil, false
		}
	}
	if len(b) > 0 && b[0] == tagBlobNamespaceVersion {
		if pb.NamespaceVersion, b, ok = consumeCanonicalUint32(b[1:]); !ok {
			return nil, false
		}
	}
	if len(b) > 0 && b[0] == tagBlobSigner {
		if pb.Signer, b, ok = consumeCanonicalBytes(b[1:]); !ok {
			return nil, false
		}
	}
	if len(b) != 0 {
		return nil, false
	}
	blob, err := share.NewBlobFromProto(&pb)
	if err != nil {
		return nil, false
	}
	return blob, true
}

// consumeCanonicalVarint consumes a varint written in the fewest bytes, as
// proto.Marshal writes it.
func consumeCanonicalVarint(b []byte) (uint64, []byte, bool) {
	v, n := protowire.ConsumeVarint(b)
	if n < 0 || n != protowire.SizeVarint(v) {
		return 0, nil, false
	}
	return v, b[n:], true
}

// consumeCanonicalLen consumes a length-prefixed value, which may be empty.
func consumeCanonicalLen(b []byte) (v, rest []byte, ok bool) {
	l, b, ok := consumeCanonicalVarint(b)
	if !ok || l > uint64(len(b)) {
		return nil, nil, false
	}
	return b[:l:l], b[l:], true
}

// consumeCanonicalBytes consumes the value of a bytes field, which
// proto.Marshal only writes when it is not empty.
func consumeCanonicalBytes(b []byte) (v, rest []byte, ok bool) {
	v, rest, ok = consumeCanonicalLen(b)
	if !ok || len(v) == 0 {
		return nil, nil, false
	}
	return v, rest, true
}

// consumeCanonicalUint32 consumes the value of a uint32 field, which
// proto.Marshal only writes when it is not zero.
func consumeCanonicalUint32(b []byte) (uint32, []byte, bool) {
	v, rest, ok := consumeCanonicalVarint(b)
	if !ok || v == 0 || v > math.MaxUint32 {
		return 0, nil, false
	}
	return uint32(v), rest, true
}
