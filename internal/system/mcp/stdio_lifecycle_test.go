package mcp

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// These tests use a real stdio input loop and workers; keep them sequential
// because the upstream library has one package-global stdio session.
func TestStdioIdleCancellationWithInputOpen(t *testing.T) {
	in, writer := io.Pipe()
	defer func() { _ = in.Close(); _ = writer.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runStdio(ctx, mcpserver.NewMCPServer("test", "1"), in, io.Discard) }()
	cancel()
	awaitStdioExit(t, done)
}

func TestStdioCancellationReachesActiveHandler(t *testing.T) {
	in, writer := io.Pipe()
	defer func() { _ = in.Close(); _ = writer.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, stopped := make(chan struct{}), make(chan struct{})
	server := mcpserver.NewMCPServer("test", "1")
	server.AddTool(mcpgo.NewTool("wait"), func(ctx context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		close(entered)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	})
	done := make(chan error, 1)
	go func() { done <- runStdio(ctx, server, in, io.Discard) }()
	if _, err := io.WriteString(writer, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"wait","arguments":{}}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	awaitStdioExit(t, done)
	select {
	case <-stopped:
	default:
		t.Fatal("server returned before handler stopped")
	}
}

func TestStdioEOFDrainsAcceptedCall(t *testing.T) {
	entered, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	server := mcpserver.NewMCPServer("test", "1")
	server.AddTool(mcpgo.NewTool("wait"), func(ctx context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		close(stopped)
		return mcpgo.NewToolResultText("completed"), nil
	})
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"wait","arguments":{}}}` + "\n")
	done := make(chan error, 1)
	go func() { done <- runStdio(context.Background(), server, input, io.Discard) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}
	select {
	case err := <-done:
		t.Fatalf("EOF returned before accepted call completed: %v", err)
	default:
	}
	close(release)
	awaitStdioExit(t, done)
	select {
	case <-stopped:
	default:
		t.Fatal("accepted call not drained")
	}
}

func TestStdioReadFailureRemainsAnError(t *testing.T) {
	sentinel := errors.New("broken input")
	err := runStdio(context.Background(), mcpserver.NewMCPServer("test", "1"), failingStdioReader{sentinel}, io.Discard)
	if !errors.Is(err, sentinel) {
		t.Fatalf("read failure masked: %v", err)
	}
}

type failingStdioReader struct{ err error }

func (r failingStdioReader) Read([]byte) (int, error) { return 0, r.err }

func awaitStdioExit(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("normal shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stdio shutdown blocked")
	}
}
