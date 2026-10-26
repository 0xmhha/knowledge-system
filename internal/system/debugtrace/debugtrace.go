// Package debugtrace records what each retrieval algorithm received and
// produced during one compose call, so an evaluation run can show the exact
// step where a known answer was lost or a wrong citation was promoted.
//
// The footprint logger keeps production telemetry small: counts, the top
// citation, error totals. That is not enough to debug ranking. A debug trace
// keeps the full lists instead — every hit, keyword, contribution and
// selection decision — on both sides of each algorithm:
//
//	stage/step  phase=before  the input the algorithm saw
//	stage/step  phase=after   the output it produced
//
// Tracing is off unless the caller attaches a Recorder to the context.
// Every method is nil-safe, and From returns nil when tracing is off, so a
// call site guards its payload construction with
//
//	if rec := debugtrace.From(ctx); rec != nil {
//		rec.After("stage2", "find_symbol", kw, payload(...))
//	}
//
// and pays nothing in normal operation.
package debugtrace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// SchemaVersion identifies the record layout written by FileSink. Bump it
// when a field changes meaning so analysis scripts can refuse old files.
const SchemaVersion = 1

// EnvPath names the environment variable cks reads to enable tracing: the
// value is the JSONL file each compose call appends one record to.
const EnvPath = "CKS_DEBUG_TRACE"

// Phase values. Before is the algorithm's input, After its output. Note is a
// single observation that has no before/after pair (a decision, a skip).
const (
	PhaseBefore = "before"
	PhaseAfter  = "after"
	PhaseNote   = "note"
)

// Event is one snapshot. Key distinguishes repeated runs of the same step:
// the keyword for per-keyword searches, the round number for recall rounds.
type Event struct {
	Stage string `json:"stage"`
	Step  string `json:"step"`
	Phase string `json:"phase"`
	Key   string `json:"key,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// Recorder collects the events of one compose call. Safe for concurrent use.
type Recorder struct {
	mu     sync.Mutex
	events []Event
}

// NewRecorder returns an empty Recorder.
func NewRecorder() *Recorder { return &Recorder{} }

// Before records an algorithm's input.
func (r *Recorder) Before(stage, step, key string, data any) {
	r.add(Event{Stage: stage, Step: step, Phase: PhaseBefore, Key: key, Data: data})
}

// After records an algorithm's output.
func (r *Recorder) After(stage, step, key string, data any) {
	r.add(Event{Stage: stage, Step: step, Phase: PhaseAfter, Key: key, Data: data})
}

// Note records a single observation with no before/after pair.
func (r *Recorder) Note(stage, step, key string, data any) {
	r.add(Event{Stage: stage, Step: step, Phase: PhaseNote, Key: key, Data: data})
}

func (r *Recorder) add(e Event) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

// Events returns a copy of the recorded events in recording order.
func (r *Recorder) Events() []Event {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

type ctxKey struct{}

// WithRecorder attaches r to ctx. A nil r returns ctx unchanged.
func WithRecorder(ctx context.Context, r *Recorder) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, r)
}

// From returns the Recorder attached to ctx, or nil when tracing is off.
func From(ctx context.Context) *Recorder {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(ctxKey{}).(*Recorder)
	return r
}

// Record is one JSONL line: everything one compose call recorded.
type Record struct {
	Schema       int     `json:"schema"`
	Seq          int     `json:"seq"`
	Prompt       string  `json:"prompt"`
	CallerIntent string  `json:"caller_intent,omitempty"`
	Error        string  `json:"error,omitempty"`
	Events       []Event `json:"events"`
}

// FileSink appends one Record per compose call to a JSONL file. Safe for
// concurrent use; Seq numbers records in write order so repeated compose
// calls for the same prompt stay distinguishable.
type FileSink struct {
	mu   sync.Mutex
	path string
	seq  int
}

// NewFileSink returns a sink that appends to path. The file is created on
// the first write.
func NewFileSink(path string) *FileSink { return &FileSink{path: path} }

var (
	sharedMu    sync.Mutex
	sharedSinks = map[string]*FileSink{}
)

// SharedFileSink returns one process-wide sink per path. A process that
// builds several composers (one per collection member) must share the sink,
// or concurrent writers would interleave lines and repeat Seq numbers.
func SharedFileSink(path string) *FileSink {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if s, ok := sharedSinks[path]; ok {
		return s
	}
	s := NewFileSink(path)
	sharedSinks[path] = s
	return s
}

// Write appends the recorder's events as one line. Errors are returned for
// tests and callers that care; compose ignores them so a full disk never
// fails a retrieval.
func (s *FileSink) Write(prompt string, callerIntent contract.Intent, r *Recorder, composeErr error) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	rec := Record{Schema: SchemaVersion, Seq: s.seq, Prompt: prompt, CallerIntent: string(callerIntent), Events: r.Events()}
	if composeErr != nil {
		rec.Error = composeErr.Error()
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("debugtrace: marshal: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("debugtrace: open %s: %w", s.path, err)
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("debugtrace: write %s: %w", s.path, err)
	}
	return f.Close()
}

// Cite is the compact form of a citation in a trace payload.
type Cite struct {
	File    string   `json:"file"`
	Start   int      `json:"start"`
	End     int      `json:"end"`
	Symbol  string   `json:"symbol,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Score   float64  `json:"score,omitempty"`
	Sources []string `json:"sources,omitempty"`
}

// Hits converts ckv/ckg hits in list order.
func Hits(hs []contract.Hit) []Cite {
	out := make([]Cite, 0, len(hs))
	for _, h := range hs {
		out = append(out, Cite{File: h.Citation.File, Start: h.Citation.StartLine, End: h.Citation.EndLine,
			Symbol: h.Symbol, Kind: h.ChunkKind, Score: h.Score})
	}
	return out
}

// Citations converts bare citations in list order.
func Citations(cs []contract.Citation) []Cite {
	out := make([]Cite, 0, len(cs))
	for _, c := range cs {
		out = append(out, Cite{File: c.File, Start: c.StartLine, End: c.EndLine})
	}
	return out
}
