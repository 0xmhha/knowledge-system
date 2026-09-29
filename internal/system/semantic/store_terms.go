package semantic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func migrateTermIndex(db *sql.DB) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, semanticTermsTable); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT project_id,dataset_id,digest,document FROM semantic_projections`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var projectID, datasetID, digest string
		var document []byte
		if err := rows.Scan(&projectID, &datasetID, &digest, &document); err != nil {
			rows.Close()
			return err
		}
		hash := sha256.Sum256(document)
		if hex.EncodeToString(hash[:]) != digest {
			rows.Close()
			return fmt.Errorf("projection %s/%s digest mismatch during term migration", projectID, datasetID)
		}
		var p Projection
		if err := json.Unmarshal(document, &p); err != nil {
			rows.Close()
			return err
		}
		if err := p.Validate(); err != nil {
			rows.Close()
			return err
		}
		if p.Snapshot.ProjectID != projectID || p.Snapshot.DatasetID != datasetID {
			rows.Close()
			return fmt.Errorf("projection %s/%s identity mismatch during term migration", projectID, datasetID)
		}
		if err := insertTermRows(ctx, tx, p); err != nil {
			rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version=3"); err != nil {
		return err
	}
	return tx.Commit()
}

func insertTermRows(ctx context.Context, tx *sql.Tx, p Projection) error {
	for _, concept := range p.Concepts {
		for _, term := range concept.Terms {
			if _, err := tx.ExecContext(ctx, `INSERT INTO semantic_concept_terms
				(project_id,dataset_id,lang,normalized_term,concept_id) VALUES (?,?,?,?,?)
				ON CONFLICT DO NOTHING`, p.Snapshot.ProjectID, p.Snapshot.DatasetID,
				strings.ToLower(term.Lang), strings.ToLower(strings.TrimSpace(term.Value)), concept.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
