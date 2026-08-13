package tx_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4/internal/test"
	fibrev1 "github.com/celestiaorg/go-square/v4/proto/celestia/fibre/v1"
	cosmostx "github.com/celestiaorg/go-square/v4/proto/cosmos/tx/v1beta1"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestTryParseFibreTx(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	signerBytes := bytes.Repeat([]byte{0xAB}, share.SignerSize)
	signer, err := test.EncodeBech32("celestia", signerBytes)
	require.NoError(t, err)

	tests := []struct {
		name    string
		txBytes []byte
		wantNil bool
		wantErr bool
	}{
		{
			name:    "random bytes",
			txBytes: []byte("not-a-cosmos-tx"),
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "empty bytes",
			txBytes: []byte{},
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "nil bytes",
			txBytes: nil,
			wantNil: true,
			wantErr: false,
		},
		{
			name: "valid MsgPayForFibre tx",
			txBytes: func() []byte {
				b, err := test.BuildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 1)
				require.NoError(t, err)
				return b
			}(),
			wantNil: false,
			wantErr: false,
		},
		{
			name: "MsgPayForFibre with nil payment promise",
			txBytes: func() []byte {
				msg := &fibrev1.MsgPayForFibre{
					Signer: signer,
				}
				msgBytes, err := proto.Marshal(msg)
				require.NoError(t, err)
				return marshalTxWithBodies(t, &cosmostx.TxBody{
					Messages: []*anypb.Any{
						{
							TypeUrl: tx.MsgPayForFibreTypeURL,
							Value:   msgBytes,
						},
					},
				})
			}(),
			wantNil: true,
			wantErr: true,
		},
		{
			name: "plain SDK tx with different message type",
			txBytes: marshalTxWithBodies(t, &cosmostx.TxBody{
				Messages: []*anypb.Any{
					{
						TypeUrl: "/cosmos.bank.v1beta1.MsgSend",
						Value:   []byte("some-value"),
					},
				},
			}),
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "SDK tx with empty body",
			txBytes: marshalTxWithBodies(t, &cosmostx.TxBody{}),
			wantNil: true,
			wantErr: false,
		},
		{
			name: "SDK tx with no body field",
			txBytes: func() []byte {
				txBytes, err := proto.Marshal(&cosmostx.TxRaw{})
				require.NoError(t, err)
				return txBytes
			}(),
			wantNil: true,
			wantErr: false,
		},
		{
			name: "BlobTx bytes",
			txBytes: func() []byte {
				b, err := test.GenerateBlobTx([]int{256})
				require.NoError(t, err)
				return b
			}(),
			wantNil: true,
			wantErr: false,
		},
		{
			name: "MsgPayForFibre with corrupted inner message",
			txBytes: marshalTxWithBodies(t, &cosmostx.TxBody{
				Messages: []*anypb.Any{
					{
						TypeUrl: tx.MsgPayForFibreTypeURL,
						Value:   []byte{0xFF, 0xFF, 0xFF},
					},
				},
			}),
			wantNil: true,
			wantErr: true,
		},
		{
			name: "MsgPayForFibre with invalid signer address",
			txBytes: func() []byte {
				msg := &fibrev1.MsgPayForFibre{
					Signer: "not-a-bech32-address",
					PaymentPromise: &fibrev1.PaymentPromise{
						Namespace:   ns.Bytes(),
						BlobVersion: 1,
						Commitment:  commitment,
					},
				}
				msgBytes, err := proto.Marshal(msg)
				require.NoError(t, err)
				return marshalTxWithBodies(t, &cosmostx.TxBody{
					Messages: []*anypb.Any{
						{
							TypeUrl: tx.MsgPayForFibreTypeURL,
							Value:   msgBytes,
						},
					},
				})
			}(),
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fibreTx, err := tx.TryParseFibreTx(tc.txBytes)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, fibreTx)
				return
			}
			require.NoError(t, err)
			if tc.wantNil {
				require.Nil(t, fibreTx)
				return
			}
			require.NotNil(t, fibreTx)
			require.Equal(t, tc.txBytes, fibreTx.Tx)
			require.Equal(t, ns, fibreTx.SystemBlob.Namespace())
			require.Equal(t, signerBytes, fibreTx.SystemBlob.Signer())
			require.Equal(t, share.ShareVersionTwo, fibreTx.SystemBlob.ShareVersion())
		})
	}
}

