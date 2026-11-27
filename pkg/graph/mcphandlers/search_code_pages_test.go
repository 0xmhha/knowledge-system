package mcphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// callToolRaw returns the structured result as the bytes a client receives,
// so a test can check the order of the keys.
func callToolRaw(t *testing.T, store persist.StoreReader, name string, args map[string]any) []byte {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, store)
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := s.GetTool(name).Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return raw
}

// callToolText returns the text of a tool's answer, errors included.
func callToolText(t *testing.T, store persist.StoreReader, name string, args map[string]any) []byte {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, store)
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := s.GetTool(name).Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, _ := json.Marshal(res.Content)
	return raw
}

// TestSearchCode_NoCut: search_code returns every matched declaration and
// every matched line of each; the caller cannot cut the result with limit,
// which is not an argument of the tool (a call with it is an error).
func TestSearchCode_NoCut(t *testing.T) {
	// The budgets below are sized for one copy of the answer.
	withContentMode(t, ContentStructured)
	store := newFixtureStore(t)
	if raw := string(callToolText(t, store, "search_code", map[string]any{"pattern": `.`, "limit": 2})); !strings.Contains(raw, `unknown argument \"limit\"`) {
		t.Errorf("limit is not an argument of search_code: %s", raw)
	}
	out := callTool(t, store, "search_code", map[string]any{"pattern": `.`})
	if out["truncated"] != nil {
		t.Errorf("result must not be cut: truncated=%v", out["truncated"])
	}
	if got, total := len(hitNames(out)), int(out["total_declarations"].(float64)); got != total {
		t.Errorf("returned %d of %d declarations", got, total)
	}
	long := false
	for _, r := range out["hits"].([]any) {
		h := r.(map[string]any)
		if h["more_lines"] != nil {
			t.Errorf("%s hides matched lines: more_lines=%v", h["qname"], h["more_lines"])
		}
		if lines, _ := h["lines"].([]any); len(lines) > 3 {
			long = true
		}
	}
	if !long {
		t.Error("fixture has no declaration with more than three matched lines; the test proves nothing")
	}
}

