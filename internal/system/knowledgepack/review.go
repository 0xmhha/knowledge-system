package knowledgepack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

// ReviewRecord refers to the exact pre-review source bytes in one captured
// dataset. Review files are separate so adding an approval never changes the
// target hash being approved.
type ReviewRecord struct {
	TargetKind   string    `yaml:"target_kind" json:"target_kind"`
	TargetID     string    `yaml:"target_id" json:"target_id"`
	TargetRef    SourceRef `yaml:"target_ref" json:"target_ref"`
	TargetSHA256 string    `yaml:"target_sha256" json:"target_sha256"`
	SnapshotID   string    `yaml:"snapshot_id" json:"snapshot_id"`
	Decision     string    `yaml:"decision" json:"decision"`
	Reviewer     string    `yaml:"reviewer" json:"reviewer"`
	Reason       string    `yaml:"reason" json:"reason"`
	ReviewedAt   string    `yaml:"reviewed_at" json:"reviewed_at"`
}

func validateReviewRecord(record ReviewRecord) error {
	if record.TargetKind != "policy" && record.TargetKind != "decision" && record.TargetKind != "relation" ||
		!instanceIDPattern.MatchString(record.TargetID) || record.TargetRef.OriginID != "repo" ||
		!strings.HasPrefix(record.TargetRef.Path, ".cks/knowledge/") || !safeRetainedPackPath(record.TargetRef.Path) ||
		!digestPattern.MatchString(record.TargetSHA256) || !digestPattern.MatchString(record.SnapshotID) ||
		(record.Decision != "verified" && record.Decision != "rejected") ||
		!instanceIDPattern.MatchString(record.Reviewer) || strings.TrimSpace(record.Reason) == "" {
		return fmt.Errorf("evidence_unverified: invalid review record")
	}
	if _, err := time.Parse(time.RFC3339, record.ReviewedAt); err != nil {
		return fmt.Errorf("evidence_unverified: invalid review timestamp")
	}
	return nil
}

// ValidateReviewRecord checks a locally authored review before it is written.
func ValidateReviewRecord(record ReviewRecord) error { return validateReviewRecord(record) }

func applyReviewRecords(root string, instances *Instances, minApprovals int) error {
	if instances == nil {
		return fmt.Errorf("evidence_unverified: missing instances")
	}
	type target struct {
		ref    SourceRef
		status *string
		by     *string
		reason *string
	}
	targets := map[string]target{}
	for i := range instances.Policies {
		p := &instances.Policies[i]
		targets["policy\x00"+p.ID] = target{p.SourceRef, &p.Status, &p.ReviewedBy, &p.ReviewReason}
	}
	for i := range instances.Decisions {
		d := &instances.Decisions[i]
		targets["decision\x00"+d.ID] = target{d.SourceRef, &d.Status, &d.ReviewedBy, &d.ReviewReason}
	}
	for i := range instances.Relations {
		r := &instances.Relations[i]
		targets["relation\x00"+r.ID] = target{r.SourceRef, &r.Status, &r.ReviewedBy, &r.ReviewReason}
	}
	votes := map[string]map[string]ReviewRecord{}
	for _, record := range instances.Reviews {
		key := record.TargetKind + "\x00" + record.TargetID
		target, ok := targets[key]
		if !ok || target.ref != record.TargetRef || *target.status != "proposed" {
			return fmt.Errorf("evidence_unverified: review target missing or already decided")
		}
		rel := strings.TrimPrefix(record.TargetRef.Path, ".cks/knowledge/")
		buf, err := setup.ReadSourceFileNoFollow(root, rel, 4<<20)
		if err != nil {
			return fmt.Errorf("evidence_unverified: review target unreadable: %w", err)
		}
		sum := sha256.Sum256(buf)
		if hex.EncodeToString(sum[:]) != record.TargetSHA256 {
			return fmt.Errorf("evidence_unverified: review target bytes changed")
		}
		if votes[key] == nil {
			votes[key] = map[string]ReviewRecord{}
		}
		if previous, duplicate := votes[key][record.Reviewer]; duplicate {
			if previous.Decision != record.Decision || previous.TargetSHA256 != record.TargetSHA256 {
				return fmt.Errorf("evidence_unverified: conflicting reviewer records")
			}
			continue // one reviewer never counts twice
		}
		votes[key][record.Reviewer] = record
	}
	for key, target := range targets {
		if *target.status != "proposed" {
			if minApprovals > 1 {
				return fmt.Errorf("evidence_unverified: inline review cannot meet multi-reviewer quorum")
			}
			continue // legacy inline approval, read-only compatibility
		}
		byDecision := map[string][]ReviewRecord{}
		for _, record := range votes[key] {
			byDecision[record.Decision] = append(byDecision[record.Decision], record)
		}
		if len(byDecision["verified"]) > 0 && len(byDecision["rejected"]) > 0 {
			continue // disputed: remain proposed
		}
		for _, decision := range []string{"verified", "rejected"} {
			approved := byDecision[decision]
			if len(approved) < minApprovals {
				continue
			}
			sort.Slice(approved, func(a, b int) bool { return approved[a].Reviewer < approved[b].Reviewer })
			reviewers, reasons := make([]string, 0, len(approved)), make([]string, 0, len(approved))
			for _, record := range approved {
				reviewers = append(reviewers, record.Reviewer)
				reasons = append(reasons, record.Reason)
			}
			*target.status = decision
			*target.by = strings.Join(reviewers, ",")
			*target.reason = strings.Join(reasons, " | ")
		}
	}
	return nil
}
