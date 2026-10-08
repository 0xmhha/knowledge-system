package evidencev2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestBudgetSkipsWholeOversizeSpanAndKeepsLaterEvidence(t *testing.T) {
	_, version := testVersion(t, strings.Repeat("x", 33000)+"\nsmall\r\n")
	pack, err := Build(context.Background(), version, "independent structure DEV", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}, {File: "README.md", StartLine: 2, EndLine: 2}}, testCleaner(t))
	if err != nil {
		t.Fatalf("fitting retained evidence lost: %v", err)
	}
	if pack.EvidenceState != "partial" || len(pack.Citations) != 1 || len(pack.Bodies) != 1 || pack.Citations[0].StartLine != 2 || pack.Citations[0].EndLine != 2 || pack.Bodies[0].Text != "small\r\n" {
		t.Fatalf("truncated or incorrect selection: %+v", pack)
	}
	sum := sha256.Sum256([]byte("small\r\n"))
	if pack.Citations[0].ContentSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("rewrote original hash")
	}
	if err := Verify(pack); err != nil {
		t.Fatal(err)
	}
}

func TestBudgetCitationSelectionKeepsRankAndEntireSpans(t *testing.T) {
	_, version := testVersion(t, strings.Repeat("small\n", 13))
	var refs []contract.Citation
	for line := 1; line <= 13; line++ {
		refs = append(refs, contract.Citation{File: "README.md", StartLine: line, EndLine: line})
	}
	pack, err := Build(context.Background(), version, "independent citation DEV", refs, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	if pack.EvidenceState != "partial" || len(pack.Citations) != 12 || len(pack.Bodies) != 12 {
		t.Fatalf("citation bound: %+v", pack)
	}
	for i, c := range pack.Citations {
		if c.StartLine != i+1 || c.EndLine != i+1 || pack.Bodies[i].Text != "small\n" {
			t.Fatal("rank/span changed")
		}
	}
}

func TestBudgetExactByteBoundaryDuplicateZeroAndTypedOverflow(t *testing.T) {
	for _, size := range []int{32000, 32001} {
		t.Run(fmtSize(size), func(t *testing.T) {
			_, version := testVersion(t, strings.Repeat("가", (size-1)/3)+strings.Repeat("x", (size-1)%3)+"\n")
			ref := contract.Citation{File: "README.md", StartLine: 1, EndLine: 1}
			pack, err := Build(context.Background(), version, "byte DEV", []contract.Citation{ref, ref}, testCleaner(t))
			if size == 32001 {
				if !errors.Is(err, ErrEvidenceBudget) {
					t.Fatalf("untyped over-budget error: %v", err)
				}
				return
			}
			if err != nil || len(pack.Bodies) != 1 || len(pack.Bodies[0].Text) != size || pack.EvidenceState != "complete" || pack.Metadata.EvidenceBudget != nil {
				t.Fatalf("exact boundary/dedup: %+v %v", pack, err)
			}
			zero, err := Build(context.Background(), version, "empty DEV", nil, testCleaner(t))
			if err != nil || zero.EvidenceState != "complete" || len(zero.Citations) != 0 {
				t.Fatal(err)
			}
		})
	}
	_, version := testVersion(t, "small\n")
	refs := make([]contract.Citation, 129)
	_, err := Build(context.Background(), version, "admission", refs, testCleaner(t))
	if !errors.Is(err, ErrEvidenceBudget) {
		t.Fatal(err)
	}
}
func fmtSize(n int) string {
	if n == 32000 {
		return "exact"
	}
	return "one-byte-over"
}

func TestBudgetPostSanitizationExpansionAndIntegrity(t *testing.T) {
	rules, err := config.LoadSanitizeRulesetBytes([]byte("version: 1\nrules:\n  - id: mask\n    description: expand\n    pattern: A\n    action: mask\n    severity: high\n"))
	if err != nil {
		t.Fatal(err)
	}
	cleaner, err := sanitize.New(rules)
	if err != nil {
		t.Fatal(err)
	}
	_, version := testVersion(t, strings.Repeat("A", 15000)+"\nsafe\n")
	pack, err := Build(context.Background(), version, "mask DEV", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}, {File: "README.md", StartLine: 2, EndLine: 2}}, cleaner)
	if err != nil || len(pack.Bodies) != 1 || pack.Bodies[0].Text != "safe\n" || pack.Metadata.EvidenceBudget == nil {
		t.Fatalf("mask expansion leaked over budget: %+v %v", pack, err)
	}
	b := pack.Metadata.EvidenceBudget
	if b.RequestedReferences != 2 || b.UniqueReferences != 2 || b.SelectedCitations != 1 || b.OmittedReferences != 1 || b.SelectedRawBytes != 5 || b.SelectedBodyBytes != 5 {
		t.Fatalf("invalid diagnostic: %+v", b)
	}
	if err := Verify(pack); err != nil {
		t.Fatal(err)
	}
	b.OmittedReferences++
	if err := Verify(pack); err == nil {
		t.Fatal("budget tamper retained integrity")
	}
}

