package evidencev2

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// AttachKnowledge adds only explicitly scoped, reviewed policy evidence.
// Failure in the optional layer preserves the base CKV/CKG candidates and
// records uncertainty instead of substituting a policy-shaped answer.
func AttachKnowledge(ctx context.Context, base contract.EvidencePackV2, versionDir, asOf, subsystem string, cleaner *sanitize.Engine) (contract.EvidencePackV2, error) {
	if err := Verify(base); err != nil {
		return contract.EvidencePackV2{}, err
	}
	if cleaner == nil || asOf == "" || subsystem == "" {
		return contract.EvidencePackV2{}, fmt.Errorf("knowledge context needs date, subsystem and sanitizer")
	}
	if _, _, _, err := setup.ReadRetainedFile(versionDir, "repo", ".cks/knowledge/manifest.yaml"); err != nil {
		if strings.Contains(err.Error(), "citation file is not in the retained inventory") {
			return base, nil
		}
		return knowledgeUnavailable(base, "knowledge_archive_unavailable", "")
	}
	instances, lock, err := knowledgepack.LoadRetainedProjectInstances(versionDir)
	if err != nil {
		return knowledgeUnavailable(base, "knowledge_archive_unavailable", "")
	}
	selected, err := instances.SelectPolicies(asOf, map[string]string{"subsystem": subsystem}, false)
	if err != nil {
		return contract.EvidencePackV2{}, err
	}
	k := emptyKnowledgeContext(selected.State, lock.LockDigest)
	k.Unknowns = append(k.Unknowns, selected.Unknowns...)
	for _, conflict := range selected.Conflicts {
		k.Conflicts = append(k.Conflicts, contract.KnowledgeConflictV2{
			LeftID: conflict.LeftID, RightID: conflict.RightID, Reason: conflict.Reason})
	}
	if selected.State == "restricted" || len(selected.Applicable) == 0 {
		base.Semantic = contract.KnowledgeSemanticV2{KnowledgeContext: k}
		if err := Stamp(&base); err != nil {
			return contract.EvidencePackV2{}, err
		}
		return base, nil
	}
	if len(base.Citations)+len(selected.Applicable) > 12 {
		return knowledgeUnavailable(base, "knowledge_citation_budget", lock.LockDigest)
	}
	refs := make([]Ref, 0, len(selected.Applicable))
	for _, p := range selected.Applicable {
		if p.SourceRef.OriginID != "repo" {
			return knowledgeUnavailable(base, "unsupported_policy_origin", lock.LockDigest)
		}
		buf, _, _, err := setup.ReadRetainedFile(versionDir, p.SourceRef.OriginID, p.SourceRef.Path)
		if err != nil {
			return knowledgeUnavailable(base, "knowledge_source_unavailable", lock.LockDigest)
		}
		lines := bytes.Count(buf, []byte{'\n'})
		if len(buf) > 0 && buf[len(buf)-1] != '\n' {
			lines++
		}
		if lines == 0 {
			return knowledgeUnavailable(base, "knowledge_source_empty", lock.LockDigest)
		}
		refs = append(refs, Ref{OriginID: p.SourceRef.OriginID,
			Citation: contract.Citation{File: p.SourceRef.Path, StartLine: 1, EndLine: lines}})
	}
	addition, err := BuildFromRefs(ctx, versionDir, base.Query, refs, cleaner)
	if err != nil {
		return knowledgeUnavailable(base, "knowledge_citation_unavailable", lock.LockDigest)
	}
	if addition.Coordinates != base.Coordinates || len(addition.Bodies) != len(addition.Citations) {
		return knowledgeUnavailable(base, "knowledge_citation_incomplete", lock.LockDigest)
	}
	original := base
	seen := map[contract.CitationKeyV2]bool{}
	for _, c := range base.Citations {
		seen[c.KeyV2()] = true
	}
	for _, c := range addition.Citations {
		if !seen[c.KeyV2()] {
			base.Citations = append(base.Citations, c)
			seen[c.KeyV2()] = true
		}
	}
	for _, body := range addition.Bodies {
		found := false
		for _, existing := range base.Bodies {
			if existing.Citation == body.Citation {
				found = true
				break
			}
		}
		if !found {
			base.Bodies = append(base.Bodies, body)
		}
	}
	var bodyBytes int
	for _, body := range base.Bodies {
		bodyBytes += len(body.Text)
	}
	if bodyBytes > 32_000 {
		return knowledgeUnavailable(original, "knowledge_citation_budget", lock.LockDigest)
	}
	for i, p := range selected.Applicable {
		k.ApplicablePolicies = append(k.ApplicablePolicies, contract.KnowledgePolicyV2{
			ID: p.ID, State: p.State, ReviewedBy: p.ReviewedBy,
			EffectiveFrom: p.EffectiveFrom, EffectiveTo: p.EffectiveTo, Citation: addition.Citations[i]})
	}
	if k.State == "needs_citation" {
		k.State = "complete"
	}
	if base.EvidenceState == "partial" && k.State == "complete" {
		k.State = "partial"
	}
	base.Semantic = contract.KnowledgeSemanticV2{KnowledgeContext: k}
	if err := Stamp(&base); err != nil {
		return contract.EvidencePackV2{}, err
	}
	return base, nil
}

