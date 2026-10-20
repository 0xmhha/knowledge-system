package comparison

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/collection"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
)

func TestRuntimeFinalVerificationDropsEarlierMutatedProjectOnly(t *testing.T) {
	r, _, q, f := runtimeFixture(t)
	manifest := filepath.Join(f.Versions[0].VersionDir, "sources/manifest.json")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.WriteFile(manifest, original, 0o600); err != nil {
			t.Error(err)
		}
	})
	open := r.openReader
	r.openReader = func(ctx context.Context, c collection.Catalog, s *Scheme, cleaner *sanitize.Engine) (*MappingReader, error) {
		if c.Versions[0].Coordinates == f.Versions[1].Coordinates {
			if err := os.Remove(manifest); err != nil {
				return nil, err
			}
		}
		return open(ctx, c, s, cleaner)
	}
	out := executeFixture(t, r, q)
	if out.State != "partial" || out.Groups[1].Cells[0].State != "similar" || len(out.Candidates) != 0 || out.UsedCitations != 5 {
		t.Fatal("later read did not invalidate earlier evidence only")
	}
	for _, cell := range out.Groups[0].Cells {
		if cell.State != "unavailable" || cell.ArchiveVerified || len(cell.Mappings) != 0 {
			t.Fatal("earlier mutated evidence escaped")
		}
	}
	for i, group := range out.Groups {
		if group.Participant.ScopeID != q.Participants[i] {
			t.Fatal("participant fallback")
		}
	}
}

func TestRuntimeCountsCandidateProofsAndOmitsWholeCandidatesAtBudget(t *testing.T) {
	for _, tc := range []struct {
		name                                          string
		limit, wantCites, wantCandidates, wantOmitted int
		candidateState                                string
	}{
		{"all proofs fit", 20, 20, 1, 0, "complete"},
		{"proof omitted after both cells", 10, 10, 0, 1, "budget_exceeded"},
		{"one cell omitted", 9, 5, 0, 0, "complete"},
		{"no complete cell fits", 1, 0, 0, 0, "complete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, q, _ := runtimeFixture(t)
			q.Budget.MaxCitations = tc.limit
			out := executeFixture(t, r, q)
			cites, body := 0, 0
			for _, g := range out.Groups {
				for _, c := range g.Cells {
					b, n := cellCost(c)
					body += b
					cites += n
				}
			}
			for _, c := range out.Candidates {
				for _, e := range c.Evidence {
					cites += len(e.Citations)
				}
			}
			if out.UsedCitations != tc.wantCites || cites != out.UsedCitations || cites > tc.limit || body != out.UsedBodyBytes || len(out.Candidates) != tc.wantCandidates || out.OmittedCandidates != tc.wantOmitted || out.CandidateState != tc.candidateState {
				t.Fatalf("budget accounting %+v", out)
			}
		})
	}
	t.Run("body boundary retains complete first project", func(t *testing.T) {
		r, _, q, _ := runtimeFixture(t)
		full := executeFixture(t, r, q)
		body, _ := cellCost(full.Groups[0].Cells[0])
		q.Budget.MaxBodyBytes = body
		limited := executeFixture(t, r, q)
		if limited.UsedBodyBytes != body || limited.Groups[0].Cells[0].State != "similar" || limited.Groups[1].Cells[0].State != "budget_exceeded" || len(limited.Groups[1].Cells[0].Mappings) != 0 {
			t.Fatal("partial source body escaped")
		}
	})
}

func TestRuntimeDistinguishesSameScopeConflictFromProjectDifference(t *testing.T) {
	for _, same := range []bool{true, false} {
		name := "between projects"
		if same {
			name = "within one project"
		}
		t.Run(name, func(t *testing.T) {
			r, _, q, _ := runtimeFixture(t)
			if same {
				negative := r.bindings.Bindings[0]
				negative.ID = "opposed-applicability"
				negative.Applicability = "not_applicable"
				negative.Reason = "Controlled contradictory applicability declaration; not approved."
				for _, e := range negative.Evidence {
					if e.Kind == "policy" {
						negative.Evidence = []BindingEvidence{e}
						break
					}
				}
				r.bindings.Bindings = append(r.bindings.Bindings, negative)
			} else {
				r.bindings.Bindings[1].Applicability = "not_applicable"
			}
			out := executeFixture(t, r, q)
			if same {
				if out.Groups[0].Cells[0].State != "conflict" || out.Groups[1].Cells[0].State != "similar" {
					t.Fatal("same-scope conflict hidden or spread")
				}
			} else if out.Groups[0].Cells[0].State != "similar" || out.Groups[1].Cells[0].State != "not_applicable" {
				t.Fatal("different projects merged into internal conflict")
			}
		})
	}
}