// TestSearchCode_Index: a result larger than one response comes as an
// index: it starts with an index header, every matched declaration is a row
// (or counted in not_listed), rows carry matched line numbers and uses but
// no code, and ids returns the chosen declarations in full.
func TestSearchCode_Index(t *testing.T) {
	// The budgets below are sized for one copy of the answer.
	withContentMode(t, ContentStructured)
	store := newFixtureStore(t)
	full := callTool(t, store, "search_code", map[string]any{"pattern": `.`})
	want := hitNames(full)
	fullRaw := callToolRaw(t, store, "search_code", map[string]any{"pattern": `.`})

	old := searchCodePageBytes
	defer func() { searchCodePageBytes = old }()
	// One byte short of the full result: the index (no code text) fits.
	searchCodePageBytes = len(fullRaw) - 1

	raw := callToolRaw(t, store, "search_code", map[string]any{"pattern": `.`})
	if !bytes.HasPrefix(raw, []byte(`{"index":`)) {
		t.Fatalf("an index must start with its header: %.80s", raw)
	}
	if len(raw) > searchCodePageBytes {
		t.Errorf("index is %d bytes, budget %d", len(raw), searchCodePageBytes)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	hdr := out["index"].(map[string]any)
	if g, _ := hdr["read_me"].(string); !strings.Contains(g, "ids=") || !strings.Contains(g, "search_code") {
		t.Errorf("header must say how to get the code: %q", g)
	}
	got := hitNames(out)
	if hdr["not_listed"] == nil && strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("index rows = %v\nwant %v", got, want)
	}
	withUses := false
	for _, r := range out["hits"].([]any) {
		h := r.(map[string]any)
		if h["lines"] != nil || h["signature"] != nil {
			t.Fatalf("index row %s carries code text or signature: %v", h["qname"], h)
		}
		withUses = withUses || h["uses"] != nil
	}
	// with_lines and names_only add up to the rows listed; a row with
	// matched lines is never counted as names_only.
	carry := 0
	for _, r := range out["hits"].([]any) {
		if r.(map[string]any)["at"] != nil {
			carry++
		}
	}
	namesOnly, _ := hdr["names_only"].(float64)
	if with := int(hdr["with_lines"].(float64)); with+int(namesOnly) != len(out["hits"].([]any)) || carry > with {
		t.Errorf("header with_lines = %v, names_only = %v, rows = %d, rows carrying lines = %d",
			hdr["with_lines"], hdr["names_only"], len(out["hits"].([]any)), carry)
	}
	if !withUses {
		t.Errorf("no index row carries uses; the rows lost why they matter (header %v, full %d bytes)", hdr, len(fullRaw))
	}

	// Details for two chosen declarations.
	rows := out["hits"].([]any)
	a, b := rows[0].(map[string]any), rows[len(rows)-1].(map[string]any)
	det := callTool(t, store, "search_code", map[string]any{"pattern": `.`, "ids": []any{a["qname"], b["qname"], "no.such.Thing"}})
	if names := hitNames(det); len(names) != 2 || names[0] != a["qname"] || names[1] != b["qname"] {
		t.Errorf("ids returned %v, want %v and %v", names, a["qname"], b["qname"])
	}
	for _, r := range det["hits"].([]any) {
		h := r.(map[string]any)
		for _, l := range asSlice(h["lines"]) {
			if l.(map[string]any)["text"] == nil {
				t.Errorf("detail of %s has no code text", h["qname"])
			}
		}
	}
	if nm := asSlice(det["ids_not_matched"]); len(nm) != 1 || nm[0] != "no.such.Thing" {
		t.Errorf("ids_not_matched = %v", det["ids_not_matched"])
	}

	// A budget too small for every name: the rest is counted per file.
	searchCodePageBytes = 1700
	raw = callToolRaw(t, store, "search_code", map[string]any{"pattern": `.`})
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(raw) > searchCodePageBytes {
		t.Errorf("index with not_listed is %d bytes, budget %d", len(raw), searchCodePageBytes)
	}
	hdr = out["index"].(map[string]any)
	listed := len(asSlice(out["hits"]))
	counted := 0
	for _, v := range out["not_listed"].(map[string]any) {
		counted += int(v.(float64))
	}
	if listed+counted != len(want) || int(hdr["not_listed"].(float64)) != counted {
		t.Errorf("listed %d + counted %d != %d declarations (header %v)", listed, counted, len(want), hdr)
	}
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// searchCodeTexts returns search_code's description and its parameters'
// descriptions, as a server registered now declares them.
func searchCodeTexts(t *testing.T, store persist.StoreReader) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, store)
	b, err := json.Marshal(s.GetTool("search_code").Tool)
	if err != nil {
		t.Fatal(err)
	}
	var tool struct {
		Description string `json:"description"`
		Schema      struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(b, &tool); err != nil {
		t.Fatal(err)
	}
	text := tool.Description
	for _, p := range tool.Schema.Properties {
		text += "\n" + p.Description
	}
	return text
}

