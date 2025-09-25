package inclusion

import (
	"crypto/sha256"

	sh "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
)

// nmtPool provides a fixed-size pool of bufferedNMT instances for efficient reuse.
type nmtPool struct {
	trees    chan *bufferedNMT
	poolSize int
	opts     []nmt.Option
}

// newNMTPool creates a new pool of buffered NMT instances.
func newNMTPool(poolSize int, maxSubtreeSize int) *nmtPool {
	pool := &nmtPool{
		trees:    make(chan *bufferedNMT, poolSize),
		poolSize: poolSize,
		opts: []nmt.Option{
			nmt.NamespaceIDSize(sh.NamespaceSize),
			nmt.IgnoreMaxNamespace(true),
			nmt.ReuseBuffers(true),
		},
	}

	// Pre-populate the pool with buffered NMT instances
	for i := 0; i < poolSize; i++ {
		pool.trees <- newBufferedNMT(maxSubtreeSize, pool)
	}

	return pool
}

// acquire gets a buffered NMT from the pool, blocking if none available.
func (p *nmtPool) acquire() *bufferedNMT {
	return <-p.trees
}

// release returns a buffered NMT to the pool for reuse.
func (p *nmtPool) release(tree *bufferedNMT) {
	tree.reset()
	p.trees <- tree
}

// bufferedNMT wraps an NMT with a pre-allocated buffer for efficient operations.
type bufferedNMT struct {
	tree      *nmt.NamespacedMerkleTree
	buffer    []byte   // Pre-allocated buffer for namespace+share data
	pool      *nmtPool // Reference to the pool for auto-release
	leafSize  int      // Size of namespace + share
	maxLeaves int      // Maximum number of leaves this buffer can handle
}

// newBufferedNMT creates a new buffered NMT wrapper.
func newBufferedNMT(maxLeaves int, pool *nmtPool) *bufferedNMT {
	leafSize := sh.NamespaceSize + sh.ShareSize
	return &bufferedNMT{
		tree:      nmt.New(sha256.New(), pool.opts...),
		buffer:    make([]byte, maxLeaves*leafSize),
		pool:      pool,
		leafSize:  leafSize,
		maxLeaves: maxLeaves,
	}
}

// reset prepares the buffered NMT for reuse.
func (t *bufferedNMT) reset() {
	t.tree.Reset()
}

// computeRoot processes a set of leaves with a given namespace and returns the root.
// It automatically releases itself back to the pool after computing the root.
func (t *bufferedNMT) computeRoot(namespace []byte, leaves [][]byte) ([]byte, error) {
	defer t.pool.release(t)

	// Pre-fill namespace in buffer for all leaves
	for i := 0; i < len(leaves); i++ {
		offset := i * t.leafSize
		copy(t.buffer[offset:offset+sh.NamespaceSize], namespace)
	}

	// Copy leaf data and push to tree
	for i, leaf := range leaves {
		offset := i * t.leafSize
		copy(t.buffer[offset+sh.NamespaceSize:offset+t.leafSize], leaf)

		// Create slice from buffer and push to NMT
		nsLeaf := t.buffer[offset : offset+sh.NamespaceSize+len(leaf)]
		if err := t.tree.Push(nsLeaf); err != nil {
			return nil, err
		}
	}

	return t.tree.Root()
}
