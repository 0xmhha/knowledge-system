// Package mcp carries the deploy-time identity knobs shared by every MCP
// server surface in this repository (graph, vector, and the fused system
// server).
//
// Tool names are two-part: a NAMESPACE ROOT that identifies the deployment
// (empty for the bare historical names, "cks" for the fused-server
// convention, or a product name like "stablenet_knowledge" for a branded
// deployment) and a BASE NAME that identifies the tool ("context.find_symbol",
// "ops.health", ...). The root is deployment identity — it must be injectable
// without code changes, which is what this package provides:
//
//	precedence: explicit (config / flag) > environment > build-time > engine default
//
// Build-time injection:
//
//	go build -ldflags "-X github.com/0xmhha/knowledge-system/pkg/mcp.BuildRoot=stablenet_knowledge" ./...
//
// Environment injection: set KNOWLEDGE_MCP_NAMESPACE.
package mcp

import "os"

// BuildRoot is the build-time namespace root, injected via -ldflags -X.
// Empty unless a deployment stamps one in.
var BuildRoot string

// EnvRoot is the environment variable Root consults between the explicit
// value and the build-time value.
const EnvRoot = "KNOWLEDGE_MCP_NAMESPACE"

// Root resolves the namespace root for one server surface. explicit comes
// from config or a CLI flag; fallback is the engine's historical default
// ("cks" for the fused system server and the vector server, "" for the graph
// server's bare names).
func Root(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv(EnvRoot); env != "" {
		return env
	}
	if BuildRoot != "" {
		return BuildRoot
	}
	return fallback
}

// Name joins a namespace root and a base tool name. An empty root returns the
// base unchanged (bare-name mode); otherwise the two are joined with a dot:
// Name("cks", "context.find_symbol") == "cks.context.find_symbol".
func Name(root, base string) string {
	if root == "" {
		return base
	}
	return root + "." + base
}
