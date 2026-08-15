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
// per entry in bodies.
func marshalTxWithBodies(t *testing.T, bodies ...*cosmostx.TxBody) []byte {
	t.Helper()
	raw := make([][]byte, 0, len(bodies))
	for _, body := range bodies {
		bodyBytes, err := proto.Marshal(body)
		require.NoError(t, err)
		raw = append(raw, bodyBytes)
	}
	return marshalTxWithBodyBytes(t, raw...)
}

// marshalTxWithBodyBytes encodes a transaction carrying one top-level body field
// per entry in bodies, without requiring each entry to be a valid TxBody.
// Concatenating separately marshalled transactions is exactly how a repeated
// occurrence of the field is encoded on the wire.
func marshalTxWithBodyBytes(t *testing.T, bodies ...[]byte) []byte {
	t.Helper()
	out := make([]byte, 0, len(bodies))
	for _, body := range bodies {
		txBytes, err := proto.Marshal(&cosmostx.TxRaw{BodyBytes: body})
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
	fibreBody, ns, signerBytes := fibreBodyFixture(t)
	normalBody := normalBodyFixture()

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

// TestTryParseFibreTxDuplicateBodyIsNotDecoded asserts that only the last
// occurrence of the body field is decoded. Earlier occurrences are opaque bytes
// to the SDK, so bytes that are not a TxBody at all must not change the answer,
// and a malformed last occurrence must not fall back to an earlier one.
func TestTryParseFibreTxDuplicateBodyIsNotDecoded(t *testing.T) {
	fibreBody, _, _ := fibreBodyFixture(t)
	normalBody := normalBodyFixture()

	fibreBodyBytes, err := proto.Marshal(fibreBody)
	require.NoError(t, err)
	normalBodyBytes, err := proto.Marshal(normalBody)
	require.NoError(t, err)
	malformed := []byte{0xFF}

	tests := []struct {
		name string
		// bodies are encoded in order; the last one is authoritative.
		bodies    [][]byte
		wantFibre bool
	}{
		{
			name:      "malformed body before fibre body is a fibre tx",
			bodies:    [][]byte{malformed, fibreBodyBytes},
			wantFibre: true,
		},
		{
			name:      "malformed body after fibre body is not a fibre tx",
			bodies:    [][]byte{fibreBodyBytes, malformed},
			wantFibre: false,
		},
		{
			name:      "malformed and valid bodies resolve to a fibre last body",
			bodies:    [][]byte{malformed, normalBodyBytes, malformed, fibreBodyBytes},
			wantFibre: true,
		},
		{
			name:      "malformed and valid bodies resolve to a normal last body",
			bodies:    [][]byte{malformed, fibreBodyBytes, malformed, normalBodyBytes},
			wantFibre: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fibreTx, err := tx.TryParseFibreTx(marshalTxWithBodyBytes(t, tc.bodies...))
			require.NoError(t, err)
			require.Equal(t, tc.wantFibre, fibreTx != nil)
		})
	}
}

// TestTryParseFibreTxDuplicateBodyBeforeSignedTx covers the shape a duplicated
// body actually takes on the wire: an extra body field prepended to an otherwise
// ordinary transaction, so the auth info and signatures that follow it still
// appear in ascending field order.
func TestTryParseFibreTxDuplicateBodyBeforeSignedTx(t *testing.T) {
	fibreBody, _, _ := fibreBodyFixture(t)
	normalBody := normalBodyFixture()

	// signedTx encodes body, auth info and a signature, as a signer would.
	signedTx := func(t *testing.T, body *cosmostx.TxBody) []byte {
		t.Helper()
		bodyBytes, err := proto.Marshal(body)
		require.NoError(t, err)
		txBytes, err := proto.Marshal(&cosmostx.TxRaw{
			BodyBytes:     bodyBytes,
			AuthInfoBytes: []byte("auth-info"),
			Signatures:    [][]byte{[]byte("signature")},
		})
		require.NoError(t, err)
		return txBytes
	}

	tests := []struct {
		name       string
		prefixBody *cosmostx.TxBody
		signedBody *cosmostx.TxBody
		wantFibre  bool
	}{
		{
			name:       "fibre body prepended to a signed normal tx is not a fibre tx",
			prefixBody: fibreBody,
			signedBody: normalBody,
			wantFibre:  false,
		},
		{
			name:       "normal body prepended to a signed fibre tx is a fibre tx",
			prefixBody: normalBody,
			signedBody: fibreBody,
			wantFibre:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			txBytes := append(marshalTxWithBodies(t, tc.prefixBody), signedTx(t, tc.signedBody)...)

			fibreTx, err := tx.TryParseFibreTx(txBytes)
			require.NoError(t, err)
			require.Equal(t, tc.wantFibre, fibreTx != nil)
		})
	}
}

// fibreBodyFixture returns a TxBody carrying a single valid MsgPayForFibre,
// along with the namespace and signer that message pays for.
func fibreBodyFixture(t *testing.T) (body *cosmostx.TxBody, ns share.Namespace, signerBytes []byte) {
	t.Helper()
	ns = share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	signerBytes = bytes.Repeat([]byte{0xAB}, share.SignerSize)
	signer, err := test.EncodeBech32("celestia", signerBytes)
	require.NoError(t, err)

	msgBytes, err := proto.Marshal(&fibrev1.MsgPayForFibre{
		Signer: signer,
		PaymentPromise: &fibrev1.PaymentPromise{
			Namespace:   ns.Bytes(),
			BlobVersion: 1,
			Commitment:  bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize),
		},
	})
	require.NoError(t, err)

	return &cosmostx.TxBody{
		Messages: []*anypb.Any{{TypeUrl: tx.MsgPayForFibreTypeURL, Value: msgBytes}},
	}, ns, signerBytes
}

// normalBodyFixture returns a TxBody carrying a single message that is not a
// MsgPayForFibre.
func normalBodyFixture() *cosmostx.TxBody {
	return &cosmostx.TxBody{
		Messages: []*anypb.Any{{TypeUrl: "/cosmos.bank.v1beta1.MsgSend", Value: []byte("v")}},
	}
}
