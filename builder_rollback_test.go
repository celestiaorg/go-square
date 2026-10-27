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

// builderView is the externally observable state of a builder. Fits* must
// leave it unchanged; see TestBuilderFitsLeavesStateUnchanged (package
// square) for the field-level check.
type builderView struct {
	txs, pfbs, pffs, blobs         int
	currentSize, numTxs            int
	txShares, pfbShares, pffShares int
	txRem, pfbRem, pffRem          int
	empty                          bool
	squareHash                     [32]byte
}

func viewBuilder(t *testing.T, b *square.Builder) builderView {
	t.Helper()
	sq, err := b.Export()
	require.NoError(t, err)
	return builderView{
		txs: len(b.Txs), pfbs: len(b.Pfbs), pffs: len(b.PayForFibreTxs), blobs: len(b.Blobs),
		currentSize: b.CurrentSize(), numTxs: b.NumTxs(),
		txShares: b.TxCounter.Size(), pfbShares: b.PfbCounter.Size(), pffShares: b.PayForFibreCounter.Size(),
		txRem: b.TxCounter.Remainder(), pfbRem: b.PfbCounter.Remainder(), pffRem: b.PayForFibreCounter.Remainder(),
		empty:      b.IsEmpty(),
		squareHash: sq.Hash(),
	}
}

func newBlobTx(t *testing.T, ns share.Namespace, blobSizes ...int) *tx.BlobTx {
	t.Helper()
	raw := generateBlobTxsWithNamespaces(t, ns.Repeat(len(blobSizes)), [][]int{blobSizes})[0]
	blobTx, isBlobTx, err := tx.UnmarshalBlobTx(raw)
	require.NoError(t, err)
	require.True(t, isBlobTx)
	return blobTx
}

// fitsAppendCase pairs a Fits call with the Append call it predicts.
type fitsAppendCase struct {
	name string
	// prefill puts the builder into the state under test.
	prefill func(t *testing.T, b *square.Builder)
	fits    func(b *square.Builder) (bool, error)
	appendF func(b *square.Builder) (bool, error)
	wantFit bool
	// wantErr is the error FitsX must return; Append then returns !added.
	wantErr string
	// wantAppendErr is whether AppendX also returns the error (AppendTx
	// cannot; it only returns false).
	wantAppendErr bool
}

