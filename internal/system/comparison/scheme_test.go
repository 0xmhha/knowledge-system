package comparison

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shippedRoot = "../../../system/packs/industry-blockchain"

func copyScheme(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"pack.yaml", "comparison.json"} {
		data, err := os.ReadFile(filepath.Join(shippedRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func editDefinition(t *testing.T, root string, edit func(*SchemeDefinition)) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d SchemeDefinition
	if err = json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	edit(&d)
	data, err = json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "comparison.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
func TestShippedSchemePinsQuestionsAndDefinitions(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	concepts := s.Concepts()
	if len(concepts) != 8 {
		t.Fatal(len(concepts))
	}
	facets := 0
	for _, c := range concepts {
		facets += len(c.View.Facets)
		if c.Definition == "" || len(c.Key.Digest) != 64 || c.Key.PackID != "industry.blockchain" || c.Key.Version != "1.0.0" || len(c.View.Excludes) == 0 {
			t.Fatalf("incomplete: %+v", c)
		}
	}
	if facets != 26 {
		t.Fatalf("facet inventory %d", facets)
	}
}
func TestBilingualAliasesAndAmbiguityAreExplicit(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range []string{"staking", "스테이킹", " STAKE ", "delegation"} {
		v := s.Resolve(term)
		if v.State != "exact_term" || len(v.Candidates) != 1 || v.Candidates[0].Key.ConceptID != "staking" {
			t.Fatalf("%s: %+v", term, v)
		}
	}
	ambiguity := s.Resolve("검증")
	if ambiguity.State != "ambiguous" || len(ambiguity.Candidates) != 2 || ambiguity.Candidates[0].Key.ConceptID != "consensus" || ambiguity.Candidates[1].Key.ConceptID != "mempool-admission" {
		t.Fatalf("%+v", ambiguity)
	}
	for _, term := range []string{"", "best chain", "staking algorithm speed", "unknown", "스테이킹은 좋은가"} {
		if got := s.Resolve(term); got.State != "unknown" || len(got.Candidates) != 0 {
			t.Fatalf("invented natural-language interpretation: %+v", got)
		}
	}
}
func TestSchemeLookupNeverReturnsMutableOwnedSlices(t *testing.T) {
	s, err := LoadScheme(shippedRoot)
	if err != nil {
		t.Fatal(err)
	}
	first := s.Resolve("staking")
	first.Candidates[0].View.Aliases[0] = "INJECTED"
	first.Candidates[0].View.Facets[0].Conditions[0] = "INJECTED"
	first.Candidates[0].Key.Digest = "changed"
	next := s.Resolve("staking").Candidates[0]
	if next.View.Aliases[0] == "INJECTED" || next.View.Facets[0].Conditions[0] == "INJECTED" || next.Key.Digest == "changed" {
		t.Fatal("caller mutated selected scheme")
	}
}
func TestSchemeByteChangesAlterExactPackIdentity(t *testing.T) {
	root := copyScheme(t)
	before, err := LoadScheme(root)
	if err != nil {
		t.Fatal(err)
	}
	editDefinition(t, root, func(d *SchemeDefinition) { d.Concepts[0].Aliases = append(d.Concepts[0].Aliases, "new explicit alias") })
	after, err := LoadScheme(root)
	if err != nil {
		t.Fatal(err)
	}
	if before.Concepts()[0].Key.Digest == after.Concepts()[0].Key.Digest {
		t.Fatal("sidecar change retained digest")
	}
	if before.Resolve("new explicit alias").State != "unknown" || after.Resolve("new explicit alias").State != "exact_term" {
		t.Fatal("old scheme mutated by disk change")
	}
}
func TestInvalidSchemeBoundaries(t *testing.T) {
	cases := map[string]func(*SchemeDefinition){
		"version":           func(d *SchemeDefinition) { d.Version = "2.0.0" },
		"pack":              func(d *SchemeDefinition) { d.PackID = "organization.other" },
		"missing concept":   func(d *SchemeDefinition) { d.Concepts = d.Concepts[1:] },
		"foreign concept":   func(d *SchemeDefinition) { d.Concepts[0].ID = "policy" },
		"duplicate concept": func(d *SchemeDefinition) { d.Concepts[0] = d.Concepts[1] },
		"empty boundary":    func(d *SchemeDefinition) { d.Concepts[0].Excludes = nil },
		"missing role":      func(d *SchemeDefinition) { d.Concepts[0].ApplicableRoles = nil },
		"missing facet":     func(d *SchemeDefinition) { d.Concepts[0].Facets = nil },
		"duplicate facet": func(d *SchemeDefinition) {
			d.Concepts[0].Facets = append(d.Concepts[0].Facets, d.Concepts[0].Facets[0])
		},
		"missing conditions": func(d *SchemeDefinition) { d.Concepts[0].Facets[0].Conditions = nil },
		"empty question":     func(d *SchemeDefinition) { d.Concepts[0].Facets[0].Question = " " },
		"duplicate alias": func(d *SchemeDefinition) {
			d.Concepts[0].Aliases = append(d.Concepts[0].Aliases, strings.ToUpper(d.Concepts[0].Aliases[0]))
		},
		"unsupported schema": func(d *SchemeDefinition) { d.SchemaVersion = 2 },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			root := copyScheme(t)
			editDefinition(t, root, edit)
			if _, err := LoadScheme(root); err == nil {
				t.Fatal("invalid scheme accepted")
			}
		})
	}
}
func TestStrictSchemeJSONRejectsAmbiguousDocuments(t *testing.T) {
	for name, data := range map[string]string{"duplicate": "{\"schema_version\":1,\"schema_version\":1}", "unknown": "{\"unexpected\":true}", "trailing": "{} {}", "oversize": strings.Repeat(" ", (1<<20)+1), "depth": strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34)} {
		t.Run(name, func(t *testing.T) {
			var d SchemeDefinition
			if err := strictJSON([]byte(data), &d); err == nil {
				t.Fatal("ambiguous document accepted")
			}
		})
	}
}
func TestSchemeDoesNotFollowRootOrDefinitionSymlinks(t *testing.T) {
	root := copyScheme(t)
	linked := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(root, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadScheme(linked); err == nil {
		t.Fatal("root link accepted")
	}
	target := filepath.Join(t.TempDir(), "comparison.json")
	data, err := os.ReadFile(filepath.Join(root, "comparison.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(target, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "comparison.json")); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, filepath.Join(root, "comparison.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadScheme(root); err == nil {
		t.Fatal("definition link accepted")
	}
}

func TestSchemeReusesPackDependencyAndTypeResolution(t *testing.T) {
	for _, name := range []string{"missing dependency", "unresolved core extension", "cyclic extension"} {
		t.Run(name, func(t *testing.T) {
			root := copyScheme(t)
			file := filepath.Join(root, "pack.yaml")
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var pack map[string]any
			if err = json.Unmarshal(data, &pack); err != nil {
				t.Fatal(err)
			}
			concepts := pack["concepts"].([]any)
			switch name {
			case "missing dependency":
				pack["requires"] = []any{map[string]any{"pack_id": "organization.missing", "version": "1.0.0", "digest": strings.Repeat("a", 64)}}
			case "unresolved core extension":
				concepts[0].(map[string]any)["extends"] = map[string]any{"pack_id": "core", "local_id": "not-a-core-type"}
			case "cyclic extension":
				concepts[0].(map[string]any)["extends"] = map[string]any{"pack_id": "industry.blockchain", "local_id": concepts[1].(map[string]any)["id"]}
				concepts[1].(map[string]any)["extends"] = map[string]any{"pack_id": "industry.blockchain", "local_id": concepts[0].(map[string]any)["id"]}
			}
			data, err = json.Marshal(pack)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(file, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err = LoadScheme(root); err == nil {
				t.Fatal("unresolved pack contract accepted")
			}
		})
	}
}
