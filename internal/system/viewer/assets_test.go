package viewer

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
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

func TestActiveAssetsPrefersBuiltApp(t *testing.T) {
	source := fstest.MapFS{"index.html": {Data: []byte("stub")}, "app/index.html": {Data: []byte("built")}, "app/_next/app.js": {Data: []byte("chunk")}}
	active, err := activeAssets(source)
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(active, "index.html")
	if err != nil || string(index) != "built" {
		t.Fatal("successful build hidden by stub", err)
	}
	chunk, err := fs.ReadFile(active, "_next/app.js")
	if err != nil || string(chunk) != "chunk" {
		t.Fatal("built root lost relative chunks", err)
	}
}
func TestActiveAssetsKeepsUnbuiltFallback(t *testing.T) {
	active, err := activeAssets(fstest.MapFS{"index.html": {Data: []byte("stub")}})
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(active, "index.html")
	if err != nil || string(index) != "stub" {
		t.Fatal("unbuilt fallback missing", err)
	}
}
func TestActiveAssetsRejectsMalformedBuild(t *testing.T) {
	if _, err := activeAssets(fstest.MapFS{"index.html": {Data: []byte("stub")}, "app/index.html": {Mode: fs.ModeDir}}); err == nil {
		t.Fatal("malformed app hidden by fallback")
	}
}
