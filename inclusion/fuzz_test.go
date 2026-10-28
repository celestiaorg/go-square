package inclusion_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/celestiaorg/go-square/v4/inclusion"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/stretchr/testify/require"
)

// go-square generates commitments; NMT owns the verifier. Exercise that boundary
// with actual go-square subtree roots and a power-of-two row, as required by NMT.
func FuzzCommitmentProofRoundTrip(f *testing.F) {
	for _, length := range []int{1, 458, 459, 478, 479, 961, 4096} {
		for _, version := range []uint8{0, 1, 2} {
			f.Add(bytes.Repeat([]byte{1}, length), version, uint8(1))
			f.Add(bytes.Repeat([]byte{2}, length), version, uint8(64))
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, version, threshold uint8) {
		if len(data) == 0 || len(data) > 16*1024 {
			t.Skip()
		}
		proof, root, roots, width, leaves, ns := fuzzCommitmentProof(t, data, version, threshold)
		hasher := nmt.NewNmtHasher(sha256.New(), share.NamespaceSize, true)
		valid, err := proof.VerifySubtreeRootInclusion(hasher, roots, width, root)
		require.NoError(t, err)
		require.True(t, valid, "go-square subtree roots must verify against the row root")
		require.True(t, proof.VerifyInclusion(sha256.New(), namespace.ID(ns.Bytes()), leaves, root))
		// Corrupt only the digest, preserving the namespace range and node format.
		badRoot := bytes.Clone(root)
		badRoot[len(badRoot)-1] ^= 1
		valid, _ = proof.VerifySubtreeRootInclusion(hasher, roots, width, badRoot)
		require.False(t, valid)
		require.False(t, proof.VerifyInclusion(sha256.New(), namespace.ID(ns.Bytes()), leaves, badRoot))
		badRoots := append([][]byte(nil), roots...)
		badRoots[0] = bytes.Clone(badRoots[0])
		badRoots[0][len(badRoots[0])-1] ^= 1
		valid, _ = proof.VerifySubtreeRootInclusion(hasher, badRoots, width, root)
		require.False(t, valid, "changed subtree digests must not verify")
		badLeaves := append([][]byte(nil), leaves...)
		badLeaves[0] = bytes.Clone(badLeaves[0])
		badLeaves[0][len(badLeaves[0])-1] ^= 1
		require.False(t, proof.VerifyInclusion(sha256.New(), namespace.ID(ns.Bytes()), badLeaves, root))
	})
}

func FuzzVerifySubtreeRootInclusion(f *testing.F) {
	f.Add(0, 0, 0, []byte{}, []byte{}, []byte{})
	f.Add(-1, 1, 1, []byte{0xff}, []byte{}, []byte{})
	for _, length := range []int{1, 479, 961, 4096} {
		proof, root, roots, width, _, _ := fuzzCommitmentProof(f, bytes.Repeat([]byte{1}, length), 0, 1)
		nodes := bytes.Join(proof.Nodes(), nil)
		subtrees := bytes.Join(roots, nil)
		f.Add(proof.Start(), proof.End(), width, root, nodes, subtrees)
		f.Add(proof.Start(), proof.End(), width, root[:len(root)-1], nodes, subtrees)
		f.Add(proof.Start(), proof.End(), width, root, nodes, subtrees[:len(subtrees)-1])
	}
	f.Fuzz(func(t *testing.T, start, end, width int, root, nodes, subtrees []byte) {
		// Keep adversarial ranges and allocation bounded, including on 32-bit hosts.
		if start < -256 || start > 256 || end < -256 || end > 256 || width < -256 || width > 256 ||
			len(root) > 256 || len(nodes) > 4096 || len(subtrees) > 4096 {
			t.Skip()
		}
		proof := nmt.NewInclusionProof(start, end, fuzzProofNodes(nodes), true)
		hasher := nmt.NewNmtHasher(sha256.New(), share.NamespaceSize, true)
		valid, err := proof.VerifySubtreeRootInclusion(hasher, fuzzProofNodes(subtrees), width, root)
		if err != nil {
			require.False(t, valid)
		}
	})
}

// Split wire nodes without dropping a short final node, so malformed hashes
// reach the verifier's format checks too.
func fuzzProofNodes(wire []byte) [][]byte {
	const nodeSize = 2*share.NamespaceSize + sha256.Size
	nodes := make([][]byte, 0, (len(wire)+nodeSize-1)/nodeSize)
	for len(wire) > 0 {
		size := min(len(wire), nodeSize)
		nodes = append(nodes, wire[:size])
		wire = wire[size:]
	}
	return nodes
}

func fuzzCommitmentProof(t testing.TB, data []byte, version, threshold uint8) (nmt.Proof, []byte, [][]byte, int, [][]byte, share.Namespace) {
	t.Helper()
	ns := share.MustNewV0Namespace([]byte{1, 0})
	version %= 3
	var signer []byte
	if version != share.ShareVersionZero {
		signer = bytes.Repeat([]byte{1}, share.SignerSize)
	}
	if version == share.ShareVersionTwo {
		commitment := sha256.Sum256(data)
		data = append([]byte{0, 0, 0, 1}, commitment[:]...)
	}
	blob, err := share.NewBlob(ns, data, version, signer)
	require.NoError(t, err)
	shares, err := blob.ToShares()
	require.NoError(t, err)
	roots, err := inclusion.GenerateSubtreeRoots(blob, 1+int(threshold))
	require.NoError(t, err)
	width, err := inclusion.SubTreeWidth(len(shares), 1+int(threshold))
	require.NoError(t, err)
	// Exercise siblings on both sides of the blob, while keeping placement
	// aligned to its first subtree as required by the commitment rules.
	start := 0
	if threshold%2 == 1 {
		start = width
	}
	rowSize, err := inclusion.RoundUpPowerOfTwo(start + len(shares))
	require.NoError(t, err)
	tree := nmt.New(sha256.New(), nmt.NamespaceIDSize(share.NamespaceSize), nmt.IgnoreMaxNamespace(true))
	leaves := share.ToBytes(shares)
	for i := 0; i < rowSize; i++ {
		// Repeat the final leaf as filler outside the proven blob range. Every
		// NMT leaf has the outer namespace prefix used by go-square commitments.
		leaf := leaves[min(max(i-start, 0), len(leaves)-1)]
		require.NoError(t, tree.Push(append(bytes.Clone(ns.Bytes()), leaf...)))
	}
	root, err := tree.Root()
	require.NoError(t, err)
	proof, err := tree.ProveRange(start, start+len(shares))
	require.NoError(t, err)
	return proof, root, roots, width, leaves, ns
}
