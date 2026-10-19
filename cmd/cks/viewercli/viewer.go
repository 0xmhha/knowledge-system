// Package viewercli is the `cks viewer` command group: the unified
// graph+vector dashboard, served by the composition engine.
//
// The dashboard UI (tools/viewer) is a cross-engine artifact — its Atlas
// page is the vector viewer — so it lives here rather than inside one
// engine's server. Engine data stays behind engine APIs: /api/* is
// reverse-proxied to a `ckg api` backend, either spawned as a sibling
// subprocess (--graph) or already running elsewhere (--api-url).
package viewercli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/system/viewer"
)

// NewCmd builds the `cks viewer` command group.
func NewCmd() *cobra.Command {
	var graph, apiURL string
	var port int
	var open bool
	cmd := &cobra.Command{
		Use:   "viewer",
		Short: "Serve the unified dashboard (graph + vector UI)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if (graph == "") == (apiURL == "") {
				return fmt.Errorf("exactly one of --graph or --api-url is required")
			}
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return runViewer(ctx, graph, apiURL, port, open, cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory containing graph.db — spawns a sibling 'ckg api' backend")
	cmd.Flags().StringVar(&apiURL, "api-url", "", "already-running graph API base URL (e.g. http://127.0.0.1:8081) — no backend is spawned")
	cmd.Flags().IntVar(&port, "port", 8080, "dashboard HTTP port")
	cmd.Flags().BoolVar(&open, "open", false, "open browser on start")
	cmd.AddCommand(newExportCmd())
	return cmd
}

type backendLauncher func(context.Context, string, int, interface{ Write([]byte) (int, error) }) (*graphBackend, error)

func runViewer(ctx context.Context, graph, apiURL string, port int, open bool, stderr interface{ Write([]byte) (int, error) }) error {
	return runViewerWithBackend(ctx, graph, apiURL, port, open, stderr, func(ctx context.Context, graph string, port int, stderr interface{ Write([]byte) (int, error) }) (*graphBackend, error) {
		return startGraphBackend(ctx, siblingCkg(), []string{"api", "--graph", graph, "--port", fmt.Sprint(port)}, stderr, 5*time.Second)
	})
}

func runViewerWithBackend(parent context.Context, graph, apiURL string, port int, open bool, stderr interface{ Write([]byte) (int, error) }, launch backendLauncher) (resultErr error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	base := apiURL
	var ownedBackend *graphBackend
	if graph != "" {
		// Spawn the graph engine's API server as a sibling subprocess on a
		// free loopback port — the same engine-CLI contract cks setup uses.
		apiPort, err := freePort()
		if err != nil {
			return err
		}
		child, err := launch(ctx, graph, apiPort, stderr)
		if err != nil {
			return fmt.Errorf("spawn graph API: %w", err)
		}
		defer func() {
			if err := child.stopError(); err != nil {
				resultErr = errors.Join(resultErr, err)
			}
		}()
		ownedBackend = child
		base = fmt.Sprintf("http://127.0.0.1:%d", apiPort)
		if err := waitReady(ctx, base+"/api/manifest", 30*time.Second, child); err != nil {
			return fmt.Errorf("graph API backend not ready: %w", err)
		}
	}

	baseURL, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("--api-url: %w", err)
	}
	h, err := viewer.Handler(baseURL, os.Getenv("CKS_DEV_VIEWER_DIR"))
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	_, _ = fmt.Fprintf(stderr, "cks viewer: dashboard on http://%s (api backend %s)\n", addr, base)
	if open {
		go openBrowser("http://" + addr)
	}

	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		if ownedBackend == nil {
			<-ctx.Done()
			return
		}
		select {
		case <-ctx.Done():
		case <-ownedBackend.done:
			cancel()
		}
	}()
	defer func() { cancel(); <-stopped; <-watched }()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	if ownedBackend != nil && parent.Err() == nil {
		select {
		case <-ownedBackend.done:
			return fmt.Errorf("graph API exited while serving: %v", ownedBackend.err)
		default:
		}
	}
	return nil
}

// newExportCmd lays the embedded dashboard assets over a `ckg
// export-static` data bundle, completing a self-contained static site.
func newExportCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Copy the dashboard assets into a static export bundle",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if out == "" {
				return fmt.Errorf("--out is required")
			}
			if err := viewer.CopyAssetsTo(out); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "cks viewer export: dashboard assets copied to %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "static export directory (a 'ckg export-static' bundle)")
	return cmd
}

// siblingCkg prefers the ckg binary sitting next to the running cks
// executable, falling back to PATH resolution.
func siblingCkg() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "ckg")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return "ckg"
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}

func waitReady(parent context.Context, probeURL string, timeout time.Duration, backend *graphBackend) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	var exited <-chan struct{}
	if backend != nil {
		exited = backend.done
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := backend.exited(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if err != nil {
			return err
		}
		res, err := client.Do(req)
		if err == nil {
			_ = res.Body.Close()
			if ctx.Err() == nil && res.StatusCode == http.StatusOK {
				if err := backend.exited(); err != nil {
					return err
				}
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
			return backend.exited()
		case <-ticker.C:
		}
	}
}

// openBrowser launches the platform's default URL handler. The child
// launcher is reaped in the existing asynchronous caller. The user's browser
// remains independent and may outlive `cks viewer`.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Run()
}
