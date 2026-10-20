package valueflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

type ExpansionLimits struct {
	MaxFunctions  int `json:"max_functions"`
	MaxHops       int `json:"max_hops"`
	MaxCandidates int `json:"max_candidates"`
	MaxTotalNodes int `json:"max_total_nodes"`
	MaxTotalEdges int `json:"max_total_edges"`
	MaxTotalFacts int `json:"max_total_facts"`
	TimeoutMS     int `json:"timeout_ms"`
}
type ExpansionInput struct {
	Function QueryInput      `json:"function"`
	Limits   ExpansionLimits `json:"limits,omitempty"`
}
type ExpandedFunction struct {
	Hop    int         `json:"hop"`
	Result QueryResult `json:"result"`
}
type SymbolHop struct {
	SourceNode      string `json:"source_node"`
	TargetNode      string `json:"target_node"`
	Relation        string `json:"relation"`
	Origin          string `json:"origin"`
	ArgumentMapping string `json:"argument_mapping"`
	SourceFile      string `json:"source_file,omitempty"`
	SourceLine      int    `json:"source_line,omitempty"`
}
type ExpansionResult struct {
	SchemaVersion  int                `json:"schema_version"`
	Status         string             `json:"status"`
	SnapshotSHA256 string             `json:"snapshot_sha256,omitempty"`
	Limits         ExpansionLimits    `json:"limits"`
	Functions      []ExpandedFunction `json:"functions"`
	Hops           []SymbolHop        `json:"hops"`
	Candidates     int                `json:"candidates"`
	Deduplicated   int                `json:"deduplicated"`
	TotalNodes     int                `json:"total_nodes"`
	TotalEdges     int                `json:"total_edges"`
	TotalFacts     int                `json:"total_facts"`
	StopReasons    []string           `json:"stop_reasons"`
	Limitations    []string           `json:"limitations"`
	ElapsedMicros  int64              `json:"elapsed_micros"`
}

