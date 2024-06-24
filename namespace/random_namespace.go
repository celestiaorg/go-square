package namespace

import (
	"crypto/rand"
	"slices"
)

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
		namespace := MustNewV0(id)
		if isBlobNamespace(namespace) {
			return namespace
		}
	}
}

// isBlobNamespace returns an true if this namespace is a valid user-specifiable
// blob namespace.
func isBlobNamespace(ns Namespace) bool {
	if ns.IsReserved() {
		return false
	}

	if !slices.Contains(SupportedBlobNamespaceVersions, ns.Version()) {
		return false
	}

	return true
}
