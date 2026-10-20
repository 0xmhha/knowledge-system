package maintenance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

func TestRegisteredProfileRetainsDocumentAndFlowOriginOrder(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source, notes, flow := filepath.Join(root, "source"), filepath.Join(root, "notes"), filepath.Join(root, "flows")
	for _, path := range []string{source, notes, flow} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	o := setup.Options{Src: source, ProjectID: "fixture", DocumentCorpora: []string{notes}, FlowCorpus: filepath.Join(flow, "corpus.jsonl")}
	if err := attachInputs(&o); err != nil {
		t.Fatal(err)
	}
	if len(o.ExternalOrigins) != 2 || o.ExternalOrigins[0].ID != "docs:corpus.0" || o.ExternalOrigins[0].Root != notes ||
		o.ExternalOrigins[1].ID != "docs:corpus.1" || o.ExternalOrigins[1].Root != flow || !o.RetainedCorporaPrepared {
		t.Fatalf("registered profile differs from vector origin order: %+v", o)
	}
}

func TestRegisteredProfileRejectsUnavailableAndDuplicateDocumentRoots(t *testing.T) {
	root := t.TempDir()
	for _, roots := range [][]string{{filepath.Join(root, "missing")}, {root, root}} {
		o := setup.Options{Src: root, DocumentCorpora: roots}
		if err := attachInputs(&o); ErrorResult(err).Code != "input_stale" {
			t.Fatalf("invalid document input accepted: %v", err)
		}
	}
}

func TestRegisteredProfileRejectsLinkedDocumentRoot(t *testing.T) {
	root, selected := t.TempDir(), t.TempDir()
	alias := filepath.Join(root, "notes")
	if err := os.Symlink(selected, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	o := setup.Options{Src: root, DocumentCorpora: []string{alias}}
	if err := attachInputs(&o); ErrorResult(err).Code != "input_stale" {
		t.Fatalf("linked document input accepted: %v", err)
	}
}
