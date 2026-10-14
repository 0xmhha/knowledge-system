package mcp

import (
	"context"
	"encoding/json"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/annotations"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type annotationFake struct {
	ckgclient.Client
	calls int
	input annotations.Input
}

func (f *annotationFake) PolicySecurity(_ context.Context, q annotations.Input) (annotations.Result, error) {
	f.calls++
	f.input = q
	return annotations.Result{SchemaVersion: 1, Status: "no_annotations", Annotations: []annotations.Annotation{}, SafetyVerdict: "unknown", AutomaticRules: "separate_opt_in_rule_candidates_not_executed", Limitations: []string{}}, nil
}
func TestPolicySecurityPublicContractLegacyAndStrictRuntime(t *testing.T) {
	f := newFixture(t, nil)
	backend := &annotationFake{Client: f.deps.CKG}
	f.deps.CKG = backend
	s := mcpserver.NewMCPServer("fixture", "1", mcpserver.WithOutputSchemaValidation())
	if e := Register(s, f.deps); e != nil {
		t.Fatal(e)
	}
	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + ToolNamePolicySecurity + `","arguments":{"node_id":"exact","max_annotations":2}}}`)
	raw, _ := json.Marshal(s.HandleMessage(context.Background(), request))
	if !strings.Contains(string(raw), `"safety_verdict":"unknown"`) || backend.calls != 1 || backend.input.MaxAnnotations != 2 {
		t.Fatal(string(raw))
	}
	for _, input := range []string{`{"node_id":"a","node_id":"b"}`, `{"node_id":null}`, `{"node_id":"a","max_annotations":1.5}`, `{"node_id":"a","max_annotations":0}`, `{"node_id":"a","unknown":1}`} {
		req := mcpgo.CallToolRequest{}
		req.Params.RawArguments = json.RawMessage(input)
		if _, e := s.GetTool(ToolNamePolicySecurity).Handler(context.Background(), req); e == nil {
			t.Fatal(input)
		}
	}
	legacy := newFixture(t, nil)
	other := mcpserver.NewMCPServer("legacy", "1")
	if e := Register(other, legacy.deps); e != nil {
		t.Fatal(e)
	}
	raw, _ = json.Marshal(other.HandleMessage(context.Background(), request))
	if !strings.Contains(string(raw), `"status":"unavailable"`) {
		t.Fatal(string(raw))
	}
}
func TestPolicySecurityHTTPACLRejectsBeforeToolAndIgnoresForwardedIP(t *testing.T) {
	f := newFixture(t, nil)
	backend := &annotationFake{Client: f.deps.CKG}
	f.deps.CKG = backend
	s, e := build(f.deps)
	if e != nil {
		t.Fatal(e)
	}
	allow, e := clientACL(HTTPPolicy{AllowRemote: true, AllowedCIDRs: []string{"192.168.1.0/24"}})
	if e != nil {
		t.Fatal(e)
	}
	h := aclMiddleware(mcpserver.NewStreamableHTTPServer(s), allow)
	for _, peer := range []string{"203.0.113.9:5555", "192.168.2.1:5555", "invalid-peer"} {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+ToolNamePolicySecurity+`","arguments":{"node_id":"exact"}}}`))
		req.RemoteAddr = peer
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden || backend.calls != 0 {
			t.Fatal(peer, w.Code, backend.calls)
		}
	}
	// Loopback reaches the HTTP handler, independent of readOnlyHint metadata.
	req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"fixture","version":"1"}}}`))
	req.RemoteAddr = "127.0.0.1:5555"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"`+ToolNamePolicySecurity+`","arguments":{"node_id":"exact"}}}`))
	req.RemoteAddr = "127.0.0.1:5555"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", w.Header().Get("Mcp-Session-Id"))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || backend.calls != 1 || !strings.Contains(w.Body.String(), `"safety_verdict":"unknown"`) {
		t.Fatal(w.Code, w.Body.String(), backend.calls)
	}
	t.Log("HTTP policy_security invocation: loopback allowed; unlisted CIDR/public/invalid peer denied before backend; forwarded loopback cannot bypass ACL")

}
