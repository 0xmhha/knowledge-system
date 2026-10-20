package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/annotations"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type hostSelectorBackend struct {
	ckgclient.Client
	calls    int
	selector string
}

func (b *hostSelectorBackend) ValueDependencies(_ context.Context, q valueflow.QueryInput) (valueflow.QueryResult, error) {
	b.calls++
	b.selector = q.NodeID + q.CanonicalID
	return valueflow.QueryResult{SchemaVersion: 1, Status: "not_found"}, nil
}
func (b *hostSelectorBackend) RuleCandidates(_ context.Context, _ valueflow.RuleQueryInput) (valueflow.QueryResult, error) {
	b.calls++
	return valueflow.QueryResult{SchemaVersion: 1, Status: "not_found"}, nil
}
func (b *hostSelectorBackend) PolicySecurity(_ context.Context, q annotations.Input) (annotations.Result, error) {
	b.calls++
	b.selector = q.NodeID + q.CanonicalID
	return annotations.Result{SchemaVersion: 1, Status: "not_found", Annotations: []annotations.Annotation{}, SafetyVerdict: "unknown"}, nil
}

// Advertising a host-compatible object must not relax the runtime trust boundary.
func TestHostCompatibleMetadataRetainsExclusiveSelectorRuntime(t *testing.T) {
	for _, name := range []string{ToolNameValueDependencies, ToolNamePolicySecurity} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, nil)
			backend := &hostSelectorBackend{Client: f.deps.CKG}
			f.deps.CKG = backend
			s := mcpserver.NewMCPServer("host-selector", "1")
			if e := Register(s, f.deps); e != nil {
				t.Fatal(e)
			}
			tool := s.GetTool(name)
			raw, e := json.Marshal(tool.Tool)
			if e != nil {
				t.Fatal(e)
			}
			var metadata struct {
				Input map[string]any `json:"inputSchema"`
			}
			if e = json.Unmarshal(raw, &metadata); e != nil {
				t.Fatal(e)
			}
			for _, key := range []string{"oneOf", "anyOf", "allOf"} {
				if _, exists := metadata.Input[key]; exists {
					t.Fatal("host excludes root composition", key)
				}
			}
			if metadata.Input["type"] != "object" || metadata.Input["additionalProperties"] != false {
				t.Fatal("metadata boundary lost")
			}
			for _, args := range []string{`{"node_id":"exact"}`, `{"canonical_id":"example.invalid/p.F"}`} {
				req := mcpgo.CallToolRequest{}
				req.Params.RawArguments = json.RawMessage(args)
				before := backend.calls
				result, err := tool.Handler(context.Background(), req)
				if err != nil || result == nil || result.IsError || backend.calls != before+1 || backend.selector == "" {
					t.Fatal("valid exact selector did not dispatch", args, err)
				}
			}
			invalid := []string{`{}`, `{"node_id":"a","canonical_id":"b"}`, `{"node_id":null}`, `{"node_id":17}`, `{"node_id":""}`, `{"canonical_id":false}`, `{"node_id":"a","unknown":true}`, `{"node_id":"a","node_id":"b"}`, `{"node_id":"a","expected_commit":"main"}`, `{"node_id":"a","expected_source_sha256":"bad"}`, `{"node_id":"a","expected_snapshot_sha256":null}`}
			if name == ToolNameValueDependencies {
				invalid = append(invalid, `{"node_id":"a","limits":{"max_nodes":0}}`, `{"node_id":"a","limits":{"max_nodes":1025}}`, `{"node_id":"a","limits":{"max_nodes":1.5}}`, `{"node_id":"a","limits":{"max_nodes":null}}`)
			} else {
				invalid = append(invalid, `{"node_id":"a","max_annotations":0}`, `{"node_id":"a","max_annotations":129}`, `{"node_id":"a","max_annotations":1.5}`, `{"node_id":"a","timeout_ms":5001}`)
			}
			for _, args := range invalid {
				req := mcpgo.CallToolRequest{}
				req.Params.RawArguments = json.RawMessage(args)
				before := backend.calls
				if _, err := tool.Handler(context.Background(), req); err == nil {
					t.Error("invalid runtime input accepted", args)
				}
				if backend.calls != before {
					t.Error("invalid input reached backend", args)
				}
			}
		})
	}
}
