package tx_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4/internal/test"
	cosmostx "github.com/celestiaorg/go-square/v4/proto/cosmos/tx/v1beta1"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// TestValidateADR027TxRaw covers the encodings the Cosmos SDK's decoder accepts
// and rejects, plus the repeated-field case the SDK's own doc comment describes
// but does not enforce.
func TestValidateADR027TxRaw(t *testing.T) {
	bodyBytes, err := proto.Marshal(&cosmostx.TxBody{})
	require.NoError(t, err)

	singleBody, err := proto.Marshal(&cosmostx.TxRaw{BodyBytes: []byte("body")})
	require.NoError(t, err)

	full, err := proto.Marshal(&cosmostx.TxRaw{
		BodyBytes:     []byte("body"),
		AuthInfoBytes: []byte("auth"),
		Signatures:    [][]byte{[]byte("sig-a"), []byte("sig-b")},
	})
	require.NoError(t, err)

	authInfoOnly, err := proto.Marshal(&cosmostx.TxRaw{AuthInfoBytes: []byte("auth")})
	require.NoError(t, err)

	tests := []struct {
		name    string
		txBytes []byte
		wantErr bool
	}{
		{
			name:    "empty bytes",
			txBytes: nil,
			wantErr: false,
		},
		{
			name:    "single body field",
			txBytes: singleBody,
			wantErr: false,
		},
		{
			name:    "body, auth info, and multiple signatures",
			txBytes: full,
			wantErr: false,
		},
		{
			name:    "duplicate body field",
			txBytes: append(append([]byte{}, singleBody...), singleBody...),
			wantErr: true,
		},
		{
			name:    "duplicate auth info field",
			txBytes: append(append([]byte{}, authInfoOnly...), authInfoOnly...),
			wantErr: true,
		},
		{
			name:    "descending field order",
			txBytes: append(append([]byte{}, authInfoOnly...), singleBody...),
			wantErr: true,
		},
		{
			name:    "non bytes wire type",
			txBytes: []byte{0x08, 0x01}, // field 1, varint wire type
			wantErr: true,
		},
		{
			name:    "truncated length prefix",
			txBytes: []byte{0x0a},
			wantErr: true,
		},
		{
			name:    "length prefix longer than the remaining bytes",
			txBytes: []byte{0x0a, 0x05, 0x01},
			wantErr: true,
		},
		{
			name:    "non minimal length prefix varint",
			txBytes: []byte{0x0a, 0x81, 0x00, 0x01}, // 1 encoded in two bytes
			wantErr: true,
		},
		{
			name:    "empty body field",
			txBytes: append([]byte{0x0a, byte(len(bodyBytes))}, bodyBytes...),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tx.ValidateADR027TxRaw(tc.txBytes)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestValidateADR027TxRawAcceptsFibreTxs guards against the validator rejecting
// transactions this package is expected to classify.
func TestValidateADR027TxRawAcceptsFibreTxs(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	signer, err := test.EncodeBech32("celestia", bytes.Repeat([]byte{0xAB}, share.SignerSize))
	require.NoError(t, err)

	txBytes, err := test.BuildMsgPayForFibreTxBytes(signer, ns.Bytes(), commitment, 1)
	require.NoError(t, err)

	require.NoError(t, tx.ValidateADR027TxRaw(txBytes))

	fibreTx, err := tx.TryParseFibreTx(txBytes)
	require.NoError(t, err)
	require.NotNil(t, fibreTx)
}
