// Package buildpipe orchestrates the full Pass 1..4 build (spec §4.7):
// detect → parse → resolve → graph build/validate → cluster → score → persist.
// Every build is cold: it parses every file and rebuilds every artifact
// (see Run).
package buildpipe

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/graph/cluster"
	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/internal/graph/filterlist"
	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/internal/graph/link"
	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	solp "github.com/0xmhha/knowledge-system/internal/graph/parse/solidity"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/graph/score"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// emitDerivedPasses runs the post-graph.Build derived passes against g IN
// MEMORY: cross-language link (Sol→TS binds_to), temporal (commit nodes
// + changed_in/blame), cluster (pkg + topic), and score. Returns the cluster
// outputs so the caller can persist them.
//
// solParser nil = skip xlang; the build passes the parser when Sol files
// exist. The build wipes the store first (openColdStore).
func emitDerivedPasses(g *graph.Graph, srcRoot string, solParser *solp.Parser,
	log *slog.Logger, strict bool,
	temporalDepth int, filter *filterlist.FilterList,
) (*cluster.PkgTree, *cluster.TopicTree, map[string][]byte, error) {
	if solParser != nil {
		abi := convertABI(solParser.ABI())
		xlEdges := link.SolToTS(g.Nodes, abi)
		g.Edges = append(g.Edges, xlEdges...)
		if _, err := validateAndSanitize(g, log, "xlang", strict); err != nil {
			return nil, nil, nil, err
		}
		log.Info("xlang linked", "binds_to", len(xlEdges))
	}
	// HTTP client → server Endpoint matching (schema 1.9 §6.9).
	// Cascade resolves AMBIGUOUS placeholder Endpoints emitted by the per-
	// language parsers (Go: distributed.go, TS: http_client.go) against the
	// real server-side Endpoints. See internal/link/http_match.go
	// for the algorithm + §3.3 exact-match decision.
	newNodes, newEdges, httpResult := link.MatchHTTPClients(g.Nodes, g.Edges)
	g.Nodes = newNodes
	g.Edges = newEdges
	if _, err := validateAndSanitize(g, log, "http_match", strict); err != nil {
		return nil, nil, nil, err
	}
	log.Info("http client matching",
		"rewired", httpResult.Rewired,
		"specific_hits", httpResult.SpecificHits,
		"wildcard_hits", httpResult.WildcardHits,
		"ambiguous_retained", httpResult.AmbiguousRetained,
		"placeholders_dropped", httpResult.PlaceholdersDropped)
	hunkBlobs, err := emitTemporalEdges(g, srcRoot, log, temporalDepth, filter)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("temporal: %w", err)
	}
	if _, err := validateAndSanitize(g, log, "temporal", strict); err != nil {
		return nil, nil, nil, err
	}
	// Scores first: topic labels name each community after its highest-
	// PageRank member (cluster.LabelCommunity), so PageRank must be filled
	// before the topic tree is built. Neither tree reads or changes scores
	// otherwise, and nothing below changes nodes or edges.
	score.Compute(g)
	pkgTree := cluster.BuildPkgTree(g)
	topicTree := cluster.BuildTopicTree(g, []float64{0.5, 1.0, 2.0}, 42)
	return pkgTree, topicTree, hunkBlobs, nil
}

