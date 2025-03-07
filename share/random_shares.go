package share

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"sort"
)

// RandShares generates total amount of shares and fills them with random data.
func RandShares(total int) ([]Share, error) {
	if total&(total-1) != 0 {
		return nil, fmt.Errorf("total must be power of 2: %d", total)
	}

	shares := make([]Share, total)
	for i := range shares {
		shr := make([]byte, ShareSize)
		copy(shr[:NamespaceSize], RandomNamespace().Bytes())
		if _, err := rand.Read(shr[NamespaceSize:]); err != nil {
			panic(err)
		}

		sh, err := NewShare(shr)
		if err != nil {
			panic(err)
		}
		if err = sh.Namespace().ValidateForData(); err != nil {
			panic(err)
		}

		shares[i] = *sh
	}
	sort.Slice(shares, func(i, j int) bool { return bytes.Compare(shares[i].ToBytes(), shares[j].ToBytes()) < 0 })
	return shares, nil
}

// RandSharesWithNamespace is the same as RandShares, but sets the same namespace for all shares.
func RandSharesWithNamespace(namespace Namespace, namespacedAmount, total int) ([]Share, error) {
	if total&(total-1) != 0 {
		return nil, fmt.Errorf("total must be power of 2: %d", total)
	}

	if namespacedAmount > total {
		return nil,
			fmt.Errorf("namespacedAmount %v must be less than or equal to total: %v", namespacedAmount, total)
	}

	shares := make([]Share, total)
	for i := range shares {
		shr := make([]byte, ShareSize)
		if i < namespacedAmount {
			copy(shr[:NamespaceSize], namespace.Bytes())
		} else {
			copy(shr[:NamespaceSize], RandomNamespace().Bytes())
		}
		_, err := rand.Read(shr[NamespaceSize:])
		if err != nil {
			panic(err)
		}

		sh, err := NewShare(shr)
		if err != nil {
			panic(err)
		}
		shares[i] = *sh
	}
	sort.Slice(shares, func(i, j int) bool { return bytes.Compare(shares[i].ToBytes(), shares[j].ToBytes()) < 0 })
	return shares, nil
}