func fitsAppendCases(t *testing.T) []fitsAppendCase {
	t.Helper()
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	// All builders are 2x2 = 4 shares.
	fullSquare := func(t *testing.T, b *square.Builder) {
		t.Helper()
		require.True(t, b.AppendTx(newTx(share.AvailableBytesFromCompactShares(4))))
	}
	noop := func(*testing.T, *square.Builder) {}
	txCase := func(name string, prefill func(*testing.T, *square.Builder), raw []byte, wantFit bool, wantErr string) fitsAppendCase {
		return fitsAppendCase{
			name: "normal/" + name, prefill: prefill,
			fits:    func(b *square.Builder) (bool, error) { return b.FitsTx(raw) },
			appendF: func(b *square.Builder) (bool, error) { return b.AppendTx(raw), nil },
			wantFit: wantFit, wantErr: wantErr,
		}
	}
	blobCase := func(name string, prefill func(*testing.T, *square.Builder), blobTx *tx.BlobTx, wantFit bool, wantErr string) fitsAppendCase {
		return fitsAppendCase{
			name: "blob/" + name, prefill: prefill,
			fits:    func(b *square.Builder) (bool, error) { return b.FitsBlobTx(blobTx) },
			appendF: func(b *square.Builder) (bool, error) { return b.AppendBlobTx(blobTx) },
			wantFit: wantFit, wantErr: wantErr, wantAppendErr: wantErr != "",
		}
	}
	fibreCase := func(name string, prefill func(*testing.T, *square.Builder), fibreTx *tx.FibreTx, wantFit bool, wantErr string) fitsAppendCase {
		return fitsAppendCase{
			name: "fibre/" + name, prefill: prefill,
			fits:    func(b *square.Builder) (bool, error) { return b.FitsFibreTx(fibreTx) },
			appendF: func(b *square.Builder) (bool, error) { return b.AppendFibreTx(fibreTx) },
			wantFit: wantFit, wantErr: wantErr, wantAppendErr: wantErr != "",
		}
	}

	fibreWithTx := func(raw []byte) *tx.FibreTx {
		f := newFibreTx(t, ns)
		f.Tx = raw
		return f
	}
	v0Blob, err := share.NewV0Blob(ns, []byte("not a system blob"))
	require.NoError(t, err)
	nilBlobTx := newBlobTx(t, ns, 10)
	nilBlobTx.Blobs[0] = nil
	twoBlobsFit := newBlobTx(t, ns, share.AvailableBytesFromSparseShares(1), share.AvailableBytesFromSparseShares(1))
	twoBlobsOver := newBlobTx(t, ns, share.AvailableBytesFromSparseShares(2)+1, share.AvailableBytesFromSparseShares(1))

	return []fitsAppendCase{
		txCase("fits", noop, newTx(100), true, ""),
		txCase("exact capacity", noop, newTx(share.AvailableBytesFromCompactShares(4)), true, ""),
		txCase("one byte over capacity", noop, newTx(share.AvailableBytesFromCompactShares(4)+1), false, ""),
		txCase("square full", fullSquare, newTx(10), false, ""),
		txCase("fits after partial fill", func(t *testing.T, b *square.Builder) {
			require.True(t, b.AppendTx(newTx(share.AvailableBytesFromCompactShares(3))))
		}, newTx(share.AvailableBytesFromCompactShares(1)-1), true, ""),
		txCase("empty", noop, []byte{}, false, "must not be empty"),
		txCase("nil", noop, nil, false, "must not be empty"),
		txCase("empty on full square", fullSquare, nil, false, "must not be empty"),

		blobCase("fits", noop, newBlobTx(t, ns, 10), true, ""),
		blobCase("exact capacity", noop, newBlobTx(t, ns, share.AvailableBytesFromSparseShares(3)), true, ""),
		blobCase("one byte over capacity", noop, newBlobTx(t, ns, share.AvailableBytesFromSparseShares(3)+1), false, ""),
		blobCase("two blobs fit", noop, twoBlobsFit, true, ""),
		blobCase("two blobs over capacity", noop, twoBlobsOver, false, ""),
		blobCase("square full", fullSquare, newBlobTx(t, ns, 1), false, ""),
		blobCase("nil blob tx", noop, nil, false, "must not be nil"),
		blobCase("nil blob", noop, nilBlobTx, false, "must not be nil"),

		fibreCase("fits", noop, newFibreTx(t, ns), true, ""),
		fibreCase("exact capacity", noop, fibreWithTx(newTx(share.AvailableBytesFromCompactShares(3))), true, ""),
		fibreCase("one byte over capacity", noop, fibreWithTx(newTx(share.AvailableBytesFromCompactShares(3)+1)), false, ""),
		fibreCase("square full", fullSquare, newFibreTx(t, ns), false, ""),
		// Three fibre txs share one compact share and use three sparse shares.
		fibreCase("third fibre tx fits exactly", func(t *testing.T, b *square.Builder) {
			mustAppendFibreTx(t, b, ns)
			mustAppendFibreTx(t, b, ns)
		}, newFibreTx(t, ns), true, ""),
		fibreCase("fourth fibre tx does not fit", func(t *testing.T, b *square.Builder) {
			mustAppendFibreTx(t, b, ns)
			mustAppendFibreTx(t, b, ns)
			mustAppendFibreTx(t, b, ns)
		}, newFibreTx(t, ns), false, ""),
		fibreCase("nil", noop, nil, false, "must not be nil"),
		fibreCase("empty tx bytes", noop, fibreWithTx([]byte{}), false, "transaction bytes"),
		fibreCase("nil tx bytes", noop, fibreWithTx(nil), false, "transaction bytes"),
		fibreCase("nil system blob", noop, &tx.FibreTx{Tx: []byte("raw")}, false, "system blob"),
		fibreCase("wrong share version", noop, &tx.FibreTx{Tx: []byte("raw"), SystemBlob: v0Blob}, false, "ShareVersionTwo"),
	}
}

// TestBuilderFitsMatchesAppend asserts that each Fits method returns the
// verdict its Append counterpart would, without changing anything a caller
// can observe about the builder.
func TestBuilderFitsMatchesAppend(t *testing.T) {
	for _, tc := range fitsAppendCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			b, err := square.NewBuilder(2, 64)
			require.NoError(t, err)
			tc.prefill(t, b)
			before := viewBuilder(t, b)

			fits, err := tc.fits(b)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantFit, fits)
			require.Equal(t, before, viewBuilder(t, b), "Fits must not change the builder")

			added, err := tc.appendF(b)
			if tc.wantAppendErr {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, fits, added, "Fits and Append disagree")
			if !added {
				require.Equal(t, before, viewBuilder(t, b), "a rejected Append must not change the builder")
				return
			}
			after := viewBuilder(t, b)
			require.NotEqual(t, before, after)
			require.Equal(t, before.numTxs+1, after.numTxs)
			require.Greater(t, after.currentSize, before.currentSize)
		})
	}
}