// Options controls one ckg build invocation.
type Options struct {
	ProjectID           string
	SnapshotID          string
	DatasetID           string
	FileManifestDigest  string
	CapturePolicyDigest string
	SourceMode          string
	SrcRoot             string
	LogicalSrcRoot      string // public project path when SrcRoot is an immutable build tree
	OutDir              string
	Languages           []string // {"auto"} | subset of {"go","ts","sol"}
	Logger              *slog.Logger
	CKGVersion          string
	// NoCache is accepted and changes nothing: every build is cold (Run).
	NoCache bool
	// DBDSN is an optional PostgreSQL DSN (e.g. "postgres://user:pass@host/db").
	// When set, the build persists to PostgreSQL instead of a local SQLite file.
	// OutDir is still used for manifest.json.
	DBDSN string
	// StrictValidate, when true, fails the build on the first dangling edge or
	// schema violation (legacy v0.x behaviour). Default false: dangling edges
	// are dropped with a warning, schema violations still abort. Lenient mode
	// is required for self-analysis, where parser bugs would
	// otherwise prevent graph.db from being written and block measurement.
	StrictValidate bool
	// FilesFromPath is the optional path to a JSON include/exclude filter
	// (see internal/filterlist). When set, only files matching the filter
	// reach the parsers. Empty means "use heuristic discovery as before".
	FilesFromPath string
	// Filter is an optional pre-built include/exclude filter. When non-nil it
	// takes precedence over FilesFromPath and is applied exactly as a loaded
	// --files-from JSON would be. This is the in-memory entry point for
	// generated filters (e.g. filterlist.GenerateFromMain for --files-from-main)
	// that never touch disk. Callers must not set both a non-nil Filter and a
	// non-empty FilesFromPath.
	Filter *filterlist.FilterList

	// FileRoles gives the role of each file of the build scope (ckg build
	// --scope): whether its nodes are test code follows the role instead of
	// the file's path. nil (a build without a scope) keeps the path rule.
	FileRoles FileRoles
	// GoBuild is the Go build configuration the graph is built for (from a
	// build scope, ckg build --scope). It sets the platform, cgo and tags of
	// both Go discovery and type checking; nil keeps the host defaults.
	GoBuild *detect.GoBuild
	// LockPropagation also records the fields used in a function called
	// while a mutex is held, as long as the mutex is still held at the use
	// (gop.LockOptions.Propagate): accessed_under_lock edges with the
	// dispatch kind PropagatedLockKind, up to gop.PropagationDepth calls
	// deep. Default false.
	LockPropagation bool

	// PolicyFile is the optional path to a governance/protocol policy
	// YAML (pkg/policy). When set, the cold build loads + resolves the
	// policy entries against the parsed graph and emits NodePolicy +
	// EdgeGovernedBy rows so an LLM can answer "what policy governs
	// this code?" without leaving the graph. Empty means "no policy
	// enrichment" — existing builds stay byte-identical.
	PolicyFile string

	// SecurityPatternFile is the optional path to a security risk
	// pattern YAML (pkg/security). Sibling to PolicyFile but for the
	// risk-surface axis: reentrancy, access-control, Byzantine,
	// overflow patterns flagged against specific code symbols. Empty
	// means "no security enrichment".
	SecurityPatternFile string

	// TemporalDepth caps the per-file commit count the temporal pass
	// walks when emitting Commit/Hunk nodes and changed_in/blame edges.
	// 0 (the zero value) means "use the built-in default" (temporalDepthDefault
	// = 10). Raising it deepens commit-level history at a roughly linear cost
	// in Commit/Hunk nodes, changed_in edges, and graph size; node_prs symbol
	// history is independent of this cap (see pr_history.go).
	TemporalDepth int
}

// resolveFilter returns the effective include/exclude filter for the build:
// the pre-built Filter when set, otherwise the one loaded from FilesFromPath
// (nil when neither is configured — "no filter").
//
// Build resolves it once and hands the result to whichever run path it picks.
// Every pass that scopes by it — discovery, and the temporal pass that
// re-enumerates from git — must see the same list, so it is passed down rather
// than re-read per path.
func (opt Options) resolveFilter() (*filterlist.FilterList, error) {
	if opt.Filter != nil {
		return opt.Filter, nil
	}
	return filterlist.Load(opt.FilesFromPath)
}

