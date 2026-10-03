package mcpcli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

type retainedOntologyProvider struct {
	storePath, versionDir, sourceRoot string
	expected                          *setup.DatasetIdentity
	initialError                      error
}

func ontologyOptions(cfg config.SemanticConfig, versionDir, sourceRoot string, ckv ckvclient.Client, recallK int) []stage2.Option {
	if cfg.OntologyMode != "relations" && cfg.OntologyMode != "concept_text" && cfg.OntologyMode != "combined" {
		return nil
	}
	p := &retainedOntologyProvider{storePath: cfg.StorePath, versionDir: versionDir, sourceRoot: sourceRoot}
	if versionDir == "" {
		p.initialError = stage2.ErrOntologyUnavailable
	} else {
		p.expected, p.initialError = setup.InspectVersionIdentity(versionDir)
		if p.expected == nil && p.initialError == nil {
			p.initialError = stage2.ErrOntologyUnavailable
		}
	}
	budget := cfg.OntologyBudgetMS
	if budget == 0 {
		budget = 1000
	}
	opts := []stage2.Option{stage2.WithOntologyProvider(p, 0.2, time.Duration(budget)*time.Millisecond)}
	if cfg.OntologyMode == "concept_text" || cfg.OntologyMode == "combined" {
		opts = append(opts, stage2.WithOntologyTextSearch(ckv, recallK, cfg.OntologyMode == "combined"))
	}
	return opts
}

func (p *retainedOntologyProvider) Resolve(ctx context.Context) (stage2.OntologyResolver, error) {
	if p.initialError != nil {
		return nil, p.initialError
	}
	if p.expected == nil || p.storePath == "" {
		return nil, stage2.ErrOntologyUnavailable
	}
	info, err := os.Stat(p.storePath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, stage2.ErrOntologyUnavailable
	}
	store, err := semantic.OpenStoreReadOnly(ctx, p.storePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", stage2.ErrOntologyStale, err)
	}
	defer store.Close()
	active, err := store.LoadAlignedRetained(ctx, p.expected.Source.ProjectID, p.expected.DatasetID,
		p.sourceRoot, filepath.Join(p.versionDir, "graph"), filepath.Join(p.versionDir, "vector"), p.versionDir)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, stage2.ErrOntologyUnavailable
		}
		return nil, fmt.Errorf("%w: %v", stage2.ErrOntologyStale, err)
	}
	snapshot := active.Snapshot()
	if snapshot.ProjectID != p.expected.Source.ProjectID || snapshot.DatasetID != p.expected.DatasetID ||
		snapshot.SnapshotID != p.expected.Source.SnapshotID || snapshot.Commit != p.expected.Source.SourceCommit {
		return nil, stage2.ErrOntologyStale
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return active, nil
}
