package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type graphCoordinates struct {
	ProjectID          string `json:"project_id"`
	SnapshotID         string `json:"snapshot_id"`
	DatasetID          string `json:"dataset_id"`
	FileManifestDigest string `json:"file_manifest_digest"`
	SchemaVersion      string `json:"schema_version"`
	SrcRoot            string `json:"src_root"`
	SrcCommit          string `json:"src_commit"`
	SourceMode         string `json:"source_mode"`
	GraphDigest        string `json:"graph_digest"`
}

type vectorCoordinates struct {
	ProjectID          string `json:"project_id"`
	SnapshotID         string `json:"snapshot_id"`
	DatasetID          string `json:"dataset_id"`
	FileManifestDigest string `json:"file_manifest_digest"`
	SrcRoot            string `json:"src_root"`
	SrcCommit          string `json:"src_commit"`
	SourceMode         string `json:"source_mode"`
	SymbolCount        int    `json:"symbol_count"`
	CanonicalCount     int    `json:"canonical_count"`
	Sources            struct {
		CKG struct {
			GraphDigest string `json:"graph_digest"`
			SrcCommit   string `json:"src_commit"`
		} `json:"ckg"`
	} `json:"sources"`
}

// ValidateCanonicalCoverage applies a project-specific promotion floor to
// CKV's code-symbol alignment. Zero disables this optional gate.
func ValidateCanonicalCoverage(vectorDir string, minimum float64) error {
	if minimum < 0 || minimum > 1 {
		return fmt.Errorf("canonical coverage minimum must be between 0 and 1")
	}
	if minimum == 0 {
		return nil
	}
	var vector vectorCoordinates
	if err := readCoordinates(filepath.Join(vectorDir, "manifest.json"), &vector); err != nil {
		return fmt.Errorf("semantic vector manifest: %w", err)
	}
	if vector.SymbolCount <= 0 || vector.CanonicalCount < 0 || vector.CanonicalCount > vector.SymbolCount {
		return fmt.Errorf("semantic canonical coverage counters missing or invalid")
	}
	ratio := float64(vector.CanonicalCount) / float64(vector.SymbolCount)
	if ratio < minimum {
		return fmt.Errorf("semantic canonical coverage %.4f below project minimum %.4f", ratio, minimum)
	}
	return nil
}

// ValidateDatasetAlignment requires all three layers to share an exact source
// commit, graph coordinate pin, and source tree. Unlike legacy setup alignment
// (which warns on old indexes), semantic promotion refuses missing coordinates.
func ValidateDatasetAlignment(p Projection, repoRoot, graphDir, vectorDir string) error {
	if err := p.Validate(); err != nil {
		return err
	}
	var graph graphCoordinates
	if err := readCoordinates(filepath.Join(graphDir, "manifest.json"), &graph); err != nil {
		return fmt.Errorf("semantic graph manifest: %w", err)
	}
	var vector vectorCoordinates
	if err := readCoordinates(filepath.Join(vectorDir, "manifest.json"), &vector); err != nil {
		return fmt.Errorf("semantic vector manifest: %w", err)
	}
	graphPinned := graph.ProjectID != "" || graph.SnapshotID != "" || graph.DatasetID != ""
	vectorPinned := vector.ProjectID != "" || vector.SnapshotID != "" || vector.DatasetID != ""
	if graphPinned || vectorPinned {
		if graph.ProjectID == "" || graph.SnapshotID == "" || graph.DatasetID == "" || graph.FileManifestDigest == "" ||
			vector.ProjectID != graph.ProjectID || vector.SnapshotID != graph.SnapshotID || vector.DatasetID != graph.DatasetID ||
			vector.FileManifestDigest != graph.FileManifestDigest ||
			p.Snapshot.ProjectID != graph.ProjectID || p.Snapshot.DatasetID != graph.DatasetID ||
			p.Snapshot.SnapshotID != graph.SnapshotID {
			return fmt.Errorf("semantic project/snapshot/dataset identity differs across graph, vector and projection")
		}
	}
	parts := strings.SplitN(graph.SchemaVersion, ".", 3)
	if len(parts) < 2 {
		return fmt.Errorf("semantic graph schema version missing or invalid")
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	if majorErr != nil || minorErr != nil || major < 1 || (major == 1 && minor < 19) {
		return fmt.Errorf("semantic graph schema %q lacks canonical_id support", graph.SchemaVersion)
	}
	mode := p.Snapshot.SourceMode
	if mode == "" {
		mode = "committed"
	}
	if graphPinned && (graph.SourceMode != vector.SourceMode ||
		(graph.SourceMode != "" && graph.SourceMode != mode) ||
		(graph.SourceMode == "" && mode != "committed")) {
		return fmt.Errorf("semantic source mode differs across graph, vector and projection")
	}
	if (mode != "snapshot-only" && (graph.SrcCommit == "" || vector.SrcCommit == "" || vector.Sources.CKG.SrcCommit == "")) ||
		(mode == "snapshot-only" && p.Snapshot.Commit != "") ||
		graph.SrcCommit != p.Snapshot.Commit || vector.SrcCommit != p.Snapshot.Commit ||
		vector.Sources.CKG.SrcCommit != p.Snapshot.Commit {
		return fmt.Errorf("semantic source commit is missing or differs across graph, vector and projection")
	}
	if !digestPattern.MatchString(graph.GraphDigest) || graph.GraphDigest != vector.Sources.CKG.GraphDigest {
		return fmt.Errorf("semantic graph digest pin is missing or mismatched")
	}
	if graph.SrcRoot == "" || vector.SrcRoot == "" {
		return fmt.Errorf("semantic source root missing from graph or vector manifest")
	}
	repo, err := normalizedRoot(repoRoot)
	if err != nil {
		return err
	}
	graphRoot, err := normalizedRoot(graph.SrcRoot)
	if err != nil {
		return err
	}
	vectorRoot, err := normalizedRoot(vector.SrcRoot)
	if err != nil {
		return err
	}
	if repo != graphRoot || repo != vectorRoot {
		return fmt.Errorf("semantic source root differs across graph, vector and projection repository")
	}
	return nil
}

// PinnedSnapshotID returns the graph candidate's immutable snapshot ID when
// the new contract is present. Legacy manifests return an empty string.
func PinnedSnapshotID(graphDir, projectID, datasetID string) (string, error) {
	var graph graphCoordinates
	if err := readCoordinates(filepath.Join(graphDir, "manifest.json"), &graph); err != nil {
		return "", err
	}
	if graph.ProjectID == "" && graph.DatasetID == "" && graph.SnapshotID == "" {
		return "", nil
	}
	if graph.ProjectID != projectID || graph.DatasetID != datasetID || graph.SnapshotID == "" {
		return "", fmt.Errorf("semantic project/dataset flags do not match pinned graph identity")
	}
	return graph.SnapshotID, nil
}

func normalizedRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("empty source root")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(abs), nil
}

