package square_test

import (
	"fmt"
	"math/rand"
	"testing"

	square "github.com/celestiaorg/go-square/v4"
	"github.com/celestiaorg/go-square/v4/internal/test"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/go-square/v4/tx"
	"github.com/stretchr/testify/require"
)

// classifiableTx is one entry of the corpus below: raw transaction bytes plus,
// for a pay-for-fibre transaction, the system blob a caller would synthesize.
type classifiableTx struct {
	name string
	raw  []byte
	// fibre is non-nil for a pay-for-fibre transaction. A fibre tx is never a
	// blob tx, so the pre-decoded blob field never applies to one.
	fibre *tx.FibreTx
}

// classifyAll classifies entries the way an application would. With preDecode
// set, a transaction that decodes as a blob tx carries its decoded form;
// otherwise ClassifiedTx.BlobTx is left nil and the square decodes the bytes
// itself. Everything else about the two results is identical by construction.
func classifyAll(entries []classifiableTx, preDecode bool) ([]square.ClassifiedTx, error) {
	classified := make([]square.ClassifiedTx, len(entries))
	for i, entry := range entries {
		var err error
		switch {
		case entry.fibre != nil:
			classified[i], err = square.NewClassifiedFibreTx(entry.fibre)
		case preDecode:
			decoded, isBlobTx, decodeErr := tx.UnmarshalBlobTx(entry.raw)
			if isBlobTx && decodeErr == nil {
				classified[i], err = square.NewClassifiedBlobTx(entry.raw, decoded)
			} else {
				classified[i] = square.NewClassifiedTx(entry.raw)
			}
		default:
			classified[i] = square.NewClassifiedTx(entry.raw)
		}
		if err != nil {
			return nil, fmt.Errorf("classifying %q at index %d: %w", entry.name, i, err)
		}
	}
	return classified, nil
}

func deterministicBytes(rng *rand.Rand, size int) []byte {
	b := make([]byte, size)
	// (*rand.Rand).Read never returns an error.
	_, _ = rng.Read(b)
	return b
}

func deterministicBlobTx(t *testing.T, rng *rand.Rand, blobSizes ...int) []byte {
	t.Helper()
	blobs := make([]*share.Blob, len(blobSizes))
	for i, size := range blobSizes {
		blob, err := share.NewBlob(test.DefaultTestNamespace, deterministicBytes(rng, size), share.ShareVersionZero, nil)
		require.NoError(t, err)
		blobs[i] = blob
	}
	raw, err := tx.MarshalBlobTx(deterministicBytes(rng, 300), blobs...)
	require.NoError(t, err)
	return raw
}

// predecodedCorpus returns transactions reaching every branch the pre-decoded
// field touches: normal txs, blob txs that decode, blob txs that are
// recognized but rejected, pay-for-fibre txs, and bytes that are not a valid
// classification at all.
func predecodedCorpus(t *testing.T) []classifiableTx {
	t.Helper()
	rng := rand.New(rand.NewSource(11))

	oneBlob := deterministicBlobTx(t, rng, 100)
	nested, err := tx.MarshalBlobTx(oneBlob, mustBlob(t, rng, 100))
	require.NoError(t, err)

	fibreRaw := []byte("pay for fibre tx bytes")
	fibreBlob, err := share.NewV2Blob(
		share.MustNewV0Namespace([]byte("fibre")),
		0,
		make([]byte, share.FibreCommitmentSize),
		make([]byte, share.SignerSize),
	)
	require.NoError(t, err)

	corpus := []classifiableTx{
		{name: "normal tx", raw: []byte("an ordinary transaction")},
		{name: "normal tx, share spanning", raw: deterministicBytes(rng, 900)},
		{name: "undecodable bytes", raw: []byte{0x00}},
		{name: "empty bytes", raw: nil},
		{name: "blob tx, one small blob", raw: oneBlob},
		{name: "blob tx, three blobs", raw: deterministicBlobTx(t, rng, 100, 500, 1000)},
		{name: "blob tx, multi-share blob", raw: deterministicBlobTx(t, rng, 2000)},
		// Recognized as a blob tx but rejected by UnmarshalBlobTx. The caller
		// cannot pre-decode these, so both sides take the decoding path and
		// must report the same error.
		{name: "non-canonical blob tx", raw: append(append([]byte(nil), oneBlob...), 0x78, 0x01)},
		{name: "nested blob tx", raw: nested},
		{name: "fibre tx", raw: fibreRaw, fibre: &tx.FibreTx{Tx: fibreRaw, SystemBlob: fibreBlob}},
	}
	return corpus
}

func mustBlob(t *testing.T, rng *rand.Rand, size int) *share.Blob {
	t.Helper()
	blob, err := share.NewBlob(test.DefaultTestNamespace, deterministicBytes(rng, size), share.ShareVersionZero, nil)
	require.NoError(t, err)
	return blob
}

