package share

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
