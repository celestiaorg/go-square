package share

import (
	"crypto/rand"
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
		if err := namespace.ValidateForBlob(); err == nil {
			return namespace
		}
	}
}
