package evalcli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/config"
	"github.com/0xmhha/knowledge-system/internal/system/eval"
	"gopkg.in/yaml.v3"
)

type matrixFake struct {
	closed  bool
	calls   int
	capture func(eval.CaptureRequest) error
}

func (f *matrixFake) Capture(ctx context.Context, r eval.CaptureRequest) (eval.RawToolCall, error) {
	if f.closed {
		return eval.RawToolCall{}, fmt.Errorf("used closed session")
	}
	f.calls++
	err := f.capture(r)
	now := time.Now()
	b, _ := json.Marshal(r.Arguments)
	return eval.RawToolCall{Arguments: b, CompletedAt: &now, ElapsedNS: 1, Response: json.RawMessage(`{"content":[]}`)}, err
}
func (f *matrixFake) Close() error { f.closed = true; return nil }
func matrixTestFiles(t *testing.T) (string, string, string, string) {
	t.Helper()
	root := t.TempDir()
	input := filepath.Join(root, "requests.json")
	cfgpath := filepath.Join(root, "base.yaml")
	binary := filepath.Join(root, "binary")
	out := filepath.Join(root, "output")
	raw := `{"schema_version":1,"requests":[{"id":"a","tool":"cks.context.get_for_task_v2","arguments":{"prompt":"Alpha","knowledge_subsystem":"public","knowledge_as_of":"2026-10-03"}},{"id":"b","tool":"cks.context.get_for_task_v2","arguments":{"prompt":"Beta"}}]}`
	c := config.Default()
	body, e := yaml.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	for p, b := range map[string][]byte{input: []byte(raw), cfgpath: body, binary: []byte("fixture")} {
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	return input, cfgpath, binary, out
}
func matrixTestInputReader(c *config.Config, cfg, input, binary string, extra []string) (map[string]string, *setup.DatasetIdentity, error) {
	m := map[string]string{}
	for _, p := range append([]string{cfg, input, binary}, extra...) {
		h, e := shaFile(p)
		if e != nil {
			return nil, nil, e
		}
		m[p] = h
	}
	return m, &setup.DatasetIdentity{DatasetID: "test-only"}, nil
}
func matrixReadRows(t *testing.T, out string) (matrixReport, []matrixRow) {
	t.Helper()
	b, e := os.ReadFile(filepath.Join(out, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	var r matrixReport
	if e := json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(filepath.Join(out, "rows.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	var rows []matrixRow
	s := bufio.NewScanner(f)
	for s.Scan() {
		var row matrixRow
		if e := json.Unmarshal(s.Bytes(), &row); e != nil {
			t.Fatal(e)
		}
		rows = append(rows, row)
	}
	if e := s.Err(); e != nil {
		t.Fatal(e)
	}
	return r, rows
}

func TestMatrixRunsRotatedPairsAndFreshColdSessions(t *testing.T) {
	input, cfg, binary, out := matrixTestFiles(t)
	var sessions []*matrixFake
	factory := func(ctx context.Context, binary, path string, timeout time.Duration) (matrixSession, error) {
		c, e := config.Load(path)
		if e != nil {
			return nil, e
		}
		if c.Semantic.OntologyMode == "" || !c.Logging.MeasureBackendCalls || c.Listen.Transport != "stdio" {
			t.Fatal("arm not wired")
		}
		s := &matrixFake{capture: func(r eval.CaptureRequest) error {
			want := strings.HasSuffix(filepath.Base(path), "_on.yaml")
			if r.Arguments["include_knowledge"] != want {
				t.Fatal("pack axis does not match arm")
			}
			return nil
		}}
		sessions = append(sessions, s)
		return s, nil
	}
	if e := matrixRunWithInputs(context.Background(), input, cfg, binary, out, 1, 2, 2, 1, time.Second, nil, factory, matrixTestInputReader); e != nil {
		t.Fatal(e)
	}
	report, rows := matrixReadRows(t, out)
	if report.State != "captured" || report.Rows != 96 || len(rows) != 96 || len(sessions) != 24 {
		t.Fatalf("counts %+v rows %d sessions %d", report, len(rows), len(sessions))
	}
	for group := 0; group < 12; group++ {
		seen := map[string]bool{}
		for position := 0; position < 8; position++ {
			r := rows[group*8+position]
			want := report.Arms[(group+position)%8].ID
			if r.Group != group || r.Position != position+1 || r.Sequence != group*8+position+1 || r.Arm != want {
				t.Fatalf("paired order mismatch %+v", r)
			}
			seen[r.Arm] = true
			if r.Phase == "cold_process" && r.ProcessStartThroughResponseNS <= 0 {
				t.Fatal("cold initialization missing")
			}
			if r.Phase != "cold_process" && r.ProcessStartThroughResponseNS != 0 {
				t.Fatal("warm labeled cold")
			}
		}
		if len(seen) != 8 {
			t.Fatal("missing arm")
		}
	}
	for i, s := range sessions {
		if !s.closed {
			t.Fatal("session leaked")
		}
		if i < 8 && s.calls != 10 {
			t.Fatalf("warm calls %d", s.calls)
		}
		if i >= 8 && s.calls != 1 {
			t.Fatal("cold process reused")
		}
	}
	// Configs must share every setting except mode and observation path.
	var common string
	for _, a := range report.Arms {
		c, e := config.Load(a.Config)
		if e != nil {
			t.Fatal(e)
		}
		c.Semantic.OntologyMode = ""
		c.Logging.FootprintDir = ""
		b, _ := yaml.Marshal(c)
		if common == "" {
			common = string(b)
		} else if string(b) != common {
			t.Fatal("retrieval conditions differ")
		}
	}
}

func TestMatrixKeepsStartupErrorsCancellationAndChangedInputs(t *testing.T) {
	for _, kind := range []string{"startup", "cancel", "input", "config", "transport", "tool"} {
		t.Run(kind, func(t *testing.T) {
			input, cfg, binary, out := matrixTestFiles(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var sessions []*matrixFake
			mutated := false
			factory := func(ctx context.Context, bin, path string, timeout time.Duration) (matrixSession, error) {
				if kind == "startup" && strings.HasSuffix(path, "baseline_off.yaml") {
					return nil, fmt.Errorf("initialize deadline fixture")
				}
				s := &matrixFake{capture: func(r eval.CaptureRequest) error {
					if !mutated {
						mutated = true
						switch kind {
						case "cancel":
							cancel()
						case "input":
							_ = os.WriteFile(input, []byte("changed"), 0600)
						case "config":
							b, _ := os.ReadFile(path)
							_ = os.WriteFile(path, append(b, []byte("# changed\n")...), 0600)
						}
					}
					if kind == "transport" {
						return fmt.Errorf("transport fixture")
					}
					return nil
				}}
				sessions = append(sessions, s)
				if kind == "tool" {
					return &matrixToolError{s}, nil
				}
				return s, nil
			}
			e := matrixRunWithInputs(ctx, input, cfg, binary, out, 0, 1, 0, 0, time.Second, nil, factory, matrixTestInputReader)
			if e == nil {
				t.Fatal("failure accepted")
			}
			report, rows := matrixReadRows(t, out)
			if report.State != "partial" || len(report.Errors) == 0 || len(rows) == 0 {
				t.Fatalf("lost failure %v %+v", e, report)
			}
			for _, s := range sessions {
				if !s.closed {
					t.Fatal("failure leaked session")
				}
			}
			switch kind {
			case "input":
				if !strings.Contains(strings.Join(report.Errors, " "), "locked input") {
					t.Fatal("input mutation lost")
				}
			case "config":
				if !strings.Contains(strings.Join(report.Errors, " "), "derived config") {
					t.Fatal("config mutation lost")
				}
			case "cancel":
				if len(rows) != 1 {
					t.Fatal("continued after cancellation")
				}
			case "startup":
				if rows[0].Error != "warm session unavailable" {
					t.Fatal("missing arm silently omitted")
				}
			}
		})
	}
}

type matrixToolError struct{ *matrixFake }

func (s *matrixToolError) Capture(ctx context.Context, r eval.CaptureRequest) (eval.RawToolCall, error) {
	call, e := s.matrixFake.Capture(ctx, r)
	call.Response = json.RawMessage(`{"isError":true,"content":[]}`)
	return call, e
}

func TestMatrixRejectsGoldV1OwnedAxisAndExistingOutput(t *testing.T) {
	for _, raw := range []string{`{"schema_version":1,"gold":"secret","requests":[]}`, `{"schema_version":1,"requests":[{"id":"a","tool":"cks.context.get_for_task","arguments":{"prompt":"Alpha"}}]}`, `{"schema_version":1,"requests":[{"id":"a","tool":"cks.context.get_for_task_v2","arguments":{"prompt":"Alpha","include_knowledge":false}}]}`} {
		if _, e := matrixPlan([]byte(raw)); e == nil {
			t.Fatal("invalid plan accepted")
		}
	}
	input, cfg, binary, out := matrixTestFiles(t)
	if e := os.Mkdir(out, 0700); e != nil {
		t.Fatal(e)
	}
	sentinel := filepath.Join(out, "preserved")
	_ = os.WriteFile(sentinel, []byte("unchanged"), 0600)
	factory := func(context.Context, string, string, time.Duration) (matrixSession, error) {
		t.Fatal("started before output check")
		return nil, nil
	}
	e := matrixRunWithInputs(context.Background(), input, cfg, binary, out, 0, 1, 0, 0, time.Second, nil, factory, matrixTestInputReader)
	if !os.IsExist(e) {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(sentinel)
	if string(b) != "unchanged" {
		t.Fatal("overwritten")
	}
}

func TestMatrixRefusesOutputInsideFrozenSource(t *testing.T) {
	input, cfgpath, binary, out := matrixTestFiles(t)
	cfg := config.Default()
	cfg.Backends.CKG.SourceRoot = filepath.Dir(out)
	body, _ := yaml.Marshal(cfg)
	if err := os.WriteFile(cfgpath, body, 0600); err != nil {
		t.Fatal(err)
	}
	factory := func(context.Context, string, string, time.Duration) (matrixSession, error) {
		t.Fatal("started inside frozen source")
		return nil, nil
	}
	err := matrixRunWithInputs(context.Background(), input, cfgpath, binary, out, 0, 1, 0, 0, time.Second, nil, factory, matrixTestInputReader)
	if err == nil || !strings.Contains(err.Error(), "outside source") {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("output mutated frozen input")
	}
}

func TestMatrixOutputBoundaryResolvesParentAlias(t *testing.T) {
	source := t.TempDir()
	parent := t.TempDir()
	alias := filepath.Join(parent, "alias")
	if e := os.Symlink(source, alias); e != nil {
		t.Fatal(e)
	}
	if !matrixWithin(source, filepath.Join(alias, "new-output")) {
		t.Fatal("parent symlink bypasses frozen source boundary")
	}
	if matrixWithin(source, filepath.Join(parent, "separate")) {
		t.Fatal("separate output wrongly inside source")
	}
}

func TestMatrixColdStartupTypedNilErrorNeverCallsSession(t *testing.T) {
	input, cfg, binary, out := matrixTestFiles(t)
	starts := 0
	var sessions []*matrixFake
	factory := func(context.Context, string, string, time.Duration) (matrixSession, error) {
		starts++
		if starts > 8 {
			var session *matrixFake
			return session, fmt.Errorf("initialize deadline typed nil")
		}
		s := &matrixFake{capture: func(eval.CaptureRequest) error { return nil }}
		sessions = append(sessions, s)
		return s, nil
	}
	err := matrixRunWithInputs(context.Background(), input, cfg, binary, out, 0, 1, 0, 1, time.Second, nil, factory, matrixTestInputReader)
	if err == nil {
		t.Fatal("cold startup error accepted")
	}
	report, rows := matrixReadRows(t, out)
	if report.State != "partial" || len(rows) != 32 {
		t.Fatal("missing rows", report.State, len(rows))
	}
	for _, row := range rows[16:] {
		if !strings.Contains(row.Error, "start cold") || row.Call.Response != nil || row.ProcessStartThroughResponseNS <= 0 {
			t.Fatalf("cold startup incorrectly called session: %+v", row)
		}
	}
	for _, s := range sessions {
		if !s.closed {
			t.Fatal("warm session leaked")
		}
	}
}
