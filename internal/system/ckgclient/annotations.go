package ckgclient

import (
	"context"
	"github.com/0xmhha/knowledge-system/pkg/graph/annotations"
	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
)

// AnnotationClient is optional; older backends explicitly report unavailable.
type AnnotationClient interface {
	PolicySecurity(context.Context, annotations.Input) (annotations.Result, error)
}

func (r *Real) PolicySecurity(ctx context.Context, q annotations.Input) (annotations.Result, error) {
	reader, ok := r.s.(*realStoreReader)
	if !ok || r.closed {
		return annotations.Result{SchemaVersion: 1, Status: "unavailable", Annotations: []annotations.Annotation{}, SafetyVerdict: "unknown", AutomaticRules: "separate_opt_in_rule_candidates_not_executed", Limitations: []string{"retained annotation backend unavailable"}}, nil
	}
	return annotations.Query(ctx, mcphandlers.NewLLMSafeReader(reader.r), q)
}