// validateAndSanitize runs the lenient/strict validation gate against g and
// returns (droppedDanglingCount, error). Schema errors always abort. Dangling
// edges abort only when strict; otherwise they are dropped in place and
// surfaced via warn-level logs grouped by edge type.
func validateAndSanitize(g *graph.Graph, log *slog.Logger, stage string, strict bool) (int, error) {
	report := graph.Inspect(g)
	if report.HasSchemaErrors() {
		return 0, fmt.Errorf("graph.Validate(%s): %w", stage, report.SchemaErrors[0])
	}
	if !report.HasDangling() {
		return 0, nil
	}
	if strict {
		d := report.DanglingEdges[0]
		side := "src"
		if !d.Src && d.Dst {
			side = "dst"
		}
		return 0, fmt.Errorf("graph.Validate(%s): dangling %s on edge of type %s: %s -> %s",
			stage, side, d.Edge.Type, d.Edge.Src, d.Edge.Dst)
	}
	dropped := graph.Sanitize(g, report)
	for et, n := range report.CountByEdgeType() {
		log.Warn("dangling edges dropped", "stage", stage, "edge_type", string(et), "count", n)
	}
	return dropped, nil
}

// Run executes the full pipeline. Side effects: writes OutDir/graph.db
// and OutDir/manifest.json. Returns the persisted Manifest summary so the
// caller can print stats without re-reading SQLite.
//
// Every build is cold: it parses every file and recomputes every artifact,
// whatever graph OutDir holds (Options.NoCache changes nothing). The
// incremental build was removed because it did not reproduce the cold graph.
func Run(opt Options) (persist.Manifest, error) {
	if opt.ProjectID != "" || opt.SnapshotID != "" || opt.DatasetID != "" || opt.FileManifestDigest != "" || opt.CapturePolicyDigest != "" || opt.SourceMode != "" {
		if opt.ProjectID == "" || opt.SnapshotID == "" || opt.DatasetID == "" || opt.FileManifestDigest == "" || opt.CapturePolicyDigest == "" ||
			(opt.SourceMode != "committed" && opt.SourceMode != "working-tree" && opt.SourceMode != "snapshot-only") {
			return persist.Manifest{}, fmt.Errorf("incomplete or unsupported pinned graph source identity")
		}
	}
	log := opt.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	if err := os.MkdirAll(opt.OutDir, 0o755); err != nil {
		return persist.Manifest{}, fmt.Errorf("mkdir out: %w", err)
	}

	// (1) detect — discovery is shared by all three paths.
	// TS/Sol use extension-based discovery (detect.Walk); Go uses
	// go/packages.Load (detect.GoFiles) to honor build constraints. See
	// pipeline_test.go for the 41-file drift this eliminates.
	log.Debug("discovery.start", "src", opt.SrcRoot)
	filter, err := opt.resolveFilter()
	if err != nil {
		return persist.Manifest{}, err
	}
	discovery, _, goCount, tsCount, solCount, protoCount, err := discoveryAll(opt.SrcRoot, opt.Languages, filter, opt.GoBuild)
	if err != nil {
		return persist.Manifest{}, err
	}
	log.Info("detected files", "go", goCount, "ts", tsCount, "sol", solCount, "proto", protoCount)
	log.Debug("discovery.end", "total", goCount+tsCount+solCount+protoCount)

	// (2) every build is cold.
	logBuildPath(log, opt)
	return runCold(opt, log, discovery, filter)
}

