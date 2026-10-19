package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func exportManifest(mode string) persist.Manifest {
	m := persist.Manifest{SchemaVersion: "1.8", ProjectID: "fixture-project", DatasetID: strings.Repeat("a", 64), SnapshotID: strings.Repeat("b", 64), FileManifestDigest: strings.Repeat("c", 64), CapturePolicyDigest: strings.Repeat("d", 64), SourceMode: mode, Stats: map[string]int{"nodes": 999}}
	if mode != "snapshot-only" {
		m.SrcCommit = strings.Repeat("e", 40)
	}
	return m
}

func graphExportFixture(t *testing.T, m persist.Manifest, empty bool) string {
	t.Helper()
	dir := t.TempDir()
	s, err := persist.Open(filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetManifest(m); err != nil {
		t.Fatal(err)
	}
	if !empty {
		nodes := []types.Node{
			{ID: "bbbbbbbbbbbbbbbb", Type: types.NodeFunction, Name: "B", QualifiedName: "p.B", FilePath: "b.go", StartLine: 2, EndLine: 3, Language: "go", Confidence: types.ConfExtracted},
			{ID: "aaaaaaaaaaaaaaaa", Type: types.NodeFunction, Name: "A", QualifiedName: "p.A", FilePath: "a.go", StartLine: 1, EndLine: 2, Language: "go", Confidence: types.ConfExtracted, Signature: "func A()"},
		}
		if err := s.InsertNodes(nodes); err != nil {
			t.Fatal(err)
		}
		if err := s.InsertEdges([]types.Edge{{Src: nodes[0].ID, Dst: nodes[1].ID, Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted}}); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGraphV2ExportModesAndMinimalPreserveCoordinates(t *testing.T) {
	for _, mode := range []string{"committed", "working-tree", "snapshot-only"} {
		for _, minimal := range []bool{false, true} {
			t.Run(mode+map[bool]string{true: "/minimal", false: "/full"}[minimal], func(t *testing.T) {
				m := exportManifest(mode)
				dir := graphExportFixture(t, m, false)
				out := filepath.Join(t.TempDir(), "export.json")
				cmd := newExportJSONCmd()
				args := []string{"--graph", dir, "--out", out, "--format=v2", "--pretty"}
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
				var doc struct {
					Format      string                     `json:"format"`
					Version     int                        `json:"format_version"`
					Coordinates map[string]string          `json:"coordinates"`
					Source      map[string]string          `json:"source"`
					Nodes       []map[string]any           `json:"nodes"`
					Edges       []types.Edge               `json:"edges"`
					Counts      struct{ Nodes, Edges int } `json:"counts"`
				}
				if err := json.Unmarshal(raw, &doc); err != nil {
					t.Fatal(err)
				}
				if doc.Format != "ckg-graph-v2" || doc.Version != 2 || doc.Coordinates["project_id"] != m.ProjectID || doc.Coordinates["dataset_id"] != m.DatasetID || doc.Coordinates["snapshot_id"] != m.SnapshotID || doc.Coordinates["source_mode"] != mode || doc.Coordinates["base_commit"] != m.SrcCommit || doc.Source["file_manifest_digest"] != m.FileManifestDigest || doc.Source["capture_policy_digest"] != m.CapturePolicyDigest {
					t.Fatal(string(raw))
				}
				if doc.Counts.Nodes != 2 || doc.Counts.Edges != 1 || len(doc.Nodes) != 2 || len(doc.Edges) != 1 || doc.Nodes[0]["id"] != "aaaaaaaaaaaaaaaa" {
					t.Fatal(string(raw))
				}
				_, signature := doc.Nodes[0]["signature"]
				if signature == minimal {
					t.Fatal("minimal shape", string(raw))
				}
				if bytes.Contains(raw, []byte("integrity_hash")) {
					t.Fatal("raw graph mislabeled as evidence")
				}
				if err := exportGraphV2(context.Background(), dir, out, false, false); err != graphExportError("output_exists") {
					t.Fatal(err)
				}
				after, _ := os.ReadFile(out)
				if !bytes.Equal(raw, after) {
					t.Fatal("overwrote destination")
				}
				entries, _ := os.ReadDir(filepath.Dir(out))
				if len(entries) != 1 {
					t.Fatal("temporary file leaked", entries)
				}
			})
		}
	}
}

func TestGraphV2EmptyArraysAndObservedCounts(t *testing.T) {
	dir := graphExportFixture(t, exportManifest("snapshot-only"), true)
	out := filepath.Join(t.TempDir(), "empty.json")
	if err := exportGraphV2(context.Background(), dir, out, false, false); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(out)
	if !bytes.Contains(raw, []byte(`"nodes":[]`)) || !bytes.Contains(raw, []byte(`"edges":[]`)) || !bytes.Contains(raw, []byte(`"counts":{"nodes":0,"edges":0}`)) {
		t.Fatal(string(raw))
	}
}

func TestGraphV2RejectsIncompleteIdentityBeforePublication(t *testing.T) {
	cases := []persist.Manifest{{}, exportManifest("unknown"), exportManifest("snapshot-only"), exportManifest("committed"), exportManifest("working-tree")}
	cases[2].SrcCommit = strings.Repeat("e", 40)
	cases[3].DatasetID = strings.Repeat("z", 64)
	cases[4].FileManifestDigest = ""
	for i, m := range cases {
		dir := graphExportFixture(t, m, true)
		out := filepath.Join(t.TempDir(), "bad.json")
		if err := exportGraphV2(context.Background(), dir, out, false, false); err != graphExportError("reindex_required") {
			t.Fatalf("%d: %v", i, err)
		}
		entries, _ := os.ReadDir(filepath.Dir(out))
		if len(entries) != 0 {
			t.Fatal("published invalid export", entries)
		}
	}
}

func TestLegacyGraphExportsRejectNoncommittedBeforeCreatingOutput(t *testing.T) {
	for _, mode := range []string{"working-tree", "snapshot-only"} {
		dir := graphExportFixture(t, exportManifest(mode), false)
		for _, static := range []bool{false, true} {
			out := filepath.Join(t.TempDir(), "output")
			cmd := newExportJSONCmd()
			if static {
				cmd = newExportStaticCmd()
			}
			cmd.SetArgs([]string{"--graph", dir, "--out", out})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err != graphExportError("requires_v2") {
				t.Fatal(err)
			}
			if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("created output", err)
			}
		}
	}
}

func TestLegacyGraphJSONDefaultShapeRemains(t *testing.T) {
	for _, m := range []persist.Manifest{{SchemaVersion: "1.8"}, exportManifest("committed")} {
		dir := graphExportFixture(t, m, false)
		out := filepath.Join(t.TempDir(), "legacy.json")
		cmd := newExportJSONCmd()
		cmd.SetArgs([]string{"--graph", dir, "--out", out})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(out)
		if bytes.Contains(raw, []byte(`"format_version"`)) || bytes.Contains(raw, []byte(`"coordinates"`)) || !bytes.Contains(raw, []byte(`"header"`)) {
			t.Fatal(string(raw))
		}
	}
}

func TestGraphV2PublicationStopsOnEncodeFailureAndCancellation(t *testing.T) {
	failure := errors.New("row iteration failed")
	for _, cancel := range []bool{false, true} {
		ctx, stop := context.WithCancel(context.Background())
		out := filepath.Join(t.TempDir(), "new.json")
		err := publishGraphV2(ctx, out, func(w io.Writer) error {
			if _, err := io.WriteString(w, "partial"); err != nil {
				return err
			}
			if cancel {
				stop()
				return nil
			}
			return failure
		})
		stop()
		if cancel {
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		} else if !errors.Is(err, failure) {
			t.Fatal(err)
		}
		entries, _ := os.ReadDir(filepath.Dir(out))
		if len(entries) != 0 {
			t.Fatal(entries)
		}
	}
}

type failedGraphWriter struct{}

func (failedGraphWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestGraphV2PropagatesFinalBufferedFlushFailure(t *testing.T) {
	dir := graphExportFixture(t, exportManifest("snapshot-only"), true)
	s, err := persist.OpenReadOnly(filepath.Join(dir, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := writeGraphV2(context.Background(), failedGraphWriter{}, s, false, false); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

func TestGraphV2InvalidFormatDoesNotOpenInput(t *testing.T) {
	cmd := newExportJSONCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--graph", "missing", "--out", filepath.Join(t.TempDir(), "bad"), "--format=invalid"})
	if err := cmd.Execute(); err != graphExportError("invalid_request") {
		t.Fatal(err)
	}
}
