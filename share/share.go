/*
Package share is an encoding and decoding protocol that takes blobs,
a struct containing arbitrary data based on a namespace and coverts
them into a slice of shares, bytes 512 in length. This logic is used
for constructing the original data square.
*/
package share

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// Share contains the raw share data (including namespace ID).
type Share struct {
	data []byte
}

func (s Share) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.data)
}

func (s *Share) UnmarshalJSON(data []byte) error {
	var buf []byte

	if err := json.Unmarshal(data, &buf); err != nil {
		return err
	}
	s.data = buf
	return validateSize(s.data)
}

// NewShare creates a new share from the raw data, validating it's
// size and versioning
func NewShare(data []byte) (*Share, error) {
	if err := validateSize(data); err != nil {
		return nil, err
	}
	return &Share{data}, nil
}

func validateSize(data []byte) error {
	if len(data) != ShareSize {
		return fmt.Errorf("share data must be %d bytes, got %d", ShareSize, len(data))
	}
	return nil
}

// Namespace returns the shares namespace
func (s *Share) Namespace() Namespace {
	return Namespace{data: s.data[:NamespaceSize]}
}

// InfoByte returns the byte after the namespace used
// for indicating versioning and whether the share is
// the first in it's sequence or a continuation
func (s *Share) InfoByte() InfoByte {
	return InfoByte(s.data[NamespaceSize])
}

// Version returns the version of the share
func (s *Share) Version() uint8 {
	return s.InfoByte().Version()
}

// CheckVersionSupported checks if the share version is supported
func (s *Share) CheckVersionSupported() error {
	ver := s.Version()
	if !bytes.Contains(SupportedShareVersions, []byte{ver}) {
		return fmt.Errorf("unsupported share version %v is not present in the list of supported share versions %v", ver, SupportedShareVersions)
	}
	return nil
}

// IsSequenceStart returns true if this is the first share in a sequence.
func (s *Share) IsSequenceStart() bool {
	infoByte := s.InfoByte()
	return infoByte.IsSequenceStart()
}

// IsCompactShare returns true if this is a compact share.
func (s Share) IsCompactShare() bool {
	ns := s.Namespace()
	isCompact := ns.IsTx() || ns.IsPayForBlob()
	return isCompact
}

// GetSigner returns the signer of the share, if the
// share is not of type v1 and is not the first share in a sequence
// it returns nil
func GetSigner(share Share) []byte {
	infoByte := share.InfoByte()
	if infoByte.Version() != ShareVersionOne {
		return nil
	}
	if !infoByte.IsSequenceStart() {
		return nil
	}
	startIndex := NamespaceSize + ShareInfoBytes + SequenceLenBytes
	endIndex := startIndex + SignerSize
	return share.data[startIndex:endIndex]
}

// SequenceLen returns the sequence length of this share.
// It returns 0 if this is a continuation share because then it doesn't contain a sequence length.
func (s *Share) SequenceLen() uint32 {
	if !s.IsSequenceStart() {
		return 0
	}

	start := NamespaceSize + ShareInfoBytes
	end := start + SequenceLenBytes
	return binary.BigEndian.Uint32(s.data[start:end])
}

// IsPadding returns whether this *share is padding or not.
func (s *Share) IsPadding() bool {
	isNamespacePadding := s.isNamespacePadding()
	isTailPadding := s.isTailPadding()
	isPrimaryReservedPadding := s.isPrimaryReservedPadding()
	return isNamespacePadding || isTailPadding || isPrimaryReservedPadding
}

func (s *Share) isNamespacePadding() bool {
	return s.IsSequenceStart() && s.SequenceLen() == 0
}

func (s *Share) isTailPadding() bool {
	ns := s.Namespace()
	return ns.IsTailPadding()
}

func (s *Share) isPrimaryReservedPadding() bool {
	ns := s.Namespace()
	return ns.IsPrimaryReservedPadding()
}

// ToBytes returns the underlying bytes of the share
func (s *Share) ToBytes() []byte {
	return s.data
}

// RawData returns the raw share data. The raw share data does not contain the
// namespace ID, info byte, sequence length and if they exist: the reserved bytes
// and signer.
func (s *Share) RawData() []byte {
	startingIndex := s.rawDataStartIndex()
	return s.data[startingIndex:]
}

func (s *Share) rawDataStartIndex() int {
	isStart := s.IsSequenceStart()
	isCompact := s.IsCompactShare()
	index := NamespaceSize + ShareInfoBytes
	if isStart {
		index += SequenceLenBytes
	}
	if isCompact {
		index += ShareReservedBytes
	}
	if s.Version() == ShareVersionOne {
		index += SignerSize
	}
	return index
}

// RawDataUsingReserved returns the raw share data while taking reserved bytes into account.
func (s *Share) RawDataUsingReserved() (rawData []byte, err error) {
	rawDataStartIndexUsingReserved, err := s.rawDataStartIndexUsingReserved()
	if err != nil {
		return nil, err
	}

	// This means share is the last share and does not have any transaction beginning in it
	if rawDataStartIndexUsingReserved == 0 {
		return []byte{}, nil
	}
	if len(s.data) < rawDataStartIndexUsingReserved {
		return rawData, fmt.Errorf("share %s is too short to contain raw data", s)
	}

	return s.data[rawDataStartIndexUsingReserved:], nil
}

// rawDataStartIndexUsingReserved returns the start index of raw data while accounting for
// reserved bytes, if it exists in the share.
func (s *Share) rawDataStartIndexUsingReserved() (int, error) {
	isStart := s.IsSequenceStart()
	isCompact := s.IsCompactShare()

	index := NamespaceSize + ShareInfoBytes
	if isStart {
		index += SequenceLenBytes
	}
	if s.Version() == ShareVersionOne {
		index += SignerSize
	}

	if isCompact {
		reservedBytes, err := ParseReservedBytes(s.data[index : index+ShareReservedBytes])
		if err != nil {
			return 0, err
		}
		return int(reservedBytes), nil
	}
	return index, nil
}

func ToBytes(shares []Share) (bytes [][]byte) {
	bytes = make([][]byte, len(shares))
	for i, share := range shares {
		bytes[i] = share.data
	}
	return bytes
}

func FromBytes(bytes [][]byte) (shares []Share, err error) {
	for _, b := range bytes {
		share, err := NewShare(b)
		if err != nil {
			return nil, err
		}
		shares = append(shares, *share)
	}
	return shares, nil
}