// runCold is the V0-equivalent full-rebuild path: wipe DB, parse every file,
// rebuild every artifact. Always emits a fresh manifest (with Files block).
func runCold(opt Options, log *slog.Logger,
	discovery []DiscoveredFile, filter *filterlist.FilterList) (persist.Manifest, error) {
	files, err := detect.Walk(opt.SrcRoot)
	if err != nil {
		return persist.Manifest{}, fmt.Errorf("detect: %w", err)
	}
	goFiles, err := detect.GoFilesFor(opt.SrcRoot, opt.GoBuild)
	if err != nil {
		return persist.Manifest{}, fmt.Errorf("detect go: %w", err)
	}
	// --files-from filter: trim every per-language list before parsing.
	if filter != nil {
		preGo, preTS, preSol, preProto := len(goFiles), len(files.TS), len(files.Sol), len(files.Proto)
		goFiles = filter.FilterPaths(goFiles)
		files.TS = filter.FilterPaths(files.TS)
		files.Sol = filter.FilterPaths(files.Sol)
		files.Proto = filter.FilterPaths(files.Proto)
		log.Info("files-from applied",
			"go", preGo, "go_after", len(goFiles),
			"ts", preTS, "ts_after", len(files.TS),
			"sol", preSol, "sol_after", len(files.Sol),
			"proto", preProto, "proto_after", len(files.Proto))
	}

	// (2)+(3) parse + link, per language
	resolved := []*parse.ResolvedGraph{}
	parseErrs := 0
	if shouldRun("go", opt.Languages) && len(goFiles) > 0 {
		log.Debug("pass1.start", "language", "go", "files", len(goFiles))
		rg, n, err := runGoPipeline(opt.SrcRoot, goFiles, opt.GoBuild, opt.FileRoles, opt.LockPropagation, log)
		if err != nil {
			return persist.Manifest{}, fmt.Errorf("go pipeline: %w", err)
		}
		parseErrs += n
		logResolveSummary(log, "go", rg)
		if rg, err = goThroughContract(log, rg, opt.FileRoles); err != nil {
			return persist.Manifest{}, err
		}
		resolved = append(resolved, rg)
		log.Debug("pass1.end", "language", "go", "nodes", len(rg.Nodes), "errs", n)
	}
	// solParser is retained across the language passes so that the
	// cross-language linker can read Solidity ABI sigs after graph.Build.
	// nil signals "no Sol pipeline ran" — xlang stage is skipped in that case.
	var solParser *solp.Parser
	if shouldRun("ts", opt.Languages) && len(files.TS) > 0 {
		log.Debug("pass1.start", "language", "ts", "files", len(files.TS))
		rg, n, err := runTSPipeline(opt.SrcRoot, files.TS, log)
		if err != nil {
			return persist.Manifest{}, fmt.Errorf("ts pipeline: %w", err)
		}
		parseErrs += n
		resolved = append(resolved, rg)
		log.Debug("pass1.end", "language", "ts", "nodes", len(rg.Nodes), "errs", n)
	}
	if shouldRun("sol", opt.Languages) && len(files.Sol) > 0 {
		log.Debug("pass1.start", "language", "sol", "files", len(files.Sol))
		rg, n, p, err := runSolPipeline(opt.SrcRoot, files.Sol, log)
		if err != nil {
			return persist.Manifest{}, fmt.Errorf("sol pipeline: %w", err)
		}
		parseErrs += n
		solParser = p
		resolved = append(resolved, rg)
		log.Debug("pass1.end", "language", "sol", "nodes", len(rg.Nodes), "errs", n)
	}
	// Schema 1.9: .proto schema parser. Hand-rolled lexer + recursive-
	// descent — emits Service/Method/MessageType/Enum/Field/Package nodes plus
	// uses_type pending refs for rpc request/response types. gRPC client/server
	// detection lives in the Go/TS parsers.
	if shouldRun("proto", opt.Languages) && len(files.Proto) > 0 {
		log.Debug("pass1.start", "language", "proto", "files", len(files.Proto))
		rg, n, err := runProtoPipeline(opt.SrcRoot, files.Proto, log)
		if err != nil {
			return persist.Manifest{}, fmt.Errorf("proto pipeline: %w", err)
		}
		parseErrs += n
		resolved = append(resolved, rg)
		log.Debug("pass1.end", "language", "proto", "nodes", len(rg.Nodes), "errs", n)
	}

	// (4) graph build + validate
	log.Debug("pass2.resolve.start")
	warnDuplicateNodeIDs(log, resolved)
	g, err := graph.Build(resolved)
	if err != nil {
		return persist.Manifest{}, fmt.Errorf("graph.Build: %w", err)
	}
	if _, err := validateAndSanitize(g, log, "post-build", opt.StrictValidate); err != nil {
		return persist.Manifest{}, err
	}
	log.Debug("pass2.resolve.end", "nodes", len(g.Nodes), "edges", len(g.Edges))

	// (4b/4c/5/6) derived passes — xlang, temporal, cluster, score. Shared
	// helper so the partial-cache rebuild path runs identical in-memory
	// transformations. An earlier "emitted-vs-DB 0" bug was caused
	// by temporal living only in cold; the helper makes that recurrence
	// structurally impossible.
	log.Debug("metrics.start")
	pkgTree, topicTree, hunkBlobs, err := emitDerivedPasses(g, opt.SrcRoot, solParser, log, opt.StrictValidate, opt.TemporalDepth, filter)
	if err != nil {
		return persist.Manifest{}, err
	}
	log.Debug("metrics.end")

	// (7) persist — the build wipes graph.db so we don't accumulate stale
	// rows.
	log.Debug("persist.start", "nodes", len(g.Nodes), "edges", len(g.Edges))
	store, commitCold, err := openColdStore(opt.OutDir, opt.DBDSN)
	if err != nil {
		return persist.Manifest{}, err
	}
	committed := false
	defer func() {
		if !committed { // error path: close (and discard) the temp store
			_ = store.Close()
		}
	}()
	if err := persistColdArtifacts(store, opt.SrcRoot, g, pkgTree, topicTree, hunkBlobs); err != nil {
		return persist.Manifest{}, err
	}
	// PR breadcrumb (schema 1.12): scan git log for PR-tagged
	// commits and persist the node ↔ PR mapping. Runs after the node
	// rows land so the node_prs FK has its targets. Failure here is
	// logged but non-fatal — PR metadata is strictly additive and a
	// missing remote (non-git source tree, sparse clone, etc.) must
	// not break the cold build.
	prByNode, prErr := ScanPRHistory(opt.SrcRoot, g.Nodes)
	if prErr != nil {
		log.Warn("scan pr_history failed; node_prs left empty", "err", prErr)
	} else if len(prByNode) > 0 {
		if err := store.InsertNodePRs(prByNode); err != nil {
			return persist.Manifest{}, fmt.Errorf("persist node_prs: %w", err)
		}
		log.Info("pr_history emitted", "nodes_with_prs", len(prByNode))
	}
	// Policy metadata (schema 1.14): load the operator-supplied
	// YAML, resolve governs[] qnames against the parsed graph, persist
	// NodePolicy rows + EdgeGovernedBy edges. Runs after the main node
	// insert so the FK on EdgeGovernedBy.src targets resolved code IDs.
	// Failure here is logged but non-fatal — policy enrichment is
	// strictly additive; a malformed YAML must not break the cold
	// build. Empty PolicyFile → skipped silently.
	// enrichNodes/enrichEdges accumulate the operator-injected overlay so the
	// manifest can carry its EnrichDigest. The rows are persisted to the store
	// but deliberately NOT appended to g: keeping g code-only leaves Stats,
	// per-file entries, and graph_digest (the coordinate pin) untouched, while
	// EnrichDigest below hashes the overlay separately (digest-split contract,
	// see graph_digest.go).
	var enrichNodes []types.Node
	var enrichEdges []types.Edge
	var inputs []persist.OptionalInput
	rowPath := func(kind, given string) string {
		if given == "" {
			return ""
		}
		in, err := describeInput(kind, given, opt.SrcRoot)
		if err != nil {
			log.Warn("optional input not readable", "kind", kind, "err", err)
		}
		inputs = append(inputs, in)
		return in.RowPath
	}
	policyRows := rowPath(inputPolicyFile, opt.PolicyFile)
	securityRows := rowPath(inputSecurityPatternFile, opt.SecurityPatternFile)
	if policyNodes, policyEdges, perr := loadPolicy(opt.PolicyFile, g.Nodes, log); perr != nil {
		log.Warn("policy enrichment failed; policy nodes/edges left empty",
			"file", opt.PolicyFile, "err", perr)
	} else if len(policyNodes) > 0 {
		relocateRows(policyNodes, policyEdges, policyRows)
		if err := store.InsertNodes(policyNodes); err != nil {
			return persist.Manifest{}, fmt.Errorf("persist policy nodes: %w", err)
		}
		if len(policyEdges) > 0 {
			if err := store.InsertEdges(policyEdges); err != nil {
				return persist.Manifest{}, fmt.Errorf("persist policy edges: %w", err)
			}
		}
		enrichNodes = append(enrichNodes, policyNodes...)
		enrichEdges = append(enrichEdges, policyEdges...)
		log.Info("policy enrichment emitted",
			"policy_nodes", len(policyNodes), "governed_by_edges", len(policyEdges))
	}
	// Security pattern annotations (schema 1.15): same load-
	// resolve-persist shape as policy enrichment above. Runs after
	// policy so its own matches[] index reuses the (now larger)
	// g.Nodes set if a future YAML wanted to label a Policy node
	// itself as a security-sensitive area — overlap is permitted but
	// not relied on. Failure is logged-only for the same additive-
	// metadata reason.
	if secNodes, secEdges, serr := loadSecurityPatterns(opt.SecurityPatternFile, g.Nodes, log); serr != nil {
		log.Warn("security enrichment failed; security nodes/edges left empty",
			"file", opt.SecurityPatternFile, "err", serr)
	} else if len(secNodes) > 0 {
		relocateRows(secNodes, secEdges, securityRows)
		if err := store.InsertNodes(secNodes); err != nil {
			return persist.Manifest{}, fmt.Errorf("persist security nodes: %w", err)
		}
		if len(secEdges) > 0 {
			if err := store.InsertEdges(secEdges); err != nil {
				return persist.Manifest{}, fmt.Errorf("persist security edges: %w", err)
			}
		}
		enrichNodes = append(enrichNodes, secNodes...)
		enrichEdges = append(enrichEdges, secEdges...)
		log.Info("security enrichment emitted",
			"security_nodes", len(secNodes), "has_security_pattern_edges", len(secEdges))
	}
	log.Debug("persist.end")
	if err := rebuildReviewIndex(store, opt.SrcRoot, g.Nodes, opt.FileRoles, log); err != nil {
		return persist.Manifest{}, err
	}

	m := buildManifestSkeleton(opt, len(goFiles), len(files.TS), len(files.Sol), len(files.Proto),
		g, pkgTree, parseErrs)
	m.UnresolvedRefsCount = unresolvedTotal(resolved)
	m.GoLoadErrorsCount = goLoadErrors(resolved)
	// buildManifestSkeleton computes EnrichDigest from g, which is code-only, so
	// it is always "" here. Recompute it over the overlay actually injected into
	// the store so the manifest pins the enrichment (empty when none was applied).
	m.EnrichDigest = ComputeEnrichDigest(enrichNodes, enrichEdges)
	m.OptionalInputs = inputs
	// Files: every discovered file becomes an entry. This is the cache
	// fingerprint that subsequent builds will diff against. We computed
	// SHAs / cache_keys lazily here — once per cold build, so the cost
	// is amortised against the parse pass.
	m.Files = computeColdFileEntries(opt.SrcRoot, opt.CKGVersion, discovery, g.Nodes, g.Edges)
	setStaleness(&m, log)
	if opt.LogicalSrcRoot != "" {
		m.SrcRoot = opt.LogicalSrcRoot
	}
	if err := store.SetManifest(m); err != nil {
		return persist.Manifest{}, err
	}
	if err := writeManifestJSON(filepath.Join(opt.OutDir, "manifest.json"), m); err != nil {
		return persist.Manifest{}, err
	}
	// Commit the cold store: close + atomically rename graph.db.building over
	// graph.db (SQLite), so a concurrent reader never sees a partial DB (Q2).
	if err := commitCold(); err != nil {
		return persist.Manifest{}, err
	}
	committed = true
	log.Info("build complete",
		"nodes", len(g.Nodes), "edges", len(g.Edges),
		"pkg_tree_edges", len(pkgTree.Edges),
		"topic_resolutions", len(topicTree.Resolutions))
	return m, nil
}