// TestInstructionsExplainSearchCodeResult: search_code's description and
// parameter descriptions, which the client delivers whole (the server guide
// is cut at maxGuideChars), explain every key a search_code result can carry
// and tell the model that a result with a page key is incomplete until
// every part is read. A sentence inside the result alone was not followed in
// agent runs.
func TestInstructionsExplainSearchCodeResult(t *testing.T) {
	text := searchCodeTexts(t, newFixtureStore(t))
	for _, key := range []string{"pattern", "total_declarations", "total_files", "total_matches", "note", "without_filters", "error",
		"hits", "qname", "type", "file", "start_line", "end_line", "signature", "canonical_id", "test", "name_match",
		"matched", "matches", "lines", "line", "text", "uses", "callers", "callees", "references",
		"page", "number", "of", "declarations", "next", "get_next",
		"index", "with_lines", "names_only", "not_listed", "read_me", "ids_not_matched", "span", "at",
		"file", "bytes", "lines",
		"packages", "name", "path", "module", "version"} {
		if !strings.Contains(text, "`"+key+"`") {
			t.Errorf("guide does not explain search_code key %q", key)
		}
	}
	for _, must := range []string{"ids=", "same pattern", "whole index"} {
		if !strings.Contains(text, must) {
			t.Errorf("guide does not state the parts rule (%q missing)", must)
		}
	}
}

// TestSearchCodeDescriptionOrder: the tool description states the order the
// code uses: production before tests first.
func TestSearchCodeDescriptionOrder(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, newFixtureStore(t))
	d := s.GetTool("search_code").Tool.Description
	if strings.Contains(d, "Production code comes before tests") || strings.Contains(d, "distinct texts, a match") {
		t.Errorf("description states the old order: %s", d)
	}
	if !strings.Contains(d, "Order: a match in the name first, then more matches, then wider use in the repository (PageRank)") {
		t.Errorf("description does not state the order the code uses: %s", d)
	}
}

// TestSetSearchCodePageBytes: a positive size replaces the part size; zero or
// less keeps it, so an unset flag does not turn paging off.
func TestSetSearchCodePageBytes(t *testing.T) {
	old := searchCodePageBytes
	defer func() { searchCodePageBytes = old }()
	SetSearchCodePageBytes(0)
	if searchCodePageBytes != old {
		t.Fatalf("0 changed the size to %d", searchCodePageBytes)
	}
	SetSearchCodePageBytes(-5)
	if searchCodePageBytes != old {
		t.Fatalf("-5 changed the size to %d", searchCodePageBytes)
	}
	SetSearchCodePageBytes(9000)
	if searchCodePageBytes != 9000 {
		t.Fatalf("size = %d, want 9000", searchCodePageBytes)
	}
}

// TestSearchCode_ExternalUses: a line that calls code outside the build names
// the call (schema 1.36 External nodes), and the result lists the package of
// each such call once: coll1.clock.arm calls time.AfterFunc on line 13.
func TestSearchCode_ExternalUses(t *testing.T) {
	store := newFixtureStore(t)
	out := callTool(t, store, "search_code", map[string]any{"pattern": `AfterFunc`})
	var uses []any
	for _, r := range out["hits"].([]any) {
		h := r.(map[string]any)
		if h["qname"] != "coll1.clock.arm" {
			continue
		}
		for _, l := range h["lines"].([]any) {
			if lm := l.(map[string]any); lm["line"].(float64) == 13 {
				uses, _ = lm["uses"].([]any)
			}
		}
	}
	found := false
	for _, u := range uses {
		found = found || u == "calls time.AfterFunc"
	}
	if !found {
		t.Fatalf("line 13 of coll1.clock.arm does not name the call: uses=%v\nresult=%v", uses, out)
	}
	pk, _ := out["packages"].([]any)
	if len(pk) != 1 {
		t.Fatalf("packages = %v, want only time", pk)
	}
	p := pk[0].(map[string]any)
	if p["name"] != "time" || p["path"] != "time" || p["module"] != "std" {
		t.Errorf("time package = %v", p)
	}
}

