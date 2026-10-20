package comparison

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

var (
	ErrBindingInvalid  = errors.New("comparison: binding_invalid")
	ErrBindingEvidence = errors.New("comparison: binding_evidence_unavailable")
)

// Conditions describe the observation scope, never an implicit global default.
type Conditions struct {
	Network       string `json:"network"`
	Configuration string `json:"configuration"`
	ComponentRole string `json:"component_role"`
	ObservedAt    string `json:"observed_at"`
}

type BindingEvidence struct {
	ID       string              `json:"id"`
	Kind     string              `json:"kind"`
	Facets   []string            `json:"facets"`
	Citation contract.CitationV2 `json:"citation"`
}

// ProjectBinding is a proposed interpretation with exact source anchors.
// Source integrity does not approve the proposed mapping semantics.
type ProjectBinding struct {
	Applicability string                 `json:"applicability,omitempty"`
	ID            string                 `json:"id"`
	Concept       ConceptKey             `json:"concept"`
	Coordinates   contract.V2Coordinates `json:"coordinates"`
	Conditions    Conditions             `json:"conditions"`
	LocalConcept  string                 `json:"local_concept"`
	Mapping       string                 `json:"mapping"`
	Reason        string                 `json:"reason"`
	Evidence      []BindingEvidence      `json:"evidence"`
}

type BindingDocument struct {
	SchemaVersion int              `json:"schema_version"`
	Bindings      []ProjectBinding `json:"bindings"`
}

type ObservedBindingEvidence struct {
	Reference BindingEvidence `json:"reference"`
	Body      string          `json:"body"`
}

type ObservedBinding struct {
	Applicability string                    `json:"applicability,omitempty"`
	BindingID     string                    `json:"binding_id"`
	BindingDigest string                    `json:"binding_digest"`
	Concept       ConceptKey                `json:"concept"`
	Coordinates   contract.V2Coordinates    `json:"coordinates"`
	Conditions    Conditions                `json:"conditions"`
	LocalConcept  string                    `json:"local_concept"`
	Mapping       string                    `json:"mapping"`
	Reason        string                    `json:"reason"`
	ReviewState   string                    `json:"review_state"`
	Verification  string                    `json:"verification"`
	Evidence      []ObservedBindingEvidence `json:"evidence"`
	Warnings      []string                  `json:"warnings"`
}

type MappingReader struct {
	session *collection.Session
	scheme  *Scheme
	cleaner *sanitize.Engine
}

func NewMappingReader(ctx context.Context, catalog collection.Catalog, scheme *Scheme, cleaner *sanitize.Engine) (*MappingReader, error) {
	if scheme == nil || cleaner == nil {
		return nil, ErrBindingInvalid
	}
	session, err := collection.Open(ctx, catalog, cleaner)
	if err != nil {
		return nil, err
	}
	return &MappingReader{session: session, scheme: scheme, cleaner: cleaner}, nil
}
func (r *MappingReader) Close() error          { return r.session.Close() }
func (r *MappingReader) CatalogDigest() string { return r.session.Digest() }

