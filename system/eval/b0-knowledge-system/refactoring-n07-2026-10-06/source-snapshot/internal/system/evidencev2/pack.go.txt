package evidencev2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// Build resolves every citation from a retained source archive. The caller's
// sanitizer is mandatory: no raw archived bytes can cross the public path.
func Build(ctx context.Context, versionDir, query string, refs []contract.Citation, cleaner *sanitize.Engine) (contract.EvidencePackV2, error) {
	requested := make([]Ref, 0, len(refs))
	for _, ref := range refs {
		requested = append(requested, Ref{OriginID: "repo", Citation: ref})
	}
	return BuildFromRefs(ctx, versionDir, query, requested, cleaner)
}

type Ref struct {
	OriginID string
	Citation contract.Citation
}

// BuildFromRefs admits explicitly registered non-repository origins while
// retaining the v1 Build signature and its default repository-only behavior.
const MaxBodyBytes = 32_000
const MaxCitations = 12
const MaxCandidateRefs = 128

var ErrEvidenceBudget = errors.New("budget_exceeded")

func BuildFromRefs(ctx context.Context, versionDir, query string, refs []Ref, cleaner *sanitize.Engine) (contract.EvidencePackV2, error) {
	if cleaner == nil || query == "" {
		return contract.EvidencePackV2{}, fmt.Errorf("v2 evidence requires a query and sanitizer")
	}
	if len(refs) > MaxCandidateRefs {
		return contract.EvidencePackV2{}, fmt.Errorf("%w: candidate reference limit", ErrEvidenceBudget)
	}
	pack := contract.EvidencePackV2{FormatVersion: 2, Query: query, Citations: []contract.CitationV2{}, Bodies: []contract.BodyV2{}, GraphNeighbors: []any{}, EvidenceState: "complete", Metadata: contract.V2Metadata{IntegrityHashAlgo: "sha256-v2"}}
	seen := map[contract.CitationKeyV2]bool{}
	reasons := map[string]bool{}
	diagnostic := contract.V2EvidenceBudget{Scope: "retained_core", MaxBodyBytes: MaxBodyBytes, MaxCitations: MaxCitations, RequestedReferences: len(refs)}
	spans := make([]setup.RetainedLineSpan, 0, len(refs))
	for _, ref := range refs {
		if !validV2Origin(ref.OriginID) {
			return contract.EvidencePackV2{}, fmt.Errorf("invalid v2 citation origin")
		}
		spans = append(spans, setup.RetainedLineSpan{OriginID: ref.OriginID, Path: ref.Citation.File, First: ref.Citation.StartLine, Last: ref.Citation.EndLine})
	}
	if len(spans) > 0 {
		err := setup.VisitRetainedLineSpans(ctx, versionDir, spans, func(i int, e setup.RetainedEvidence) error {
			ref := refs[i].Citation
			if ref.CommitHash != "" && ref.CommitHash != e.BaseCommit {
				return fmt.Errorf("snapshot_mismatch: citation commit differs from retained source")
			}
			coordinates := contract.V2Coordinates{ProjectID: e.ProjectID, DatasetID: e.DatasetID, SnapshotID: e.SnapshotID, SourceMode: e.SourceMode, BaseCommit: e.BaseCommit}
			if pack.Coordinates.ProjectID == "" {
				pack.Coordinates = coordinates
			} else if pack.Coordinates != coordinates {
				return fmt.Errorf("snapshot_mismatch: mixed evidence coordinates")
			}
			citation := contract.CitationV2{File: e.File, StartLine: e.StartLine, EndLine: e.EndLine, CommitHash: e.CommitHash, ProjectID: e.ProjectID, DatasetID: e.DatasetID, SnapshotID: e.SnapshotID, SourceMode: e.SourceMode, BaseCommit: e.BaseCommit, OriginID: e.OriginID, FileSHA256: e.FileSHA256, ContentSHA256: e.ContentSHA256}
			if seen[citation.KeyV2()] {
				return nil
			}
			seen[citation.KeyV2()] = true
			diagnostic.UniqueReferences++
			cleaned, err := cleaner.Sanitize(ctx, []sanitize.Sanitizable{{Citation: ref, Body: e.Text}})
			if err != nil {
				return err
			}
			if cleaned.FailClosed {
				return fmt.Errorf("fail_closed: retained evidence blocked by sanitization policy")
			}
			if len(cleaned.Items) != 1 || cleaned.Items[0].Citation != ref {
				return fmt.Errorf("snapshot_mismatch: sanitizer returned incomplete evidence")
			}
			item := cleaned.Items[0]
			// Sanitization policy (especially fail_closed) is checked even when the
			// whole source span will be omitted by the byte/citation budget.
			if len(e.Text) > MaxBodyBytes {
				diagnostic.OmittedReferences++
				reasons["body_bytes"] = true
				return nil
			}
			overBytes := len(e.Text) > MaxBodyBytes-diagnostic.SelectedRawBytes || len(item.Body) > MaxBodyBytes-diagnostic.SelectedBodyBytes
			overCitations := len(pack.Citations) >= MaxCitations
			if overBytes || overCitations {
				diagnostic.OmittedReferences++
				if overBytes {
					reasons["body_bytes"] = true
				}
				if overCitations {
					reasons["citation_count"] = true
				}
				return nil
			}
			pack.Citations = append(pack.Citations, citation)
			diagnostic.SelectedRawBytes += len(e.Text)
			if item.Dropped {
				pack.EvidenceState = "partial"
			} else {
				pack.Bodies = append(pack.Bodies, contract.BodyV2{Citation: citation, Text: item.Body})
				diagnostic.SelectedBodyBytes += len(item.Body)
			}
			return nil
		})
		if err != nil {
			return contract.EvidencePackV2{}, err
		}
	} else {
		if info, err := os.Lstat(filepath.Join(versionDir, "sources", "manifest.json")); err != nil || !info.Mode().IsRegular() {
			return contract.EvidencePackV2{}, fmt.Errorf("source_missing: retained source manifest is unavailable")
		}
		identity, err := setup.InspectVersionIdentity(versionDir)
		if err != nil {
			return contract.EvidencePackV2{}, err
		}
		if identity == nil {
			return contract.EvidencePackV2{}, fmt.Errorf("reindex_required: v2 evidence needs a pinned dataset")
		}
		pack.Coordinates = contract.V2Coordinates{ProjectID: identity.Source.ProjectID, DatasetID: identity.DatasetID, SnapshotID: identity.Source.SnapshotID, SourceMode: identity.Source.SourceMode, BaseCommit: identity.Source.SourceCommit}
	}
	if diagnostic.OmittedReferences > 0 {
		if len(pack.Citations) == 0 {
			return contract.EvidencePackV2{}, fmt.Errorf("%w: no complete retained span fits", ErrEvidenceBudget)
		}
		pack.EvidenceState = "partial"
		diagnostic.SelectedCitations = len(pack.Citations)
		for reason := range reasons {
			diagnostic.Reasons = append(diagnostic.Reasons, reason)
		}
		sort.Strings(diagnostic.Reasons)
		pack.Metadata.EvidenceBudget = &diagnostic
	}
	if err := Stamp(&pack); err != nil {
		return contract.EvidencePackV2{}, err
	}
	return pack, nil
}

