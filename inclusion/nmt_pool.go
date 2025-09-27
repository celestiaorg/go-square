package inclusion

import (
	"crypto/sha256"
	"errors"

	sh "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
)

// nmtPool provides a fixed-size pool of bufferTree instances for efficient reuse.
type nmtPool struct {
	trees    chan *bufferTree
	poolSize int
	opts     []nmt.Option
}

// newNMTPool creates a new pool of buffered NMT instances.
func newNMTPool(poolSize int, maxLeaves int) (*nmtPool, error) {
	if poolSize <= 0 {
		return nil, errors.New("pool size must be positive")
	}
	if maxLeaves <= 0 {
		return nil, errors.New("max leaves must be positive")
	}
	pool := &nmtPool{
		trees:    make(chan *bufferTree, poolSize),
		poolSize: poolSize,
		opts: []nmt.Option{
			nmt.NamespaceIDSize(sh.NamespaceSize),
			nmt.IgnoreMaxNamespace(true),
			nmt.ReuseBuffers(true),
		},
	}

	// pre-populate the pool with buffered NMT instances
	for i := 0; i < poolSize; i++ {
		pool.trees <- newBufferTree(maxLeaves, pool)
	}

	return pool, nil
}

// acquire gets a buffered NMT from the pool, blocking if none available.
func (p *nmtPool) acquire() *bufferTree {
	return <-p.trees
}

// release returns a buffered NMT to the pool for reuse.
func (p *nmtPool) release(tree *bufferTree) {
	tree.reset()
	p.trees <- tree
}

// bufferTree wraps an NMT with a pre-allocated buffer for efficient operations.
type bufferTree struct {
	// tree is an instance of NMT for root calculation
	tree *nmt.NamespacedMerkleTree
	// buffer is a pre-allocated buffer for namespace and share data
	buffer []byte
	// pool reference to the pool for release
	pool *nmtPool
	// leafSize is a size of namespace + share
	leafSize int
}

// newBufferTree creates a new buffered NMT wrapper.
func newBufferTree(maxLeaves int, pool *nmtPool) *bufferTree {
	leafSize := sh.NamespaceSize + sh.ShareSize
	return &bufferTree{
		tree:     nmt.New(sha256.New(), pool.opts...),
		buffer:   make([]byte, maxLeaves*leafSize),
		pool:     pool,
		leafSize: leafSize,
	}
}

// reset prepares the buffered NMT for reuse.
func (t *bufferTree) reset() {
	t.tree.Reset()
}

// computeRoot processes a set of leaves with a given namespace and returns the root.
// It automatically releases itself back to the pool after computing the root.
func (t *bufferTree) computeRoot(namespace []byte, leaves [][]byte) ([]byte, error) {
	defer t.pool.release(t)

	// pre-fill namespace in buffer for all leaves
	for i := 0; i < len(leaves); i++ {
		offset := i * t.leafSize
		copy(t.buffer[offset:offset+sh.NamespaceSize], namespace)
	}

	// copy leaf data and push to tree
	for i, leaf := range leaves {
		offset := i * t.leafSize
		copy(t.buffer[offset+sh.NamespaceSize:offset+t.leafSize], leaf)

		nsLeaf := t.buffer[offset : offset+sh.NamespaceSize+len(leaf)]
		if err := t.tree.Push(nsLeaf); err != nil {
			return nil, err
		}
	}
	return t.tree.Root()
}
