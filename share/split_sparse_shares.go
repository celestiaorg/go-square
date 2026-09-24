package share

import (
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
	if len(rawData) == 0 {
		return errors.New("cannot write blob with empty data")
	}
	blobNamespace := blob.Namespace()

	b, err := newBuilder(blobNamespace, blob.ShareVersion(), true)
	if err != nil {
		return err
	}
	// Blob.Data already contains the encoded Fibre version and commitment for
	// v2, so all versions use the same sequence writer.
	if err := b.WriteSequenceLen(uint32(len(rawData))); err != nil {
		return err
	}
	b.WriteSigner(blob.Signer())

	writer := sequenceWriter{shares: sss.shares, pending: b}
	defer func() { sss.shares = writer.shares }()
	if err := writer.write(rawData); err != nil {
		return err
	}
	_, err = writer.finalize()
	return err
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