// TestBuilderFitsThenAppendMatchesAppendThenRevert drives one builder with the
// deprecated Append-then-Revert pattern and another with Fits-then-Append and
// asserts they build the same square.
func TestBuilderFitsThenAppendMatchesAppendThenRevert(t *testing.T) {
	ns := share.MustNewV0Namespace(bytes.Repeat([]byte{1}, share.NamespaceVersionZeroIDSize))
	ns2 := share.MustNewV0Namespace(bytes.Repeat([]byte{2}, share.NamespaceVersionZeroIDSize))

	type step struct {
		kind string
		keep bool
		raw  []byte
		blob *tx.BlobTx
		fib  *tx.FibreTx
	}
	oversizedFibre := newFibreTx(t, ns)
	oversizedFibre.Tx = newTx(100_000)
	steps := []step{
		{kind: "normal", keep: true, raw: newTx(100)},
		{kind: "normal", keep: false, raw: newTx(200)},
		{kind: "normal", keep: true, raw: newTx(300)},
		{kind: "normal", keep: true, raw: newTx(100_000)}, // never fits
		{kind: "normal", keep: false, raw: newTx(50)},
		{kind: "normal", keep: true, raw: newTx(share.AvailableBytesFromCompactShares(2))},
		{kind: "blob", keep: true, blob: newBlobTx(t, ns2, 100)},
		{kind: "blob", keep: false, blob: newBlobTx(t, ns, 2000)},
		{kind: "blob", keep: true, blob: newBlobTx(t, ns, 500, 700)},
		{kind: "blob", keep: true, blob: newBlobTx(t, ns, 100_000)}, // never fits
		{kind: "blob", keep: false, blob: newBlobTx(t, ns2, 10)},
		{kind: "blob", keep: true, blob: newBlobTx(t, ns2, share.AvailableBytesFromSparseShares(4))},
		{kind: "fibre", keep: true, fib: newFibreTx(t, ns)},
		{kind: "fibre", keep: false, fib: newFibreTx(t, ns2)},
		{kind: "fibre", keep: true, fib: oversizedFibre}, // never fits
		{kind: "fibre", keep: true, fib: newFibreTx(t, ns2)},
		{kind: "fibre", keep: false, fib: newFibreTx(t, ns)},
	}

	revertBuilder, err := square.NewBuilder(8, 64)
	require.NoError(t, err)
	fitsBuilder, err := square.NewBuilder(8, 64)
	require.NoError(t, err)

	kept := 0
	for i, s := range steps {
		var revertAdded, fits bool
		switch s.kind {
		case "normal":
			revertAdded = revertBuilder.AppendTx(s.raw)
			if revertAdded && !s.keep {
				require.NoError(t, revertBuilder.RevertLastTx())
			}
			fits, err = fitsBuilder.FitsTx(s.raw)
			require.NoError(t, err)
			if fits && s.keep {
				require.True(t, fitsBuilder.AppendTx(s.raw))
			}
		case "blob":
			revertAdded, err = revertBuilder.AppendBlobTx(s.blob)
			require.NoError(t, err)
			if revertAdded && !s.keep {
				require.NoError(t, revertBuilder.RevertLastBlobTx())
			}
			fits, err = fitsBuilder.FitsBlobTx(s.blob)
			require.NoError(t, err)
			if fits && s.keep {
				mustAppendBlobTx(t, fitsBuilder, s.blob)
			}
		case "fibre":
			revertAdded, err = revertBuilder.AppendFibreTx(s.fib)
			require.NoError(t, err)
			if revertAdded && !s.keep {
				require.NoError(t, revertBuilder.RevertLastPayForFibreTx())
			}
			fits, err = fitsBuilder.FitsFibreTx(s.fib)
			require.NoError(t, err)
			if fits && s.keep {
				added, err := fitsBuilder.AppendFibreTx(s.fib)
				require.NoError(t, err)
				require.True(t, added)
			}
		}
		require.Equal(t, revertAdded, fits, "step %d", i)
		if fits && s.keep {
			kept++
		}
		require.Equal(t, revertBuilder.CurrentSize(), fitsBuilder.CurrentSize(), "step %d", i)
	}
	require.Equal(t, 8, kept)
	require.Equal(t, kept, fitsBuilder.NumTxs())
	require.Equal(t, viewBuilder(t, revertBuilder), viewBuilder(t, fitsBuilder))
	want, err := revertBuilder.Export()
	require.NoError(t, err)
	got, err := fitsBuilder.Export()
	require.NoError(t, err)
	require.True(t, want.Equals(got))
	require.False(t, got.IsEmpty())
}

// TestBuilderEmptyTxSurfacesClearError asserts that an empty transaction is
// reported as invalid, with ErrEmptyTx, everywhere go-square sees it, rather
// than as a transaction that does not fit.
func TestBuilderEmptyTxSurfacesClearError(t *testing.T) {
	for _, empty := range [][]byte{nil, {}} {
		b, err := square.NewBuilder(8, 64)
		require.NoError(t, err)

		fits, err := b.FitsTx(empty)
		require.ErrorIs(t, err, square.ErrEmptyTx)
		require.False(t, fits)
		// AppendTx keeps its boolean signature, so it can only say no.
		require.False(t, b.AppendTx(empty))
		require.True(t, b.IsEmpty())

		require.ErrorIs(t, square.NewClassifiedTx(empty).Validate(), square.ErrEmptyTx)

		_, err = square.Construct([]square.ClassifiedTx{square.NewClassifiedTx(empty)}, 8, 64)
		require.ErrorIs(t, err, square.ErrEmptyTx)
		require.ErrorContains(t, err, "classified tx at index 0")
		require.NotContains(t, err.Error(), "not enough space")

		_, err = square.NewBuilder(8, 64, square.NewClassifiedTx([]byte("ok")), square.NewClassifiedTx(empty))
		require.ErrorIs(t, err, square.ErrEmptyTx)
		require.ErrorContains(t, err, "classified tx at index 1")
	}
}
