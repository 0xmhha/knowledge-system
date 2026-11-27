package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/internal/graph/filterlist"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

func newBuildCmd() *cobra.Command {
	var src, out, outTag, atCommit, dbDsn, filesFrom, policyFile, securityPatternFile, scopePath string
	var projectID, snapshotID, datasetID, fileManifestDigest, capturePolicyDigest, sourceMode, logicalSrcRoot string
	var langs, filesFromMain, solInclude, excludes []string
	var noCache, force, rebuildMetrics, strictValidate, lockPropagation, failOnParseErrors bool
	var temporalDepth int
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Parse a source tree and produce graph.db",
		RunE: func(cmd *cobra.Command, args []string) error {
			effectiveSrc := src
			var worktreeCleanup func()
			if atCommit != "" {
				wt, clean, err := checkoutWorktree(src, atCommit)
				if err != nil {
					return err
				}
				effectiveSrc = wt
				worktreeCleanup = clean
				defer worktreeCleanup()
			}

			effectiveOut, err := resolveOutDir(out, outTag, effectiveSrc)
			if err != nil {
				return err
			}

			if !force && !noCache && graphExists(effectiveOut) {
				_, _ = fmt.Fprintf(os.Stderr, "ckg: graph already exists at %s (use --force to rebuild)\n", effectiveOut)
				return nil
			}

			logFile, err := buildLogFile(rootLogFile, effectiveOut)
			if err != nil {
				return fmt.Errorf("log file: %w", err)
			}
			log, cleanup, err := newLogger(rootVerbose, logFile)
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}
			defer cleanup()

			// --files-from-main generates the include/exclude filter in-memory
			// from a Go main-package closure (go list -deps), replacing the
			// former graph/scripts/index-project.sh shell logic. It is mutually
			// exclusive with --files-from (an explicit filter file).
			var generatedFilter *filterlist.FilterList
			var goBuild *detect.GoBuild
			// fileRoles carries each scoped file's role to the build, so
			// is_test follows the scope and not the file's name alone.
			var fileRoles buildpipe.FileRoles
			if scopePath != "" {
				if filesFrom != "" || len(filesFromMain) > 0 {
					return fmt.Errorf("--scope cannot be combined with --files-from or --files-from-main")
				}
				s, b, fl, err := loadScope(scopePath)
				if err != nil {
					return err
				}
				generatedFilter, goBuild = fl, b
				fileRoles = make(buildpipe.FileRoles, len(s.Files))
				for _, f := range s.Files {
					fileRoles[f.Path] = buildpipe.FileRole(f.Role)
				}
				_, _ = fmt.Fprintf(os.Stderr, "ckg: scope %s: %d files for %s/%s cgo=%s\n",
					s.ConfigID, len(s.Files), b.GOOS, b.GOARCH, b.CGOEnabled)
			}
			if len(filesFromMain) > 0 {
				if filesFrom != "" {
					return fmt.Errorf("--files-from and --files-from-main are mutually exclusive")
				}
				fl, err := filterlist.GenerateFromMain(cmd.Context(), effectiveSrc, filesFromMain, solInclude, excludes)
				if err != nil {
					return err
				}
				generatedFilter = fl
				_, _ = fmt.Fprintf(os.Stderr, "ckg: files-from-main: %d include / %d exclude patterns from %v\n",
					len(fl.Include), len(fl.Exclude), filesFromMain)
			}

			m, err := buildpipe.Run(buildpipe.Options{
				ProjectID: projectID, SnapshotID: snapshotID, DatasetID: datasetID,
				FileManifestDigest: fileManifestDigest, CapturePolicyDigest: capturePolicyDigest, SourceMode: sourceMode,
				SrcRoot:             effectiveSrc,
				LogicalSrcRoot:      logicalSrcRoot,
				OutDir:              effectiveOut,
				Languages:           langs,
				Logger:              log,
				CKGVersion:          ckgVersion,
				NoCache:             noCache,
				DBDSN:               dbDsn,
				StrictValidate:      strictValidate,
				FilesFromPath:       filesFrom,
				Filter:              generatedFilter,
				GoBuild:             goBuild,
				FileRoles:           fileRoles,
				LockPropagation:     lockPropagation,
				PolicyFile:          policyFile,
				SecurityPatternFile: securityPatternFile,
				TemporalDepth:       temporalDepth,
			})
			if err != nil {
				return err
			}
			// Surface dropped files: parse failures are skipped during the
			// build, so a plain success line would misreport a partial graph
			// as complete. --fail-on-parse-errors turns that into a hard
			// failure for CI / reproducible builds.
			nodes, edges := builtCounts(m, effectiveOut, dbDsn)
			if m.ParseErrorsCount > 0 {
				_, _ = fmt.Fprintf(os.Stderr, "ckg: built %d nodes / %d edges into %s (%d files failed to parse and were skipped)\n",
					nodes, edges, effectiveOut, m.ParseErrorsCount)
				if failOnParseErrors {
					return fmt.Errorf("ckg: %d files failed to parse (--fail-on-parse-errors)", m.ParseErrorsCount)
				}
				return nil
			}
			_, _ = fmt.Fprintf(os.Stderr, "ckg: built %d nodes / %d edges into %s\n",
				nodes, edges, effectiveOut)
			return nil
		},
	}
	cmd.Flags().StringVar(&src, "src", "", "source root (required)")
	cmd.Flags().StringVar(&projectID, "project-id", "", "stable project ID for a pinned dataset")
	cmd.Flags().StringVar(&snapshotID, "snapshot-id", "", "captured source snapshot ID")
	cmd.Flags().StringVar(&datasetID, "dataset-id", "", "pre-build dataset ID")
	cmd.Flags().StringVar(&fileManifestDigest, "file-manifest-digest", "", "source file manifest digest")
	cmd.Flags().StringVar(&capturePolicyDigest, "capture-policy-digest", "", "versioned source capture policy digest")
	cmd.Flags().StringVar(&sourceMode, "source-mode", "", "committed source mode for pinned builds")
	cmd.Flags().StringVar(&logicalSrcRoot, "logical-src-root", "", "public project root when --src is a retained build tree")
	cmd.Flags().StringVar(&out, "out", "", "output directory (required)")
	cmd.Flags().StringVar(&outTag, "out-tag", "",
		`suffix appended to --out directory; "auto-commit-hash" appends the source tree's path-aware HEAD commit (short SHA)`)
	cmd.Flags().StringVar(&atCommit, "at-commit", "",
		"build from a specific git commit using git worktree (leaves --src working tree untouched)")
	cmd.Flags().BoolVar(&force, "force", false,
		"build over an existing graph.db at the output directory; the build is cold like every build and reuses nothing from it")
	cmd.Flags().StringSliceVar(&langs, "lang", []string{"auto"}, "languages: auto|go,ts,sol")
	cmd.Flags().BoolVar(&noCache, "no-cache", false,
		"no effect on the build: every build is cold (parses every file); like --force it builds over an existing graph.db. Kept so existing scripts still parse")
	cmd.Flags().BoolVar(&rebuildMetrics, "rebuild-metrics", false, "no effect; kept so existing scripts still parse")
	_ = cmd.Flags().MarkDeprecated("rebuild-metrics",
		"it has no effect: every build recomputes scores and communities")
	cmd.Flags().StringVar(&dbDsn, "db", "",
		"PostgreSQL DSN (e.g. postgres://user:pass@host/dbname); if set, store graph in PG instead of local SQLite. DEPRECATED (ADR-0003): SQLite is the sole maintained backend")
	cmd.Flags().BoolVar(&strictValidate, "strict-validate", false,
		"abort on first dangling edge (legacy v0.x behaviour); default lenient drops them with a warning")
	cmd.Flags().StringVar(&filesFrom, "files-from", "",
		"path to JSON file with {include, exclude} glob patterns; restricts which files reach the parsers")
	cmd.Flags().StringVar(&scopePath, "scope", "",
		"build scope JSON from `ckg scope`: read only the files it lists, Go type-checked with its GOOS, GOARCH, cgo and tags; without a scope every file of the tree is read")
	cmd.Flags().StringSliceVar(&filesFromMain, "files-from-main", nil,
		"Go main package(s) (e.g. ./cmd/gstable; repeatable or comma-separated); generates the include/exclude filter in-memory from `go list -deps` of the module-local closure. Mutually exclusive with --files-from")
	cmd.Flags().StringSliceVar(&solInclude, "sol-include", nil,
		"Solidity include glob(s) appended to a --files-from-main filter (repeatable or comma-separated), e.g. contracts/**/*.sol")
	cmd.Flags().StringSliceVar(&excludes, "exclude", nil,
		"exclude glob(s) applied on top of a --files-from-main filter (repeatable or comma-separated); exclude trumps include")
	cmd.Flags().BoolVar(&lockPropagation, "lock-propagation", false,
		"also record fields used in a function called while a mutex is held, where the mutex is still held at the use (up to 5 calls deep; accessed_under_lock edges with dispatch_kind \"propagated\")")
	cmd.Flags().StringVar(&policyFile, "policy-file", "",
		"path to governance/protocol policy YAML (pkg/policy); enriches the graph with NodePolicy + EdgeGovernedBy rows")
	cmd.Flags().IntVar(&temporalDepth, "temporal-depth", 0,
		"max commits per file the temporal pass walks for changed_in/Hunk/blame edges (0 = default 10; higher deepens commit history at ~linear graph-size cost; does not affect node_prs symbol history)")
	cmd.Flags().StringVar(&securityPatternFile, "security-pattern-file", "",
		"path to security risk pattern YAML (pkg/security); enriches the graph with NodeSecurityPattern + EdgeHasSecurityPattern rows")
	cmd.Flags().BoolVar(&failOnParseErrors, "fail-on-parse-errors", false,
		"exit non-zero if any source file fails to parse (default: skip the file and report the count); use in CI to reject partial graphs")
	_ = cmd.MarkFlagRequired("src")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

