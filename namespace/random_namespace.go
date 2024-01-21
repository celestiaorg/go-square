package namespace

import "crypto/rand"

func RandomNamespace() Namespace {
	for {
		id := RandomVerzionZeroID()
		namespace, err := New(NamespaceVersionZero, id)
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
