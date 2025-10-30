package share

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPayForFibreNamespace(t *testing.T) {
	// Verify PayForFibreNamespace has the correct value: 0x0000000000000000000000000000000000000000000000000000000005
	expectedBytes := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05}
	actualBytes := PayForFibreNamespace.Bytes()
	assert.Equal(t, expectedBytes, actualBytes, "PayForFibreNamespace should be 0x0000000000000000000000000000000000000000000000000000000005")
}

func TestIsPayForFibre(t *testing.T) {
	type testCase struct {
		ns   Namespace
		want bool
	}
	testCases := []testCase{
		{
			ns:   PayForFibreNamespace,
			want: true,
		},
		{
			ns:   PayForBlobNamespace,
			want: false,
		},
		{
			ns:   TxNamespace,
			want: false,
		},
		{
			ns:   MustNewV0Namespace(bytes.Repeat([]byte{1}, NamespaceVersionZeroIDSize)),
			want: false,
		},
	}

	for _, tc := range testCases {
		got := tc.ns.IsPayForFibre()
		assert.Equal(t, tc.want, got, "IsPayForFibre() for namespace %x", tc.ns.Bytes())
	}
}
