package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

// reportSchemaVersion is the cks-eval report schema version. Bumped on
// breaking changes only; additive fields with `omitempty` keep
// downstream consumers compatible without a bump.
const reportSchemaVersion = 1

// Report is the top-level cks-eval output. One Report covers an entire
// `cks-eval` invocation; ScenarioResult is the per-scenario row.
//
// IntentSummary is a derived per-intent rollup. The CLI populates it
// before serialization so downstream consumers don't have to re-group
// the per-scenario rows.
type Report struct {
	SchemaVersion  int              `json:"schema_version"`
	BuilderVersion string           `json:"builder_version,omitempty"`
	StartedAt      time.Time        `json:"started_at"`
	FinishedAt     time.Time        `json:"finished_at,omitempty"`
	Results        []ScenarioResult `json:"results"`
	IntentSummary  []IntentSummary  `json:"intent_summary,omitempty"`
	// RankSummary and IntentAccuracy aggregate the per-scenario first-hit
	// rank and expected_intent checks; absent when no scenario supplies them.
	RankSummary    *RankSummary    `json:"rank_summary,omitempty"`
	IntentAccuracy *IntentAccuracy `json:"intent_accuracy,omitempty"`
}

// IntentSummary is the per-intent rollup across all scenarios that
// share that intent. Scenarios with an empty Intent group under the
// sentinel "(unspecified)" so they remain visible. Errored runs are
// skipped from the average so a single broken scenario doesn't drag
// the group's metrics toward zero.
type IntentSummary struct {
	Intent       string  `json:"intent"`
	Count        int     `json:"count"`
	AvgPrecision float64 `json:"avg_precision"`
	AvgMRR       float64 `json:"avg_mrr"`
	AvgRecall    float64 `json:"avg_recall"`
	AvgF1        float64 `json:"avg_f1"`
	AvgLatencyMS int64   `json:"avg_latency_ms"`
}

// SummarizeByIntent buckets results by Intent and computes the average
// metrics inside each bucket. Returns nil for empty input. Output is
// sorted by Intent name so consumers get stable ordering across runs.
func SummarizeByIntent(results []ScenarioResult) []IntentSummary {
	if len(results) == 0 {
		return nil
	}
	const unspecified = "(unspecified)"
	type acc struct {
		count                          int
		sumP, sumM, sumR, sumF, sumLat float64
	}
	buckets := make(map[string]*acc)
	for _, r := range results {
		if r.Error != "" {
			continue
		}
		key := r.Intent
		if key == "" {
			key = unspecified
		}
		a, ok := buckets[key]
		if !ok {
			a = &acc{}
			buckets[key] = a
		}
		a.count++
		a.sumP += r.Metrics.FilePrecision
		a.sumM += r.Metrics.FileMRR
		a.sumR += r.Metrics.FileRecall
		a.sumF += r.Metrics.FileF1
		a.sumLat += float64(r.Metrics.LatencyMS)
	}
	if len(buckets) == 0 {
		return nil
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]IntentSummary, 0, len(keys))
	for _, k := range keys {
		a := buckets[k]
		n := float64(a.count)
		out = append(out, IntentSummary{
			Intent:       k,
			Count:        a.count,
			AvgPrecision: a.sumP / n,
			AvgMRR:       a.sumM / n,
			AvgRecall:    a.sumR / n,
			AvgF1:        a.sumF / n,
			AvgLatencyMS: int64(a.sumLat / n),
		})
	}
	return out
}

// RankSummary is hit@k and per-query MRR over scenarios that declare
// expected citations. A scenario that errored counts as a miss, so a
// failing backend cannot raise the rates by dropping out of the denominator.
type RankSummary struct {
	Scenarios int     `json:"scenarios"`
	HitAt1    float64 `json:"hit_at_1"`
	HitAt3    float64 `json:"hit_at_3"`
	MRR       float64 `json:"mrr"`
}

// IntentAccuracy compares the classified intent with expected_intent.
// Confusion is keyed expected → classified; an errored scenario is
// recorded under classifiedError and counts as wrong.
type IntentAccuracy struct {
	Scenarios int                       `json:"scenarios"`
	Correct   int                       `json:"correct"`
	Accuracy  float64                   `json:"accuracy"`
	Confusion map[string]map[string]int `json:"confusion"`
}

// classifiedError and classifiedUnknown label confusion cells that have
// no classified intent value.
const (
	classifiedError   = "(error)"
	classifiedUnknown = "(unknown)"
)

// rankHitLimits are the k values RankSummary reports.
const (
	hitTop1 = 1
	hitTop3 = 3
)

// SummarizeRanks returns nil when no scenario declares expected citations.
func SummarizeRanks(results []ScenarioResult) *RankSummary {
	var out RankSummary
	var hit1, hit3 int
	var rr float64
	for _, r := range results {
		if r.RetrievalState == "not_evaluated" {
			continue
		}
		out.Scenarios++
		rank := r.Metrics.FirstHitRank
		if r.Error != "" || rank == 0 {
			continue
		}
		if rank <= hitTop1 {
			hit1++
		}
		if rank <= hitTop3 {
			hit3++
		}
		rr += 1 / float64(rank)
	}
	if out.Scenarios == 0 {
		return nil
	}
	n := float64(out.Scenarios)
	out.HitAt1, out.HitAt3, out.MRR = float64(hit1)/n, float64(hit3)/n, rr/n
	return &out
}

// SummarizeIntents returns nil when no scenario declares expected_intent.
func SummarizeIntents(results []ScenarioResult) *IntentAccuracy {
	out := IntentAccuracy{Confusion: map[string]map[string]int{}}
	for _, r := range results {
		if r.ExpectedIntent == "" {
			continue
		}
		out.Scenarios++
		got := classifiedError
		if r.IntentMatch != nil && *r.IntentMatch {
			out.Correct++
		}
		if len(r.ClassifiedIntents) > 0 {
			got = r.ClassifiedIntents[0]
			if got == "" {
				got = classifiedUnknown
			}
		}
		if out.Confusion[r.ExpectedIntent] == nil {
			out.Confusion[r.ExpectedIntent] = map[string]int{}
		}
		out.Confusion[r.ExpectedIntent][got]++
	}
	if out.Scenarios == 0 {
		return nil
	}
	out.Accuracy = float64(out.Correct) / float64(out.Scenarios)
	return &out
}

// NewReport returns a Report with the schema version pre-set.
func NewReport(builderVersion string) Report {
	return Report{
		SchemaVersion:  reportSchemaVersion,
		BuilderVersion: builderVersion,
		StartedAt:      time.Now().UTC(),
	}
}

// WriteJSON serializes rep as indented JSON with a trailing newline,
// suitable for piping to `jq` or committing as a CI artifact.
//
// Indented output is the cks-eval default — a CI run produces one
// report per invocation, so the storage cost is negligible and human
// readability is high. Callers that need compact output can wrap
// json.Marshal directly.
func WriteJSON(w io.Writer, rep Report) error {
	buf, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("eval: marshal report: %w", err)
	}
	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("eval: write report: %w", err)
	}
	if _, err := w.Write([]byte{'\n'}); err != nil {
		return fmt.Errorf("eval: write report newline: %w", err)
	}
	return nil
}
