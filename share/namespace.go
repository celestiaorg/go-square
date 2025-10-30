package share

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

type Namespace struct {
	data []byte
}

// MarshalJSON encodes namespace to the json encoded bytes.
func (n Namespace) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.data)
}

// UnmarshalJSON decodes json bytes to the namespace.
func (n *Namespace) UnmarshalJSON(data []byte) error {
	var buf []byte
	if err := json.Unmarshal(data, &buf); err != nil {
		return err
	}

	ns, err := NewNamespaceFromBytes(buf)
	if err != nil {
		return err
	}
	*n = ns
	return nil
}

// NewNamespace validates the provided version and id and returns a new namespace.
// This should be used for user specified namespaces.
func NewNamespace(version uint8, id []byte) (Namespace, error) {
	ns := newNamespace(version, id)
	if err := ns.validate(); err != nil {
		return Namespace{}, err
	}
	return ns, nil
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

	ns := Namespace{data: bytes}
	if err := ns.validate(); err != nil {
		return Namespace{}, err
	}
	return ns, nil
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

// String stringifies the Namespace.
func (n Namespace) String() string {
	return hex.EncodeToString(n.data)
}

// validate returns an error if the provided version is not
// supported or the provided id does not meet the requirements
// for the provided version. This should be used for validating
// user specified namespaces
func (n Namespace) validate() error {
	err := n.validateVersionSupported()
	if err != nil {
		return err
	}
	return n.validateID()
}

// ValidateForData checks if the Namespace is of real/useful data.
func (n Namespace) ValidateForData() error {
	if err := n.validate(); err != nil {
		return err
	}
	if !n.IsUsableNamespace() {
		return fmt.Errorf("invalid data namespace(%s): parity and tail padding namespace are forbidden", n)
	}
	return nil
}

// ValidateForBlob verifies whether the Namespace is appropriate for blob data.
// A valid blob namespace must meet two conditions: it cannot be reserved for special purposes,
// and its version must be supported by the system. If either of these conditions is not met,
// an error is returned indicating the issue. This ensures that only valid namespaces are
// used when dealing with blob data.
func (n Namespace) ValidateForBlob() error {
	if err := n.ValidateForData(); err != nil {
		return err
	}

	if n.IsReserved() {
		return fmt.Errorf("invalid data namespace(%s): reserved data is forbidden", n)
	}

	if !slices.Contains(SupportedBlobNamespaceVersions, n.Version()) {
		return fmt.Errorf("blob version %d is not supported", n.Version())
	}
	return nil
}

// validateVersionSupported returns an error if the version is not supported.
func (n Namespace) validateVersionSupported() error {
	if n.Version() != NamespaceVersionZero && n.Version() != NamespaceVersionMax {
		return fmt.Errorf("unsupported namespace version %v", n.Version())
	}
	return nil
}

// validateID returns an error if the provided id does not meet the requirements
// for the provided version.
func (n Namespace) validateID() error {
	if len(n.ID()) != NamespaceIDSize {
		return fmt.Errorf("unsupported namespace id length: id %v must be %v bytes but it was %v bytes", n.ID(), NamespaceIDSize, len(n.ID()))
	}

	if n.Version() == NamespaceVersionZero && !bytes.HasPrefix(n.ID(), NamespaceVersionZeroPrefix) {
		return fmt.Errorf("unsupported namespace id with version %v. ID %v must start with %v leading zeros", n.Version(), n.ID(), len(NamespaceVersionZeroPrefix))
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

func (n Namespace) IsPayForFibre() bool {
	return n.Equals(PayForFibreNamespace)
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

// AddInt adds arbitrary int value to namespace, treating namespace as big-endian
// implementation of int. It could be helpful for users to create adjacent namespaces.
func (n Namespace) AddInt(val int) (Namespace, error) {
	if val == 0 {
		return n, nil
	}
	// Convert the input integer to a byte slice and add it to result slice
	result := make([]byte, NamespaceSize)
	if val > 0 {
		binary.BigEndian.PutUint64(result[NamespaceSize-8:], uint64(val))
	} else {
		binary.BigEndian.PutUint64(result[NamespaceSize-8:], uint64(-val))
	}

	// Perform addition byte by byte
	var carry int
	nn := n.Bytes()
	for i := NamespaceSize - 1; i >= 0; i-- {
		var sum int
		if val > 0 {
			sum = int(nn[i]) + int(result[i]) + carry
		} else {
			sum = int(nn[i]) - int(result[i]) + carry
		}

		switch {
		case sum > 255:
			carry = 1
			sum -= 256
		case sum < 0:
			carry = -1
			sum += 256
		default:
			carry = 0
		}

		result[i] = uint8(sum)
	}

	// Handle any remaining carry
	if carry != 0 {
		return Namespace{}, errors.New("namespace overflow")
	}
	return Namespace{data: result}, nil
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
