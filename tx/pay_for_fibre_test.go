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
	signer := test.EncodeBech32("celestia", signerBytes)

	tests := []struct {
		name        string
		txBytes     []byte
		wantFibreTx bool
		wantErr     bool
	}{
		{
			name:        "random bytes returns error",
			txBytes:     []byte("not-a-cosmos-tx"),
			wantFibreTx: false,
			wantErr:     true,
		},
		{
			name:        "empty bytes",
			txBytes:     []byte{},
			wantFibreTx: false,
			wantErr:     false,
		},
		{
			name:        "nil bytes",
			txBytes:     nil,
			wantFibreTx: false,
			wantErr:     false,
		},
		{
			name:        "valid MsgPayForFibre tx",
			txBytes:     test.BuildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 1),
			wantFibreTx: true,
			wantErr:     false,
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
			wantFibreTx: true,
			wantErr:     true,
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
			wantFibreTx: false,
			wantErr:     false,
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
			wantFibreTx: false,
			wantErr:     false,
		},
		{
			name: "SDK tx with nil body",
			txBytes: func() []byte {
				sdkTx := &cosmostx.Tx{}
				txBytes, err := proto.Marshal(sdkTx)
				require.NoError(t, err)
				return txBytes
			}(),
			wantFibreTx: false,
			wantErr:     false,
		},
		{
			name:        "BlobTx bytes returns error",
			txBytes:     test.GenerateBlobTx([]int{256}),
			wantFibreTx: false,
			wantErr:     true,
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
			wantFibreTx: true,
			wantErr:     true,
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
			wantFibreTx: true,
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fibreTx, isFibreTx, err := tx.TryParseFibreTx(tc.txBytes)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantFibreTx, isFibreTx)
			if !tc.wantFibreTx {
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
	signer := test.EncodeBech32("celestia", signerBytes)

	txBytes := test.BuildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 2)

	fibreTx, isFibreTx, err := tx.TryParseFibreTx(txBytes)
	require.NoError(t, err)
	require.True(t, isFibreTx)
	require.NotNil(t, fibreTx)

	expected, err := share.NewV2Blob(ns, 2, commitment, signerBytes)
	require.NoError(t, err)

	require.Equal(t, expected.Namespace(), fibreTx.SystemBlob.Namespace())
	require.Equal(t, expected.Data(), fibreTx.SystemBlob.Data())
	require.Equal(t, expected.ShareVersion(), fibreTx.SystemBlob.ShareVersion())
	require.Equal(t, expected.Signer(), fibreTx.SystemBlob.Signer())
}
