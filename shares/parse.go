package shares

import (
	"bytes"
	"fmt"
)

// ParseTxs collects all of the transactions from the shares provided
func ParseTxs(shares []Share) ([][]byte, error) {
	// parse the shares
	rawTxs, err := parseCompactShares(shares, SupportedShareVersions)
	if err != nil {
		return nil, err
	}

	return rawTxs, nil
}

// ParseBlobs collects all blobs from the shares provided
func ParseBlobs(shares []Share) ([]*Blob, error) {
	blobList, err := parseSparseShares(shares, SupportedShareVersions)
	if err != nil {
		return []*Blob{}, err
	}

	return blobList, nil
}

// ParseShares parses the shares provided and returns a list of ShareSequences.
// If ignorePadding is true then the returned ShareSequences will not contain
// any padding sequences.
func ParseShares(shares []Share, ignorePadding bool) ([]ShareSequence, error) {
	sequences := []ShareSequence{}
	currentSequence := ShareSequence{}

	for _, share := range shares {
		isStart := share.IsSequenceStart()
		ns, err := share.Namespace()
		if err != nil {
			return sequences, err
		}
		if isStart {
			if len(currentSequence.Shares) > 0 {
				sequences = append(sequences, currentSequence)
			}
			currentSequence = ShareSequence{
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

	result := []ShareSequence{}
	for _, sequence := range sequences {
		isPadding, err := sequence.isPadding()
		if err != nil {
			return nil, err
		}
		if ignorePadding && isPadding {
			continue
		}
		result = append(result, sequence)
	}

	return result, nil
}
