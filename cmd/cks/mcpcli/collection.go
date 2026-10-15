package mcpcli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/composer/budget"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/footprint"
	cksmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
)

func buildCollection(ctx context.Context, outer *config.Config, rules *config.SanitizeRuleset, cleaner *sanitize.Engine, fp *footprint.Logger) (*cksmcp.CollectionService, error) {
	if outer.Collection.CatalogPath == "" {
		return nil, nil
	}
	file, err := os.Open(outer.Collection.CatalogPath)
	if err != nil {
		return nil, collection.ErrCatalogInvalid
	}
	catalog, decodeErr := collection.Decode(file)
	closeErr := file.Close()
	if decodeErr != nil || closeErr != nil {
		return nil, collection.ErrCatalogInvalid
	}
	profiles := map[string]*config.Config{}
	for _, name := range outer.Collection.MemberConfigs {
		member, err := config.Load(name)
		if err != nil || member.Collection.CatalogPath != "" || len(member.Collection.MemberConfigs) != 0 || member.Sanitize != outer.Sanitize {
			return nil, collection.ErrCatalogInvalid
		}
		dir := memberVersionDir(member)
		if dir == "" || profiles[dir] != nil {
			return nil, collection.ErrDatasetMismatch
		}
		profiles[dir] = member
	}
	if len(profiles) != len(catalog.Versions) {
		return nil, collection.ErrDatasetMismatch
	}
	for _, version := range catalog.Versions {
		if profiles[version.VersionDir] == nil {
			return nil, collection.ErrDatasetMismatch
		}
	}
	return cksmcp.NewCollectionService(ctx, catalog, cleaner, func(ctx context.Context, version collection.Version) (cksmcp.Deps, func(), error) {
		return buildCollectionMember(ctx, profiles[version.VersionDir], version, rules, cleaner, fp)
	})
}

func memberVersionDir(cfg *config.Config) string {
	graph := filepath.Dir(cfg.Backends.CKG.Path)
	version := filepath.Dir(graph)
	if !filepath.IsAbs(version) || filepath.Base(graph) != "graph" || cfg.Backends.CKG.Path != filepath.Join(version, "graph", "graph.db") ||
		cfg.Backends.CKV.Path != filepath.Join(version, "vector") {
		return ""
	}
	return version
}

func buildCollectionMember(ctx context.Context, cfg *config.Config, version collection.Version, rules *config.SanitizeRuleset, cleaner *sanitize.Engine, fp *footprint.Logger) (cksmcp.Deps, func(), error) {
	// The collection session already holds the version lease. Do not resolve
	// current, inherit another member's source roots, or permit dummy fallback.
	deriveSourceRoot(cfg)
	reader, err := setup.OpenRetainedSourceReader(ctx, version.VersionDir)
	if err != nil {
		return cksmcp.Deps{}, nil, collection.ErrDatasetMismatch
	}
	be, err := buildBackends(ctx, cfg, reader)
	if err != nil {
		return cksmcp.Deps{}, nil, collection.ErrFetchFailed
	}
	if _, ok := be.ckg.(*ckgclient.Real); !ok {
		return cksmcp.Deps{}, be.close, collection.ErrFetchFailed
	}
	if _, ok := be.ckv.(*ckvclient.Real); !ok {
		return cksmcp.Deps{}, be.close, collection.ErrFetchFailed
	}
	alignment := computeStartupAlignment(ctx, cfg, be, filepath.Base(version.VersionDir))
	if alignment == nil || !alignment.OK {
		return cksmcp.Deps{}, be.close, collection.ErrDatasetMismatch
	}
	resolver, err := buildVocabResolver(cfg.Vocab.GlossaryPath)
	if err != nil {
		return cksmcp.Deps{}, be.close, collection.ErrCatalogInvalid
	}
	fetcher := &budget.FilesystemFetcher{Root: cfg.Backends.CKG.SourceRoot, ReadSource: reader}
	c, err := buildComposer(ctx, be.ckg, be.ckv, be.intentEmb, fetcher, rules, resolver, fp,
		cfg.Semantic.OntologyMode != "", cfg.Retrieval.EffectiveRecallK(),
		ontologyOptions(cfg.Semantic, version.VersionDir, cfg.Backends.CKG.SourceRoot, be.ckv, cfg.Retrieval.EffectiveRecallK())...)
	if err != nil {
		return cksmcp.Deps{}, be.close, collection.ErrFetchFailed
	}
	return cksmcp.Deps{Composer: c, CKG: be.ckg, CKV: be.ckv, EvidenceVersionDir: version.VersionDir,
		EvidenceSanitizer: cleaner, EvidenceSourceRoot: cfg.Backends.CKG.SourceRoot, Alignment: alignment,
		SemanticStorePath: cfg.Semantic.StorePath, SemanticOntologyMode: cfg.Semantic.OntologyMode,
		SemanticBudgetMS: cfg.Semantic.OntologyBudgetMS, TypeNeighbors: cfg.Retrieval.TypeNeighbors}, be.close, nil
}
