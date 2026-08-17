package square_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNoSDKTransactionSchemaDependency verifies that the root square package
// does not depend on a Cosmos SDK transaction schema: classification belongs
// to the caller.
func TestNoSDKTransactionSchemaDependency(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	require.NoError(t, err)

	forbidden := []string{
		"/proto/cosmos",
		"/proto/celestia/fibre",
	}
	for _, dep := range strings.Split(string(out), "\n") {
		for _, bad := range forbidden {
			require.NotContains(t, dep, bad,
				"the root square package must not depend on an SDK transaction schema; classification belongs to the caller")
		}
	}
}
