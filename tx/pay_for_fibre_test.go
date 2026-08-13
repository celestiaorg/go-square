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
	"google.golang.org/protobuf/encoding/protowire"
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
				sdkTx := &cosmostx.Tx{
					Body: &cosmostx.TxBody{
						Messages: []*anypb.Any{
							{
								TypeUrl: tx.MsgPayForFibreTypeURL,
								Value:   msgBytes,
							},
						},
					},
				}
				txBytes, err := proto.Marshal(sdkTx)
				require.NoError(t, err)
				return txBytes
			}(),
			wantNil: true,
			wantErr: true,
		},
		{
			name: "plain SDK tx with different message type",
			txBytes: func() []byte {
				sdkTx := &cosmostx.Tx{
					Body: &cosmostx.TxBody{
						Messages: []*anypb.Any{
							{
								TypeUrl: "/cosmos.bank.v1beta1.MsgSend",
								Value:   []byte("some-value"),
							},
						},
					},
				}
				txBytes, err := proto.Marshal(sdkTx)
				require.NoError(t, err)
				return txBytes
			}(),
			wantNil: true,
			wantErr: false,
		},
		{
			name: "SDK tx with empty body",
			txBytes: func() []byte {
				sdkTx := &cosmostx.Tx{
					Body: &cosmostx.TxBody{},
				}
				txBytes, err := proto.Marshal(sdkTx)
				require.NoError(t, err)
				return txBytes
			}(),
			wantNil: true,
			wantErr: false,
		},
		{
			name: "SDK tx with nil body",
			txBytes: func() []byte {
				sdkTx := &cosmostx.Tx{}
				txBytes, err := proto.Marshal(sdkTx)
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
			txBytes: func() []byte {
				sdkTx := &cosmostx.Tx{
					Body: &cosmostx.TxBody{
						Messages: []*anypb.Any{
							{
								TypeUrl: tx.MsgPayForFibreTypeURL,
								Value:   []byte{0xFF, 0xFF, 0xFF},
							},
						},
					},
				}
				txBytes, err := proto.Marshal(sdkTx)
				require.NoError(t, err)
				return txBytes
			}(),
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
				sdkTx := &cosmostx.Tx{
					Body: &cosmostx.TxBody{
						Messages: []*anypb.Any{
							{
								TypeUrl: tx.MsgPayForFibreTypeURL,
								Value:   msgBytes,
							},
						},
					},
				}
				txBytes, err := proto.Marshal(sdkTx)
				require.NoError(t, err)
				return txBytes
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

// appendBodyField appends a top-level field 1 (body) carrying body to txBytes.
func appendBodyField(txBytes, body []byte) []byte {
	out := append([]byte(nil), txBytes...)
	out = protowire.AppendTag(out, 1, protowire.BytesType)
	return protowire.AppendBytes(out, body)
}

// bodyOf returns the single top-level field 1 value of a marshalled Tx.
func bodyOf(t *testing.T, txBytes []byte) []byte {
	t.Helper()
	num, typ, n := protowire.ConsumeTag(txBytes)
	require.GreaterOrEqual(t, n, 0)
	require.Equal(t, protowire.Number(1), num)
	require.Equal(t, protowire.BytesType, typ)
	body, m := protowire.ConsumeBytes(txBytes[n:])
	require.GreaterOrEqual(t, m, 0)
	require.Len(t, txBytes, n+m, "expected exactly one top-level field")
	return body
}

// TestTryParseFibreTxDuplicateBody pins the semantics of a Tx carrying more than
// one top-level body field. protobuf-go merges repeated occurrences of a
// singular message field, whereas the Cosmos SDK reads the same wire field as a
// scalar `bytes` (TxRaw.body_bytes) and keeps only the last occurrence. The two
// must agree on which body is authoritative, so TryParseFibreTx follows the
// SDK's last-one-wins rule.
func TestTryParseFibreTxDuplicateBody(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	signerBytes := bytes.Repeat([]byte{0xAB}, share.SignerSize)
	signer, err := test.EncodeBech32("celestia", signerBytes)
	require.NoError(t, err)

	fibreTxBytes, err := test.BuildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 1)
	require.NoError(t, err)
	fibreBody := bodyOf(t, fibreTxBytes)

	normalTxBytes, err := proto.Marshal(&cosmostx.Tx{
		Body: &cosmostx.TxBody{
			Messages: []*anypb.Any{{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte("v")}},
		},
	})
	require.NoError(t, err)
	normalBody := bodyOf(t, normalTxBytes)

	tests := []struct {
		name string
		// bodies are appended in order; the last one is authoritative.
		bodies    [][]byte
		wantFibre bool
	}{
		{
			name:      "fibre body then normal body is not a fibre tx",
			bodies:    [][]byte{fibreBody, normalBody},
			wantFibre: false,
		},
		{
			name:      "normal body then fibre body is a fibre tx",
			bodies:    [][]byte{normalBody, fibreBody},
			wantFibre: true,
		},
		{
			name:      "trailing fibre body wins over several normal bodies",
			bodies:    [][]byte{normalBody, normalBody, fibreBody},
			wantFibre: true,
		},
		{
			name:      "trailing normal body wins over several fibre bodies",
			bodies:    [][]byte{fibreBody, fibreBody, normalBody},
			wantFibre: false,
		},
		{
			// protobuf-go cannot merge a body that is not valid wire format, so
			// these bytes are rejected before the last-occurrence rule is reached.
			// The SDK's decoder rejects them too, since body_bytes must parse as a
			// TxBody, so the two still agree on this input.
			name:      "trailing body that is not a valid TxBody is not a fibre tx",
			bodies:    [][]byte{fibreBody, {0xFF}},
			wantFibre: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var txBytes []byte
			for _, body := range tc.bodies {
				txBytes = appendBodyField(txBytes, body)
			}

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