// TestConstructIgnoresPreDecodedBlobTx is the property the ClassifiedTx.BlobTx
// field exists to preserve: handing Construct a blob tx the caller already
// decoded changes nothing it decides. For every generated list, the square
// bytes, the accept/reject verdict and the error text must match the same list
// classified without the field.
//
// This is a consensus property, not an optimization detail: both forms decide
// a proposal's data root, and ProcessProposal uses the pre-decoded one while
// other callers do not.
func TestConstructIgnoresPreDecodedBlobTx(t *testing.T) {
	corpus := predecodedCorpus(t)
	rng := rand.New(rand.NewSource(7))

	for _, maxSquareSize := range []int{4, 8, 64, 128} {
		for iteration := range 400 {
			entries := make([]classifiableTx, 1+rng.Intn(7))
			for i := range entries {
				entries[i] = corpus[rng.Intn(len(corpus))]
			}

			names := make([]string, len(entries))
			for i, entry := range entries {
				names[i] = entry.name
			}
			where := fmt.Sprintf("maxSquareSize=%d iteration=%d txs=%v", maxSquareSize, iteration, names)

			got, gotErr := classifyAll(entries, true)
			want, wantErr := classifyAll(entries, false)
			require.Equal(t, fmt.Sprint(wantErr), fmt.Sprint(gotErr), "classification disagreed: %s", where)
			if wantErr != nil {
				continue
			}
			require.Len(t, got, len(want))

			gotSquare, gotErr := square.Construct(got, maxSquareSize, defaultSubtreeRootThreshold)
			wantSquare, wantErr := square.Construct(want, maxSquareSize, defaultSubtreeRootThreshold)
			require.Equal(t, fmt.Sprint(wantErr), fmt.Sprint(gotErr), "construction disagreed: %s", where)
			if wantErr != nil {
				continue
			}
			require.Equal(t, wantSquare, gotSquare, "squares differ: %s", where)
		}
	}
}

// TestShareRangesIgnorePreDecodedBlobTx covers the other two exported callers
// of populateBuilder, which report share positions rather than a square.
func TestShareRangesIgnorePreDecodedBlobTx(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	fibreRaw := []byte("pay for fibre tx bytes")
	fibreBlob, err := share.NewV2Blob(
		share.MustNewV0Namespace([]byte("fibre")),
		0,
		make([]byte, share.FibreCommitmentSize),
		make([]byte, share.SignerSize),
	)
	require.NoError(t, err)

	entries := []classifiableTx{
		{name: "normal tx", raw: []byte("an ordinary transaction")},
		{name: "blob tx, two blobs", raw: deterministicBlobTx(t, rng, 300, 1500)},
		{name: "blob tx, one blob", raw: deterministicBlobTx(t, rng, 700)},
		{name: "fibre tx", raw: fibreRaw, fibre: &tx.FibreTx{Tx: fibreRaw, SystemBlob: fibreBlob}},
	}

	got, err := classifyAll(entries, true)
	require.NoError(t, err)
	want, err := classifyAll(entries, false)
	require.NoError(t, err)

	for txIndex := range entries {
		gotRange, gotErr := square.TxShareRange(got, txIndex, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		wantRange, wantErr := square.TxShareRange(want, txIndex, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.Equal(t, fmt.Sprint(wantErr), fmt.Sprint(gotErr), "TxShareRange at %d", txIndex)
		require.Equal(t, wantRange, gotRange, "TxShareRange at %d", txIndex)
	}

	// blob share ranges for the two blob txs, by (tx index, blob index)
	for _, position := range [][2]int{{1, 0}, {1, 1}, {2, 0}} {
		txIndex, blobIndex := position[0], position[1]
		gotRange, gotErr := square.BlobShareRange(got, txIndex, blobIndex, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		wantRange, wantErr := square.BlobShareRange(want, txIndex, blobIndex, defaultMaxSquareSize, defaultSubtreeRootThreshold)
		require.Equal(t, fmt.Sprint(wantErr), fmt.Sprint(gotErr), "BlobShareRange at %d/%d", txIndex, blobIndex)
		require.Equal(t, wantRange, gotRange, "BlobShareRange at %d/%d", txIndex, blobIndex)
	}
}

// TestConstructTrustsPreDecodedBlobTx pins the field's contract from the other
// side: Construct uses the pre-decoded blob tx as given and does not decode
// Bytes to check it. A caller that sets a blob tx which is not the decode of
// Bytes gets a square built from what it supplied.
//
// This is the documented, caller-trusted behaviour - the same trust
// Builder.AppendBlobTx already takes on its argument. The test exists so that
// re-introducing a decode here, which would silently undo the optimization and
// change what a mismatched caller produces, fails loudly.
func TestConstructTrustsPreDecodedBlobTx(t *testing.T) {
	rng := rand.New(rand.NewSource(17))
	realBytes := deterministicBlobTx(t, rng, 400)
	decoded, isBlobTx, err := tx.UnmarshalBlobTx(realBytes)
	require.NoError(t, err)
	require.True(t, isBlobTx)

	// Bytes that are not a blob tx at all, carrying the decoded blob tx above.
	mismatched, err := square.NewClassifiedBlobTx([]byte("not a blob tx"), decoded)
	require.NoError(t, err)
	fromMismatch, err := square.Construct([]square.ClassifiedTx{mismatched}, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.NoError(t, err)

	honest, err := square.NewClassifiedBlobTx(realBytes, decoded)
	require.NoError(t, err)
	fromHonest, err := square.Construct([]square.ClassifiedTx{honest}, defaultMaxSquareSize, defaultSubtreeRootThreshold)
	require.NoError(t, err)

	// The raw bytes never reach the square for a blob tx - only blobTx.Tx,
	// wrapped in an IndexWrapper - so the two squares are identical.
	require.Equal(t, fromHonest, fromMismatch)

	// Without the field the same mismatched bytes are a normal tx, and the
	// square differs. That difference is what the caller is trusted not to
	// introduce.
	asNormal, err := square.Construct(
		[]square.ClassifiedTx{square.NewClassifiedTx([]byte("not a blob tx"))},
		defaultMaxSquareSize, defaultSubtreeRootThreshold,
	)
	require.NoError(t, err)
	require.NotEqual(t, asNormal, fromMismatch)
}
