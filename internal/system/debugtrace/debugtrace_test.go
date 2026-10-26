package debugtrace

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestFromWithoutRecorderIsNil(t *testing.T) {
	t.Parallel()
	if r := From(context.Background()); r != nil {
		t.Fatalf("From(background) = %v, want nil", r)
	}
	if ctx := WithRecorder(context.Background(), nil); From(ctx) != nil {
		t.Fatal("WithRecorder(nil) attached a recorder")
	}
}

func TestNilRecorderIsNoop(t *testing.T) {
	t.Parallel()
	var r *Recorder
	r.Before("s", "step", "", 1)
	r.After("s", "step", "", 1)
	r.Note("s", "step", "", 1)
	if got := r.Events(); got != nil {
		t.Fatalf("nil recorder events = %v, want nil", got)
	}
}

func TestRecorderKeepsOrderAndPhase(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	ctx := WithRecorder(context.Background(), r)
	From(ctx).Before("stage1", "rerank", "1", []string{"a"})
	From(ctx).After("stage1", "rerank", "1", []string{"a"})
	From(ctx).Note("stage4", "decision", "", "skip")
	ev := r.Events()
	if len(ev) != 3 {
		t.Fatalf("events = %d, want 3", len(ev))
	}
	want := []string{PhaseBefore, PhaseAfter, PhaseNote}
	for i, e := range ev {
		if e.Phase != want[i] {
			t.Errorf("event %d phase = %q, want %q", i, e.Phase, want[i])
		}
	}
}

func TestFileSinkAppendsOneLinePerCall(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	s := NewFileSink(path)
	r := NewRecorder()
	r.After("stage1", "ckv_search", "1", Hits([]contract.Hit{{Citation: contract.Citation{File: "a.go", StartLine: 1, EndLine: 2}, Symbol: "A"}}))
	if err := s.Write("p1", contract.IntentBugFix, r, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Write("p2", "", NewRecorder(), errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var recs []Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec Record
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("line %q: %v", sc.Text(), err)
		}
		recs = append(recs, rec)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2", len(recs))
	}
	if recs[0].Seq != 1 || recs[1].Seq != 2 || recs[0].Schema != SchemaVersion {
		t.Errorf("seq/schema = %d,%d,%d", recs[0].Seq, recs[1].Seq, recs[0].Schema)
	}
	if recs[0].CallerIntent != string(contract.IntentBugFix) || len(recs[0].Events) != 1 {
		t.Errorf("record 0 = %+v", recs[0])
	}
	if recs[1].Error != "boom" {
		t.Errorf("record 1 error = %q, want boom", recs[1].Error)
	}
}
