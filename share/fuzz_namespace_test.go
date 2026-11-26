package share_test

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/celestiaorg/go-square/v4/share"
	"github.com/stretchr/testify/require"
)

// FuzzNamespace checks the namespace constructors and validators against an
// independent statement of the namespace rules. Namespaces arrive from users
// (PFB and Fibre messages, JSON blobs) and from the network (shwap IDs, row
// roots), so every constructor must reject what the rules reject, without
// panicking.
func FuzzNamespace(f *testing.F) {
	zeroPrefix := make([]byte, share.NamespaceVersionZeroPrefixSize)
	for _, ns := range []share.Namespace{
		share.TxNamespace, share.PayForBlobNamespace, share.PayForFibreNamespace,
		share.PrimaryReservedPaddingNamespace, share.MaxPrimaryReservedNamespace,
		share.MinSecondaryReservedNamespace, share.TailPaddingNamespace, share.ParitySharesNamespace,
		share.MustNewV0Namespace([]byte{1}), share.MustNewV0Namespace(bytes.Repeat([]byte{0xff}, 10)),
	} {
		f.Add(ns.Version(), ns.ID(), share.TxNamespace.Bytes(), int64(1))
		f.Add(ns.Version(), ns.ID(), ns.Bytes(), int64(-1))
	}
	f.Add(uint8(0), append(bytes.Clone(zeroPrefix), make([]byte, 9)...), []byte{}, int64(0))
	f.Add(uint8(0), append([]byte{1}, make([]byte, 27)...), []byte{0}, int64(-1<<63))
	f.Add(uint8(1), make([]byte, share.NamespaceIDSize), []byte("{}"), int64(1<<62))
	f.Add(uint8(255), make([]byte, share.NamespaceIDSize+1), []byte(`"AA=="`), int64(256))

	f.Fuzz(func(t *testing.T, version uint8, id, other []byte, delta int64) {
		if len(id) > 2*share.NamespaceSize || len(other) > 4*share.NamespaceSize {
			t.Skip()
		}
		idCopy, otherCopy := bytes.Clone(id), bytes.Clone(other)
		raw := append([]byte{version}, id...)
		wantValid := validNamespace(raw)

		ns, err := share.NewNamespace(version, id)
		require.Equal(t, wantValid, err == nil, "NewNamespace(%d, %x): %v", version, id, err)
		fromBytes, err := share.NewNamespaceFromBytes(bytes.Clone(raw))
		require.Equal(t, wantValid, err == nil, "NewNamespaceFromBytes(%x): %v", raw, err)
		var fromJSON share.Namespace
		encoded, err := json.Marshal(raw)
		require.NoError(t, err)
		err = fromJSON.UnmarshalJSON(encoded)
		require.Equal(t, wantValid, err == nil, "UnmarshalJSON(%s): %v", encoded, err)
		// Malformed JSON must also be rejected without a panic.
		var junk share.Namespace
		_ = junk.UnmarshalJSON(other)
		require.Equal(t, idCopy, id, "constructors must not modify their input")
		require.Equal(t, otherCopy, other)
		if len(id) <= share.NamespaceVersionZeroIDSize {
			v0, err := share.NewV0Namespace(id)
			require.NoError(t, err, "every short subID is a valid v0 namespace")
			require.Equal(t, uint8(0), v0.Version())
			require.Equal(t, append(make([]byte, share.NamespaceSize-len(id)), id...), v0.Bytes())
		} else {
			_, err := share.NewV0Namespace(id)
			require.Error(t, err)
		}
		if !wantValid {
			return
		}

		for _, n := range []share.Namespace{ns, fromBytes, fromJSON} {
			require.Equal(t, raw, n.Bytes())
			require.Equal(t, version, n.Version())
			require.Equal(t, id, n.ID())
			require.False(t, n.IsEmpty())
			require.True(t, n.Equals(ns))
		}
		roundTrip, err := ns.MarshalJSON()
		require.NoError(t, err)
		var decoded share.Namespace
		require.NoError(t, decoded.UnmarshalJSON(roundTrip))
		require.True(t, decoded.Equals(ns))

		checkNamespaceClassification(t, ns)
		if o, err := share.NewNamespaceFromBytes(other); err == nil {
			checkNamespaceOrder(t, ns, o)
		}
		checkNamespaceOrder(t, ns, share.TxNamespace)
		checkNamespaceOrder(t, ns, share.TailPaddingNamespace)
		checkAddInt(t, ns, int(delta))
	})
}

// validNamespace states the namespace rules independently of share.Namespace:
// 29 bytes, version 0 with 18 leading zero ID bytes, or version 255.
func validNamespace(raw []byte) bool {
	if len(raw) != share.NamespaceSize {
		return false
	}
	switch raw[0] {
	case 0:
		return bytes.Equal(raw[1:1+share.NamespaceVersionZeroPrefixSize], make([]byte, share.NamespaceVersionZeroPrefixSize))
	case 255:
		return true
	default:
		return false
	}
}

