package share

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequenceRawData(t *testing.T) {
	type testCase struct {
		name     string
		Sequence Sequence
		want     []byte
		wantErr  bool
	}
	blobNamespace := RandomBlobNamespace()

	testCases := []testCase{
		{
			name: "empty share sequence",
			Sequence: Sequence{
				Namespace: TxNamespace,
				Shares:    []Share{},
			},
			want:    []byte{},
			wantErr: false,
		},
		{
			name: "one empty share",
			Sequence: Sequence{
				Namespace: TxNamespace,
				Shares: []Share{
					shareWithData(blobNamespace, true, 0, []byte{}),
				},
			},
			want:    []byte{},
			wantErr: false,
		},
		{
			name: "one share with one byte",
			Sequence: Sequence{
				Namespace: TxNamespace,
				Shares: []Share{
					shareWithData(blobNamespace, true, 1, []byte{0x0f}),
				},
			},
			want:    []byte{0xf},
			wantErr: false,
		},
		{
			name: "removes padding from last share",
			Sequence: Sequence{
				Namespace: TxNamespace,
				Shares: []Share{
					shareWithData(blobNamespace, true, FirstSparseShareContentSize+1, bytes.Repeat([]byte{0xf}, FirstSparseShareContentSize)),
					shareWithData(blobNamespace, false, 0, []byte{0x0f}),
				},
			},
			want:    bytes.Repeat([]byte{0xf}, FirstSparseShareContentSize+1),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.Sequence.RawData()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCompactSharesNeeded(t *testing.T) {
	type testCase struct {
		sequenceLen uint32
		want        int
	}
	testCases := []testCase{
		{0, 0},
		{1, 1},
		{2, 1},
		{FirstCompactShareContentSize, 1},
		{FirstCompactShareContentSize + 1, 2},
		{FirstCompactShareContentSize + ContinuationCompactShareContentSize, 2},
		{FirstCompactShareContentSize + ContinuationCompactShareContentSize*100, 101},
		{1000, 3},
		{10000, 21},
		{100000, 210},
		{math.MaxUint32 - ShareSize, 8985287},
		{math.MaxUint32, 8985288},
	}
	for _, tc := range testCases {
		got := CompactSharesNeeded(tc.sequenceLen)
		assert.Equal(t, tc.want, got)
	}
}

func TestSparseSharesNeeded(t *testing.T) {
	type testCase struct {
		sequenceLen    uint32
		containsSigner bool
		want           int
	}
	testCases := []testCase{
		{0, false, 0},
		{1, false, 1},
		{2, false, 1},
		{FirstSparseShareContentSize, false, 1},
		{FirstSparseShareContentSize + 1, false, 2},
		{FirstSparseShareContentSize + ContinuationSparseShareContentSize, false, 2},
		{FirstSparseShareContentSize + ContinuationCompactShareContentSize*2, false, 3},
		{FirstSparseShareContentSize + ContinuationCompactShareContentSize*99, false, 100},
		{1000, false, 3},
		{10000, false, 21},
		{100000, false, 208},
		{math.MaxUint32 - ShareSize, false, 8910720},
		{math.MaxUint32, false, 8910721},
		// Test case inspired by https://github.com/celestiaorg/celestia-node/issues/4490#issuecomment-3210533374
		{FirstSparseShareContentSizeWithSigner, true, 1},
		{FirstSparseShareContentSizeWithSigner + 1, true, 2},
	}
	for _, tc := range testCases {
		got := SparseSharesNeeded(tc.sequenceLen, tc.containsSigner)
		assert.Equal(t, tc.want, got)
	}
}

func shareWithData(namespace Namespace, isSequenceStart bool, sequenceLen uint32, data []byte) (rawShare Share) {
	infoByte, _ := NewInfoByte(ShareVersionZero, isSequenceStart)
	rawShareBytes := make([]byte, 0, ShareSize)
	rawShareBytes = append(rawShareBytes, namespace.Bytes()...)
	rawShareBytes = append(rawShareBytes, byte(infoByte))
	if isSequenceStart {
		sequenceLenBuf := make([]byte, SequenceLenBytes)
		binary.BigEndian.PutUint32(sequenceLenBuf, sequenceLen)
		rawShareBytes = append(rawShareBytes, sequenceLenBuf...)
	}
	rawShareBytes = append(rawShareBytes, data...)

	return padShare(Share{data: rawShareBytes})
}

func Test_validSequenceLen(t *testing.T) {
	type testCase struct {
		name     string
		Sequence Sequence
		wantErr  bool
	}

	tailPadding := Sequence{
		Namespace: TailPaddingNamespace,
		Shares:    []Share{TailPaddingShare()},
	}

	ns1 := MustNewV0Namespace(bytes.Repeat([]byte{0x1}, NamespaceVersionZeroIDSize))
	share, err := NamespacePaddingShare(ns1, ShareVersionZero)
	require.NoError(t, err)
	namespacePadding := Sequence{
		Namespace: ns1,
		Shares:    []Share{share},
	}

	reservedPadding := Sequence{
		Namespace: PrimaryReservedPaddingNamespace,
		Shares:    []Share{ReservedPaddingShare()},
	}

	notSequenceStart := Sequence{
		Namespace: ns1,
		Shares: []Share{
			shareWithData(ns1, false, 0, []byte{0x0f}),
		},
	}

	testCases := []testCase{
		{
			name:     "empty share sequence",
			Sequence: Sequence{},
			wantErr:  true,
		},
		{
			name:     "valid share sequence",
			Sequence: generateValidSequence(t),
			wantErr:  false,
		},
		{
			name:     "tail padding",
			Sequence: tailPadding,
			wantErr:  false,
		},
		{
			name:     "namespace padding",
			Sequence: namespacePadding,
			wantErr:  false,
		},
		{
			name:     "reserved padding",
			Sequence: reservedPadding,
			wantErr:  false,
		},
		{
			name:     "sequence length where first share is not sequence start",
			Sequence: notSequenceStart,
			wantErr:  true, // error: "share sequence has 1 shares but needed 0 shares"
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.Sequence.validSequenceLen()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func generateValidSequence(t *testing.T) Sequence {
	css := NewCompactShareSplitter(TxNamespace, ShareVersionZero)
	txs := generateRandomTxs(5, 200)
	for _, tx := range txs {
		err := css.WriteTx(tx)
		require.NoError(t, err)
	}
	shares, err := css.Export()
	require.NoError(t, err)

	return Sequence{
		Namespace: TxNamespace,
		Shares:    shares,
	}
}

func FuzzValidSequenceLen(f *testing.F) {
	f.Fuzz(func(t *testing.T, rawData []byte, rawNamespace []byte) {
		share, err := NewShare(rawData)
		if err != nil {
			t.Skip()
		}

		ns, err := NewNamespaceFromBytes(rawNamespace)
		if err != nil {
			t.Skip()
		}

		Sequence := Sequence{
			Namespace: ns,
			Shares:    []Share{share},
		}

		// want := fmt.Errorf("share sequence has 1 shares but needed 0 shares")
		err = Sequence.validSequenceLen()
		assert.NoError(t, err)
	})
}

// padShare returns a share padded with trailing zeros.
func padShare(share Share) (paddedShare Share) {
	return fillShare(share, 0)
}
