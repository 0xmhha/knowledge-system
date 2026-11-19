package observe_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/graph/verify/observe"
)

// TestInstrumentRunCompare builds a graph of testdata/obsmod, instruments a
// copy, runs its test and compares the records with the graph: every
// interface call, function-value call and channel pair that ran is in it.
func TestInstrumentRunCompare(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test on a copy")
	}
	src := "testdata/obsmod"
	gdir := t.TempDir()
	if _, err := buildpipe.Run(buildpipe.Options{SrcRoot: src, OutDir: gdir, Languages: []string{"auto"}, CKGVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	cp := filepath.Join(t.TempDir(), "copy")
	st, err := observe.Instrument(src, cp, "example.com/obs", []string{"a/a.go", "a/a_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Files != 2 || st.ChannelOps != 4 || st.SkippedOps != 0 {
		t.Fatalf("instrument stats %+v", st)
	}
	obs := t.TempDir()
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = cp
	cmd.Env = append(os.Environ(), "CKGOBS_DIR="+obs, "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test in the copy: %v\n%s", err, out)
	}
	store, err := persist.OpenReadOnly(filepath.Join(gdir, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	r, err := observe.Compare(obs, store, 10)
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]observe.PairStats{
		"interface": r.Interface, "func value": r.FuncValue, "channel ops": r.ChannelOps, "channel pairs": r.ChannelPairs,
	} {
		want := map[string]int{"interface": 2, "func value": 2, "channel ops": 4, "channel pairs": 2}[name]
		if got.Observed != want || got.InGraph != want {
			t.Errorf("%s: %+v, want %d observed and in the graph", name, got, want)
		}
	}
	if r.InterfaceByTypeOnly != 0 {
		t.Errorf("both implementations reach use by value flow; %d linked only by type", r.InterfaceByTypeOnly)
	}
}

func TestCanonical(t *testing.T) {
	for in, want := range map[string]string{
		"example.com/obs/a.(*gov).Tip":      "example.com/obs/a.(*gov).Tip",
		"example.com/obs/a.fixed.Tip":       "example.com/obs/a.(fixed).Tip",
		"example.com/obs/a.Run.func1":       "example.com/obs/a.Run",
		"example.com/obs/a.Run.func1.2":     "example.com/obs/a.Run",
		"example.com/obs/a.each[...]":       "example.com/obs/a.each",
		"example.com/obs/a.(*box[...]).Get": "example.com/obs/a.(*box).Get",
		"example.com/obs/a.init.0":          "example.com/obs/a.init",
	} {
		if got := observe.Canonical(in); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", in, got, want)
		}
	}
}
