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

// AttachKnowledge adds only explicitly scoped, reviewed policy and ADR evidence.
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
	decisions, err := instances.SelectDecisions(asOf, map[string]string{"subsystem": subsystem}, false)
	if err != nil {
		return contract.EvidencePackV2{}, err
	}
	k := emptyKnowledgeContext(selected.State, lock.LockDigest)
	k.Unknowns = append(k.Unknowns, selected.Unknowns...)
	k.Unknowns = append(k.Unknowns, decisions.Unknowns...)
	for _, conflict := range selected.Conflicts {
		k.Conflicts = append(k.Conflicts, contract.KnowledgeConflictV2{
			LeftID: conflict.LeftID, RightID: conflict.RightID, Reason: conflict.Reason})
	}
	if decisions.State == "restricted" {
		k.State = "restricted"
		k.Conflicts = nil
	}
	if k.State == "restricted" || len(selected.Applicable)+len(decisions.Applicable) == 0 {
		base.Semantic = contract.KnowledgeSemanticV2{KnowledgeContext: k}
		if err := Stamp(&base); err != nil {
			return contract.EvidencePackV2{}, err
		}
		return base, nil
	}
	selectedIDs := map[string]bool{}
	for _, p := range selected.Applicable {
		selectedIDs[p.ID] = true
	}
	for _, d := range decisions.Applicable {
		selectedIDs[d.ID] = true
	}
	localRelations := []knowledgepack.RelationInstance{}
	if selected.State != "conflict" {
		for _, relation := range instances.Relations {
			if relation.Status == "verified" && relation.Visibility == "public" &&
				selectedIDs[relation.Subject.ID] && selectedIDs[relation.Object.ID] {
				localRelations = append(localRelations, relation)
			}
		}
	}
	if len(base.Citations)+len(selected.Applicable)+len(decisions.Applicable)+len(localRelations) > 12 {
		return knowledgeUnavailable(base, "knowledge_citation_budget", lock.LockDigest)
	}
	refs := make([]Ref, 0, len(selected.Applicable)+len(decisions.Applicable)+len(localRelations))
	appendRef := func(source knowledgepack.SourceRef) error {
		if source.OriginID != "repo" {
			return fmt.Errorf("unsupported knowledge origin")
		}
		buf, _, _, err := setup.ReadRetainedFile(versionDir, source.OriginID, source.Path)
		if err != nil {
			return err
		}
		lines := bytes.Count(buf, []byte{'\n'})
		if len(buf) > 0 && buf[len(buf)-1] != '\n' {
			lines++
		}
		if lines == 0 {
			return fmt.Errorf("empty knowledge source")
		}
		refs = append(refs, Ref{OriginID: source.OriginID,
			Citation: contract.Citation{File: source.Path, StartLine: 1, EndLine: lines}})
		return nil
	}
	for _, p := range selected.Applicable {
		if err := appendRef(p.SourceRef); err != nil {
			return knowledgeUnavailable(base, "knowledge_source_unavailable", lock.LockDigest)
		}
	}
	for _, d := range decisions.Applicable {
		if err := appendRef(d.SourceRef); err != nil {
			return knowledgeUnavailable(base, "knowledge_source_unavailable", lock.LockDigest)
		}
	}
	for _, relation := range localRelations {
		if err := appendRef(relation.SourceRef); err != nil {
			return knowledgeUnavailable(base, "knowledge_source_unavailable", lock.LockDigest)
		}
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
	unverifiedLinks := false
	for i, d := range decisions.Applicable {
		k.Decisions = append(k.Decisions, contract.KnowledgeDecisionV2{ID: d.ID, State: d.State,
			ReviewedBy: d.ReviewedBy, Date: d.Date, RequirementIDs: []string{},
			Citation: addition.Citations[len(selected.Applicable)+i]})
		if len(d.RequirementIDs) > 0 {
			// ADR front matter declares intent. Without a checked link to the
			// pinned semantic requirement, these IDs cannot enter the coding path.
			k.Unknowns = append(k.Unknowns, "unverified_adr_requirement_links")
			unverifiedLinks = true
		}
	}
	for i, relation := range localRelations {
		k.Relations = append(k.Relations, contract.KnowledgeRelationV2{
			ID: relation.ID, PackID: relation.Type.PackID, Predicate: relation.Type.LocalID,
			SubjectID: relation.Subject.ID, ObjectID: relation.Object.ID,
			ReviewedBy: relation.ReviewedBy,
			Citation:   addition.Citations[len(selected.Applicable)+len(decisions.Applicable)+i],
		})
	}
	if selected.State == "unknown" || selected.State == "stale" {
		if len(k.Decisions) > 0 {
			k.State = "partial"
		}
	}
	if k.State == "needs_citation" {
		k.State = "complete"
	}
	if unverifiedLinks && k.State == "complete" {
		k.State = "partial"
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
		Relations:   []contract.KnowledgeRelationV2{},
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
		if len(k.ApplicablePolicies) != 0 || len(k.Decisions) != 0 || len(k.Relations) != 0 || len(k.Conflicts) != 0 {
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
		if d.ID == "" || ids[d.ID] || d.State != "current" || d.ReviewedBy == "" || d.Date == "" ||
			!citations[d.Citation] || d.Citation.OriginID != "repo" {
			return fmt.Errorf("v2 knowledge decision lacks unique archive citation")
		}
		ids[d.ID] = true
	}
	for _, relation := range k.Relations {
		if relation.ID == "" || ids[relation.ID] || relation.PackID == "" || relation.Predicate == "" ||
			relation.ReviewedBy == "" || !ids[relation.SubjectID] || !ids[relation.ObjectID] ||
			!citations[relation.Citation] || relation.Citation.OriginID != "repo" {
			return fmt.Errorf("v2 knowledge relation lacks reviewed endpoints and archive citation")
		}
		ids[relation.ID] = true
	}
	for _, conflict := range k.Conflicts {
		if conflict.LeftID == conflict.RightID || !ids[conflict.LeftID] || !ids[conflict.RightID] || conflict.Reason == "" {
			return fmt.Errorf("v2 knowledge conflict has missing policy endpoints")
		}
	}
	if k.State == "conflict" && (len(k.Conflicts) == 0 || len(k.Relations) != 0) ||
		k.State == "complete" && len(k.ApplicablePolicies) == 0 && len(k.Decisions) == 0 {
		return fmt.Errorf("v2 knowledge state has no supporting records")
	}
	return nil
}
