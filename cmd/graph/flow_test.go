package main

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/execflow"
)

// TestFlowCmdPrintsTheToolAnswer: `ckg flow` passes its flags to the
// execution flow query and prints the same text the tool answers with.
func TestFlowCmdPrintsTheToolAnswer(t *testing.T) {
	src, err := filepath.Abs("../../pkg/graph/execflow/testdata/flowfix")
	if err != nil {
		t.Fatal(err)
	}
	graph := t.TempDir()
	build := newBuildCmd()
	build.SetArgs([]string{"--src=" + src, "--out=" + graph, "--lang=go"})
	build.SetOut(io.Discard)
	build.SetErr(io.Discard)
	if err := build.Execute(); err != nil {
		t.Fatalf("build: %v", err)
	}
	var out bytes.Buffer
	flow := newFlowCmd()
	flow.SetArgs([]string{"--graph=" + graph, "--direction=backward", "--depth=2", "flowfix.Pool.add"})
	flow.SetOut(&out)
	if err := flow.Execute(); err != nil {
		t.Fatalf("flow: %v", err)
	}
	reader, err := persist.OpenReadOnly(filepath.Join(graph, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	want, err := execflow.Trace(reader, execflow.Query{Seed: "flowfix.Pool.add", Direction: execflow.Backward, Depth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Errorf("ckg flow printed:\n%s\nthe query answers:\n%s", out.String(), want)
	}
	bad := newFlowCmd()
	bad.SetArgs([]string{"--graph=" + graph, "--follow=jumps", "flowfix.Pool.add"})
	bad.SetOut(io.Discard)
	bad.SetErr(io.Discard)
	if err := bad.Execute(); err == nil {
		t.Error("an unknown link is accepted")
	}
}