func readCoordinates(path string, out any) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(buf, out); err != nil {
		return err
	}
	return nil
}

// PutAligned is the promotion entry point for projections attached to built
// CKV/CKG datasets. It verifies engine coordinates and source bytes before
// the immutable projection is persisted. Put remains available for preparing
// an unpromoted projection without engine indexes.
func (s *Store) PutAligned(ctx context.Context, p Projection, repoRoot, graphDir, vectorDir string) error {
	if err := ValidateDatasetAlignment(p, repoRoot, graphDir, vectorDir); err != nil {
		return err
	}
	if err := ValidateCodeAnchors(p, graphDir); err != nil {
		return err
	}
	if err := ValidateCKVChunkLinks(ctx, p, vectorDir); err != nil {
		return err
	}
	return s.Put(ctx, p, repoRoot)
}

// PutAlignedRetained validates raw meaning against the immutable candidate
// archive, including when the source has no Git history or has since changed.
func (s *Store) PutAlignedRetained(ctx context.Context, p Projection, repoRoot, graphDir, vectorDir, versionDir string) error {
	if err := ValidateDatasetAlignment(p, repoRoot, graphDir, vectorDir); err != nil {
		return err
	}
	if err := ValidateCodeAnchors(p, graphDir); err != nil {
		return err
	}
	if err := ValidateCKVChunkLinks(ctx, p, vectorDir); err != nil {
		return err
	}
	if err := p.ValidateRetainedSources(versionDir); err != nil {
		return err
	}
	return s.putVerified(ctx, p)
}

// ActivateAligned rechecks current engine coordinates immediately before
// selecting a projection. It keeps an older stored candidate from becoming
// live after the graph/vector indexes have moved to another snapshot.
func (s *Store) ActivateAligned(ctx context.Context, projectID, datasetID, repoRoot, graphDir, vectorDir string) error {
	p, err := s.Load(ctx, projectID, datasetID)
	if err != nil {
		return err
	}
	if err := ValidateDatasetAlignment(p, repoRoot, graphDir, vectorDir); err != nil {
		return err
	}
	if err := ValidateCodeAnchors(p, graphDir); err != nil {
		return err
	}
	if err := ValidateCKVChunkLinks(ctx, p, vectorDir); err != nil {
		return err
	}
	if err := p.ValidateSources(ctx, repoRoot); err != nil {
		return err
	}
	return s.Activate(ctx, projectID, datasetID)
}

func (s *Store) ActivateAlignedRetained(ctx context.Context, projectID, datasetID, repoRoot, graphDir, vectorDir, versionDir string) error {
	p, err := s.Load(ctx, projectID, datasetID)
	if err != nil {
		return err
	}
	if err := ValidateDatasetAlignment(p, repoRoot, graphDir, vectorDir); err != nil {
		return err
	}
	if err := ValidateCodeAnchors(p, graphDir); err != nil {
		return err
	}
	if err := ValidateCKVChunkLinks(ctx, p, vectorDir); err != nil {
		return err
	}
	if err := p.ValidateRetainedSources(versionDir); err != nil {
		return err
	}
	return s.Activate(ctx, projectID, datasetID)
}
