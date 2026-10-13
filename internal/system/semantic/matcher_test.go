package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchConceptsPreservesAmbiguityAndNaturalLanguage(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "spec-driven", "ontology-pilot.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := ExtractOntology(Snapshot{ProjectID: "knowledge-system", DatasetID: "d",
		Commit: strings.Repeat("a", 40)}, "ontology.yaml", source)
	if err != nil {
		t.Fatal(err)
	}
	hits, err := p.MatchConcepts("프로젝트의 소스 스냅샷은 어디인가?", 5)
	if err != nil || len(hits) != 2 || hits[0].Score != 0.5 {
		t.Fatalf("Korean query matches = %+v, err=%v", hits, err)
	}
	p.Concepts[1].Terms = append([]Term(nil), p.Concepts[1].Terms...)
	p.Concepts[1].Terms[0].Value = "project"
	hits, err = p.MatchConcepts("project", 1)
	if err != nil || len(hits) != 2 || hits[0].ConceptID != "dataset" || hits[1].ConceptID != "project" {
		t.Fatalf("ambiguous term collapsed: %+v, err=%v", hits, err)
	}
	hits, err = p.MatchConcepts("xyzzy unseen wording", 5)
	if err != nil || len(hits) != 0 {
		t.Fatalf("unregistered query should preserve no ontology candidates: %+v, %v", hits, err)
	}
}

func TestLatinTermNeedsWordBoundary(t *testing.T) {
	if containsTerm("capillary", "api") {
		t.Fatal("api matched inside capillary")
	}
	if !containsTerm("use the api endpoint", "api") {
		t.Fatal("api term missed")
	}
	if !containsTerm("정책을 검토", "정책") {
		t.Fatal("Korean particle blocked exact term")
	}
}

func TestMixedLanguageTermUsesItsEndpointBoundaries(t *testing.T) {
	for _, tc := range []struct {
		query, term string
		want        bool
	}{
		{"current 읽기 락과 쓰기 보호", "current 읽기 락", true},
		{"xcurrent 읽기 락과", "current 읽기 락", false},
		{"쿼럼 api endpoint", "쿼럼 api", true},
		{"쿼럼 apical", "쿼럼 api", false},
		{"읽기 currentmutex와", "읽기 currentmutex", false},
		{"capillary api", "api", true},
	} {
		if got := containsTerm(tc.query, tc.term); got != tc.want {
			t.Fatalf("query=%q term=%q got=%v want=%v", tc.query, tc.term, got, tc.want)
		}
	}
}
