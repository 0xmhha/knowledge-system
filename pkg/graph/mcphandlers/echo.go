package mcphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"
)

// maxEchoBytes is how much of a caller's argument (a pattern, qname, query
// or path) an answer repeats. An argument longer than maxEchoBytes plus
// echoCutMark is repeated cut to maxEchoBytes and the mark, and the answer
// says under echoCutKey how long it was; a shorter one is repeated whole,
// so a cut never makes an echo longer.
const maxEchoBytes = 200

// echoCutKey is the answer key that gives, by argument name, the length in
// bytes of each argument the answer repeats cut.
const echoCutKey = "input_cut"

// echoCutMark ends a cut repetition.
const echoCutMark = "…"

// boundEchoes wraps every tool of s so that its answers repeat no caller
// argument longer than maxEchoBytes: the one rule for every tool, normal
// and error answers alike.
func boundEchoes(s *server.MCPServer) {
	for _, t := range s.ListTools() {
		if t == nil || t.Handler == nil {
			continue
		}
		s.AddTool(t.Tool, echoBounded(t.Handler))
	}
}

// echoBounded runs h and cuts, in every block of its answer, each
// occurrence of an argument longer than maxEchoBytes.
func echoBounded(h server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		long := longArguments(req.GetArguments())
		if len(long) == 0 {
			return h(ctx, req)
		}
		// A handler that sizes its answer (search_code) cuts the echoes
		// itself first, through the context (cutAnswerEchoes); for the
		// others the answer is cut here, after it is built.
		res, err := h(context.WithValue(ctx, echoArgsKey{}, long), req)
		if err != nil {
			// A handler error reaches the client as the JSON-RPC error
			// message, which repeats input too.
			return nil, errors.New(cutText(err.Error(), long))
		}
		if res == nil {
			return res, nil
		}
		return cutEchoes(res, long), nil
	}
}

// longArguments returns, by argument name, the string arguments (and the
// strings of list arguments) longer than maxEchoBytes, the longest first
// per name so a shorter one never cuts inside a longer one.
func longArguments(args map[string]any) map[string][]string {
	out := map[string][]string{}
	add := func(name string, v any) {
		if s, ok := v.(string); ok && len(s) > maxEchoBytes+len(echoCutMark) {
			out[name] = append(out[name], s)
		}
	}
	for name, v := range args {
		if list, ok := v.([]any); ok {
			for _, e := range list {
				add(name, e)
			}
			continue
		}
		add(name, v)
	}
	for _, l := range out {
		slices.SortFunc(l, func(a, b string) int { return len(b) - len(a) })
	}
	return out
}

// cutEcho is the repetition of s an answer keeps.
func cutEcho(s string) string {
	n := maxEchoBytes
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + echoCutMark
}

// boundedEcho is the repetition of s an answer keeps when s is derived
// from an argument (a path with each "\" turned into "/", say), so the cut of
// the argument itself does not find it: s cut as cutEcho cuts it, when long.
func boundedEcho(s string) string {
	if len(s) <= maxEchoBytes+len(echoCutMark) {
		return s
	}
	return cutEcho(s)
}

// cutText replaces every occurrence of a long argument in plain text, as
// written or Go-quoted.
func cutText(text string, long map[string][]string) string {
	for _, l := range long {
		for _, s := range l {
			for _, form := range []func(string) string{identity, goQuoted} {
				text = strings.ReplaceAll(text, form(s), form(cutEcho(s)))
			}
		}
	}
	return text
}

// echoArgsKey is the context key of the long arguments of a call.
type echoArgsKey struct{}

// cutAnswerEchoes cuts, in the top-level strings and string lists of out,
// every long argument of the call in ctx, and records the original lengths
// under echoCutKey: for an answer that is sized (paged, indexed) after it
// is built, so the size measured is the size sent. It returns out.
func cutAnswerEchoes(ctx context.Context, out map[string]any) map[string]any {
	long, _ := ctx.Value(echoArgsKey{}).(map[string][]string)
	if len(long) == 0 {
		return out
	}
	for k, v := range out {
		switch x := v.(type) {
		case string:
			out[k] = cutText(x, long)
		case []string:
			cut := make([]string, len(x))
			for i, s := range x {
				cut[i] = cutText(s, long)
			}
			out[k] = cut
		}
	}
	out[echoCutKey] = echoLengths(long)
	return out
}

