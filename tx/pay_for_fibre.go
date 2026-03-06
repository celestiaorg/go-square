package tx

import (
	"fmt"
	"strings"

	"github.com/celestiaorg/go-square/v4/share"
	"google.golang.org/protobuf/encoding/protowire"
)

// MsgPayForFibreTypeURL is the Cosmos SDK message type URL for MsgPayForFibre.
const MsgPayForFibreTypeURL = "/celestia.fibre.v1.MsgPayForFibre"

// SynthesizeFibreTx attempts to detect a MsgPayForFibre message inside plain
// Cosmos SDK Tx bytes and synthesize the corresponding FibreTx.
//
// Returns (fibreTx, true, nil) when the tx contains MsgPayForFibre.
// Returns (nil, false, nil) when the tx does not contain MsgPayForFibre.
// Returns (nil, true, err) when the tx contains MsgPayForFibre but parsing fails.
func SynthesizeFibreTx(txBytes []byte) (*FibreTx, bool, error) {
	fields, err := extractMsgPayForFibreFields(txBytes)
	if err != nil {
		return nil, false, err
	}
	if fields == nil {
		return nil, false, nil
	}

	ns, err := share.NewNamespaceFromBytes(fields.namespace)
	if err != nil {
		return nil, true, fmt.Errorf("invalid namespace in MsgPayForFibre: %w", err)
	}

	signerBytes, err := decodeBech32Address(fields.signer)
	if err != nil {
		return nil, true, fmt.Errorf("decoding signer address in MsgPayForFibre: %w", err)
	}

	systemBlob, err := share.NewV2Blob(ns, fields.blobVersion, fields.commitment, signerBytes)
	if err != nil {
		return nil, true, fmt.Errorf("creating system blob for MsgPayForFibre: %w", err)
	}

	return &FibreTx{
		Tx:         txBytes,
		SystemBlob: systemBlob,
	}, true, nil
}

type msgPayForFibreFields struct {
	namespace   []byte
	blobVersion uint32
	commitment  []byte
	signer      string
}

// extractMsgPayForFibreFields parses the Cosmos SDK Tx proto wire format to
// extract fields from the first MsgPayForFibre message found.
//
// Cosmos SDK Tx proto layout (field numbers):
//
//	Tx:
//	  body (1, LEN) → TxBody:
//	    messages (1, LEN, repeated) → Any:
//	      type_url (1, LEN)
//	      value    (2, LEN) → MsgPayForFibre:
//	        signer           (1, LEN)
//	        payment_promise  (2, LEN) → PaymentPromise:
//	          namespace        (3, LEN)
//	          blob_version     (5, VARINT)
//	          commitment       (6, LEN)
func extractMsgPayForFibreFields(txBytes []byte) (*msgPayForFibreFields, error) {
	bodyBytes, ok := consumeField(txBytes, 1, protowire.BytesType)
	if !ok {
		return nil, nil
	}

	anyBytes, ok := consumeField(bodyBytes, 1, protowire.BytesType)
	if !ok {
		return nil, nil
	}

	typeURLBytes, ok := consumeField(anyBytes, 1, protowire.BytesType)
	if !ok {
		return nil, nil
	}
	if string(typeURLBytes) != MsgPayForFibreTypeURL {
		return nil, nil
	}

	valueBytes, ok := consumeField(anyBytes, 2, protowire.BytesType)
	if !ok {
		return nil, nil
	}

	signerBytes, _ := consumeField(valueBytes, 1, protowire.BytesType)

	promiseBytes, ok := consumeField(valueBytes, 2, protowire.BytesType)
	if !ok {
		return nil, fmt.Errorf("MsgPayForFibre is missing payment_promise field")
	}

	namespace, _ := consumeField(promiseBytes, 3, protowire.BytesType)
	blobVersion, _ := consumeVarint(promiseBytes, 5)
	commitment, _ := consumeField(promiseBytes, 6, protowire.BytesType)

	return &msgPayForFibreFields{
		namespace:   namespace,
		blobVersion: uint32(blobVersion),
		commitment:  commitment,
		signer:      string(signerBytes),
	}, nil
}

// consumeField returns the value bytes of the first field with the given
// number and type. It returns (nil, false) if the field is not found or the
// input is malformed.
func consumeField(b []byte, fieldNum protowire.Number, wantType protowire.Type) ([]byte, bool) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, false
		}
		b = b[n:]

		if num == fieldNum && typ == wantType {
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, false
			}
			return v, true
		}

		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return nil, false
		}
		b = b[n:]
	}
	return nil, false
}

// consumeVarint returns the value of the first varint field with the given
// number. It returns (0, false) if the field is not found or the input is
// malformed.
func consumeVarint(b []byte, fieldNum protowire.Number) (uint64, bool) {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return 0, false
		}
		b = b[n:]

		if num == fieldNum && typ == protowire.VarintType {
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return 0, false
			}
			return v, true
		}

		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return 0, false
		}
		b = b[n:]
	}
	return 0, false
}

// decodeBech32Address decodes a bech32 address string (e.g. "celestia1...") and
// returns the raw 20-byte address. The human-readable prefix is not validated.
func decodeBech32Address(addr string) ([]byte, error) {
	lower := strings.ToLower(addr)
	sep := strings.LastIndexByte(lower, '1')
	if sep < 1 {
		return nil, fmt.Errorf("invalid bech32 address: missing separator in %q", addr)
	}

	encoded := lower[sep+1:]
	if len(encoded) < 7 {
		return nil, fmt.Errorf("invalid bech32 address: too short")
	}

	// Decode base32 characters (data section only, strip 6-char checksum).
	dataPart := encoded[:len(encoded)-6]
	decoded := make([]byte, 0, len(dataPart))
	for _, c := range dataPart {
		idx, ok := bech32CharsetRev[c]
		if !ok {
			return nil, fmt.Errorf("invalid bech32 character %q", c)
		}
		decoded = append(decoded, idx)
	}

	return convertBits(decoded, 5, 8, false)
}

// bech32Charset is the bech32 alphabet defined in BIP-173.
const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// bech32CharsetRev maps each bech32 character to its 5-bit value.
var bech32CharsetRev = func() map[rune]byte {
	m := make(map[rune]byte, len(bech32Charset))
	for i, c := range bech32Charset {
		m[c] = byte(i)
	}
	return m
}()

// convertBits performs the bit-group conversion described in BIP-173.
func convertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	acc := 0
	bits := uint(0)
	var result []byte
	maxv := (1 << toBits) - 1
	for _, v := range data {
		acc = (acc << fromBits) | int(v)
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			result = append(result, byte((acc>>bits)&maxv))
		}
	}
	if pad {
		if bits > 0 {
			result = append(result, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, fmt.Errorf("invalid padding in bech32 decoding")
	}
	return result, nil
}
