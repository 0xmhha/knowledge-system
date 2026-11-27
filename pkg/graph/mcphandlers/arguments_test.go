package mcphandlers

import (
	"context"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"
)

// TestArgumentsChecked: every tool answers a missing required argument and
// an unknown argument with an error that names it and lists the arguments
// the tool takes, instead of an empty result.
func TestArgumentsChecked(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, newFixtureStore(t))
	for name, st := range s.ListTools() {
		props, required := declaredArguments(st.Tool)
		if len(props) == 0 {
			t.Errorf("%s: no declared arguments read", name)
			continue
		}
		call := func(args map[string]any) *mcp.CallToolResult {
			req := mcp.CallToolRequest{}
			req.Params.Name = name
			req.Params.Arguments = args
			res, err := st.Handler(context.Background(), req)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			return res
		}
		text := func(r *mcp.CallToolResult) string {
			var b strings.Builder
			for _, c := range r.Content {
				if tc, ok := c.(mcp.TextContent); ok {
					b.WriteString(tc.Text)
				}
			}
			return b.String()
		}
		args := map[string]any{}
		for _, r := range required {
			args[r] = "x"
		}
		args["zz_not_an_argument"] = 1
		if r := call(args); !r.IsError || !strings.Contains(text(r), "zz_not_an_argument") || !strings.Contains(text(r), props[0]) {
			t.Errorf("%s: an unknown argument gives %v %q", name, r.IsError, text(r))
		}
		if len(required) > 0 {
			if r := call(map[string]any{}); !r.IsError || !strings.Contains(text(r), required[0]) {
				t.Errorf("%s: a missing %s gives %v %q", name, required[0], r.IsError, text(r))
			}
		}
		// A tool that takes one of several seeds says so when given none.
		if g, ok := seedGroups[name]; ok {
			if r := call(map[string]any{}); !r.IsError || !strings.Contains(text(r), g.args[0]) {
				t.Errorf("%s: no seed gives %v %q", name, r.IsError, text(r))
			}
			if !strings.Contains(st.Tool.Description, g.rule()) {
				t.Errorf("%s: the description does not say %q", name, g.rule())
			}
		}
	}
}
