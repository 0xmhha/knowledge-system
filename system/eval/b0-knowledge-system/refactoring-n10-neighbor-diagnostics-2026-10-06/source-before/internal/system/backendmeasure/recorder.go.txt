// Package backendmeasure records logical client calls and scoped Ollama HTTP
// transport attempts. It does not count SQLite statements or model internals.
package backendmeasure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/envelope"
)

type Call struct {
	Ordinal           int             `json:"ordinal"`
	Backend           string          `json:"backend"`
	Method            string          `json:"method"`
	InputSHA256       string          `json:"input_sha256,omitempty"`
	InputBytes        int             `json:"input_bytes"`
	Options           json.RawMessage `json:"options,omitempty"`
	Outcome           string          `json:"outcome"`
	ResultCount       int             `json:"result_count"`
	ElapsedNS         int64           `json:"elapsed_ns"`
	HTTPStatus        int             `json:"http_status,omitempty"`
	ResponseBodyBytes int64           `json:"response_body_bytes,omitempty"`
}

type Summary struct {
	MeasurementID string `json:"measurement_id"`
	Tool          string `json:"tool"`
	Outcome       string `json:"outcome"`
	Calls         []Call `json:"calls"`
	PendingCalls  int    `json:"pending_calls"`
}

type Recorder struct {
	Emit func(context.Context, Summary)
}

type scopeKey struct{}

type Scope struct {
	mu       sync.Mutex
	closed   bool
	summary  Summary
	recorder *Recorder
	ctx      context.Context
}

// Begin creates an isolated scope. No raw prompt, symbol name, result body,
// vector, or backend error message enters the recorder.
func (r *Recorder) Begin(ctx context.Context, id, tool string) (context.Context, *Scope) {
	if id == "" {
		id = envelope.NewTraceID()
	}
	ctx = envelope.WithTraceID(ctx, id)
	s := &Scope{summary: Summary{MeasurementID: id, Tool: tool, Calls: []Call{}}, recorder: r, ctx: ctx}
	return context.WithValue(ctx, scopeKey{}, s), s
}

// Finish records attempted calls even when a cooperative backend has not
// returned. Late completions cannot mutate an already emitted summary.
func (s *Scope) Finish(outcome string) Summary {
	s.mu.Lock()
	if s.closed {
		value := s.summary
		s.mu.Unlock()
		return value
	}
	s.closed = true
	s.summary.Outcome = outcome
	for _, call := range s.summary.Calls {
		if call.Outcome == "in_flight" {
			s.summary.PendingCalls++
		}
	}
	s.summary.Calls = append([]Call{}, s.summary.Calls...)
	value := s.summary
	s.mu.Unlock()
	if s.recorder != nil && s.recorder.Emit != nil {
		s.recorder.Emit(s.ctx, value)
	}
	return value
}

func begin(ctx context.Context, backend, method, input string, opts any) func(int, error) {
	finish := start(ctx, backend, method, input, opts)
	return func(count int, err error) { finish(count, 0, 0, -1, err) }
}

// BeginHTTP observes physical Ollama requests inside a request/anchor scope.
// Constructor probes outside those scopes remain uncounted. These records
// must be counted separately from the parent CKV/intent logical calls.
func BeginHTTP(ctx context.Context, method, path string, requestBytes int64) func(int, int64, int64, error) {
	if _, ok := ctx.Value(scopeKey{}).(*Scope); !ok {
		return nil
	}
	finish := start(ctx, "ollama_http", method, "", struct {
		Path             string `json:"path"`
		RequestBodyBytes int64  `json:"request_body_bytes"`
	}{path, requestBytes})
	return func(status int, responseBytes, elapsedNS int64, err error) {
		finish(0, status, responseBytes, elapsedNS, err)
	}
}

func start(ctx context.Context, backend, method, input string, opts any) func(int, int, int64, int64, error) {
	s, _ := ctx.Value(scopeKey{}).(*Scope)
	if s == nil {
		return func(int, int, int64, int64, error) {}
	}
	call := Call{Backend: backend, Method: method, InputBytes: len(input), Outcome: "in_flight"}
	if input != "" {
		sum := sha256.Sum256([]byte(input))
		call.InputSHA256 = hex.EncodeToString(sum[:])
	}
	if opts != nil {
		call.Options, _ = json.Marshal(opts)
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return func(int, int, int64, int64, error) {}
	}
	index := len(s.summary.Calls)
	call.Ordinal = index + 1
	s.summary.Calls = append(s.summary.Calls, call)
	s.mu.Unlock()
	started := time.Now()
	return func(count, status int, responseBytes, measuredElapsed int64, err error) {
		elapsed := time.Since(started).Nanoseconds()
		if measuredElapsed >= 0 {
			elapsed = measuredElapsed
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.closed {
			return
		}
		call := &s.summary.Calls[index]
		call.ElapsedNS, call.ResultCount = elapsed, count
		call.HTTPStatus, call.ResponseBodyBytes = status, responseBytes
		call.Outcome = "returned"
		if status >= 400 {
			call.Outcome = "http_error"
		}
		if err != nil {
			call.Outcome = "backend_error"
			if errors.Is(err, context.Canceled) {
				call.Outcome = "cancelled"
			}
			if errors.Is(err, context.DeadlineExceeded) {
				call.Outcome = "deadline_exceeded"
			}
		}
	}
}
