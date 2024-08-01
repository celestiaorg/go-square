package share

import (
	"bytes"
	"fmt"
)

type Namespace struct {
	data []byte
}

// NewNamespace validates the provided version and id and returns a new namespace.
// This should be used for user specified namespaces.
func NewNamespace(version uint8, id []byte) (Namespace, error) {
	if err := ValidateUserNamespace(version, id); err != nil {
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

// MustNewNamespace returns a new namespace with the provided version and id. It panics
// if the provided version or id are not supported.
func MustNewNamespace(version uint8, id []byte) Namespace {
	ns, err := NewNamespace(version, id)
	if err != nil {
		panic(err)
	}
	return ns
}

// NewNamespaceFromBytes returns a new namespace from the provided byte slice.
// This is for user specified namespaces.
func NewNamespaceFromBytes(bytes []byte) (Namespace, error) {
	if len(bytes) != NamespaceSize {
		return Namespace{}, fmt.Errorf("invalid namespace length: %d. Must be %d bytes", len(bytes), NamespaceSize)
	}
	if err := ValidateUserNamespace(bytes[VersionIndex], bytes[NamespaceVersionSize:]); err != nil {
		return Namespace{}, err
	}

	return Namespace{
		data: bytes,
	}, nil
}

// NewV0Namespace returns a new namespace with version 0 and the provided subID. subID
// must be <= 10 bytes. If subID is < 10 bytes, it will be left-padded with 0s
// to fill 10 bytes.
func NewV0Namespace(subID []byte) (Namespace, error) {
	if lenSubID := len(subID); lenSubID > NamespaceVersionZeroIDSize {
		return Namespace{}, fmt.Errorf("subID must be <= %v, but it was %v bytes", NamespaceVersionZeroIDSize, lenSubID)
	}

	namespace := make([]byte, NamespaceSize)
	copy(namespace[NamespaceSize-len(subID):], subID)

	return NewNamespaceFromBytes(namespace)
}

// MustNewV0Namespace returns a new namespace with version 0 and the provided subID. This
// function panics if the provided subID would result in an invalid namespace.
func MustNewV0Namespace(subID []byte) Namespace {
	ns, err := NewV0Namespace(subID)
	if err != nil {
		panic(err)
	}
	return ns
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

// ValidateUserNamespace returns an error if the provided version is not
// supported or the provided id does not meet the requirements
// for the provided version. This should be used for validating
// user specified namespaces
func ValidateUserNamespace(version uint8, id []byte) error {
	err := validateVersionSupported(version)
	if err != nil {
		return err
	}
	return validateID(version, id)
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

// IsEmpty returns true if the namespace is empty
func (n Namespace) IsEmpty() bool {
	return len(n.data) == 0
}

// IsReserved returns true if the namespace is reserved
// for the Celestia state machine
func (n Namespace) IsReserved() bool {
	return n.IsPrimaryReserved() || n.IsSecondaryReserved()
}

func (n Namespace) IsPrimaryReserved() bool {
	return n.IsLessOrEqualThan(MaxPrimaryReservedNamespace)
}

func (n Namespace) IsSecondaryReserved() bool {
	return n.IsGreaterOrEqualThan(MinSecondaryReservedNamespace)
}

// IsUsableNamespace refers to the range of namespaces that are
// not reserved by the square protocol i.e. not parity shares or
// tail padding
func (n Namespace) IsUsableNamespace() bool {
	return !n.IsParityShares() && !n.IsTailPadding()
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
