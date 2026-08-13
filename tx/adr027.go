package tx

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// signaturesFieldNumber is the only TxRaw field that may legitimately appear
// more than once: a transaction carries one signature per signer.
const signaturesFieldNumber = protowire.Number(3)

// ValidateADR027TxRaw reports whether txBytes is a canonical ADR-027 encoding of
// a TxRaw. It is a port of the Cosmos SDK's rejectNonADR027TxRaw, which the SDK
// runs before decoding a transaction, and it checks that:
//   - every field uses the bytes wire type,
//   - field numbers are in ascending order,
//   - only the signatures field repeats,
//   - and length-prefix varints are as short as possible.
//
// This is deliberately not called by TryParseFibreTx or by square construction.
// Rejecting bytes that the proposing node accepted would turn a malformed
// transaction into a failure to build the square, which is no better than
// classifying it differently. Callers that decode transactions before they are
// proposed — an application's CheckTx, say — can use this to reject the
// ambiguous encodings outright, which is the durable place to handle them.
//
// The SDK's own version permits any field to repeat even though its doc comment
// describes the rule as "1, 2, and potentially multiple 3s". This enforces the
// documented rule.
func ValidateADR027TxRaw(txBytes []byte) error {
	prevTagNum := protowire.Number(0)

	for len(txBytes) > 0 {
		tagNum, wireType, n := protowire.ConsumeTag(txBytes)
		if n < 0 {
			return fmt.Errorf("invalid tag: %w", protowire.ParseError(n))
		}
		// Every field of TxRaw is a bytes field.
		if wireType != protowire.BytesType {
			return fmt.Errorf("expected wire type %d for field %d, got %d", protowire.BytesType, tagNum, wireType)
		}
		if tagNum < prevTagNum {
			return fmt.Errorf("txRaw must follow ADR-027, got field %d after field %d", tagNum, prevTagNum)
		}
		if tagNum == prevTagNum && tagNum != signaturesFieldNumber {
			return fmt.Errorf("txRaw must follow ADR-027, field %d appears more than once", tagNum)
		}
		prevTagNum = tagNum

		// Every field is length delimited, so the tag is followed by a varint
		// length prefix. Verify that prefix is minimally encoded; the field's
		// contents are validated by whoever decodes them.
		lengthPrefix, prefixLen := protowire.ConsumeVarint(txBytes[n:])
		if prefixLen < 0 {
			return fmt.Errorf("invalid length prefix for field %d: %w", tagNum, protowire.ParseError(prefixLen))
		}
		if minLen := varintMinLength(lengthPrefix); minLen != prefixLen {
			return fmt.Errorf("length prefix varint for field %d is not as short as possible, read %d bytes, only need %d", tagNum, prefixLen, minLen)
		}

		_, _, fieldLen := protowire.ConsumeField(txBytes)
		if fieldLen < 0 {
			return fmt.Errorf("invalid field %d: %w", tagNum, protowire.ParseError(fieldLen))
		}
		txBytes = txBytes[fieldLen:]
	}

	return nil
}

// varintMinLength returns the minimum number of bytes needed to varint-encode n.
func varintMinLength(n uint64) int {
	switch {
	case n < 1<<(7*1):
		return 1
	case n < 1<<(7*2):
		return 2
	case n < 1<<(7*3):
		return 3
	case n < 1<<(7*4):
		return 4
	case n < 1<<(7*5):
		return 5
	case n < 1<<(7*6):
		return 6
	case n < 1<<(7*7):
		return 7
	case n < 1<<(7*8):
		return 8
	case n < 1<<(7*9):
		return 9
	default:
		return 10
	}
}
