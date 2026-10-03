package mcpcli

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

func TestOntologyOptionsAreOptInAndMissingInputFallsBack(t *testing.T) {
	for _, mode := range []string{"", "off", "baseline"} {
		if len(ontologyOptions(config.SemanticConfig{OntologyMode: mode}, "", "", nil)) != 0 {
			t.Fatalf("%s enabled ontology", mode)
		}
	}
	for _, mode := range []string{"concept_text", "combined"} {
		cfg := config.Default()
		cfg.Semantic.OntologyMode = mode
		if err := cfg.Validate(); err != nil {
			t.Fatal(err)
		}
		if len(ontologyOptions(cfg.Semantic, "", "", nil)) != 2 {
			t.Fatal("text arm not connected")
		}
	}
	p := &retainedOntologyProvider{}
	if _, err := p.Resolve(context.Background()); !errors.Is(err, stage2.ErrOntologyUnavailable) {
		t.Fatal("missing store/version must fall back")
	}
	if len(ontologyOptions(config.SemanticConfig{OntologyMode: "relations"}, "", "", nil)) != 1 {
		t.Fatal("relations not connected")
	}
	for _, mode := range []string{"typo"} {
		cfg := config.Default()
		cfg.Semantic.OntologyMode = mode
		if err := cfg.Validate(); err == nil {
			t.Fatal("unknown arm silently accepted")
		}
	}
}

func TestOntologyProviderDoesNotBorrowOtherDatasetOrCurrentPointer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.db")
	s, err := semantic.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	p := &retainedOntologyProvider{storePath: path, expected: &setup.DatasetIdentity{Source: setup.SourceIdentity{ProjectID: "required-project"}, DatasetID: "required-dataset"}}
	if _, err := p.Resolve(context.Background()); !errors.Is(err, stage2.ErrOntologyUnavailable) {
		t.Fatalf("missing pinned tuple not unavailable: %v", err)
	}
}
