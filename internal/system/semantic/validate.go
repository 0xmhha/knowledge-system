package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

var fullCommit = regexp.MustCompile(`^[0-9a-f]{40}$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (s Snapshot) validate() error {
	if strings.TrimSpace(s.ProjectID) == "" || strings.TrimSpace(s.DatasetID) == "" {
		return fmt.Errorf("project_id and dataset_id are required")
	}
	if !fullCommit.MatchString(s.Commit) {
		return fmt.Errorf("commit must be a full 40-character lowercase Git SHA")
	}
	return nil
}

// Validate checks schema shape, local references, and snapshot isolation.
// It does not prove source bytes; call ValidateSources before promotion.
func (p Projection) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("semantic schema version %d, want %d", p.SchemaVersion, SchemaVersion)
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
		sections[section.ID] = section
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
	if err := p.Validate(); err != nil {
		return err
	}
	for _, e := range p.Evidence {
		cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "show", p.Snapshot.Commit+":"+e.Path)
		content, err := cmd.Output()
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
	return nil
}
