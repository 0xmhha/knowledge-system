package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// exportedJSON is the part of an export-json file the tests read.
type exportedJSON struct {
	Header struct {
		SchemaVersion string         `json:"schema_version"`
		Stats         map[string]int `json:"stats"`
		Exported      map[string]int `json:"exported"`
		LeftOut       []string       `json:"left_out"`
	} `json:"header"`
	Nodes []struct {
		ID     string `json:"id"`
		IsTest *bool  `json:"is_test"`
	} `json:"nodes"`
	Edges   []json.RawMessage `json:"edges"`
	NodePRs []struct {
		NodeID string `json:"node_id"`
		Number int    `json:"number"`
	} `json:"node_prs"`
}

// TestExportJSON_CarriesTestRoleAndPRs: the v1 export holds each node's
// test role (is_test) and the node_prs rows, its header counts what the
// arrays hold, and it names what it leaves out. The export dropped is_test
// and node_prs while the help called it lossless, and its header stats
// (the manifest's) did not match the arrays.
func TestExportJSON_CarriesTestRoleAndPRs(t *testing.T) {
	dir := graphExportFixture(t, exportManifest("committed"), false)
	s, err := persist.Open(filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RebuildReviewIndex([]string{"b.go"}); err != nil {
		t.Fatal(err)
	}
	merged := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := s.InsertNodePRs(map[string][]types.PRRef{"aaaaaaaaaaaaaaaa": {{Number: 7, Title: "t", MergedAtUTC: merged}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for _, minimal := range []bool{false, true} {
		out := filepath.Join(t.TempDir(), "g.json")
		cmd := newExportJSONCmd()
		args := []string{"--graph", dir, "--out", out}
		if minimal {
			args = append(args, "--minimal")
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var got exportedJSON
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("minimal=%v: %v", minimal, err)
		}
		roles := map[string]bool{}
		for _, n := range got.Nodes {
			if n.IsTest == nil {
				t.Fatalf("minimal=%v: node %s has no is_test", minimal, n.ID)
			}
			roles[n.ID] = *n.IsTest
		}
		if !roles["bbbbbbbbbbbbbbbb"] || roles["aaaaaaaaaaaaaaaa"] {
			t.Errorf("minimal=%v: is_test = %v, want b.go test and a.go not", minimal, roles)
		}
		if len(got.NodePRs) != 1 || got.NodePRs[0].Number != 7 || got.NodePRs[0].NodeID != "aaaaaaaaaaaaaaaa" {
			t.Errorf("minimal=%v: node_prs = %+v, want one row for PR 7", minimal, got.NodePRs)
		}
		want := map[string]int{"nodes": len(got.Nodes), "edges": len(got.Edges), "node_prs": len(got.NodePRs)}
		for k, v := range want {
			if got.Header.Exported[k] != v {
				t.Errorf("minimal=%v: header.exported[%s] = %d, want %d", minimal, k, got.Header.Exported[k], v)
			}
		}
		if len(got.Header.LeftOut) == 0 {
			t.Errorf("minimal=%v: header.left_out is empty", minimal)
		}
	}
}

// TestExportJSON_PRRowsHaveEveryField: a node_prs row carries every field
// the help lists, empty ones included. Empty repo and summary were left out.
func TestExportJSON_PRRowsHaveEveryField(t *testing.T) {
	b, err := json.Marshal(toNodePRRow("n", types.PRRef{Number: 1}))
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(b, &row); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"node_id", "number", "title", "summary", "base_sha", "head_sha", "merged_at", "repo"} {
		if _, ok := row[k]; !ok {
			t.Errorf("row %s lacks %s", b, k)
		}
	}
}

// TestExportV2_FailureNamesTheCause: a v2 export of a directory without
// graph.db (a failed build) fails with export_failed and says why. It said
// only "Graph export failed."
func TestExportV2_FailureNamesTheCause(t *testing.T) {
	dir := t.TempDir()
	err := exportGraphV2(t.Context(), dir, filepath.Join(t.TempDir(), "g.json"), false, false)
	var code graphExportError
	if !errors.As(err, &code) || code != "export_failed" {
		t.Fatalf("err = %v, want export_failed", err)
	}
	if cause := graphExportCause(err, code); !strings.Contains(cause, "graph.db") {
		t.Errorf("cause %q does not name graph.db", cause)
	}
}

// TestExportJSON_MissingGraphNamesTheFile: the v1 export and export-static
// of a directory without graph.db name the file. v1 said only "read
// manifest: unable to open database file (14)".
func TestExportJSON_MissingGraphNamesTheFile(t *testing.T) {
	dir := t.TempDir()
	for _, cmd := range []*cobra.Command{newExportJSONCmd(), newExportStaticCmd()} {
		cmd.SetArgs([]string{"--graph", dir, "--out", filepath.Join(t.TempDir(), "out")})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), filepath.Join(dir, "graph.db")) {
			t.Errorf("%s: err = %v, want it to name %s", cmd.Use, err, filepath.Join(dir, "graph.db"))
		}
	}
}

// TestExportJSON_HelpIsTrue: the help shows the real top level (a header
// object, the current schema), says what is left out instead of calling the
// export lossless, and names the build flags v2 needs.
func TestExportJSON_HelpIsTrue(t *testing.T) {
	long := newExportJSONCmd().Long
	for _, bad := range []string{"lossless", `"schema_version": "1.8"`} {
		if strings.Contains(long, bad) {
			t.Errorf("help still says %q", bad)
		}
	}
	for _, want := range append([]string{`"header"`, `"node_prs"`, "--project-id", "--dataset-id", "--snapshot-id",
		"--file-manifest-digest", "--capture-policy-digest", "--source-mode"}, exportLeftOutNames()...) {
		if !strings.Contains(long, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
}
