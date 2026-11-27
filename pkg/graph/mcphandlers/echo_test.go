package mcphandlers

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"
)

// longInputArgs fills every argument of a tool's schema with long input: a
// string, a list of one string, an object holding one.
func longInputArgs(tool mcp.Tool, long string) map[string]any {
	args := map[string]any{}
	for name, p := range tool.InputSchema.Properties {
		prop, _ := p.(map[string]any)
		switch prop["type"] {
		case "string":
			args[name] = long
		case "array":
			args[name] = []any{long}
		case "object":
			args[name] = map[string]any{"canonical_id": long}
		}
	}
	return args
}

// answerStrings returns every string the answer holds, decoded: the text
// blocks and each string inside them and inside structuredContent.
func answerStrings(t *testing.T, res *mcp.CallToolResult) []string {
	t.Helper()
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			out = append(out, x)
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for k, e := range x {
				out = append(out, k)
				walk(e)
			}
		}
	}
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			out = append(out, tc.Text)
			var v any
			if json.Unmarshal([]byte(tc.Text), &v) == nil {
				walk(v)
			}
		}
	}
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		var v any
		if json.Unmarshal(b, &v) == nil {
			walk(v)
		}
	}
	return out
}

// TestEveryTool_CutsLongEchoes: no tool's answer, error answers included,
// repeats more than maxEchoBytes of a caller argument, for plain and
// escaping-heavy input in every content mode, and every answer stays
// within the default page size. A 5,000-byte pattern was repeated whole
// (10,175 bytes) and one of "<&>" characters reached 65,120 bytes.
func TestEveryTool_CutsLongEchoes(t *testing.T) {
	store := newFixtureStore(t)
	for _, long := range []string{strings.Repeat("hub", 2000), strings.Repeat("<&>\"\\", 1200)} {
		for _, m := range ContentModes {
			withContentMode(t, m)
			s := server.NewMCPServer("test", "0")
			RegisterAll(s, store)
			tools := s.ListTools()
			if len(tools) < 17 {
				t.Fatalf("%d tools registered, want 17 or more", len(tools))
			}
			for name, tool := range tools {
				req := mcp.CallToolRequest{}
				req.Params.Name = name
				req.Params.Arguments = longInputArgs(tool.Tool, long)
				res, err := tool.Handler(context.Background(), req)
				quoted := strconv.Quote(long)
				var strs []string
				if err != nil {
					strs = []string{err.Error()} // sent as the JSON-RPC error message
				} else if res != nil {
					strs = answerStrings(t, res)
				}
				for _, str := range strs {
					if strings.Contains(str, long[:maxEchoBytes+len(long)/1000]) || strings.Contains(str, quoted[1:2*maxEchoBytes]) {
						t.Errorf("%s %s: the answer repeats more than %d bytes of the input", m, name, maxEchoBytes)
						break
					}
				}
				if res == nil {
					continue
				}
				if w := wireContentBytes(res); w > searchCodePageBytes {
					t.Errorf("%s %s: answer is %d bytes, over %d", m, name, w, searchCodePageBytes)
				}
			}
		}
	}
}

