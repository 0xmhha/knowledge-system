package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

var fullCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var conceptIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var languageTagPattern = regexp.MustCompile(`^[a-z]{2,3}(?:-[A-Za-z0-9]+)*$`)

func (s Snapshot) validate() error {
	if strings.TrimSpace(s.ProjectID) == "" || strings.TrimSpace(s.DatasetID) == "" {
		return fmt.Errorf("project_id and dataset_id are required")
	}
	switch s.SourceMode {
	case "", "committed", "working-tree":
		if !fullCommit.MatchString(s.Commit) {
			return fmt.Errorf("commit must be a full 40-character lowercase Git SHA")
		}
	case "snapshot-only":
		if s.Commit != "" {
			return fmt.Errorf("snapshot-only source cannot claim a Git commit")
		}
	default:
		return fmt.Errorf("unsupported semantic source mode %q", s.SourceMode)
	}
	if s.SnapshotID != "" && !digestPattern.MatchString(s.SnapshotID) {
		return fmt.Errorf("snapshot_id must be a SHA-256 digest")
	}
	if s.SourceMode != "" && s.SnapshotID == "" {
		return fmt.Errorf("source-mode semantic evidence requires snapshot_id")
	}
	return nil
}

// Validate checks schema shape, local references, and snapshot isolation.
// It does not prove source bytes; call ValidateSources before promotion.
func (p Projection) Validate() error {
	if p.SchemaVersion != 1 && p.SchemaVersion != 2 && p.SchemaVersion != SchemaVersion && p.SchemaVersion != PackProjectionVersion {
		return fmt.Errorf("semantic schema version %d is unsupported", p.SchemaVersion)
	}
	if p.SchemaVersion == PackProjectionVersion {
		if p.Snapshot.SnapshotID == "" || p.Snapshot.SourceMode == "" {
			return fmt.Errorf("v4 pack projection requires pinned source coordinates")
		}
		if err := p.Knowledge.validate(); err != nil {
			return err
		}
	} else if p.Knowledge != nil {
		return fmt.Errorf("semantic pack projection requires schema version 4")
	}
	if p.SchemaVersion == 1 && len(p.Requirements) != 0 {
		return fmt.Errorf("semantic schema version 1 cannot contain requirements")
	}
	if err := p.Snapshot.validate(); err != nil {
		return err
	}
	ids := map[string]bool{}
	claimID := func(id string) error {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("empty semantic record id")
		}
		if ids[id] {
			return fmt.Errorf("duplicate semantic record id %q", id)
		}
		ids[id] = true
		return nil
	}
	evidence := make(map[string]EvidenceSpan, len(p.Evidence))
	for _, e := range p.Evidence {
		if err := claimID(e.ID); err != nil {
			return err
		}
		if e.Snapshot != p.Snapshot {
			return fmt.Errorf("evidence %q crosses project or snapshot", e.ID)
		}
		if e.Kind != SourceDocument && e.Kind != SourceCode && e.Kind != SourceTest {
			return fmt.Errorf("evidence %q has invalid source kind %q", e.ID, e.Kind)
		}
		if !safeRelativePath(e.Path) || e.StartLine < 1 || e.EndLine < e.StartLine {
			return fmt.Errorf("evidence %q has invalid source location", e.ID)
		}
		if !digestPattern.MatchString(e.ContentSHA256) || strings.TrimSpace(e.Extractor) == "" {
			return fmt.Errorf("evidence %q needs a SHA-256 source digest and extractor", e.ID)
		}
		evidence[e.ID] = e
	}
	sections := make(map[string]DocumentSection, len(p.Sections))
	for _, section := range p.Sections {
		if err := claimID(section.ID); err != nil {
			return err
		}
		if strings.TrimSpace(section.Heading) == "" {
			return fmt.Errorf("section %q needs a heading", section.ID)
		}
		e, ok := evidence[section.EvidenceID]
		if !ok || e.Kind != SourceDocument {
			return fmt.Errorf("section %q needs document evidence", section.ID)
		}
		chunkIDs := map[string]bool{}
		for _, id := range section.ChunkIDs {
			if !digestPattern.MatchString(id) || chunkIDs[id] {
				return fmt.Errorf("section %q has invalid or duplicate CKV chunk ID", section.ID)
			}
			chunkIDs[id] = true
		}
		sections[section.ID] = section
	}
	concepts := make(map[string]Concept, len(p.Concepts))
	for _, concept := range p.Concepts {
		if err := claimID(concept.ID); err != nil {
			return err
		}
		if !conceptIDPattern.MatchString(concept.ID) {
			return fmt.Errorf("concept %q has invalid stable id", concept.ID)
		}
		switch concept.Kind {
		case "entity", "artifact", "process", "rule":
		default:
			return fmt.Errorf("concept %q has invalid kind %q", concept.ID, concept.Kind)
		}
		if strings.TrimSpace(concept.Definition) == "" || len(concept.Includes) == 0 || len(concept.Excludes) == 0 {
			return fmt.Errorf("concept %q needs a definition and explicit included/excluded scope", concept.ID)
		}
		for _, scope := range append(append([]string{}, concept.Includes...), concept.Excludes...) {
			if strings.TrimSpace(scope) == "" {
				return fmt.Errorf("concept %q has empty scope item", concept.ID)
			}
		}
		if len(concept.Terms) == 0 {
			return fmt.Errorf("concept %q needs language terms", concept.ID)
		}
		seenTerms := map[string]bool{}
		preferredByLang := map[string]int{}
		for _, term := range concept.Terms {
			if !languageTagPattern.MatchString(term.Lang) || strings.TrimSpace(term.Value) == "" {
				return fmt.Errorf("concept %q has invalid language term", concept.ID)
			}
			key := term.Lang + "\x00" + strings.ToLower(strings.TrimSpace(term.Value))
			if seenTerms[key] {
				return fmt.Errorf("concept %q repeats term %q", concept.ID, term.Value)
			}
			seenTerms[key] = true
			if term.Preferred {
				preferredByLang[term.Lang]++
			}
		}
		for _, term := range concept.Terms {
			if preferredByLang[term.Lang] != 1 {
				return fmt.Errorf("concept %q needs exactly one preferred term in %s", concept.ID, term.Lang)
			}
		}
		e, ok := evidence[concept.EvidenceID]
		if !ok || e.Kind != SourceDocument || e.Extractor != "ontology-yaml-v1" {
			return fmt.Errorf("concept %q needs document evidence", concept.ID)
		}
		switch concept.Status {
		case StatusProposed:
		case StatusVerified, StatusRejected:
			if strings.TrimSpace(concept.ReviewedBy) == "" {
				return fmt.Errorf("reviewed concept %q needs reviewed_by", concept.ID)
			}
		default:
			return fmt.Errorf("concept %q has invalid status %q", concept.ID, concept.Status)
		}
		concepts[concept.ID] = concept
	}
	usedSpecEvidence := map[string]bool{}
	criteria := map[string]AcceptanceCriterion{}
	criterionRequirement := map[string]Requirement{}
	for _, requirement := range p.Requirements {
		if err := claimID(requirement.ID); err != nil {
			return err
		}
		if requirement.Version < 1 || strings.TrimSpace(requirement.Title) == "" || strings.TrimSpace(requirement.Statement) == "" {
			return fmt.Errorf("requirement %q needs a positive version, title, and statement", requirement.ID)
		}
		e, ok := evidence[requirement.EvidenceID]
		if !ok || e.Kind != SourceDocument || e.Extractor != "spec-yaml-v1" {
			return fmt.Errorf("requirement %q needs specification source evidence", requirement.ID)
		}
		usedSpecEvidence[requirement.EvidenceID] = true
		switch requirement.Status {
		case StatusProposed:
		case StatusVerified, StatusRejected:
			if strings.TrimSpace(requirement.ReviewedBy) == "" {
				return fmt.Errorf("reviewed requirement %q needs reviewed_by", requirement.ID)
			}
		default:
			return fmt.Errorf("requirement %q has invalid status %q", requirement.ID, requirement.Status)
		}
		for _, conceptID := range requirement.ConceptIDs {
			if _, ok := concepts[conceptID]; !ok {
				return fmt.Errorf("requirement %q refers to missing concept %q", requirement.ID, conceptID)
			}
		}
		if len(requirement.AcceptanceCriteria) == 0 {
			return fmt.Errorf("requirement %q needs at least one acceptance criterion", requirement.ID)
		}
		for _, criterion := range requirement.AcceptanceCriteria {
			if err := claimID(criterion.ID); err != nil {
				return err
			}
			if strings.TrimSpace(criterion.Given) == "" || strings.TrimSpace(criterion.When) == "" || strings.TrimSpace(criterion.Then) == "" {
				return fmt.Errorf("criterion %q needs given, when, and then", criterion.ID)
			}
			ce, ok := evidence[criterion.EvidenceID]
			if !ok || ce.Kind != SourceDocument || ce.Extractor != "spec-yaml-v1" || ce.Path != e.Path ||
				ce.StartLine < e.StartLine || ce.EndLine > e.EndLine {
				return fmt.Errorf("criterion %q needs source evidence inside requirement %q", criterion.ID, requirement.ID)
			}
			usedSpecEvidence[criterion.EvidenceID] = true
			criteria[criterion.ID] = criterion
			criterionRequirement[criterion.ID] = requirement
		}
	}
	for _, e := range p.Evidence {
		if e.Extractor == "spec-yaml-v1" && !usedSpecEvidence[e.ID] {
			return fmt.Errorf("orphan specification evidence %q", e.ID)
		}
	}
	claims := make(map[string]Claim, len(p.Claims))
	for _, claim := range p.Claims {
		if err := claimID(claim.ID); err != nil {
			return err
		}
		if strings.TrimSpace(claim.Statement) == "" {
			return fmt.Errorf("claim %q needs a statement", claim.ID)
		}
		section, ok := sections[claim.SectionID]
		if !ok {
			return fmt.Errorf("claim %q has no source section", claim.ID)
		}
		if len(claim.EvidenceIDs) == 0 {
			return fmt.Errorf("claim %q has no evidence", claim.ID)
		}
		sectionEvidence := false
		for _, id := range claim.EvidenceIDs {
			if _, ok := evidence[id]; !ok {
				return fmt.Errorf("claim %q refers to missing evidence %q", claim.ID, id)
			}
			if id == section.EvidenceID {
				sectionEvidence = true
			}
		}
		if !sectionEvidence {
			return fmt.Errorf("claim %q omits its source section evidence", claim.ID)
		}
		switch claim.Status {
		case StatusProposed:
		case StatusVerified, StatusRejected:
			if strings.TrimSpace(claim.ReviewedBy) == "" {
				return fmt.Errorf("reviewed claim %q needs reviewed_by", claim.ID)
			}
		default:
			return fmt.Errorf("claim %q has invalid status %q", claim.ID, claim.Status)
		}
		claims[claim.ID] = claim
	}
	for _, assertion := range p.Assertions {
		if err := claimID(assertion.ID); err != nil {
			return err
		}
		if assertion.SubjectID == assertion.ObjectID {
			return fmt.Errorf("assertion %q has identical endpoints", assertion.ID)
		}
		if len(assertion.EvidenceIDs) == 0 {
			return fmt.Errorf("assertion %q has no evidence", assertion.ID)
		}
		seenEvidence := map[string]bool{}
		for _, id := range assertion.EvidenceIDs {
			if _, ok := evidence[id]; !ok {
				return fmt.Errorf("assertion %q refers to missing evidence %q", assertion.ID, id)
			}
			if seenEvidence[id] {
				return fmt.Errorf("assertion %q repeats evidence %q", assertion.ID, id)
			}
			seenEvidence[id] = true
		}
		switch assertion.Predicate {
		case PredicateSupports:
			section, ok := sections[assertion.SubjectID]
			if !ok {
				return fmt.Errorf("assertion %q SUPPORTS subject must be a document section", assertion.ID)
			}
			claim, ok := claims[assertion.ObjectID]
			if !ok {
				return fmt.Errorf("assertion %q SUPPORTS object must be a claim", assertion.ID)
			}
			if claim.SectionID != section.ID || !seenEvidence[section.EvidenceID] {
				return fmt.Errorf("assertion %q SUPPORTS must cite its claim's source section", assertion.ID)
			}
			if assertion.Status == StatusVerified && claim.Status != StatusVerified {
				return fmt.Errorf("verified assertion %q refers to unverified claim", assertion.ID)
			}
		case PredicateContradicts:
			left, leftOK := claims[assertion.SubjectID]
			right, rightOK := claims[assertion.ObjectID]
			if !leftOK || !rightOK {
				return fmt.Errorf("assertion %q CONTRADICTS endpoints must be claims", assertion.ID)
			}
			if !hasAnyEvidence(seenEvidence, left.EvidenceIDs) || !hasAnyEvidence(seenEvidence, right.EvidenceIDs) {
				return fmt.Errorf("assertion %q CONTRADICTS must cite both claims", assertion.ID)
			}
			if assertion.Status == StatusVerified && (left.Status != StatusVerified || right.Status != StatusVerified) {
				return fmt.Errorf("verified assertion %q refers to unverified claim", assertion.ID)
			}
		case PredicateAbout:
			claim, claimOK := claims[assertion.SubjectID]
			concept, conceptOK := concepts[assertion.ObjectID]
			if !claimOK || !conceptOK {
				return fmt.Errorf("assertion %q ABOUT must connect claim to concept", assertion.ID)
			}
			if !hasAnyEvidence(seenEvidence, claim.EvidenceIDs) || !seenEvidence[concept.EvidenceID] {
				return fmt.Errorf("assertion %q ABOUT must cite claim and concept sources", assertion.ID)
			}
			if assertion.Status == StatusVerified && (claim.Status != StatusVerified || concept.Status != StatusVerified) {
				return fmt.Errorf("verified assertion %q refers to unverified claim or concept", assertion.ID)
			}
		case PredicateImplementedBy:
			concept, ok := concepts[assertion.SubjectID]
			if !ok {
				return fmt.Errorf("assertion %q IMPLEMENTED_BY subject must be a concept", assertion.ID)
			}
			if !seenEvidence[concept.EvidenceID] {
				return fmt.Errorf("assertion %q IMPLEMENTED_BY must cite concept source", assertion.ID)
			}
			codeProof := false
			for _, id := range assertion.EvidenceIDs {
				e := evidence[id]
				if (e.Kind == SourceCode || e.Kind == SourceTest) && e.CanonicalID == assertion.ObjectID && e.CanonicalID != "" {
					codeProof = true
				}
			}
			if !codeProof {
				return fmt.Errorf("assertion %q IMPLEMENTED_BY needs a matching CKG code anchor", assertion.ID)
			}
			if assertion.Status == StatusVerified && concept.Status != StatusVerified {
				return fmt.Errorf("verified assertion %q refers to unverified concept", assertion.ID)
			}
		case PredicateTestedBy:
			if !hasCanonicalEvidence(seenEvidence, evidence, assertion.SubjectID, SourceCode) ||
				!hasCanonicalEvidence(seenEvidence, evidence, assertion.ObjectID, SourceTest) {
				return fmt.Errorf("assertion %q TESTED_BY needs code and test CKG anchors", assertion.ID)
			}
		case PredicateAcceptedBy, PredicateCheckedBy:
			if assertion.Predicate == PredicateAcceptedBy && p.SchemaVersion >= 3 {
				return fmt.Errorf("assertion %q ACCEPTED_BY is read-only in schema 3; write CHECKED_BY", assertion.ID)
			}
			if assertion.Predicate == PredicateCheckedBy && p.SchemaVersion < 3 {
				return fmt.Errorf("assertion %q CHECKED_BY requires schema 3", assertion.ID)
			}
			criterion, ok := criteria[assertion.SubjectID]
			if !ok || !seenEvidence[criterion.EvidenceID] ||
				!hasCanonicalEvidence(seenEvidence, evidence, assertion.ObjectID, SourceTest) {
				return fmt.Errorf("assertion %q %s needs criterion and test sources", assertion.ID, assertion.Predicate)
			}
			if assertion.Status == StatusVerified && criterionRequirement[criterion.ID].Status != StatusVerified {
				return fmt.Errorf("verified assertion %q refers to unapproved requirement", assertion.ID)
			}
		default:
			return fmt.Errorf("assertion %q has invalid predicate %q", assertion.ID, assertion.Predicate)
		}
		switch assertion.Status {
		case StatusProposed:
		case StatusVerified, StatusRejected:
			if strings.TrimSpace(assertion.ReviewedBy) == "" {
				return fmt.Errorf("reviewed assertion %q needs reviewed_by", assertion.ID)
			}
		default:
			return fmt.Errorf("assertion %q has invalid status %q", assertion.ID, assertion.Status)
		}
	}
	return nil
}

