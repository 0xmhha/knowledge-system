package mcphandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"
)

// withContentMode runs the rest of the test under mode m.
func withContentMode(t *testing.T, m ContentMode) {
	t.Helper()
	old := contentMode
	SetContentMode(m)
	t.Cleanup(func() { SetContentMode(old) })
}

// resultText joins the text blocks of r.
func resultText(r *mcp.CallToolResult) (string, int) {
	text, n := "", 0
	for _, c := range r.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text += tc.Text
			n++
		}
	}
	return text, n
}

// TestShapeResult_EveryMode: under each mode a JSON answer and a prose
// answer carry the blocks the mode names, and the text of a JSON answer is
// the bytes structuredContent encodes to. The investigate tools sent text
// only while every other tool sent structuredContent with an empty text.
func TestShapeResult_EveryMode(t *testing.T) {
	payload := map[string]any{"q": "a<b", "n": []int{1, 2}}
	wantJSON, _ := json.Marshal(payload)
	for _, c := range []struct {
		mode             ContentMode
		text, structured bool
	}{
		{ContentBoth, true, true},
		{ContentText, true, false},
		{ContentStructured, false, true},
	} {
		withContentMode(t, c.mode)
		for _, r := range []struct {
			name string
			res  *mcp.CallToolResult
			text string
			sc   any
		}{
			{"json", textResult(payload), string(wantJSON), payload},
			{"prose", proseResult("card: p.F"), "card: p.F", map[string]string{proseKey: "card: p.F"}},
		} {
			text, blocks := resultText(r.res)
			if c.text != (blocks == 1) || (c.text && text != r.text) {
				t.Errorf("%s %s: %d text blocks %q, want text=%v %q", c.mode, r.name, blocks, text, c.text, r.text)
			}
			if c.structured != (r.res.StructuredContent != nil) {
				t.Errorf("%s %s: structuredContent %v, want present=%v", c.mode, r.name, r.res.StructuredContent, c.structured)
			}
			if c.structured {
				got, _ := json.Marshal(r.res.StructuredContent)
				want, _ := json.Marshal(r.sc)
				if string(got) != string(want) {
					t.Errorf("%s %s: structuredContent %s, want %s", c.mode, r.name, got, want)
				}
			}
			if r.res.Content == nil {
				t.Errorf("%s %s: content is nil; MCP requires a list", c.mode, r.name)
			}
		}
	}
}

// TestContentBytes_MatchTheWire: contentBytes is the size of the content
// blocks as encoded, for every mode.
func TestContentBytes_MatchTheWire(t *testing.T) {
	payload := map[string]any{"code": `if a < b { s := "x\\y" }`, "n": 3}
	b, _ := json.Marshal(payload)
	for _, m := range ContentModes {
		withContentMode(t, m)
		r := textResult(payload)
		wire := 0
		if r.StructuredContent != nil {
			sc, _ := json.Marshal(r.StructuredContent)
			wire += len(sc)
		}
		if text, n := resultText(r); n > 0 {
			enc, _ := json.Marshal(text)
			wire += len(enc) - 2 // the quotes around the string
		}
		if got := contentBytes(b); got != wire {
			t.Errorf("%s: contentBytes = %d, wire = %d", m, got, wire)
		}
	}
}