func emptyKnowledgeContext(state, lockDigest string) contract.KnowledgeContextV2 {
	return contract.KnowledgeContextV2{State: state, LockDigest: lockDigest,
		ApplicablePolicies: []contract.KnowledgePolicyV2{}, Decisions: []contract.KnowledgeDecisionV2{},
		Constraints: []string{}, RelatedRequirements: []string{}, TestLinks: []string{},
		Unknowns: []string{}, Conflicts: []contract.KnowledgeConflictV2{}}
}

func knowledgeUnavailable(base contract.EvidencePackV2, reason, lockDigest string) (contract.EvidencePackV2, error) {
	state := "unavailable"
	if reason == "knowledge_citation_budget" {
		state = "budget_exceeded"
	}
	k := emptyKnowledgeContext(state, lockDigest)
	k.Unknowns = append(k.Unknowns, reason)
	base.Semantic = contract.KnowledgeSemanticV2{KnowledgeContext: k}
	if err := Stamp(&base); err != nil {
		return contract.EvidencePackV2{}, err
	}
	return base, nil
}

func validateKnowledgeSemantic(value any, citations map[contract.CitationV2]bool) error {
	if value == nil {
		return nil
	}
	overlay, ok := value.(contract.KnowledgeSemanticV2)
	if !ok {
		return fmt.Errorf("v2 semantic overlay has unsupported type")
	}
	k := overlay.KnowledgeContext
	switch k.State {
	case "complete", "partial", "conflict", "unknown", "stale", "restricted", "budget_exceeded", "unavailable":
	default:
		return fmt.Errorf("v2 knowledge context has invalid state")
	}
	if (k.State != "unavailable" && k.LockDigest == "") || k.LockDigest != "" && !hexDigest(k.LockDigest) {
		return fmt.Errorf("v2 knowledge lock digest is invalid")
	}
	if k.State == "restricted" || k.State == "unavailable" {
		if len(k.ApplicablePolicies) != 0 || len(k.Decisions) != 0 || len(k.Conflicts) != 0 {
			return fmt.Errorf("v2 restricted or unavailable context exposes knowledge identifiers")
		}
	}
	ids := map[string]bool{}
	for _, p := range k.ApplicablePolicies {
		if p.ID == "" || ids[p.ID] || p.State != "current" || p.ReviewedBy == "" ||
			!citations[p.Citation] || p.Citation.OriginID != "repo" {
			return fmt.Errorf("v2 knowledge policy lacks unique reviewed archive citation")
		}
		ids[p.ID] = true
	}
	for _, d := range k.Decisions {
		if d.ID == "" || ids[d.ID] || d.State != "current" || !citations[d.Citation] {
			return fmt.Errorf("v2 knowledge decision lacks unique archive citation")
		}
		ids[d.ID] = true
	}
	for _, conflict := range k.Conflicts {
		if conflict.LeftID == conflict.RightID || !ids[conflict.LeftID] || !ids[conflict.RightID] || conflict.Reason == "" {
			return fmt.Errorf("v2 knowledge conflict has missing policy endpoints")
		}
	}
	if k.State == "conflict" && len(k.Conflicts) == 0 || k.State == "complete" && len(k.ApplicablePolicies) == 0 && len(k.Decisions) == 0 {
		return fmt.Errorf("v2 knowledge state has no supporting records")
	}
	return nil
}