func TestBudgetOmissionCannotHideForeignMissingOrCorruptedEvidence(t *testing.T) {
	for _, mode := range []string{"foreign", "missing", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			_, version := testVersion(t, strings.Repeat("x", 33000)+"\nsafe\n")
			refs := []contract.Citation{{File: "README.md", StartLine: 2, EndLine: 2}, {File: "README.md", StartLine: 1, EndLine: 1}}
			code := "snapshot_mismatch"
			switch mode {
			case "foreign":
				refs[1].CommitHash = strings.Repeat("a", 40)
			case "missing":
				refs[1].File = "missing.md"
				code = "source_missing"
			case "corrupt":
				entries, _ := os.ReadDir(filepath.Join(version, "sources", "blobs"))
				path := filepath.Join(version, "sources", "blobs", entries[0].Name())
				os.Chmod(path, 0600)
				os.WriteFile(path, []byte("tampered"), 0600)
			}
			_, err := Build(context.Background(), version, "scope DEV", refs, testCleaner(t))
			if err == nil || !strings.Contains(err.Error(), code) || errors.Is(err, ErrEvidenceBudget) {
				t.Fatalf("budget hid source failure: %v", err)
			}
		})
	}
}

func TestBudgetStampEnforcesGlobalBodyLimit(t *testing.T) {
	_, version := testVersion(t, "safe\n")
	pack, err := Build(context.Background(), version, "stamp DEV", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	pack.Bodies[0].Text = strings.Repeat("x", 32001)
	if err := Stamp(&pack); !errors.Is(err, ErrEvidenceBudget) {
		t.Fatalf("restamped oversize body: %v", err)
	}
}

func TestBudgetSecretDropRemainsPartialWithoutBudgetError(t *testing.T) {
	_, version := testVersion(t, "SECRET=private\nsafe\n")
	pack, err := Build(context.Background(), version, "secret DEV", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}, {File: "README.md", StartLine: 2, EndLine: 2}}, testCleaner(t))
	if err != nil || pack.EvidenceState != "partial" || len(pack.Citations) != 2 || len(pack.Bodies) != 1 || pack.Metadata.EvidenceBudget != nil || pack.Bodies[0].Text != "safe\n" {
		t.Fatalf("mixed secret and budget denominators: %+v %v", pack, err)
	}
}

func TestBudgetOversizeCannotHideFailClosedSanitizer(t *testing.T) {
	rules, err := config.LoadSanitizeRulesetBytes([]byte("version: 1\nrules:\n  - id: block\n    description: block\n    pattern: SECRET\n    action: fail_closed\n    severity: high\n"))
	if err != nil {
		t.Fatal(err)
	}
	cleaner, err := sanitize.New(rules)
	if err != nil {
		t.Fatal(err)
	}
	_, version := testVersion(t, "SECRET "+strings.Repeat("x", 33000)+"\nsafe\n")
	_, err = Build(context.Background(), version, "fail-closed DEV", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}, {File: "README.md", StartLine: 2, EndLine: 2}}, cleaner)
	if err == nil || !strings.Contains(err.Error(), "fail_closed") || errors.Is(err, ErrEvidenceBudget) {
		t.Fatalf("budget hid sanitizer policy: %v", err)
	}
}