// echoLengths is, by argument name, the longest long argument's length.
func echoLengths(long map[string][]string) map[string]int {
	lengths := map[string]int{}
	for name, l := range long {
		for _, s := range l {
			lengths[name] = max(lengths[name], len(s))
		}
	}
	return lengths
}

// cutEchoes replaces every occurrence of a long argument in res, in its
// text blocks as written and as JSON-encoded and in its structuredContent
// as encoded, and records the original lengths under echoCutKey. An
// answer with nothing to cut (cut already, by cutAnswerEchoes) is returned
// as it is.
func cutEchoes(res *mcp.CallToolResult, long map[string][]string) *mcp.CallToolResult {
	lengths := echoLengths(long)
	type pair struct{ from, to []byte }
	var raw, enc []pair
	for _, name := range slices.Sorted(maps.Keys(long)) {
		for _, s := range long[name] {
			// An argument is repeated as written, or Go-quoted (%q) in a
			// message; each form also appears JSON-encoded.
			for _, form := range []func(string) string{identity, goQuoted} {
				from, to := form(s), form(cutEcho(s))
				raw = append(raw, pair{[]byte(from), []byte(to)})
				enc = append(enc, pair{jsonString(from), jsonString(to)})
			}
		}
	}
	replaceAll := func(b []byte, ps []pair) []byte {
		for _, p := range ps {
			b = bytes.ReplaceAll(b, p.from, p.to)
		}
		return b
	}
	cutJSON, _ := json.Marshal(lengths)
	found := false
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			for _, p := range append(enc, raw...) {
				found = found || bytes.Contains([]byte(tc.Text), p.from)
			}
		}
	}
	if b, err := json.Marshal(res.StructuredContent); err == nil && res.StructuredContent != nil {
		for _, p := range enc {
			found = found || bytes.Contains(b, p.from)
		}
	}
	if !found {
		return res
	}
	out := &mcp.CallToolResult{Result: res.Result, IsError: res.IsError, Content: make([]mcp.Content, 0, len(res.Content))}
	for _, c := range res.Content {
		tc, ok := c.(mcp.TextContent)
		if !ok {
			out.Content = append(out.Content, c)
			continue
		}
		text := replaceAll(replaceAll([]byte(tc.Text), enc), raw)
		if json.Valid(text) {
			text = withKey(text, echoCutKey, cutJSON)
		} else {
			text = append(text, fmt.Sprintf("\n(%s: %s)", echoCutKey, cutJSON)...)
		}
		tc.Text = string(text)
		out.Content = append(out.Content, tc)
	}
	if res.StructuredContent != nil {
		if b, err := json.Marshal(res.StructuredContent); err == nil {
			out.StructuredContent = json.RawMessage(withKey(replaceAll(b, enc), echoCutKey, cutJSON))
		} else {
			out.StructuredContent = res.StructuredContent
		}
	}
	return out
}

func identity(s string) string { return s }

// goQuoted is s as %q writes it, without the quotes.
func goQuoted(s string) string {
	q := strconv.Quote(s)
	return q[1 : len(q)-1]
}

// jsonString is s as JSON encodes it inside a string, without the quotes.
func jsonString(s string) []byte {
	b, _ := json.Marshal(s)
	return b[1 : len(b)-1]
}

// withKey adds "key": value at the end of the JSON object b; b is returned
// unchanged when it is not an object or already holds key at its top level
// (cutAnswerEchoes put it there before the answer was sized).
func withKey(b []byte, key string, value []byte) []byte {
	var top map[string]json.RawMessage
	if json.Unmarshal(b, &top) == nil {
		if _, ok := top[key]; ok {
			return b
		}
	}
	t := bytes.TrimRight(b, " \n")
	if len(t) < 2 || t[0] != '{' || t[len(t)-1] != '}' {
		return b
	}
	body := t[:len(t)-1]
	sep := ","
	if strings.TrimSpace(string(body[1:])) == "" {
		sep = ""
	}
	out := append([]byte{}, body...)
	out = append(out, fmt.Sprintf(`%s%q:`, sep, key)...)
	out = append(out, value...)
	return append(out, '}')
}