// openColdStore prepares a store for a cold rebuild and returns a commit func
// that must be called on success. For SQLite (Q2, ADR reindex-migration
// 2026-07-10) the build writes to a temporary "graph.db.building" file; commit
// closes the store — which checkpoints and removes the WAL/SHM sidecars on the
// last connection — then atomically renames it over the live graph.db. This
// removes the multi-second window in which the old destructive `os.Remove(dbPath)
// → rewrite` left graph.db absent or half-written for a concurrent reader
// (e.g. `ckg serve`). The fully-atomic cross-file boundary is still the
// versioned-dir + `current` symlink swap owned by the build orchestration; this
// makes even a naive same-dir rebuild safe.
//
// When dbDsn is set the store is a PostgreSQL database wiped in place via
// TRUNCATE; commit is a plain Close (pg atomicity is out of scope here).
func openColdStore(outDir, dbDsn string) (persist.Store, func() error, error) {
	if dbDsn != "" {
		store, err := persist.OpenPostgresCold(dbDsn)
		if err != nil {
			return nil, nil, err
		}
		return store, store.Close, nil
	}
	dbPath := filepath.Join(outDir, "graph.db")
	tmpPath := dbPath + ".building"
	removeSQLiteFiles(tmpPath) // clear artifacts from an aborted prior build
	store, err := persist.Open(tmpPath)
	if err != nil {
		return nil, nil, err
	}
	if err := store.Migrate(); err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	commit := func() error {
		// Close first so WAL is checkpointed into the main file and -wal/-shm
		// are dropped — the temp file becomes a complete standalone DB.
		if err := store.Close(); err != nil {
			return fmt.Errorf("close cold store before rename: %w", err)
		}
		// Rename replaces the live file atomically (same directory / filesystem).
		if err := os.Rename(tmpPath, dbPath); err != nil {
			return fmt.Errorf("rename %s -> graph.db: %w", filepath.Base(tmpPath), err)
		}
		// Drop any stale sidecars left by a prior WAL-mode graph.db so a fresh
		// reader of the renamed file never pairs it with a mismatched -wal.
		_ = os.Remove(dbPath + "-wal")
		_ = os.Remove(dbPath + "-shm")
		return nil
	}
	return store, commit, nil
}