// graphExists returns true if the output directory already has a graph.db.
func graphExists(outDir string) bool {
	info, err := os.Stat(filepath.Join(outDir, "graph.db"))
	return err == nil && !info.IsDir()
}

// checkoutWorktree creates a temporary git worktree at the given commit
// and returns the worktree path + cleanup function.
func checkoutWorktree(repoSrc, commit string) (string, func(), error) {
	absRepo, err := filepath.Abs(repoSrc)
	if err != nil {
		return "", nil, err
	}
	toplevel, err := exec.Command("git", "-C", absRepo, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", nil, fmt.Errorf("--at-commit: %s is not a git repository", repoSrc)
	}
	repoRoot := strings.TrimSpace(string(toplevel))

	fullSHA, err := exec.Command("git", "-C", repoRoot, "rev-parse", commit).Output()
	if err != nil {
		return "", nil, fmt.Errorf("--at-commit: cannot resolve %q: %w", commit, err)
	}
	sha := strings.TrimSpace(string(fullSHA))
	short := sha
	if len(short) > 12 {
		short = short[:12]
	}

	wtDir, err := os.MkdirTemp("", "ckg-worktree-"+short+"-*")
	if err != nil {
		return "", nil, err
	}
	if out, err := exec.Command("git", "-C", repoRoot, "worktree", "add", "--detach", wtDir, sha).CombinedOutput(); err != nil {
		_ = os.RemoveAll(wtDir)
		return "", nil, fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	cleanup := func() {
		_ = exec.Command("git", "-C", repoRoot, "worktree", "remove", "--force", wtDir).Run()
		_ = os.RemoveAll(wtDir)
	}
	return wtDir, cleanup, nil
}

// builtCounts returns the node and edge counts a build reports: the rows
// of the graph.db it wrote, which hold rows the build writes after its
// manifest counts them (Policy nodes and governed_by edges). A build into
// PostgreSQL (dsn set), or a graph.db that cannot be read, reports the
// manifest's counts.
func builtCounts(m persist.Manifest, outDir, dsn string) (nodes, edges int) {
	nodes, edges = m.Stats["nodes"], m.Stats["edges"]
	if dsn != "" {
		return nodes, edges
	}
	store, err := persist.OpenReadOnly(filepath.Join(outDir, "graph.db"))
	if err != nil {
		return nodes, edges
	}
	defer func() { _ = store.Close() }()
	if n, e, err := store.RowCounts(); err == nil {
		return n, e
	}
	return nodes, edges
}

// resolveOutDir applies --out-tag to --out.
func resolveOutDir(out, tag, srcRoot string) (string, error) {
	if tag == "" {
		return out, nil
	}
	if tag == "auto-commit-hash" {
		sha, err := srcCommitHash(srcRoot)
		if err != nil {
			return "", fmt.Errorf("--out-tag=auto-commit-hash: %w", err)
		}
		if sha == "" {
			return "", fmt.Errorf("--out-tag=auto-commit-hash: no git history for %s", srcRoot)
		}
		short := sha
		if len(short) > 12 {
			short = short[:12]
		}
		return out + "-" + short, nil
	}
	return out + "-" + tag, nil
}

func srcCommitHash(srcRoot string) (string, error) {
	absRoot, err := filepath.Abs(srcRoot)
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", absRoot, "log", "-1", "--format=%H").Output()
	if err != nil {
		return "", fmt.Errorf("git log in %s: %w", absRoot, err)
	}
	return strings.TrimSpace(string(out)), nil
}
