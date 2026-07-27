package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/system/config"
	cksmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	"github.com/0xmhha/knowledge-system/internal/system/netutil"
)

// runPrintMCPConfig emits a ready-to-paste MCP client config entry (the Claude
// Code .mcp.json shape) for the server described by --config, so an operator or
// agent can register this instance without hand-assembling the URL/command.
// For HTTP transport it prints a reachable URL (the LAN IP for a wildcard bind);
// for stdio it prints the command + args to launch this binary.
func runPrintMCPConfig(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("print-mcp-config", flag.ContinueOnError)
	fs.SetOutput(stdout)
	configPath := fs.String("config", "", "cks config to describe (required)")
	nameOverride := fs.String("name", "", "override the server key in the emitted config")
	httpAddrOverride := fs.String("http-addr", "", "override the config's listen address in the emitted URL (match a --http-addr'd instance)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return fmt.Errorf("-config is required")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	name := cfg.Name
	if *nameOverride != "" {
		name = *nameOverride
	}
	if name == "" {
		name = cksmcp.DefaultInstanceName()
	}

	// A single config file backs several instances via --http-addr overrides, so
	// honor the same override here to emit the URL of the instance actually run.
	bindAddr := cfg.Listen.HTTPAddr
	if *httpAddrOverride != "" {
		bindAddr = *httpAddrOverride
	}

	var entry map[string]any
	if *httpAddrOverride != "" || cfg.Listen.ResolvedTransport() == "http" {
		entry = map[string]any{
			"type": "http",
			"url":  "http://" + netutil.AdvertiseHostPort(bindAddr) + "/mcp",
		}
	} else {
		self, err := os.Executable()
		if err != nil {
			self = "system-mcp"
		}
		abs, err := filepath.Abs(*configPath)
		if err != nil {
			abs = *configPath
		}
		entry = map[string]any{
			"command": self,
			"args":    []string{"--config", abs},
		}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"mcpServers": map[string]any{name: entry}})
}
