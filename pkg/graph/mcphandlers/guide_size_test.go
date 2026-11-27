package mcphandlers

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

// TestGuideFitsTheClient: the server guide stays within maxGuideChars, the
// part a client was measured to deliver, and every tool's description and
// parameter descriptions within maxToolTextChars. It logs the size of the
// guide and of every tool definition, which every session receives.
func TestGuideFitsTheClient(t *testing.T) {
	g := Instructions()
	if len(g) > maxGuideChars {
		t.Errorf("guide is %d characters, more than the %d a client delivers", len(g), maxGuideChars)
	}
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, newFixtureStore(t))
	names := make([]string, 0)
	tools := s.ListTools()
	for n := range tools {
		names = append(names, n)
	}
	sort.Strings(names)
	total := 0
	for _, n := range names {
		tool := tools[n].Tool
		b, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		total += len(b)
		t.Logf("%s: description %d, definition %d", n, len(tool.Description), len(b))
		if len(tool.Description) > maxToolTextChars {
			t.Errorf("%s: description is %d characters, more than %d", n, len(tool.Description), maxToolTextChars)
		}
		var def struct {
			Schema struct {
				Properties map[string]struct {
					Description string `json:"description"`
				} `json:"properties"`
			} `json:"inputSchema"`
		}
		if err := json.Unmarshal(b, &def); err != nil {
			t.Fatal(err)
		}
		for p, d := range def.Schema.Properties {
			if len(d.Description) > maxToolTextChars {
				t.Errorf("%s.%s: description is %d characters, more than %d", n, p, len(d.Description), maxToolTextChars)
			}
		}
	}
	t.Logf("guide %d, tool definitions %d, together %d", len(g), total, len(g)+total)
}

// TestEveryArgumentDescribed: every argument of every tool has a
// description: a model reads the form, the default and the limit there.
func TestEveryArgumentDescribed(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, newFixtureStore(t))
	for n, st := range s.ListTools() {
		b, err := json.Marshal(st.Tool)
		if err != nil {
			t.Fatal(err)
		}
		var def struct {
			Schema struct {
				Properties map[string]map[string]any `json:"properties"`
			} `json:"inputSchema"`
		}
		if err := json.Unmarshal(b, &def); err != nil {
			t.Fatal(err)
		}
		for p, v := range def.Schema.Properties {
			if d, _ := v["description"].(string); d == "" {
				t.Errorf("%s.%s has no description", n, p)
			}
		}
	}
}
