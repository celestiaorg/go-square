package share

// parseSparseShares decodes blobs through the same sequence representation used
// by ParseShares. Version checks and blob validation remain specific to blobs.
func parseSparseShares(shares []Share) (blobs []*Blob, err error) {
	for _, share := range shares {
		if err := share.CheckVersionSupported(); err != nil {
			return nil, err
		}
	}

	sequences, err := collectSequences(shares, true)
	if err != nil {
		return nil, err
	}
	for _, sequence := range sequences {
		data, err := sequence.RawData()
		if err != nil {
			return nil, err
		}
		first := sequence.Shares[0]
		blob, err := NewBlob(sequence.Namespace, data, first.Version(), GetSigner(first))
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, blob)
	}
	return blobs, nil
}
