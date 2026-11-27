package mcphandlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	mcp "github.com/mark3labs/mcp-go/mcp"
)

// ContentMode says which blocks a tool result carries: a text block, a
// structuredContent object, or both. Every tool result is shaped by
// shapeResult under the one mode the server runs with.
type ContentMode string

// The content modes.
const (
	// ContentBoth sends the answer as a text block and as
	// structuredContent: the MCP specification says a tool that returns
	// structured content SHOULD also return functionally equivalent
	// unstructured content, and a client that reads only text content
	// otherwise sees nothing. The response then carries the answer twice.
	ContentBoth ContentMode = "both"
	// ContentText sends only the text block.
	ContentText ContentMode = "text"
	// ContentStructured sends only structuredContent and an empty content
	// list, the shape the server sent before the mode existed.
	ContentStructured ContentMode = "structured"
)

// DefaultContentMode is the mode a server runs with unless told otherwise.
const DefaultContentMode = ContentBoth

// ContentModes lists the modes in the order the help gives them.
var ContentModes = []ContentMode{ContentBoth, ContentText, ContentStructured}

// proseKey is the structuredContent key that holds a prose answer (the
// investigate tools answer in text): structuredContent must be an object.
const proseKey = "text"

var contentMode = DefaultContentMode

// ParseContentMode returns the mode named s.
func ParseContentMode(s string) (ContentMode, error) {
	for _, m := range ContentModes {
		if string(m) == s {
			return m, nil
		}
	}
	names := make([]string, len(ContentModes))
	for i, m := range ContentModes {
		names[i] = string(m)
	}
	return "", fmt.Errorf("unknown content mode %q (want %s)", s, strings.Join(names, ", "))
}

// SetContentMode sets the mode every tool result is shaped with. Call it
// before serving.
func SetContentMode(m ContentMode) { contentMode = m }

// textResult wraps a JSON payload. All handlers route their JSON payload
// through here so the response shape stays uniform across the tool set.
func textResult(payload any) *mcp.CallToolResult {
	return shapeResult(payload, "", false)
}

// proseResult wraps a prose answer (investigate, investigate_open). Under
// ContentStructured and ContentBoth its structuredContent is
// {"text": <the prose>}.
func proseResult(text string) *mcp.CallToolResult {
	return shapeResult(nil, text, true)
}

// shapeResult builds a tool result under contentMode: the one place that
// decides which blocks a result carries. The text of a JSON answer is the
// payload as compact JSON, exactly as structuredContent is encoded.
func shapeResult(payload any, prose string, isProse bool) *mcp.CallToolResult {
	text, structured := prose, payload
	if isProse {
		structured = map[string]string{proseKey: prose}
	} else {
		text = compactJSON(payload)
	}
	r := &mcp.CallToolResult{Content: []mcp.Content{}}
	if contentMode != ContentStructured {
		r.Content = append(r.Content, mcp.TextContent{Type: "text", Text: text})
	}
	if contentMode != ContentText {
		r.StructuredContent = structured
	}
	return r
}

// compactJSON renders payload as one line of JSON, as json.Marshal does,
// so the text and structuredContent of one answer are the same bytes. A
// payload that cannot be encoded yields the encoding error as text.
func compactJSON(payload any) string {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("encode result: %v", err)
	}
	return string(b)
}

// contentCopies is how many copies of an answer a response carries.
func contentCopies() int {
	if contentMode == ContentBoth {
		return 2
	}
	return 1
}

// contentBytes is how many bytes an answer whose JSON (json.Marshal
// output) is b takes in a response's content blocks on the wire: b itself
// as structuredContent, and b as a JSON string in the text block, where
// each quote and backslash gains an escaping backslash. The JSON-RPC
// envelope around the blocks is not counted.
func contentBytes(b []byte) int {
	textBytes := len(b) + bytes.Count(b, []byte(`"`)) + bytes.Count(b, []byte(`\`))
	switch contentMode {
	case ContentText:
		return textBytes
	case ContentStructured:
		return len(b)
	default:
		return len(b) + textBytes
	}
}

// wireBytes is exactly how many bytes the content blocks of an answer
// whose JSON is b take on the wire: the content list (each text block
// encoded as {"type":"text","text":...}) and the structuredContent value,
// encoded as the transport encodes them. contentBytes is the cheaper
// estimate used to plan a split; wireBytes is the test a response must pass.
func wireBytes(b []byte) int {
	r := shapeResult(json.RawMessage(b), "", false)
	n := 0
	if c, err := json.Marshal(r.Content); err == nil {
		n += len(c)
	}
	if r.StructuredContent != nil {
		if sc, err := json.Marshal(r.StructuredContent); err == nil {
			n += len(sc)
		}
	}
	return n
}

// rowBytes is contentBytes of one array element and its separator.
func rowBytes(b []byte) int { return contentBytes(b) + contentCopies() }
