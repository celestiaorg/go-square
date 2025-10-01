package inclusion

import (
	"crypto/sha256"
	"errors"
	"fmt"

	sh "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
	"golang.org/x/sync/errgroup"
)

type MerkleRootFn func([][]byte) []byte

// CreateCommitment generates the share commitment for a given blob.
// See [data square layout rationale] and [blob share commitment rules].
//
// [data square layout rationale]: ../../specs/src/specs/data_square_layout.md
// [blob share commitment rules]: ../../specs/src/specs/data_square_layout.md#blob-share-commitment-rules
func CreateCommitment(blob *sh.Blob, merkleRootFn MerkleRootFn, subtreeRootThreshold int) ([]byte, error) {
	subTreeRoots, err := GenerateSubtreeRoots(blob, subtreeRootThreshold)
	if err != nil {
		return nil, err
	}
	return merkleRootFn(subTreeRoots), nil
}

// GenerateSubtreeRoots generates the subtree roots of a blob.
// See [data square layout rationale] and [blob share commitment rules].
//
// [data square layout rationale]: ../../specs/src/specs/data_square_layout.md
// [blob share commitment rules]: ../../specs/src/specs/data_square_layout.md#blob-share-commitment-rules
func GenerateSubtreeRoots(blob *sh.Blob, subtreeRootThreshold int) ([][]byte, error) {
	shares, err := splitBlobs(blob)
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

	namespace := blob.Namespace()
	// create the commitments by pushing each leaf set onto an NMT
	subTreeRoots := make([][]byte, len(leafSets))
	for i, set := range leafSets {
		// Create the NMT. TODO: use NMT wrapper.
		tree := nmt.New(sha256.New(), nmt.NamespaceIDSize(sh.NamespaceSize), nmt.IgnoreMaxNamespace(true))
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
	return subTreeRoots, nil
}

// CreateParallelCommitments generates commitments for multiple blobs in parallel using a pool of NMT instances.
// See docs for CreateCommitment for more details.
func CreateParallelCommitments(blobs []*sh.Blob, merkleRootFn MerkleRootFn, subtreeRootThreshold int, numWorkers int) ([][]byte, error) {
	if len(blobs) == 0 {
		return [][]byte{}, nil
	}
	if numWorkers <= 0 {
		return nil, errors.New("number of workers must be positive")
	}

	// split all blobs into shares in parallel
	blobSharesResults := make([][]sh.Share, len(blobs))
	g := new(errgroup.Group)
	g.SetLimit(numWorkers)

	for i := range blobs {
		idx := i
		g.Go(func() error {
			shares, err := splitBlobs(blobs[idx])
			if err != nil {
				return err
			}
			blobSharesResults[idx] = shares
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("failed to split blob shares: %w", err)
	}

	maxSubtreeSize := 0
	type blobInfo struct {
		shares    []sh.Share
		namespace sh.Namespace
		leafSets  [][][]byte
	}
	blobInfos := make([]blobInfo, len(blobs))

	// calculate the maximum subtree size across all blobs and prepare
	// subtree for parallel calculation using pooled nmts
	for i, blob := range blobs {
		shares := blobSharesResults[i]
		subTreeWidth := SubTreeWidth(len(shares), subtreeRootThreshold)
		treeSizes, err := MerkleMountainRangeSizes(uint64(len(shares)), uint64(subTreeWidth))
		if err != nil {
			return nil, err
		}
		for _, size := range treeSizes {
			if int(size) > maxSubtreeSize {
				maxSubtreeSize = int(size)
			}
		}

		// prepare leaf sets for this blob
		leafSets := make([][][]byte, len(treeSizes))
		cursor := uint64(0)
		for j, treeSize := range treeSizes {
			leafSets[j] = sh.ToBytes(shares[cursor : cursor+treeSize])
			cursor += treeSize
		}
		blobInfos[i] = blobInfo{
			shares:    shares,
			namespace: blob.Namespace(),
			leafSets:  leafSets,
		}
	}

	pool, err := newNMTPool(numWorkers, maxSubtreeSize)
	if err != nil {
		return nil, err
	}

	// process all subtree roots in parallel
	type subtreeResult struct {
		blobIdx int
		treeIdx int
		root    []byte
	}
	totalSubtrees := 0
	for _, info := range blobInfos {
		totalSubtrees += len(info.leafSets)
	}
	resultChan := make(chan subtreeResult, totalSubtrees)
	g = new(errgroup.Group)
	g.SetLimit(numWorkers)

	// queue all subtree computations
	// since go 1.22 there is no need to copy the variables used in loop
	for blobIdx, info := range blobInfos {
		for treeIdx, leafSet := range info.leafSets {
			g.Go(func() error {
				tree := pool.acquire()
				root, err := tree.computeRoot(info.namespace.Bytes(), leafSet)
				resultChan <- subtreeResult{
					blobIdx: blobIdx,
					treeIdx: treeIdx,
					root:    root,
				}
				return err
			})
		}
	}

	if err := g.Wait(); err != nil {
		close(resultChan)
		return nil, err
	}
	close(resultChan)

	// collect results and organize by blob
	subtreeRootsByBlob := make([][][]byte, len(blobs))
	for i, info := range blobInfos {
		subtreeRootsByBlob[i] = make([][]byte, len(info.leafSets))
	}

	for result := range resultChan {
		subtreeRootsByBlob[result.blobIdx][result.treeIdx] = result.root
	}

	// compute final commitments using the merkle root function
	commitments := make([][]byte, len(blobs))
	for i, subtreeRoots := range subtreeRootsByBlob {
		commitments[i] = merkleRootFn(subtreeRoots)
	}

	return commitments, nil
}

// CreateCommitments generates commitments sequentially for given blobs.
func CreateCommitments(blobs []*sh.Blob, merkleRootFn MerkleRootFn, subtreeRootThreshold int) ([][]byte, error) {
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
			treeSize, err := RoundDownPowerOfTwo(totalSize)
			if err != nil {
				return treeSizes, err
			}
			treeSizes = append(treeSizes, treeSize)
			totalSize -= treeSize
		}
	}

	return treeSizes, nil
}

// splitBlobs splits the provided blobs into shares.
func splitBlobs(blobs ...*sh.Blob) ([]sh.Share, error) {
	writer := sh.NewSparseShareSplitter()
	for _, blob := range blobs {
		if err := writer.Write(blob); err != nil {
			return nil, err
		}
	}
	return writer.Export(), nil
}
