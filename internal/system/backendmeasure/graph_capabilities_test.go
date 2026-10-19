package backendmeasure

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

type retainedProbe struct {
	ckgclient.Client
	ckgclient.CitationResolver
	ckgclient.AnnotationClient // Presence only; deferred policy evaluation is not invoked.
	ctx                        context.Context
	input                      any
	calls                      int
	err                        error
}

func (p *retainedProbe) ValueDependencies(ctx context.Context, q valueflow.QueryInput) (valueflow.QueryResult, error) {
	p.ctx, p.input = ctx, q
	p.calls++
	return valueflow.QueryResult{Status: "partial", Reason: "bounded source"}, p.err
}
func (p *retainedProbe) RuleCandidates(ctx context.Context, q valueflow.RuleQueryInput) (valueflow.QueryResult, error) {
	p.ctx, p.input = ctx, q
	p.calls++
	return valueflow.QueryResult{Status: "partial", Reason: "bounded rules"}, p.err
}
func (p *retainedProbe) ValueDependencyExpansion(ctx context.Context, q valueflow.ExpansionInput) (valueflow.ExpansionResult, error) {
	p.ctx, p.input = ctx, q
	p.calls++
	return valueflow.ExpansionResult{Status: "partial", StopReasons: []string{"bounded"}}, p.err
}

func TestRetainedMeasurementDelegatesSourceInputsResultsAndErrors(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			p := &retainedProbe{Client: graphWithoutResolver{Client: &ckgclient.Fake{}}}
			if failure {
				p.err = errors.New("owned provider failure")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure {
				cancel()
			}
			wrapped := WrapCKG(p)
			analysis := wrapped.(ckgclient.ValueAnalysisClient)
			input := valueflow.QueryInput{NodeID: "retained-node", ExpectedCommit: "source-pin"}
			got, err := analysis.ValueDependencies(ctx, input)
			if got.Status != "partial" || got.Reason != "bounded source" || err != p.err || p.ctx != ctx || !reflect.DeepEqual(p.input, input) || p.calls != 1 {
				t.Fatal("value analysis delegation changed")
			}
			rules := valueflow.RuleQueryInput{Function: input, RulePack: "go-local-review@1.0.0", MaxCandidates: 3}
			got, err = analysis.RuleCandidates(ctx, rules)
			if got.Status != "partial" || got.Reason != "bounded rules" || err != p.err || p.ctx != ctx || !reflect.DeepEqual(p.input, rules) || p.calls != 2 {
				t.Fatal("rule analysis delegation changed")
			}
			expansion := valueflow.ExpansionInput{Function: input}
			expanded, err := wrapped.(ckgclient.ValueExpansionClient).ValueDependencyExpansion(ctx, expansion)
			if expanded.Status != "partial" || !reflect.DeepEqual(expanded.StopReasons, []string{"bounded"}) || err != p.err || p.ctx != ctx || !reflect.DeepEqual(p.input, expansion) || p.calls != 3 {
				t.Fatal("expansion delegation changed")
			}
		})
	}
}

// Narrow the fixture's static method set to each independently declared subset.
func graphCapabilityFixture(mask int, p *retainedProbe) ckgclient.Client {
	switch mask {
	case 1:
		return &struct {
			ckgclient.Client
			ckgclient.CitationResolver
		}{p.Client, p}
	case 2:
		return &struct {
			ckgclient.Client
			ckgclient.ValueAnalysisClient
		}{p.Client, p}
	case 3:
		return &struct {
			ckgclient.Client
			ckgclient.CitationResolver
			ckgclient.ValueAnalysisClient
		}{p.Client, p, p}
	case 4:
		return &struct {
			ckgclient.Client
			ckgclient.ValueExpansionClient
		}{p.Client, p}
	case 5:
		return &struct {
			ckgclient.Client
			ckgclient.CitationResolver
			ckgclient.ValueExpansionClient
		}{p.Client, p, p}
	case 6:
		return &struct {
			ckgclient.Client
			ckgclient.ValueAnalysisClient
			ckgclient.ValueExpansionClient
		}{p.Client, p, p}
	case 7:
		return &struct {
			ckgclient.Client
			ckgclient.CitationResolver
			ckgclient.ValueAnalysisClient
			ckgclient.ValueExpansionClient
		}{p.Client, p, p, p}
	case 8:
		return &struct {
			ckgclient.Client
			ckgclient.AnnotationClient
		}{p.Client, p}
	case 9:
		return &struct {
			ckgclient.Client
			ckgclient.CitationResolver
			ckgclient.AnnotationClient
		}{p.Client, p, p}
	case 10:
		return &struct {
			ckgclient.Client
			ckgclient.ValueAnalysisClient
			ckgclient.AnnotationClient
		}{p.Client, p, p}
	case 11:
		return &struct {
			ckgclient.Client
			ckgclient.CitationResolver
			ckgclient.ValueAnalysisClient
			ckgclient.AnnotationClient
		}{p.Client, p, p, p}
	case 12:
		return &struct {
			ckgclient.Client
			ckgclient.ValueExpansionClient
			ckgclient.AnnotationClient
		}{p.Client, p, p}
	case 13:
		return &struct {
			ckgclient.Client
			ckgclient.CitationResolver
			ckgclient.ValueExpansionClient
			ckgclient.AnnotationClient
		}{p.Client, p, p, p}
	case 14:
		return &struct {
			ckgclient.Client
			ckgclient.ValueAnalysisClient
			ckgclient.ValueExpansionClient
			ckgclient.AnnotationClient
		}{p.Client, p, p, p}
	case 15:
		return &struct {
			ckgclient.Client
			ckgclient.CitationResolver
			ckgclient.ValueAnalysisClient
			ckgclient.ValueExpansionClient
			ckgclient.AnnotationClient
		}{p.Client, p, p, p, p}
	default:
		return p.Client
	}
}

func TestGraphMeasurementKeepsEveryOptionalMethodSubset(t *testing.T) {
	for mask := 0; mask < 16; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			p := &retainedProbe{Client: graphWithoutResolver{Client: &ckgclient.Fake{}}}
			got := WrapCKG(graphCapabilityFixture(mask, p))
			_, resolver := got.(ckgclient.CitationResolver)
			_, analysis := got.(ckgclient.ValueAnalysisClient)
			_, expansion := got.(ckgclient.ValueExpansionClient)
			_, annotation := got.(ckgclient.AnnotationClient)
			actual := []bool{resolver, analysis, expansion, annotation}
			for bit, present := range actual {
				if present != (mask&(1<<bit) != 0) {
					t.Fatalf("capability%d present=%v in subset%d", bit, present, mask)
				}
			}
		})
	}
}
