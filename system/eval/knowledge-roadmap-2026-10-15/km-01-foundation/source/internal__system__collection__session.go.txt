package collection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

const (
	Implementation = "implementation"
	Comparison     = "comparison"
)

// Request never resolves an implicit current or infers a project from a name.
// Implementation has exactly one target; comparison has explicit participants.
type Request struct {
	CatalogDigest string                   `json:"catalog_digest"`
	CollectionID  string                   `json:"collection_id"`
	Mode          string                   `json:"mode"`
	Query         string                   `json:"query"`
	Target        *contract.V2Coordinates  `json:"target,omitempty"`
	Participants  []contract.V2Coordinates `json:"participants,omitempty"`
}

type Group struct {
	Coordinates contract.V2Coordinates  `json:"coordinates"`
	Evidence    contract.EvidencePackV2 `json:"evidence"`
}

// Result deliberately has no flat citations or implementation patch target
// in comparison mode. Even identical code from two forks stays in two groups.
type Result struct {
	CatalogDigest string  `json:"catalog_digest"`
	CollectionID  string  `json:"collection_id"`
	Mode          string  `json:"mode"`
	Groups        []Group `json:"groups"`
}

// Fetcher is supplied by a future CLI/MCP adapter. The returned pack is
// checked against the selected archive, including sanitized body bytes.
// Errors are fail-closed for the entire request; partial comparison reporting
// belongs to the later comparison API, not implicit cross-project fallback.
type Fetcher func(context.Context, Version, string) (contract.EvidencePackV2, error)

type Session struct {
	mu       sync.RWMutex
	closed   bool
	digest   string
	versions map[contract.V2Coordinates]Version
	groups   map[string]map[contract.V2Coordinates]bool
	leases   []*setup.ReaderLease
	cleaner  *sanitize.Engine
}

// Open verifies the complete declared catalog before returning a reader.
// Leases protect pinned versions from GC. It does not promote a held candidate,
// modify a dataset, create a writer or rewrite historical identity formats.
func Open(ctx context.Context, catalog Catalog, cleaner *sanitize.Engine) (*Session, error) {
	if cleaner == nil {
		return nil, ErrCatalogInvalid
	}
	raw, err := catalog.CanonicalBytes()
	if err != nil {
		return nil, err
	}
	// Own every slice: later mutation of the caller's config cannot alter a pin.
	var owned Catalog
	if err := json.Unmarshal(raw, &owned); err != nil {
		return nil, ErrCatalogInvalid
	}
	catalog = owned
	hash := sha256.Sum256(append([]byte("cks.collection-catalog.v1\x00"), raw...))
	s := &Session{digest: hex.EncodeToString(hash[:]), versions: map[contract.V2Coordinates]Version{},
		groups: map[string]map[contract.V2Coordinates]bool{}, cleaner: cleaner}
	for _, v := range catalog.Versions {
		if err := ctx.Err(); err != nil {
			_ = s.Close()
			return nil, err
		}
		if !fixedDirectory(v.VersionDir) {
			_ = s.Close()
			return nil, ErrDatasetMismatch
		}
		lease, err := setup.PinVersionReader(v.VersionDir)
		if err != nil {
			_ = s.Close()
			return nil, ErrDatasetMismatch
		}
		s.leases = append(s.leases, lease)
		identity, err := setup.InspectVersionIdentityContext(ctx, v.VersionDir)
		if err != nil || !sameIdentity(identity, v.Coordinates) || !sameOrigins(v) {
			_ = s.Close()
			return nil, ErrDatasetMismatch
		}
		s.versions[v.Coordinates] = v
	}
	for _, group := range catalog.Collections {
		s.groups[group.ID] = map[contract.V2Coordinates]bool{}
		for _, member := range group.Members {
			s.groups[group.ID][member] = true
		}
	}
	return s, nil
}

func fixedDirectory(dir string) bool {
	resolved, err := filepath.EvalSymlinks(dir)
	return err == nil && resolved == dir
}

