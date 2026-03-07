package tx

import (
	"fmt"

	fibrev1 "github.com/celestiaorg/go-square/v4/proto/celestia/fibre/v1"
	cosmostx "github.com/celestiaorg/go-square/v4/proto/cosmos/tx/v1beta1"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/cosmos/btcutil/bech32"
	"google.golang.org/protobuf/proto"
)

// MsgPayForFibreTypeURL is the Cosmos SDK message type URL for MsgPayForFibre.
const MsgPayForFibreTypeURL = "/celestia.fibre.v1.MsgPayForFibre"

// TryParseFibreTx attempts to detect a MsgPayForFibre message inside plain
// Cosmos SDK Tx bytes and synthesize the corresponding FibreTx.
func TryParseFibreTx(txBytes []byte) (*FibreTx, bool, error) {
	var sdkTx cosmostx.Tx
	if err := proto.Unmarshal(txBytes, &sdkTx); err != nil {
		return nil, false, nil
	}
	if sdkTx.Body == nil || len(sdkTx.Body.Messages) == 0 {
		return nil, false, nil
	}

	anyMsg := sdkTx.Body.Messages[0]
	if anyMsg.TypeUrl != MsgPayForFibreTypeURL {
		return nil, false, nil
	}

	var msg fibrev1.MsgPayForFibre
	if err := proto.Unmarshal(anyMsg.Value, &msg); err != nil {
		return nil, true, fmt.Errorf("unmarshalling MsgPayForFibre: %w", err)
	}

	if msg.PaymentPromise == nil {
		return nil, true, fmt.Errorf("MsgPayForFibre is missing payment_promise field")
	}

	ns, err := share.NewNamespaceFromBytes(msg.PaymentPromise.Namespace)
	if err != nil {
		return nil, true, fmt.Errorf("invalid namespace in MsgPayForFibre: %w", err)
	}

	signerBytes, err := decodeBech32Address(msg.Signer)
	if err != nil {
		return nil, true, fmt.Errorf("decoding signer address in MsgPayForFibre: %w", err)
	}

	systemBlob, err := share.NewV2Blob(ns, msg.PaymentPromise.BlobVersion, msg.PaymentPromise.Commitment, signerBytes)
	if err != nil {
		return nil, true, fmt.Errorf("creating system blob for MsgPayForFibre: %w", err)
	}

	return &FibreTx{
		Tx:         txBytes,
		SystemBlob: systemBlob,
	}, true, nil
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