// TestSearchCode_EchoCutBeforeSizing: for every pattern length from 190 to
// 230 bytes, plain and escaping-heavy, in every mode, at the floor and at
// the default bound, each page of the answer fits the bound with the cut
// echo and input_cut in it, and the echo is never longer than the input.
// The cut ran after paging: 4 of 90 floor cases went over (40,057 bytes at
// 40,000; 3,008 at 3,000).
func TestSearchCode_EchoCutBeforeSizing(t *testing.T) {
	store := newFixtureStore(t)
	old := searchCodePageBytes
	defer func() { searchCodePageBytes = old }()
	for _, m := range ContentModes {
		withContentMode(t, m)
		s := server.NewMCPServer("test", "0")
		RegisterAll(s, store)
		tool := s.GetTool("search_code")
		// The floor, the default, and bounds between where a page is
		// filled close to the bound.
		bounds := []int{MinSearchCodePageBytes, 40000}
		for b := MinSearchCodePageBytes + 137; b < 9000; b += 611 {
			bounds = append(bounds, b)
		}
		for _, bound := range bounds {
			searchCodePageBytes = bound
			for n := 190; n <= 230; n++ {
				for _, pattern := range []string{
					("func |" + strings.Repeat("q", n))[:n],
					("err|" + strings.Repeat("<&>", n) + "x")[:n],
				} {
					for page, parts := 1, 1; page <= parts; page++ {
						req := mcp.CallToolRequest{}
						req.Params.Name = "search_code"
						req.Params.Arguments = map[string]any{"pattern": pattern, "page": float64(page)}
						res, err := tool.Handler(context.Background(), req)
						if err != nil {
							t.Fatal(err)
						}
						if w := wireContentBytes(res); w > bound {
							t.Errorf("%s %d len %d page %d: %d bytes", m, bound, n, page, w)
						}
						var out map[string]any
						raw, _ := json.Marshal(res.StructuredContent)
						if res.StructuredContent == nil {
							text, _ := resultText(res)
							raw = []byte(text)
						}
						if err := json.Unmarshal(raw, &out); err != nil {
							t.Fatalf("%s len %d: %v", m, n, err)
						}
						if echo, ok := out["pattern"].(string); ok {
							if len(echo) > len(pattern) {
								t.Errorf("%s len %d: echo %d bytes, longer than the input", m, n, len(echo))
							}
							if n <= maxEchoBytes+len(echoCutMark) && echo != pattern {
								t.Errorf("%s len %d: a short input was cut: %q", m, n, echo)
							}
						}
						if hdr, ok := out["page"].(map[string]any); ok {
							parts = int(hdr["of"].(float64))
						}
					}
				}
			}
		}
	}
}

// TestSearchCode_SentIsMeasured: the answer search_code sizes is the
// answer sent: the echo wrap changes nothing after paging, so the one fit
// function measured the final bytes. The wrap cut the echo and added
// input_cut after the page had been sized.
func TestSearchCode_SentIsMeasured(t *testing.T) {
	store := newFixtureStore(t)
	old := searchCodePageBytes
	defer func() { searchCodePageBytes = old }()
	searchCodePageBytes = MinSearchCodePageBytes
	for _, m := range ContentModes {
		withContentMode(t, m)
		plain := server.NewMCPServer("plain", "0")
		RegisterSearchCode(plain, store)
		wrapped := server.NewMCPServer("wrapped", "0")
		RegisterAll(wrapped, store)
		for _, pattern := range []string{"func |" + strings.Repeat("q", 220), "err|" + strings.Repeat("<&>", 80) + "x"} {
			req := mcp.CallToolRequest{}
			req.Params.Name = "search_code"
			req.Params.Arguments = map[string]any{"pattern": pattern}
			ctx := context.WithValue(context.Background(), echoArgsKey{}, longArguments(req.GetArguments()))
			measured, err := plain.GetTool("search_code").Handler(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			sent, err := wrapped.GetTool("search_code").Handler(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(measured)
			b, _ := json.Marshal(sent)
			if string(a) != string(b) {
				t.Errorf("%s %.12q: the sent answer differs from the one sized (%d vs %d bytes)", m, pattern, len(b), len(a))
			}
		}
	}
}

// TestCutEchoes_RecordsTheLength: a cut echo keeps maxEchoBytes and the
// mark, and the answer says how long the argument was.
func TestCutEchoes_RecordsTheLength(t *testing.T) {
	withContentMode(t, ContentBoth)
	long := strings.Repeat("ab", 300)
	res := cutEchoes(textResult(map[string]any{"pattern": long}), map[string][]string{"pattern": {long}})
	var got map[string]any
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["pattern"] != long[:maxEchoBytes]+echoCutMark {
		t.Errorf("pattern = %q", got["pattern"])
	}
	if cut, _ := got[echoCutKey].(map[string]any); cut["pattern"] != float64(len(long)) {
		t.Errorf("%s = %v, want pattern %d", echoCutKey, got[echoCutKey], len(long))
	}
	if text, _ := resultText(res); text != string(b) {
		t.Errorf("text %q differs from structuredContent %s", text, b)
	}
}
