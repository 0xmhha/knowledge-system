package knowledgepack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validPack(id string) Pack {
	return Pack{PackSchemaVersion: 1, PackID: id, Version: "1.0.0", Owner: "project",
		Scope: "fixture", CompetencyQuestions: []string{"Which rule applies?"},
		Concepts: []ConceptType{{ID: "business-policy", Kind: "rule", Definition: "A project rule."}}}
}

func TestResolveRejectsCoreRedefinitionMissingAndCycle(t *testing.T) {
	a := validPack("engineering.decisions")
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	a.Concepts[0].ID = "policy"
	if err := a.Validate(); err != nil {
		t.Fatalf("pack-local policy should remain distinct from core policy: %v", err)
	}
	a.Concepts[0].ID = "business-policy"
	b := validPack("industry.blockchain")
	aDigest, bDigest := strings.Repeat("a", 64), strings.Repeat("b", 64)
	b.Requires = []Dependency{{PackID: a.PackID, Version: a.Version, Digest: aDigest}}
	ordered, err := Resolve([]LoadedPack{{Pack: a, Digest: aDigest}, {Pack: b, Digest: bDigest}})
	if err != nil || len(ordered) != 2 || ordered[0].Pack.PackID != a.PackID {
		t.Fatalf("dependency-first order: %+v, %v", ordered, err)
	}
	if _, err := Resolve([]LoadedPack{{Pack: b, Digest: bDigest}}); err == nil || !strings.Contains(err.Error(), "dependency") {
		t.Fatalf("missing dependency accepted: %v", err)
	}
	a.Requires = []Dependency{{PackID: b.PackID, Version: b.Version, Digest: bDigest}}
	if _, err := Resolve([]LoadedPack{{Pack: a, Digest: aDigest}, {Pack: b, Digest: bDigest}}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("dependency cycle accepted: %v", err)
	}
	b.Requires = nil
	b.RelationTypes = []RelationType{{Predicate: "governs", SubjectType: TypeRef{PackID: "core", LocalID: "policy"},
		ObjectType: TypeRef{PackID: "industry.blockchain", LocalID: "business-policy"}, Direction: "forward",
		Cardinality: "many-to-many", RequiredEvidence: true, ReviewRule: "human"}}
	if _, err := Resolve([]LoadedPack{{Pack: b, Digest: bDigest}}); err != nil {
		t.Fatal(err)
	}
	b.RelationTypes[0].SubjectType.LocalID = "missing"
	if _, err := Resolve([]LoadedPack{{Pack: b, Digest: bDigest}}); err == nil {
		t.Fatal("unknown relation endpoint accepted")
	}
	b.RelationTypes = nil
	b.Concepts = []ConceptType{
		{ID: "first", Kind: "entity", Definition: "First", Extends: &TypeRef{PackID: b.PackID, LocalID: "second"}},
		{ID: "second", Kind: "entity", Definition: "Second", Extends: &TypeRef{PackID: b.PackID, LocalID: "first"}},
	}
	if _, err := Resolve([]LoadedPack{{Pack: b, Digest: bDigest}}); err == nil || !strings.Contains(err.Error(), "inheritance cycle") {
		t.Fatalf("concept inheritance cycle accepted: %v", err)
	}
}

func TestLockPinsPackAndOverlayBytes(t *testing.T) {
	root := t.TempDir()
	overlay := filepath.Join(root, ".cks", "knowledge")
	packRoot := filepath.Join(root, "vendor", "engineering-decisions")
	for _, dir := range []string{overlay, packRoot, filepath.Join(overlay, "domain")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	packYAML := `pack_schema_version: 1
pack_id: engineering.decisions
version: 1.0.0
owner: project
scope: fixture
requires: []
concepts:
  - id: business-policy
    kind: rule
    definition: A project rule.
relation_types: []
constraints: []
competency_questions: ["Which rule applies?"]
`
	if err := os.WriteFile(filepath.Join(packRoot, "pack.yaml"), []byte(packYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := DigestTree(packRoot)
	if err != nil {
		t.Fatal(err)
	}
	manifest := `schema_version: 1
project_id: fixture
selected_packs:
  - pack_id: engineering.decisions
    version: 1.0.0
    source: ./vendor/engineering-decisions
    sha256: ` + digest + `
overlay_root: .cks/knowledge
`
	if err := os.WriteFile(filepath.Join(overlay, "manifest.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overlay, "domain", "fixture.yaml"), []byte("status: proposed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := BuildLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packs) != 1 || lock.Packs[0].OriginID != "knowledge:engineering.decisions" || lock.LockDigest == "" {
		t.Fatalf("invalid lock: %+v", lock)
	}
	data, _ := json.Marshal(lock)
	if err := os.WriteFile(filepath.Join(overlay, "knowledge.lock.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyLock(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overlay, "domain", "fixture.yaml"), []byte("status: verified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyLock(root); err == nil || !strings.Contains(err.Error(), "pack_lock_mismatch") {
		t.Fatalf("changed overlay accepted: %v", err)
	}
	if err := os.Symlink(filepath.Join(packRoot, "pack.yaml"), filepath.Join(packRoot, "linked.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := DigestTree(packRoot); err == nil || !strings.Contains(err.Error(), "linked") {
		t.Fatalf("linked pack source accepted: %v", err)
	}
}
