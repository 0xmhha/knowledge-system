package store

import (
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5"
)

// IsMissingRow identifies an absent optional retained row for both backends.
// Other storage failures must remain unavailable, never empty/safe results.
func IsMissingRow(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}