// TestTryParseFibreTxMatchesManualConstruction verifies that TryParseFibreTx
// produces a FibreTx whose system blob matches one constructed manually from the
// same namespace, blobVersion, commitment, and signer bytes.
func TestTryParseFibreTxMatchesManualConstruction(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, share.NamespaceVersionZeroIDSize))
	commitment := bytes.Repeat([]byte{0xCC}, share.FibreCommitmentSize)
	signerBytes := bytes.Repeat([]byte{0x12}, share.SignerSize)
	signer, err := test.EncodeBech32("celestia", signerBytes)
	require.NoError(t, err)

	txBytes, err := test.BuildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 2)
	require.NoError(t, err)

	fibreTx, err := tx.TryParseFibreTx(txBytes)
	require.NoError(t, err)
	require.NotNil(t, fibreTx)

	expected, err := share.NewV2Blob(ns, 2, commitment, signerBytes)
	require.NoError(t, err)

	require.Equal(t, expected.Namespace(), fibreTx.SystemBlob.Namespace())
	require.Equal(t, expected.Data(), fibreTx.SystemBlob.Data())
	require.Equal(t, expected.ShareVersion(), fibreTx.SystemBlob.ShareVersion())
	require.Equal(t, expected.Signer(), fibreTx.SystemBlob.Signer())
}

// marshalTxWithBodies encodes a transaction carrying one top-level body field
// per entry in bodies. Concatenating separately marshalled transactions is
// exactly how a repeated occurrence of the field is encoded on the wire.
func marshalTxWithBodies(t *testing.T, bodies ...*cosmostx.TxBody) []byte {
	t.Helper()
	out := make([]byte, 0, len(bodies))
	for _, body := range bodies {
		bodyBytes, err := proto.Marshal(body)
		require.NoError(t, err)
		txBytes, err := proto.Marshal(&cosmostx.TxRaw{BodyBytes: bodyBytes})
		require.NoError(t, err)
		out = append(out, txBytes...)
	}
	return out
}

// TestTryParseFibreTxDuplicateBody pins which body wins when a transaction
// carries the body field more than once. The Cosmos SDK reads that wire field as
// TxRaw.body_bytes, a scalar `bytes`, so it keeps the last occurrence. A
// classifier that disagrees with the SDK about a transaction they both see is a
// consensus hazard, so this package must reach the same answer.
func TestTryParseFibreTxDuplicateBody(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	signerBytes := bytes.Repeat([]byte{0xAB}, share.SignerSize)
	signer, err := test.EncodeBech32("celestia", signerBytes)
	require.NoError(t, err)

	msg := &fibrev1.MsgPayForFibre{
		Signer: signer,
		PaymentPromise: &fibrev1.PaymentPromise{
			Namespace:   ns.Bytes(),
			BlobVersion: 1,
			Commitment:  commitment,
		},
	}
	msgBytes, err := proto.Marshal(msg)
	require.NoError(t, err)

	fibreBody := &cosmostx.TxBody{
		Messages: []*anypb.Any{{TypeUrl: tx.MsgPayForFibreTypeURL, Value: msgBytes}},
	}
	normalBody := &cosmostx.TxBody{
		Messages: []*anypb.Any{{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte("v")}},
	}

	tests := []struct {
		name string
		// bodies are encoded in order; the last one is authoritative.
		bodies    []*cosmostx.TxBody
		wantFibre bool
	}{
		{
			name:      "fibre body followed by normal body is not a fibre tx",
			bodies:    []*cosmostx.TxBody{fibreBody, normalBody},
			wantFibre: false,
		},
		{
			name:      "normal body followed by fibre body is a fibre tx",
			bodies:    []*cosmostx.TxBody{normalBody, fibreBody},
			wantFibre: true,
		},
		{
			name:      "duplicated fibre body is a fibre tx",
			bodies:    []*cosmostx.TxBody{fibreBody, fibreBody},
			wantFibre: true,
		},
		{
			name:      "duplicated normal body is not a fibre tx",
			bodies:    []*cosmostx.TxBody{normalBody, normalBody},
			wantFibre: false,
		},
		{
			name:      "three bodies resolve to the last",
			bodies:    []*cosmostx.TxBody{fibreBody, fibreBody, normalBody},
			wantFibre: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			txBytes := marshalTxWithBodies(t, tc.bodies...)

			fibreTx, err := tx.TryParseFibreTx(txBytes)
			require.NoError(t, err)
			if !tc.wantFibre {
				require.Nil(t, fibreTx)
				return
			}
			require.NotNil(t, fibreTx)
			require.Equal(t, txBytes, fibreTx.Tx)
			require.Equal(t, ns, fibreTx.SystemBlob.Namespace())
			require.Equal(t, signerBytes, fibreTx.SystemBlob.Signer())
		})
	}
}
