package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec"
)

func TestQueryFailureCodesPreserveCause(t *testing.T) {
	for _, tc := range []struct {
		cause error
		code  string
	}{
		{sqlitevec.ErrSearchIncomplete, "incomplete"},
		{sqlitevec.ErrIncompleteIndex, "incomplete"},
		{context.DeadlineExceeded, "incomplete"},
		{context.Canceled, "cancelled"},
	} {
		err := queryFailure(tc.cause)
		if !errors.Is(err, tc.cause) || !strings.HasPrefix(err.Error(), "code="+tc.code+": ") {
			t.Fatalf("query failure lost code or cause: %v", err)
		}
	}
}
