package inclusion_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/celestiaorg/go-square/v4/inclusion"
	"github.com/celestiaorg/go-square/v4/share"
	"github.com/stretchr/testify/require"
)

const (
	blobSpecSize = 3
	maxSpecBlobs = 8
	maxSpecData  = 4096
)

// fuzzSpecBlobs builds at most maxSpecBlobs blobs from spec. Each blob takes
// three bytes: a selector (share version in the low two bits, namespace ID
// byte above) and a little-endian data length of at most 4 KiB. Data is
// filled cyclically from data so that blobs share the fuzzer's bytes.
func fuzzSpecBlobs(t *testing.T, spec, data []byte) []*share.Blob {
	t.Helper()
	var blobs []*share.Blob
	for i := 0; i+blobSpecSize <= len(spec) && len(blobs) < maxSpecBlobs; i += blobSpecSize {
		sel := spec[i]
		size := 1 + int(binary.LittleEndian.Uint16(spec[i+1:]))%maxSpecData
		payload := make([]byte, size)
		for j := range payload {
			if len(data) > 0 {
				payload[j] = data[(i+j)%len(data)]
			} else {
				payload[j] = byte(i + j)
			}
		}
		ns := share.MustNewV0Namespace([]byte{sel >> 2})
		signer := bytes.Repeat([]byte{sel}, share.SignerSize)
		var blob *share.Blob
		var err error
		switch sel % 3 {
		case 0:
			blob, err = share.NewV0Blob(ns, payload)
		case 1:
			blob, err = share.NewV1Blob(ns, payload, signer)
		default:
			commitment := make([]byte, share.FibreCommitmentSize)
			copy(commitment, payload)
			blob, err = share.NewV2Blob(ns, uint32(size), commitment, signer)
		}
		require.NoError(t, err)
		blobs = append(blobs, blob)
	}
	return blobs
}

// FuzzCommitmentParity asserts that CreateParallelCommitments, CreateCommitments
// and CreateCommitment agree on every commitment and on whether an error occurs,
// for any blob set, subtree root threshold (including non-positive ones) and
// worker count.
func FuzzCommitmentParity(f *testing.F) {
	spec := func(sizes ...uint16) []byte {
		out := make([]byte, 0, 3*len(sizes))
		for i, size := range sizes {
			out = append(out, byte(i*5), byte(size), byte(size>>8))
		}
		return out
	}
	data := bytes.Repeat([]byte{1, 2, 3, 5, 8}, 100)
	for _, threshold := range []int8{1, 2, 64, 0, -1} {
		f.Add(spec(0), data, threshold, uint8(1))
		f.Add(spec(477, 478, 479), data, threshold, uint8(2))
		f.Add(spec(4095, 1, 960, 961, 2000), data, threshold, uint8(4))
		f.Add(spec(10, 20, 30, 40, 50, 60, 70, 80), []byte{}, threshold, uint8(8))
	}
	f.Add([]byte{}, data, int8(64), uint8(0))
	f.Add(spec(100), data, int8(64), uint8(0))
	f.Fuzz(func(t *testing.T, spec, data []byte, threshold int8, workers uint8) {
		if len(spec) > maxSpecBlobs*blobSpecSize || len(data) > maxSpecData {
			t.Skip()
		}
		checkCommitmentParity(t, spec, data, int(threshold), int(workers%9))
	})
}

func TestCommitmentParitySeeds(t *testing.T) {
	spec := []byte{0, 0xdd, 1, 1, 0xff, 0x0f, 2, 1, 0, 6, 0xc1, 3}
	for _, threshold := range []int{1, 2, 64} {
		for workers := 1; workers <= 8; workers++ {
			require.Len(t, checkCommitmentParity(t, spec, []byte("data"), threshold, workers), 4)
		}
	}
}

func checkCommitmentParity(t *testing.T, spec, data []byte, threshold, workers int) [][]byte {
	t.Helper()
	blobs := fuzzSpecBlobs(t, spec, data)
	sequential, seqErr := inclusion.CreateCommitments(blobs, hashConcatenatedData, threshold)
	parallel, parErr := inclusion.CreateParallelCommitments(blobs, hashConcatenatedData, threshold, workers)
	if workers <= 0 && len(blobs) > 0 {
		require.Error(t, parErr, "non-positive worker count must be rejected")
		return nil
	}
	require.Equal(t, seqErr == nil, parErr == nil, "verdicts differ: sequential %v, parallel %v", seqErr, parErr)
	if seqErr != nil {
		return nil
	}
	require.Len(t, sequential, len(blobs))
	require.Equal(t, sequential, parallel, "commitments differ")
	for i, blob := range blobs {
		single, err := inclusion.CreateCommitment(blob, hashConcatenatedData, threshold)
		require.NoError(t, err)
		require.Equal(t, sequential[i], single, "blob %d", i)
	}
	return parallel
}