// removeSQLiteFiles deletes a SQLite DB file together with its WAL/SHM sidecars.
func removeSQLiteFiles(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
}

// TopicTreeForPersist re-exposes persist.TopicTreeInput under a buildpipe-
// local alias, so persistColdArtifacts can take it as a typed parameter
// without leaking the persist package detail to every caller.
type TopicTreeForPersist = persist.TopicTreeInput

// persistColdArtifacts performs the bulk-insert phase of a cold rebuild.
// All inserts are unconditional — the DB was just wiped by openColdStore.
//
// hunkBlobs (schema 1.8) is the gzip-compressed unified-diff text per
// Hunk node ID, returned by emitTemporalEdges. Merged into the regular
// extractBlobs map so a single InsertBlobs round-trip persists both
// CodeNode source slices and hunk patches under the same blobs.node_id PK.
func persistColdArtifacts(store persist.Store, srcRoot string,
	g *graph.Graph, pkgTree *cluster.PkgTree, topicTree TopicTreeForPersist,
	hunkBlobs map[string][]byte) error {
	if err := store.InsertNodes(g.Nodes); err != nil {
		return err
	}
	if err := store.InsertEdges(g.Edges); err != nil {
		return err
	}
	if err := store.InsertPkgTreeFromCluster(pkgTree.PersistEdges()); err != nil {
		return err
	}
	if err := store.InsertTopicTree(topicTree); err != nil {
		return err
	}
	blobs := extractBlobs(srcRoot, g.Nodes)
	maps.Copy(blobs, hunkBlobs)
	if err := store.InsertBlobs(blobs); err != nil {
		return err
	}
	return store.RebuildFTS()
}

