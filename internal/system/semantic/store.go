package semantic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Store persists immutable semantic projections by project and dataset. The
// active pointer is separate, so a failed build cannot replace live meaning.
type Store struct{ db *sql.DB }

// StoreSchemaVersion tracks SQLite tables independently of the immutable
// projection JSON contract (SchemaVersion). Normalized term rows are derived
// indexes; migrating them never rewrites a stored projection document.
const StoreSchemaVersion = 3

const semanticTermsTable = `CREATE TABLE IF NOT EXISTS semantic_concept_terms (
	project_id TEXT NOT NULL, dataset_id TEXT NOT NULL,
	lang TEXT NOT NULL, normalized_term TEXT NOT NULL, concept_id TEXT NOT NULL,
	PRIMARY KEY(project_id, dataset_id, lang, normalized_term, concept_id),
	FOREIGN KEY(project_id, dataset_id) REFERENCES semantic_projections(project_id, dataset_id)
)`

func OpenStore(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("semantic: empty store path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", absolute)
	if err != nil {
		return nil, err
	}
	// SQLite connection PRAGMAs are connection-scoped. A single connection
	// keeps foreign-key and busy-timeout behavior consistent for this store.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL"} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version > StoreSchemaVersion {
		db.Close()
		return nil, fmt.Errorf("semantic store schema %d is newer than supported %d", version, StoreSchemaVersion)
	}
	if version == 0 {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS semantic_projections (
			project_id TEXT NOT NULL, dataset_id TEXT NOT NULL, commit_sha TEXT NOT NULL,
			digest TEXT NOT NULL, document BLOB NOT NULL,
			PRIMARY KEY (project_id, dataset_id)
		);
		CREATE TABLE IF NOT EXISTS semantic_current (
			project_id TEXT PRIMARY KEY, dataset_id TEXT NOT NULL,
			FOREIGN KEY(project_id, dataset_id) REFERENCES semantic_projections(project_id, dataset_id)
		);
		` + semanticTermsTable + `;
		PRAGMA user_version=3;`); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize semantic schema: %w", err)
		}
	}
	if version == 1 || version == 2 {
		// v2 added optional requirements inside immutable JSON. v3 adds a
		// normalized concept-term index; historical documents are not changed.
		if err := migrateTermIndex(db); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate semantic schema: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Put validates exact committed source bytes before storing a projection. A
// dataset ID is immutable: an identical retry succeeds, a changed document
// requires a new dataset ID and explicit promotion.
func (s *Store) Put(ctx context.Context, p Projection, repoRoot string) error {
	if err := p.ValidateSources(ctx, repoRoot); err != nil {
		return err
	}
	buf, err := json.Marshal(p)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(buf)
	digest := hex.EncodeToString(hash[:])
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO semantic_projections
		(project_id, dataset_id, commit_sha, digest, document) VALUES (?,?,?,?,?)
		ON CONFLICT(project_id, dataset_id) DO NOTHING`, p.Snapshot.ProjectID, p.Snapshot.DatasetID,
		p.Snapshot.Commit, digest, buf); err != nil {
		return err
	}
	var existing string
	if err := tx.QueryRowContext(ctx, `SELECT digest FROM semantic_projections WHERE project_id=? AND dataset_id=?`,
		p.Snapshot.ProjectID, p.Snapshot.DatasetID).Scan(&existing); err != nil {
		return err
	}
	if existing != digest {
		return fmt.Errorf("semantic dataset %s/%s already exists with different content", p.Snapshot.ProjectID, p.Snapshot.DatasetID)
	}
	if err := insertTermRows(ctx, tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

// LookupTerm resolves one exact, case-folded language term within a pinned
// dataset. It loads the immutable projection first so DB index rows cannot
// become stand-alone assertions. Natural-language matching remains in
// Projection.MatchConcepts; this indexed lookup supports vocabulary review.
func (s *Store) LookupTerm(ctx context.Context, projectID, datasetID, lang, term string) ([]ConceptCandidate, error) {
	p, err := s.Load(ctx, projectID, datasetID)
	if err != nil {
		return nil, err
	}
	lang, term = strings.ToLower(strings.TrimSpace(lang)), strings.ToLower(strings.TrimSpace(term))
	if lang == "" || term == "" {
		return nil, fmt.Errorf("semantic term lookup requires language and term")
	}
	var indexedCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM semantic_concept_terms WHERE project_id=? AND dataset_id=?`,
		projectID, datasetID).Scan(&indexedCount); err != nil {
		return nil, err
	}
	expected := 0
	concepts := make(map[string]Concept, len(p.Concepts))
	for _, concept := range p.Concepts {
		expected += len(concept.Terms)
		concepts[concept.ID] = concept
	}
	if indexedCount != expected {
		return nil, fmt.Errorf("semantic term index count mismatch for %s/%s", projectID, datasetID)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT concept_id FROM semantic_concept_terms
		WHERE project_id=? AND dataset_id=? AND lang=? AND normalized_term=?`, projectID, datasetID, lang, term)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ConceptCandidate, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		concept, ok := concepts[id]
		if !ok {
			return nil, fmt.Errorf("semantic term index has foreign concept %q", id)
		}
		matched := false
		for _, candidate := range concept.Terms {
			if strings.ToLower(candidate.Lang) != lang || strings.ToLower(strings.TrimSpace(candidate.Value)) != term {
				continue
			}
			matched = true
			if concept.Status != StatusRejected {
				score := 0.8
				if candidate.Preferred {
					score = 1
				}
				if concept.Status == StatusProposed {
					score *= 0.5
				}
				out = append(out, ConceptCandidate{ConceptID: id, Term: candidate.Value, Status: concept.Status, Score: score})
			}
			break
		}
		if !matched {
			return nil, fmt.Errorf("semantic term index differs from projection for concept %q", id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ConceptID < out[j].ConceptID
		}
		return out[i].Score > out[j].Score
	})
	return out, nil
}