// TestSearchCode_File: with a result folder set, a result larger than one
// response is saved as an indented JSON file and the response holds only
// where it is: no hit, so a part cannot be taken for the whole. The file
// holds the full result, line by line.
func TestSearchCode_File(t *testing.T) {
	// The budgets below are sized for one copy of the answer.
	withContentMode(t, ContentStructured)
	store := newFixtureStore(t)
	full := callTool(t, store, "search_code", map[string]any{"pattern": `.`})
	fullRaw := callToolRaw(t, store, "search_code", map[string]any{"pattern": `.`})

	oldB, oldD := searchCodePageBytes, searchCodeDir
	defer func() { searchCodePageBytes, searchCodeDir = oldB, oldD }()
	searchCodePageBytes = len(fullRaw) - 1
	searchCodeDir = t.TempDir()

	out := callTool(t, store, "search_code", map[string]any{"pattern": `.`})
	if out["hits"] != nil || out["index"] != nil || out["page"] != nil {
		t.Fatalf("a saved result must not send any part of itself: %v", out)
	}
	f := out["file"].(map[string]any)
	path := f["path"].(string)
	if !strings.HasPrefix(path, searchCodeDir) {
		t.Errorf("file %s is not in the result folder %s", path, searchCodeDir)
	}
	if r, _ := f["read_me"].(string); !strings.Contains(r, path) || !strings.Contains(r, "offset and limit") {
		t.Errorf("read_me must name the file and how to read a long one: %q", r)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(body, []byte("\n"))+1 != int(f["lines"].(float64)) || len(body) != int(f["bytes"].(float64)) {
		t.Errorf("file size %d bytes, %d lines; response says %v", len(body), bytes.Count(body, []byte("\n"))+1, f)
	}
	var saved map[string]any
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if strings.Join(hitNames(saved), ",") != strings.Join(hitNames(full), ",") {
		t.Error("the file does not hold the full result")
	}
	for i, l := range bytes.Split(body, []byte("\n")) {
		if len(l) > 2000 {
			t.Fatalf("line %d is %d bytes; file readers cut lines over 2000", i+1, len(l))
		}
	}

	// Without a folder the index answers, as before.
	searchCodeDir = ""
	if idx := callTool(t, store, "search_code", map[string]any{"pattern": `.`}); idx["index"] == nil {
		t.Errorf("no folder: want an index, got %v", idx)
	}
}

// TestSearchCodeDeclaresMaxResultChars: with a size set, the search_code
// tool carries _meta["anthropic/maxResultSizeChars"], which Claude Code reads
// to give a large result to the model instead of saving it to a file; with
// none set the tool carries no _meta.
func TestSearchCodeDeclaresMaxResultChars(t *testing.T) {
	old := searchCodeMaxResultChars
	defer func() { searchCodeMaxResultChars = old }()
	for _, n := range []int{0, 400000} {
		SetSearchCodeMaxResultChars(n)
		s := server.NewMCPServer("test", "0")
		RegisterAll(s, newFixtureStore(t))
		raw, err := json.Marshal(s.GetTool("search_code").Tool)
		if err != nil {
			t.Fatal(err)
		}
		has := bytes.Contains(raw, []byte(`"_meta":{"anthropic/maxResultSizeChars":400000}`))
		if (n > 0) != has {
			t.Errorf("size %d: tool JSON %s", n, raw[:min(len(raw), 200)])
		}
	}
}

// TestSearchCodeTiesNotByName: hits equal on every ranking key are ties.
// The name must not order them (it put names starting with "a" first), and
// the order of ties must be the same on every call.
func TestSearchCodeTiesNotByName(t *testing.T) {
	b := codeDecl{node: types.Node{QualifiedName: "pkg.Beta", PageRank: 0.5}}
	a := codeDecl{node: types.Node{QualifiedName: "pkg.Alpha", PageRank: 0.5}}
	hb := &codeHit{terms: map[string]bool{"x": true}, matches: 1}
	ha := &codeHit{terms: map[string]bool{"x": true}, matches: 1}
	if rankBefore(a, b, ha, hb) || rankBefore(b, a, hb, ha) {
		t.Error("equal hits are ordered by name")
	}
	if !rankBefore(codeDecl{node: types.Node{PageRank: 0.6}}, b, ha, hb) {
		t.Error("PageRank no longer orders hits that tie before it")
	}

	store := newFixtureStore(t)
	first := hitNames(callTool(t, store, "search_code", map[string]any{"pattern": `.`}))
	for i := 0; i < 5; i++ {
		if got := hitNames(callTool(t, store, "search_code", map[string]any{"pattern": `.`})); strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("call %d returned another order:\n%v\nfirst:\n%v", i+2, got, first)
		}
	}
}

