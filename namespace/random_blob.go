package namespace

import (
	"crypto/rand"

	"golang.org/x/exp/slices"
)

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
