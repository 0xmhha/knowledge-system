package mcphandlers

import (
	"context"
	"sort"
	"time"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
)

const defaultChangeHistoryK = 20

// historyKey and historyUnavailable are the key and the text that
// change_history and evidence_for_intent add to their answer when the graph
// was built from a source without git history. Both tools read only what
// the build took from git (merged pull requests, commit hunks), so without
// the key an empty answer could not be told from "no change matched".
const (
	historyKey         = "history"
	historyUnavailable = "not recorded: the graph was built from a source without git history, so no commit or pull request is known for any symbol"
)

// historyNote returns historyUnavailable when the graph records no git
// history, "" otherwise. A build records the commit of its source in the
// manifest (src_commit) when the source root is a git checkout with
// commits, and it reads commits and pull requests only then: an empty
// src_commit means none were read. A manifest that cannot be read says
// nothing either way, so no note is given.
func historyNote(reader store.Reader) string {
	m, err := store.GetManifest(reader)
	if err != nil || m.CommitHash != "" {
		return ""
	}
	return historyUnavailable
}

// RegisterChangeHistory wires the change_history tool: the merged pull
// requests that touched a symbol (number, title, summary, merged-at), newest
// first. It exposes the graph's existing per-node PR breadcrumbs
// (Reader.GetNodePRs) over MCP so an agent can answer "when/why did this code
// change" from a single HEAD graph — no pre-fix re-index or git fallback.
func RegisterChangeHistory(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader) // enforce the §11.3 retrieval boundary regardless of caller
	tool := nsTool("change_history",
		mcp.WithDescription(
			"PR history for a symbol: the merged pull requests (number, title, summary, merged_at) that changed it, newest first. "+
				"qname may be a full qualified_name or a bare short name (suffix-resolved; an ambiguous bare name returns candidates). "+
				"Optional `cutoff` (RFC3339) returns only PRs merged strictly before it; `k` caps the count (default 20). "+
				"Use this to answer \"when/why did this code change\" without reading git — e.g. which PR introduced or fixed a behaviour. "+
				"When the graph was built from a source without git history the answer has history: \""+historyUnavailable+"\".",
		),
		mcp.WithString("qname", mcp.Required()),
		mcp.WithString("cutoff"),
		mcp.WithNumber("k", mcp.DefaultNumber(defaultChangeHistoryK)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := req.GetString("qname", "")
		k := int(req.GetFloat("k", defaultChangeHistoryK))
		var cutoff time.Time
		if cs := req.GetString("cutoff", ""); cs != "" {
			if t, err := time.Parse(time.RFC3339, cs); err == nil {
				cutoff = t
			}
		}
		res, err := changeHistory(reader, q, cutoff, k)
		if err != nil {
			return nil, err
		}
		if note := historyNote(reader); note != "" {
			res[historyKey] = note
		}
		return textResult(res), nil
	})
}

// changeHistory resolves qname and aggregates the PR breadcrumbs across the
// resolved node(s), deduped by PR number and sorted newest-first (capped at
// k). Seed resolution mirrors the traversal tools — bare names resolve by
// suffix, multi-match returns `ambiguous` + candidates, no match returns
// `not_found` — so callers handle resolution one way across the tool set.
func changeHistory(reader store.Reader, qname string, cutoff time.Time, k int) (map[string]any, error) {
	resolved, cands, ambiguous, ok := resolveSeed(reader, qname, "")
	if miss := seedMiss(qname, resolved, cands, ambiguous, ok); miss != nil {
		// A history answer has prs where a traversal has nodes and edges.
		delete(miss, "nodes")
		delete(miss, "edges")
		miss["prs"] = []map[string]any{}
		return miss, nil
	}

	nodes, err := resolved.nodes(reader)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	prs := []map[string]any{}
	for _, n := range nodes {
		refs, err := reader.GetNodePRs(n.ID, cutoff)
		if err != nil {
			return nil, err
		}
		for _, r := range refs {
			if seen[r.Number] {
				continue
			}
			seen[r.Number] = true
			prs = append(prs, map[string]any{
				"number":    r.Number,
				"title":     r.Title,
				"summary":   r.Summary,
				"merged_at": r.MergedAtUTC.Format(time.RFC3339),
				"repo":      r.Repo,
				"head_sha":  r.HeadSHA,
				"base_sha":  r.BaseSHA,
			})
		}
	}
	// Aggregating across nodes can interleave PRs; re-sort newest-first.
	// RFC3339-UTC strings sort lexicographically == chronologically.
	sort.SliceStable(prs, func(i, j int) bool {
		return prs[i]["merged_at"].(string) > prs[j]["merged_at"].(string)
	})
	if k > 0 && len(prs) > k {
		prs = prs[:k]
	}
	return resolved.result(map[string]any{"prs": prs}), nil
}
