package shares

import (
	"bytes"
	"crypto/sha256"
	"reflect"
	"testing"

	"github.com/celestiaorg/go-square/pkg/blob"
	"github.com/celestiaorg/go-square/pkg/namespace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitTxs_forTxShares(t *testing.T) {
	smallTransactionA := []byte{0xa}
	smallTransactionB := []byte{0xb}
	largeTransaction := bytes.Repeat([]byte{0xc}, 512)

	type testCase struct {
		name string
		txs  [][]byte
		want []Share
	}
	testCases := []testCase{
		{
			name: "empty txs",
			txs:  [][]byte{},
			want: []Share{},
		},
		{
			name: "one small tx",
			txs:  [][]byte{smallTransactionA},
			want: []Share{
				padShare(Share{
					data: append(
						namespace.TxNamespace.Bytes(),
						[]byte{
							0x1,                // info byte
							0x0, 0x0, 0x0, 0x2, // 1 byte (unit) + 1 byte (unit length) = 2 bytes sequence length
							0x0, 0x0, 0x0, 0x26, // reserved bytes
							0x1, // unit length of first transaction
							0xa, // data of first transaction
						}...,
					),
				},
				),
			},
		},
		{
			name: "two small txs",
			txs:  [][]byte{smallTransactionA, smallTransactionB},
			want: []Share{
				padShare(Share{
					data: append(
						namespace.TxNamespace.Bytes(),
						[]byte{
							0x1,                // info byte
							0x0, 0x0, 0x0, 0x4, // 2 bytes (first transaction) + 2 bytes (second transaction) = 4 bytes sequence length
							0x0, 0x0, 0x0, 0x26, // reserved bytes
							0x1, // unit length of first transaction
							0xa, // data of first transaction
							0x1, // unit length of second transaction
							0xb, // data of second transaction
						}...,
					),
				},
				),
			},
		},
		{
			name: "one large tx that spans two shares",
			txs:  [][]byte{largeTransaction},
			want: []Share{
				fillShare(Share{
					data: append(
						namespace.TxNamespace.Bytes(),
						[]byte{
							0x1,                // info byte
							0x0, 0x0, 0x2, 0x2, // 512 (unit) + 2 (unit length) = 514 sequence length
							0x0, 0x0, 0x0, 0x26, // reserved bytes
							128, 4, // unit length of transaction is 512
						}...,
					),
				},
					0xc, // data of transaction
				),
				padShare(Share{
					data: append(
						append(
							namespace.TxNamespace.Bytes(),
							[]byte{
								0x0,                // info byte
								0x0, 0x0, 0x0, 0x0, // reserved bytes
							}...,
						),
						bytes.Repeat([]byte{0xc}, 40)..., // continuation data of transaction
					),
				},
				),
			},
		},
		{
			name: "one small tx then one large tx that spans two shares",
			txs:  [][]byte{smallTransactionA, largeTransaction},
			want: []Share{
				fillShare(Share{
					data: append(
						namespace.TxNamespace.Bytes(),
						[]byte{
							0x1,                // info byte
							0x0, 0x0, 0x2, 0x4, // 2 bytes (first transaction) + 514 bytes (second transaction) = 516 bytes sequence length
							0x0, 0x0, 0x0, 0x26, // reserved bytes
							1,      // unit length of first transaction
							0xa,    // data of first transaction
							128, 4, // unit length of second transaction is 512
						}...,
					),
				},
					0xc, // data of second transaction
				),
				padShare(Share{
					data: append(
						append(
							namespace.TxNamespace.Bytes(),
							[]byte{
								0x0,                // info byte
								0x0, 0x0, 0x0, 0x0, // reserved bytes
							}...,
						),
						bytes.Repeat([]byte{0xc}, 42)..., // continuation data of second transaction
					),
				},
				),
			},
		},
		{
			name: "one large tx that spans two shares then one small tx",
			txs:  [][]byte{largeTransaction, smallTransactionA},
			want: []Share{
				fillShare(Share{
					data: append(
						namespace.TxNamespace.Bytes(),
						[]byte{
							0x1,                // info byte
							0x0, 0x0, 0x2, 0x4, // 514 bytes (first transaction) + 2 bytes (second transaction) = 516 bytes sequence length
							0x0, 0x0, 0x0, 0x26, // reserved bytes
							128, 4, // unit length of first transaction is 512
						}...,
					),
				},
					0xc, // data of first transaction
				),
				padShare(Share{
					data: append(
						namespace.TxNamespace.Bytes(),
						[]byte{
							0x0,                 // info byte
							0x0, 0x0, 0x0, 0x4a, // reserved bytes
							0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, // continuation data of first transaction
							0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, // continuation data of first transaction
							0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, 0xc, // continuation data of first transaction
							1,   // unit length of second transaction
							0xa, // data of second transaction
						}...,
					),
				},
				),
			},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _, err := SplitTxs(tt.txs)
			require.NoError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitTxs()\n got %#v\n want %#v", got, tt.want)
			}
		})
	}
}

