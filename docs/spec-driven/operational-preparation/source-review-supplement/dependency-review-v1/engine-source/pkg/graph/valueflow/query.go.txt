package valueflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

type QueryInput struct {
	NodeID                 string  `json:"node_id,omitempty"`
	CanonicalID            string  `json:"canonical_id,omitempty"`
	ExpectedCommit         string  `json:"expected_commit,omitempty"`
	ExpectedSourceSHA256   string  `json:"expected_source_sha256,omitempty"`
	ExpectedSnapshotSHA256 string  `json:"expected_snapshot_sha256,omitempty"`
	Limits                 Options `json:"limits,omitempty"`
}
type QueryResult struct {
	SchemaVersion  int     `json:"schema_version"`
	Status         string  `json:"status"`
	Reason         string  `json:"reason,omitempty"`
	SnapshotSHA256 string  `json:"snapshot_sha256,omitempty"`
	Analysis       *Result `json:"analysis"`
	ElapsedMicros  int64   `json:"elapsed_micros"`
}

func hashBytes(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func validSHA(s string, lengths ...int) bool {
	if s == "" {
		return true
	}
	lengthOK := false
	for _, length := range lengths {
		lengthOK = lengthOK || len(s) == length
	}
	if !lengthOK {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
func (q QueryInput) validate() error {
	if (q.NodeID == "") == (q.CanonicalID == "") || len(q.NodeID) > 128 || len(q.CanonicalID) > 512 {
		return fmt.Errorf("exactly one bounded node_id or canonical_id is required")
	}
	if !validSHA(q.ExpectedCommit, 40, 64) || !validSHA(q.ExpectedSourceSHA256, 64) || !validSHA(q.ExpectedSnapshotSHA256, 64) {
		return fmt.Errorf("invalid immutable source/snapshot pin")
	}
	_, e := limits(q.Limits)
	return e
}

// DecodeQueryInput validates actual types, unknown fields, duplicates and nulls.
// Schema metadata alone is not a runtime validation boundary.
func DecodeQueryInput(raw []byte) (QueryInput, error) {
	var q QueryInput
	if len(raw) > 8192 {
		return q, fmt.Errorf("value-flow input exceeds8KiB")
	}
	if e := validateInputObject(raw); e != nil {
		return q, e
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return q, e
	}
	for _, key := range []string{"node_id", "canonical_id", "expected_commit", "expected_source_sha256", "expected_snapshot_sha256"} {
		if field, ok := fields[key]; ok {
			var value string
			if e := json.Unmarshal(field, &value); e != nil || value == "" {
				return q, fmt.Errorf("explicit nonempty string required")
			}
		}
	}
	if field, ok := fields["limits"]; ok {
		var bounds map[string]int
		if e := json.Unmarshal(field, &bounds); e != nil {
			return q, e
		}
		for _, value := range bounds {
			if value < 1 {
				return q, fmt.Errorf("explicit limits must be positive integers")
			}
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&q); e != nil {
		return q, e
	}
	return q, q.validate()
}

// Query reads only a retained graph symbol/blob. It never opens its live file.
// Name-based ambiguous resolution belongs in find_symbol; this query requires
// the exact selected identity and verifies snapshot/blob/metadata before return.
func Query(ctx context.Context, r store.Reader, q QueryInput) (QueryResult, error) {
	return query(ctx, r, q, nil)
}

func query(ctx context.Context, r store.Reader, q QueryInput, rules *RuleOptions) (out QueryResult, err error) {
	started := time.Now()
	defer func() { out.ElapsedMicros = time.Since(started).Microseconds() }()
	out = QueryResult{SchemaVersion: 1, Status: "unavailable"}
	if e := q.validate(); e != nil {
		return out, e
	}
	if r == nil || ctx == nil {
		return out, fmt.Errorf("context and retained graph reader required")
	}
	opts, _ := limits(q.Limits)
	bounded, cancel := context.WithTimeout(ctx, time.Duration(opts.TimeoutMS)*time.Millisecond)
	defer cancel()
	finish := func(status, reason string) (QueryResult, error) {
		out.Status = status
		out.Reason = reason
		out.Analysis = nil
		return out, nil
	}
	if bounded.Err() != nil {
		return finish("budget_exhausted", "query context ended")
	}
	before, e := r.GetManifest()
	if e != nil {
		return finish("unavailable", "retained manifest unavailable")
	}
	raw, e := json.Marshal(before)
	if e != nil {
		return out, e
	}
	snapshot := hashBytes(raw)
	out.SnapshotSHA256 = snapshot
	if q.ExpectedSnapshotSHA256 != "" && q.ExpectedSnapshotSHA256 != snapshot {
		return finish("source_mismatch", "expected retained snapshot differs")
	}
	if q.ExpectedCommit != "" && q.ExpectedCommit != before.SrcCommit {
		return finish("source_mismatch", "expected retained source commit differs")
	}
	var selected types.Node
	if q.NodeID != "" {
		nodes, e := r.NodesByIDs([]string{q.NodeID})
		if e != nil {
			return finish("unavailable", "retained symbol unavailable")
		}
		if len(nodes) == 0 {
			return finish("not_found", "selected node identity not found")
		}
		if len(nodes) != 1 || nodes[0].ID != q.NodeID {
			return finish("ambiguous", "selected node identity is not unique")
		}
		selected = nodes[0]
	} else {
		node, found, e := r.FindByCanonicalID(q.CanonicalID)
		if e != nil {
			return finish("unavailable", "canonical symbol unavailable")
		}
		if !found {
			return finish("not_found", "canonical identity not found")
		}
		if node.CanonicalID != q.CanonicalID {
			return finish("source_mismatch", "canonical identity differs")
		}
		selected = node
	}
	if selected.Type != types.NodeFunction && selected.Type != types.NodeMethod {
		return finish("unsupported", "only retained functions/methods supported")
	}
	normalized := strings.ReplaceAll(selected.FilePath, "\\", "/")
	parts := strings.Split(normalized, "/")
	if path.IsAbs(normalized) || path.Clean(normalized) != normalized || normalized == "." {
		return finish("unsupported", "retained relative source path required")
	}
	for i, part := range parts {
		if part == ".." || part == ".claude" || i == 0 && part == "logs" {
			return finish("unsupported", "private knowledge source excluded")
		}
	}
	blob, e := r.GetBlob(selected.ID)
	if e != nil {
		return finish("unavailable", "retained function source unavailable")
	}
	if bounded.Err() != nil {
		return finish("budget_exhausted", "source resolution deadline ended")
	}
	sourceSHA := hashBytes(blob)
	if q.ExpectedSourceSHA256 != "" && q.ExpectedSourceSHA256 != sourceSHA {
		return finish("source_mismatch", "expected function blob differs")
	}
	if selected.EndByte-selected.StartByte != len(blob) || selected.EndLine-selected.StartLine != bytes.Count(blob, []byte{'\n'}) {
		return finish("source_mismatch", "retained node coordinates/blob length differ")
	}
	source := Source{FunctionName: selected.Name, NodeID: selected.ID, CanonicalID: selected.CanonicalID, File: selected.FilePath, Commit: before.SrcCommit, SnapshotSHA256: snapshot, Language: selected.Language, StartLine: selected.StartLine, StartByte: selected.StartByte}
	result, e := build(bounded, source, blob, opts, true, rules)
	if e != nil {
		return out, e
	}
	// Re-read pins after computation. No result can mix old metadata/new bytes.
	after, e := r.GetManifest()
	if e != nil {
		return finish("unavailable", "final snapshot unavailable")
	}
	afterRaw, e := json.Marshal(after)
	if e != nil {
		return out, e
	}
	if hashBytes(afterRaw) != snapshot {
		return finish("snapshot_changed", "retained graph manifest changed during analysis")
	}
	current, e := r.NodesByIDs([]string{selected.ID})
	if e != nil || len(current) != 1 {
		return finish("snapshot_changed", "selected node changed during analysis")
	}
	oldNode, _ := json.Marshal(selected)
	newNode, _ := json.Marshal(current[0])
	if !bytes.Equal(oldNode, newNode) {
		return finish("snapshot_changed", "selected node metadata changed during analysis")
	}
	currentBlob, e := r.GetBlob(selected.ID)
	if e != nil || hashBytes(currentBlob) != sourceSHA {
		return finish("snapshot_changed", "selected function bytes changed during analysis")
	}
	if bounded.Err() != nil && result.Status != "budget_exhausted" {
		return finish("budget_exhausted", "final source verification deadline ended")
	}
	out.Status = result.Status
	out.Analysis = &result
	return out, nil
}

func validateInputObject(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 4 {
			return fmt.Errorf("input nesting limit exceeded")
		}
		v, e := d.Token()
		if e != nil {
			return e
		}
		if v == nil {
			return fmt.Errorf("explicit null is not a supported input")
		}
		delimiter, ok := v.(json.Delim)
		if !ok {
			return nil
		}
		if delimiter != '{' {
			return fmt.Errorf("only schema objects accepted")
		}
		seen := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return e
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate input field")
			}
			seen[name] = true
			if e = walk(depth + 1); e != nil {
				return e
			}
		}
		_, e = d.Token()
		return e
	}
	if e := walk(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return fmt.Errorf("one input document required")
	}
	return nil
}
