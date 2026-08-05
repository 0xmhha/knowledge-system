// Command cks-agent is the coding agent CLI for cks.
//
// Phase D (Slim): builds an LLM-ready markdown prompt for a vibe-style
// task by calling cks-mcp over an MCP stdio subprocess. cks-agent does
// NOT itself invoke an LLM — the agent's job is to compose good
// context, the LLM-invocation layer (Claude CLI, OpenAI, etc.) is left
// to the caller who can pipe the markdown to whichever tool fits.
//
// Pipeline:
//
//	vibe prompt
//	  -> cks-mcp subprocess (spawned here)
//	    -> cks.context.get_for_task tool call
//	      -> contract.EvidencePack (returned over MCP)
//	  -> markdown formatter (cmd/cks-agent/format.go)
//	  -> stdout (or -output file)
//
// cks-agent talks to cks-mcp through the same MCP surface any external
// agent would, by design: this binary is meant to extract to a sibling
// repo cleanly. The only cks imports are the mcp-go upstream client
// and the wire-struct mirrors of contract.EvidencePack in format.go.
//
// Usage:
//
//	cks-agent -prompt "find where Login validates input"
//	echo "find Login" | cks-agent
//	cks-agent -prompt "..." -cks-mcp /usr/local/bin/cks-mcp -config ./cks.yaml -output prompt.md
package agentcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// NewCmd builds the `cks agent` command: compose an LLM-ready markdown
// prompt for a task by querying the fused MCP server.
func NewCmd() *cobra.Command {
	var prompt, mcpBinary, mcpConfig, output string
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Compose LLM-ready context for a task (prompt in, markdown out)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return run(ctx, prompt, mcpBinary, mcpConfig, output)
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "vibe prompt (when empty, reads from stdin)")
	cmd.Flags().StringVar(&mcpBinary, "cks-mcp", "", "MCP server binary (empty = this cks binary's own mcp subcommand)")
	cmd.Flags().StringVar(&mcpConfig, "config", "", "path to cks.yaml forwarded to the server")
	cmd.Flags().StringVar(&output, "output", "", "write markdown to this file (empty = stdout)")
	return cmd
}

func run(ctx context.Context, prompt, mcpBinary, mcpConfig, outputPath string) error {
	p, err := resolvePrompt(prompt, os.Stdin)
	if err != nil {
		return err
	}

	agent, err := NewAgent(ctx, AgentOpts{
		CKSMCPBinary: mcpBinary,
		CKSMCPConfig: mcpConfig,
	})
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	defer func() { _ = agent.Close() }()

	out, closer, err := openOutput(outputPath)
	if err != nil {
		return fmt.Errorf("open output: %w", err)
	}
	defer func() { _ = closer() }()

	return agent.Run(ctx, p, out)
}

// resolvePrompt picks the prompt source: -prompt flag wins when set;
// otherwise read all of stdin and trim trailing whitespace. Empty
// stdin is rejected so the caller knows they need to provide input.
func resolvePrompt(flagPrompt string, stdin io.Reader) (string, error) {
	if flagPrompt != "" {
		return flagPrompt, nil
	}
	buf, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	s := strings.TrimSpace(string(buf))
	if s == "" {
		return "", errors.New("no prompt provided: pass -prompt or pipe into stdin")
	}
	return s, nil
}

// openOutput selects the writer + closer based on path. Empty path
// returns os.Stdout and a no-op closer. The agent buffers internally
// so we don't need bufio here.
func openOutput(path string) (io.Writer, func() error, error) {
	if path == "" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}
