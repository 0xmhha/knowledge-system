package semantic

import (
	"strings"
	"testing"
)

func TestReviewReportsAdjudicationSeparatelyFromQueue(t *testing.T) {
	p, _ := fixture(t)
	// Unreviewed precision is unknown, not a perfect or failed score.
	r, err := p.Review(5, "audit-1")
	if err != nil || r.Precision != nil || r.Proposed != 1 || len(r.Sample) != 1 {
		t.Fatalf("unreviewed report = %+v, err = %v", r, err)
	}
	if got := r.Sample[0].Evidence[0].Path; got != "guide.md" {
		t.Fatalf("sample lost source path: %q", got)
	}
	for i := 0; i < 8; i++ {
		claim := p.Claims[0]
		claim.ID = "candidate-" + string(rune('a'+i))
		p.Claims = append(p.Claims, claim)
	}
	a, err := p.Review(3, "audit-1")
	if err != nil {
		t.Fatal(err)
	}
	// Input order must not change the sample for a fixed dataset and salt.
	for i, j := 0, len(p.Claims)-1; i < j; i, j = i+1, j-1 {
		p.Claims[i], p.Claims[j] = p.Claims[j], p.Claims[i]
	}
	b, err := p.Review(3, "audit-1")
	if err != nil {
		t.Fatal(err)
	}
	for i := range a.Sample {
		if a.Sample[i].ClaimID != b.Sample[i].ClaimID {
			t.Fatalf("sample changed with input order: %v vs %v", a.Sample, b.Sample)
		}
	}
	p.Claims[0].Status, p.Claims[0].ReviewedBy = StatusVerified, "human"
	p.Claims[1].Status, p.Claims[1].ReviewedBy = StatusRejected, "human"
	r, err = p.Review(20, "audit-1")
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 9 || r.Proposed != 7 || r.Verified != 1 || r.Rejected != 1 ||
		r.Precision == nil || *r.Precision != 0.5 || len(r.Sample) != 7 {
		t.Fatalf("adjudicated report = %+v", r)
	}
	p.Claims[1].ReviewedBy = ""
	if _, err := p.Review(20, "audit-1"); err == nil || !strings.Contains(err.Error(), "reviewed_by") {
		t.Fatalf("unattributed rejection accepted: %v", err)
	}
}
