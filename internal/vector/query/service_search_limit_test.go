package query

import (
	"context"
	"errors"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

func TestStoreSearchRejectsOversizedKBeforeOverfetch(t *testing.T) {
	svc := &StoreSearchService{}
	hits, err := svc.Run(context.Background(), nil,
		sqlitevec.DefaultMaxSearchK/overfetchFactor+1, types.Filter{})
	if !errors.Is(err, sqlitevec.ErrSearchIncomplete) || hits != nil {
		t.Fatalf("oversized overfetch must fail before store use: hits=%v err=%v", hits, err)
	}
}
