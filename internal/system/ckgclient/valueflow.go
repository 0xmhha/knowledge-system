package ckgclient

import (
	"context"

	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

// ValueAnalysisClient is an optional backend capability. Older/dummy backends
// stay available and explicitly report unavailable; they never invent analysis.
type ValueAnalysisClient interface {
	ValueDependencies(context.Context, valueflow.QueryInput) (valueflow.QueryResult, error)
	RuleCandidates(context.Context, valueflow.RuleQueryInput) (valueflow.QueryResult, error)
}

var _ ValueAnalysisClient = (*Real)(nil)

func (r *Real) ValueDependencies(ctx context.Context, q valueflow.QueryInput) (valueflow.QueryResult, error) {
	reader, ok := r.s.(*realStoreReader)
	if !ok || r.closed {
		return valueflow.QueryResult{SchemaVersion: 1, Status: "unavailable", Reason: "retained value-analysis backend unavailable"}, nil
	}
	return valueflow.Query(ctx, mcphandlers.NewLLMSafeReader(reader.r), q)
}
func (r *Real) RuleCandidates(ctx context.Context, q valueflow.RuleQueryInput) (valueflow.QueryResult, error) {
	reader, ok := r.s.(*realStoreReader)
	if !ok || r.closed {
		return valueflow.QueryResult{SchemaVersion: 1, Status: "unavailable", Reason: "retained rule-analysis backend unavailable"}, nil
	}
	return valueflow.QueryRules(ctx, mcphandlers.NewLLMSafeReader(reader.r), q)
}
