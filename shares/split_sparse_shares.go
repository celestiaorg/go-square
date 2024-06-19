package shares

import (
	"errors"
	"fmt"

	"github.com/celestiaorg/go-square/blob"
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
func (sss *SparseShareSplitter) Write(blob *blob.Blob) error {
	if !slices.Contains(SupportedShareVersions, blob.ShareVersion()) {
		return fmt.Errorf("unsupported share version: %d", blob.ShareVersion())
	}

	rawData := blob.Data()
	blobNamespace := blob.Namespace()

	b, err := NewBuilder(blobNamespace, blob.ShareVersion(), true)
	if err != nil {
		return err
	}
	if err := b.WriteSequenceLen(uint32(len(rawData))); err != nil {
		return err
	}
	// add the signer to the first share for v1 share versions only
	if blob.ShareVersion() == ShareVersionOne {
		b.WriteSigner(blob.Signer())
	}

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

		b, err = NewBuilder(blobNamespace, blob.ShareVersion(), false)
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
	lastBlobNs, err := lastBlob.Namespace()
	if err != nil {
		return err
	}
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
