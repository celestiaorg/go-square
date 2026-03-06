package tx_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
)

// buildMsgPayForFibreTxBytes constructs minimal Cosmos SDK Tx proto bytes
// containing a single MsgPayForFibre message, using raw protowire encoding.
// Field numbers mirror the proto definitions in celestia-app.
func buildMsgPayForFibreTxBytes(signer string, ns, commitment []byte, blobVersion uint32) []byte {
	// PaymentPromise: namespace=field3, blob_version=field5, commitment=field6
	var promise []byte
	promise = protowire.AppendTag(promise, 3, protowire.BytesType)
	promise = protowire.AppendBytes(promise, ns)
	promise = protowire.AppendTag(promise, 5, protowire.VarintType)
	promise = protowire.AppendVarint(promise, uint64(blobVersion))
	promise = protowire.AppendTag(promise, 6, protowire.BytesType)
	promise = protowire.AppendBytes(promise, commitment)

	// MsgPayForFibre: signer=field1, payment_promise=field2
	var msg []byte
	msg = protowire.AppendTag(msg, 1, protowire.BytesType)
	msg = protowire.AppendBytes(msg, []byte(signer))
	msg = protowire.AppendTag(msg, 2, protowire.BytesType)
	msg = protowire.AppendBytes(msg, promise)

	// Any: type_url=field1, value=field2
	var any []byte
	any = protowire.AppendTag(any, 1, protowire.BytesType)
	any = protowire.AppendBytes(any, []byte(tx.MsgPayForFibreTypeURL))
	any = protowire.AppendTag(any, 2, protowire.BytesType)
	any = protowire.AppendBytes(any, msg)

	// TxBody: messages=field1
	var body []byte
	body = protowire.AppendTag(body, 1, protowire.BytesType)
	body = protowire.AppendBytes(body, any)

	// Tx: body=field1
	var txBytes []byte
	txBytes = protowire.AppendTag(txBytes, 1, protowire.BytesType)
	txBytes = protowire.AppendBytes(txBytes, body)

	return txBytes
}

func TestSynthesizeFibreTx(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	// 20 raw address bytes encoded as bech32 with prefix "celestia"
	signerBytes := bytes.Repeat([]byte{0xAB}, share.SignerSize)
	signer := encodeBech32(t, "celestia", signerBytes)

	tests := []struct {
		name        string
		txBytes     []byte
		wantFibreTx bool
		wantErr     bool
	}{
		{
			name:        "non-fibre tx (random bytes)",
			txBytes:     []byte("not-a-cosmos-tx"),
			wantFibreTx: false,
			wantErr:     false,
		},
		{
			name:        "empty tx",
			txBytes:     []byte{},
			wantFibreTx: false,
			wantErr:     false,
		},
		{
			name:        "valid MsgPayForFibre tx",
			txBytes:     buildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 1),
			wantFibreTx: true,
			wantErr:     false,
		},
		{
			name: "tx with different message type",
			txBytes: func() []byte {
				var any []byte
				any = protowire.AppendTag(any, 1, protowire.BytesType)
				any = protowire.AppendBytes(any, []byte("/cosmos.bank.v1beta1.MsgSend"))
				var body []byte
				body = protowire.AppendTag(body, 1, protowire.BytesType)
				body = protowire.AppendBytes(body, any)
				var txBytes []byte
				txBytes = protowire.AppendTag(txBytes, 1, protowire.BytesType)
				txBytes = protowire.AppendBytes(txBytes, body)
				return txBytes
			}(),
			wantFibreTx: false,
			wantErr:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fibreTx, isFibreTx, err := tx.SynthesizeFibreTx(tc.txBytes)
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

// TestSynthesizeFibreTxMatchesManualConstruction verifies that SynthesizeFibreTx
// produces a FibreTx whose system blob matches one constructed manually from the
// same namespace, blobVersion, commitment, and signer bytes.
func TestSynthesizeFibreTxMatchesManualConstruction(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, share.NamespaceVersionZeroIDSize))
	commitment := bytes.Repeat([]byte{0xCC}, share.FibreCommitmentSize)
	signerBytes := bytes.Repeat([]byte{0x12}, share.SignerSize)
	signer := encodeBech32(t, "celestia", signerBytes)

	txBytes := buildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 2)

	fibreTx, isFibreTx, err := tx.SynthesizeFibreTx(txBytes)
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

// encodeBech32 encodes raw address bytes as a bech32 string.
// This is a test helper only; the production bech32 decoder lives in pay_for_fibre.go.
func encodeBech32(t *testing.T, hrp string, data []byte) string {
	t.Helper()
	const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

	// Convert 8-bit groups to 5-bit groups (with padding)
	acc, bits := 0, uint(0)
	var converted []byte
	for _, b := range data {
		acc = (acc << 8) | int(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			converted = append(converted, byte((acc>>bits)&0x1f))
		}
	}
	if bits > 0 {
		converted = append(converted, byte((acc<<(5-bits))&0x1f))
	}

	// Compute checksum
	checksum := bech32Checksum(hrp, converted)
	combined := append(converted, checksum...)

	s := hrp + "1"
	for _, b := range combined {
		s += string(charset[b])
	}
	return s
}

// bech32Checksum computes a 6-byte bech32 checksum over the hrp and data.
func bech32Checksum(hrp string, data []byte) []byte {
	generator := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}

	var values []byte
	for _, c := range hrp {
		values = append(values, byte(c)>>5)
	}
	values = append(values, 0)
	for _, c := range hrp {
		values = append(values, byte(c)&31)
	}
	values = append(values, data...)
	values = append(values, 0, 0, 0, 0, 0, 0)

	chk := uint32(1)
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (top>>i)&1 != 0 {
				chk ^= generator[i]
			}
		}
	}
	chk ^= 1

	out := make([]byte, 6)
	for i := range out {
		out[i] = byte((chk >> (5 * (5 - i))) & 0x1f)
	}
	return out
}
