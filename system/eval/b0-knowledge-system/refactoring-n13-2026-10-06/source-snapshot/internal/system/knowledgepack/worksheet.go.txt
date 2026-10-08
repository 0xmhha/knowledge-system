package knowledgepack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"gopkg.in/yaml.v3"
)

type WorksheetSource struct {
	Path          string `yaml:"path" json:"path"`
	First         int    `yaml:"first" json:"first"`
	Last          int    `yaml:"last" json:"last"`
	FileSHA256    string `yaml:"file_sha256" json:"file_sha256"`
	ContentSHA256 string `yaml:"content_sha256" json:"content_sha256"`
}
type WorksheetReference struct {
	Label  string          `yaml:"label"`
	Source WorksheetSource `yaml:"source"`
}
type WorksheetRisk struct {
	ID           string          `yaml:"id"`
	Label        string          `yaml:"label"`
	MappingLabel string          `yaml:"mapping_label"`
	Keywords     []string        `yaml:"keywords"`
	Type         TypeRef         `yaml:"type"`
	Source       WorksheetSource `yaml:"source"`
}
type WorksheetCatalog struct {
	SchemaVersion int                  `yaml:"catalog_schema_version"`
	PackID        string               `yaml:"pack_id"`
	PackVersion   string               `yaml:"pack_version"`
	Title         string               `yaml:"title"`
	Authority     string               `yaml:"authority"`
	Scope         string               `yaml:"scope"`
	References    []WorksheetReference `yaml:"references"`
	Items         []WorksheetRisk      `yaml:"items"`
	Digest        string               `yaml:"-"`
	SourceRoot    string               `yaml:"-"` // project-relative; never a machine path
}

// LoadWorksheetCatalogs uses explicitly pinned local packs. Resolve applies the
// existing complete-set dependency/type rules; a pack with no catalog is valid.
// Catalogs are heuristic review aids and never upgrade an instance's status.
func LoadWorksheetCatalogs(projectRoot string, selections []Selection) ([]WorksheetCatalog, error) {
	if len(selections) > 32 {
		return nil, fmt.Errorf("pack_incompatible: too many worksheet packs")
	}
	loaded := make([]LoadedPack, 0, len(selections))
	sources := map[string]string{}
	for _, selected := range selections {
		if !packIDPattern.MatchString(selected.PackID) || !versionPattern.MatchString(selected.Version) || !digestPattern.MatchString(selected.SHA256) || sources[selected.PackID] != "" {
			return nil, fmt.Errorf("pack_incompatible: invalid worksheet selection")
		}
		root, err := registeredDir(projectRoot, selected.Source)
		if err != nil {
			return nil, err
		}
		pack, err := Load(root)
		if err != nil {
			return nil, err
		}
		if pack.Pack.PackID != selected.PackID || pack.Pack.Version != selected.Version || pack.Digest != selected.SHA256 {
			return nil, fmt.Errorf("pack_lock_mismatch: worksheet selection differs from pack bytes")
		}
		loaded = append(loaded, pack)
		sources[selected.PackID] = selected.Source
	}
	ordered, err := Resolve(loaded)
	if err != nil {
		return nil, err
	}
	types := map[TypeRef]bool{}
	for _, pack := range ordered {
		for _, concept := range pack.Pack.Concepts {
			types[TypeRef{PackID: pack.Pack.PackID, LocalID: concept.ID}] = true
		}
	}
	catalogs := make([]WorksheetCatalog, 0, len(ordered))
	for _, pack := range ordered {
		path := filepath.Join(pack.Root, "worksheet-catalog.yaml")
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("pack_incompatible: worksheet catalog must be regular")
		}
		data, err := setup.ReadSourceFileNoFollow(pack.Root, "worksheet-catalog.yaml", 1<<20)
		if err != nil {
			return nil, err
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		var catalog WorksheetCatalog
		if err := decoder.Decode(&catalog); err != nil {
			return nil, fmt.Errorf("pack_incompatible: invalid worksheet catalog")
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, fmt.Errorf("pack_incompatible: multiple worksheet catalog documents")
		}
		if err := catalog.validate(pack, types); err != nil {
			return nil, err
		}
		digest, err := DigestTree(pack.Root)
		if err != nil {
			return nil, err
		}
		if digest != pack.Digest {
			return nil, fmt.Errorf("pack_lock_mismatch: worksheet pack changed during validation")
		}
		catalog.Digest = pack.Digest
		catalog.SourceRoot = sources[pack.Pack.PackID]
		catalogs = append(catalogs, catalog)
	}
	return catalogs, nil
}

func worksheetText(s string) bool {
	if s == "" || len(s) > 2048 || !utf8.ValidString(s) || strings.TrimSpace(s) != s || strings.ContainsAny(s, "`\r\n") {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (c WorksheetCatalog) validate(pack LoadedPack, types map[TypeRef]bool) error {
	if c.SchemaVersion != 1 || c.PackID != pack.Pack.PackID || c.PackVersion != pack.Pack.Version || !worksheetText(c.Title) || !worksheetText(c.Authority) || !worksheetText(c.Scope) || len(c.Items) == 0 || len(c.Items) > 128 || len(c.References) > 32 {
		return fmt.Errorf("pack_incompatible: invalid worksheet catalog header")
	}
	dependencies := map[string]bool{pack.Pack.PackID: true}
	for _, dep := range pack.Pack.Requires {
		dependencies[dep.PackID] = true
	}
	seen := map[string]bool{}
	for _, item := range c.Items {
		if !localIDPattern.MatchString(item.ID) || seen[item.ID] || !worksheetText(item.Label) || !worksheetText(item.MappingLabel) || len(item.Keywords) == 0 || len(item.Keywords) > 64 || !types[item.Type] || !dependencies[item.Type.PackID] {
			return fmt.Errorf("pack_incompatible: invalid worksheet risk type or item")
		}
		seen[item.ID] = true
		keywords := map[string]bool{}
		for _, word := range item.Keywords {
			key := strings.ToLower(word)
			if !worksheetText(word) || len(word) > 128 || keywords[key] {
				return fmt.Errorf("pack_incompatible: invalid worksheet keywords")
			}
			keywords[key] = true
		}
		if err := validateWorksheetSource(pack.Root, item.Source); err != nil {
			return err
		}
	}
	for _, ref := range c.References {
		if !worksheetText(ref.Label) {
			return fmt.Errorf("pack_incompatible: invalid worksheet reference")
		}
		if err := validateWorksheetSource(pack.Root, ref.Source); err != nil {
			return err
		}
	}
	return nil
}
func validateWorksheetSource(root string, source WorksheetSource) error {
	if source.First < 1 || source.Last < source.First || !digestPattern.MatchString(source.FileSHA256) || !digestPattern.MatchString(source.ContentSHA256) {
		return fmt.Errorf("pack_incompatible: invalid worksheet provenance")
	}
	data, err := setup.ReadSourceFileNoFollow(root, source.Path, 1<<20)
	if err != nil {
		return fmt.Errorf("pack_incompatible: worksheet provenance source unavailable")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != source.FileSHA256 {
		return fmt.Errorf("pack_incompatible: worksheet provenance file hash differs")
	}
	lines := bytes.SplitAfter(data, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if source.Last > len(lines) {
		return fmt.Errorf("pack_incompatible: worksheet provenance line range differs")
	}
	sum = sha256.Sum256(bytes.Join(lines[source.First-1:source.Last], nil))
	if hex.EncodeToString(sum[:]) != source.ContentSHA256 {
		return fmt.Errorf("pack_incompatible: worksheet provenance span hash differs")
	}
	return nil
}
