package summary

import (
	"context"
	"errors"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

type fakeGen struct {
	calls int
	out   string
	err   error
}

func (f *fakeGen) Generate(_ context.Context, _ string) (string, error) {
	f.calls++
	return f.out, f.err
}

func symbolChunk() types.Chunk {
	return types.Chunk{
		ID: "code", File: "core/vm/contracts.go", StartLine: 10, EndLine: 14, Language: "go",
		SymbolName: "activePrecompiledContracts", SymbolKind: "Function", ChunkKind: types.ChunkSymbol,
		CommitHash: "abc", CanonicalID: "core/vm.activePrecompiledContracts",
		Text: "func activePrecompiledContracts(rules params.Rules) PrecompiledContracts {\n" +
			"\tswitch {\n\tcase rules.IsAnzeon:\n\t\treturn PrecompiledContractsAnzeon\n\t}\n}",
	}
}

const goodOut = "EN: Picks the precompiled contract set for the active fork; extend it when a new fork adds contracts.\n" +
	"KO: 활성 포크에 맞는 precompiled 컨트랙트 집합을 고른다. 새 포크가 컨트랙트를 추가할 때 수정한다."

func TestAppend_AddsSummaryWithTheSymbolSpan(t *testing.T) {
	g := &fakeGen{out: goodOut}
	s, err := New(g, "m", t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	code := symbolChunk()
	short := code
	short.ID, short.StartLine, short.EndLine = "short", 20, 21
	test := code
	test.ID, test.IsTest = "test", true

	got := s.Append(context.Background(), []types.Chunk{code, short, test})
	if len(got) != 4 {
		t.Fatalf("chunks = %d, want 3 inputs + 1 summary", len(got))
	}
	sum := got[3]
	if sum.ChunkKind != types.ChunkSymbolSummary || sum.File != code.File ||
		sum.StartLine != code.StartLine || sum.EndLine != code.EndLine ||
		sum.CanonicalID != code.CanonicalID || sum.CommitHash != code.CommitHash {
		t.Fatalf("summary does not carry the symbol identity and span: %+v", sum)
	}
	if sum.ID == "" || sum.ID == code.ID {
		t.Fatalf("summary ID = %q, want a new ID", sum.ID)
	}
	if st := s.Stats(); st.Eligible != 1 || st.Generated != 1 {
		t.Fatalf("stats = %+v, want 1 eligible, 1 generated", st)
	}
}

func TestAppend_MinLinesSkipsShorterSymbols(t *testing.T) {
	g := &fakeGen{out: goodOut}
	s, _ := New(g, "m", t.TempDir(), 10)
	long := symbolChunk()
	long.ID, long.EndLine = "long", long.StartLine+9
	got := s.Append(context.Background(), []types.Chunk{symbolChunk(), long})
	if len(got) != 3 || got[2].StartLine != long.StartLine || got[2].EndLine != long.EndLine {
		t.Fatalf("want one summary for the 10-line symbol only, got %d chunks", len(got))
	}
	if st := s.Stats(); st.Eligible != 1 {
		t.Fatalf("stats = %+v, want 1 eligible", st)
	}
}

func TestAppend_CachesSummariesAndRejectionsButNotFailures(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	ok := &fakeGen{out: goodOut}
	s, _ := New(ok, "m", dir, 0)
	s.Append(ctx, []types.Chunk{symbolChunk()})
	s2, _ := New(ok, "m", dir, 0)
	if got := s2.Append(ctx, []types.Chunk{symbolChunk()}); len(got) != 2 || ok.calls != 1 {
		t.Fatalf("cache miss: chunks=%d calls=%d", len(got), ok.calls)
	}

	other := symbolChunk()
	other.Text += "\n// other body"
	bad := &fakeGen{out: "EN: Calls makeShinyThing.\nKO: makeShinyThing 을 호출한다."}
	s3, _ := New(bad, "m", dir, 0)
	s3.Append(ctx, []types.Chunk{other})
	s3.Append(ctx, []types.Chunk{other})
	if bad.calls != 1 || s3.Stats().Rejected != 1 {
		t.Fatalf("rejection not cached: calls=%d stats=%+v", bad.calls, s3.Stats())
	}

	down := &fakeGen{err: errors.New("model down")}
	third := symbolChunk()
	third.Text += "\n// third body"
	s4, _ := New(down, "m", dir, 0)
	s4.Append(ctx, []types.Chunk{third})
	s4.Append(ctx, []types.Chunk{third})
	if down.calls != 2 || s4.Stats().Failed != 2 {
		t.Fatalf("failure must not be cached: calls=%d stats=%+v", down.calls, s4.Stats())
	}
}

func TestValidate(t *testing.T) {
	c := symbolChunk()
	cases := []struct {
		name string
		out  string
		ok   bool
	}{
		{"two lines", goodOut, true},
		{"bold labels and preamble", "Here you go:\n**EN:** Picks the set, e.g. for Anzeon.\n**KO:** 포크별 집합을 고른다.", true},
		{"known identifiers", "EN: Returns PrecompiledContractsAnzeon when rules.IsAnzeon is set.\nKO: rules.IsAnzeon 이면 PrecompiledContractsAnzeon 을 돌려준다.", true},
		{"missing korean line", "EN: Picks the set.", false},
		{"korean line without hangul", "EN: Picks the set.\nKO: Picks the set.", false},
		{"invented identifier", "EN: Calls registerAnzeonUpgrade.\nKO: registerAnzeonUpgrade 를 호출한다.", false},
	}
	for _, tc := range cases {
		if _, ok := Validate(tc.out, c); ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.ok)
		}
	}
}

func TestNilSummarizerIsNoOp(t *testing.T) {
	var s *Summarizer
	in := []types.Chunk{symbolChunk()}
	if got := s.Append(context.Background(), in); len(got) != 1 {
		t.Fatalf("nil summarizer changed chunks: %d", len(got))
	}
}