func hasAnyEvidence(in map[string]bool, ids []string) bool {
	for _, id := range ids {
		if in[id] {
			return true
		}
	}
	return false
}

func hasCanonicalEvidence(in map[string]bool, evidence map[string]EvidenceSpan, canonicalID string, kind SourceKind) bool {
	if canonicalID == "" {
		return false
	}
	for id := range in {
		e := evidence[id]
		if e.CanonicalID == canonicalID && e.Kind == kind {
			return true
		}
	}
	return false
}

func safeRelativePath(file string) bool {
	if file == "" || strings.Contains(file, "\\") || strings.ContainsRune(file, '\x00') || strings.HasPrefix(file, "/") {
		return false
	}
	clean := path.Clean(file)
	return clean == file && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

// ValidateSources checks every evidence line range against the exact commit
// recorded in the projection. It reads Git objects rather than the current
// working tree, so uncommitted edits cannot masquerade as verified evidence.
func (p Projection) ValidateSources(ctx context.Context, repoRoot string) error {
	if p.SchemaVersion == PackProjectionVersion {
		return fmt.Errorf("requires_v2: pack projection requires retained source validation")
	}
	if p.Snapshot.SourceMode == "working-tree" || p.Snapshot.SourceMode == "snapshot-only" {
		return fmt.Errorf("requires_v2: non-committed semantic evidence requires retained source validation")
	}
	return p.validateSourceBytes(func(file string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "show", p.Snapshot.Commit+":"+file)
		return cmd.Output()
	})
}

