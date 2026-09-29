package semantic

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestAnnotatePackPreservesLegacyIntegrityAndCitationScope(t *testing.T) {
	p := traceFixture(t)
	codeCitation := contract.Citation{File: "main.go", StartLine: 2, EndLine: 5, CommitHash: p.Snapshot.Commit}
	pack := contract.EvidencePack{Intent: contract.IntentUnknown, Query: "Alpha", Citations: []contract.Citation{codeCitation}}
	if err := contract.StampIntegrity(&pack); err != nil {
		t.Fatal(err)
	}
	baseHash := pack.Metadata.IntegrityHash
	if err := (ActiveProjection{projection: p}).AnnotatePack(&pack); err != nil {
		t.Fatal(err)
	}
	if pack.Semantic == nil || len(pack.Semantic.Links) != 1 || !pack.Semantic.Links[0].Unconfirmed ||
		pack.Semantic.Links[0].RequirementID != "req-alpha" || pack.Metadata.IntegrityHash != baseHash || !pack.IsValid() {
		t.Fatalf("incorrect annotation: %+v", pack.Semantic)
	}
	legacy := pack
	legacy.Semantic = nil
	if valid, err := contract.VerifyIntegrity(legacy); err != nil || !valid {
		t.Fatalf("legacy consumer hash changed: %v, %v", valid, err)
	}
	pack.Semantic.Links[0].RequirementID = "injected"
	if pack.IsValid() {
		t.Fatal("tampered semantic link passed digest check")
	}
}

func TestAnnotatePackRejectsMixedCommitsAndDoesNotAddCitations(t *testing.T) {
	p := traceFixture(t)
	other := contract.Citation{File: "other.go", StartLine: 1, EndLine: 2, CommitHash: p.Snapshot.Commit}
	pack := contract.EvidencePack{Intent: contract.IntentUnknown, Query: "Alpha", Citations: []contract.Citation{other}}
	if err := contract.StampIntegrity(&pack); err != nil {
		t.Fatal(err)
	}
	if err := (ActiveProjection{projection: p}).AnnotatePack(&pack); err != nil || pack.Semantic != nil || len(pack.Citations) != 1 {
		t.Fatalf("unrelated pack gained trace: %+v, %v", pack, err)
	}
	pack.Citations[0].CommitHash = "0000000000000000000000000000000000000000"
	if err := contract.StampIntegrity(&pack); err != nil {
		t.Fatal(err)
	}
	if err := (ActiveProjection{projection: p}).AnnotatePack(&pack); err == nil {
		t.Fatal("mixed commit accepted")
	}
}
