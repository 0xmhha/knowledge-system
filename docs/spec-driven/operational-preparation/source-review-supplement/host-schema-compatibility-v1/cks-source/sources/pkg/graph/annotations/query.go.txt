// Package annotations provides bounded, retained operator metadata queries.
package annotations

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

type Input struct {
	NodeID                 string `json:"node_id,omitempty"`
	CanonicalID            string `json:"canonical_id,omitempty"`
	ExpectedCommit         string `json:"expected_commit,omitempty"`
	ExpectedSourceSHA256   string `json:"expected_source_sha256,omitempty"`
	ExpectedSnapshotSHA256 string `json:"expected_snapshot_sha256,omitempty"`
	MaxAnnotations         int    `json:"max_annotations,omitempty"`
	TimeoutMS              int    `json:"timeout_ms,omitempty"`
}
type KnownText struct {
	Status string `json:"status"`
	Value  string `json:"value"`
}
type Provenance struct {
	Origin         string    `json:"origin"`
	SourcePath     string    `json:"source_path"`
	SourceSHA256   KnownText `json:"source_sha256"`
	SourceLine     KnownText `json:"source_line"`
	Version        KnownText `json:"version"`
	Reviewer       KnownText `json:"reviewer"`
	ReviewedAt     KnownText `json:"reviewed_at"`
	Approval       KnownText `json:"approval"`
	IdentityTrust  string    `json:"identity_trust"`
	MetadataSHA256 string    `json:"metadata_sha256"`
}
type Annotation struct {
	ID               string     `json:"id"`
	Kind             string     `json:"kind"`
	Name             string     `json:"name"`
	Category         string     `json:"category"`
	RelatedSymbolID  string     `json:"related_symbol_id"`
	Explanation      KnownText  `json:"explanation"`
	ModificationHint KnownText  `json:"modification_hint"`
	Severity         KnownText  `json:"severity"`
	Provenance       Provenance `json:"provenance"`
}
type Symbol struct {
	NodeID         string    `json:"node_id"`
	CanonicalID    string    `json:"canonical_id"`
	File           string    `json:"file"`
	IndexedCommit  string    `json:"indexed_commit"`
	SourceSHA256   KnownText `json:"source_sha256"`
	MetadataSHA256 string    `json:"metadata_sha256"`
}
type Result struct {
	SchemaVersion  int          `json:"schema_version"`
	Status         string       `json:"status"`
	Reason         string       `json:"reason"`
	SnapshotSHA256 string       `json:"snapshot_sha256"`
	Symbol         *Symbol      `json:"symbol"`
	Annotations    []Annotation `json:"annotations"`
	More           bool         `json:"more"`
	MaxAnnotations int          `json:"max_annotations"`
	TimeoutMS      int          `json:"timeout_ms"`
	SafetyVerdict  string       `json:"safety_verdict"`
	AutomaticRules string       `json:"automatic_rules"`
	Limitations    []string     `json:"limitations"`
	ElapsedMicros  int64        `json:"elapsed_micros"`
}

func digest(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func metadata(v any) string    { raw, _ := json.Marshal(v); return digest(raw) }
func unknown() KnownText       { return KnownText{Status: "unknown"} }
func known(s string) KnownText { return KnownText{Status: "retained_operator_value", Value: s} }
func (q Input) validate() error {
	fields := map[string]string{}
	for k, v := range map[string]string{"node_id": q.NodeID, "canonical_id": q.CanonicalID, "expected_commit": q.ExpectedCommit, "expected_source_sha256": q.ExpectedSourceSHA256, "expected_snapshot_sha256": q.ExpectedSnapshotSHA256} {
		if v != "" {
			fields[k] = v
		}
	}
	raw, _ := json.Marshal(fields)
	if _, e := valueflow.DecodeQueryInput(raw); e != nil {
		return e
	}
	if q.MaxAnnotations < 0 || q.MaxAnnotations > 128 || q.TimeoutMS < 0 || q.TimeoutMS > 5000 {
		return fmt.Errorf("annotation limits outside caps")
	}
	return nil
}

// Decode rejects duplicate/null/unknown fields as well as schema type bypasses.
func Decode(raw []byte) (Input, error) {
	var q Input
	if len(raw) > 8192 {
		return q, fmt.Errorf("input exceeds8KiB")
	}
	// Reuse the strict object/token boundary, substituting only our numeric caps
	// for the already validated local-analysis limits object.
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return q, e
	}
	for _, key := range []string{"max_annotations", "timeout_ms"} {
		if v, ok := fields[key]; ok {
			var n int
			if e := json.Unmarshal(v, &n); e != nil || n < 1 {
				return q, fmt.Errorf("explicit limits must be positive integers")
			}
			delete(fields, key)
		}
	}
	// The original document must also reject duplicate keys. Scan tokens directly.
	d := json.NewDecoder(bytes.NewReader(raw))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return q, fmt.Errorf("object required")
	}
	seen := map[string]bool{}
	for d.More() {
		k, e := d.Token()
		if e != nil {
			return q, e
		}
		key, ok := k.(string)
		if !ok || seen[key] {
			return q, fmt.Errorf("duplicate field")
		}
		seen[key] = true
		var v json.RawMessage
		if e = d.Decode(&v); e != nil {
			return q, e
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return q, fmt.Errorf("null field")
		}
	}
	rest, _ := json.Marshal(fields)
	if _, e = valueflow.DecodeQueryInput(rest); e != nil {
		return q, e
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e = d.Decode(&q); e != nil {
		return q, e
	}
	// valueflow's document boundary rejects trailing input through this check.
	var extra any
	if d.Decode(&extra) != io.EOF {
		return q, fmt.Errorf("one document required")
	}
	if e = q.validate(); e != nil {
		return q, e
	}
	return q, nil
}
func private(file string, relative bool) bool {
	file = strings.ReplaceAll(file, "\\", "/")
	if relative && (path.IsAbs(file) || path.Clean(file) != file || file == "." || file == "") {
		return true
	}
	for i, part := range strings.Split(file, "/") {
		if part == ".." || part == ".claude" || part == "logs" && (i == 0 || !relative) {
			return true
		}
	}
	return false
}

