// Package viewer embeds the unified dashboard (tools/viewer build output)
//
// and hands it to the `cks viewer` command. Assets are bundled via embed.FS
// so the binary needs no sidecar files. The build assumes
// `make viewer` has been run so web_assets/{index.html,
// assets/viewer.js[.map]} exist before `go build`.
package viewer

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// viewerFS embeds the bundled viewer assets. The `all:` prefix includes
// dotfiles and underscore-prefixed entries so future asset names don't
// silently get skipped by the default embed semantics.
//
//go:embed all:web_assets
var viewerFS embed.FS

// CopyAssetsTo materialises the embedded viewer onto disk under dst.
// Used by `cks viewer export` so the same assets that ship with the
// server can be pinned into a static export bundle.
func CopyAssetsTo(dst string) error {
	sub, err := fs.Sub(viewerFS, "web_assets")
	if err != nil {
		return fmt.Errorf("sub web_assets: %w", err)
	}
	return fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		return nil
	})
}
