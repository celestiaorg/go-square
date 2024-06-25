// Package shares contains the Share data structure.
package shares

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Share contains the raw share data (including namespace ID).
type Share struct {
	data []byte
}

func NewShare(data []byte) (*Share, error) {
	if err := validateSize(data); err != nil {
		return nil, err
	}
	if err := validate(data[0], data[1:NamespaceSize]); err != nil {
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
func (s *Share) Namespace() (Namespace, error) {
	return NewNamespaceFromBytes(s.data[:NamespaceSize])
}

func (s *Share) InfoByte() InfoByte {
	// the info byte is the first byte after the namespace
	return InfoByte(s.data[NamespaceSize])
}

func (s *Share) Version() uint8 {
	return s.InfoByte().Version()
}

func (s *Share) DoesSupportVersions(supportedShareVersions []uint8) error {
	ver := s.Version()
	if !bytes.Contains(supportedShareVersions, []byte{ver}) {
		return fmt.Errorf("unsupported share version %v is not present in the list of supported share versions %v", ver, supportedShareVersions)
	}
	return nil
}

// IsSequenceStart returns true if this is the first share in a sequence.
func (s *Share) IsSequenceStart() bool {
	infoByte := s.InfoByte()
	return infoByte.IsSequenceStart()
}

// IsCompactShare returns true if this is a compact share.
func (s Share) IsCompactShare() (bool, error) {
	ns, err := s.Namespace()
	if err != nil {
		return false, err
	}
	isCompact := ns.IsTx() || ns.IsPayForBlob()
	return isCompact, nil
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
func (s *Share) IsPadding() (bool, error) {
	isNamespacePadding := s.isNamespacePadding()
	isTailPadding, err := s.isTailPadding()
	if err != nil {
		return false, err
	}
	isPrimaryReservedPadding, err := s.isPrimaryReservedPadding()
	if err != nil {
		return false, err
	}
	return isNamespacePadding || isTailPadding || isPrimaryReservedPadding, nil
}

func (s *Share) isNamespacePadding() bool {
	return s.IsSequenceStart() && s.SequenceLen() == 0
}

func (s *Share) isTailPadding() (bool, error) {
	ns, err := s.Namespace()
	if err != nil {
		return false, err
	}
	return ns.IsTailPadding(), nil
}

func (s *Share) isPrimaryReservedPadding() (bool, error) {
	ns, err := s.Namespace()
	if err != nil {
		return false, err
	}
	return ns.IsPrimaryReservedPadding(), nil
}

func (s *Share) ToBytes() []byte {
	return s.data
}

// RawData returns the raw share data. The raw share data does not contain the
// namespace ID, info byte, sequence length and if they exist: the reserved bytes
// and signer.
func (s *Share) RawData() ([]byte, error) {
	startingIndex, err := s.rawDataStartIndex()
	if err != nil {
		return nil, err
	}
	return s.data[startingIndex:], nil
}

func (s *Share) rawDataStartIndex() (int, error) {
	isStart := s.IsSequenceStart()
	isCompact, err := s.IsCompactShare()
	if err != nil {
		return 0, err
	}

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
	return index, nil
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
	isCompact, err := s.IsCompactShare()
	if err != nil {
		return 0, err
	}

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
