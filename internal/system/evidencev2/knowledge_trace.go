package evidencev2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// AttachVerifiedTraces adds at most two reviewed, exact external paths to an
// already cited knowledge response. The caller must supply an ActiveProjection
// returned by LoadAlignedRetained for this response's pinned dataset.
func AttachVerifiedTraces(ctx context.Context, base contract.EvidencePackV2, versionDir, asOf, subsystem string, projection semantic.ActiveProjection, cleaner *sanitize.Engine) (contract.EvidencePackV2, error) {
	if err := Verify(base); err != nil {
		return contract.EvidencePackV2{}, err
	}
	k, ok := base.Semantic.(contract.KnowledgeSemanticV2)
	if !ok || cleaner == nil || asOf == "" || subsystem == "" {
		return base, nil
	}
	k, err := cloneKnowledgeSemantic(k)
	if err != nil {
		return contract.EvidencePackV2{}, err
	}
	if k.KnowledgeContext.State == "restricted" || k.KnowledgeContext.State == "conflict" || k.KnowledgeContext.State == "unavailable" {
		return base, nil
	}
	if snapshot := projection.Snapshot(); snapshot.ProjectID != base.Coordinates.ProjectID || snapshot.DatasetID != base.Coordinates.DatasetID || snapshot.SnapshotID != base.Coordinates.SnapshotID {
		return traceUnknown(base, "trace_snapshot_mismatch")
	}
	instances, lock, err := knowledgepack.LoadRetainedProjectInstances(versionDir)
	if err != nil || lock.LockDigest != k.KnowledgeContext.LockDigest {
		return traceUnknown(base, "trace_archive_unavailable")
	}
	decisions, err := instances.SelectDecisions(asOf, map[string]string{"subsystem": subsystem}, false)
	if err != nil || decisions.State == "restricted" {
		return traceUnknown(base, "trace_scope_unavailable")
	}
	selected := map[string]knowledgepack.DecisionSummary{}
	for _, decision := range decisions.Applicable {
		selected[decision.ID] = decision
	}
	links := append([]knowledgepack.TraceLink{}, instances.TraceLinks...)
	sort.Slice(links, func(i, j int) bool { return links[i].ID < links[j].ID })
	type resolved struct {
		link    knowledgepack.TraceLink
		anchors semantic.TraceAnchors
		refs    [5]Ref
	}
	chosen := []resolved{}
	unverified := false
	for _, link := range links {
		if link.Status != "verified" || link.Visibility != "public" {
			continue
		}
		decision, exists := selected[link.DecisionID]
		if !exists || !containsString(decision.RequirementIDs, link.RequirementID) {
			unverified = true
			continue
		}
		anchors, err := projection.ResolveTracePath(link.RequirementID, link.CriterionID, link.CodeCanonicalID, link.TestCanonicalID)
		if err != nil {
			unverified = true
			continue
		}
		linkRef, err := traceSourceRef(versionDir, link.SourceRef)
		if err != nil {
			unverified = true
			continue
		}
		chosen = append(chosen, resolved{link: link, anchors: anchors,
			refs: [5]Ref{linkRef, traceSpanRef(anchors.Requirement), traceSpanRef(anchors.Criterion), traceSpanRef(anchors.Code), traceSpanRef(anchors.Test)}})
	}
	if len(chosen) == 0 {
		if unverified {
			return traceUnknown(base, "trace_anchor_unverified")
		}
		return base, nil
	}
	if len(chosen) > 2 || len(base.Citations)+5*len(chosen) > 12 {
		return traceUnknown(base, "trace_citation_budget")
	}
	refs := make([]Ref, 0, 5*len(chosen))
	for _, item := range chosen {
		refs = append(refs, item.refs[:]...)
	}
	addition, err := BuildFromRefs(ctx, versionDir, base.Query, refs, cleaner)
	if err != nil || addition.Coordinates != base.Coordinates {
		return traceUnknown(base, "trace_citation_unavailable")
	}
	all := append(append([]contract.CitationV2{}, base.Citations...), addition.Citations...)
	availableBodies := map[contract.CitationV2]bool{}
	for _, body := range append(append([]contract.BodyV2{}, base.Bodies...), addition.Bodies...) {
		availableBodies[body.Citation] = true
	}
	claims := make([]contract.KnowledgeTraceLinkV2, 0, len(chosen))
	for _, item := range chosen {
		citations := [5]contract.CitationV2{}
		for index, ref := range item.refs {
			match := false
			for _, citation := range all {
				if citation.OriginID != ref.OriginID || citation.File != ref.Citation.File || citation.StartLine != ref.Citation.StartLine || citation.EndLine != ref.Citation.EndLine || !availableBodies[citation] {
					continue
				}
				if index > 0 && citation.ContentSHA256 != []semantic.EvidenceSpan{item.anchors.Requirement, item.anchors.Criterion, item.anchors.Code, item.anchors.Test}[index-1].ContentSHA256 {
					continue
				}
				citations[index], match = citation, true
				break
			}
			if !match {
				return traceUnknown(base, "trace_citation_unverified")
			}
		}
		claims = append(claims, contract.KnowledgeTraceLinkV2{ID: item.link.ID, DecisionID: item.link.DecisionID,
			RequirementID: item.link.RequirementID, CriterionID: item.link.CriterionID,
			CodeCanonicalID: item.link.CodeCanonicalID, TestCanonicalID: item.link.TestCanonicalID,
			ReviewedBy: item.link.ReviewedBy, LinkCitation: citations[0], RequirementCitation: citations[1],
			CriterionCitation: citations[2], CodeCitation: citations[3], TestCitation: citations[4]})
	}
	seen := map[contract.CitationKeyV2]bool{}
	for _, citation := range base.Citations {
		seen[citation.KeyV2()] = true
	}
	for _, citation := range addition.Citations {
		if !seen[citation.KeyV2()] {
			base.Citations = append(base.Citations, citation)
			seen[citation.KeyV2()] = true
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
	bytesTotal := 0
	for _, body := range base.Bodies {
		bytesTotal += len(body.Text)
	}
	if bytesTotal > 32_000 || len(base.Citations) > 12 {
		return traceUnknown(base, "trace_citation_budget")
	}
	k.KnowledgeContext.TraceLinks = claims
	for _, claim := range claims {
		for index := range k.KnowledgeContext.Decisions {
			if k.KnowledgeContext.Decisions[index].ID == claim.DecisionID && !containsString(k.KnowledgeContext.Decisions[index].RequirementIDs, claim.RequirementID) {
				k.KnowledgeContext.Decisions[index].RequirementIDs = append(k.KnowledgeContext.Decisions[index].RequirementIDs, claim.RequirementID)
			}
		}
		if !containsString(k.KnowledgeContext.RelatedRequirements, claim.RequirementID) {
			k.KnowledgeContext.RelatedRequirements = append(k.KnowledgeContext.RelatedRequirements, claim.RequirementID)
		}
		if !containsString(k.KnowledgeContext.TestLinks, claim.TestCanonicalID) {
			k.KnowledgeContext.TestLinks = append(k.KnowledgeContext.TestLinks, claim.TestCanonicalID)
		}
		k.CodingContext.ImplementedBehavior = append(k.CodingContext.ImplementedBehavior,
			contract.KnowledgeReferenceV2{ID: claim.CodeCanonicalID, State: "reviewed_trace", Citation: claim.CodeCitation})
	}
	k.CodingContext.Evidence = append([]contract.CitationV2{}, base.Citations...)
	k.CodingContext.Unknowns = withoutString(k.CodingContext.Unknowns, "implementation_link_unverified")
	if unverified {
		k.KnowledgeContext.Unknowns = append(k.KnowledgeContext.Unknowns, "some_trace_anchors_unverified")
		k.CodingContext.Unknowns = append(k.CodingContext.Unknowns, "some_trace_anchors_unverified")
		k.KnowledgeContext.State = "partial"
	}
	base.Semantic = k
	if err := Stamp(&base); err != nil {
		return contract.EvidencePackV2{}, fmt.Errorf("stamp verified trace: %w", err)
	}
	return base, nil
}

func traceSourceRef(versionDir string, source knowledgepack.SourceRef) (Ref, error) {
	if source.OriginID != "repo" {
		return Ref{}, fmt.Errorf("unsupported trace origin")
	}
	buf, _, _, err := setup.ReadRetainedFile(versionDir, source.OriginID, source.Path)
	if err != nil {
		return Ref{}, err
	}
	lines := bytes.Count(buf, []byte{'\n'})
	if len(buf) > 0 && buf[len(buf)-1] != '\n' {
		lines++
	}
	if lines == 0 {
		return Ref{}, fmt.Errorf("empty trace source")
	}
	return Ref{OriginID: "repo", Citation: contract.Citation{File: source.Path, StartLine: 1, EndLine: lines}}, nil
}

func traceSpanRef(span semantic.EvidenceSpan) Ref {
	return Ref{OriginID: "repo", Citation: contract.Citation{File: span.Path, StartLine: span.StartLine, EndLine: span.EndLine}}
}

func traceUnknown(base contract.EvidencePackV2, reason string) (contract.EvidencePackV2, error) {
	k, ok := base.Semantic.(contract.KnowledgeSemanticV2)
	if !ok {
		return base, nil
	}
	k, err := cloneKnowledgeSemantic(k)
	if err != nil {
		return contract.EvidencePackV2{}, err
	}
	k.KnowledgeContext.Unknowns = append(k.KnowledgeContext.Unknowns, reason)
	k.CodingContext.Unknowns = append(k.CodingContext.Unknowns, reason)
	if k.KnowledgeContext.State == "complete" {
		k.KnowledgeContext.State = "partial"
	}
	base.Semantic = k
	if err := Stamp(&base); err != nil {
		return contract.EvidencePackV2{}, err
	}
	return base, nil
}

func cloneKnowledgeSemantic(value contract.KnowledgeSemanticV2) (contract.KnowledgeSemanticV2, error) {
	buf, err := json.Marshal(value)
	if err != nil {
		return contract.KnowledgeSemanticV2{}, err
	}
	var copied contract.KnowledgeSemanticV2
	if err := json.Unmarshal(buf, &copied); err != nil {
		return contract.KnowledgeSemanticV2{}, err
	}
	return copied, nil
}

func withoutString(values []string, target string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}
