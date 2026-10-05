// b0-vector-probe measures frozen CKV vectors without changing the index.
// It is an evaluation utility, not a production retrieval endpoint.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/0xmhha/knowledge-system/internal/vector/embed/mock"
	"github.com/0xmhha/knowledge-system/internal/vector/manifest"
	"github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec"
	"github.com/0xmhha/knowledge-system/pkg/vector/embed/ollama"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

type oracleHit struct {
	ID        string  `json:"id"`
	File      string  `json:"file"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Distance  float64 `json:"distance"`
}

type oracleResult struct {
	EligibleCount int         `json:"eligible_count"`
	Hits          []oracleHit `json:"hits"`
	// Preserve the full independent inventory before top-K truncation. The
	// sparse fixture verdict must not infer all eligible files from top K alone.
	EligibleHits []oracleHit `json:"eligible_hits"`
}

type probeReport struct {
	SchemaVersion          int                     `json:"schema_version"`
	Gate                   string                  `json:"gate"`
	QualityMetrics         any                     `json:"quality_metrics"`
	State                  string                  `json:"state"`
	Error                  string                  `json:"error,omitempty"`
	ManifestSHA256         string                  `json:"manifest_sha256"`
	ManifestSHA256After    string                  `json:"manifest_sha256_after"`
	DatabaseSHA256Before   string                  `json:"database_sha256_before"`
	DatabaseSHA256After    string                  `json:"database_sha256_after"`
	EmbeddingIdentity      types.EmbeddingIdentity `json:"embedding_identity"`
	IdentityVerifiedAfter  bool                    `json:"identity_verified_after"`
	Coordinates            map[string]string       `json:"coordinates"`
	CoordinateState        string                  `json:"coordinate_state"`
	Prompt                 string                  `json:"prompt"`
	QueryVector            []float32               `json:"query_vector"`
	EmbedNS                int64                   `json:"embed_ns"`
	SearchNS               map[string]int64        `json:"search_ns"`
	Filter                 types.Filter            `json:"filter"`
	K                      int                     `json:"k"`
	MaxExactCandidates     int                     `json:"max_exact_candidates"`
	Unfiltered             sqlitevec.SearchResult  `json:"unfiltered"`
	Filtered               sqlitevec.SearchResult  `json:"filtered"`
	Budget                 sqlitevec.SearchResult  `json:"budget"`
	Oracle                 oracleResult            `json:"oracle"`
	ExactAgreement         bool                    `json:"exact_agreement"`
	SparseFixtureQualified bool                    `json:"sparse_fixture_qualified"`
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func requireCheckpointed(path string) error {
	for _, suffix := range []string{"-wal", "-journal"} {
		info, err := os.Stat(path + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Size() > 0 {
			return fmt.Errorf("unsealed database side file %s", suffix)
		}
	}
	return nil
}

func parseFilter(raw []byte) (types.Filter, error) {
	var filter types.Filter
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&filter); err != nil {
		return filter, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return filter, fmt.Errorf("filter must be one JSON object")
	}
	return filter, nil
}

// exactOracle intentionally scans all metadata and stored vectors. It does not
// reuse candidateWhere, SearchDetailed or the store's exact fallback, so SQL
// prefilter errors cannot make both the implementation and oracle agree.
func exactOracle(ctx context.Context, path string, query []float32, filter types.Filter, k int) (oracleResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return oracleResult{}, err
	}
	uri := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return oracleResult{}, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `SELECT c.id,c.file,c.start_line,c.end_line,c.language,c.is_test,
	 COALESCE(c.symbol_kind,''),c.chunk_kind,c.commit_hash,v.embedding
	 FROM chunks c LEFT JOIN chunk_vec v ON v.chunk_id=c.id`)
	if err != nil {
		return oracleResult{}, err
	}
	defer rows.Close()
	result := oracleResult{Hits: []oracleHit{}, EligibleHits: []oracleHit{}}
	for rows.Next() {
		var chunk types.Chunk
		var isTest int
		var blob []byte
		if err := rows.Scan(&chunk.ID, &chunk.File, &chunk.StartLine, &chunk.EndLine, &chunk.Language, &isTest, &chunk.SymbolKind, &chunk.ChunkKind, &chunk.CommitHash, &blob); err != nil {
			return result, err
		}
		chunk.IsTest = isTest != 0
		if !filter.Matches(chunk) {
			continue
		}
		if len(blob) != 4*len(query) {
			return result, fmt.Errorf("oracle missing or wrong-dimension vector for %s", chunk.ID)
		}
		var sum float64
		for i, value := range query {
			stored := math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
			if math.IsNaN(float64(stored)) || math.IsInf(float64(stored), 0) {
				return result, fmt.Errorf("oracle nonfinite vector")
			}
			d := float64(value) - float64(stored)
			sum += d * d
		}
		result.EligibleCount++
		result.Hits = append(result.Hits, oracleHit{ID: chunk.ID, File: chunk.File, StartLine: chunk.StartLine, EndLine: chunk.EndLine, Distance: math.Sqrt(sum)})
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	sort.Slice(result.Hits, func(i, j int) bool {
		if result.Hits[i].Distance != result.Hits[j].Distance {
			return result.Hits[i].Distance < result.Hits[j].Distance
		}
		return result.Hits[i].ID < result.Hits[j].ID
	})
	result.EligibleHits = append(result.EligibleHits, result.Hits...)
	if len(result.Hits) > k {
		result.Hits = result.Hits[:k]
	}
	return result, nil
}

func probe(ctx context.Context, directory, prompt string, filter types.Filter, k, budget int, embedder types.Embedder) (report probeReport, runErr error) {
	report = probeReport{SchemaVersion: 1, Gate: "B0-vector-probe", State: "recorded", Prompt: prompt, Filter: filter, K: k, MaxExactCandidates: budget, SearchNS: map[string]int64{}}
	defer func() {
		if runErr != nil {
			report.State = "error"
			report.Error = runErr.Error()
		}
	}()
	if prompt == "" || k <= 0 || k > sqlitevec.DefaultMaxSearchK || budget < 0 {
		return report, fmt.Errorf("invalid query/K/budget")
	}
	if filter.PathGlob != "" {
		if _, err := filepath.Match(filter.PathGlob, "fixture.go"); err != nil {
			return report, err
		}
	}
	manifestPath := filepath.Join(directory, "manifest.json")
	dbPath := filepath.Join(directory, "vector.db")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return report, err
	}
	var metadata manifest.Manifest
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return report, err
	}
	sum := sha256.Sum256(raw)
	report.ManifestSHA256 = hex.EncodeToString(sum[:])
	report.EmbeddingIdentity = embedder.Identity()
	report.Coordinates = map[string]string{"project_id": metadata.ProjectID, "snapshot_id": metadata.SnapshotID, "dataset_id": metadata.DatasetID, "commit": metadata.SrcCommit}
	if metadata.EmbeddingChecksum != embedder.Identity().Checksum() || metadata.EmbeddingDim != embedder.Dimension() {
		return report, fmt.Errorf("embedding identity mismatch")
	}
	report.DatabaseSHA256Before, err = fileHash(dbPath)
	if err != nil {
		return report, err
	}
	if metadata.DBSHA256 == "" || metadata.DBSHA256 != report.DatabaseSHA256Before {
		return report, fmt.Errorf("published database hash mismatch or missing")
	}
	if err := requireCheckpointed(dbPath); err != nil {
		return report, err
	}
	store, err := sqlitevec.OpenReadOnly(dbPath, embedder.Dimension())
	if err != nil {
		return report, err
	}
	defer store.Close()
	abs, _ := filepath.Abs(dbPath)
	uri := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return report, err
	}
	defer db.Close()
	var nativeChecksum string
	if err := db.QueryRowContext(ctx, "SELECT value FROM manifest WHERE key='embedding_checksum'").Scan(&nativeChecksum); err != nil {
		return report, err
	}
	if nativeChecksum != metadata.EmbeddingChecksum {
		return report, fmt.Errorf("DB/sidecar embedding identity mismatch")
	}
	report.CoordinateState = "unpinned"
	if metadata.ProjectID != "" && metadata.SnapshotID != "" && metadata.DatasetID != "" {
		report.CoordinateState = "pinned"
	}
	for key, expected := range map[string]string{
		"project_id": metadata.ProjectID, "snapshot_id": metadata.SnapshotID, "dataset_id": metadata.DatasetID,
		"file_manifest_digest": metadata.FileManifestDigest, "capture_policy_digest": metadata.CapturePolicyDigest,
	} {
		var actual string
		readErr := db.QueryRowContext(ctx, "SELECT value FROM manifest WHERE key=?", key).Scan(&actual)
		if readErr != nil && !(readErr == sql.ErrNoRows && expected == "") {
			return report, readErr
		}
		if actual != expected {
			return report, fmt.Errorf("DB/sidecar coordinate mismatch: %s", key)
		}
	}
	started := time.Now()
	var vectors [][]float32
	if asymmetric, ok := embedder.(types.QueryEmbedder); ok {
		vectors, err = asymmetric.EmbedQuery(ctx, []string{prompt})
	} else {
		vectors, err = embedder.Embed(ctx, []string{prompt})
	}
	report.EmbedNS = time.Since(started).Nanoseconds()
	if err != nil {
		return report, err
	}
	if len(vectors) != 1 || len(vectors[0]) != embedder.Dimension() {
		return report, fmt.Errorf("query vector dimension mismatch")
	}
	report.QueryVector = vectors[0]
	for _, v := range vectors[0] {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return report, fmt.Errorf("query vector is nonfinite")
		}
	}
	started = time.Now()
	report.Unfiltered, err = store.SearchDetailed(ctx, vectors[0], k, types.Filter{}, sqlitevec.SearchOptions{})
	report.SearchNS["unfiltered"] = time.Since(started).Nanoseconds()
	if err != nil {
		return report, err
	}
	started = time.Now()
	report.Filtered, err = store.SearchDetailed(ctx, vectors[0], k, filter, sqlitevec.SearchOptions{})
	report.SearchNS["filtered"] = time.Since(started).Nanoseconds()
	if err != nil {
		return report, err
	}
	started = time.Now()
	report.Budget, err = store.SearchDetailed(ctx, vectors[0], k, filter, sqlitevec.SearchOptions{MaxExactCandidates: budget})
	report.SearchNS["budget"] = time.Since(started).Nanoseconds()
	if err != nil {
		return report, err
	}
	started = time.Now()
	report.Oracle, err = exactOracle(ctx, dbPath, vectors[0], filter, k)
	report.SearchNS["oracle"] = time.Since(started).Nanoseconds()
	if err != nil {
		return report, err
	}
	report.ExactAgreement = report.Filtered.Status == sqlitevec.SearchComplete && len(report.Filtered.Hits) == len(report.Oracle.Hits)
	for i, hit := range report.Filtered.Hits {
		if i >= len(report.Oracle.Hits) || hit.Chunk.ID != report.Oracle.Hits[i].ID || math.Abs(hit.Score.VectorDistance-report.Oracle.Hits[i].Distance) > 1e-6 {
			report.ExactAgreement = false
			break
		}
	}
	unfilteredFiles := map[string]bool{}
	for _, hit := range report.Unfiltered.Hits {
		unfilteredFiles[hit.Chunk.File] = true
	}
	for _, hit := range report.Oracle.Hits {
		if !unfilteredFiles[hit.File] {
			report.SparseFixtureQualified = true
		}
	}
	if verifier, ok := embedder.(types.IdentityVerifier); ok {
		if err := verifier.VerifyIdentity(ctx); err != nil {
			return report, err
		}
	}
	report.IdentityVerifiedAfter = true
	report.DatabaseSHA256After, err = fileHash(dbPath)
	if err != nil {
		return report, err
	}
	if report.DatabaseSHA256Before != report.DatabaseSHA256After {
		return report, fmt.Errorf("database changed during probe")
	}
	report.ManifestSHA256After, err = fileHash(manifestPath)
	if err != nil {
		return report, err
	}
	if report.ManifestSHA256 != report.ManifestSHA256After {
		return report, fmt.Errorf("manifest changed during probe")
	}
	if err := requireCheckpointed(dbPath); err != nil {
		return report, err
	}
	return report, nil
}

func main() {
	directory := flag.String("vector-dir", "", "published vector directory (required)")
	prompt := flag.String("query", "", "raw query (required)")
	filterPath := flag.String("filter-file", "", "JSON CKV filter file")
	k := flag.Int("k", 5, "top K")
	budget := flag.Int("max-exact-candidates", 2, "bounded fallback candidate limit")
	provider := flag.String("embedder", "ollama", "ollama or mock")
	model := flag.String("model-name", "bge-m3:latest", "exact Ollama model tag")
	endpoint := flag.String("ollama-url", "http://127.0.0.1:11434", "local Ollama endpoint")
	flag.Parse()
	var filter types.Filter
	var err error
	if *filterPath != "" {
		var raw []byte
		raw, err = os.ReadFile(*filterPath)
		if err == nil {
			filter, err = parseFilter(raw)
		}
	}
	var embedder types.Embedder
	if err == nil {
		switch *provider {
		case "mock":
			embedder = mock.Default()
		case "ollama":
			parsed, parseErr := url.Parse(*endpoint)
			if parseErr != nil || parsed.Scheme != "http" || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" && parsed.Hostname() != "::1") {
				err = fmt.Errorf("Ollama endpoint must be local HTTP loopback")
			} else {
				embedder, err = ollama.Open(ollama.Options{Endpoint: *endpoint, ModelName: *model, QueryPrefixPolicy: "registry"})
			}
		default:
			err = fmt.Errorf("unsupported embedder")
		}
	}
	report := probeReport{SchemaVersion: 1, Gate: "B0-vector-probe", State: "error"}
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		report, err = probe(ctx, *directory, *prompt, filter, *k, *budget, embedder)
	} else {
		report.Error = err.Error()
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if encodeErr := encoder.Encode(report); encodeErr != nil {
		fmt.Fprintln(os.Stderr, encodeErr)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "B0 vector probe failed; inspect the preserved JSON")
		os.Exit(1)
	}
}