func (s *Scheme) validateBinding(b ProjectBinding) error {
	selected := s.concept(b.Concept.ConceptID)
	if !localID(b.ID) || selected.Key != b.Concept || b.Concept.PackID == "" || !validConditions(b.Conditions) || !annotation(b.LocalConcept) || !annotation(b.Reason) || len(b.Evidence) == 0 || len(b.Evidence) > evidencev2.MaxCitations {
		return ErrBindingInvalid
	}
	switch b.Mapping {
	case "exact", "close", "broader", "narrower", "related":
	default:
		return ErrBindingInvalid
	}
	switch b.Applicability {
	case "", "applicable", "unknown", "not_applicable":
	default:
		return ErrBindingInvalid
	}
	allowed := map[string]bool{}
	for _, f := range selected.View.Facets {
		allowed[f.ID] = true
	}
	ids := map[string]bool{}
	refs := map[contract.CitationKeyV2]bool{}
	for _, e := range b.Evidence {
		c := e.Citation
		coords := contract.V2Coordinates{ProjectID: c.ProjectID, DatasetID: c.DatasetID, SnapshotID: c.SnapshotID, SourceMode: c.SourceMode, BaseCommit: c.BaseCommit}
		commit := ""
		if b.Coordinates.SourceMode == "committed" {
			commit = b.Coordinates.BaseCommit
		}
		if !localID(e.ID) || ids[e.ID] || refs[c.KeyV2()] || coords != b.Coordinates || c.CommitHash != commit || c.StartLine < 1 || c.EndLine < c.StartLine || c.EndLine-c.StartLine > 255 || !hash64(c.FileSHA256) || !hash64(c.ContentSHA256) || len(e.Facets) == 0 || len(e.Facets) > 32 {
			return ErrBindingInvalid
		}
		if _, err := (collection.ScopedRef{Coordinates: coords, OriginID: c.OriginID, Path: c.File, LocalID: e.ID}).Key(); err != nil {
			return ErrBindingInvalid
		}
		switch e.Kind {
		case "code", "document", "configuration", "policy", "test":
		default:
			return ErrBindingInvalid
		}
		seen := map[string]bool{}
		for _, f := range e.Facets {
			if !allowed[f] || seen[f] {
				return ErrBindingInvalid
			}
			seen[f] = true
		}
		ids[e.ID], refs[c.KeyV2()] = true, true
	}
	if b.Applicability == "not_applicable" {
		declared := false
		for _, e := range b.Evidence {
			declared = declared || e.Kind == "document" || e.Kind == "policy" || e.Kind == "configuration"
		}
		if !declared {
			return ErrBindingInvalid
		}
	}
	return nil
}
func annotation(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 4096 }
func validConditions(c Conditions) bool {
	at, err := time.Parse(time.RFC3339, c.ObservedAt)
	return err == nil && !at.IsZero() && annotation(c.Network) && annotation(c.Configuration) && annotation(c.ComponentRole) && len(c.Network) <= 256 && len(c.Configuration) <= 256 && len(c.ComponentRole) <= 256
}
func hash64(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && strings.ToLower(s) == s
}

func (s *Scheme) DecodeBindings(data []byte) (BindingDocument, error) {
	var d BindingDocument
	if strictJSON(data, &d) != nil || (d.SchemaVersion != 1 && d.SchemaVersion != 2) || d.SchemaVersion == 1 && len(d.Bindings) == 0 || len(d.Bindings) > 128 {
		return BindingDocument{}, ErrBindingInvalid
	}
	type scopedID struct {
		Coordinates contract.V2Coordinates
		Conditions  Conditions
		Concept     ConceptKey
		ID          string
	}
	seen := map[scopedID]bool{}
	for _, b := range d.Bindings {
		if s.validateBinding(b) != nil || d.SchemaVersion == 1 && b.Applicability != "" {
			return BindingDocument{}, ErrBindingInvalid
		}
		scope := b.Conditions
		at, _ := time.Parse(time.RFC3339, scope.ObservedAt)
		scope.ObservedAt = at.UTC().Format(time.RFC3339Nano)
		key := scopedID{b.Coordinates, scope, b.Concept, b.ID}
		if seen[key] {
			return BindingDocument{}, ErrBindingInvalid
		}
		seen[key] = true
	}
	return d, nil
}

// BindingDigest binds the exact interpreted scope and source hashes. It is
// not a signature or a semantic review, and it changes when evidence changes.
func (s *Scheme) BindingDigest(b ProjectBinding) (string, error) {
	if err := s.validateBinding(b); err != nil {
		return "", err
	}
	data, err := json.Marshal(b)
	if err != nil {
		return "", ErrBindingInvalid
	}
	sum := sha256.Sum256(append([]byte("cks.project-concept-binding.v1\x00"), data...))
	return hex.EncodeToString(sum[:]), nil
}