// computeColdFileEntries hashes every discovered file and returns FileEntry
// records for the new manifest. Called on cold rebuild so the next build can
// diff against this baseline. EdgeIDs are int64 PRIMARY KEY values assigned
// by the AUTOINCREMENT INSERT just performed.
//
// Meta nodes (Commit, Hunk) are excluded from per-file NodeIDs (schema 1.8
// §11.8 decision): they're emitted wholesale by emitTemporalEdges on every
// build, so attributing them to a file would inflate the manifest and
// trigger spurious cache invalidations whenever that file's content changes.
func computeColdFileEntries(srcRoot, ckgVersion string, discovery []DiscoveredFile, nodes []types.Node, edges []types.Edge) []persist.FileEntry {
	nodesByPath := map[string][]string{}
	for _, n := range nodes {
		if n.FilePath == "" {
			continue
		}
		if isMetaNodeType(n.Type) {
			continue
		}
		nodesByPath[n.FilePath] = append(nodesByPath[n.FilePath], n.ID)
	}
	edgesByPath := map[string][]int64{}
	for _, e := range edges {
		if e.FilePath == "" {
			continue
		}
		edgesByPath[e.FilePath] = append(edgesByPath[e.FilePath], e.ID)
	}
	out := make([]persist.FileEntry, 0, len(discovery))
	for _, df := range discovery {
		full := filepath.Join(srcRoot, filepath.FromSlash(df.Path))
		content, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		st, _ := os.Stat(full)
		var mtime int64
		if st != nil {
			mtime = st.ModTime().UnixNano()
		}
		parserVer := parserVersionFor(df.Language)
		out = append(out, persist.FileEntry{
			Path:          df.Path,
			Language:      df.Language,
			SHA256:        SHA256Hex(content),
			CacheKey:      ComputeCacheKey(content, ckgVersion, parserVer),
			MTime:         mtime,
			ParserVersion: parserVer,
			NodeIDs:       nodesByPath[df.Path],
			EdgeIDs:       edgesByPath[df.Path],
		})
	}
	return out
}

