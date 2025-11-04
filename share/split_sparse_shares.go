package share

import (
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/exp/slices"
)

// SparseShareSplitter lazily splits blobs into shares that will eventually be
// included in a data square. It also has methods to help progressively count
// how many shares the blobs written take up.
type SparseShareSplitter struct {
	shares []Share
}

func NewSparseShareSplitter() *SparseShareSplitter {
	return &SparseShareSplitter{}
}

// Write writes the provided blob to this sparse share splitter. It returns an
// error or nil if no error is encountered.
func (sss *SparseShareSplitter) Write(blob *Blob) error {
	if !slices.Contains(SupportedShareVersions, blob.ShareVersion()) {
		return fmt.Errorf("unsupported share version: %d", blob.ShareVersion())
	}

	rawData := blob.Data()
	blobNamespace := blob.Namespace()

	b, err := newBuilder(blobNamespace, blob.ShareVersion(), true)
	if err != nil {
		return err
	}

	// For share version 2, sequence length is the length of the commitment (32 bytes)
	// For other versions, sequence length is the total data length
	var sequenceLen uint32
	if blob.ShareVersion() == ShareVersionTwo {
		sequenceLen = CommitmentSize
	} else {
		sequenceLen = uint32(len(rawData))
	}

	if err := b.WriteSequenceLen(sequenceLen); err != nil {
		return err
	}

	// add the signer to the first share for v1 and v2 share versions
	if blob.ShareVersion() == ShareVersionOne || blob.ShareVersion() == ShareVersionTwo {
		b.WriteSigner(blob.Signer())
	}

	// For share version 2, write row_version and commitment separately
	if blob.ShareVersion() == ShareVersionTwo {
		if len(rawData) != RowVersionSize+CommitmentSize {
			return fmt.Errorf("share version 2 requires data of size %d bytes (row_version + commitment), got %d", RowVersionSize+CommitmentSize, len(rawData))
		}
		// Extract row_version (first 4 bytes) and commitment (last 32 bytes)
		rowVersion := binary.BigEndian.Uint32(rawData[0:RowVersionSize])
		commitment := rawData[RowVersionSize:]
		b.WriteRowVersion(rowVersion)
		b.WriteCommitment(commitment)
		// Zero pad the share since all data fits in one share
		b.ZeroPadIfNecessary()
		share, err := b.Build()
		if err != nil {
			return err
		}
		sss.shares = append(sss.shares, *share)
		return nil
	}

	// For share versions 0 and 1, write data normally
	for rawData != nil {
		rawDataLeftOver := b.AddData(rawData)
		if rawDataLeftOver == nil {
			// Just call it on the latest share
			b.ZeroPadIfNecessary()
		}

		share, err := b.Build()
		if err != nil {
			return err
		}
		sss.shares = append(sss.shares, *share)

		b, err = newBuilder(blobNamespace, blob.ShareVersion(), false)
		if err != nil {
			return err
		}
		rawData = rawDataLeftOver
	}

	return nil
}

// WriteNamespacePaddingShares adds padding shares with the namespace of the
// last written share. This is useful to follow the non-interactive default
// rules. This function assumes that at least one share has already been
// written.
func (sss *SparseShareSplitter) WriteNamespacePaddingShares(count int) error {
	if count < 0 {
		return errors.New("cannot write negative namespaced shares")
	}
	if count == 0 {
		return nil
	}
	if len(sss.shares) == 0 {
		return errors.New("cannot write namespace padding shares on an empty SparseShareSplitter")
	}
	lastBlob := sss.shares[len(sss.shares)-1]
	lastBlobNs := lastBlob.Namespace()
	lastBlobInfo := lastBlob.InfoByte()
	nsPaddingShares, err := NamespacePaddingShares(lastBlobNs, lastBlobInfo.Version(), count)
	if err != nil {
		return err
	}
	sss.shares = append(sss.shares, nsPaddingShares...)

	return nil
}

// Export finalizes and returns the underlying shares.
func (sss *SparseShareSplitter) Export() []Share {
	return sss.shares
}

// Count returns the current number of shares that will be made if exporting.
func (sss *SparseShareSplitter) Count() int {
	return len(sss.shares)
}
