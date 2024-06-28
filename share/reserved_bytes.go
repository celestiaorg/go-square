package share

import (
	"encoding/binary"
	"fmt"
)

// NewReservedBytes returns a byte slice of length
// ShareReservedBytes that contains the byteIndex of the first
// unit that starts in a compact share.
func NewReservedBytes(byteIndex uint32) ([]byte, error) {
	if byteIndex >= ShareSize {
		return []byte{}, fmt.Errorf("byte index %d must be less than share size %d", byteIndex, ShareSize)
	}
	reservedBytes := make([]byte, ShareReservedBytes)
	binary.BigEndian.PutUint32(reservedBytes, byteIndex)
	return reservedBytes, nil
}

// ParseReservedBytes parses a byte slice of length
// ShareReservedBytes into a byteIndex.
func ParseReservedBytes(reservedBytes []byte) (uint32, error) {
	if len(reservedBytes) != ShareReservedBytes {
		return 0, fmt.Errorf("reserved bytes must be of length %d", ShareReservedBytes)
	}
	byteIndex := binary.BigEndian.Uint32(reservedBytes)
	if ShareSize <= byteIndex {
		return 0, fmt.Errorf("byteIndex must be less than share size %d", ShareSize)
	}
	return byteIndex, nil
}
