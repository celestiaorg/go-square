package square_test

import (
	"bytes"
	"testing"

	"github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

func TestBuilderRevertAfterRejectedAppend(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	blobTxBytes := generateBlobTxsWithNamespaces(t, []share.Namespace{ns}, [][]int{{100}})[0]
	blobTx, _, err := tx.UnmarshalBlobTx(blobTxBytes)
	require.NoError(t, err)

	for _, kind := range []string{"normal", "blob", "fibre"} {
		for _, retainBlob := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/empty", true: "/retained-blob"}[retainBlob], func(t *testing.T) {
				b, err := square.NewBuilder(8, 64)
				require.NoError(t, err)
				var remaining []square.ClassifiedTx
				if retainBlob {
					mustAppendBlobTx(t, b, blobTx)
					remaining = append(remaining, square.NewClassifiedTx(blobTxBytes))
				}
				switch kind {
				case "normal":
					require.True(t, b.AppendTx(newTx(100)))
					for range 2 {
						require.False(t, b.AppendTx(newTx(100_000)))
					}
					require.NoError(t, b.RevertLastTx())
				case "blob":
					mustAppendBlobTx(t, b, blobTx)
					oversized := *blobTx
					oversized.Tx = newTx(100_000)
					for range 2 {
						added, err := b.AppendBlobTx(&oversized)
						require.NoError(t, err)
						require.False(t, added)
					}
					require.NoError(t, b.RevertLastBlobTx())
				case "fibre":
					mustAppendFibreTx(t, b, ns)
					oversized := newFibreTx(t, ns)
					oversized.Tx = newTx(100_000)
					for range 2 {
						added, err := b.AppendFibreTx(oversized)
						require.NoError(t, err)
						require.False(t, added)
					}
					require.NoError(t, b.RevertLastPayForFibreTx())
				}
				require.Equal(t, !retainBlob, b.IsEmpty())
				got, err := b.Export()
				require.NoError(t, err)
				want, err := square.Construct(remaining, 8, 64)
				require.NoError(t, err)
				require.True(t, want.Equals(got), "rollback must match reconstruction from the remaining transactions")
				for _, s := range got {
					require.Len(t, s.ToBytes(), share.ShareSize)
				}
			})
		}
	}
}

func TestBuilderRevertFibreAfterExport(t *testing.T) {
	nsA := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	nsB := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, share.NamespaceVersionZeroIDSize))
	for _, sameNamespace := range []bool{false, true} {
		t.Run(map[bool]string{false: "reverse namespace order", true: "same namespace and blob"}[sameNamespace], func(t *testing.T) {
			b, err := square.NewBuilder(8, 64)
			require.NoError(t, err)
			first := newFibreTx(t, nsB)
			first.Tx = []byte("first-fibre-tx")
			mustAppend := func(f *tx.FibreTx) {
				added, err := b.AppendFibreTx(f)
				require.NoError(t, err)
				require.True(t, added)
			}
			mustAppend(first)
			second := newFibreTx(t, nsA)
			if sameNamespace {
				second.SystemBlob = first.SystemBlob
			}
			second.Tx = []byte("second-fibre-tx")
			mustAppend(second)
			_, err = b.Export()
			require.NoError(t, err)
			require.NoError(t, b.RevertLastPayForFibreTx())
			require.Len(t, b.Blobs, 1)
			require.Same(t, first.SystemBlob, b.Blobs[0].Blob)
			require.Equal(t, [][]byte{first.Tx}, b.PayForFibreTxs)
			classified, err := square.NewClassifiedFibreTx(first)
			require.NoError(t, err)
			want, err := square.Construct([]square.ClassifiedTx{classified}, 8, 64)
			require.NoError(t, err)
			got, err := b.Export()
			require.NoError(t, err)
			require.True(t, want.Equals(got))
			// Reusing the builder must also track the newly appended element.
			mustAppend(second)
			_, err = b.Export()
			require.NoError(t, err)
			require.NoError(t, b.RevertLastPayForFibreTx())
			got, err = b.Export()
			require.NoError(t, err)
			require.True(t, want.Equals(got))
		})
	}
}

func TestBuilderRejectsEmptyTxBytes(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	for _, fibre := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "fibre"}[fibre], func(t *testing.T) {
			b, err := square.NewBuilder(8, 64)
			require.NoError(t, err)
			for _, empty := range [][]byte{nil, {}} {
				if fibre {
					f := newFibreTx(t, ns)
					f.Tx = empty
					added, err := b.AppendFibreTx(f)
					require.Error(t, err)
					require.False(t, added)
				} else {
					require.False(t, b.AppendTx(empty))
				}
			}
			require.True(t, b.IsEmpty())
			require.Zero(t, b.NumTxs())
			require.Empty(t, b.Blobs)
			want := [][]byte{[]byte("first"), []byte("second")}
			for _, raw := range want {
				if fibre {
					f := newFibreTx(t, ns)
					f.Tx = raw
					added, err := b.AppendFibreTx(f)
					require.NoError(t, err)
					require.True(t, added)
				} else {
					require.True(t, b.AppendTx(raw))
				}
			}
			sq, err := b.Export()
			require.NoError(t, err)
			namespace := share.TxNamespace
			if fibre {
				namespace = share.PayForFibreNamespace
			}
			txRange := share.GetShareRangeForNamespace(sq, namespace)
			parsed, err := share.ParseTxs(sq[txRange.Start:txRange.End])
			require.NoError(t, err)
			require.Equal(t, want, parsed)
		})
	}
}

func TestWriteSquareReservedPaddingWithoutBlobs(t *testing.T) {
	for _, withTx := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "with tx"}[withTx], func(t *testing.T) {
			txWriter := share.NewCompactShareSplitter(share.TxNamespace, share.ShareVersionZero)
			if withTx {
				require.NoError(t, txWriter.WriteTx([]byte("tx")))
			}
			sq, err := square.WriteSquare(txWriter,
				share.NewCompactShareSplitter(share.PayForBlobNamespace, share.ShareVersionZero),
				share.NewCompactShareSplitter(share.PayForFibreNamespace, share.ShareVersionZero),
				share.NewSparseShareSplitter(), txWriter.Count()+1, 2)
			require.NoError(t, err)
			for _, s := range sq {
				require.Len(t, s.ToBytes(), share.ShareSize)
			}
			require.Equal(t, share.ReservedPaddingShares(1)[0], sq[txWriter.Count()])
		})
	}
}
