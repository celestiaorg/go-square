package square_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/celestiaorg/go-square/v2"
)

var dirPath = filepath.Join("testdata", "corpra", "builder")

func init() {
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		panic(err)
	}
}

type builderCorpus struct {
	MaxSquareSize        int      `json:"max_sq_size"`
	SubtreeRootThreshold int      `json:"sr_thresh"`
	Txs                  [][]byte `json:"txs"`
}

func FuzzBuilderExport(f *testing.F) {
	if testing.Short() {
		f.Skip("running in -short mode")
	}

	// 1. Add the corpra.
	paths, err := filepath.Glob(filepath.Join(dirPath, "*.json"))
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range paths {
		jsonBlob, err := os.ReadFile(path)
		if err == nil {
			f.Add(jsonBlob)
		}
	}

	// 2. Run the fuzzer.
	f.Fuzz(func(t *testing.T, inputJSON []byte) {
		corpus := new(builderCorpus)
		if err := json.Unmarshal(inputJSON, corpus); err != nil {
			return
		}
		b, err := square.NewBuilder(corpus.MaxSquareSize, corpus.SubtreeRootThreshold, corpus.Txs...)
		if err != nil {
			return
		}
		_, _ = b.Export()
	})
}
