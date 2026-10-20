package maintenance

import (
	"github.com/0xmhha/knowledge-system/internal/setup"
	"os"
	"testing"
)

// Private service fixtures use the test binary as their registered CKS role.
// Exercise the same supervisor protocol without requiring prebuilt binaries.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "__dataset-step" {
		if setup.RunGuardedDatasetStep(os.Args[2]) != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
