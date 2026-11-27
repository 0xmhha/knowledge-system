package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
	kmcp "github.com/0xmhha/knowledge-system/pkg/mcp"
)

const mcpServerVersion = "0.1.0"

// newMCPCmd serves the graph tool set over MCP stdio. The client-visible
// tool names follow the shared namespace rule (see pkg/mcp): bare names by
// default, <root>.context.<name> when a namespace root is injected via
// --namespace, KNOWLEDGE_MCP_NAMESPACE, or -ldflags.
// contentFlagHelp is the help of ckg mcp --content.
const contentFlagHelp = "which blocks a tool result carries: both (a text block and structuredContent, the same answer twice), " +
	"text (the text block only) or structured (structuredContent only, with an empty content list). " +
	"The default is both because the MCP specification says a tool that returns structured content SHOULD also return " +
	"functionally equivalent unstructured content, and a client that reads only text content sees nothing otherwise; " +
	"the cost is a response about twice as large. The text of a JSON answer is its JSON; investigate and investigate_open " +
	"answer in prose, which is the text, and their structuredContent is {\"text\": <the prose>}. " +
	"--page-bytes bounds all blocks together; the investigate budget argument bounds the characters of the prose, " +
	"which under both is sent in each block"

func newMCPCmd() *cobra.Command {
	var graph, name, namespace, trace string
	var pageBytes int
	var resultDir string
	var maxResultChars int
	var noInstructions bool
	var toolTextFile string
	var nameAsBoth, reasonTail bool
	var content string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP stdio server",
		RunE: func(cmd *cobra.Command, args []string) error {
			log, cleanup, err := newLogger(rootVerbose, serverLogFile(rootLogFile, "mcp"))
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}
			defer cleanup()

			db := filepath.Join(graph, "graph.db")
			store, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()

			mode, err := mcphandlers.ParseContentMode(content)
			if err != nil {
				return fmt.Errorf("--content: %w", err)
			}
			mcphandlers.SetContentMode(mode)
			mcphandlers.SetNamespaceRoot(kmcp.Root(namespace, ""))
			// The default applies only when the flag is absent; a given
			// size below the minimum (0 and negatives too) is refused.
			if cmd.Flags().Changed("page-bytes") {
				if err := mcphandlers.ValidateSearchCodePageBytes(pageBytes); err != nil {
					return err
				}
				mcphandlers.SetSearchCodePageBytes(pageBytes)
			}
			if resultDir == "" {
				resultDir = os.Getenv("CKG_SEARCH_CODE_DIR")
			}
			mcphandlers.SetSearchCodeDir(resultDir)
			mcphandlers.SetSearchCodeMaxResultChars(maxResultChars)
			mcphandlers.SetSearchCodeOptions(nameAsBoth, reasonTail)
			// One record per tool call (area "mcp"), so an answer can be traced
			// to the call and arguments that produced it.
			opts := []server.ServerOption{
				server.WithToolHandlerMiddleware(mcphandlers.LogToolCalls(log)),
			}
			// The server guide goes next to the system prompt; --no-instructions
			// leaves it out so a run can measure what it changes.
			text, err := loadToolText(toolTextFile)
			if err != nil {
				return err
			}
			if !noInstructions {
				instructions := mcphandlers.Instructions()
				if text != nil {
					instructions = text.Instructions
				}
				opts = append(opts, server.WithInstructions(instructions))
			}
			// With --trace, also what each call did inside: matched and ranked
			// declarations, what the result kept and what it cut.
			if trace == "" {
				trace = os.Getenv("CKG_MCP_TRACE")
			}
			if trace != "" {
				f, err := os.OpenFile(trace, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
				if err != nil {
					return fmt.Errorf("open trace file: %w", err)
				}
				defer func() { _ = f.Close() }()
				opts = append(opts, server.WithToolHandlerMiddleware(mcphandlers.TraceToolCalls(f)))
			}
			s := server.NewMCPServer(name, mcpServerVersion, opts...)
			mcphandlers.RegisterAll(s, store)
			applyToolText(s, text)

			log.With("area", "mcp").Info("mcp server started", "graph", db)
			_, _ = fmt.Fprintf(os.Stderr, "ckg mcp: stdio server bound to %s\n", db)
			if err := server.ServeStdio(s); err != nil {
				return fmt.Errorf("serve stdio: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory (required)")
	cmd.Flags().StringVar(&name, "name", "knowledge-graph", "MCP server name reported in the handshake")
	cmd.Flags().StringVar(&namespace, "namespace", "", "tool-namespace root override (default: env/build-time, else bare names)")
	cmd.Flags().StringVar(&trace, "trace", "", "append one JSON line per tool call with what it matched, kept and cut (default: $CKG_MCP_TRACE; off when empty)")
	cmd.Flags().StringVar(&content, "content", string(mcphandlers.DefaultContentMode), contentFlagHelp)
	cmd.Flags().IntVar(&pageBytes, "page-bytes", 0, "largest search_code response in bytes: the content list (each text block with its type key) and the structuredContent value, as encoded on the wire, both under --content both (the JSON-RPC envelope around them is not counted); a larger result is sent in parts (default 40000, sized for Claude Code's 25,000-token tool output limit; at least "+
		strconv.Itoa(mcphandlers.MinSearchCodePageBytes)+", the smallest answer, or the server refuses to start)")
	cmd.Flags().StringVar(&resultDir, "result-dir", "", "folder where a search_code result larger than one response is saved as a JSON file; the response then holds only its path (default: $CKG_SEARCH_CODE_DIR; empty sends an index instead). The agent must be allowed to read it")
	cmd.Flags().IntVar(&maxResultChars, "max-result-chars", 0, "declare to the client (Claude Code _meta anthropic/maxResultSizeChars) the largest search_code result it should give the model inline; 0 declares none. The value is sent as given: the server does not cut answers to it, and how a client counts a response that carries both blocks is the client's")
	cmd.Flags().BoolVar(&noInstructions, "no-instructions", false, "do not send the server guide (instructions) in the MCP handshake; for measuring what the guide changes")
	cmd.Flags().BoolVar(&nameAsBoth, "name-as-both", false, "answer search_code in=name with in=both: name matches first, then body matches")
	cmd.Flags().BoolVar(&reasonTail, "reason-tail", false, "give search_code a reasons argument; graph neighbours of the hits that hold a reason word are listed after the hits under related")
	cmd.Flags().StringVar(&toolTextFile, "tool-text", "", "JSON file with the server guide and tool definitions another ckg build sent (instructions, tools by name); the server sends those texts instead of its own and hides tools the file does not list. For measuring what tool text changes, with behaviour held the same")
	_ = cmd.MarkFlagRequired("graph")
	return cmd
}

// toolText is what an MCP client saw from another ckg build: its server
// guide and its tool definitions.
type toolText struct {
	Instructions string              `json:"instructions"`
	Tools        map[string]mcp.Tool `json:"tools"`
}

func loadToolText(path string) (*toolText, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tool text: %w", err)
	}
	var t toolText
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("tool text %s: %w", path, err)
	}
	if len(t.Tools) == 0 {
		return nil, fmt.Errorf("tool text %s lists no tools", path)
	}
	return &t, nil
}

// applyToolText gives every tool the description and input schema of the
// same tool in text, keeping this build's handler and _meta, and removes the
// tools text does not list. Arguments only the old schema has reach the
// handler and are ignored by it.
func applyToolText(s *server.MCPServer, text *toolText) {
	if text == nil {
		return
	}
	var drop []string
	for name, st := range s.ListTools() {
		old, ok := text.Tools[name]
		if !ok {
			drop = append(drop, name)
			continue
		}
		t := st.Tool
		t.Description = old.Description
		t.InputSchema = old.InputSchema
		t.RawInputSchema = old.RawInputSchema
		s.AddTool(t, st.Handler)
	}
	s.DeleteTools(drop...)
}