func TestRuntimeMissingBindingsAndArchivesNeverBecomeFeatureAbsence(t *testing.T) {
	t.Run("one project not collected", func(t *testing.T) {
		r, _, q, _ := runtimeFixture(t)
		r.bindings.Bindings = r.bindings.Bindings[:1]
		out := executeFixture(t, r, q)
		if out.Groups[0].Cells[0].State != "similar" || out.Groups[1].Cells[0].State != "not_collected" || out.Groups[1].Cells[1].State != "not_collected" || len(out.Candidates) != 0 {
			t.Fatal("missing mapping became absence or inherited another project")
		}
	})
	t.Run("all archives unavailable", func(t *testing.T) {
		r, _, q, f := runtimeFixture(t)
		for _, v := range f.Versions {
			if err := os.Remove(filepath.Join(v.VersionDir, "sources/manifest.json")); err != nil {
				t.Fatal(err)
			}
		}
		out := executeFixture(t, r, q)
		if out.State != "partial" || out.UsedCitations != 0 || out.UsedBodyBytes != 0 || len(out.Candidates) != 0 || len(out.Groups) != 2 {
			t.Fatal("unavailable inputs silently omitted")
		}
		for i, g := range out.Groups {
			if g.Participant.ScopeID != q.Participants[i] {
				t.Fatal("changed target")
			}
			for _, c := range g.Cells {
				if c.State != "unavailable" || len(c.Mappings) != 0 {
					t.Fatal("unavailable treated as feature absence")
				}
			}
		}
	})
}

func TestRuntimeCancellationDrainsReadersBeforeClose(t *testing.T) {
	r, _, q, f := runtimeFixture(t)
	entered := make(chan struct{})
	open := r.openReader
	r.openReader = func(ctx context.Context, c collection.Catalog, s *Scheme, cleaner *sanitize.Engine) (*MappingReader, error) {
		if c.Versions[0].Coordinates == f.Versions[1].Coordinates {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return open(ctx, c, s, cleaner)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		bundle ComparisonBundle
		err    error
	}
	done := make(chan result, 1)
	go func() { bundle, err := r.Extract(ctx, data); done <- result{bundle, err} }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("second reader not entered")
	}
	closed := make(chan error, 1)
	go func() { closed <- r.Close() }()
	select {
	case <-closed:
		t.Fatal("close returned with request and reader active")
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case out := <-done:
		if !errors.Is(out.err, context.Canceled) || len(out.bundle.Groups) != 0 {
			t.Fatal("canceled response released earlier evidence")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled request stuck")
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not drain request")
	}
	if _, err := r.Describe(context.Background()); !errors.Is(err, ErrComparisonClosed) {
		t.Fatal("served after close")
	}
}

func TestRuntimeRequestDeadlineNeverReleasesPartialSuccess(t *testing.T) {
	r, _, q, _ := runtimeFixture(t)
	r.openReader = func(ctx context.Context, _ collection.Catalog, _ *Scheme, _ *sanitize.Engine) (*MappingReader, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	q.Budget.TimeoutMS = 1
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Extract(context.Background(), data)
	if !errors.Is(err, context.DeadlineExceeded) || len(out.Groups) != 0 || out.UsedCitations != 0 {
		t.Fatal("deadline returned apparent successful evidence")
	}
}

func TestRuntimeProfileFilesCannotExpandOrLinkConfiguredScope(t *testing.T) {
	for _, role := range []string{"catalog", "bindings", "sanitization", "scheme"} {
		t.Run("symlink "+role, func(t *testing.T) {
			_, path, _, _ := runtimeFixture(t)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var p ComparisonProfileDocument
			if err = json.Unmarshal(data, &p); err != nil {
				t.Fatal(err)
			}
			target := &p.CatalogPath
			switch role {
			case "bindings":
				target = &p.BindingsPath
			case "sanitization":
				target = &p.SanitizeRulesPath
			case "scheme":
				target = &p.SchemeRoot
			}
			link := filepath.Join(filepath.Dir(path), "linked-input")
			if err = os.Symlink(*target, link); err != nil {
				t.Fatal(err)
			}
			*target = link
			data, err = json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if runtime, err := OpenComparisonRuntime(context.Background(), path); !errors.Is(err, ErrComparisonProfile) {
				if runtime != nil {
					_ = runtime.Close()
				}
				t.Fatal("linked selected input allowed")
			}
		})
	}
	t.Run("binding for unconfigured condition", func(t *testing.T) {
		r, path, _, f := runtimeFixture(t)
		r.bindings.Bindings[1].Conditions.Network = "unregistered-network"
		data, err := json.Marshal(r.bindings)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(f.BindingsPath, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if fresh, err := OpenComparisonRuntime(context.Background(), path); !errors.Is(err, ErrComparisonProfile) {
			if fresh != nil {
				_ = fresh.Close()
			}
			t.Fatal("binding expanded configured scope")
		}
	})
}
