package share

import (
	"bytes"
	"fmt"
)

type sequence struct {
	ns           Namespace
	shareVersion uint8
	data         []byte
	sequenceLen  uint32
	signer       []byte
}

// parseSparseShares iterates through rawShares and parses out individual
// blobs. It returns an error if a rawShare contains a share version that
// isn't present in supportedShareVersions.
func parseSparseShares(shares []Share) (blobs []*Blob, err error) {
	if len(shares) == 0 {
		return nil, nil
	}
	sequences := make([]sequence, 0)

	for _, share := range shares {
		version := share.Version()
		if !bytes.Contains(SupportedShareVersions, []byte{version}) {
			return nil, fmt.Errorf("unsupported share version %v is not present in supported share versions %v", version, SupportedShareVersions)
		}

		if share.IsPadding() {
			continue
		}

		if share.IsSequenceStart() {
			sequences = append(sequences, sequence{
				ns:           share.Namespace(),
				shareVersion: version,
				data:         share.RawData(),
				sequenceLen:  share.SequenceLen(),
				signer:       GetSigner(share),
			})
		} else { // continuation share
			if len(sequences) == 0 {
				return nil, fmt.Errorf("continuation share %v without a sequence start share", share)
			}
			if !share.Namespace().Equals(sequences[len(sequences)-1].ns) {
				return nil, fmt.Errorf("continuation share %v has a different namespace than the previous share %v",
					share.Namespace(), sequences[len(sequences)-1].ns)
			}
			prev := &sequences[len(sequences)-1]
			prev.data = append(prev.data, share.RawData()...)
		}
	}
	for _, sequence := range sequences {
		if sequence.sequenceLen > uint32(len(sequence.data)) {
			return nil, fmt.Errorf("sequence length %v is greater than the number of bytes in the sequence %v",
				sequence.sequenceLen, len(sequence.data))
		}
		// trim any padding from the end of the sequence
		sequence.data = sequence.data[:sequence.sequenceLen]
		blob, err := NewBlob(sequence.ns, sequence.data, sequence.shareVersion, sequence.signer)
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, blob)
	}

	return blobs, nil
}
