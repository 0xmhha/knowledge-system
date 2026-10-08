package semanticcli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

func newBuildCmd() *cobra.Command {
	var repo, project, dataset, graph, vector, storePath, output, ontology, spec, versionDir string
	var docs []string
	var activate, extractOnly, includePacks bool
	var minimumCoverage float64
	cmd := &cobra.Command{
		Use: "build", Short: "Extract archived or committed Markdown sections into an aligned semantic dataset",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuild(cmd.Context(), cmd, buildOptions{
				repo: repo, project: project, dataset: dataset, graph: graph, vector: vector,
				store: storePath, output: output, docs: docs, ontology: ontology, spec: spec, versionDir: versionDir,
				activate: activate, extractOnly: extractOnly, includePacks: includePacks, minimumCoverage: minimumCoverage,
			})
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "logical source root (Git repository for legacy committed builds)")
	cmd.Flags().StringVar(&project, "project-id", "", "stable project identifier")
	cmd.Flags().StringVar(&dataset, "dataset-id", "", "immutable semantic dataset identifier")
	cmd.Flags().StringVar(&graph, "graph", "", "CKG data directory")
	cmd.Flags().StringVar(&vector, "vector", "", "CKV data directory")
	cmd.Flags().StringVar(&storePath, "store", "", "CKS semantic SQLite path")
	cmd.Flags().StringVar(&output, "out", "", "reviewable projection JSON output")
	cmd.Flags().StringSliceVar(&docs, "docs", nil, "repository-relative committed Markdown paths (repeatable)")
	cmd.Flags().StringVar(&ontology, "ontology", "", "repository-relative committed ontology YAML path")
	cmd.Flags().StringVar(&spec, "spec", "", "repository-relative committed specification YAML path")
	cmd.Flags().StringVar(&versionDir, "version-dir", "", "pinned dataset directory with retained source archive (required for working-tree and snapshot-only)")
	cmd.Flags().BoolVar(&activate, "activate", false, "activate this dataset after validation")
	cmd.Flags().BoolVar(&extractOnly, "extract-only", false, "write a reviewable projection without storing or activating it")
	cmd.Flags().BoolVar(&includePacks, "include-packs", false, "derive a v4 tuple-scoped pack projection from the retained knowledge lock")
	cmd.Flags().Float64Var(&minimumCoverage, "min-canonical-ratio", 0, "measured project minimum for CKV-to-CKG symbol alignment (0 disables)")
	for _, flag := range []string{"repo", "project-id", "dataset-id", "graph", "vector", "store", "out"} {
		_ = cmd.MarkFlagRequired(flag)
	}
	return cmd
}

type buildOptions struct {
	repo, project, dataset, graph, vector, store, output, ontology, spec, versionDir string
	docs                                                                             []string
	activate                                                                         bool
	extractOnly                                                                      bool
	includePacks                                                                     bool
	minimumCoverage                                                                  float64
}

