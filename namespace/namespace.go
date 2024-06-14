// Package namespace contains the Namespace data structure.
package namespace

import (
	"bytes"
	"fmt"
)

type Namespace struct {
	data []byte
}

// New returns a new namespace with the provided version and id.
func New(version uint8, id []byte) (Namespace, error) {
	err := validateVersionSupported(version)
	if err != nil {
		return Namespace{}, err
	}

	err = validateID(version, id)
	if err != nil {
		return Namespace{}, err
	}

	return newNamespace(version, id), nil
}

func newNamespace(version uint8, id []byte) Namespace {
	data := make([]byte, NamespaceVersionSize+len(id))
	data[VersionIndex] = version
	copy(data[NamespaceVersionSize:], id)
	return Namespace{
		data: data,
	}
}

// MustNew returns a new namespace with the provided version and id. It panics
// if the provided version or id are not supported.
func MustNew(version uint8, id []byte) Namespace {
	ns, err := New(version, id)
	if err != nil {
		panic(err)
	}
	return ns
}

// NewFromBytes returns a new namespace from the provided byte slice.
func NewFromBytes(bytes []byte) (Namespace, error) {
	if len(bytes) != NamespaceSize {
		return Namespace{}, fmt.Errorf("invalid namespace length: %v must be %v", len(bytes), NamespaceSize)
	}

	err := validateVersionSupported(bytes[VersionIndex])
	if err != nil {
		return Namespace{}, err
	}

	err = validateID(bytes[VersionIndex], bytes[NamespaceVersionSize:])
	if err != nil {
		return Namespace{}, err
	}

	return Namespace{
		data: bytes,
	}, nil
}

// NewV0 returns a new namespace with version 0 and the provided subID. subID
// must be <= 10 bytes. If subID is < 10 bytes, it will be left-padded with 0s
// to fill 10 bytes.
func NewV0(subID []byte) (Namespace, error) {
	if lenSubID := len(subID); lenSubID > NamespaceVersionZeroIDSize {
		return Namespace{}, fmt.Errorf("subID must be <= %v, but it was %v bytes", NamespaceVersionZeroIDSize, lenSubID)
	}

	namespace := make([]byte, NamespaceSize)
	copy(namespace[NamespaceSize-len(subID):], subID)

	return NewFromBytes(namespace)
}

// MustNewV0 returns a new namespace with version 0 and the provided subID. This
// function panics if the provided subID would result in an invalid namespace.
func MustNewV0(subID []byte) Namespace {
	ns, err := NewV0(subID)
	if err != nil {
		panic(err)
	}
	return ns
}

// From returns a namespace from the provided byte slice.
// Deprecated: Please use NewFromBytes instead.
func From(b []byte) (Namespace, error) {
	return NewFromBytes(b)
}

// Bytes returns this namespace as a byte slice.
func (n Namespace) Bytes() []byte {
	return n.data
}

// Version return this namespace's version
func (n Namespace) Version() uint8 {
	return n.data[VersionIndex]
}

// ID returns this namespace's ID
func (n Namespace) ID() []byte {
	return n.data[NamespaceVersionSize:]
}

// validateVersionSupported returns an error if the version is not supported.
func validateVersionSupported(version uint8) error {
	if version != NamespaceVersionZero && version != NamespaceVersionMax {
		return fmt.Errorf("unsupported namespace version %v", version)
	}
	return nil
}

// validateID returns an error if the provided id does not meet the requirements
// for the provided version.
func validateID(version uint8, id []byte) error {
	if len(id) != NamespaceIDSize {
		return fmt.Errorf("unsupported namespace id length: id %v must be %v bytes but it was %v bytes", id, NamespaceIDSize, len(id))
	}

	if version == NamespaceVersionZero && !bytes.HasPrefix(id, NamespaceVersionZeroPrefix) {
		return fmt.Errorf("unsupported namespace id with version %v. ID %v must start with %v leading zeros", version, id, len(NamespaceVersionZeroPrefix))
	}
	return nil
}

// IsReserved returns true if the namespace is reserved for protocol-use.
func (n Namespace) IsReserved() bool {
	return n.IsPrimaryReserved() || n.IsSecondaryReserved()
}

func (n Namespace) IsPrimaryReserved() bool {
	return n.IsLessOrEqualThan(MaxPrimaryReservedNamespace)
}

func (n Namespace) IsSecondaryReserved() bool {
	return n.IsGreaterOrEqualThan(MinSecondaryReservedNamespace)
}

func (n Namespace) IsParityShares() bool {
	return n.Equals(ParitySharesNamespace)
}

func (n Namespace) IsTailPadding() bool {
	return n.Equals(TailPaddingNamespace)
}

func (n Namespace) IsPrimaryReservedPadding() bool {
	return n.Equals(PrimaryReservedPaddingNamespace)
}

func (n Namespace) IsTx() bool {
	return n.Equals(TxNamespace)
}

func (n Namespace) IsPayForBlob() bool {
	return n.Equals(PayForBlobNamespace)
}

func (n Namespace) Repeat(times int) []Namespace {
	ns := make([]Namespace, times)
	for i := 0; i < times; i++ {
		ns[i] = n.deepCopy()
	}
	return ns
}

func (n Namespace) Equals(n2 Namespace) bool {
	return bytes.Equal(n.data, n2.data)
}

func (n Namespace) IsLessThan(n2 Namespace) bool {
	return n.Compare(n2) == -1
}

func (n Namespace) IsLessOrEqualThan(n2 Namespace) bool {
	return n.Compare(n2) < 1
}

func (n Namespace) IsGreaterThan(n2 Namespace) bool {
	return n.Compare(n2) == 1
}

func (n Namespace) IsGreaterOrEqualThan(n2 Namespace) bool {
	return n.Compare(n2) > -1
}

func (n Namespace) Compare(n2 Namespace) int {
	return bytes.Compare(n.data, n2.data)
}

// leftPad returns a new byte slice with the provided byte slice left-padded to the provided size.
// If the provided byte slice is already larger than the provided size, the original byte slice is returned.
func leftPad(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	pad := make([]byte, size-len(b))
	return append(pad, b...)
}

// deepCopy returns a deep copy of the Namespace object.
func (n Namespace) deepCopy() Namespace {
	// Create a deep copy of the ID slice
	copyData := make([]byte, len(n.data))
	copy(copyData, n.data)

	return Namespace{
		data: copyData,
	}
}