func sameIdentity(id *setup.DatasetIdentity, c contract.V2Coordinates) bool {
	return id != nil && id.DatasetID == c.DatasetID && id.Source.ProjectID == c.ProjectID &&
		id.Source.SnapshotID == c.SnapshotID && id.Source.SourceMode == c.SourceMode && id.Source.SourceCommit == c.BaseCommit
}

func sameOrigins(v Version) bool {
	var captured setup.CapturedSource
	raw, err := os.ReadFile(filepath.Join(v.VersionDir, "sources", "manifest.json"))
	if err != nil || json.Unmarshal(raw, &captured) != nil {
		return false
	}
	actual := map[string]bool{"repo": true}
	for _, f := range captured.Files {
		actual[f.OriginID] = true
	}
	if len(actual) != len(v.Origins) {
		return false
	}
	for _, id := range v.Origins {
		if !actual[id] {
			return false
		}
	}
	return true
}

func (s *Session) Digest() string { return s.digest }

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var errs []error
	for _, lease := range s.leases {
		errs = append(errs, lease.Close())
	}
	return errors.Join(errs...)
}

func (s *Session) selectVersions(r Request) ([]Version, error) {
	if s.closed {
		return nil, ErrClosed
	}
	if r.CatalogDigest != s.digest {
		return nil, ErrStaleCatalog
	}
	members, exists := s.groups[r.CollectionID]
	if !exists || strings.TrimSpace(r.Query) == "" || len(r.Query) > 8192 {
		return nil, ErrScopeMismatch
	}
	var coordinates []contract.V2Coordinates
	switch r.Mode {
	case Implementation:
		if r.Target == nil {
			return nil, ErrTargetRequired
		}
		if len(r.Participants) != 0 {
			return nil, ErrScopeMismatch
		}
		coordinates = []contract.V2Coordinates{*r.Target}
	case Comparison:
		if r.Target != nil || len(r.Participants) < 2 || len(r.Participants) > 16 {
			return nil, ErrScopeMismatch
		}
		coordinates = r.Participants
	default:
		return nil, ErrUnsupportedMode
	}
	seen := map[contract.V2Coordinates]bool{}
	versions := make([]Version, 0, len(coordinates))
	for _, c := range coordinates {
		if !members[c] || seen[c] {
			return nil, ErrScopeMismatch
		}
		seen[c] = true
		v := s.versions[c]
		v.Origins = append([]string(nil), v.Origins...)
		versions = append(versions, v)
	}
	return versions, nil
}

// ScopeKey is a routing/cache NAMESPACE, not a complete result cache key.
// Adapters must additionally bind tool options, model/config/sanitizer identity
// and any authorization context; this package stores no retrieval-result cache.
func (s *Session) ScopeKey(request Request) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := s.selectVersions(request); err != nil {
		return "", err
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return "", ErrScopeMismatch
	}
	hash := sha256.Sum256(append([]byte("cks.collection-request.v1\x00"), raw...))
	return hex.EncodeToString(hash[:]), nil
}

