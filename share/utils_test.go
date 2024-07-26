package share

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_zeroPadIfNecessary(t *testing.T) {
	type args struct {
		share []byte
		width int
	}
	tests := []struct {
		name               string
		args               args
		wantPadded         []byte
		wantBytesOfPadding int
	}{
		{"pad", args{[]byte{1, 2, 3}, 6}, []byte{1, 2, 3, 0, 0, 0}, 3},
		{"not necessary (equal to shareSize)", args{[]byte{1, 2, 3}, 3}, []byte{1, 2, 3}, 0},
		{"not necessary (greater shareSize)", args{[]byte{1, 2, 3}, 2}, []byte{1, 2, 3}, 0},
	}
	for _, tt := range tests {
		tt := tt // stupid scopelint :-/
		t.Run(tt.name, func(t *testing.T) {
			gotPadded, gotBytesOfPadding := zeroPadIfNecessary(tt.args.share, tt.args.width)
			if !reflect.DeepEqual(gotPadded, tt.wantPadded) {
				t.Errorf("zeroPadIfNecessary gotPadded %v, wantPadded %v", gotPadded, tt.wantPadded)
			}
			if gotBytesOfPadding != tt.wantBytesOfPadding {
				t.Errorf("zeroPadIfNecessary gotBytesOfPadding %v, wantBytesOfPadding %v", gotBytesOfPadding, tt.wantBytesOfPadding)
			}
		})
	}
}

func TestParseDelimiter(t *testing.T) {
	for i := uint64(0); i < 100; i++ {
		tx := generateRandomTxs(1, int(i))[0]
		input, err := MarshalDelimitedTx(tx)
		if err != nil {
			panic(err)
		}
		res, txLen, err := parseDelimiter(input)
		if err != nil {
			panic(err)
		}
		assert.Equal(t, i, txLen)
		assert.Equal(t, tx, res)
	}
}

func TestAvailableBytesFromCompactShares(t *testing.T) {
	testCases := []struct {
		name          string
		numShares     int
		expectedBytes int
	}{
		{
			name:          "1 share",
			numShares:     1,
			expectedBytes: 474,
		},
		{
			name:          "10 shares",
			numShares:     10,
			expectedBytes: 4776,
		},
		{
			name:          "negative",
			numShares:     -1,
			expectedBytes: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedBytes, AvailableBytesFromCompactShares(tc.numShares))
		})
	}
}

func TestAvailableBytesFromSparseShares(t *testing.T) {
	testCases := []struct {
		name          string
		numShares     int
		expectedBytes int
	}{
		{
			name:          "1 share",
			numShares:     1,
			expectedBytes: 478,
		},
		{
			name:          "10 shares",
			numShares:     10,
			expectedBytes: 4816,
		},
		{
			name:      "negative",
			numShares: -1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedBytes, AvailableBytesFromSparseShares(tc.numShares))
		})
	}
}