func runBuild(ctx context.Context, cmd *cobra.Command, o buildOptions) error {
	if o.extractOnly && o.activate {
		return fmt.Errorf("--extract-only cannot be combined with --activate")
	}
	if len(o.docs) == 0 && o.ontology == "" && o.spec == "" && !o.includePacks {
		return fmt.Errorf("at least one --docs, --ontology, --spec, or --include-packs is required")
	}
	if o.includePacks && o.versionDir == "" {
		return fmt.Errorf("--include-packs requires --version-dir with a retained knowledge lock")
	}
	if err := semantic.ValidateCanonicalCoverage(o.vector, o.minimumCoverage); err != nil {
		return err
	}
	repo, err := filepath.Abs(o.repo)
	if err != nil {
		return err
	}
	snapshot := semantic.Snapshot{ProjectID: o.project, DatasetID: o.dataset}
	readSource := func(file string) ([]byte, error) {
		return exec.CommandContext(ctx, "git", "-C", repo, "show", snapshot.Commit+":"+file).Output()
	}
	if o.versionDir != "" {
		identity, inspectErr := setup.InspectVersionIdentity(o.versionDir)
		if inspectErr != nil {
			return inspectErr
		}
		if identity == nil || identity.Source.ProjectID != o.project || identity.DatasetID != o.dataset {
			return fmt.Errorf("semantic version directory has no matching pinned identity")
		}
		snapshot.Commit = identity.Source.SourceCommit
		snapshot.SnapshotID = identity.Source.SnapshotID
		snapshot.SourceMode = identity.Source.SourceMode
		readSource = func(file string) ([]byte, error) {
			buf, _, _, err := setup.ReadRetainedFile(o.versionDir, "repo", file)
			return buf, err
		}
	} else {
		commitBytes, commitErr := exec.CommandContext(ctx, "git", "-C", repo, "rev-parse", "HEAD").Output()
		if commitErr != nil {
			return fmt.Errorf("read source HEAD: %w", commitErr)
		}
		snapshot.Commit = strings.TrimSpace(string(commitBytes))
		snapshot.SnapshotID, err = semantic.PinnedSnapshotID(o.graph, o.project, o.dataset)
		if err != nil {
			return err
		}
	}
	p := semantic.Projection{SchemaVersion: semantic.SchemaVersion, Snapshot: snapshot}
	seen := map[string]bool{}
	for _, file := range o.docs {
		if !safeInputPath(file) {
			return fmt.Errorf("unsafe document path %q", file)
		}
		if seen[file] {
			return fmt.Errorf("duplicate document path %q", file)
		}
		seen[file] = true
		content, err := readSource(file)
		if err != nil {
			return fmt.Errorf("read source document %q: %w", file, err)
		}
		part, err := semantic.ExtractMarkdown(snapshot, file, content)
		if err != nil {
			return err
		}
		p.Evidence = append(p.Evidence, part.Evidence...)
		p.Sections = append(p.Sections, part.Sections...)
	}
	if o.ontology != "" {
		if !safeInputPath(o.ontology) || seen[o.ontology] {
			return fmt.Errorf("unsafe or duplicate ontology path %q", o.ontology)
		}
		content, err := readSource(o.ontology)
		if err != nil {
			return fmt.Errorf("read committed ontology %q: %w", o.ontology, err)
		}
		part, _, err := semantic.ExtractOntology(snapshot, o.ontology, content)
		if err != nil {
			return err
		}
		p.Evidence = append(p.Evidence, part.Evidence...)
		p.Concepts = append(p.Concepts, part.Concepts...)
		seen[o.ontology] = true
	}
	if o.spec != "" {
		if !safeInputPath(o.spec) || seen[o.spec] {
			return fmt.Errorf("unsafe or duplicate specification path %q", o.spec)
		}
		content, err := readSource(o.spec)
		if err != nil {
			return fmt.Errorf("read committed specification %q: %w", o.spec, err)
		}
		part, _, err := semantic.ExtractSpec(snapshot, o.spec, content)
		if err != nil {
			return err
		}
		p.Evidence = append(p.Evidence, part.Evidence...)
		p.Requirements = append(p.Requirements, part.Requirements...)
	}
	if o.includePacks {
		p.Knowledge, err = knowledgepack.ProjectRetainedKnowledge(o.versionDir)
		if err != nil {
			return err
		}
		p.SchemaVersion = semantic.PackProjectionVersion
	}
	linkedSections, err := semantic.AttachCKVChunks(ctx, &p, o.vector)
	if err != nil {
		return err
	}
	var store *semantic.Store
	if o.extractOnly {
		if err := semantic.ValidateDatasetAlignment(p, repo, o.graph, o.vector); err != nil {
			return err
		}
		if o.versionDir != "" {
			if err := p.ValidateRetainedSources(o.versionDir); err != nil {
				return err
			}
		} else if err := p.ValidateSources(ctx, repo); err != nil {
			return err
		}
	} else {
		store, err = semantic.OpenStore(o.store)
		if err != nil {
			return err
		}
		defer store.Close()
		if o.versionDir != "" {
			err = store.PutAlignedRetained(ctx, p, repo, o.graph, o.vector, o.versionDir)
		} else {
			err = store.PutAligned(ctx, p, repo, o.graph, o.vector)
		}
		if err != nil {
			return err
		}
	}
	buf, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	buf = append(buf, '\n')
	if err := writeProjection(o.output, buf); err != nil {
		return err
	}
	if o.activate {
		if o.versionDir != "" {
			err = store.ActivateAlignedRetained(ctx, o.project, o.dataset, repo, o.graph, o.vector, o.versionDir)
		} else {
			err = store.ActivateAligned(ctx, o.project, o.dataset, repo, o.graph, o.vector)
		}
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
		"project_id": o.project, "dataset_id": o.dataset, "commit": snapshot.Commit,
		"sections": len(p.Sections), "chunk_linked_sections": linkedSections,
		"concepts": len(p.Concepts), "requirements": len(p.Requirements), "schema_version": p.SchemaVersion,
		"evidence": len(p.Evidence), "stored": !o.extractOnly, "activated": o.activate,
	})
}

func safeInputPath(file string) bool {
	return file != "" && !strings.Contains(file, "\\") && path.Clean(file) == file &&
		file != "." && file != ".." && !strings.HasPrefix(file, "../") && !strings.HasPrefix(file, "/")
}

func writeProjection(output string, content []byte) error {
	dir := filepath.Dir(output)
	tmp, err := os.CreateTemp(dir, ".semantic-projection-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), output)
}