func expansionLimits(o ExpansionLimits) (ExpansionLimits, error) {
	defaults := ExpansionLimits{4, 1, 32, 2048, 8192, 131072, 1000}
	requested := []*int{&o.MaxFunctions, &o.MaxHops, &o.MaxCandidates, &o.MaxTotalNodes, &o.MaxTotalEdges, &o.MaxTotalFacts, &o.TimeoutMS}
	values := []int{defaults.MaxFunctions, defaults.MaxHops, defaults.MaxCandidates, defaults.MaxTotalNodes, defaults.MaxTotalEdges, defaults.MaxTotalFacts, defaults.TimeoutMS}
	caps := []int{8, 3, 128, 4096, 16384, 262144, 5000}
	for i, v := range requested {
		if *v == 0 {
			*v = values[i]
		}
		if *v < 1 || *v > caps[i] {
			return o, fmt.Errorf("expansion limit outside supported bounds")
		}
	}
	return o, nil
}
func DecodeExpansionInput(raw []byte) (ExpansionInput, error) {
	var q ExpansionInput
	if len(raw) > 8192 {
		return q, fmt.Errorf("expansion input exceeds8KiB")
	}
	if e := validateInputObject(raw); e != nil {
		return q, e
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&q); e != nil {
		return q, e
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	function, e := DecodeQueryInput(fields["function"])
	if e != nil {
		return q, e
	}
	q.Function = function
	if field, ok := fields["limits"]; ok {
		var values map[string]int
		if e = json.Unmarshal(field, &values); e != nil {
			return q, e
		}
		for _, v := range values {
			if v < 1 {
				return q, fmt.Errorf("explicit positive expansion limits required")
			}
		}
	}
	_, e = expansionLimits(q.Limits)
	return q, e
}

// Expand composes distinct function-local computations through retained call
// hops. A hop is never claimed as computed argument/return or memory transfer.
func Expand(ctx context.Context, r store.Reader, q ExpansionInput) (out ExpansionResult, err error) {
	started := time.Now()
	defer func() { out.ElapsedMicros = time.Since(started).Microseconds() }()
	o, e := expansionLimits(q.Limits)
	if e != nil {
		return out, e
	}
	if e = q.Function.validate(); e != nil {
		return out, e
	}
	if ctx == nil || r == nil {
		return out, fmt.Errorf("context and graph reader required")
	}
	out = ExpansionResult{SchemaVersion: 1, Status: "expanded_bounded", Limits: o, Functions: []ExpandedFunction{}, Hops: []SymbolHop{}, StopReasons: []string{}, Limitations: []string{"local_value_control_analysis_not_whole_program", "call_hops_not_computed_argument_or_return_transfer", "curated_flow_is_separate_operator_metadata", "memory_alias_external_effects_and_path_feasibility_unknown", "logical_fact_and_output_caps_not_process_memory_bounds", "deadline_cooperative_for_parser_typechecker_and_legacy_reader"}}
	bounded, cancel := context.WithTimeout(ctx, time.Duration(o.TimeoutMS)*time.Millisecond)
	defer cancel()
	stop := func(reason string) {
		out.Status = "budget_exhausted"
		for _, s := range out.StopReasons {
			if s == reason {
				return
			}
		}
		out.StopReasons = append(out.StopReasons, reason)
	}
	invalid := func(status, reason string) {
		out.Status = status
		out.Functions = []ExpandedFunction{}
		out.Hops = []SymbolHop{}
		out.StopReasons = []string{reason}
	}
	type pending struct {
		input QueryInput
		hop   int
	}
	queue := []pending{{q.Function, 0}}
	seen := map[string]bool{}
	seenCandidates := map[string]bool{}
	// Re-read the exact consulted node/edge/blob records at the end, not only a
	// manifest. A custom mutable reader must not mix intermediate snapshots.
	retainedNodes := map[string]types.Node{}
	retainedBlobs := map[string]string{}
	pages := map[string][]types.Edge{}
	pageMore := map[string]bool{}
	pageInputs := map[string]store.EdgePageInput{}
	var seedSnapshot string
	for len(queue) > 0 {
		if bounded.Err() != nil {
			stop("deadline")
			break
		}
		if len(out.Functions) >= o.MaxFunctions {
			stop("functions")
			break
		}
		item := queue[0]
		queue = queue[1:]
		if item.input.NodeID != "" && seen[item.input.NodeID] {
			out.Deduplicated++
			continue
		}
		remainingNodes := o.MaxTotalNodes - out.TotalNodes
		remainingEdges := o.MaxTotalEdges - out.TotalEdges - len(out.Hops)
		remainingFacts := o.MaxTotalFacts - out.TotalFacts
		if min(remainingNodes, remainingEdges, remainingFacts) < 1 {
			stop("aggregate_analysis")
			break
		}
		opts, e := limits(item.input.Limits)
		if e != nil {
			return out, e
		}
		opts.MaxNodes = min(opts.MaxNodes, remainingNodes)
		opts.MaxEdges = min(opts.MaxEdges, remainingEdges)
		opts.MaxFacts = min(opts.MaxFacts, remainingFacts)
		item.input.Limits = opts
		if seedSnapshot != "" {
			item.input.ExpectedSnapshotSHA256 = seedSnapshot
		}
		result, e := Query(bounded, r, item.input)
		if e != nil {
			return out, e
		}
		if result.Status == "snapshot_changed" || result.Status == "source_mismatch" {
			invalid(result.Status, "source_or_snapshot_changed")
			return out, nil
		}
		out.Functions = append(out.Functions, ExpandedFunction{item.hop, result})
		if result.Analysis == nil {
			if len(out.Functions) == 1 {
				out.Status = result.Status
			}
			out.StopReasons = append(out.StopReasons, "function_"+result.Status)
			continue
		}
		a := result.Analysis
		id := a.Source.NodeID
		seen[id] = true
		if seedSnapshot == "" {
			seedSnapshot = result.SnapshotSHA256
			out.SnapshotSHA256 = seedSnapshot
		}
		out.TotalNodes += len(a.Nodes)
		out.TotalEdges += len(a.Edges)
		out.TotalFacts += a.FactsPeak
		if a.Truncated {
			stop("function_analysis")
			break
		}
		nodes, e := r.NodesByIDs([]string{id})
		if e != nil || len(nodes) != 1 {
			invalid("snapshot_changed", "selected_node_unavailable")
			return out, nil
		}
		retainedNodes[id] = nodes[0]
		retainedBlobs[id] = a.Source.SHA256
		if item.hop >= o.MaxHops {
			out.StopReasons = append(out.StopReasons, "hop_boundary")
			continue
		}
		edgesReader, ok := r.(store.BoundedEdgeReader)
		if !ok {
			out.Status = "partial"
			out.StopReasons = append(out.StopReasons, "bounded_reader_unavailable")
			continue
		}
		remainingCandidates := o.MaxCandidates - out.Candidates
		if remainingCandidates < 1 {
			stop("candidates")
			break
		}
		request := store.EdgePageInput{NodeID: id, Direction: "outgoing", Types: []string{"calls", "invokes"}, Limit: min(remainingCandidates, 256)}
		edges, more, e := edgesReader.IncidentEdges(bounded, request)
		if e != nil {
			if bounded.Err() != nil {
				stop("deadline")
			} else {
				out.Status = "partial"
				out.StopReasons = append(out.StopReasons, "bounded_edges_unavailable")
			}
			break
		}
		pages[id] = edges
		pageMore[id] = more
		pageInputs[id] = request
		if more {
			stop("candidates")
		}
		for _, edge := range edges {
			if len(out.Hops)+out.TotalEdges >= o.MaxTotalEdges {
				stop("aggregate_edges")
				break
			}
			if edge.Src != id || (edge.Type != "calls" && edge.Type != "invokes") {
				invalid("unavailable", "reader_edge_contract_invalid")
				return out, nil
			}
			out.Hops = append(out.Hops, SymbolHop{id, edge.Dst, string(edge.Type), "retained_call_graph", "not_computed", edge.FilePath, edge.Line})
			if seenCandidates[edge.Dst] || seen[edge.Dst] {
				out.Deduplicated++
				continue
			}
			seenCandidates[edge.Dst] = true
			out.Candidates++
			target, e := r.NodesByIDs([]string{edge.Dst})
			if e != nil || len(target) != 1 {
				out.StopReasons = append(out.StopReasons, "unresolved_call_target")
				continue
			}
			retainedNodes[edge.Dst] = target[0]
			if target[0].Type != types.NodeFunction && target[0].Type != types.NodeMethod {
				out.StopReasons = append(out.StopReasons, "non_function_target")
				continue
			}
			queue = append(queue, pending{QueryInput{NodeID: edge.Dst, Limits: q.Function.Limits}, item.hop + 1})
		}
	}
	// Verification is part of the same budget. A missed deadline cannot certify
	// a complete expansion, even when some earlier local calculations succeeded.
	if bounded.Err() != nil {
		stop("deadline")
		return out, nil
	}
	if seedSnapshot != "" {
		m, e := r.GetManifest()
		if e != nil {
			invalid("unavailable", "final_manifest_unavailable")
			return out, nil
		}
		raw, e := json.Marshal(m)
		if e != nil {
			return out, e
		}
		if hashBytes(raw) != seedSnapshot {
			invalid("snapshot_changed", "final_manifest_changed")
			return out, nil
		}
	}
	for id, expected := range retainedNodes {
		nodes, e := r.NodesByIDs([]string{id})
		if e != nil || len(nodes) != 1 || !reflect.DeepEqual(nodes[0], expected) {
			invalid("snapshot_changed", "final_node_changed")
			return out, nil
		}
	}
	for id, expected := range retainedBlobs {
		blob, e := r.GetBlob(id)
		if e != nil || hashBytes(blob) != expected {
			invalid("snapshot_changed", "final_blob_changed")
			return out, nil
		}
	}
	for id, expected := range pages {
		edges, more, e := r.(store.BoundedEdgeReader).IncidentEdges(bounded, pageInputs[id])
		if e != nil || more != pageMore[id] || !reflect.DeepEqual(edges, expected) {
			invalid("snapshot_changed", "final_edge_page_changed")
			return out, nil
		}
	}
	if bounded.Err() != nil {
		stop("deadline")
	}
	return out, nil
}
