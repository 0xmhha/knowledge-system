package temporal

import (
	"reflect"
	"testing"
)

// TestLineChangeTouches pins the one rule for "a change touched these
// lines": added or replaced lines inside the range, or a deletion between
// two of its lines; not a deletion just before or just after it.
func TestLineChangeTouches(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    LineChange
		want bool
	}{
		{"replaced line inside", LineChange{OldStart: 12, OldCount: 1, NewStart: 12, NewCount: 1}, true},
		{"added lines overlapping the start", LineChange{OldStart: 8, OldCount: 0, NewStart: 9, NewCount: 2}, true},
		{"added lines just before", LineChange{OldStart: 8, OldCount: 0, NewStart: 8, NewCount: 2}, false},
		{"added lines just after", LineChange{OldStart: 20, OldCount: 0, NewStart: 21, NewCount: 3}, false},
		{"deletion between two lines", LineChange{OldStart: 13, OldCount: 2, NewStart: 12, NewCount: 0}, true},
		{"deletion just before", LineChange{OldStart: 9, OldCount: 2, NewStart: 9, NewCount: 0}, false},
		{"deletion just after", LineChange{OldStart: 21, OldCount: 2, NewStart: 20, NewCount: 0}, false},
	} {
		if got := tc.c.Touches(10, 20); got != tc.want {
			t.Errorf("%s: Touches(10, 20) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestPatchChanges reads a hunk with context lines into changes numbered
// like -U0 headers, so context lines are never changed lines.
func TestPatchChanges(t *testing.T) {
	patch := []byte("@@ -5,8 +5,8 @@ func f() {\n ctx\n ctx\n ctx\n-old\n+new\n ctx\n-gone\n ctx\n ctx\n+added\n")
	got, ok := PatchChanges(patch)
	if !ok {
		t.Fatal("no header")
	}
	want := []LineChange{
		{OldStart: 8, OldCount: 1, NewStart: 8, NewCount: 1},
		{OldStart: 10, OldCount: 1, NewStart: 9, NewCount: 0},
		{OldStart: 12, OldCount: 0, NewStart: 12, NewCount: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PatchChanges = %+v, want %+v", got, want)
	}
	if _, ok := PatchChanges([]byte("no header\n")); ok {
		t.Error("a patch without a header was read")
	}
}

// TestNumberSpacesRule states both rules in words.
func TestNumberSpacesRule(t *testing.T) {
	own := NumberSpaces{Repo: "fork/repo"}.Rule()
	up := NumberSpaces{Repo: "fork/repo", UpstreamRepo: "up/repo", UpstreamBase: "abc"}.Rule()
	if own == up || own == "" || up == "" {
		t.Errorf("rules: %q / %q", own, up)
	}
}

// TestTellingLines: braces, bare returns and comments tell nothing, so they
// cannot carry a declaration into a copy's source.
func TestTellingLines(t *testing.T) {
	for line, want := range map[string]bool{
		"}": false, "\t})": false, "return nil": false, "\treturn err": false, "// note": false,
		"return x + 1": true, "GetState(common.Address) common.Hash": true, "func F() {": true,
	} {
		if got := isTellingLine(line); got != want {
			t.Errorf("isTellingLine(%q) = %v, want %v", line, got, want)
		}
	}
	if !namesIdent("\tGetRefund() uint64", "GetRefund") || namesIdent("\tAddRefund(*big.Int)", "GetRefund") {
		t.Error("namesIdent does not match whole identifiers")
	}
}