// Query never reads a live source/YAML file or executes a rule detector.
func Query(ctx context.Context, r store.Reader, q Input) (out Result, err error) {
	start := time.Now()
	defer func() { out.ElapsedMicros = time.Since(start).Microseconds() }()
	out = Result{SchemaVersion: 1, Status: "unavailable", Annotations: []Annotation{}, SafetyVerdict: "unknown", AutomaticRules: "separate_opt_in_rule_candidates_not_executed", Limitations: []string{"Manual tags are operator assertions, not verified vulnerabilities or policy compliance; empty tags do not prove safety", "YAML bytes, real row, version, reviewer, review time and approval were not retained; identity/signature trust unsupported", "SQL bounds row count; synchronous legacy node/blob reads and row bytes are not hard process memory or preemptive time bounds", "Legacy merged descriptions cannot reliably separate remediation or severity", "Final rechecks are not a database snapshot transaction and cannot detect changes reverted between reads"}}
	if e := q.validate(); e != nil {
		return out, e
	}
	if r == nil || ctx == nil {
		return out, fmt.Errorf("retained reader and context required")
	}
	if q.MaxAnnotations == 0 {
		q.MaxAnnotations = 32
	}
	if q.TimeoutMS == 0 {
		q.TimeoutMS = 1000
	}
	out.MaxAnnotations = q.MaxAnnotations
	out.TimeoutMS = q.TimeoutMS
	ctx, cancel := context.WithTimeout(ctx, time.Duration(q.TimeoutMS)*time.Millisecond)
	defer cancel()
	finish := func(status, why string) (Result, error) {
		out.Status = status
		out.Reason = why
		out.Symbol = nil
		out.Annotations = []Annotation{}
		out.More = false
		return out, nil
	}
	if ctx.Err() != nil {
		return finish("budget_exhausted", "deadline ended")
	}
	m, e := r.GetManifest()
	if e != nil {
		return finish("unavailable", "manifest unavailable")
	}
	out.SnapshotSHA256 = metadata(m)
	if q.ExpectedCommit != "" && q.ExpectedCommit != m.SrcCommit || q.ExpectedSnapshotSHA256 != "" && q.ExpectedSnapshotSHA256 != out.SnapshotSHA256 {
		return finish("source_mismatch", "expected retained snapshot differs")
	}
	var seed types.Node
	if q.NodeID != "" {
		nodes, e := r.NodesByIDs([]string{q.NodeID})
		if e != nil {
			return finish("unavailable", "symbol unavailable")
		}
		if len(nodes) == 0 {
			return finish("not_found", "exact symbol missing")
		}
		if len(nodes) != 1 || nodes[0].ID != q.NodeID {
			return finish("source_mismatch", "exact symbol differs")
		}
		seed = nodes[0]
	} else {
		n, found, e := r.FindByCanonicalID(q.CanonicalID)
		if e != nil {
			return finish("unavailable", "canonical lookup unavailable")
		}
		if !found {
			return finish("not_found", "canonical symbol missing")
		}
		if n.CanonicalID != q.CanonicalID {
			return finish("source_mismatch", "canonical identity differs")
		}
		seed = n
	}
	if seed.Type == types.NodePolicy || seed.Type == types.NodeSecurityPattern || seed.Type == types.NodeCommit || seed.Type == types.NodeHunk || private(seed.FilePath, true) {
		return finish("unsupported", "code symbol with public relative source required")
	}
	blob, e := r.GetBlob(seed.ID)
	missingBlob := store.IsMissingRow(e)
	if e != nil && !missingBlob {
		return finish("unavailable", "retained source lookup failed")
	}
	source := unknown()
	if len(blob) > 0 {
		source = KnownText{Status: "retained_blob", Value: digest(blob)}
	}
	if q.ExpectedSourceSHA256 != "" && (source.Status == "unknown" || source.Value != q.ExpectedSourceSHA256) {
		return finish("source_mismatch", "expected retained blob differs")
	}
	bounded, ok := r.(store.BoundedEdgeReader)
	if !ok {
		return finish("unavailable", "bounded edge reader unsupported")
	}
	page := store.EdgePageInput{NodeID: seed.ID, Direction: "outgoing", Types: []string{string(types.EdgeGovernedBy), string(types.EdgeHasSecurityPattern)}, Limit: q.MaxAnnotations}
	edges, more, e := bounded.IncidentEdges(ctx, page)
	if ctx.Err() != nil {
		return finish("budget_exhausted", "annotation page deadline ended")
	}
	if e != nil {
		return finish("unavailable", "bounded annotation page unavailable")
	}
	if len(edges) > q.MaxAnnotations {
		return finish("unavailable", "reader exceeded page cap")
	}
	out.Symbol = &Symbol{NodeID: seed.ID, CanonicalID: seed.CanonicalID, File: seed.FilePath, IndexedCommit: m.SrcCommit, SourceSHA256: source, MetadataSHA256: metadata(seed)}
	retained := []types.Node{}
	for _, edge := range edges {
		if ctx.Err() != nil {
			return finish("budget_exhausted", "deadline ended")
		}
		if edge.Src != seed.ID || (edge.Type != types.EdgeGovernedBy && edge.Type != types.EdgeHasSecurityPattern) {
			return finish("source_mismatch", "unexpected relation")
		}
		nodes, e := r.NodesByIDs([]string{edge.Dst})
		if e != nil || len(nodes) != 1 || nodes[0].ID != edge.Dst {
			return finish("unavailable", "annotation target unavailable")
		}
		n := nodes[0]
		if edge.Type == types.EdgeGovernedBy && n.Type != types.NodePolicy || edge.Type == types.EdgeHasSecurityPattern && n.Type != types.NodeSecurityPattern {
			return finish("source_mismatch", "relation target type differs")
		}
		if private(n.FilePath, false) || private(edge.FilePath, false) {
			return finish("unsupported", "private annotation source excluded")
		}
		retained = append(retained, n)
		a := Annotation{ID: n.ID, Kind: string(n.Type), Name: n.Name, Category: n.SubKind, RelatedSymbolID: seed.ID, Explanation: known(n.DocComment), ModificationHint: unknown(), Severity: unknown(), Provenance: Provenance{Origin: "operator_curated_metadata", SourcePath: n.FilePath, SourceSHA256: unknown(), SourceLine: unknown(), Version: unknown(), Reviewer: unknown(), ReviewedAt: unknown(), Approval: unknown(), IdentityTrust: "not_authenticated", MetadataSHA256: metadata(n)}}
		if n.Annotation != nil {
			a.Explanation = known(n.Annotation.Description)
			a.ModificationHint = known(n.Annotation.Remediation)
			a.Severity = known(n.Annotation.Severity)
		}
		out.Annotations = append(out.Annotations, a)
	}
	// Reject mixed snapshots, including consulted metadata and page truncation flag.
	after, e := r.GetManifest()
	if e != nil || metadata(after) != out.SnapshotSHA256 {
		return finish("snapshot_changed", "manifest changed")
	}
	seeds, e := r.NodesByIDs([]string{seed.ID})
	if e != nil || len(seeds) != 1 || metadata(seeds[0]) != metadata(seed) {
		return finish("snapshot_changed", "symbol changed")
	}
	again, e := r.GetBlob(seed.ID)
	if (e != nil && !store.IsMissingRow(e)) || missingBlob != store.IsMissingRow(e) || !bytes.Equal(blob, again) {
		return finish("snapshot_changed", "blob changed")
	}
	for _, n := range retained {
		nodes, e := r.NodesByIDs([]string{n.ID})
		if e != nil || len(nodes) != 1 || metadata(nodes[0]) != metadata(n) {
			return finish("snapshot_changed", "annotation changed")
		}
	}
	againEdges, againMore, e := bounded.IncidentEdges(ctx, page)
	if ctx.Err() != nil {
		return finish("budget_exhausted", "annotation recheck deadline ended")
	}
	if e != nil || more != againMore || !reflect.DeepEqual(edges, againEdges) {
		return finish("snapshot_changed", "annotation page changed")
	}
	if ctx.Err() != nil {
		return finish("budget_exhausted", "deadline ended")
	}
	out.More = more
	out.Status = "annotations_found"
	if len(out.Annotations) == 0 {
		out.Status = "no_annotations"
	}
	if more {
		out.Status = "partial"
		out.Reason = "max_annotations"
	}
	return out, nil
}
