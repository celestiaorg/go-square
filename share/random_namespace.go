package share

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
)

func RandomNamespace() Namespace {
	for {
		id := RandomVerzionZeroID()
		namespace, err := NewNamespace(NamespaceVersionZero, id)
		if err != nil {
			continue
		}
		return namespace
	}
}

func RandomVerzionZeroID() []byte {
	namespace := make([]byte, NamespaceVersionZeroIDSize)
	_, err := rand.Read(namespace)
	if err != nil {
		panic(err)
	}
	return append(NamespaceVersionZeroPrefix, namespace...)
}

func RandomBlobNamespaceID() []byte {
	namespace := make([]byte, NamespaceVersionZeroIDSize)
	_, err := rand.Read(namespace)
	if err != nil {
		panic(err)
	}
	return namespace
}

func RandomBlobNamespace() Namespace {
	for {
		id := RandomBlobNamespaceID()
		namespace := MustNewV0Namespace(id)
		if IsBlobNamespace(namespace) {
			return namespace
		}
	}
}

// AddInt adds arbitrary int value to namespace, treating namespace as big-endian
// implementation of int
func AddInt(n Namespace, val int) (Namespace, error) {
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