func (r *MappingReader) ReadBinding(ctx context.Context, collectionID string, b ProjectBinding) (ObservedBinding, error) {
	if err := ctx.Err(); err != nil {
		return ObservedBinding{}, err
	}
	// Own request slices before validation or archive I/O.
	if len(b.Evidence) > evidencev2.MaxCitations {
		return ObservedBinding{}, ErrBindingInvalid
	}
	b.Evidence = append([]BindingEvidence(nil), b.Evidence...)
	for i := range b.Evidence {
		if len(b.Evidence[i].Facets) > 32 {
			return ObservedBinding{}, ErrBindingInvalid
		}
		b.Evidence[i].Facets = append([]string(nil), b.Evidence[i].Facets...)
	}
	digest, err := r.scheme.BindingDigest(b)
	if err != nil {
		return ObservedBinding{}, err
	}
	request := collection.Request{CatalogDigest: r.session.Digest(), CollectionID: collectionID, Mode: collection.Implementation, Query: "comparison binding " + b.ID, Target: &b.Coordinates}
	result, err := r.session.Read(ctx, request, func(ctx context.Context, v collection.Version, q string) (contract.EvidencePackV2, error) {
		refs := make([]evidencev2.Ref, 0, len(b.Evidence))
		expected := map[contract.CitationV2]bool{}
		for _, e := range b.Evidence {
			c := e.Citation
			refs = append(refs, evidencev2.Ref{OriginID: c.OriginID, Citation: contract.Citation{File: c.File, StartLine: c.StartLine, EndLine: c.EndLine, CommitHash: c.CommitHash}})
			expected[c] = true
		}
		pack, err := evidencev2.BuildFromRefs(ctx, v.VersionDir, q, refs, r.cleaner)
		if err != nil {
			return contract.EvidencePackV2{}, ErrBindingEvidence
		}
		if pack.EvidenceState != "complete" || len(pack.Citations) != len(refs) || len(pack.Bodies) != len(refs) {
			return contract.EvidencePackV2{}, ErrBindingEvidence
		}
		for _, c := range pack.Citations {
			if !expected[c] {
				return contract.EvidencePackV2{}, ErrBindingEvidence
			}
		}
		return pack, nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return ObservedBinding{}, ctx.Err()
		}
		if errors.Is(err, collection.ErrFetchFailed) {
			return ObservedBinding{}, ErrBindingEvidence
		}
		return ObservedBinding{}, err
	}
	pack := result.Groups[0].Evidence
	body := map[contract.CitationV2]string{}
	for _, item := range pack.Bodies {
		body[item.Citation] = item.Text
	}
	clean := func(value string) (string, error) {
		c := b.Evidence[0].Citation
		ref := contract.Citation{File: c.File, StartLine: c.StartLine, EndLine: c.EndLine, CommitHash: c.CommitHash}
		result, err := r.cleaner.Sanitize(ctx, []sanitize.Sanitizable{{Citation: ref, Body: value}})
		if err != nil || result.FailClosed || len(result.Items) != 1 || result.Items[0].Dropped {
			return "", ErrBindingEvidence
		}
		return result.Items[0].Body, nil
	}
	local, err := clean(b.LocalConcept)
	if err != nil {
		return ObservedBinding{}, err
	}
	reason, err := clean(b.Reason)
	if err != nil {
		return ObservedBinding{}, err
	}
	conditions := b.Conditions
	for _, p := range []*string{&conditions.Network, &conditions.Configuration, &conditions.ComponentRole} {
		value, err := clean(*p)
		if err != nil {
			return ObservedBinding{}, err
		}
		*p = value
	}
	observed := ObservedBinding{Applicability: b.Applicability, BindingID: b.ID, BindingDigest: digest, Concept: b.Concept, Coordinates: b.Coordinates, Conditions: conditions, LocalConcept: local, Mapping: b.Mapping, Reason: reason, ReviewState: "proposed", Verification: "archived_source_hash_and_span_only", Evidence: []ObservedBindingEvidence{}, Warnings: []string{"mapping_semantics_not_reviewed", "evidence_kind_is_declared_not_inferred", "test_source_is_not_test_execution"}}
	for _, e := range b.Evidence {
		e.Facets = append([]string(nil), e.Facets...)
		observed.Evidence = append(observed.Evidence, ObservedBindingEvidence{Reference: e, Body: body[e.Citation]})
	}
	return observed, nil
}