// TestSearchCode_PageBudgetHoldsInEveryMode: each part of a split
// search_code result keeps its content blocks within --page-bytes,
// counting every block the mode sends. Under both, a part was 85,303
// bytes on the wire with --page-bytes 40000.
func TestSearchCode_PageBudgetHoldsInEveryMode(t *testing.T) {
	store := newFixtureStore(t)
	old := searchCodePageBytes
	defer func() { searchCodePageBytes = old }()
	for _, m := range ContentModes {
		withContentMode(t, m)
		searchCodePageBytes = 6000
		s := server.NewMCPServer("test", "0")
		RegisterAll(s, store)
		req := mcp.CallToolRequest{}
		req.Params.Name = "search_code"
		req.Params.Arguments = map[string]any{"pattern": `.`}
		res, err := s.GetTool("search_code").Handler(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		wire := 0
		if res.StructuredContent != nil {
			sc, _ := json.Marshal(res.StructuredContent)
			wire += len(sc)
		}
		if text, n := resultText(res); n > 0 {
			enc, _ := json.Marshal(text)
			wire += len(enc) - 2
		}
		if wire > searchCodePageBytes {
			t.Errorf("%s: content is %d bytes, page budget %d", m, wire, searchCodePageBytes)
		}
	}
}

// wireContentBytes is the size of r's content blocks as encoded.
// The content list is counted whole, each block with its "type" key.
func wireContentBytes(r *mcp.CallToolResult) int {
	c, _ := json.Marshal(r.Content)
	wire := len(c)
	if r.StructuredContent != nil {
		sc, _ := json.Marshal(r.StructuredContent)
		wire += len(sc)
	}
	return wire
}

// TestSearchCode_EveryPageFits: walking every page of paged results, with
// and without ids, in every mode and at two bounds, each page fits the
// bound and the pages together hold every hit once. Paged ids= walks were
// up to 650 bytes over --page-bytes: the page header was not counted.
func TestSearchCode_EveryPageFits(t *testing.T) {
	store := newFixtureStore(t)
	withContentMode(t, ContentStructured)
	full := callTool(t, store, "search_code", map[string]any{"pattern": `.`})
	all := hitNames(full)
	ids := make([]any, 0, len(all))
	for _, n := range all {
		ids = append(ids, n)
	}
	old := searchCodePageBytes
	defer func() { searchCodePageBytes = old }()
	queries := []map[string]any{
		{"pattern": `.`, "ids": ids},
		{"pattern": `func`, "ids": ids},
		{"pattern": `return`, "ids": ids},
	}
	paged := 0
	for _, m := range ContentModes {
		withContentMode(t, m)
		for _, bound := range []int{4000, 9000} {
			searchCodePageBytes = bound
			for _, q := range queries {
				s := server.NewMCPServer("test", "0")
				RegisterAll(s, store)
				seen := map[string]int{}
				for page, parts := 1, 1; page <= parts; page++ {
					args := map[string]any{"page": float64(page)}
					for k, v := range q {
						args[k] = v
					}
					req := mcp.CallToolRequest{}
					req.Params.Name = "search_code"
					req.Params.Arguments = args
					res, err := s.GetTool("search_code").Handler(context.Background(), req)
					if err != nil {
						t.Fatal(err)
					}
					var out map[string]any
					raw, _ := json.Marshal(res.StructuredContent)
					if res.StructuredContent == nil {
						text, _ := resultText(res)
						raw = []byte(text)
					}
					if err := json.Unmarshal(raw, &out); err != nil {
						t.Fatal(err)
					}
					hits, _ := out["hits"].([]any)
					if w := wireContentBytes(res); w > bound && len(hits) > 1 {
						t.Errorf("%s %d %v page %d: %d bytes, bound %d", m, bound, q["pattern"], page, w, bound)
					}
					if hdr, ok := out["page"].(map[string]any); ok {
						parts = int(hdr["of"].(float64))
						paged++
					}
					for _, n := range hitNames(out) {
						seen[n]++
					}
				}
				for n, c := range seen {
					if c != 1 {
						t.Errorf("%s %d %v: %s in %d pages", m, bound, q["pattern"], n, c)
					}
				}
			}
		}
	}
	if paged == 0 {
		t.Error("no result was split; the test proves nothing")
	}
}

// TestSearchCode_FirstResponseFits: the first response of a result too
// large for one response (an index) fits the bound in every mode, at three
// bounds, even when not one declaration fits by name; its read_me then says
// that none is listed and how to narrow the search. An index of 0 rows
// with 623 per-file counts was 40,766 bytes against a bound of 40,000, and
// its read_me said every declaration was listed with its lines.
func TestSearchCode_FirstResponseFits(t *testing.T) {
	store := newFixtureStore(t)
	old := searchCodePageBytes
	defer func() { searchCodePageBytes = old }()
	noneListed := 0
	for _, m := range ContentModes {
		withContentMode(t, m)
		for _, bound := range []int{20000, 8000, 3000, MinSearchCodePageBytes} {
			searchCodePageBytes = bound
			// The longest pattern the floor holds for, matching every line.
			long := `.|` + strings.Repeat("z", maxFloorPatternBytes-2)
			for _, q := range []map[string]any{{"pattern": `.`}, {"pattern": "func ", "fixed": true}, {"pattern": `return`}, {"pattern": long}} {
				s := server.NewMCPServer("test", "0")
				RegisterAll(s, store)
				req := mcp.CallToolRequest{}
				req.Params.Name = "search_code"
				req.Params.Arguments = q
				res, err := s.GetTool("search_code").Handler(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				if w := wireContentBytes(res); w > bound {
					t.Errorf("%s %d %v: first response %d bytes", m, bound, q, w)
				}
				var out map[string]any
				raw, _ := json.Marshal(res.StructuredContent)
				if res.StructuredContent == nil {
					text, _ := resultText(res)
					raw = []byte(text)
				}
				if err := json.Unmarshal(raw, &out); err != nil {
					t.Fatal(err)
				}
				hdr, ok := out["index"].(map[string]any)
				if !ok {
					continue
				}
				hits, _ := out["hits"].([]any)
				readMe, _ := hdr["read_me"].(string)
				if len(hits) == 0 {
					noneListed++
					if !strings.Contains(readMe, "none is listed") || !strings.Contains(readMe, "path") {
						t.Errorf("%s %d %v: read_me of an empty index: %q", m, bound, q, readMe)
					}
				}
				if strings.Contains(readMe, "for the 0 files") {
					t.Errorf("%s %d %v: read_me describes an empty list as a list: %q", m, bound, q, readMe)
				}
				if strings.Contains(readMe, "all of them") && len(hits) != int(hdr["declarations"].(float64)) {
					t.Errorf("%s %d %v: read_me says all are listed, %d of %v are", m, bound, q, len(hits), hdr["declarations"])
				}
			}
		}
	}
	if noneListed == 0 {
		t.Error("no first response listed no declaration; the test does not reach that case")
	}
}

// TestRenderIndex_BoundsEveryList: every list of an index is cut by the
// same bound and counted, packages included, and the cut index fits. The
// packages list alone took 10,138 bytes of a 20,000-byte page and was
// never cut.
func TestRenderIndex_BoundsEveryList(t *testing.T) {
	old := searchCodePageBytes
	defer func() { searchCodePageBytes = old }()
	searchCodePageBytes = MinSearchCodePageBytes
	withContentMode(t, ContentBoth)
	var rows []map[string]any
	for i := range 300 {
		rows = append(rows, map[string]any{"qname": "p.F" + strings.Repeat("x", i%7), "file": "f" + strings.Repeat("y", i%40) + ".go"})
	}
	var pkgs []map[string]any
	for i := range 500 {
		pkgs = append(pkgs, map[string]any{"name": "pkg", "path": "example.com/" + strings.Repeat("p", i%30), "module": "example.com/m"})
	}
	out := map[string]any{"pattern": "x", "hits": rows, "packages": pkgs, "total_declarations": len(rows)}
	res, ok := indexSearchCode(out, nil, "search_code")
	if !ok {
		t.Fatal("no index for a result over the bound")
	}
	raw := res.(json.RawMessage)
	if !pageFits(raw) {
		t.Fatalf("index is %d bytes, bound %d", wireBytes(raw), searchCodePageBytes)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if n, _ := got["packages"+leftOutSuffix].(float64); n == 0 || int(n)+len(got["packages"].([]any)) != len(pkgs) {
		t.Errorf("packages %d listed, %v left out, of %d", len(got["packages"].([]any)), got["packages"+leftOutSuffix], len(pkgs))
	}
}

// TestValidateSearchCodePageBytes: a page size below the floor, zero and
// negatives included, is refused with the minimum named; the floor and
// larger sizes are accepted.
func TestValidateSearchCodePageBytes(t *testing.T) {
	// 0 and -1 started silently on the default.
	for _, n := range []int{MinSearchCodePageBytes - 1, 1, 0, -1} {
		if err := ValidateSearchCodePageBytes(n); err == nil || !strings.Contains(err.Error(), fmt.Sprint(MinSearchCodePageBytes)) {
			t.Errorf("%d: %v, want a refusal naming the minimum", n, err)
		}
	}
	for _, n := range []int{MinSearchCodePageBytes, 40000} {
		if err := ValidateSearchCodePageBytes(n); err != nil {
			t.Errorf("%d: %v", n, err)
		}
	}
}

// TestParseContentMode: the three names parse, anything else is an error.
func TestParseContentMode(t *testing.T) {
	for _, m := range ContentModes {
		if got, err := ParseContentMode(string(m)); err != nil || got != m {
			t.Errorf("ParseContentMode(%q) = %q, %v", m, got, err)
		}
	}
	if _, err := ParseContentMode("json"); err == nil {
		t.Error("ParseContentMode(json) should fail")
	}
}
