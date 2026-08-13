package square_test

import (
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

// systemBlob returns a share version two blob suitable for a FibreTx.
func systemBlob(t *testing.T) *share.Blob {
	t.Helper()
	ns := share.MustNewV0Namespace([]byte("fibre"))
	blob, err := share.NewV2Blob(ns, 0, make([]byte, share.FibreCommitmentSize), make([]byte, share.SignerSize))
	require.NoError(t, err)
	return blob
}

func TestClassifiedTxValidate(t *testing.T) {
	raw := []byte("raw tx bytes")
	blob := systemBlob(t)

	v0Blob, err := share.NewV0Blob(share.MustNewV0Namespace([]byte("fibre")), []byte("data"))
	require.NoError(t, err)

	testCases := []struct {
		name    string
		ctx     square.ClassifiedTx
		wantErr string
	}{
		{
			name: "normal tx",
			ctx:  square.ClassifiedTx{Bytes: raw},
		},
		{
			name: "fibre tx",
			ctx:  square.ClassifiedTx{Bytes: raw, FibreTx: &tx.FibreTx{Tx: raw, SystemBlob: blob}},
		},
		{
			name:    "empty bytes",
			ctx:     square.ClassifiedTx{},
			wantErr: "has no bytes",
		},
		{
			name:    "fibre tx with nil system blob",
			ctx:     square.ClassifiedTx{Bytes: raw, FibreTx: &tx.FibreTx{Tx: raw}},
			wantErr: "system blob",
		},
		{
			name:    "fibre tx bytes differ from classified bytes",
			ctx:     square.ClassifiedTx{Bytes: raw, FibreTx: &tx.FibreTx{Tx: []byte("other"), SystemBlob: blob}},
			wantErr: "differ from classified tx bytes",
		},
		{
			name:    "system blob is not share version two",
			ctx:     square.ClassifiedTx{Bytes: raw, FibreTx: &tx.FibreTx{Tx: raw, SystemBlob: v0Blob}},
			wantErr: "share version",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ctx.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNewClassifiedTx(t *testing.T) {
	raw := []byte("raw tx bytes")
	ctx := square.NewClassifiedTx(raw)
	require.Equal(t, raw, ctx.Bytes)
	require.Nil(t, ctx.FibreTx)
	require.NoError(t, ctx.Validate())
}

func TestNewClassifiedFibreTx(t *testing.T) {
	raw := []byte("raw tx bytes")
	blob := systemBlob(t)

	t.Run("valid", func(t *testing.T) {
		ctx, err := square.NewClassifiedFibreTx(&tx.FibreTx{Tx: raw, SystemBlob: blob})
		require.NoError(t, err)
		require.Equal(t, raw, ctx.Bytes)
		require.NotNil(t, ctx.FibreTx)
	})

	t.Run("nil fibre tx", func(t *testing.T) {
		_, err := square.NewClassifiedFibreTx(nil)
		require.ErrorContains(t, err, "nil")
	})

	t.Run("nil system blob", func(t *testing.T) {
		_, err := square.NewClassifiedFibreTx(&tx.FibreTx{Tx: raw})
		require.ErrorContains(t, err, "system blob")
	})
}
