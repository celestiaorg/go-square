package square_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNoSDKTransactionSchemaDependency guards the property that closed the
// classification divergence: go-square must not carry its own copy of a Cosmos
// SDK transaction schema. Two schemas over the same bytes is how this library
// and celestia-app came to disagree about what a transaction says, and a
// disagreement there means the proposer and its validators build different
// squares from the same block.
func TestNoSDKTransactionSchemaDependency(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	require.NoError(t, err)

	forbidden := []string{
		"go-square/v4/proto/cosmos",
		"go-square/v4/proto/celestia",
	}
	for _, dep := range strings.Split(string(out), "\n") {
		for _, bad := range forbidden {
			require.NotContains(t, dep, bad,
				"the root square package must not depend on an SDK transaction schema; classification belongs to the caller")
		}
	}
}
