package inclusion

import (
	"crypto/sha256"

	"github.com/celestiaorg/go-square/blob"
	ns "github.com/celestiaorg/go-square/namespace"
	sh "github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/nmt"
)

type MerkleRootFn func([][]byte) []byte

// CreateCommitment generates the share commitment for a given blob.
// See [data square layout rationale] and [blob share commitment rules].
//
// [data square layout rationale]: ../../specs/src/specs/data_square_layout.md
// [blob share commitment rules]: ../../specs/src/specs/data_square_layout.md#blob-share-commitment-rules
func CreateCommitment(blob *blob.Blob, merkleRootFn MerkleRootFn, subtreeRootThreshold int) ([]byte, error) {
	if err := blob.Validate(); err != nil {
		return nil, err
	}
	namespace := blob.Namespace()

	shares, err := sh.SplitBlobs(blob)
	if err != nil {
		return nil, err
	}

	// the commitment is the root of a merkle mountain range with max tree size
	// determined by the number of roots required to create a share commitment
	// over that blob. The size of the tree is only increased if the number of
	// subtree roots surpasses a constant threshold.
	subTreeWidth := SubTreeWidth(len(shares), subtreeRootThreshold)
	treeSizes, err := MerkleMountainRangeSizes(uint64(len(shares)), uint64(subTreeWidth))
	if err != nil {
		return nil, err
	}
	leafSets := make([][][]byte, len(treeSizes))
	cursor := uint64(0)
	for i, treeSize := range treeSizes {
		leafSets[i] = sh.ToBytes(shares[cursor : cursor+treeSize])
		cursor += treeSize
	}

	// create the commitments by pushing each leaf set onto an NMT
	subTreeRoots := make([][]byte, len(leafSets))
	for i, set := range leafSets {
		// Create the NMT. TODO: use NMT wrapper.
		tree := nmt.New(sha256.New(), nmt.NamespaceIDSize(ns.NamespaceSize), nmt.IgnoreMaxNamespace(true))
		for _, leaf := range set {
			// the namespace must be added again here even though it is already
			// included in the leaf to ensure that the hash will match that of
			// the NMT wrapper (pkg/wrapper). Each namespace is added to keep
			// the namespace in the share, and therefore the parity data, while
			// also allowing for the manual addition of the parity namespace to
			// the parity data.
			nsLeaf := make([]byte, 0)
			nsLeaf = append(nsLeaf, namespace.Bytes()...)
			nsLeaf = append(nsLeaf, leaf...)

			err = tree.Push(nsLeaf)
			if err != nil {
				return nil, err
			}
		}
		// add the root
		root, err := tree.Root()
		if err != nil {
			return nil, err
		}
		subTreeRoots[i] = root
	}
	return merkleRootFn(subTreeRoots), nil
}

func CreateCommitments(blobs []*blob.Blob, merkleRootFn MerkleRootFn, subtreeRootThreshold int) ([][]byte, error) {
	commitments := make([][]byte, len(blobs))
	for i, blob := range blobs {
		commitment, err := CreateCommitment(blob, merkleRootFn, subtreeRootThreshold)
		if err != nil {
			return nil, err
		}
		commitments[i] = commitment
	}
	return commitments, nil
}

// MerkleMountainRangeSizes returns the sizes (number of leaf nodes) of the
// trees in a merkle mountain range constructed for a given totalSize and
// maxTreeSize.
//
// https://docs.grin.mw/wiki/chain-state/merkle-mountain-range/
// https://github.com/opentimestamps/opentimestamps-server/blob/master/doc/merkle-mountain-range.md
func MerkleMountainRangeSizes(totalSize, maxTreeSize uint64) ([]uint64, error) {
	var treeSizes []uint64

	for totalSize != 0 {
		switch {
		case totalSize >= maxTreeSize:
			treeSizes = append(treeSizes, maxTreeSize)
			totalSize -= maxTreeSize
		case totalSize < maxTreeSize:
			treeSize, err := sh.RoundDownPowerOfTwo(totalSize)
			if err != nil {
				return treeSizes, err
			}
			treeSizes = append(treeSizes, treeSize)
			totalSize -= treeSize
		}
	}

	return treeSizes, nil
}