// Stamp hashes the entire v2 object except metadata.integrity_hash. This
// implementation accepts the v2 DTO's I-JSON subset: strings, booleans,
// null, arrays, maps, and safe integers. Semantic overlays with floating
// numbers require a full ECMAScript number serializer before exposure.
func Stamp(pack *contract.EvidencePackV2) error {
	if pack == nil || pack.FormatVersion != 2 || pack.Metadata.IntegrityHashAlgo != "sha256-v2" {
		return fmt.Errorf("invalid v2 evidence metadata")
	}
	if len(pack.GraphNeighbors) != 0 {
		return fmt.Errorf("v2 semantic and graph overlays require coordinate validation before release")
	}
	if err := validatePack(*pack); err != nil {
		return err
	}
	pack.Metadata.IntegrityHash = ""
	canonical, err := canonicalBytes(*pack)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(canonical)
	pack.Metadata.IntegrityHash = hex.EncodeToString(sum[:])
	return nil
}

func validatePack(pack contract.EvidencePackV2) error {
	c := pack.Coordinates
	if c.ProjectID == "" || len(c.DatasetID) != 64 || len(c.SnapshotID) != 64 ||
		(c.SourceMode != "committed" && c.SourceMode != "working-tree" && c.SourceMode != "snapshot-only") ||
		((c.SourceMode == "snapshot-only") != (c.BaseCommit == "")) ||
		(c.BaseCommit != "" && len(c.BaseCommit) != 40) ||
		(pack.EvidenceState != "complete" && pack.EvidenceState != "partial") {
		return fmt.Errorf("invalid v2 evidence coordinates")
	}
	if !hexDigest(c.DatasetID) || !hexDigest(c.SnapshotID) || len(pack.Citations) > 12 || pack.Query == "" {
		return fmt.Errorf("invalid v2 evidence identity or limit")
	}
	seen := make(map[contract.CitationKeyV2]bool)
	full := make(map[contract.CitationV2]bool)
	for _, citation := range pack.Citations {
		if citation.ProjectID != c.ProjectID || citation.DatasetID != c.DatasetID || citation.SnapshotID != c.SnapshotID ||
			citation.SourceMode != c.SourceMode || citation.BaseCommit != c.BaseCommit || !validV2Origin(citation.OriginID) ||
			citation.File == "" || citation.StartLine <= 0 || citation.EndLine < citation.StartLine ||
			!hexDigest(citation.FileSHA256) || !hexDigest(citation.ContentSHA256) {
			return fmt.Errorf("invalid v2 citation coordinate")
		}
		if c.SourceMode == "committed" && citation.CommitHash != c.BaseCommit ||
			c.SourceMode != "committed" && citation.CommitHash != "" {
			return fmt.Errorf("invalid v2 citation commit")
		}
		key := citation.KeyV2()
		if seen[key] {
			return fmt.Errorf("duplicate v2 citation")
		}
		seen[key] = true
		full[citation] = true
	}
	bodySeen := make(map[contract.CitationV2]bool)
	bodyBytes := 0
	for _, body := range pack.Bodies {
		if !full[body.Citation] || bodySeen[body.Citation] {
			return fmt.Errorf("v2 body has no unique citation")
		}
		bodySeen[body.Citation] = true
		if len(body.Text) > MaxBodyBytes-bodyBytes {
			return fmt.Errorf("%w: public body bytes exceeded", ErrEvidenceBudget)
		}
		bodyBytes += len(body.Text)
	}
	if b := pack.Metadata.EvidenceBudget; b != nil {
		if b.Scope != "retained_core" || b.MaxBodyBytes != MaxBodyBytes || b.MaxCitations != MaxCitations || b.RequestedReferences > MaxCandidateRefs || b.RequestedReferences < b.UniqueReferences || b.UniqueReferences != b.SelectedCitations+b.OmittedReferences || b.SelectedCitations < 1 || b.SelectedCitations > len(pack.Citations) || b.OmittedReferences < 1 || b.SelectedRawBytes < 0 || b.SelectedRawBytes > MaxBodyBytes || b.SelectedBodyBytes < 0 || b.SelectedBodyBytes > bodyBytes || pack.EvidenceState != "partial" || len(b.Reasons) == 0 || len(b.Reasons) > 2 {
			return fmt.Errorf("invalid v2 evidence budget diagnostic")
		}
		for i, reason := range b.Reasons {
			if reason != "body_bytes" && reason != "citation_count" || i > 0 && b.Reasons[i-1] >= reason {
				return fmt.Errorf("invalid v2 evidence budget reason")
			}
		}
	}
	if pack.EvidenceState == "complete" && len(pack.Bodies) != len(pack.Citations) {
		return fmt.Errorf("v2 evidence claims completeness without all bodies")
	}
	if err := validateKnowledgeSemantic(pack.Semantic, full); err != nil {
		return err
	}
	return nil
}

