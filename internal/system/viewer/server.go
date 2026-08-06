package viewer

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// Handler serves the embedded dashboard at `/` and reverse-proxies every
// /api/* request to apiBase — the graph engine's API server (`ckg api`).
// The vector atlas backend joins the same proxy surface when it lands in
// Go.
//
// devDir, when non-empty, overrides the embedded assets with a disk path
// (the CKS_DEV_VIEWER_DIR loop: edit viewer source → `make viewer` →
// reload browser, no rebuild of the cks binary).
func Handler(apiBase *url.URL, devDir string) (http.Handler, error) {
	mux := http.NewServeMux()

	proxy := httputil.NewSingleHostReverseProxy(apiBase)
	mux.Handle("/api/", proxy)

	if devDir != "" {
		mux.Handle("/", http.FileServerFS(os.DirFS(devDir)))
		return mux, nil
	}
	sub, err := fs.Sub(viewerFS, "web_assets")
	if err != nil {
		// Compile-time `go:embed all:web_assets` guarantees the directory
		// exists; an error here is unrecoverable startup state.
		return nil, fmt.Errorf("viewer FS missing web_assets/: %w", err)
	}
	mux.Handle("/", http.FileServerFS(sub))
	return mux, nil
}
