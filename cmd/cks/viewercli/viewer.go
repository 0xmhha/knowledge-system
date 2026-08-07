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

func runViewer(ctx context.Context, graph, apiURL string, port int, open bool, stderr interface{ Write([]byte) (int, error) }) error {
	base := apiURL
	if graph != "" {
		// Spawn the graph engine's API server as a sibling subprocess on a
		// free loopback port — the same engine-CLI contract cks setup uses.
		apiPort, err := freePort()
		if err != nil {
			return err
		}
		bin := siblingCkg()
		child := exec.CommandContext(ctx, bin, "api", "--graph", graph, "--port", fmt.Sprint(apiPort))
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			return fmt.Errorf("spawn %s api: %w", bin, err)
		}
		defer func() { _ = child.Process.Kill() }()
		base = fmt.Sprintf("http://127.0.0.1:%d", apiPort)
		if err := waitReady(ctx, base+"/api/manifest", 30*time.Second); err != nil {
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
	fmt.Fprintf(stderr, "cks viewer: dashboard on http://%s (api backend %s)\n", addr, base)
	if open {
		go openBrowser("http://" + addr)
	}

	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
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
			fmt.Fprintf(cmd.ErrOrStderr(), "cks viewer export: dashboard assets copied to %s\n", out)
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

func waitReady(ctx context.Context, probeURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		res, err := http.Get(probeURL)
		if err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("no 200 from %s within %s", probeURL, timeout)
}

// openBrowser launches the platform's default URL handler. The child
// process is intentionally detached (no Wait) — the browser may outlive
// `cks viewer`, and blocking on it would defeat the goroutine.
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
	_ = cmd.Start()
}
