package maintenance

import (
	"encoding/json"
	"slices"
	"time"
)

var terminalStates = []string{"held_for_review", "succeeded", "failed", "canceled", "interrupted"}
var stateTransitions = map[string][]string{
	"queued":           {"running", "cancel_requested", "interrupted", "recovering"},
	"running":          {"cancel_requested", "held_for_review", "succeeded", "failed", "interrupted", "recovering"},
	"cancel_requested": {"canceled", "failed", "interrupted", "recovering"},
	"recovering":       {"held_for_review", "succeeded", "failed", "canceled", "interrupted"},
	"held_for_review":  {}, "succeeded": {}, "failed": {}, "canceled": {}, "interrupted": {},
}
var statusPhases = []string{"accepted", "capture", "build_graph", "build_vector", "build_knowledge", "build_semantic", "gate", "publish", "reconcile", "finished"}
var gateNames = []string{"source", "identity", "graph", "vector", "knowledge", "semantic"}
var errorCodes = []string{"invalid_request", "profile_unknown", "profile_stale", "project_mismatch", "source_stale", "input_stale", "embedding_stale", "base_stale", "version_exists", "idempotency_conflict", "writer_busy", "unsupported_capability", "job_unknown", "cancel_too_late", "reconcile_required", "gate_failed", "deadline_exceeded", "canceled", "interrupted", "operation_failed"}

func equalJSON(a, b any) bool {
	left, lerr := json.Marshal(a)
	right, rerr := json.Marshal(b)
	return lerr == nil && rerr == nil && string(left) == string(right)
}

// ValidateStatus checks stored or emitted state without inferring publication
// from candidate files. The same independent design fixtures also test this.
func ValidateStatus(s JobStatus, r BuildRequest, p ProfileContext) error {
	bad := func() error { return reject("invalid_status") }
	digest, err := Digest(r, "cks.dataset-build.v1")
	_, knownState := stateTransitions[s.State]
	if err != nil || r.Validate() != nil || s.SchemaVersion != 1 || !identifier.MatchString(s.JobID) ||
		s.ProjectID != r.ProjectID || r.ProjectID != p.ProjectID || s.ProfileID != r.ProfileID || r.ProfileID != p.ProfileID ||
		s.RequestDigest != digest || s.TargetVersion != r.NewVersion || s.Revision < 1 || !knownState ||
		!slices.Contains(statusPhases, s.Phase) || s.ServingAdoption != "not_observed" || len(s.Gates) != len(gateNames) ||
		s.Terminal != slices.Contains(terminalStates, s.State) || s.Terminal != (s.Phase == "finished") ||
		s.Activated != (s.State == "succeeded") {
		return bad()
	}
	for _, name := range []string{"source", "identity", "graph", "vector"} {
		if !slices.Contains(p.RequiredGates, name) {
			return bad()
		}
	}
	for _, name := range gateNames {
		value, exists := s.Gates[name]
		if !exists || !slices.Contains([]string{"not_run", "passed", "failed", "not_configured"}, value) ||
			(value == "not_configured" && slices.Contains(p.RequiredGates, name)) ||
			(s.BuildComplete && value != "passed" && value != "not_configured") ||
			(s.State == "queued" && value != "not_run" && value != "not_configured") {
			return bad()
		}
	}
	if (s.State == "queued" && (s.Phase != "accepted" || s.BuildComplete || s.Candidate != nil)) ||
		(s.State == "recovering" && s.Phase != "reconcile") || (s.BuildComplete && s.Candidate == nil) {
		return bad()
	}
	if c := s.Candidate; c != nil {
		pin := r.SourcePin
		if !validCoordinates(c.Coordinates) || c.Version != r.NewVersion || c.Coordinates.ProjectID != r.ProjectID ||
			c.Coordinates.SnapshotID != pin.SnapshotID || c.Coordinates.SourceMode != pin.Mode || c.Coordinates.BaseCommit != pin.BaseCommit ||
			c.InputDigest != r.InputDigest || c.EmbeddingDigest != r.EmbeddingDigest {
			return bad()
		}
	}
	if s.State == "succeeded" {
		publication := s.Publication
		if r.Activation != "activate_after_gate" || !s.BuildComplete || publication == nil ||
			!equalJSON(publication.Base, r.Base) || publication.Version != s.Candidate.Version || publication.Coordinates != s.Candidate.Coordinates {
			return bad()
		}
		if _, err := time.Parse(time.RFC3339Nano, publication.PublishedAt); err != nil {
			return bad()
		}
		body := map[string]any{"base": publication.Base, "version": publication.Version, "coordinates": publication.Coordinates, "published_at": publication.PublishedAt}
		digest, err := Digest(body, "cks.dataset-publication.v1")
		if err != nil || digest != publication.ReceiptDigest {
			return bad()
		}
	} else if s.Publication != nil {
		return bad()
	}
	if s.State == "held_for_review" && (r.Activation != "hold_for_review" || !s.BuildComplete) {
		return bad()
	}
	if slices.Contains([]string{"failed", "canceled", "interrupted"}, s.State) {
		if s.Error == nil || !slices.Contains(errorCodes, s.Error.Code) || !slices.Contains(statusPhases, s.Error.Phase) ||
			((s.State == "canceled" || s.State == "interrupted") && s.Error.Code != s.State) {
			return bad()
		}
	} else if s.Error != nil {
		return bad()
	}
	return nil
}

func validateTransition(before, after string) error {
	next, ok := stateTransitions[before]
	_, afterKnown := stateTransitions[after]
	if !ok || !afterKnown || (before != after && !slices.Contains(next, after)) {
		return reject("invalid_transition")
	}
	return nil
}

// Journal updates observe every persisted transition; polling clients may skip
// intermediate revisions and must not apply this immediate-transition check.
func validateJournalUpdate(before, after JobStatus) error {
	if before.JobID != after.JobID || before.ProjectID != after.ProjectID || before.ProfileID != after.ProfileID ||
		before.RequestDigest != after.RequestDigest || before.TargetVersion != after.TargetVersion || after.Revision < before.Revision ||
		((before.Revision == after.Revision || before.Terminal) && !equalJSON(before, after)) {
		return reject("invalid_observation")
	}
	return validateTransition(before.State, after.State)
}