// shouldRun returns true when lang is requested explicitly or via the "auto"
// catch-all in opts.
func shouldRun(lang string, opts []string) bool {
	for _, l := range opts {
		if l == "auto" || l == lang {
			return true
		}
	}
	return false
}

// extractBlobs reads every node's source slice (StartByte..EndByte) into a
// per-node blob, caching file contents to amortize IO. Package nodes are
// skipped (they have no syntactic body); meta nodes (Commit, Hunk) are
// also skipped (they have no on-disk byte range — Hunk patch bytes come
// from emitTemporalEdges' hunkBlobs map merged into the result by the
// caller, NOT from a file slice). Offsets are bounds-checked defensively
// to avoid panics on malformed nodes.
func extractBlobs(root string, nodes []types.Node) map[string][]byte {
	blobs := map[string][]byte{}
	cache := map[string][]byte{}
	for _, n := range nodes {
		if n.Type == types.NodePackage || n.Type == types.NodeExternal {
			continue
		}
		if isMetaNodeType(n.Type) {
			continue
		}
		full := filepath.Join(root, n.FilePath)
		src, ok := cache[full]
		if !ok {
			b, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			cache[full] = b
			src = b
		}
		if n.StartByte < 0 || n.EndByte > len(src) || n.StartByte >= n.EndByte {
			continue
		}
		blobs[n.ID] = append([]byte(nil), src[n.StartByte:n.EndByte]...)
	}
	return blobs
}

// writeManifestJSON pretty-prints the manifest to path for human inspection.
func writeManifestJSON(path string, m persist.Manifest) error {
	m = m.WithGraphBuilderIdentity()
	buf, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(path, buf, 0o644)
}
