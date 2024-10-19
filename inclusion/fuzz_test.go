package inclusion_test

import (
	"encoding/json"
	"testing"

	"github.com/celestiaorg/go-square/v2/inclusion"
)

type subtreeWidthCorpus struct {
	ShareCount int `json:"sc"`
	Threshold  int `json:"th"`
}

func FuzzSubtreeWidth(f *testing.F) {
	// 1. Create the corpra.
	corpra := []*subtreeWidthCorpus{
		{2, 16},
		{16, 16},
		{2, defaultSubtreeRootThreshold},
		{8, defaultSubtreeRootThreshold},
	}
	for _, corpus := range corpra {
		jsonBlob, err := json.Marshal(corpus)
		if err == nil {
			f.Add(jsonBlob)
		}
	}

	// 2. Run the fuzzers.
	f.Fuzz(func(t *testing.T, jsonBlob []byte) {
		input := new(subtreeWidthCorpus)
		if err := json.Unmarshal(jsonBlob, input); err != nil {
			return
		}
		_ = inclusion.SubTreeWidth(input.ShareCount, input.Threshold)
	})
}
