package square

import (
	"bytes"
	"reflect"
	"testing"

	v4 "github.com/celestiaorg/go-square/v4/proto/blob/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

// snapshotBuilder copies b, including every unexported field, so that two
// snapshots compare equal with reflect.DeepEqual only if nothing changed.
// Counters are copied by value so the comparison does not short-circuit on
// shared pointers.
func snapshotBuilder(b *Builder) Builder {
	c := *b
	txCounter, pfbCounter, pffCounter := *b.TxCounter, *b.PfbCounter, *b.PayForFibreCounter
	c.TxCounter, c.PfbCounter, c.PayForFibreCounter = &txCounter, &pfbCounter, &pffCounter
	c.Txs = append([][]byte(nil), b.Txs...)
	c.Pfbs = append([]*v4.IndexWrapper(nil), b.Pfbs...)
	c.PayForFibreTxs = append([][]byte(nil), b.PayForFibreTxs...)
	c.Blobs = append([]*Element(nil), b.Blobs...)
	return c
}

// TestBuilderFitsLeavesStateUnchanged asserts, field by field, that no Fits
// method mutates the builder, for fitting, non-fitting and invalid input,
// before and after an Export, and after a revert (so the revert flags and
// undo sizes are populated).
func TestBuilderFitsLeavesStateUnchanged(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	signer := bytes.Repeat([]byte{0xAA}, share.SignerSize)
	commitment := bytes.Repeat([]byte{0xFF}, share.FibreCommitmentSize)
	systemBlob, err := share.NewV2Blob(ns, 1, commitment, signer)
	require.NoError(t, err)
	blob, err := share.NewV0Blob(ns, bytes.Repeat([]byte{2}, 300))
	require.NoError(t, err)
	fibre := func(raw []byte) *tx.FibreTx { return &tx.FibreTx{Tx: raw, SystemBlob: systemBlob} }
	blobTx := func(raw []byte) *tx.BlobTx { return &tx.BlobTx{Tx: raw, Blobs: []*share.Blob{blob}} }

	probes := []struct {
		name string
		fits func(b *Builder) (bool, error)
	}{
		{"tx fits", func(b *Builder) (bool, error) { return b.FitsTx([]byte("small")) }},
		{"tx too large", func(b *Builder) (bool, error) { return b.FitsTx(make([]byte, 100_000)) }},
		{"tx empty", func(b *Builder) (bool, error) { return b.FitsTx(nil) }},
		{"blob tx fits", func(b *Builder) (bool, error) { return b.FitsBlobTx(blobTx([]byte("pfb"))) }},
		{"blob tx too large", func(b *Builder) (bool, error) { return b.FitsBlobTx(blobTx(make([]byte, 100_000))) }},
		{"blob tx nil", func(b *Builder) (bool, error) { return b.FitsBlobTx(nil) }},
		{"fibre tx fits", func(b *Builder) (bool, error) { return b.FitsFibreTx(fibre([]byte("pff"))) }},
		{"fibre tx too large", func(b *Builder) (bool, error) { return b.FitsFibreTx(fibre(make([]byte, 100_000))) }},
		{"fibre tx invalid", func(b *Builder) (bool, error) { return b.FitsFibreTx(fibre(nil)) }},
	}

	states := []struct {
		name  string
		setup func(t *testing.T, b *Builder)
	}{
		{"fresh", func(*testing.T, *Builder) {}},
		{"populated", func(t *testing.T, b *Builder) {
			require.True(t, b.AppendTx([]byte("tx")))
			added, err := b.AppendBlobTx(blobTx([]byte("pfb")))
			require.NoError(t, err)
			require.True(t, added)
			added, err = b.AppendFibreTx(fibre([]byte("pff")))
			require.NoError(t, err)
			require.True(t, added)
		}},
		{"populated and exported", func(t *testing.T, b *Builder) {
			require.True(t, b.AppendTx([]byte("tx")))
			added, err := b.AppendBlobTx(blobTx([]byte("pfb")))
			require.NoError(t, err)
			require.True(t, added)
			_, err = b.Export()
			require.NoError(t, err)
			require.True(t, b.done)
		}},
		{"populated and reverted", func(t *testing.T, b *Builder) {
			require.True(t, b.AppendTx([]byte("tx")))
			require.True(t, b.AppendTx([]byte("tx2")))
			require.NoError(t, b.RevertLastTx())
			added, err := b.AppendBlobTx(blobTx([]byte("pfb")))
			require.NoError(t, err)
			require.True(t, added)
			require.NoError(t, b.RevertLastBlobTx())
			added, err = b.AppendFibreTx(fibre([]byte("pff")))
			require.NoError(t, err)
			require.True(t, added)
			require.NoError(t, b.RevertLastPayForFibreTx())
			require.True(t, b.txReverted && b.blobTxReverted && b.payForFibreTxReverted)
		}},
	}

	for _, state := range states {
		for _, probe := range probes {
			t.Run(state.name+"/"+probe.name, func(t *testing.T) {
				b, err := NewBuilder(8, 64)
				require.NoError(t, err)
				state.setup(t, b)
				before := snapshotBuilder(b)
				_, _ = probe.fits(b)
				require.True(t, reflect.DeepEqual(before, snapshotBuilder(b)), "Fits mutated the builder")
			})
		}
	}
}
