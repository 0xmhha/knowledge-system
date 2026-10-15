package comparison

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func reviewInstant(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return at
}
func fixtureMappingReview(t *testing.T, s *Scheme, b ProjectBinding, id, actor, decision, at string) MappingReview {
	t.Helper()
	digest, err := s.BindingDigest(b)
	if err != nil {
		t.Fatal(err)
	}
	return MappingReview{ID: id, BindingID: b.ID, Concept: b.Concept, Coordinates: b.Coordinates, Conditions: b.Conditions, BindingDigest: digest, Decision: decision, ReviewerID: actor, Reason: "Controlled policy test event; not a human approval.", ReviewedAt: at}
}
func TestDirectMappingMeaningNeverMergesEntitiesOrInfersClosure(t *testing.T) {
	for _, kind := range []string{"exact", "close", "broader", "narrower", "related"} {
		t.Run(kind, func(t *testing.T) {
			meaning, err := MappingSemantics(kind)
			if err != nil {
				t.Fatal(err)
			}
			if meaning.Kind != kind || meaning.Direction != "local_to_shared" || meaning.Meaning == "" || meaning.EntityIdentity || meaning.AutomaticTransitivity {
				t.Fatalf("unbounded mapping meaning: %+v", meaning)
			}
		})
	}
	if _, err := MappingSemantics("same-entity"); !errors.Is(err, ErrBindingInvalid) {
		t.Fatal("unknown entity equivalence accepted")
	}
	broader, _ := MappingSemantics("broader")
	narrower, _ := MappingSemantics("narrower")
	if !strings.Contains(broader.Meaning, "local role includes a broader") || !strings.Contains(narrower.Meaning, "local role covers a narrower") {
		t.Fatal("mapping direction reversed")
	}
}
func TestMappingReviewQuorumWithdrawalConflictAndTime(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	b := proposedBinding(t, s, v, refs, "staking", "Stake")
	a := fixtureMappingReview(t, s, b, "approve-a", "reviewer-a", "verified", "2026-10-16T01:00:00Z")
	other := fixtureMappingReview(t, s, b, "approve-b", "reviewer-b", "verified", "2026-10-16T01:10:00Z")
	reject := fixtureMappingReview(t, s, b, "reject-b", "reviewer-b", "rejected", "2026-10-16T01:20:00Z")
	withdraw := fixtureMappingReview(t, s, b, "withdraw-a", "reviewer-a", "withdrawn", "2026-10-16T02:00:00Z")
	laterReject := fixtureMappingReview(t, s, b, "reject-a", "reviewer-a", "rejected", "2026-10-16T03:00:00Z")
	tie := fixtureMappingReview(t, s, b, "tie-a", "reviewer-a", "rejected", a.ReviewedAt)
	future := fixtureMappingReview(t, s, b, "future-a", "reviewer-a", "verified", "2026-10-18T00:00:00Z")
	futureOpposite := future
	futureOpposite.ID = "future-reject"
	futureOpposite.ReviewerID = "reviewer-b"
	futureOpposite.Decision = "rejected"
	cases := []struct {
		name                          string
		min                           int
		records                       []MappingReview
		state, validity               string
		approvals, rejections, future int
	}{
		{"no invented review", 1, nil, "proposed", "unknown", 0, 0, 0},
		{"one recorded vote", 1, []MappingReview{a}, "verified", "current", 1, 0, 0},
		{"same actor duplicates not extra votes", 2, []MappingReview{a, a}, "proposed", "current", 1, 0, 0},
		{"second reviewer needed", 2, []MappingReview{a}, "proposed", "current", 1, 0, 0},
		{"two distinct labels meet quorum", 2, []MappingReview{a, other}, "verified", "current", 2, 0, 0},
		{"opposing reviewers", 1, []MappingReview{a, reject}, "disputed", "conflict", 1, 1, 0},
		{"only rejection", 1, []MappingReview{reject}, "rejected", "current", 0, 1, 0},
		{"withdraw own earlier vote", 1, []MappingReview{withdraw, a}, "withdrawn", "current", 0, 0, 0},
		{"withdraw drops two voter quorum", 2, []MappingReview{a, other, withdraw}, "proposed", "current", 1, 0, 0},
		{"withdraw does not erase another actor", 1, []MappingReview{a, other, withdraw}, "verified", "current", 1, 0, 0},
		{"later rejection supersedes own approval", 1, []MappingReview{laterReject, a}, "rejected", "current", 0, 1, 0},
		{"same timestamp conflicts", 1, []MappingReview{a, tie}, "disputed", "conflict", 0, 0, 0},
		{"future review cannot approve early", 1, []MappingReview{future}, "proposed", "unknown", 0, 0, 1},
		{"future opposite does not revoke now", 1, []MappingReview{a, futureOpposite}, "verified", "current", 1, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: c.min}, Records: c.records}
			result, err := s.AssessMappingReview(b, d, reviewInstant(t, "2026-10-17T00:00:00Z"))
			if err != nil {
				t.Fatal(err)
			}
			if result.State != c.state || result.Validity != c.validity || result.Approvals != c.approvals || result.Rejections != c.rejections || result.FutureReviewCount != c.future || result.ReviewerIdentityAuthenticated || result.IndependentQualityAcceptance {
				t.Fatalf("wrong recorded review state: %+v", result)
			}
		})
	}
}
func TestMappingReviewsAreExactPinsAndProjectLocal(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	b := proposedBinding(t, s, v, refs, "staking", "Stake")
	r := fixtureMappingReview(t, s, b, "approve-a", "reviewer-a", "verified", "2026-10-16T01:00:00Z")
	d := MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: 1}, Records: []MappingReview{r}}
	at := reviewInstant(t, "2026-10-17T00:00:00Z")
	changed := b
	changed.Reason = "A changed proposed interpretation requires fresh review."
	assessment, err := s.AssessMappingReview(changed, d, at)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.State != "proposed" || assessment.Validity != "stale" || assessment.StaleReviewCount != 1 {
		t.Fatal("changed interpretation inherited approval")
	}
	other, or := bindingFixture(t, "chain-b", "Stake")
	foreign := proposedBinding(t, s, other, or, "staking", "Stake")
	assessment, err = s.AssessMappingReview(foreign, d, at)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.State != "proposed" || assessment.StaleReviewCount != 0 {
		t.Fatal("same local ID pulled another project's review")
	}
	network := b
	network.Conditions.Network = "another-network"
	assessment, err = s.AssessMappingReview(network, d, at)
	if err != nil {
		t.Fatal(err)
	}
	if assessment.State != "proposed" || assessment.StaleReviewCount != 0 {
		t.Fatal("network authority crossed scopes")
	}
	forged := r
	forged.Coordinates = other.Coordinates
	d.Records = []MappingReview{forged}
	if _, err = s.AssessMappingReview(b, d, at); !errors.Is(err, ErrMappingReviewInvalid) {
		t.Fatal("current digest accepted with foreign header")
	}
}
func TestReviewedBindingChecksCurrentArchiveAndKeepsHistoricalVersion(t *testing.T) {
	ctx := context.Background()
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	old, oldrefs := bindingFixture(t, "chain-a", "Stake")
	newer, newrefs := bindingFixture(t, "chain-a", "Bond")
	reader, err := NewMappingReader(ctx, bindingCatalog(old, newer), s, bindingCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	}()
	b1 := proposedBinding(t, s, old, oldrefs, "staking", "Stake")
	b2 := proposedBinding(t, s, newer, newrefs, "staking", "Bond")
	review := fixtureMappingReview(t, s, b1, "approve-a", "controlled-reviewer", "verified", "2026-10-16T01:00:00Z")
	d := MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: 1}, Records: []MappingReview{review}}
	at := reviewInstant(t, "2026-10-17T00:00:00Z")
	current, err := reader.ReadReviewedBinding(ctx, "chains", b2, d, at)
	if err != nil {
		t.Fatal(err)
	}
	if current.Binding.ReviewState != "proposed" || current.Review.Validity != "stale" || !strings.Contains(current.Binding.Evidence[0].Body, "Bond") {
		t.Fatal("new snapshot inherited old review or old source")
	}
	historical, err := reader.ReadReviewedBinding(ctx, "chains", b1, d, at)
	if err != nil {
		t.Fatal(err)
	}
	if historical.Binding.ReviewState != "verified" || historical.Review.Validity != "current" || historical.Review.ReviewerIdentityAuthenticated || !strings.Contains(historical.Binding.Evidence[0].Body, "Stake") {
		t.Fatal("historical scoped approval/source lost")
	}
	blob := filepath.Join(old.VersionDir, "sources/blobs", oldrefs[0].Citation.FileSHA256)
	if err = os.Chmod(blob, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(blob, []byte("corrupt archive\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := reader.ReadReviewedBinding(ctx, "chains", b1, d, at)
	if err == nil || len(result.Binding.Evidence) != 0 {
		t.Fatal("recorded approval bypassed retained source integrity")
	}
}
func TestMappingReviewDoesNotMigrateAcrossPackVersions(t *testing.T) {
	root := copyScheme(t)
	old, err := LoadScheme(root)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	before := proposedBinding(t, old, v, refs, "staking", "Stake")
	r := fixtureMappingReview(t, old, before, "approve-a", "controlled-reviewer", "verified", "2026-10-16T01:00:00Z")
	d := MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: 1}, Records: []MappingReview{r}}
	data, err := os.ReadFile(filepath.Join(root, "pack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var pack map[string]any
	if err = json.Unmarshal(data, &pack); err != nil {
		t.Fatal(err)
	}
	pack["version"] = "2.0.0"
	data, err = json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "pack.yaml"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	editDefinition(t, root, func(d *SchemeDefinition) { d.Version = "2.0.0" })
	current, err := LoadScheme(root)
	if err != nil {
		t.Fatal(err)
	}
	after := proposedBinding(t, current, v, refs, "staking", "Stake")
	at := reviewInstant(t, "2026-10-17T00:00:00Z")
	result, err := current.AssessMappingReview(after, d, at)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "proposed" || result.Validity != "stale" {
		t.Fatal("new pack auto-migrated approval")
	}
	historical, err := old.AssessMappingReview(before, d, at)
	if err != nil || historical.State != "verified" {
		t.Fatal("immutable old vocabulary/review lost")
	}
}
func TestMappingReviewStrictSchemaAndNoMissingActor(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	b := proposedBinding(t, s, v, refs, "staking", "Stake")
	record := fixtureMappingReview(t, s, b, "approve-a", "controlled-reviewer", "verified", "2026-10-16T01:00:00Z")
	edits := map[string]func(*MappingReview){
		"no actor":               func(r *MappingReview) { r.ReviewerID = "" },
		"no reason":              func(r *MappingReview) { r.Reason = "" },
		"unknown decision":       func(r *MappingReview) { r.Decision = "assumed" },
		"bad time":               func(r *MappingReview) { r.ReviewedAt = "now" },
		"predates observation":   func(r *MappingReview) { r.ReviewedAt = "2026-10-15T00:00:00Z" },
		"bad target digest":      func(r *MappingReview) { r.BindingDigest = "not-pinned" },
		"no source pin":          func(r *MappingReview) { r.Coordinates.SnapshotID = "" },
		"invalid pack namespace": func(r *MappingReview) { r.Concept.PackID = "core" },
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			r := record
			edit(&r)
			d := MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: 1}, Records: []MappingReview{r}}
			data, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.DecodeMappingReviews(data); !errors.Is(err, ErrMappingReviewInvalid) {
				t.Fatal("invalid review admitted")
			}
		})
	}
	d := MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: 1}, Records: []MappingReview{record, record}}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecodeMappingReviews(data); err != nil {
		t.Fatal(err)
	}
	inconsistent := record
	inconsistent.Reason = "Changed bytes under same scoped event ID."
	d.Records = append(d.Records, inconsistent)
	data, err = json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecodeMappingReviews(data); !errors.Is(err, ErrMappingReviewInvalid) {
		t.Fatal("rewritten event accepted")
	}
	if _, err = s.AssessMappingReview(b, d, reviewInstant(t, "2026-10-17T00:00:00Z")); !errors.Is(err, ErrMappingReviewInvalid) {
		t.Fatal("typed API bypassed event identity")
	}
	for _, minimum := range []int{0, 9} {
		d.Policy.MinApprovals = minimum
		d.Records = nil
		data, err = json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.DecodeMappingReviews(data); err == nil {
			t.Fatal("unbounded quorum accepted")
		}
	}
}

func TestMappingReviewRejectsCaseAliases(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	v, refs := bindingFixture(t, "chain-a", "Stake")
	b := proposedBinding(t, s, v, refs, "staking", "Stake")
	record := fixtureMappingReview(t, s, b, "approve-a", "controlled-reviewer", "verified", "2026-10-16T01:00:00Z")
	d := MappingReviewDocument{SchemaVersion: 1, Policy: MappingReviewPolicy{MinApprovals: 1}, Records: []MappingReview{record}}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	samples := []string{
		strings.Replace(string(data), `"reviewer_id":"controlled-reviewer"`, `"reviewer_id":"controlled-reviewer","REVIEWER_ID":"another-reviewer"`, 1),
		strings.Replace(string(data), `"schema_version":1`, `"schema_version":1,"ſchema_version":1`, 1),
	}
	for _, sample := range samples {
		if _, err = s.DecodeMappingReviews([]byte(sample)); !errors.Is(err, ErrMappingReviewInvalid) {
			t.Fatal("case alias silently replaced a declared configuration field")
		}
	}
}
