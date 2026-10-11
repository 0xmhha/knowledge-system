package mcp

import (
	"context"
	"errors"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// ToolNameReviewFunction is the wire name of the function review tool.
var ToolNameReviewFunction = toolName("context.review_function")

// reviewLimits names what the graph cannot show, so a caller does not read
// an empty dynamic section as "no dynamic behaviour".
var reviewLimits = []string{
	"calls through function values, callbacks, reflection and generated code have no graph edges and do not appear in callers or callees",
	"a channel send or receive is listed, but the goroutine on the other end is not resolved",
	"history covers the commits the index walked per file (a bounded recent window), not the whole repository history",
}

type reviewCodeView struct {
	Signature             string                 `json:"signature"`
	UsesTypes             []store.ReviewSymbol   `json:"uses_types"`
	Instantiates          []store.ReviewSymbol   `json:"instantiates"`
	WritesFields          []store.ReviewSymbol   `json:"writes_fields"`
	ReturnStatements      int                    `json:"return_statements"`
	Control               map[string]int         `json:"control"`
	CallSites             []store.ReviewCallSite `json:"call_sites"`
	CallSitesTotal        int                    `json:"call_sites_total"`
	Callees               []store.ReviewSymbol   `json:"callees"`
	InterfaceImplementers map[string][]string    `json:"interface_implementers"`
	Callers               []store.ReviewSymbol   `json:"callers"`
	ViaInterfaces         []store.ReviewSymbol   `json:"via_interfaces"`
	CallerPaths           [][]string             `json:"caller_paths"`
	EntryPoints           int                    `json:"entry_points"`
	CallerWalkTruncated   bool                   `json:"caller_walk_truncated"`
	Goroutines            []store.ReviewSymbol   `json:"goroutines"`
	SendsTo               []store.ReviewSymbol   `json:"sends_to"`
	RecvsFrom             []store.ReviewSymbol   `json:"recvs_from"`
	AcquiresLocks         []store.ReviewSymbol   `json:"acquires_locks"`
	ReleasesLocks         []store.ReviewSymbol   `json:"releases_locks"`
	TimeoutPaths          []store.ReviewSymbol   `json:"timeout_paths"`
	CancellationPaths     []store.ReviewSymbol   `json:"cancellation_paths"`
}

type reviewHistoryView struct {
	Commits      []store.ReviewCommit `json:"commits"`
	CommitsTotal int                  `json:"commits_total"`
	PRs          []contract.PRRef     `json:"prs"`
}

type reviewTestsView struct {
	Callers      []store.ReviewSymbol `json:"callers"`
	CallersTotal int                  `json:"callers_total"`
}

// reviewFunctionResponse keeps the three views apart: code is production
// code only, history and tests are lookups that are never walked.
type reviewFunctionResponse struct {
	Seed      store.ReviewSymbol `json:"seed"`
	Code      reviewCodeView     `json:"code"`
	History   reviewHistoryView  `json:"history"`
	Tests     reviewTestsView    `json:"tests"`
	Available []string           `json:"available"`
	Limits    []string           `json:"limits"`
}

func registerReviewFunction(s *mcpserver.MCPServer, d Deps) {
	tool := mcpgo.NewTool(ToolNameReviewFunction,
		mcpgo.WithOutputSchema[reviewFunctionResponse](),
		mcpgo.WithDescription(
			"Review one function the way a person reads it, in three separate views. "+
				"code (production code only): signature, used/created types and written fields, "+
				"control statements and call sites in source order, callees with the implementations "+
				"behind interface calls, callers and call paths back to entry points (also through the "+
				"interfaces a method implements), goroutines, channels, locks, timeout/cancellation paths. "+
				"history: the commits and PRs that changed the function. tests: its test callers, listed "+
				"but not followed. Cheaper and more focused than get_subgraph, which mixes all edge types.",
		),
		mcpgo.WithString("symbol", mcpgo.Required(),
			mcpgo.Description("Function or method: a ckg qualified_name, a bare name that resolves unambiguously, or a canonical_id.")),
		mcpgo.WithNumber("caller_depth",
			mcpgo.Description("How far to walk callers toward entry points (1-5, default 5).")),
		mcpgo.WithNumber("max_items",
			mcpgo.Description("Entries per list (default 40); totals are always reported.")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return handleReviewFunction(ctx, d, req)
	})
}

func handleReviewFunction(ctx context.Context, d Deps, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if denied := legacyContextGuard(d); denied != nil {
		return denied, nil
	}
	symbol := req.GetString("symbol", "")
	if symbol == "" {
		return mcpgo.NewToolResultError(ToolNameReviewFunction + ": missing required argument \"symbol\""), nil
	}
	opts := ckgclient.ReviewOpts{
		CallerDepth: min(max(intArg(req, "caller_depth", 5), 1), maxTraversalDepth),
		MaxItems:    min(max(intArg(req, "max_items", 40), 1), 200),
	}
	rv, err := d.CKG.ReviewFunction(ctx, symbol, opts)
	if errors.Is(err, store.ErrNoReviewIndex) {
		return mcpgo.NewToolResultErrorf("%s: requires_reindex: this graph predates the review index (schema 1.24); rebuild the graph, or use find_callers/get_subgraph meanwhile", ToolNameReviewFunction), nil
	}
	if err != nil {
		return mcpgo.NewToolResultErrorf("%s: %v", ToolNameReviewFunction, err), nil
	}
	resp := reviewFunctionResponse{
		Seed: rv.Seed,
		Code: reviewCodeView{
			Signature: rv.Signature, UsesTypes: rv.UsesTypes, Instantiates: rv.Instantiates, WritesFields: rv.WritesFields,
			ReturnStatements: rv.ReturnStatements, Control: rv.Control, CallSites: rv.CallSites, CallSitesTotal: rv.CallSitesTotal,
			Callees: rv.Callees, InterfaceImplementers: rv.InterfaceImplementers, Callers: rv.Callers, ViaInterfaces: rv.ViaInterfaces,
			CallerPaths: rv.CallerPaths, EntryPoints: rv.EntryPoints, CallerWalkTruncated: rv.CallerWalkTruncated, Goroutines: rv.Goroutines, SendsTo: rv.SendsTo, RecvsFrom: rv.RecvsFrom,
			AcquiresLocks: rv.AcquiresLocks, ReleasesLocks: rv.ReleasesLocks, TimeoutPaths: rv.TimeoutPaths, CancellationPaths: rv.CancellationPaths,
		},
		History: reviewHistoryView{Commits: rv.Commits, CommitsTotal: rv.CommitsTotal},
		Tests:   reviewTestsView{Callers: rv.TestCallers, CallersTotal: rv.TestCallersTotal},
		Limits:  reviewLimits,
	}
	prs, prErr := d.CKG.GetNodePRs(ctx, rv.Seed.QualifiedName, ckgclient.PRRefOpts{MaxCount: opts.MaxItems})
	if prErr == nil {
		resp.History.PRs = prs
	}
	// Say what else exists, so a caller knows to ask for it.
	resp.Available = append(resp.Available,
		fmt.Sprintf("history: %d commits and %d PRs changed this function; diffs and related past changes: %s",
			rv.CommitsTotal, len(resp.History.PRs), ToolNameChangeHistory),
		fmt.Sprintf("tests: %d test functions call it; %s lists them (it includes tests unless exclude_tests=true)",
			rv.TestCallersTotal, ToolNameFindCallers))
	if prErr != nil {
		resp.Available = append(resp.Available, "PRs could not be read: "+prErr.Error())
	}
	return mcpgo.NewToolResultStructured(resp, ToolNameReviewFunction+" result"), nil
}
