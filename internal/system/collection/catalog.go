// Package collection binds implementation and comparison reads to explicit,
// immutable project versions. It does not activate datasets or merge indexes.
package collection

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

const SchemaVersion = 1
const MaxCatalogBytes = 1 << 20

var (
	ErrCatalogInvalid   = errors.New("collection: catalog_invalid")
	ErrScopeMismatch    = errors.New("collection: scope_mismatch")
	ErrTargetRequired   = errors.New("collection: target_required")
	ErrStaleCatalog     = errors.New("collection: stale_catalog")
	ErrUnsupportedMode  = errors.New("collection: unsupported_mode")
	ErrDatasetMismatch  = errors.New("collection: dataset_mismatch")
	ErrEvidenceMismatch = errors.New("collection: evidence_mismatch")
	ErrClosed           = errors.New("collection: closed")
	ErrFetchFailed      = errors.New("collection: fetch_failed")
)

// Project IDs are explicitly registered, never derived from a name, URL,
// directory, import path, commit or source bytes. Upstream is metadata only.
type Project struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	SourceURI         string `json:"source_uri,omitempty"`
	UpstreamProjectID string `json:"upstream_project_id,omitempty"`
}

// VersionDir is local deployment configuration, not a source identity. It
// must name a real immutable version directory, never a current symlink.
type Version struct {
	Coordinates contract.V2Coordinates `json:"coordinates"`
	VersionDir  string                 `json:"version_dir"`
	Origins     []string               `json:"origins"`
}

type Collection struct {
	ID      string                   `json:"id"`
	Members []contract.V2Coordinates `json:"members"`
}

