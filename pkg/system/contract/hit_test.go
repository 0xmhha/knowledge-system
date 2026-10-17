package contract

import (
	"encoding/json"
	"testing"
)

func TestHit_IsValid(t *testing.T) {
	t.Parallel()
	good := Citation{File: "a.go", StartLine: 1, EndLine: 1}
	bad := Citation{}
	cases := []struct {
		h    Hit
		want bool
	}{
		{Hit{Citation: good, Rank: 1, Score: 0.5}, true},
		{Hit{Citation: good, Rank: 1, Score: -1.0}, true}, // negative score allowed
		{Hit{Citation: good, Rank: 0}, false},             // rank must be >= 1
		{Hit{Citation: bad, Rank: 1}, false},
		{Hit{Citation: good, Rank: 1, NonCitableMetadata: true}, false}, // metadata cannot borrow a plausible code location
	}
	for _, tc := range cases {
		if got := tc.h.IsValid(); got != tc.want {
			t.Errorf("IsValid(%+v) = %v, want %v", tc.h, got, tc.want)
		}
	}
}

func TestHit_MetadataFlagSurvivesJSON(t *testing.T) {
	t.Parallel()
	in := Hit{Citation: Citation{File: "pkg/<convention>"}, Rank: 1, ChunkKind: "convention", NonCitableMetadata: true, Text: "derived package summary"}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil || wire["non_citable_metadata"] != true || wire["text"] != in.Text {
		t.Fatalf("lost metadata contract: %s (%v)", raw, err)
	}
	var out Hit
	if err := json.Unmarshal(raw, &out); err != nil || out.IsValid() {
		t.Fatalf("metadata became a source citation: %+v (%v)", out, err)
	}
}

func TestHit_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := Hit{
		Citation: Citation{File: "a.go", StartLine: 1, EndLine: 10},
		Rank:     3,
		Score:    0.0123,
		Source:   HitSourceFused,
	}
	buf, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Hit
	if err := json.Unmarshal(buf, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Fatalf("round-trip: got %+v, want %+v", out, in)
	}
}

func TestHit_JSONOmitsEmptySource(t *testing.T) {
	t.Parallel()
	in := Hit{
		Citation: Citation{File: "a.go", StartLine: 1, EndLine: 1},
		Rank:     1,
	}
	buf, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(buf)
	want := `{"citation":{"file":"a.go","start_line":1,"end_line":1,"commit_hash":""},"rank":1,"score":0}`
	if got != want {
		t.Fatalf("marshal:\n got: %s\nwant: %s", got, want)
	}
}
