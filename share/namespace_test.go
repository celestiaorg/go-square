package share

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	validID         = append(NamespaceVersionZeroPrefix, bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize)...)
	tooShortID      = append(NamespaceVersionZeroPrefix, []byte{1}...)
	tooLongID       = append(NamespaceVersionZeroPrefix, bytes.Repeat([]byte{1}, NamespaceSize)...)
	invalidPrefixID = bytes.Repeat([]byte{1}, NamespaceSize)
)

func TestNew(t *testing.T) {
	type testCase struct {
		name    string
		version uint8
		id      []byte
		wantErr bool
		want    Namespace
	}

	testCases := []testCase{
		{
			name:    "valid namespace",
			version: NamespaceVersionZero,
			id:      validID,
			wantErr: false,
			want:    MustNewNamespace(NamespaceVersionZero, validID),
		},
		{
			name:    "unsupported version",
			version: uint8(1),
			id:      validID,
			wantErr: true,
		},
		{
			name:    "unsupported id: too short",
			version: NamespaceVersionZero,
			id:      tooShortID,
			wantErr: true,
		},
		{
			name:    "unsupported id: too long",
			version: NamespaceVersionZero,
			id:      tooLongID,
			wantErr: true,
		},
		{
			name:    "unsupported id: invalid prefix",
			version: NamespaceVersionZero,
			id:      invalidPrefixID,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewNamespace(tc.version, tc.id)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestNewV0(t *testing.T) {
	type testCase struct {
		name    string
		subID   []byte
		want    Namespace
		wantErr bool
	}

	testCases := []testCase{
		{
			name:    "valid namespace",
			subID:   bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize),
			want:    MustNewNamespace(NamespaceVersionZero, append(NamespaceVersionZeroPrefix, bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize)...)),
			wantErr: false,
		},
		{
			name:    "left pads subID if too short",
			subID:   []byte{1, 2, 3, 4},
			want:    MustNewNamespace(NamespaceVersionZero, append(NamespaceVersionZeroPrefix, []byte{0, 0, 0, 0, 0, 0, 1, 2, 3, 4}...)),
			wantErr: false,
		},
		{
			name:    "invalid namespace because subID is too long",
			subID:   bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize+1),
			want:    Namespace{},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		got, err := NewV0Namespace(tc.subID)
		if tc.wantErr {
			assert.Error(t, err)
			return
		}
		assert.NoError(t, err)
		assert.Equal(t, tc.want, got)
	}
}

func TestFrom(t *testing.T) {
	type testCase struct {
		name    string
		bytes   []byte
		wantErr bool
		want    Namespace
	}
	validNamespace := []byte{}
	validNamespace = append(validNamespace, NamespaceVersionZero)
	validNamespace = append(validNamespace, NamespaceVersionZeroPrefix...)
	validNamespace = append(validNamespace, bytes.Repeat([]byte{0x1}, NamespaceVersionZeroIDSize)...)
	parityNamespace := bytes.Repeat([]byte{0xFF}, NamespaceSize)

	testCases := []testCase{
		{
			name:    "valid namespace",
			bytes:   validNamespace,
			wantErr: false,
			want:    MustNewNamespace(NamespaceVersionZero, validID),
		},
		{
			name:    "parity namespace",
			bytes:   parityNamespace,
			wantErr: false,
			want:    MustNewNamespace(NamespaceVersionMax, bytes.Repeat([]byte{0xFF}, NamespaceIDSize)),
		},
		{
			name:    "unsupported version",
			bytes:   append([]byte{1}, append(NamespaceVersionZeroPrefix, bytes.Repeat([]byte{1}, NamespaceSize-len(NamespaceVersionZeroPrefix))...)...),
			wantErr: true,
		},
		{
			name:    "unsupported id: too short",
			bytes:   append([]byte{NamespaceVersionZero}, tooShortID...),
			wantErr: true,
		},
		{
			name:    "unsupported id: too long",
			bytes:   append([]byte{NamespaceVersionZero}, tooLongID...),
			wantErr: true,
		},
		{
			name:    "unsupported id: invalid prefix",
			bytes:   append([]byte{NamespaceVersionZero}, invalidPrefixID...),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewNamespaceFromBytes(tc.bytes)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBytes(t *testing.T) {
	namespace, err := NewNamespace(NamespaceVersionZero, validID)
	assert.NoError(t, err)

	want := append([]byte{NamespaceVersionZero}, validID...)
	got := namespace.Bytes()

	assert.Equal(t, want, got)
}

func TestLeftPad(t *testing.T) {
	tests := []struct {
		input    []byte
		size     int
		expected []byte
	}{
		// input smaller than pad size
		{[]byte{1, 2, 3}, 10, []byte{0, 0, 0, 0, 0, 0, 0, 1, 2, 3}},
		{[]byte{1}, 5, []byte{0, 0, 0, 0, 1}},
		{[]byte{1, 2}, 4, []byte{0, 0, 1, 2}},

		// input equal to pad size
		{[]byte{1, 2, 3}, 3, []byte{1, 2, 3}},
		{[]byte{1, 2, 3, 4}, 4, []byte{1, 2, 3, 4}},

		// input larger than pad size
		{[]byte{1, 2, 3, 4, 5}, 4, []byte{1, 2, 3, 4, 5}},
		{[]byte{1, 2, 3, 4, 5, 6, 7}, 3, []byte{1, 2, 3, 4, 5, 6, 7}},

		// input size 0
		{[]byte{}, 8, []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{[]byte{}, 0, []byte{}},
	}

	for _, test := range tests {
		result := leftPad(test.input, test.size)
		assert.True(t, reflect.DeepEqual(result, test.expected))
	}
}

func TestIsReserved(t *testing.T) {
	type testCase struct {
		ns   Namespace
		want bool
	}
	testCases := []testCase{
		{
			ns:   MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize)),
			want: false,
		},
		{
			ns:   TxNamespace,
			want: true,
		},
		{
			ns:   IntermediateStateRootsNamespace,
			want: true,
		},
		{
			ns:   PayForBlobNamespace,
			want: true,
		},
		{
			ns:   PayForFibreNamespace,
			want: true,
		},
		{
			ns:   PrimaryReservedPaddingNamespace,
			want: true,
		},
		{
			ns:   MaxPrimaryReservedNamespace,
			want: true,
		},
		{
			ns:   MinSecondaryReservedNamespace,
			want: true,
		},
		{
			ns:   TailPaddingNamespace,
			want: true,
		},
		{
			ns:   ParitySharesNamespace,
			want: true,
		},
		{
			ns:   MustNewNamespace(math.MaxUint8, append(bytes.Repeat([]byte{0xFF}, NamespaceIDSize-1), 1)),
			want: true,
		},
	}

	for _, tc := range testCases {
		got := tc.ns.IsReserved()
		assert.Equal(t, tc.want, got)
	}
}

func Test_compareMethods(t *testing.T) {
	minID := RandomBlobNamespaceID()
	maxID := RandomBlobNamespaceID()
	// repeat until maxID meets our expectations (maxID > minID).
	for bytes.Compare(maxID, minID) != 1 {
		maxID = RandomBlobNamespaceID()
	}

	vers := []byte{NamespaceVersionZero, NamespaceVersionMax}
	ids := [][]byte{append(NamespaceVersionZeroPrefix, minID...), append(NamespaceVersionZeroPrefix, maxID...)}

	// collect all possible pairs: (ver1 ?? ver2) x (id1 ?? id2)
	var testPairs [][2]Namespace
	for _, ver1 := range vers {
		for _, ver2 := range vers {
			for _, id1 := range ids {
				for _, id2 := range ids {
					testPairs = append(testPairs, [2]Namespace{
						MustNewNamespace(ver1, id1),
						MustNewNamespace(ver2, id2),
					})
				}
			}
		}
	}
	require.Len(t, testPairs, 16) // len(vers) * len(vers) * len(ids) * len(ids)

	type testCase struct {
		name string
		fn   func(n, n2 Namespace) bool
		old  func(n, n2 Namespace) bool
	}
	testCases := []testCase{
		{
			name: "Equals",
			fn:   Namespace.Equals,
			old: func(n, n2 Namespace) bool {
				return bytes.Equal(n.Bytes(), n2.Bytes())
			},
		},
		{
			name: "IsLessThan",
			fn:   Namespace.IsLessThan,
			old: func(n, n2 Namespace) bool {
				return bytes.Compare(n.Bytes(), n2.Bytes()) == -1
			},
		},
		{
			name: "IsLessOrEqualThan",
			fn:   Namespace.IsLessOrEqualThan,
			old: func(n, n2 Namespace) bool {
				return bytes.Compare(n.Bytes(), n2.Bytes()) < 1
			},
		},
		{
			name: "IsGreaterThan",
			fn:   Namespace.IsGreaterThan,
			old: func(n, n2 Namespace) bool {
				return bytes.Compare(n.Bytes(), n2.Bytes()) == 1
			},
		},
		{
			name: "IsGreaterOrEqualThan",
			fn:   Namespace.IsGreaterOrEqualThan,
			old: func(n, n2 Namespace) bool {
				return bytes.Compare(n.Bytes(), n2.Bytes()) > -1
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i, p := range testPairs {
				n, n2 := p[0], p[1]
				got := tc.fn(n, n2)
				want := tc.old(n, n2)
				assert.Equal(t, want, got, "for pair %d", i)
			}
		})
	}
}

func TestMarshalNamespace(t *testing.T) {
	ns := RandomNamespace()
	b, err := ns.MarshalJSON()
	require.NoError(t, err)

	newNs := Namespace{}
	err = newNs.UnmarshalJSON(b)
	require.NoError(t, err)

	require.Equal(t, ns, newNs)
}

func BenchmarkEqual(b *testing.B) {
	n1 := RandomNamespace()
	n2 := RandomNamespace()
	// repeat until n2 meets our expectations (n1 != n2).
	for n1.Equals(n2) {
		n2 = RandomNamespace()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if n1.Equals(n2) {
			b.Fatal()
		}
	}
}

func BenchmarkCompare(b *testing.B) {
	n1 := RandomNamespace()
	n2 := RandomNamespace()
	// repeat until n2 meets our expectations (n1 > n2).
	for n1.Compare(n2) != 1 {
		n2 = RandomNamespace()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if n1.Compare(n2) != 1 {
			b.Fatal()
		}
	}
}

func TestValidateForData(t *testing.T) {
	valid := MustNewNamespace(NamespaceVersionZero, validID)
	invalid := Namespace{
		data: []byte{NamespaceVersionZero, 0xFF}, // invalid length
	}

	type testCase struct {
		namespace Namespace
		wantErr   error
	}
	testCases := []testCase{
		{
			namespace: valid,
			wantErr:   nil,
		},
		{
			namespace: ParitySharesNamespace,
			wantErr:   fmt.Errorf("invalid data namespace(%s): parity and tail padding namespace are forbidden", ParitySharesNamespace),
		},
		{
			namespace: TailPaddingNamespace,
			wantErr:   fmt.Errorf("invalid data namespace(%s): parity and tail padding namespace are forbidden", TailPaddingNamespace),
		},
		{
			namespace: invalid,
			wantErr:   fmt.Errorf("unsupported namespace id length: id [255] must be 28 bytes but it was 1 bytes"),
		},
	}

	for _, tc := range testCases {
		err := tc.namespace.ValidateForData()
		assert.Equal(t, tc.wantErr, err)
	}
}

func TestValidateForBlob(t *testing.T) {
	valid := MustNewNamespace(NamespaceVersionZero, validID)
	invalidLength := Namespace{
		data: []byte{NamespaceVersionZero, 0xFF}, // invalid length
	}
	invalidVersion := newNamespace(uint8(1), bytes.Repeat([]byte{0x00}, NamespaceIDSize))

	type testCase struct {
		namespace Namespace
		wantErr   error
	}
	testCases := []testCase{
		{
			namespace: valid,
			wantErr:   nil,
		},
		{
			namespace: ParitySharesNamespace,
			wantErr:   fmt.Errorf("invalid data namespace(%s): parity and tail padding namespace are forbidden", ParitySharesNamespace),
		},
		{
			namespace: TailPaddingNamespace,
			wantErr:   fmt.Errorf("invalid data namespace(%s): parity and tail padding namespace are forbidden", TailPaddingNamespace),
		},
		{
			namespace: invalidLength,
			wantErr:   fmt.Errorf("unsupported namespace id length: id [255] must be 28 bytes but it was 1 bytes"),
		},
		{
			namespace: TxNamespace, // reserved namespace
			wantErr:   fmt.Errorf("invalid data namespace(0000000000000000000000000000000000000000000000000000000001): reserved data is forbidden"),
		},
		{
			namespace: PayForBlobNamespace, // reserved namespace
			wantErr:   fmt.Errorf("invalid data namespace(0000000000000000000000000000000000000000000000000000000004): reserved data is forbidden"),
		},
		{
			namespace: PayForFibreNamespace, // reserved namespace
			wantErr:   fmt.Errorf("invalid data namespace(0000000000000000000000000000000000000000000000000000000005): reserved data is forbidden"),
		},
		{
			namespace: invalidVersion,
			wantErr:   fmt.Errorf("unsupported namespace version 1"),
		},
	}

	for _, tc := range testCases {
		err := tc.namespace.ValidateForBlob()
		assert.Equal(t, tc.wantErr, err)
	}
}

func TestPayForFibreNamespace(t *testing.T) {
	t.Run("Bytes", func(t *testing.T) {
		want := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05}
		got := PayForFibreNamespace.Bytes()
		assert.Equal(t, want, got)
	})
	t.Run("IsPayForFibre", func(t *testing.T) {
		assert.True(t, PayForFibreNamespace.IsPayForFibre())
		assert.False(t, PayForBlobNamespace.IsPayForFibre())
		assert.False(t, TxNamespace.IsPayForFibre())
		assert.False(t, MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize)).IsPayForFibre())
	})
}
