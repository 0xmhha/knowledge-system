// vector-mcp is the standalone MCP server for the vector engine: semantic
// search, keyword search, invariants/conventions lookups, and index
// operations over a built vector data directory — without the graph engine
// or the fused system pipeline. Use it when an agent only needs semantic
// retrieval (e.g. non-code knowledge bases).
//
// Tool names follow the shared namespace rule (see pkg/mcp): the engine
// default root is "cks" (the convention this server has always spoken); a
// deployment overrides it via --namespace, KNOWLEDGE_MCP_NAMESPACE, or
// -ldflags.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/0xmhha/knowledge-system/internal/vector/embed/bgeonnx"
	"github.com/0xmhha/knowledge-system/internal/vector/embed/mock"
	"github.com/0xmhha/knowledge-system/internal/vector/query"
	kmcp "github.com/0xmhha/knowledge-system/pkg/mcp"
	"github.com/0xmhha/knowledge-system/pkg/vector/embed/ollama"
	ckvmcp "github.com/0xmhha/knowledge-system/pkg/vector/mcp"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
	flag "github.com/spf13/pflag"
)

func main() {
	out := flag.String("out", "./ckv-data", "vector data directory")
	httpAddr := flag.String("http", "", "HTTP listen address (e.g. :8080); empty uses stdio")
	embedder := flag.String("embedder", "", "embedding backend: mock (default), bgeonnx, ollama")
	modelDir := flag.String("model-dir", "", "local model directory (bgeonnx)")
	modelName := flag.String("model-name", "", "model name (bgeonnx registry lookup / ollama, default bge-m3)")
	embedDim := flag.Int("embed-dim", 0, "target embedding dimension (ollama)")
	namespace := flag.String("namespace", "", "tool-namespace root override (default: env/build-time, else \"cks\")")
	flag.Parse()

	if err := run(*out, *httpAddr, *embedder, *modelDir, *modelName, *embedDim, *namespace); err != nil {
		fmt.Fprintf(os.Stderr, "vector-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run(out, httpAddr, embedder, modelDir, modelName string, embedDim int, namespace string) error {
	emb, cleanup, err := resolveEmbedder(embedder, modelDir, modelName, embedDim)
	if err != nil {
		return err
	}
	defer cleanup()

	eng, err := query.Open(out, emb)
	if err != nil {
		if errors.Is(err, query.ErrIndexUnavailable) {
			fmt.Fprintln(os.Stderr, "vector-mcp:", err)
		}
		return err
	}
	defer eng.Close()

	ckvmcp.SetNamespaceRoot(kmcp.Root(namespace, "cks"))
	srv := ckvmcp.NewServer(eng)

	if httpAddr != "" {
		fmt.Fprintf(os.Stderr, "vector-mcp: HTTP listening on %s\n", httpAddr)
		return srv.ServeHTTP(httpAddr)
	}
	// stdio default: stdout is reserved for JSON-RPC frames.
	fmt.Fprintf(os.Stderr, "vector-mcp: stdio server bound to %s\n", out)
	return srv.ServeStdio()
}

// resolveEmbedder mirrors the vector CLI's embedder selection with explicit
// parameters instead of global flags.
func resolveEmbedder(name, modelDir, modelName string, embedDim int) (types.Embedder, func(), error) {
	noop := func() {}
	switch name {
	case "", "mock":
		return mock.Default(), noop, nil
	case "bgeonnx":
		a, err := bgeonnx.Open(bgeonnx.Options{ModelDir: modelDir, ModelName: modelName})
		if err != nil {
			return nil, noop, fmt.Errorf("embedder bgeonnx: %w", err)
		}
		return a, func() { _ = a.Close() }, nil
	case "ollama":
		if modelName == "" {
			modelName = "bge-m3"
		}
		a, err := ollama.Open(ollama.Options{ModelName: modelName, TargetDim: embedDim})
		if err != nil {
			return nil, noop, fmt.Errorf("embedder ollama: %w", err)
		}
		return a, func() { _ = a.Close() }, nil
	default:
		return nil, noop, errors.New("unknown --embedder " + name + " (supported: mock, bgeonnx, ollama)")
	}
}