// Activate changes only one project's active dataset pointer. It can also
// roll back to any retained earlier dataset of the same project.
func (s *Store) Activate(ctx context.Context, projectID, datasetID string) error {
	if projectID == "" || datasetID == "" {
		return errors.New("semantic activation needs project and dataset IDs")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var one int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM semantic_projections WHERE project_id=? AND dataset_id=?`, projectID, datasetID).Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("semantic dataset %s/%s does not exist", projectID, datasetID)
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO semantic_current(project_id,dataset_id) VALUES(?,?)
		ON CONFLICT(project_id) DO UPDATE SET dataset_id=excluded.dataset_id`, projectID, datasetID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Load(ctx context.Context, projectID, datasetID string) (Projection, error) {
	var buf []byte
	var digest string
	err := s.db.QueryRowContext(ctx, `SELECT document,digest FROM semantic_projections
		WHERE project_id=? AND dataset_id=?`, projectID, datasetID).Scan(&buf, &digest)
	if err != nil {
		return Projection{}, err
	}
	hash := sha256.Sum256(buf)
	if hex.EncodeToString(hash[:]) != digest {
		return Projection{}, fmt.Errorf("semantic projection digest mismatch for %s/%s", projectID, datasetID)
	}
	var p Projection
	if err := json.Unmarshal(buf, &p); err != nil {
		return Projection{}, err
	}
	if err := p.Validate(); err != nil {
		return Projection{}, err
	}
	if p.Snapshot.ProjectID != projectID || p.Snapshot.DatasetID != datasetID {
		return Projection{}, fmt.Errorf("semantic projection identity mismatch for %s/%s", projectID, datasetID)
	}
	return p, nil
}

func (s *Store) Current(ctx context.Context, projectID string) (Projection, error) {
	var datasetID string
	if err := s.db.QueryRowContext(ctx, `SELECT dataset_id FROM semantic_current WHERE project_id=?`, projectID).Scan(&datasetID); err != nil {
		return Projection{}, err
	}
	return s.Load(ctx, projectID, datasetID)
}
