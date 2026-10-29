package stage1

import (
	"context"
	"strings"
)

type codeQueryKey struct{}

// WithCodeQuery attaches a caller-written code-vocabulary rewrite of the
// prompt (expected identifiers, packages, files) to ctx. Extract uses it as
// the first semantic query and as the base for later augmented rounds; the
// prompt still drives intent classification and keyword extraction.
//
// Measured 2026-10-29 on the grpc-go dev split (35 PRs): the labelled core
// function in vector top 12 went from 16 to 23 (bge-m3) and 12 to 24
// (qwen3) when the Korean prompt was replaced by such a rewrite. Blank
// values leave ctx unchanged.
func WithCodeQuery(ctx context.Context, q string) context.Context {
	q = strings.TrimSpace(q)
	if q == "" {
		return ctx
	}
	return context.WithValue(ctx, codeQueryKey{}, q)
}

// CodeQueryFrom returns the code query attached by WithCodeQuery, or "".
func CodeQueryFrom(ctx context.Context) string {
	q, _ := ctx.Value(codeQueryKey{}).(string)
	return q
}
