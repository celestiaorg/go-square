package shares

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequenceLen(t *testing.T) {
	type testCase struct {
		name    string
		share   Share
		wantLen uint32
	}
	firstShare := append(bytes.Repeat([]byte{1}, NamespaceSize),
		[]byte{
			1,           // info byte
			0, 0, 0, 10, // sequence len
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, // data
		}...)
	firstShareWithLongSequence := append(bytes.Repeat([]byte{1}, NamespaceSize),
		[]byte{
			1,           // info byte
			0, 0, 1, 67, // sequence len
		}...)
	continuationShare := append(bytes.Repeat([]byte{1}, NamespaceSize),
		[]byte{
			0, // info byte
		}...)
	compactShare := append(TxNamespace.Bytes(),
		[]byte{
			1,           // info byte
			0, 0, 0, 10, // sequence len
		}...)
	testCases := []testCase{
		{
			name:    "first share",
			share:   Share{data: firstShare},
			wantLen: 10,
		},
		{
			name:    "first share with long sequence",
			share:   Share{data: firstShareWithLongSequence},
			wantLen: 323,
		},
		{
			name:    "continuation share",
			share:   Share{data: continuationShare},
			wantLen: 0,
		},
		{
			name:    "compact share",
			share:   Share{data: compactShare},
			wantLen: 10,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			length := tc.share.SequenceLen()
			if tc.wantLen != length {
				t.Errorf("want %d, got %d", tc.wantLen, length)
			}
		})
	}
}

func TestRawData(t *testing.T) {
	type testCase struct {
		name  string
		share Share
		want  []byte
	}
	sparseNamespaceID := MustNewV0Namespace(bytes.Repeat([]byte{0x1}, NamespaceVersionZeroIDSize))
	firstSparseShare := append(
		sparseNamespaceID.Bytes(),
		[]byte{
			1,           // info byte
			0, 0, 0, 10, // sequence len
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, // data
		}...)
	continuationSparseShare := append(
		sparseNamespaceID.Bytes(),
		[]byte{
			0,                             // info byte
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, // data
		}...)
	firstCompactShare := append(TxNamespace.Bytes(),
		[]byte{
			1,           // info byte
			0, 0, 0, 10, // sequence len
			0, 0, 0, 15, // reserved bytes
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, // data
		}...)
	continuationCompactShare := append(TxNamespace.Bytes(),
		[]byte{
			0,          // info byte
			0, 0, 0, 0, // reserved bytes
			1, 2, 3, 4, 5, 6, 7, 8, 9, 10, // data
		}...)
	testCases := []testCase{
		{
			name:  "first sparse share",
			share: Share{data: firstSparseShare},
			want:  []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			name:  "continuation sparse share",
			share: Share{data: continuationSparseShare},
			want:  []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			name:  "first compact share",
			share: Share{data: firstCompactShare},
			want:  []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			name:  "continuation compact share",
			share: Share{data: continuationCompactShare},
			want:  []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.share.RawData())
		})
	}
}

func TestIsCompactShare(t *testing.T) {
	type testCase struct {
		name  string
		share Share
		want  bool
	}

	ns1 := MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize))
	txShare, _ := zeroPadIfNecessary(TxNamespace.Bytes(), ShareSize)
	pfbTxShare, _ := zeroPadIfNecessary(PayForBlobNamespace.Bytes(), ShareSize)
	blobShare, _ := zeroPadIfNecessary(ns1.Bytes(), ShareSize)

	testCases := []testCase{
		{
			name:  "tx share",
			share: Share{data: txShare},
			want:  true,
		},
		{
			name:  "pfb tx share",
			share: Share{data: pfbTxShare},
			want:  true,
		},
		{
			name:  "blob share",
			share: Share{data: blobShare},
			want:  false,
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.want, tc.share.IsCompactShare())
	}
}

func TestIsPadding(t *testing.T) {
	type testCase struct {
		name  string
		share Share
		want  bool
	}
	blobShare, _ := zeroPadIfNecessary(
		append(
			ns1.Bytes(),
			[]byte{
				1,          // info byte
				0, 0, 0, 1, // sequence len
				0xff, // data
			}...,
		),
		ShareSize)

	nsPadding, err := NamespacePaddingShare(ns1, ShareVersionZero)
	require.NoError(t, err)

	testCases := []testCase{
		{
			name:  "blob share",
			share: Share{data: blobShare},
			want:  false,
		},
		{
			name:  "namespace padding",
			share: nsPadding,
			want:  true,
		},
		{
			name:  "tail padding",
			share: TailPaddingShare(),
			want:  true,
		},
		{
			name:  "reserved padding",
			share: ReservedPaddingShare(),
			want:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.share.IsPadding())
		})
	}
}
