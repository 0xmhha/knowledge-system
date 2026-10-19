package persist_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func TestVisitGraphPinnedReadSurvivesPublishedReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.db")
	s, err := persist.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetManifest(persist.Manifest{ProjectID: "old"}); err != nil {
		t.Fatal(err)
	}
	ns := []types.Node{{ID: "b", Type: types.NodeFunction, Name: "B", QualifiedName: "p.B", FilePath: "b.go", Language: "go", Confidence: types.ConfExtracted}, {ID: "a", Type: types.NodeFunction, Name: "A", QualifiedName: "p.A", FilePath: "a.go", Language: "go", Confidence: types.ConfExtracted, Annotation: &types.AnnotationPayload{Description: "retained annotation"}}}
	if err := s.InsertNodes(ns); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEdges([]types.Edge{{Src: "b", Dst: "a", Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement.db")
	next, err := persist.Open(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := next.SetManifest(persist.Manifest{ProjectID: "new"}); err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := persist.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	var ids []string
	edgeCount := 0
	err = reader.VisitGraph(context.Background(), func(m persist.Manifest) error {
		if m.ProjectID != "old" {
			t.Fatalf("identity changed: %+v", m)
		}
		return nil
	}, func(n types.Node) error {
		ids = append(ids, n.ID)
		if len(ids) == 1 {
			if n.Annotation == nil || n.Annotation.Description != "retained annotation" {
				t.Fatal("node attrs lost", n)
			}
			return os.Rename(replacement, path)
		}
		return nil
	}, func(e types.Edge) error {
		edgeCount++
		if e.Src != "b" || e.Dst != "a" {
			t.Fatal(e)
		}
		return nil
	})
	if err != nil || len(ids) != 2 || ids[0] != "a" || ids[1] != "b" || edgeCount != 1 {
		t.Fatal(err, ids, edgeCount)
	}
	current, err := persist.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = current.Close() }()
	m, err := current.GetManifest()
	if err != nil || m.ProjectID != "new" {
		t.Fatal(err, m)
	}
}

func TestVisitGraphCallbackFailureAndCancellationReleaseRead(t *testing.T) {
	s, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	fail := errors.New("stop before nodes")
	nodeCalls := 0
	err = s.VisitGraph(context.Background(), func(persist.Manifest) error { return fail }, func(types.Node) error { nodeCalls++; return nil }, func(types.Edge) error { return nil })
	if !errors.Is(err, fail) || nodeCalls != 0 {
		t.Fatal(err, nodeCalls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.VisitGraph(ctx, func(persist.Manifest) error { return nil }, func(types.Node) error { return nil }, func(types.Edge) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.SetManifest(persist.Manifest{ProjectID: "still-writable"}); err != nil {
		t.Fatal("read transaction leaked", err)
	}
}
