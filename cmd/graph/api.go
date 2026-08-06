package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/graph/server"
)

// newAPICmd serves the graph REST API (/api/*). The dashboard UI lives
// with the composition engine — `cks viewer` spawns this server and
// reverse-proxies to it.
func newAPICmd() *cobra.Command {
	var graph, dbDsn string
	var port int
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Serve the graph REST API over HTTP (/api/*)",
		RunE: func(cmd *cobra.Command, args []string) error {
			log, cleanup, err := newLogger(rootVerbose, rootLogFile)
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}
			defer cleanup()

			// Require exactly one of --graph or --db.
			if graph == "" && dbDsn == "" {
				return fmt.Errorf("one of --graph or --db must be provided")
			}
			return runAPI(apiOpts{
				GraphDir: graph, DBDsn: dbDsn, Port: port, Log: log,
			})
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory containing graph.db")
	cmd.Flags().StringVar(&dbDsn, "db", "",
		"PostgreSQL DSN (e.g. postgres://user:pass@host/dbname); if set, read graph from PG (--graph not required). DEPRECATED (ADR-0003): SQLite is the sole maintained backend")
	// Default port 8080 matches the EXECUTION-GUIDE S0 spec
	// ("HTTP 127.0.0.1:8080 loopback bind"). Operators who run multiple
	// CKG instances or already use 8080 can override via --port.
	cmd.Flags().IntVar(&port, "port", 8080, "HTTP port")
	// --graph is no longer required when --db is provided; enforce manually in RunE.
	return cmd
}

// apiOpts groups the runAPI parameters so newAPICmd and the quickstart
// command can share the same path without an unwieldy positional
// signature.
type apiOpts struct {
	GraphDir string // path to the graph.db dir (mutually exclusive with DBDsn)
	DBDsn    string // PostgreSQL DSN (overrides GraphDir when set)
	Port     int    // bind port on 127.0.0.1
	Log      *slog.Logger
}

// runAPI opens the configured store, builds the HTTP server, installs
// SIGINT/SIGTERM handlers, and blocks until the server exits. Extracted
// from the api cobra RunE so quickstart can compose it after build.
func runAPI(o apiOpts) error {
	var store persist.StoreReader
	var sourceLabel string
	var err error
	if o.DBDsn != "" {
		store, err = persist.OpenPostgresReadOnly(o.DBDsn)
		if err != nil {
			return fmt.Errorf("open postgres: %w", err)
		}
		sourceLabel = "postgres"
	} else {
		db := filepath.Join(o.GraphDir, "graph.db")
		store, err = persist.OpenReadOnly(db)
		if err != nil {
			return fmt.Errorf("open graph: %w", err)
		}
		sourceLabel = db
	}
	defer func() { _ = store.Close() }()

	srv := server.New(store, o.Log)

	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	addr := fmt.Sprintf("127.0.0.1:%d", o.Port)
	_, _ = fmt.Fprintf(os.Stderr, "ckg: serving %s API on http://%s/api/\n", sourceLabel, addr)
	_, _ = fmt.Fprintf(os.Stderr, "ckg: dashboard: cks viewer --api-url http://%s\n", addr)
	return srv.ListenAndServe(ctx, addr)
}
