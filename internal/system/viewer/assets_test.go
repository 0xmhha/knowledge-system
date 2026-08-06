package viewer

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCopyAssetsTo verifies that CopyAssetsTo materialises the embedded
// dashboard onto disk — index.html must appear (the `cks viewer export`
// half of a static bundle).
func TestCopyAssetsTo(t *testing.T) {
	dst := t.TempDir()
	if err := CopyAssetsTo(dst); err != nil {
		t.Fatalf("CopyAssetsTo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "index.html")); err != nil {
		t.Errorf("index.html not written: %v", err)
	}
}
