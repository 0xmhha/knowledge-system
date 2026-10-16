package mcp

import (
	"context"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// Owned integration fixture server used by bench/suspects/smoke.py and an
// opt-in real host check. Normal tests never bind a port or read stdin.
func TestServeSuspectFixture(t *testing.T) {
	path := os.Getenv("KS_SUSPECT_FIXTURE_GRAPH")
	if path == "" {
		t.Skip("explicit fixture server only")
	}
	g, e := ckgclient.NewReal(path)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := g.Close(); e != nil {
			t.Error(e)
		}
	}()
	f := newFixture(t, nil)
	f.deps.CKG = g
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	if addr := os.Getenv("KS_SUSPECT_FIXTURE_HTTP"); addr != "" {
		e = RunHTTP(ctx, f.deps, addr, HTTPPolicy{})
	} else {
		e = Run(ctx, f.deps)
	}
	if e != nil {
		t.Fatal(e)
	}
}
