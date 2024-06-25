package shares

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseShares(t *testing.T) {
	ns1 := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	ns2 := MustNewV0Namespace(bytes.Repeat([]byte{2}, NamespaceVersionZeroIDSize))

	txShares, _, _, err := SplitTxs(generateRandomTxs(2, 1000))
	require.NoError(t, err)
	txShareStart := txShares[0]
	txShareContinuation := txShares[1]

	blobOneShares, err := SplitBlobs(generateRandomBlobWithNamespace(ns1, 1000))
	require.NoError(t, err)
	blobOneStart := blobOneShares[0]
	blobOneContinuation := blobOneShares[1]

	blobTwoShares, err := SplitBlobs(generateRandomBlobWithNamespace(ns2, 1000))
	require.NoError(t, err)
	blobTwoStart := blobTwoShares[0]
	blobTwoContinuation := blobTwoShares[1]

	// tooLargeSequenceLen is a single share with too large of a sequence len
	// because it takes more than one share to store a sequence of 1000 bytes
	tooLargeSequenceLen := generateRawShare(t, ns1, true, uint32(1000))

	ns1Padding, err := NamespacePaddingShare(ns1, ShareVersionZero)
	require.NoError(t, err)

	type testCase struct {
		name          string
		shares        []Share
		ignorePadding bool
		want          []ShareSequence
		expectErr     bool
	}

	tests := []testCase{
		{
			name:          "empty",
			shares:        []Share{},
			ignorePadding: false,
			want:          []ShareSequence{},
			expectErr:     false,
		},
		{
			name:          "one transaction share",
			shares:        []Share{txShareStart},
			ignorePadding: false,
			want:          []ShareSequence{{Namespace: TxNamespace, Shares: []Share{txShareStart}}},
			expectErr:     false,
		},
		{
			name:          "two transaction shares",
			shares:        []Share{txShareStart, txShareContinuation},
			ignorePadding: false,
			want:          []ShareSequence{{Namespace: TxNamespace, Shares: []Share{txShareStart, txShareContinuation}}},
			expectErr:     false,
		},
		{
			name:          "one blob share",
			shares:        []Share{blobOneStart},
			ignorePadding: false,
			want:          []ShareSequence{{Namespace: ns1, Shares: []Share{blobOneStart}}},
			expectErr:     false,
		},
		{
			name:          "two blob shares",
			shares:        []Share{blobOneStart, blobOneContinuation},
			ignorePadding: false,
			want:          []ShareSequence{{Namespace: ns1, Shares: []Share{blobOneStart, blobOneContinuation}}},
			expectErr:     false,
		},
		{
			name:          "two blobs with two shares each",
			shares:        []Share{blobOneStart, blobOneContinuation, blobTwoStart, blobTwoContinuation},
			ignorePadding: false,
			want: []ShareSequence{
				{Namespace: ns1, Shares: []Share{blobOneStart, blobOneContinuation}},
				{Namespace: ns2, Shares: []Share{blobTwoStart, blobTwoContinuation}},
			},
			expectErr: false,
		},
		{
			name:          "one transaction, one blob",
			shares:        []Share{txShareStart, blobOneStart},
			ignorePadding: false,
			want: []ShareSequence{
				{Namespace: TxNamespace, Shares: []Share{txShareStart}},
				{Namespace: ns1, Shares: []Share{blobOneStart}},
			},
			expectErr: false,
		},
		{
			name:          "one transaction, two blobs",
			shares:        []Share{txShareStart, blobOneStart, blobTwoStart},
			ignorePadding: false,
			want: []ShareSequence{
				{Namespace: TxNamespace, Shares: []Share{txShareStart}},
				{Namespace: ns1, Shares: []Share{blobOneStart}},
				{Namespace: ns2, Shares: []Share{blobTwoStart}},
			},
			expectErr: false,
		},
		{
			name:          "blob one start followed by blob two continuation",
			shares:        []Share{blobOneStart, blobTwoContinuation},
			ignorePadding: false,
			want:          []ShareSequence{},
			expectErr:     true,
		},
		{
			name:          "one share with too large sequence length",
			shares:        []Share{{data: tooLargeSequenceLen}},
			ignorePadding: false,
			want:          []ShareSequence{},
			expectErr:     true,
		},
		{
			name:          "tail padding shares",
			shares:        TailPaddingShares(2),
			ignorePadding: false,
			want: []ShareSequence{
				{
					Namespace: TailPaddingNamespace,
					Shares:    []Share{TailPaddingShare()},
				},
				{
					Namespace: TailPaddingNamespace,
					Shares:    []Share{TailPaddingShare()},
				},
			},
			expectErr: false,
		},
		{
			name:          "reserve padding shares",
			shares:        ReservedPaddingShares(2),
			ignorePadding: false,
			want: []ShareSequence{
				{
					Namespace: PrimaryReservedPaddingNamespace,
					Shares:    []Share{ReservedPaddingShare()},
				},
				{
					Namespace: PrimaryReservedPaddingNamespace,
					Shares:    []Share{ReservedPaddingShare()},
				},
			},
			expectErr: false,
		},
		{
			name:          "namespace padding shares",
			shares:        []Share{ns1Padding, ns1Padding},
			ignorePadding: false,
			want: []ShareSequence{
				{
					Namespace: ns1,
					Shares:    []Share{ns1Padding},
				},
				{
					Namespace: ns1,
					Shares:    []Share{ns1Padding},
				},
			},
			expectErr: false,
		},
		{
			name:          "ignores all types of padding shares",
			shares:        []Share{TailPaddingShare(), ReservedPaddingShare(), ns1Padding},
			ignorePadding: true,
			want:          []ShareSequence{},
			expectErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseShares(tt.shares, tt.ignorePadding)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseShares() got %v, want %v", got, tt.want)
			}
		})
	}
}

func generateRawShare(t *testing.T, namespace Namespace, isSequenceStart bool, sequenceLen uint32) (rawShare []byte) {
	infoByte, _ := NewInfoByte(ShareVersionZero, isSequenceStart)

	sequenceLenBuf := make([]byte, SequenceLenBytes)
	binary.BigEndian.PutUint32(sequenceLenBuf, sequenceLen)

	rawShare = append(rawShare, namespace.Bytes()...)
	rawShare = append(rawShare, byte(infoByte))
	rawShare = append(rawShare, sequenceLenBuf...)

	return padWithRandomBytes(t, rawShare)
}

func padWithRandomBytes(t *testing.T, partialShare []byte) (paddedShare []byte) {
	paddedShare = make([]byte, ShareSize)
	copy(paddedShare, partialShare)
	_, err := rand.Read(paddedShare[len(partialShare):])
	require.NoError(t, err)
	return paddedShare
}

func generateRandomTxs(count, size int) [][]byte {
	txs := make([][]byte, count)
	for i := 0; i < count; i++ {
		tx := make([]byte, size)
		_, err := rand.Read(tx)
		if err != nil {
			panic(err)
		}
		txs[i] = tx
	}
	return txs
}
