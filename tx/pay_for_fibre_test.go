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

// TestTryParseFibreTxDuplicateBodyField verifies that a repeated wire field 1
// resolves to its last occurrence, which is the body the Cosmos SDK's TxRaw
// decoder keeps.
//
// TxRaw.body_bytes is a scalar bytes field, so gogoproto discards all but the
// last occurrence. go-square models field 1 as the singular message Tx.body, and
// protobuf-go merges repeated occurrences of a message field, concatenating the
// inner repeated messages. Left unhandled, the two disagree about which message
// is first, and go-square must follow the SDK. Both orderings are covered so the
// agreement holds in either direction.
func TestTryParseFibreTxDuplicateBodyField(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	signerBytes := bytes.Repeat([]byte{0xAB}, share.SignerSize)
	signer, err := test.EncodeBech32("celestia", signerBytes)
	require.NoError(t, err)

	// A body holding a single MsgPayForFibre.
	fibreTxBytes, err := test.BuildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 1)
	require.NoError(t, err)
	fibreBody := firstBodyField(t, fibreTxBytes)

	// A body whose single message is an empty Any, so its TypeUrl is not
	// MsgPayForFibreTypeURL.
	otherBody, err := proto.Marshal(&cosmostx.TxBody{Messages: []*anypb.Any{{}}})
	require.NoError(t, err)

	// A repeated field 1 whose last occurrence is not a valid TxBody. The SDK's
	// decoder rejects these outright.
	invalidBody := []byte{0xFF}

	tests := []struct {
		name     string
		bodies   [][]byte
		wantNil  bool
		wantLast []byte
	}{
		{
			name:     "fibre body last",
			bodies:   [][]byte{otherBody, fibreBody},
			wantNil:  false,
			wantLast: fibreBody,
		},
		{
			name:     "fibre body first",
			bodies:   [][]byte{fibreBody, otherBody},
			wantNil:  true,
			wantLast: otherBody,
		},
		{
			name:     "duplicate fibre body",
			bodies:   [][]byte{fibreBody, fibreBody},
			wantNil:  false,
			wantLast: fibreBody,
		},
		{
			name:    "invalid body last",
			bodies:  [][]byte{fibreBody, invalidBody},
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var txBytes []byte
			for _, body := range tc.bodies {
				txBytes = protowire.AppendBytes(protowire.AppendTag(txBytes, 1, protowire.BytesType), body)
			}
			// Field numbers stay non-descending, so these bytes also satisfy the
			// SDK's ADR-027 check.
			if tc.wantLast != nil {
				require.Equal(t, tc.wantLast, lastBodyField(t, txBytes))
			}

			fibreTx, err := tx.TryParseFibreTx(txBytes)
			require.NoError(t, err)
			if tc.wantNil {
				require.Nil(t, fibreTx)
				return
			}
			require.NotNil(t, fibreTx)
			require.Equal(t, txBytes, fibreTx.Tx)
			require.Equal(t, ns, fibreTx.SystemBlob.Namespace())
			require.Equal(t, signerBytes, fibreTx.SystemBlob.Signer())
			require.Equal(t, share.ShareVersionTwo, fibreTx.SystemBlob.ShareVersion())
		})
	}
}

// firstBodyField returns the value of the first field 1 occurrence in txBytes.
func firstBodyField(t *testing.T, txBytes []byte) []byte {
	t.Helper()
	num, typ, n := protowire.ConsumeTag(txBytes)
	require.Equal(t, protowire.Number(1), num)
	require.Equal(t, protowire.BytesType, typ)
	body, m := protowire.ConsumeBytes(txBytes[n:])
	require.Positive(t, m)
	return body
}

// lastBodyField returns the value of the last field 1 occurrence in txBytes,
// which is the body_bytes the Cosmos SDK's TxRaw decoder keeps.
func lastBodyField(t *testing.T, txBytes []byte) []byte {
	t.Helper()
	var last []byte
	for len(txBytes) > 0 {
		num, typ, n := protowire.ConsumeTag(txBytes)
		require.Positive(t, n)
		require.Equal(t, protowire.BytesType, typ)
		value, m := protowire.ConsumeBytes(txBytes[n:])
		require.Positive(t, m)
		if num == 1 {
			last = value
		}
		txBytes = txBytes[n+m:]
	}
	require.NotNil(t, last)
	return last
}