func checkNamespaceClassification(t *testing.T, ns share.Namespace) {
	t.Helper()
	raw := ns.Bytes()
	maxPrimary := append(make([]byte, share.NamespaceSize-1), 0xff)
	minSecondary := append(bytes.Repeat([]byte{0xff}, share.NamespaceSize-1), 0x00)
	primary := bytes.Compare(raw, maxPrimary) <= 0
	secondary := bytes.Compare(raw, minSecondary) >= 0
	parity := bytes.Equal(raw, bytes.Repeat([]byte{0xff}, share.NamespaceSize))
	tail := bytes.Equal(raw, append(bytes.Repeat([]byte{0xff}, share.NamespaceSize-1), 0xfe))

	require.Equal(t, primary, ns.IsPrimaryReserved())
	require.Equal(t, secondary, ns.IsSecondaryReserved())
	require.Equal(t, primary || secondary, ns.IsReserved())
	require.Equal(t, parity, ns.IsParityShares())
	require.Equal(t, tail, ns.IsTailPadding())
	require.Equal(t, !parity && !tail, ns.IsUsableNamespace())
	require.Equal(t, !parity && !tail, ns.ValidateForData() == nil)
	wantBlob := !primary && !secondary && raw[0] == share.NamespaceVersionZero
	require.Equal(t, wantBlob, ns.ValidateForBlob() == nil, "ValidateForBlob(%x)", raw)
	for _, special := range []struct {
		got  bool
		want share.Namespace
	}{
		{ns.IsTx(), share.TxNamespace},
		{ns.IsPayForBlob(), share.PayForBlobNamespace},
		{ns.IsPayForFibre(), share.PayForFibreNamespace},
		{ns.IsPrimaryReservedPadding(), share.PrimaryReservedPaddingNamespace},
	} {
		require.Equal(t, bytes.Equal(raw, special.want.Bytes()), special.got)
	}
}

func checkNamespaceOrder(t *testing.T, a, b share.Namespace) {
	t.Helper()
	c := bytes.Compare(a.Bytes(), b.Bytes())
	require.Equal(t, c, a.Compare(b))
	require.Equal(t, -c, b.Compare(a))
	require.Equal(t, c == 0, a.Equals(b))
	require.Equal(t, c < 0, a.IsLessThan(b))
	require.Equal(t, c <= 0, a.IsLessOrEqualThan(b))
	require.Equal(t, c > 0, a.IsGreaterThan(b))
	require.Equal(t, c >= 0, a.IsGreaterOrEqualThan(b))
}

// checkAddInt compares AddInt with big-integer arithmetic on the 29-byte
// big-endian value: it fails exactly when the sum leaves [0, 2^232).
func checkAddInt(t *testing.T, ns share.Namespace, delta int) {
	t.Helper()
	before := bytes.Clone(ns.Bytes())
	got, err := ns.AddInt(delta)
	require.Equal(t, before, ns.Bytes(), "AddInt must not modify the receiver")

	sum := new(big.Int).SetBytes(before)
	sum.Add(sum, big.NewInt(int64(delta)))
	limit := new(big.Int).Lsh(big.NewInt(1), 8*share.NamespaceSize)
	if sum.Sign() < 0 || sum.Cmp(limit) >= 0 {
		require.Error(t, err, "AddInt(%x, %d) should overflow", before, delta)
		return
	}
	require.NoError(t, err)
	want := sum.FillBytes(make([]byte, share.NamespaceSize))
	require.Equal(t, want, got.Bytes(), "AddInt(%x, %d)", before, delta)
}

func TestNamespaceRepeatCopies(t *testing.T) {
	ns := share.MustNewV0Namespace([]byte{1, 2, 3})
	copies := ns.Repeat(3)
	require.Len(t, copies, 3)
	copies[0].Bytes()[share.NamespaceSize-1] ^= 0xff
	require.True(t, copies[1].Equals(ns), "Repeat must return independent copies")
	require.True(t, ns.Equals(share.MustNewV0Namespace([]byte{1, 2, 3})))
	require.Empty(t, ns.Repeat(0))
}

func TestMustNewNamespacePanicsOnInvalidInput(t *testing.T) {
	require.Panics(t, func() { share.MustNewNamespace(1, make([]byte, share.NamespaceIDSize)) })
	require.Panics(t, func() { share.MustNewV0Namespace(make([]byte, share.NamespaceVersionZeroIDSize+1)) })
	require.NotPanics(t, func() { share.MustNewNamespace(255, make([]byte, share.NamespaceIDSize)) })
}