// TestInvestigateTiesNotByName: investigate's last tie-break is production
// before tests, then the node id, not the qualified name.
func TestInvestigateTiesNotByName(t *testing.T) {
	x := &invIndex{
		nodes: map[string]types.Node{"id1": {QualifiedName: "pkg.Zeta"}, "id2": {QualifiedName: "pkg.Alpha"}, "id0": {QualifiedName: "pkg.Mid"}},
		test:  map[string]bool{"id0": true},
	}
	if !x.less("id1", "id2") || x.less("id2", "id1") {
		t.Error("ties are ordered by name, not by id")
	}
	if !x.less("id2", "id0") {
		t.Error("production no longer comes before tests")
	}
}

// TestSearchCodeSplitsTests: search_code returns production code only, with
// no list of tests per hit (left out to keep results small); search_test_code
// returns test code only. Fixture: coll1/timers_test.go.
func TestSearchCodeSplitsTests(t *testing.T) {
	store := newFixtureStore(t)
	prod := callTool(t, store, "search_code", map[string]any{"pattern": `arm|fire`})
	if len(hitNames(prod)) == 0 {
		t.Fatal("search_code found nothing")
	}
	for _, r := range prod["hits"].([]any) {
		h := r.(map[string]any)
		if h["test"] == true || strings.HasSuffix(h["file"].(string), "_test.go") {
			t.Errorf("search_code returned test code: %v", h["qname"])
		}
		if _, ok := h["tests"]; ok {
			t.Errorf("search_code hit %v carries a tests list", h["qname"])
		}
	}
	tests := callTool(t, store, "search_test_code", map[string]any{"pattern": `arm`})
	if len(hitNames(tests)) == 0 {
		t.Fatal("search_test_code found nothing")
	}
	for _, r := range tests["hits"].([]any) {
		h := r.(map[string]any)
		if !strings.HasSuffix(h["file"].(string), "_test.go") && !strings.Contains(h["file"].(string), "/testdata/") {
			t.Errorf("search_test_code returned production code: %v %v", h["qname"], h["file"])
		}
	}
}

// TestSearchCodeOrderKeys: the order is name match, then more matches, then
// PageRank; distinct matched strings no longer order hits.
func TestSearchCodeOrderKeys(t *testing.T) {
	d := codeDecl{node: types.Node{PageRank: 0.1}}
	many := &codeHit{terms: map[string]bool{"a": true, "b": true, "c": true}, matches: 3}
	often := &codeHit{terms: map[string]bool{"a": true}, matches: 9}
	if !rankBefore(d, d, often, many) || rankBefore(d, d, many, often) {
		t.Error("more distinct strings still beat more matches")
	}
	name := &codeHit{terms: map[string]bool{"a": true}, matches: 1, inName: true}
	if !rankBefore(d, d, name, often) {
		t.Error("a name match no longer comes first")
	}
}

// TestSearchCodeNameAsBoth: with the option on, in=name also returns body
// matches, name matches first; off, in=name returns name matches only.
func TestSearchCodeNameAsBoth(t *testing.T) {
	store := newFixtureStore(t)
	defer SetSearchCodeOptions(false, false)
	SetSearchCodeOptions(false, false)
	name := callTool(t, store, "search_code", map[string]any{"pattern": `fire`, "in": "name"})
	SetSearchCodeOptions(true, false)
	both := callTool(t, store, "search_code", map[string]any{"pattern": `fire`, "in": "name"})
	if len(hitNames(both)) <= len(hitNames(name)) {
		t.Fatalf("in=name as both returned %v, name only %v", hitNames(both), hitNames(name))
	}
	seenBody := false
	for _, r := range both["hits"].([]any) {
		h := r.(map[string]any)
		if h["name_match"] == nil {
			seenBody = true
		} else if seenBody {
			t.Errorf("name match %v after a body match", h["qname"])
		}
	}
	if !strings.Contains(searchCodeTexts(t, store), "in=name also returns body matches") {
		t.Error("search_code does not say in=name returns body matches")
	}
}