// Catalog is an immutable configuration document. Adding a version or
// changing membership produces a new digest; a request pins that digest.
type Catalog struct {
	SchemaVersion int          `json:"schema_version"`
	Projects      []Project    `json:"projects"`
	Versions      []Version    `json:"versions"`
	Collections   []Collection `json:"collections"`
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var origin = regexp.MustCompile(`^(repo|docs:corpus\.(0|[1-9][0-9]*)|knowledge:[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+)$`)

func digest(value string, size int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == size && strings.ToLower(value) == value
}

func validCoordinates(c contract.V2Coordinates) bool {
	if !identifier.MatchString(c.ProjectID) || !digest(c.DatasetID, 32) || !digest(c.SnapshotID, 32) {
		return false
	}
	switch c.SourceMode {
	case "committed", "working-tree":
		return digest(c.BaseCommit, 20)
	case "snapshot-only":
		return c.BaseCommit == ""
	default:
		return false
	}
}

func coordinateKey(c contract.V2Coordinates) string {
	return c.ProjectID + "\x00" + c.DatasetID + "\x00" + c.SnapshotID + "\x00" + c.SourceMode + "\x00" + c.BaseCommit
}

// CanonicalBytes validates and sorts copies. Caller-owned slices are never
// retained; JSON array order does not change the membership identity.
func (c Catalog) CanonicalBytes() ([]byte, error) {
	if c.SchemaVersion != SchemaVersion || len(c.Projects) == 0 || len(c.Projects) > 64 ||
		len(c.Versions) == 0 || len(c.Versions) > 128 || len(c.Collections) == 0 || len(c.Collections) > 64 {
		return nil, ErrCatalogInvalid
	}
	projects := make(map[string]Project)
	for _, p := range c.Projects {
		if !identifier.MatchString(p.ID) || strings.TrimSpace(p.Name) == "" || len(p.Name) > 256 ||
			len(p.SourceURI) > 2048 || p.UpstreamProjectID == p.ID {
			return nil, ErrCatalogInvalid
		}
		if _, exists := projects[p.ID]; exists {
			return nil, ErrCatalogInvalid
		}
		projects[p.ID] = p
	}
	for _, p := range projects {
		seen := map[string]bool{p.ID: true}
		for upstream := p.UpstreamProjectID; upstream != ""; {
			parent, exists := projects[upstream]
			if !exists || seen[upstream] {
				return nil, ErrCatalogInvalid
			}
			seen[upstream], upstream = true, parent.UpstreamProjectID
		}
	}
	versions := make(map[contract.V2Coordinates]bool)
	projectDatasets := make(map[[2]string]bool)
	dirs := make(map[string]bool)
	for i, v := range c.Versions {
		_, registered := projects[v.Coordinates.ProjectID]
		pair := [2]string{v.Coordinates.ProjectID, v.Coordinates.DatasetID}
		if !registered || !validCoordinates(v.Coordinates) || versions[v.Coordinates] || projectDatasets[pair] ||
			!filepath.IsAbs(v.VersionDir) || filepath.Clean(v.VersionDir) != v.VersionDir ||
			filepath.Base(v.VersionDir) == "current" || dirs[v.VersionDir] || len(v.Origins) == 0 || len(v.Origins) > 64 {
			return nil, ErrCatalogInvalid
		}
		seen := map[string]bool{}
		for _, id := range v.Origins {
			if !origin.MatchString(id) || seen[id] {
				return nil, ErrCatalogInvalid
			}
			seen[id] = true
		}
		if !seen["repo"] {
			return nil, ErrCatalogInvalid
		}
		versions[v.Coordinates], projectDatasets[pair], dirs[v.VersionDir] = true, true, true
		// Replace the slice before sorting so CanonicalBytes is non-mutating.
		if i == 0 {
			c.Versions = append([]Version(nil), c.Versions...)
		}
		c.Versions[i].Origins = append([]string(nil), v.Origins...)
		sort.Strings(c.Versions[i].Origins)
	}
	collections := map[string]bool{}
	c.Collections = append([]Collection(nil), c.Collections...)
	for i, group := range c.Collections {
		if !identifier.MatchString(group.ID) || collections[group.ID] || len(group.Members) == 0 || len(group.Members) > 32 {
			return nil, ErrCatalogInvalid
		}
		collections[group.ID] = true
		seen := map[contract.V2Coordinates]bool{}
		for _, member := range group.Members {
			if !versions[member] || seen[member] {
				return nil, ErrCatalogInvalid
			}
			seen[member] = true
		}
		c.Collections[i].Members = append([]contract.V2Coordinates(nil), group.Members...)
		sort.Slice(c.Collections[i].Members, func(a, b int) bool {
			return coordinateKey(c.Collections[i].Members[a]) < coordinateKey(c.Collections[i].Members[b])
		})
	}
	c.Projects = append([]Project(nil), c.Projects...)
	sort.Slice(c.Projects, func(a, b int) bool { return c.Projects[a].ID < c.Projects[b].ID })
	sort.Slice(c.Versions, func(a, b int) bool {
		return coordinateKey(c.Versions[a].Coordinates) < coordinateKey(c.Versions[b].Coordinates)
	})
	sort.Slice(c.Collections, func(a, b int) bool { return c.Collections[a].ID < c.Collections[b].ID })
	raw, err := json.Marshal(c)
	if err != nil || len(raw) > MaxCatalogBytes {
		return nil, ErrCatalogInvalid
	}
	return raw, nil
}

// Decode rejects unknown fields, duplicate object keys and trailing values.
func Decode(reader io.Reader) (Catalog, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, MaxCatalogBytes+1))
	if err != nil || len(raw) > MaxCatalogBytes {
		return Catalog{}, ErrCatalogInvalid
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(raw)), 0); err != nil {
		return Catalog{}, ErrCatalogInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var catalog Catalog
	if decoder.Decode(&catalog) != nil {
		return Catalog{}, ErrCatalogInvalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return Catalog{}, ErrCatalogInvalid
	}
	if _, err := catalog.CanonicalBytes(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func uniqueJSON(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return ErrCatalogInvalid
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, container := token.(json.Delim)
	if !container {
		return nil
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delim == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return ErrCatalogInvalid
			}
			seen[name] = true
		}
		if err := uniqueJSON(decoder, depth+1); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

// ScopedRef scopes local graph/canonical/chunk IDs and dedup/cache keys.
// Same-named symbols in forks remain different references.
type ScopedRef struct {
	Coordinates contract.V2Coordinates `json:"coordinates"`
	OriginID    string                 `json:"origin_id"`
	Path        string                 `json:"path"`
	LocalID     string                 `json:"local_id"`
}

func (r ScopedRef) Key() (string, error) {
	if !validCoordinates(r.Coordinates) || !origin.MatchString(r.OriginID) || r.LocalID == "" ||
		len(r.LocalID) > 4096 || r.Path == "" || path.IsAbs(r.Path) || path.Clean(r.Path) != r.Path ||
		r.Path == ".." || strings.HasPrefix(r.Path, "../") || strings.ContainsAny(r.Path+r.LocalID, "\x00\\") {
		return "", ErrScopeMismatch
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("%w: reference encoding", ErrScopeMismatch)
	}
	hash := sha256.Sum256(append([]byte("cks.scoped-ref.v1\x00"), raw...))
	return hex.EncodeToString(hash[:]), nil
}
