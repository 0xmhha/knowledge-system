// Command cks-glossary-gen builds a project's vocab.Glossary YAML by
// scanning that project's entries/*.yaml under docs/domain-knowledge.
//
// Each scanned entry contributes one Glossary entry: its korean_aliases
// and english_aliases together become the matchable aliases, its
// code_keywords (deduplicated, order preserved) become the canonical
// keyword list, and the entry's id is recorded under canonical for
// provenance.
//
// By default the generator includes only entries with
// status: verified — the same gate ckv build uses. Pass --status to
// override, e.g. --status=all when iterating during development.
//
// Usage:
//
//	cks-glossary-gen \
//	    -project projects/stablenet/domain-knowledge \
//	    -out    projects/stablenet/domain-knowledge/glossary.yaml \
//	    -status verified
package domaincli

import (
	"strings"
)

// entryFile is the subset of every entry YAML the generator cares about.
// We unmarshal lazily — unknown fields are tolerated so a schema bump
// that doesn't touch these fields does not break the generator.
type entryFile struct {
	ID             string   `yaml:"id"`
	Status         string   `yaml:"status"`
	KoreanAliases  []string `yaml:"korean_aliases"`
	EnglishAliases []string `yaml:"english_aliases"`
	CodeKeywords   []string `yaml:"code_keywords"`
}

// glossaryFile is the on-disk shape of a vocab.Glossary plus a single
// canonical-tracking field per entry. The shape matches vocab.Load
// exactly except for the canonical field, which the loader tolerates.
type glossaryFile struct {
	Version int                 `yaml:"version"`
	Entries []glossaryFileEntry `yaml:"entries"`
}

type glossaryFileEntry struct {
	Aliases      []string `yaml:"aliases"`
	Canonical    string   `yaml:"canonical"`
	CodeKeywords []string `yaml:"code_keywords"`
}

// statusIncluded reports whether a given entry status matches the gate.
// "all" includes everything; any other value is matched verbatim.
func statusIncluded(entryStatus, gate string) bool {
	if gate == "all" {
		return true
	}
	return entryStatus == gate
}

// buildGlossaryEntry assembles one glossary entry from a domain knowledge
// entry. Returns false when the entry contributes nothing matchable —
// either it has no aliases at all, or it has no code_keywords to map
// aliases onto. Empty / whitespace-only fields are filtered out, and
// duplicates within each list are collapsed while preserving order.
func buildGlossaryEntry(e entryFile) (glossaryFileEntry, bool) {
	aliases := dedupNonEmpty(append(append([]string{}, e.KoreanAliases...), e.EnglishAliases...))
	keywords := dedupNonEmpty(e.CodeKeywords)
	if len(aliases) == 0 || len(keywords) == 0 {
		return glossaryFileEntry{}, false
	}
	return glossaryFileEntry{
		Aliases:      aliases,
		Canonical:    e.ID,
		CodeKeywords: keywords,
	}, true
}

func dedupNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