var knowledgeOrigin = regexp.MustCompile(`^knowledge:[a-z][a-z0-9]*(?:\.[a-z][a-z0-9]*)+$`)

func validV2Origin(origin string) bool {
	return origin == "repo" || knowledgeOrigin.MatchString(origin)
}

func hexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' && c < 'a' || c > 'f' {
			return false
		}
	}
	return true
}

func Verify(pack contract.EvidencePackV2) error {
	want := pack.Metadata.IntegrityHash
	if len(want) != 64 {
		return fmt.Errorf("invalid v2 evidence hash")
	}
	if err := Stamp(&pack); err != nil {
		return err
	}
	if pack.Metadata.IntegrityHash != want {
		return fmt.Errorf("v2 evidence integrity mismatch")
	}
	return nil
}

func canonicalBytes(value any) ([]byte, error) {
	if err := validateUnicode(reflect.ValueOf(value)); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var parsed any
	if err := dec.Decode(&parsed); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, parsed); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func validateUnicode(value reflect.Value) error {
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		return validateUnicode(value.Elem())
	case reflect.String:
		if !utf8.ValidString(value.String()) {
			return fmt.Errorf("v2 JCS invalid UTF-8")
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).IsExported() {
				if err := validateUnicode(value.Field(i)); err != nil {
					return err
				}
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := validateUnicode(value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			if iter.Key().Kind() != reflect.String {
				return fmt.Errorf("v2 JCS requires string object keys")
			}
			if err := validateUnicode(iter.Key()); err != nil {
				return err
			}
			if err := validateUnicode(iter.Value()); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeCanonical(out *bytes.Buffer, value any) error {
	switch v := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if v {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		return writeJSONString(out, v)
	case json.Number:
		i, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil || i < -9007199254740991 || i > 9007199254740991 {
			return fmt.Errorf("v2 JCS permits safe integers only")
		}
		out.WriteString(strconv.FormatInt(i, 10))
	case []any:
		out.WriteByte('[')
		for i, element := range v {
			if i != 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, element); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		// All DTO property names are ASCII. Reject non-ASCII names until UTF-16
		// key ordering is supported for user-supplied semantic objects.
		for _, key := range keys {
			for _, r := range key {
				if r > 127 {
					return fmt.Errorf("v2 JCS non-ASCII property name unsupported")
				}
			}
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, key := range keys {
			if i != 0 {
				out.WriteByte(',')
			}
			if err := writeJSONString(out, key); err != nil {
				return err
			}
			out.WriteByte(':')
			if err := writeCanonical(out, v[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("v2 JCS unsupported value %T", value)
	}
	return nil
}

func writeJSONString(out *bytes.Buffer, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("v2 JCS invalid UTF-8")
	}
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\f':
			out.WriteString(`\f`)
		case '\r':
			out.WriteString(`\r`)
		default:
			if r < 0x20 {
				out.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return nil
}