// ValidateRetainedSources checks the immutable archive used by non-committed
// snapshots. It also works for retained committed candidates and never opens
// the mutable checkout or asks Git for HEAD content.
func (p Projection) ValidateRetainedSources(versionDir string) error {
	if err := p.validateSourceBytes(func(file string) ([]byte, error) {
		buf, identity, _, err := setup.ReadRetainedFile(versionDir, "repo", file)
		if err != nil {
			return nil, err
		}
		if identity.Source.ProjectID != p.Snapshot.ProjectID || identity.DatasetID != p.Snapshot.DatasetID ||
			identity.Source.SnapshotID != p.Snapshot.SnapshotID || identity.Source.SourceCommit != p.Snapshot.Commit ||
			identity.Source.SourceMode != p.Snapshot.SourceMode {
			return nil, fmt.Errorf("snapshot_mismatch: semantic source coordinates differ from archive")
		}
		return buf, nil
	}); err != nil {
		return err
	}
	if p.SchemaVersion == PackProjectionVersion {
		return p.Knowledge.validateRetained(versionDir)
	}
	return nil
}

func (p Projection) validateSourceBytes(read func(string) ([]byte, error)) error {
	if err := p.Validate(); err != nil {
		return err
	}
	contents := map[string][]byte{}
	load := func(file string) ([]byte, error) {
		if content, ok := contents[file]; ok {
			return content, nil
		}
		content, err := read(file)
		if err != nil {
			return nil, err
		}
		contents[file] = content
		return content, nil
	}
	for _, e := range p.Evidence {
		content, err := load(e.Path)
		if err != nil {
			return fmt.Errorf("evidence %q source unavailable at commit: %w", e.ID, err)
		}
		lines := strings.SplitAfter(string(content), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		if e.EndLine > len(lines) {
			return fmt.Errorf("evidence %q line range extends past EOF", e.ID)
		}
		span := strings.Join(lines[e.StartLine-1:e.EndLine], "")
		digest := sha256.Sum256([]byte(span))
		if hex.EncodeToString(digest[:]) != e.ContentSHA256 {
			return fmt.Errorf("evidence %q source digest mismatch", e.ID)
		}
	}
	// A source hash proves that the cited YAML bytes exist, but by itself it
	// does not prove that the projected definition and terms still say what
	// those bytes say. Reparse each ontology source and compare the full set.
	byPath := map[string]map[string]Concept{}
	evidenceByID := map[string]EvidenceSpan{}
	for _, e := range p.Evidence {
		evidenceByID[e.ID] = e
	}
	for _, concept := range p.Concepts {
		file := evidenceByID[concept.EvidenceID].Path
		if byPath[file] == nil {
			byPath[file] = map[string]Concept{}
		}
		byPath[file][concept.ID] = concept
	}
	for file, want := range byPath {
		extracted, _, err := ExtractOntology(p.Snapshot, file, contents[file])
		if err != nil {
			return fmt.Errorf("ontology %q source parse: %w", file, err)
		}
		if len(extracted.Concepts) != len(want) {
			return fmt.Errorf("ontology %q concept set differs from source", file)
		}
		for _, original := range extracted.Concepts {
			current, ok := want[original.ID]
			if !ok || !reflect.DeepEqual(current, original) {
				return fmt.Errorf("ontology concept %q differs from source", original.ID)
			}
		}
	}
	specByPath := map[string]map[string]Requirement{}
	for _, requirement := range p.Requirements {
		file := evidenceByID[requirement.EvidenceID].Path
		if specByPath[file] == nil {
			specByPath[file] = map[string]Requirement{}
		}
		specByPath[file][requirement.ID] = requirement
	}
	for file, want := range specByPath {
		extracted, _, err := ExtractSpec(p.Snapshot, file, contents[file])
		if err != nil {
			return fmt.Errorf("spec %q source parse: %w", file, err)
		}
		if len(extracted.Requirements) != len(want) {
			return fmt.Errorf("spec %q requirement set differs from source", file)
		}
		for _, original := range extracted.Requirements {
			current, ok := want[original.ID]
			if !ok || !reflect.DeepEqual(current, original) {
				return fmt.Errorf("spec requirement %q differs from source", original.ID)
			}
		}
	}
	return nil
}
