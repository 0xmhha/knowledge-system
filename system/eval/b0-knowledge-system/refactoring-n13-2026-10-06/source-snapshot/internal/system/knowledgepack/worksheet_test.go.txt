package knowledgepack

import (
	"crypto/sha256"
	"encoding/hex"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func worksheetFixture(t *testing.T) (string, Selection, WorksheetCatalog) {
	t.Helper()
	root := t.TempDir()
	packRoot := filepath.Join(root, "packs", "example")
	if err := os.MkdirAll(packRoot, 0700); err != nil {
		t.Fatal(err)
	}
	pack := Pack{PackSchemaVersion: 1, PackID: "organization.example", Version: "1.0.0", Owner: "project", Scope: "test review aid", Requires: []Dependency{}, Concepts: []ConceptType{{ID: "risk", Kind: "rule", Definition: "A review topic."}}, CompetencyQuestions: []string{"Which topic needs review?"}}
	putWorksheetYAML(t, filepath.Join(packRoot, "pack.yaml"), pack)
	source := []byte("keyword source\n")
	if err := os.WriteFile(filepath.Join(packRoot, "source.md"), source, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(source)
	ref := WorksheetSource{Path: "source.md", First: 1, Last: 1, FileSHA256: hex.EncodeToString(sum[:]), ContentSHA256: hex.EncodeToString(sum[:])}
	catalog := WorksheetCatalog{SchemaVersion: 1, PackID: pack.PackID, PackVersion: pack.Version, Title: "Review topics", Authority: "test source", Scope: "test only", References: []WorksheetReference{{Label: "Source", Source: ref}}, Items: []WorksheetRisk{{ID: "one", Label: "Lock review", MappingLabel: "risk 1", Keywords: []string{"lock"}, Type: TypeRef{PackID: pack.PackID, LocalID: "risk"}, Source: ref}}}
	putWorksheetYAML(t, filepath.Join(packRoot, "worksheet-catalog.yaml"), catalog)
	return root, worksheetSelection(t, root, "packs/example", pack.PackID), catalog
}
func putWorksheetYAML(t *testing.T, path string, value any) {
	t.Helper()
	data, err := yaml.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func worksheetSelection(t *testing.T, root, source, id string) Selection {
	t.Helper()
	digest, err := DigestTree(filepath.Join(root, source))
	if err != nil {
		t.Fatal(err)
	}
	return Selection{PackID: id, Version: "1.0.0", Source: source, SHA256: digest}
}

func TestWorksheetCatalogPinnedSourceAndLegacyAbsent(t *testing.T) {
	root, selected, _ := worksheetFixture(t)
	catalogs, err := LoadWorksheetCatalogs(root, []Selection{selected})
	if err != nil || len(catalogs) != 1 || catalogs[0].Digest != selected.SHA256 || catalogs[0].SourceRoot != selected.Source || catalogs[0].Items[0].Type.LocalID != "risk" {
		t.Fatalf("valid selected catalog lost: %+v %v", catalogs, err)
	}
	if err := os.Remove(filepath.Join(root, selected.Source, "worksheet-catalog.yaml")); err != nil {
		t.Fatal(err)
	}
	selected = worksheetSelection(t, root, selected.Source, selected.PackID)
	catalogs, err = LoadWorksheetCatalogs(root, []Selection{selected})
	if err != nil || len(catalogs) != 0 {
		t.Fatalf("old pack without catalog failed: %v", err)
	}
	catalogs, err = LoadWorksheetCatalogs(root, nil)
	if err != nil || len(catalogs) != 0 {
		t.Fatal(err)
	}
}

func TestWorksheetCatalogRejectsRepinnedInvalidClaims(t *testing.T) {
	for _, mode := range []string{"schema", "version", "type", "source-hash", "span", "traversal", "duplicate-item", "keyword", "unknown-field", "multiple-documents"} {
		t.Run(mode, func(t *testing.T) {
			root, selected, catalog := worksheetFixture(t)
			path := filepath.Join(root, selected.Source, "worksheet-catalog.yaml")
			switch mode {
			case "schema":
				catalog.SchemaVersion = 2
			case "version":
				catalog.PackVersion = "2.0.0"
			case "type":
				catalog.Items[0].Type.LocalID = "missing"
			case "source-hash":
				catalog.Items[0].Source.ContentSHA256 = strings.Repeat("a", 64)
			case "span":
				catalog.Items[0].Source.Last = 2
			case "traversal":
				catalog.Items[0].Source.Path = "../source.md"
			case "duplicate-item":
				catalog.Items = append(catalog.Items, catalog.Items[0])
			case "keyword":
				catalog.Items[0].Keywords = append(catalog.Items[0].Keywords, "LOCK")
			}
			putWorksheetYAML(t, path, catalog)
			if mode == "unknown-field" {
				data, _ := os.ReadFile(path)
				var object map[string]any
				yaml.Unmarshal(data, &object)
				object["made_up_field"] = true
				putWorksheetYAML(t, path, object)
			}
			if mode == "multiple-documents" {
				f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				f.WriteString("\n---\n{}\n")
				f.Close()
			}
			selected = worksheetSelection(t, root, selected.Source, selected.PackID)
			if _, err := LoadWorksheetCatalogs(root, []Selection{selected}); err == nil || !strings.Contains(err.Error(), "pack_incompatible") {
				t.Fatalf("repinned invalid %s accepted: %v", mode, err)
			}
		})
	}
}

func TestWorksheetCatalogRejectsStalePinMissingDependencyAndSymlink(t *testing.T) {
	for _, mode := range []string{"stale", "version", "missing-pack", "duplicate-selection", "missing-dependency", "linked-source", "linked-pack"} {
		t.Run(mode, func(t *testing.T) {
			root, selected, _ := worksheetFixture(t)
			packRoot := filepath.Join(root, selected.Source)
			switch mode {
			case "stale":
				os.WriteFile(filepath.Join(packRoot, "source.md"), []byte("changed\n"), 0600)
			case "version":
				selected.Version = "2.0.0"
			case "missing-pack":
				selected.Source = "missing"
			case "missing-dependency":
				loaded, err := Load(packRoot)
				if err != nil {
					t.Fatal(err)
				}
				loaded.Pack.Requires = []Dependency{{PackID: "organization.other", Version: "1.0.0", Digest: strings.Repeat("a", 64)}}
				putWorksheetYAML(t, filepath.Join(packRoot, "pack.yaml"), loaded.Pack)
				selected = worksheetSelection(t, root, selected.Source, selected.PackID)
			case "linked-source":
				os.Remove(filepath.Join(packRoot, "source.md"))
				os.Symlink(filepath.Join(t.TempDir(), "foreign.md"), filepath.Join(packRoot, "source.md"))
			case "linked-pack":
				os.Symlink(packRoot, filepath.Join(root, "alias"))
				selected.Source = "alias"
			}
			selections := []Selection{selected}
			if mode == "duplicate-selection" {
				selections = append(selections, selected)
			}
			if _, err := LoadWorksheetCatalogs(root, selections); err == nil {
				t.Fatalf("unsafe %s selection accepted", mode)
			}
		})
	}
}

func TestWorksheetCatalogCrossPackRequiresDeclaredDependency(t *testing.T) {
	root, selected, catalog := worksheetFixture(t)
	otherRoot := filepath.Join(root, "packs", "other")
	if err := os.MkdirAll(otherRoot, 0700); err != nil {
		t.Fatal(err)
	}
	other := Pack{PackSchemaVersion: 1, PackID: "organization.other", Version: "1.0.0", Owner: "project", Scope: "test", Concepts: []ConceptType{{ID: "risk", Kind: "rule", Definition: "Other review type."}}, CompetencyQuestions: []string{"Which type?"}}
	putWorksheetYAML(t, filepath.Join(otherRoot, "pack.yaml"), other)
	otherSelection := worksheetSelection(t, root, "packs/other", other.PackID)
	catalog.Items[0].Type.PackID = other.PackID
	putWorksheetYAML(t, filepath.Join(root, selected.Source, "worksheet-catalog.yaml"), catalog)
	selected = worksheetSelection(t, root, selected.Source, selected.PackID)
	if _, err := LoadWorksheetCatalogs(root, []Selection{selected, otherSelection}); err == nil {
		t.Fatal("undeclared selected namespace accepted")
	}
	pack, err := Load(filepath.Join(root, selected.Source))
	if err != nil {
		t.Fatal(err)
	}
	pack.Pack.Requires = []Dependency{{PackID: other.PackID, Version: other.Version, Digest: otherSelection.SHA256}}
	putWorksheetYAML(t, filepath.Join(pack.Root, "pack.yaml"), pack.Pack)
	selected = worksheetSelection(t, root, selected.Source, selected.PackID)
	if catalogs, err := LoadWorksheetCatalogs(root, []Selection{selected, otherSelection}); err != nil || len(catalogs) != 1 {
		t.Fatalf("declared dependency rejected: %v", err)
	}
}