func TestSplitTxs(t *testing.T) {
	type testCase struct {
		name          string
		txs           [][]byte
		wantTxShares  []Share
		wantPfbShares []Share
		wantMap       map[[sha256.Size]byte]Range
	}

	smallTx := []byte{0xa} // spans one share
	smallTxShares := []Share{
		padShare(Share{
			data: append(namespace.TxNamespace.Bytes(),
				[]byte{
					0x1,                // info byte
					0x0, 0x0, 0x0, 0x2, // 1 byte (unit) + 1 byte (unit length) = 2 bytes sequence length
					0x0, 0x0, 0x0, 0x26, // reserved bytes
					0x1, // unit length of first transaction
					0xa, // data of first transaction
				}...,
			),
		},
		),
	}

	pfbTx, err := blob.MarshalIndexWrapper([]byte{0xb}, 10) // spans one share
	require.NoError(t, err)
	pfbTxShares := []Share{
		padShare(Share{
			data: append(
				namespace.PayForBlobNamespace.Bytes(),
				[]uint8{
					0x1,               // info byte
					0x0, 0x0, 0x0, 13, // 1 byte (unit) + 1 byte (unit length) = 2 bytes sequence length
					0x0, 0x0, 0x0, 0x26, // reserved bytes
					12,                                                               // unit length of first transaction
					0xa, 0x1, 0xb, 0x12, 0x1, 0xa, 0x1a, 0x4, 0x49, 0x4e, 0x44, 0x58, // data of first transaction
				}...,
			),
		},
		),
	}

	largeTx := bytes.Repeat([]byte{0xc}, ShareSize) // spans two shares
	largeTxShares := []Share{
		fillShare(Share{
			data: append(namespace.TxNamespace.Bytes(),
				[]uint8{
					0x1,                // info byte
					0x0, 0x0, 0x2, 0x2, // 512 (unit) + 2 (unit length) = 514 sequence length
					0x0, 0x0, 0x0, 0x26, // reserved bytes
					128, 4, // unit length of transaction is 512
				}...,
			),
		},
			0xc), // data of transaction
		padShare(Share{
			data: append(
				append(
					namespace.TxNamespace.Bytes(),
					[]uint8{
						0x0,                // info byte
						0x0, 0x0, 0x0, 0x0, // reserved bytes
					}...,
				),
				bytes.Repeat([]byte{0xc}, 40)..., // continuation data of transaction
			),
		},
		),
	}

	testCases := []testCase{
		{
			name:          "empty",
			txs:           [][]byte{},
			wantTxShares:  []Share{},
			wantPfbShares: []Share{},
			wantMap:       map[[sha256.Size]byte]Range{},
		},
		{
			name:          "smallTx",
			txs:           [][]byte{smallTx},
			wantTxShares:  smallTxShares,
			wantPfbShares: []Share{},
			wantMap: map[[sha256.Size]byte]Range{
				sha256.Sum256(smallTx): {0, 1},
			},
		},
		{
			name:          "largeTx",
			txs:           [][]byte{largeTx},
			wantTxShares:  largeTxShares,
			wantPfbShares: []Share{},
			wantMap: map[[sha256.Size]byte]Range{
				sha256.Sum256(largeTx): {0, 2},
			},
		},
		{
			name:          "pfbTx",
			txs:           [][]byte{pfbTx},
			wantTxShares:  []Share{},
			wantPfbShares: pfbTxShares,
			wantMap: map[[sha256.Size]byte]Range{
				sha256.Sum256(pfbTx): {0, 1},
			},
		},
		{
			name:          "largeTx then pfbTx",
			txs:           [][]byte{largeTx, pfbTx},
			wantTxShares:  largeTxShares,
			wantPfbShares: pfbTxShares,
			wantMap: map[[sha256.Size]byte]Range{
				sha256.Sum256(largeTx): {0, 2},
				sha256.Sum256(pfbTx):   {2, 3},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			txShares, pfbTxShares, gotMap, err := SplitTxs(tc.txs)
			require.NoError(t, err)
			assert.Equal(t, tc.wantTxShares, txShares)
			assert.Equal(t, tc.wantPfbShares, pfbTxShares)
			assert.Equal(t, tc.wantMap, gotMap)
		})
	}
}

// padShare returns a share padded with trailing zeros.
func padShare(share Share) (paddedShare Share) {
	return fillShare(share, 0)
}

// fillShare returns a share filled with filler so that the share length
// is equal to ShareSize.
func fillShare(share Share, filler byte) (paddedShare Share) {
	return Share{data: append(share.data, bytes.Repeat([]byte{filler}, ShareSize-len(share.data))...)}
}

func Test_mergeMaps(t *testing.T) {
	type testCase struct {
		name   string
		mapOne map[[sha256.Size]byte]Range
		mapTwo map[[sha256.Size]byte]Range
		want   map[[sha256.Size]byte]Range
	}
	testCases := []testCase{
		{
			name:   "empty maps",
			mapOne: map[[sha256.Size]byte]Range{},
			mapTwo: map[[sha256.Size]byte]Range{},
			want:   map[[sha256.Size]byte]Range{},
		},
		{
			name: "merges maps with one key each",
			mapOne: map[[sha256.Size]byte]Range{
				{0x1}: {0, 1},
			},
			mapTwo: map[[sha256.Size]byte]Range{
				{0x2}: {2, 3},
			},
			want: map[[sha256.Size]byte]Range{
				{0x1}: {0, 1},
				{0x2}: {2, 3},
			},
		},
		{
			name: "merges maps with multiple keys each",
			mapOne: map[[sha256.Size]byte]Range{
				{0x1}: {0, 1},
				{0x2}: {2, 3},
			},
			mapTwo: map[[sha256.Size]byte]Range{
				{0x3}: {3, 3},
				{0x4}: {4, 4},
			},
			want: map[[sha256.Size]byte]Range{
				{0x1}: {0, 1},
				{0x2}: {2, 3},
				{0x3}: {3, 3},
				{0x4}: {4, 4},
			},
		},
		{
			name: "merges maps with a duplicate key and the second map's value takes precedence",
			mapOne: map[[sha256.Size]byte]Range{
				{0x1}: {0, 0},
			},
			mapTwo: map[[sha256.Size]byte]Range{
				{0x1}: {1, 1},
			},
			want: map[[sha256.Size]byte]Range{
				{0x1}: {1, 1},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeMaps(tc.mapOne, tc.mapTwo)
			assert.Equal(t, tc.want, got)
		})
	}
}
