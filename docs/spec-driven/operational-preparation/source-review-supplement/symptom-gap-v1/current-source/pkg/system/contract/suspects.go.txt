package contract

import "time"

// Signal is an observation, never a symptom-specific cause judgment.
// Value remains explicit null when absent; status explains why.
type Signal[T any] struct {
	Value      *T           `json:"value"`
	Status     string       `json:"status"`
	Reason     string       `json:"reason,omitempty"`
	Provenance []SignalFrom `json:"provenance"`
}

type SignalFrom struct {
	Tool     string      `json:"tool"`
	Snapshot string      `json:"snapshot"`
	Anchor   Citation    `json:"anchor"`
	Retained *CitationV2 `json:"retained_anchor"`
}

type SuspectRecency struct {
	LastMergedAt *time.Time `json:"last_merged_at"`
	PRCount      int        `json:"pr_count"`
	Scope        string     `json:"scope"`
}

type SuspectConcurrency struct {
	Touched   bool     `json:"touched"`
	Direction []string `json:"direction"`
}

type SuspectRetrieval struct {
	Rank   int       `json:"rank"`
	Score  float64   `json:"score"`
	Source HitSource `json:"source"`
}

type SuspectSignals struct {
	Distance    Signal[int]                `json:"distance"`
	Relations   Signal[[]Relation]         `json:"relations"`
	Impact      Signal[[]ImpactCategory]   `json:"impact_category"`
	Recency     Signal[SuspectRecency]     `json:"recency"`
	Concurrency Signal[SuspectConcurrency] `json:"concurrency"`
	Retrieval   Signal[[]SuspectRetrieval] `json:"retrieval"`
	Security    Signal[[]string]           `json:"security_tag"`
	Dataflow    Signal[bool]               `json:"dataflow_on_path"`
}

type SuspectCandidate struct {
	Citation Citation       `json:"citation"`
	Retained *CitationV2    `json:"retained_citation"`
	Symbol   *string        `json:"symbol"`
	Signals  SuspectSignals `json:"signals"`
}

type SuspectBudget struct {
	MaxCandidates int      `json:"max_candidates"`
	MaxSeeds      int      `json:"max_seeds"`
	MaxEnriched   int      `json:"max_enriched"`
	MaxCalls      int      `json:"max_calls"`
	Calls         int      `json:"calls"`
	Depth         int      `json:"depth"`
	ElapsedMS     int64    `json:"elapsed_ms"`
	TimeBudgetMS  int      `json:"time_budget_ms"`
	StopReasons   []string `json:"stop_reasons"`
}

type SuspectCandidates struct {
	SchemaVersion int                `json:"schema_version"`
	Symptom       string             `json:"symptom"`
	Snapshot      string             `json:"snapshot"`
	Coordinates   *V2Coordinates     `json:"coordinates"`
	Candidates    []SuspectCandidate `json:"candidates"`
	Partial       bool               `json:"partial"`
	Budget        SuspectBudget      `json:"budget"`
	Notes         []string           `json:"notes"`
}
