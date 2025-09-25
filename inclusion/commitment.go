package inclusion

import (
	"crypto/sha256"
	"sync"

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

// GenerateSubtreeRootsReusedNMT generates the subtree roots of a blob using a reusable NMT.
// This is an optimized version that reuses the same NMT instance across multiple subtrees
// to reduce memory allocations.
func GenerateSubtreeRootsReusedNMT(blob *sh.Blob, subtreeRootThreshold int) ([][]byte, error) {
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
	// Create a single NMT instance with ReuseBuffer option
	tree := nmt.New(sha256.New(),
		nmt.NamespaceIDSize(sh.NamespaceSize),
		nmt.IgnoreMaxNamespace(true),
		nmt.ReuseBuffers(true))

	// Pre-allocate a single large buffer for all shares with namespace prepended
	// This allows NMT to use the buffer directly without copying
	leafSize := sh.NamespaceSize + sh.ShareSize
	nsLeafBuf := make([]byte, len(shares)*leafSize)

	// Pre-fill all namespace prefixes in the buffer
	for i := 0; i < len(shares); i++ {
		copy(nsLeafBuf[i*leafSize:i*leafSize+sh.NamespaceSize], namespace.Bytes())
	}

	// create the commitments by pushing each leaf set onto the reused NMT
	subTreeRoots := make([][]byte, len(leafSets))
	shareIdx := 0
	for i, set := range leafSets {
		// Reset the tree for reuse
		tree.Reset()

		for _, leaf := range set {
			// Calculate offset in the large buffer
			offset := shareIdx * leafSize
			// Copy share data after the namespace
			copy(nsLeafBuf[offset+sh.NamespaceSize:offset+leafSize], leaf)
			// Push slice from the large buffer - NMT will use it directly
			nsLeaf := nsLeafBuf[offset : offset+sh.NamespaceSize+len(leaf)]

			err = tree.Push(nsLeaf)
			if err != nil {
				return nil, err
			}
			shareIdx++
		}
		// add the root
		root, err := tree.Root()
		if err != nil {
			return nil, err
		}
		// Make a copy of the root since the tree buffer will be reused
		subTreeRoots[i] = append([]byte(nil), root...)
	}
	return subTreeRoots, nil
}

// GenerateSubtreeRootsParallel generates the subtree roots of a blob using parallel processing.
// This version uses goroutines to process multiple leaf sets concurrently with reusable NMTs.
func GenerateSubtreeRootsParallel(blob *sh.Blob, subtreeRootThreshold int) ([][]byte, error) {
	return GenerateSubtreeRootsParallelWithWorkers(blob, subtreeRootThreshold, 16)
}

// GenerateSubtreeRootsParallelWithWorkers generates the subtree roots of a blob using parallel processing
// with a configurable number of worker goroutines.
//
// Work Distribution Strategy:
// The function splits the work into "leaf sets" where each leaf set represents a subtree in the
// Merkle Mountain Range. The work is distributed as follows:
//
// 1. Each leaf set is an independent unit of work that produces one subtree root
// 2. Leaf sets are processed via a work queue (channel) that workers pull from
// 3. Each worker goroutine:
//   - Has its own reusable NMT instance to avoid lock contention
//   - Pulls leaf sets from the work queue until it's empty
//   - Processes each leaf set by pushing all its leaves into the NMT and computing the root
//
// For example, with 128 shares and subtreeRootThreshold=64:
// - This creates 2 leaf sets of 64 shares each
// - With 2 workers, each worker would process 1 leaf set
// - With 4 workers, 2 workers would each process 1 leaf set, and 2 would be idle
//
// The work distribution is dynamic (work-stealing pattern) rather than static partitioning,
// which provides better load balancing when leaf sets have different sizes.
func GenerateSubtreeRootsParallelWithWorkers(blob *sh.Blob, subtreeRootThreshold int, numWorkers int) ([][]byte, error) {
	shares, err := splitBlobs(blob)
	if err != nil {
		return nil, err
	}

	// Calculate Merkle Mountain Range structure
	subTreeWidth := SubTreeWidth(len(shares), subtreeRootThreshold)
	treeSizes, err := MerkleMountainRangeSizes(uint64(len(shares)), uint64(subTreeWidth))
	if err != nil {
		return nil, err
	}

	// Create leaf sets - each will become one subtree root
	leafSets := make([][][]byte, len(treeSizes))
	cursor := uint64(0)
	for i, treeSize := range treeSizes {
		leafSets[i] = sh.ToBytes(shares[cursor : cursor+treeSize])
		cursor += treeSize
	}

	namespace := blob.Namespace()

	// Pre-allocate a single large buffer for all shares with namespace prepended
	// This avoids allocations during NMT operations
	leafSize := sh.NamespaceSize + sh.ShareSize
	nsLeafBuf := make([]byte, len(shares)*leafSize)

	// Pre-fill all namespace prefixes and share data in the buffer
	for i := 0; i < len(shares); i++ {
		copy(nsLeafBuf[i*leafSize:i*leafSize+sh.NamespaceSize], namespace.Bytes())
	}

	shareIdx := 0
	for _, set := range leafSets {
		for _, leaf := range set {
			offset := shareIdx * leafSize
			copy(nsLeafBuf[offset+sh.NamespaceSize:offset+leafSize], leaf)
			shareIdx++
		}
	}

	// Result slice to hold computed roots
	subTreeRoots := make([][]byte, len(leafSets))

	// Adjust number of workers based on available work
	if numWorkers < 1 {
		numWorkers = 1
	}
	if len(leafSets) < numWorkers {
		numWorkers = len(leafSets)
	}

	// Use errgroup for clean error handling and goroutine management
	g := new(errgroup.Group)
	workChan := make(chan int, len(leafSets))

	// Queue all work items (leaf set indices)
	for i := range leafSets {
		workChan <- i
	}
	close(workChan)

	// Mutex to protect subTreeRoots writes (though each index is written only once)
	var mu sync.Mutex

	// Start worker goroutines
	for w := 0; w < numWorkers; w++ {
		g.Go(func() error {
			// Each worker has its own reusable NMT to avoid contention
			tree := nmt.New(sha256.New(),
				nmt.NamespaceIDSize(sh.NamespaceSize),
				nmt.IgnoreMaxNamespace(true),
				nmt.ReuseBuffers(true))

			// Process work items from the queue
			for leafSetIdx := range workChan {
				tree.Reset()
				set := leafSets[leafSetIdx]

				// Calculate starting position in the pre-filled buffer
				startIdx := 0
				for j := 0; j < leafSetIdx; j++ {
					startIdx += len(leafSets[j])
				}

				// Push all leaves for this leaf set into the NMT
				for j, leaf := range set {
					offset := (startIdx + j) * leafSize
					nsLeaf := nsLeafBuf[offset : offset+sh.NamespaceSize+len(leaf)]

					if err := tree.Push(nsLeaf); err != nil {
						return err
					}
				}

				// Compute the root for this subtree
				root, err := tree.Root()
				if err != nil {
					return err
				}

				// Store the root (making a copy since tree buffer will be reused)
				mu.Lock()
				subTreeRoots[leafSetIdx] = append([]byte(nil), root...)
				mu.Unlock()
			}
			return nil
		})
	}

	// Wait for all workers to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return subTreeRoots, nil
}

// CreateParallelCommitments generates commitments for multiple blobs in parallel using a pool of NMT instances.
// This implementation:
// 1. Splits blobs into shares in parallel using X goroutines
// 2. Uses a pool of buffered NMT wrappers for efficient memory reuse
// 3. Processes all subtree roots across all blobs concurrently
// 4. Returns commitments in the same order as input blobs
func CreateParallelCommitments(blobs []*sh.Blob, merkleRootFn MerkleRootFn, subtreeRootThreshold int, numWorkers int) ([][]byte, error) {
	if len(blobs) == 0 {
		return [][]byte{}, nil
	}

	// Step 1: Split all blobs into shares in parallel
	type blobShares struct {
		shares []sh.Share
		err    error
	}

	blobSharesResults := make([]blobShares, len(blobs))
	g := new(errgroup.Group)
	g.SetLimit(numWorkers) // Limit concurrent blob splitting

	for i := range blobs {
		idx := i
		g.Go(func() error {
			shares, err := splitBlobs(blobs[idx])
			blobSharesResults[idx] = blobShares{shares: shares, err: err}
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Step 2: Calculate the maximum subtree size across all blobs
	maxSubtreeSize := 0
	type blobInfo struct {
		shares    []sh.Share
		namespace sh.Namespace
		treeSizes []uint64
		leafSets  [][][]byte
	}
	blobInfos := make([]blobInfo, len(blobs))

	for i, blob := range blobs {
		shares := blobSharesResults[i].shares
		subTreeWidth := SubTreeWidth(len(shares), subtreeRootThreshold)
		treeSizes, err := MerkleMountainRangeSizes(uint64(len(shares)), uint64(subTreeWidth))
		if err != nil {
			return nil, err
		}

		// Track maximum subtree size for buffer allocation
		for _, size := range treeSizes {
			if int(size) > maxSubtreeSize {
				maxSubtreeSize = int(size)
			}
		}

		// Prepare leaf sets for this blob
		leafSets := make([][][]byte, len(treeSizes))
		cursor := uint64(0)
		for j, treeSize := range treeSizes {
			leafSets[j] = sh.ToBytes(shares[cursor : cursor+treeSize])
			cursor += treeSize
		}

		blobInfos[i] = blobInfo{
			shares:    shares,
			namespace: blob.Namespace(),
			treeSizes: treeSizes,
			leafSets:  leafSets,
		}
	}

	// Step 3: Create NMT pool with appropriate buffer size
	poolSize := numWorkers * 2 // Allow some buffer for concurrent operations
	pool := newNMTPool(poolSize, maxSubtreeSize)

	// Step 4: Process all subtree roots in parallel
	type subtreeResult struct {
		blobIdx int
		treeIdx int
		root    []byte
		err     error
	}

	// Calculate total number of subtrees
	totalSubtrees := 0
	for _, info := range blobInfos {
		totalSubtrees += len(info.leafSets)
	}

	resultChan := make(chan subtreeResult, totalSubtrees)
	g = new(errgroup.Group)
	g.SetLimit(numWorkers)

	// Queue all subtree computations
	for blobIdx, info := range blobInfos {
		bIdx := blobIdx
		bInfo := info
		for treeIdx, leafSet := range info.leafSets {
			tIdx := treeIdx
			leaves := leafSet

			g.Go(func() error {
				// Acquire a buffered NMT from the pool
				tree := pool.acquire()

				// Compute the root for this subtree
				root, err := tree.computeRoot(bInfo.namespace.Bytes(), leaves)

				resultChan <- subtreeResult{
					blobIdx: bIdx,
					treeIdx: tIdx,
					root:    root,
					err:     err,
				}
				return err
			})
		}
	}

	// Wait for all computations to complete
	if err := g.Wait(); err != nil {
		close(resultChan)
		return nil, err
	}
	close(resultChan)

	// Step 5: Collect results and organize by blob
	subtreeRootsByBlob := make([][][]byte, len(blobs))
	for i, info := range blobInfos {
		subtreeRootsByBlob[i] = make([][]byte, len(info.leafSets))
	}

	for result := range resultChan {
		if result.err != nil {
			return nil, result.err
		}
		subtreeRootsByBlob[result.blobIdx][result.treeIdx] = result.root
	}

	// Step 6: Compute final commitments using the merkle root function
	commitments := make([][]byte, len(blobs))
	for i, subtreeRoots := range subtreeRootsByBlob {
		commitments[i] = merkleRootFn(subtreeRoots)
	}

	return commitments, nil
}

// CreateCommitments is the original sequential implementation for comparison.
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
