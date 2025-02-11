package share

import (
	"bytes"
	"fmt"
)

// ParseTxs collects all of the transactions from the shares provided
func ParseTxs(shares []Share) ([][]byte, error) {
	// parse the shares. Only share version 0 is supported for transactions
	rawTxs, err := parseCompactShares(shares)
	if err != nil {
		return nil, err
	}

	return rawTxs, nil
}

// ParseBlobs collects all blobs from the shares provided
func ParseBlobs(shares []Share) ([]*Blob, error) {
	blobList, err := parseSparseShares(shares)
	if err != nil {
		return []*Blob{}, err
	}

	return blobList, nil
}

// ParseShares parses the shares provided and returns a list of Sequences.
// If ignorePadding is true then the returned Sequences will not contain
// any padding sequences.
func ParseShares(shares []Share, ignorePadding bool) ([]Sequence, error) {
	sequences := []Sequence{}
	currentSequence := Sequence{}

	for _, share := range shares {
		ns := share.Namespace()
		if share.IsSequenceStart() {
			if len(currentSequence.Shares) > 0 {
				sequences = append(sequences, currentSequence)
			}
			currentSequence = Sequence{
				Shares:    []Share{share},
				Namespace: ns,
			}
		} else {
			if !bytes.Equal(currentSequence.Namespace.Bytes(), ns.Bytes()) {
				return sequences, fmt.Errorf("share sequence %v has inconsistent namespace IDs with share %v", currentSequence, share)
			}
			currentSequence.Shares = append(currentSequence.Shares, share)
		}
	}

	if len(currentSequence.Shares) > 0 {
		sequences = append(sequences, currentSequence)
	}

	for _, sequence := range sequences {
		if err := sequence.validSequenceLen(); err != nil {
			return sequences, err
		}
	}

	result := []Sequence{}
	for _, sequence := range sequences {
		if ignorePadding && sequence.isPadding() {
			continue
		}
		result = append(result, sequence)
	}

	return result, nil
}
