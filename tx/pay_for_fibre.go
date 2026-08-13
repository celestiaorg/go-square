package tx

import (
	"fmt"

	fibrev1 "github.com/celestiaorg/go-square/v4/proto/celestia/fibre/v1"
	cosmostx "github.com/celestiaorg/go-square/v4/proto/cosmos/tx/v1beta1"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/cosmos/btcutil/bech32"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// MsgPayForFibreTypeURL is the Cosmos SDK message type URL for MsgPayForFibre.
const MsgPayForFibreTypeURL = "/celestia.fibre.v1.MsgPayForFibre"

// TryParseFibreTx attempts to detect a MsgPayForFibre message inside plain
// Cosmos SDK Tx bytes and synthesize the corresponding FibreTx.
//
// Returns:
//   - (nil, nil): txBytes do not contain a MsgPayForFibre (not a fibre tx).
//   - (nil, err): txBytes contain a MsgPayForFibre but it is malformed.
//   - (ft, nil): successfully parsed and synthesized a FibreTx.
func TryParseFibreTx(txBytes []byte) (*FibreTx, error) {
	var sdkTx cosmostx.Tx
	// Not returning an error here because BlobTx bytes fail proto.Unmarshal
	// into cosmos.tx.v1beta1.Tx and callers pass BlobTx bytes through here.
	if err := proto.Unmarshal(txBytes, &sdkTx); err != nil {
		return nil, nil
	}
	body := resolveTxBody(txBytes, sdkTx.Body)
	if body == nil || len(body.Messages) == 0 {
		return nil, nil
	}

	anyMsg := body.Messages[0]
	if anyMsg.TypeUrl != MsgPayForFibreTypeURL {
		return nil, nil
	}

	var msg fibrev1.MsgPayForFibre
	if err := proto.Unmarshal(anyMsg.Value, &msg); err != nil {
		return nil, fmt.Errorf("unmarshalling MsgPayForFibre: %w", err)
	}

	if msg.PaymentPromise == nil {
		return nil, fmt.Errorf("MsgPayForFibre is missing payment_promise field")
	}

	ns, err := share.NewNamespaceFromBytes(msg.PaymentPromise.Namespace)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace in MsgPayForFibre: %w", err)
	}

	signerBytes, err := decodeBech32Address(msg.Signer)
	if err != nil {
		return nil, fmt.Errorf("decoding signer address in MsgPayForFibre: %w", err)
	}

	systemBlob, err := share.NewV2Blob(ns, msg.PaymentPromise.BlobVersion, msg.PaymentPromise.Commitment, signerBytes)
	if err != nil {
		return nil, fmt.Errorf("creating system blob for MsgPayForFibre: %w", err)
	}

	return &FibreTx{
		Tx:         txBytes,
		SystemBlob: systemBlob,
	}, nil
}

// resolveTxBody returns the transaction body that the Cosmos SDK's decoder reads
// out of txBytes, which is not always the body protobuf-go produces.
//
// The SDK decodes these bytes as TxRaw, whose field 1 (body_bytes) is a scalar
// bytes field, so gogoproto keeps only the last occurrence when field 1 is
// repeated. go-square models field 1 as the singular message Tx.body, and
// protobuf-go merges repeated occurrences of a message field, concatenating the
// inner repeated messages. On a repeated field 1 the two therefore disagree about
// which message comes first, and go-square must follow the SDK because the SDK is
// what validated and signed over the transaction.
//
// parsed is the already-merged body from proto.Unmarshal. Every transaction a
// standard signer produces encodes field 1 exactly once, in which case the two
// interpretations coincide and parsed is returned untouched. A repeated field 1
// whose last occurrence is not a valid TxBody yields nil: the SDK's decoder
// rejects those bytes outright, so they are not a fibre tx here either.
func resolveTxBody(txBytes []byte, parsed *cosmostx.TxBody) *cosmostx.TxBody {
	lastBody, count := lastTopLevelField1(txBytes)
	if count < 2 {
		return parsed
	}
	var body cosmostx.TxBody
	if err := proto.Unmarshal(lastBody, &body); err != nil {
		return nil
	}
	return &body
}

// lastTopLevelField1 returns the value of the last top-level wire field 1 in
// txBytes along with the number of times field 1 occurs. A malformed wire
// encoding stops the scan and returns what was found so far; txBytes has already
// been accepted by proto.Unmarshal before this is reached.
func lastTopLevelField1(txBytes []byte) (lastBody []byte, count int) {
	for len(txBytes) > 0 {
		num, typ, tagLen := protowire.ConsumeTag(txBytes)
		if tagLen < 0 {
			return lastBody, count
		}
		txBytes = txBytes[tagLen:]

		if num == 1 && typ == protowire.BytesType {
			value, valueLen := protowire.ConsumeBytes(txBytes)
			if valueLen < 0 {
				return lastBody, count
			}
			lastBody = value
			count++
			txBytes = txBytes[valueLen:]
			continue
		}

		valueLen := protowire.ConsumeFieldValue(num, typ, txBytes)
		if valueLen < 0 {
			return lastBody, count
		}
		txBytes = txBytes[valueLen:]
	}
	return lastBody, count
}

// decodeBech32Address decodes a bech32 address string (e.g. "celestia1...") and
// returns the raw address bytes.
func decodeBech32Address(addr string) ([]byte, error) {
	_, data, err := bech32.DecodeToBase256(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid bech32 address %q: %w", addr, err)
	}
	return data, nil
}