// TestSearchCodeReasons: with the option on, reasons add related rows: graph
// neighbours of the hits that hold a reason word, after the hits, with the
// hits unchanged. Fixture: clock.arm calls clock.fire inside a closure and
// clock.disarm; searching "AfterFunc" hits arm, and the reason "fire" reaches
// clock.fire through arm.
func TestSearchCodeReasons(t *testing.T) {
	store := newFixtureStore(t)
	defer SetSearchCodeOptions(false, false)
	SetSearchCodeOptions(false, false)
	if raw := string(callToolText(t, store, "search_code", map[string]any{"pattern": `AfterFunc`, "reasons": []any{"fire"}})); !strings.Contains(raw, `unknown argument \"reasons\"`) {
		t.Fatalf("reasons accepted while the option is off: %s", raw)
	}
	plain := callTool(t, store, "search_code", map[string]any{"pattern": `AfterFunc`})
	SetSearchCodeOptions(false, true)
	out := callTool(t, store, "search_code", map[string]any{"pattern": `AfterFunc`, "reasons": []any{"fire", "x"}})
	if strings.Join(hitNames(out), ",") != strings.Join(hitNames(plain), ",") {
		t.Errorf("hits changed: %v vs %v", hitNames(out), hitNames(plain))
	}
	var found map[string]any
	for _, r := range asSlice(out["related"]) {
		m := r.(map[string]any)
		if m["qname"] == "coll1.clock.fire" {
			found = m
		}
		for _, h := range hitNames(out) {
			if m["qname"] == h {
				t.Errorf("related repeats hit %s", h)
			}
		}
	}
	if found == nil {
		t.Fatalf("clock.fire not related: %v", out["related"])
	}
	if rm := asSlice(found["reason_matched"]); len(rm) != 1 || rm[0] != "fire" {
		t.Errorf("reason_matched = %v (short words must be dropped)", found["reason_matched"])
	}
	if v := asSlice(found["via"]); len(v) == 0 || v[0].(map[string]any)["hit"] != "coll1.clock.arm" {
		t.Errorf("via = %v, want coll1.clock.arm", found["via"])
	}
	s := server.NewMCPServer("t", "0")
	RegisterAll(s, store)
	if _, ok := s.GetTool("search_code").Tool.InputSchema.Properties["reasons"]; !ok {
		t.Error("reasons argument missing with the option on")
	}
	texts := searchCodeTexts(t, store)
	for _, key := range []string{"related", "reason_matched", "via"} {
		if !strings.Contains(texts, "`"+key+"`") {
			t.Errorf("search_code does not explain %q", key)
		}
	}
}

// TestReasonWordForms: a reason of several words matches its identifier
// forms; a one-word reason matches as a substring, ignoring case.
func TestReasonWordForms(t *testing.T) {
	cases := []struct {
		text, reason string
		want         bool
	}{
		{"func (c *core) sendRoundChange()", "round change", true},
		{"var round_change_timer", "round change", true},
		{"// start a round change", "round-change", true},
		{"func roundTimer()", "round change", false},
		{"func expireTimer()", "expire", true},
	}
	for _, c := range cases {
		got := len(wordsIn(c.text, reasonWords([]string{c.reason}))) == 1
		if got != c.want {
			t.Errorf("wordsIn(%q, %q) = %v, want %v", c.text, c.reason, got, c.want)
		}
	}
}