// Read guards both sides of a backend call. A stale archive, mismatched pack
// or unsanitized/foreign body discards every result. No fallback selects another
// project when the requested version has no result or is unavailable.
func (s *Session) Read(ctx context.Context, request Request, fetch Fetcher) (Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions, err := s.selectVersions(request)
	if err != nil {
		return Result{}, err
	}
	if fetch == nil {
		return Result{}, ErrFetchFailed
	}
	result := Result{CatalogDigest: s.digest, CollectionID: request.CollectionID, Mode: request.Mode, Groups: []Group{}}
	for _, v := range versions {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if !fixedDirectory(v.VersionDir) {
			return Result{}, ErrDatasetMismatch
		}
		qctx, verification, err := setup.BeginRequestVerification(ctx, v.VersionDir)
		if err != nil || verification == nil {
			return Result{}, ErrDatasetMismatch
		}
		if !sameIdentity(verification.Identity(), v.Coordinates) {
			verification.Close()
			return Result{}, ErrDatasetMismatch
		}
		pack, fetchErr := fetch(qctx, v, request.Query)
		if fetchErr != nil {
			verification.Close()
			return Result{}, ErrFetchFailed
		}
		if err := s.verifyPack(qctx, v, request.Query, pack); err != nil {
			verification.Close()
			return Result{}, ErrEvidenceMismatch
		}
		if err := verification.Finish(ctx); err != nil {
			return Result{}, ErrDatasetMismatch
		}
		result.Groups = append(result.Groups, Group{Coordinates: v.Coordinates, Evidence: pack})
	}
	// A later participant must not change an archive already checked earlier
	// in this comparison. Publish only after every pin is checked once more.
	for _, v := range versions {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		id, err := setup.InspectVersionIdentityContext(ctx, v.VersionDir)
		if err != nil || !fixedDirectory(v.VersionDir) || !sameIdentity(id, v.Coordinates) {
			return Result{}, ErrDatasetMismatch
		}
	}
	return result, nil
}

func (s *Session) verifyPack(ctx context.Context, v Version, query string, pack contract.EvidencePackV2) error {
	if pack.Coordinates != v.Coordinates || pack.Query != query || evidencev2.Verify(pack) != nil {
		return ErrEvidenceMismatch
	}
	bodies := map[contract.CitationV2]string{}
	for _, body := range pack.Bodies {
		bodies[body.Citation] = body.Text
	}
	allowed := map[string]bool{}
	for _, id := range v.Origins {
		allowed[id] = true
	}
	spans := make([]setup.RetainedLineSpan, 0, len(pack.Citations))
	for _, c := range pack.Citations {
		if !allowed[c.OriginID] {
			return ErrEvidenceMismatch
		}
		spans = append(spans, setup.RetainedLineSpan{OriginID: c.OriginID, Path: c.File, First: c.StartLine, Last: c.EndLine})
	}
	// Definition-search diagnostics also carry source coordinates. A pack with
	// correctly scoped code must not attach another project's ontology proof.
	var textSources []contract.OntologyTextSource
	if pack.Metadata.Ontology != nil {
		textSources = pack.Metadata.Ontology.TextSources
	}
	if len(textSources) > 8 {
		return ErrEvidenceMismatch
	}
	for _, source := range textSources {
		if source.ProjectID != v.Coordinates.ProjectID || source.DatasetID != v.Coordinates.DatasetID ||
			source.SnapshotID != v.Coordinates.SnapshotID || source.Commit != v.Coordinates.BaseCommit ||
			!digest(source.QuerySHA256, 32) {
			return ErrEvidenceMismatch
		}
		spans = append(spans, setup.RetainedLineSpan{OriginID: "repo", Path: source.File, First: source.StartLine, Last: source.EndLine})
	}
	return setup.VisitRetainedLineSpans(ctx, v.VersionDir, spans, func(index int, actual setup.RetainedEvidence) error {
		if index >= len(pack.Citations) {
			if textSources[index-len(pack.Citations)].ContentSHA256 != actual.ContentSHA256 {
				return ErrEvidenceMismatch
			}
			return nil
		}
		c := pack.Citations[index]
		if c.FileSHA256 != actual.FileSHA256 || c.ContentSHA256 != actual.ContentSHA256 {
			return ErrEvidenceMismatch
		}
		ref := contract.Citation{File: c.File, StartLine: c.StartLine, EndLine: c.EndLine, CommitHash: c.CommitHash}
		cleaned, err := s.cleaner.Sanitize(ctx, []sanitize.Sanitizable{{Citation: ref, Body: actual.Text}})
		if err != nil || cleaned.FailClosed || len(cleaned.Items) != 1 {
			return ErrEvidenceMismatch
		}
		if text, present := bodies[c]; present && (cleaned.Items[0].Dropped || text != cleaned.Items[0].Body) {
			return ErrEvidenceMismatch
		}
		return nil
	})
}
