package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDocumentCorporaConfigSelectsOnlyExplicitRoots(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "setup.yaml")
	if err := os.WriteFile(path, []byte("src: source\nout: dataset\ndocument_corpora: [notes, other]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := LoadConfigStrict(path)
	if err != nil || !reflect.DeepEqual(o.DocumentCorpora, []string{filepath.Join(root, "notes"), filepath.Join(root, "other")}) {
		t.Fatalf("document paths are not config-relative: %+v %v", o, err)
	}
	if err := os.WriteFile(path, []byte("src: source\nout: dataset\n"), 0600); err != nil {
		t.Fatal(err)
	}
	o, err = LoadConfigStrict(path)
	if err != nil || len(o.DocumentCorpora) != 0 {
		t.Fatalf("documents were selected implicitly: %+v %v", o, err)
	}
}

func TestDocumentRootsRefuseAmbiguousSelections(t *testing.T) {
	for name, o := range map[string]Options{
		"blank":         {DocumentCorpora: []string{" "}},
		"duplicate":     {DocumentCorpora: []string{"/notes", "/notes/../notes"}},
		"semantic":      {SemanticCorpus: "/notes", DocumentCorpora: []string{"/notes"}},
		"domain":        {DomainKnowledge: "/pack/domain", DerivedDir: "/derived", DocumentCorpora: []string{"/derived/domain-corpus"}},
		"flow":          {FlowCorpus: "/flows/corpus.jsonl", DocumentCorpora: []string{"/flows"}},
		"relative_flow": {FlowCorpus: "corpus.jsonl", DocumentCorpora: []string{"."}},
		"too_many":      {DocumentCorpora: make([]string, 17)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ExtraDocumentRoots(o); err == nil {
				t.Fatal("ambiguous input was accepted")
			}
		})
	}
}

func TestRetainedDocumentRootsUseExactlyVectorOriginOrder(t *testing.T) {
	o := Options{DomainKnowledge: "/pack/domain", DerivedDir: "/derived", SemanticCorpus: "/semantic", DocumentCorpora: []string{"/manager/notes", "/other/notes"}, FlowCorpus: "/flows/corpus.jsonl"}
	roots, err := RetainedDocumentRoots(o)
	want := []string{"/derived/domain-corpus", "/semantic", "/manager/notes", "/other/notes", "/flows"}
	if err != nil || !reflect.DeepEqual(roots, want) {
		t.Fatalf("retained origins disagree with vector input order: %+v %v", roots, err)
	}
	var calls []Step
	if err := PrepareRetainedCorpora(context.Background(), &o, corpusRecorder{&calls}, nil); err != nil {
		t.Fatal(err)
	}
	for index, root := range roots {
		if o.ExternalOrigins[index].Root != root {
			t.Fatalf("prepared roots changed order: %+v", o.ExternalOrigins)
		}
	}
}

func TestDocumentPlanVerifiesInputsAndActualContentAroundVectorBuild(t *testing.T) {
	o := Options{Src: "/src", Out: filepath.Join(t.TempDir(), "dataset"), DocumentCorpora: []string{"/notes"}}
	plan, err := BuildPlan(o)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, step := range plan.Steps {
		ids = append(ids, step.ID)
		if step.ID == "vector-build" && !strings.Contains(strings.Join(step.Cmd, " "), "--docs /notes") {
			t.Fatal("selected documents never reach CKV")
		}
	}
	if !reflect.DeepEqual(ids, []string{"graph-build", "document-corpora-verify", "vector-build", "verify-align", "verify-content"}) {
		t.Fatalf("document verification is missing/out of order: %+v", ids)
	}
	o.SkipVector = true
	if _, err := BuildPlan(o); err == nil {
		t.Fatal("graph-only build silently ignored selected documents")
	}
}

func TestDocumentInputDigestBindsBytesAdditionsDeletionAndRootOrder(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	file := filepath.Join(a, "reviewed.md")
	if err := os.WriteFile(file, []byte("# Actual supplied review\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "other.md"), []byte("# Other input\n"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := func(roots ...string) string {
		t.Helper()
		hash, err := ConfiguredInputDigest(Options{DocumentCorpora: roots})
		if err != nil {
			t.Fatal(err)
		}
		return hash
	}
	initial := digest(a, b)
	if initial == digest(b, a) {
		t.Fatal("different origin order collapsed")
	}
	if err := os.WriteFile(file, []byte("# Changed supplied review\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := digest(a, b)
	if changed == initial {
		t.Fatal("changed review bytes did not change input identity")
	}
	extra := filepath.Join(a, "extra.md")
	if err := os.WriteFile(extra, []byte("# Added\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if digest(a, b) == changed {
		t.Fatal("added document did not change input identity")
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if digest(a, b) != changed {
		t.Fatal("same exact input did not restore its identity")
	}
}

func TestDocumentPreflightRejectsEmptyBinaryLinkedAndMissingInput(t *testing.T) {
	for name, body := range map[string][]byte{"whitespace": []byte(" \n"), "binary": {0, 1, 2}, "invalid_utf8": {0xff}} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "note.md"), body, 0600); err != nil {
				t.Fatal(err)
			}
			if err := VerifyDocumentCorpora(context.Background(), []string{root}, CaptureLimits{}); err == nil {
				t.Fatal("unusable Markdown accepted")
			}
		})
	}
	for _, root := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing")} {
		if err := VerifyDocumentCorpora(context.Background(), []string{root}, CaptureLimits{}); err == nil {
			t.Fatal("missing/empty selected corpus was accepted")
		}
	}
	root, external := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "note.md"), []byte("# Not in selected corpus\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(external, "note.md"), filepath.Join(root, "note.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := VerifyDocumentCorpora(context.Background(), []string{root}, CaptureLimits{}); err == nil {
		t.Fatal("linked document was accepted")
	}
}

func TestDocumentPreflightHonorsResourceAndCancellationBounds(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("# A real note\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for name, limits := range map[string]CaptureLimits{
		"files": {MaxFiles: 1}, "file_bytes": {MaxFileBytes: 2}, "total_bytes": {MaxTotalBytes: 15},
	} {
		t.Run(name, func(t *testing.T) {
			if err := VerifyDocumentCorpora(context.Background(), []string{root}, limits); !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("resource limit was ignored: %v", err)
			}
		})
	}
	if err := VerifyDocumentCorpora(context.Background(), []string{root}, CaptureLimits{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := VerifyDocumentCorpora(ctx, []string{root}, CaptureLimits{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was ignored: %v", err)
	}
}

func TestDocumentContentMustReachActualVectorManifest(t *testing.T) {
	dir := t.TempDir()
	vector := filepath.Join(dir, "vector")
	if err := os.Mkdir(vector, 0700); err != nil {
		t.Fatal(err)
	}
	o := Options{Out: dir, DocumentCorpora: []string{"/reviewed"}}
	for name, manifest := range map[string]map[string]any{
		"root_missing": {"docs_roots": []string{}, "languages": map[string]int{"markdown": 1}},
		"no_chunks":    {"docs_roots": []string{"/reviewed"}, "languages": map[string]int{}},
	} {
		t.Run(name, func(t *testing.T) {
			writeManifest(t, vector, manifest)
			if err := VerifyContent(o, nil); err == nil {
				t.Fatal("zero-exit index omitted declared document content")
			}
		})
	}
	writeManifest(t, vector, map[string]any{"docs_roots": []string{"/reviewed"}, "languages": map[string]int{"markdown": 1}})
	if err := VerifyContent(o, nil); err != nil {
		t.Fatal(err)
	}
}
