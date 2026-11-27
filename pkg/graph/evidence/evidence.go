// Package evidence implements the EvidencePack assembler — given a
// free-form intent string and an optional seed qname, ranks past commits by
// BM25, decorates each with its hunks (or, outside the hunk window, the
// declarations it changed), and returns the Pack JSON the Coding Agent can
// fold into a few-shot prompt frame.
//
// Algorithm (after docs/graph/design/hunk-graph.md §5.2):
//
//  1. Documents: one per hunk = subject + decompressed patch +
//     modifies-qnames; two per recorded commit = its subject, and its body
//     + the qnames of the declarations its changed_in edges come from.
//  2. BM25-score against intent → top bm25TopN documents.
//  3. If seed_qname set: keep documents reaching the seed (or a 1-hop
//     call-graph neighbour) via modifies or changed_in edges.
//  4. Group by commit; a commit's score is its best document's.
//  5. Order commits by score DESC (then author time, then SHA); stop
//     emitting once cumulative patch text exceeds budget_tokens.
//
// §11.3 retrieval boundary: only EXTRACTED-confidence Hunks and
// Commits enter the corpus. AMBIGUOUS rows (the unreachable-history
// recovery track) are filtered out at scan time so the LLM never sees
// code paths that were rolled back.
package evidence

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/bm25"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Options controls one BuildPack invocation. The defaults below match
// the design §5.1 schema. Zero / negative values fall back to the
// defaults — callers can pass an empty struct for "default behaviour
// over the default intent" (rare but legal).
//
// IssueID, when set, restricts the candidate hunks to those whose
// parent commit's extracted issue set contains the requested ID.
// Combines additively with Intent / SeedQname:
//   - IssueID alone: returns the ticket's hunks ordered by commit recency.
//   - IssueID + Intent: BM25-rank inside the ticket subset.
//   - IssueID + SeedQname: filter by ticket AND by the seed neighbourhood.
type Options struct {
	Intent       string
	SeedQname    string
	IssueID      string
	K            int
	BudgetTokens int
	// Offset skips the first N commits in the recency-sorted result —
	// the "Load more" page boundary. Used by viewer/agent flows that
	// already consumed page 0 and want to keep walking back through a
	// large ticket without raising BudgetTokens (which would also
	// inflate the per-call payload). Stable across calls because
	// commit recency tie-breaks on SHA in groupByCommit's sort.
	Offset int
	// Mode picks the term-match strategy applied on top of the BM25
	// ranking:
	//   - "" or "or" (default): keep BM25's any-term-match behaviour.
	//     A high-scoring hit only needs to share one query token with
	//     the candidate doc; useful for fuzzy semantic search.
	//   - "and": after BM25 ranking, drop hits whose virtual document
	//     doesn't contain *every* query token. Useful for precise
	//     agent queries like "all hunks mentioning RetryPolicy AND
	//     Backoff" where OR's looser fuzzy match would surface noise.
	//
	// AND is purely a post-filter: the BM25 ranking still computes
	// across the full corpus, so the relative scoring among AND-mode
	// survivors matches what they would have been in OR mode.
	Mode string
}

// Pack is the EvidencePack JSON shape (§1.5). Stable field names so the
// Agent prompt format is portable across CKG versions.
type Pack struct {
	Intent string `json:"intent"`
	Hits   []Hit  `json:"hits"`
	// History, when set, says why the pack holds no history at all: the
	// graph was built from a source without git history. The tool layer
	// sets it; BuildPack leaves it empty.
	History string `json:"history,omitempty"`
	// Searched says what the ranking read, so an answer without the commit
	// sought can be told from one whose commit was never searchable.
	Searched Searched `json:"searched"`
}

// Searched describes the corpus a pack was ranked over: the patches of the
// HunkCommits commits whose hunks the graph holds (author times
// HunkOldestTime to HunkNewestTime, unix seconds), and the subject, body
// and changed symbol names of all Commits recorded commits.
type Searched struct {
	// Walked, Unrecorded and RecordedRule come from the build: of the
	// Walked commits only the recorded ones have documents.
	Walked         int    `json:"walked,omitempty"`
	Unrecorded     int    `json:"unrecorded,omitempty"`
	RecordedRule   string `json:"recorded_rule,omitempty"`
	HunkCommits    int    `json:"hunk_commits"`
	HunkOldest     string `json:"hunk_oldest,omitempty"`
	HunkOldestTime int64  `json:"hunk_oldest_time,omitempty"`
	HunkNewestTime int64  `json:"hunk_newest_time,omitempty"`
	Commits        int    `json:"commits"`
	Summary        string `json:"summary"`
}

// Hit is one commit of a pack: its hunks when the graph holds them
// (InHunkWindow), else the declarations it changed (Changed, at most
// changedListMax of ChangedTotal).
type Hit struct {
	Commit CommitInfo `json:"commit"`
	// Score is the commit's BM25 score, its best document's; Matched names
	// the kinds of its documents that matched (docSubject, docBody,
	// docHunk); SearchedBy says what of the commit could match.
	Score        float64        `json:"score"`
	Matched      []string       `json:"matched"`
	SearchedBy   string         `json:"searched_by"`
	InHunkWindow bool           `json:"in_hunk_window"`
	Hunks        []HunkRow      `json:"hunks"`
	Changed      []ModifiesInfo `json:"changed,omitempty"`
	ChangedTotal int            `json:"changed_total,omitempty"`
}

type CommitInfo struct {
	SHA        string `json:"sha"`
	Subject    string `json:"subject"`
	AuthorTime int64  `json:"author_time"`
	// IssueIDs lists the issue/PR IDs extracted from the commit
	// message; empty when none were recognised.
	IssueIDs []string `json:"issue_ids,omitempty"`
	// TopFiles is populated by TicketIndex's pickSampleCommits — the
	// top-3 most-frequently-touched directory paths across the
	// commit's hunks (e.g. ["crypto/secp256k1", "consensus", "core"]).
	// Lets the viewer's TicketIndex panel hint at a ticket's reach
	// before the user pays the cost of fetching the full EvidencePack.
	// Left empty (omitempty) by EvidencePack's BuildPack flow because
	// hunk file_path is already surfaced per-HunkRow there.
	TopFiles []string `json:"top_files,omitempty"`
}

type HunkRow struct {
	ID        string         `json:"id"`
	FilePath  string         `json:"file_path"`
	StartLine int            `json:"start_line"`
	EndLine   int            `json:"end_line"`
	PatchText string         `json:"patch_text"`
	Modifies  []ModifiesInfo `json:"modifies,omitempty"`
}

type ModifiesInfo struct {
	Qname     string `json:"qname"`
	Type      string `json:"type"`
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

const (
	defaultK            = 5
	defaultBudgetTokens = 6000
	bm25TopN            = 200 // first-stage shortlist size before seed/budget filtering
	// changedListMax bounds the declarations listed for a commit whose
	// hunks the graph does not hold.
	changedListMax = 20
	// subjectDocPrefix and commitDocPrefix start the BM25 document ids of
	// a commit's own documents: its subject alone, so that a subject given
	// as the intent finds its commit however long the commit's other text
	// is, and its body with the changed symbol names. Hunk documents use the
	// hunk node id.
	subjectDocPrefix = "subject:"
	commitDocPrefix  = "commit:"
	// Kinds of matched documents, and what a hit could be matched by.
	docSubject      = "subject"
	docBody         = "body_and_changed_names"
	docHunk         = "hunk"
	searchedMessage = "message only (subject, body, changed symbol names): the graph holds no hunks of this commit"
	searchedAll     = "message and patches"
)

// BuildPack runs the full EvidencePack assembly. Returns an empty Pack (with
// hits=[]) when no hunks match — never nil — so the Agent's JSON
// parsers don't have to guard against null. Errors propagate from the
// underlying store or from gzip decompression.
//
// This is the uncached entrypoint — every call rebuilds the BM25
// corpus from scratch. Long-lived processes (ckg serve, mcp Run)
// should hold a *Cache instance instead and call Cache.BuildPack so
// the indexing cost amortises across queries. See cache.go.
func BuildPack(store persist.StoreReader, opt Options) (*Pack, error) {
	// Single-shot path: a per-call Cache that gets discarded after
	// return. The Cache code path is the source of truth for the
	// algorithm; this wrapper exists for callers that don't want the
	// caching surface (one-off CLI tools, tests, eval harnesses).
	return NewCache().BuildPack(store, opt)
}

// hunkCorpus is the in-memory index BuildPack walks. Built once per
// invocation; for repeat queries the caller is responsible for caching.
type hunkCorpus struct {
	hunks         []types.Node          // EXTRACTED hunks only
	hunkByID      map[string]types.Node // for fast lookup
	commitBySHA   map[string]types.Node // EXTRACTED commits only
	modifiesOf    map[string][]types.Node
	calleesOf     map[string][]types.Node // qname → its calls/invokes targets (for seed expansion)
	callersOf     map[string][]types.Node // qname → things that call it
	nodeByID      map[string]types.Node
	hunkSHA       map[string]string // hunk ID → parent commit SHA
	hunkModifyIDs map[string]map[string]bool
	// changedBySHA lists the declarations whose changed_in edges reach a
	// commit: what it changed, also outside the hunk window.
	changedBySHA map[string][]types.Node
	// hunksBySHA lists each commit's hunks.
	hunksBySHA map[string][]string
	// issuesBySHA: issue-ID aggregation (hunk-graph.md §10.4). Hunks store their commit's
	// issue IDs in DocComment with the `issues:` prefix; we union
	// per-SHA so the EvidencePack's CommitInfo.IssueIDs reflects the
	// full set seen across that commit's hunks. (In practice all hunks
	// of one commit share the same set, but we union defensively in
	// case a future change varies it per-hunk.)
	issuesBySHA map[string]map[string]bool
}

// indexCorpus builds the hunkCorpus from the raw nodes + edges slices.
// Filters to confidence='EXTRACTED' on Hunk + Commit per §11.3 — other
// node types (Function/Method/etc) keep their own AMBIGUOUS rows so
// the modifies-decoration pass surfaces them as long as the source
// hunk is EXTRACTED.
func indexCorpus(nodes []types.Node, edges []types.Edge) *hunkCorpus {
	c := &hunkCorpus{
		hunkByID:      make(map[string]types.Node),
		commitBySHA:   make(map[string]types.Node),
		modifiesOf:    make(map[string][]types.Node),
		calleesOf:     make(map[string][]types.Node),
		callersOf:     make(map[string][]types.Node),
		nodeByID:      make(map[string]types.Node),
		hunkSHA:       make(map[string]string),
		hunkModifyIDs: make(map[string]map[string]bool),
		issuesBySHA:   make(map[string]map[string]bool),
		changedBySHA:  make(map[string][]types.Node),
		hunksBySHA:    make(map[string][]string),
	}
	for _, n := range nodes {
		c.nodeByID[n.ID] = n
		switch n.Type {
		case types.NodeHunk:
			if n.Confidence == types.ConfExtracted {
				c.hunks = append(c.hunks, n)
				c.hunkByID[n.ID] = n
				sha := parseHunkSHA(n.QualifiedName)
				c.hunkSHA[n.ID] = sha
				c.hunksBySHA[sha] = append(c.hunksBySHA[sha], n.ID)
				// Parse the `issues:` prefix from doc_comment and
				// union into the per-SHA aggregate.
				if ids := decodeIssuesFromDocComment(n.DocComment); len(ids) > 0 {
					set, hit := c.issuesBySHA[sha]
					if !hit {
						set = make(map[string]bool, len(ids))
						c.issuesBySHA[sha] = set
					}
					for _, id := range ids {
						set[id] = true
					}
				}
			}
		case types.NodeCommit:
			if n.Confidence == types.ConfExtracted {
				sha := strings.TrimPrefix(n.QualifiedName, "commit:")
				c.commitBySHA[sha] = n
			}
		}
	}
	// Index edges. Skip AMBIGUOUS rows even on non-meta types — for
	// the call-graph expansion the LLM-bound retrieval pass should not
	// dispatch through edges Resolve was unsure about.
	for _, e := range edges {
		if e.Confidence == types.ConfAmbiguous {
			continue
		}
		switch e.Type {
		case types.EdgeModifies:
			dst, ok := c.nodeByID[e.Dst]
			if !ok {
				continue
			}
			c.modifiesOf[e.Src] = append(c.modifiesOf[e.Src], dst)
			set, hit := c.hunkModifyIDs[e.Src]
			if !hit {
				set = make(map[string]bool)
				c.hunkModifyIDs[e.Src] = set
			}
			set[dst.ID] = true
		case types.EdgeChangedIn:
			src, ok1 := c.nodeByID[e.Src]
			dst, ok2 := c.nodeByID[e.Dst]
			if !ok1 || !ok2 || src.Type == types.NodeFile || dst.Type != types.NodeCommit {
				continue
			}
			sha := strings.TrimPrefix(dst.QualifiedName, "commit:")
			c.changedBySHA[sha] = append(c.changedBySHA[sha], src)
		case types.EdgeCalls, types.EdgeInvokes:
			srcN, ok1 := c.nodeByID[e.Src]
			dstN, ok2 := c.nodeByID[e.Dst]
			if !ok1 || !ok2 {
				continue
			}
			c.calleesOf[srcN.QualifiedName] = append(c.calleesOf[srcN.QualifiedName], dstN)
			c.callersOf[dstN.QualifiedName] = append(c.callersOf[dstN.QualifiedName], srcN)
		}
	}
	return c
}

// decodeIssuesFromDocComment extracts the §10.4 issue list from a
// Hunk's doc_comment column (formatted `issues:ID1;ID2`). Inlined
// here rather than imported from internal/temporal because pkg/
// evidence is a public package and shouldn't depend on internal/.
// Returns nil for any input lacking the prefix.
func decodeIssuesFromDocComment(docComment string) []string {
	const prefix = "issues:"
	if !strings.HasPrefix(docComment, prefix) {
		return nil
	}
	body := docComment[len(prefix):]
	if body == "" {
		return nil
	}
	parts := strings.Split(body, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseHunkSHA pulls the commit SHA out of a hunk's QualifiedName,
// formatted "hunk:<full-sha>:<file>:<idx>" by makeHunkNode. Returns
// "" on malformed names so the caller can skip the hunk gracefully.
func parseHunkSHA(qname string) string {
	if !strings.HasPrefix(qname, "hunk:") {
		return ""
	}
	rest := qname[len("hunk:"):]
	idx := strings.IndexByte(rest, ':')
	if idx <= 0 {
		return ""
	}
	return rest[:idx]
}

// hunkDocTokens builds the BM25 virtual document for one hunk:
// subject + (decompressed) patch text + modifies qnames. We tokenise
// each piece with the bm25.Tokenize splitter (camelCase + snake_case
// + qname-aware) so the resulting term frequency captures both
// natural-language tokens (commit subjects) and code identifiers
// (callee qnames / patch identifiers) in one pass.
//
// Errors fetching the blob (or decompressing) silently degrade to
// "no patch tokens" — better a less-rich document than none.
func hunkDocTokens(store persist.StoreReader, h types.Node, c *hunkCorpus) []string {
	var out []string
	if commit, ok := c.commitBySHA[c.hunkSHA[h.ID]]; ok {
		subject := stripCommitTimestamp(commit.Signature)
		out = append(out, bm25.Tokenize(subject)...)
	}
	if blob, err := store.GetBlob(h.ID); err == nil {
		if patch, gErr := gunzipIfNeeded(blob); gErr == nil {
			out = append(out, bm25.Tokenize(string(patch))...)
		}
	}
	for _, m := range c.modifiesOf[h.ID] {
		out = append(out, bm25.Tokenize(m.QualifiedName)...)
	}
	return out
}

// commitDocs builds the BM25 documents of one commit: its whole subject,
// and its body with the names of the declarations it changed. Every
// recorded commit has them, so a commit is found by its message even when
// the graph holds none of its hunks.
func commitDocs(commit types.Node, c *hunkCorpus, sha string) []bm25.Document {
	var rest []string
	if commit.Commit != nil {
		rest = append(rest, bm25.Tokenize(commit.Commit.Body)...)
	}
	for _, n := range c.changedBySHA[sha] {
		rest = append(rest, bm25.Tokenize(n.QualifiedName)...)
	}
	return []bm25.Document{
		{ID: subjectDocPrefix + sha, Tokens: bm25.Tokenize(stripCommitTimestamp(commit.Signature))},
		{ID: commitDocPrefix + sha, Tokens: rest},
	}
}

// docKind names the kind of a BM25 document.
func docKind(id string) string {
	switch {
	case strings.HasPrefix(id, subjectDocPrefix):
		return docSubject
	case strings.HasPrefix(id, commitDocPrefix):
		return docBody
	}
	return docHunk
}

// commitDocSHA returns the commit of one of a commit's own documents.
func commitDocSHA(id string) (string, bool) {
	if sha, ok := strings.CutPrefix(id, subjectDocPrefix); ok {
		return sha, true
	}
	return strings.CutPrefix(id, commitDocPrefix)
}

// docSHA returns the commit a BM25 document belongs to.
func docSHA(c *hunkCorpus, id string) string {
	if sha, ok := commitDocSHA(id); ok {
		return sha
	}
	return c.hunkSHA[id]
}

// stripCommitTimestamp converts the persisted Signature shape
// "<unix>: <subject>" back to the bare subject text. Mirrors the
// makeCommitNode formatting in internal/buildpipe/temporal.go.
func stripCommitTimestamp(sig string) string {
	idx := strings.IndexByte(sig, ':')
	if idx < 0 {
		return sig
	}
	return strings.TrimSpace(sig[idx+1:])
}

// commitTimestamp parses the unix-seconds prefix of Signature so we
// can sort hits by recency without hitting the Timestamp column on
// the persisted CommitInfo (the persisted node row only carries the
// Signature concatenation).
func commitTimestamp(sig string) int64 {
	idx := strings.IndexByte(sig, ':')
	if idx < 0 {
		return 0
	}
	t, err := strconv.ParseInt(strings.TrimSpace(sig[:idx]), 10, 64)
	if err != nil {
		return 0
	}
	return t
}

// gunzipIfNeeded mirrors handleBlob's egress logic: hunk blobs come
// gzipped from the hunk storage layer; pre-1.8 data may be raw. The
// magic-byte check covers both eras.
func gunzipIfNeeded(b []byte) ([]byte, error) {
	if len(b) >= 3 && b[0] == 0x1f && b[1] == 0x8b && b[2] == 0x08 {
		gr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer func() { _ = gr.Close() }()
		return io.ReadAll(gr)
	}
	return b, nil
}

// buildSeedAllowList returns the set of node IDs the seed-qname
// expansion permits as `modifies` targets: the seed itself + its 1-hop
// neighbours on the call graph (calls / invokes, both directions).
//
// Per design §5.4 the expansion is bounded — depth=1, max-degree-cap
// = 50 per node — so super-hub seeds (e.g. `Errorf`) don't pull every
// hunk in the corpus into the candidate set.
func buildSeedAllowList(c *hunkCorpus, seedQname string) map[string]bool {
	allowed := map[string]bool{}
	// Seed itself: every node whose qname matches.
	for _, n := range c.nodeByID {
		if n.QualifiedName == seedQname {
			allowed[n.ID] = true
		}
	}
	if len(allowed) == 0 {
		return allowed
	}
	const maxDegree = 50
	// One hop on the call graph (callers + callees). Cap per-node to bound noise.
	addCapped := func(targets []types.Node) {
		for i, t := range targets {
			if i >= maxDegree {
				break
			}
			allowed[t.ID] = true
		}
	}
	addCapped(c.calleesOf[seedQname])
	addCapped(c.callersOf[seedQname])
	return allowed
}

// filterByModifiesReach drops hunks whose `modifies` edges don't reach
// any allowed node. Hunks with no modifies edges are also dropped —
// without seed_qname we'd keep them; with seed_qname the user is
// asking specifically for hunks that touch the seed neighbourhood.
func filterByModifiesReach(scored []bm25.ScoredDoc, c *hunkCorpus, allowed map[string]bool) []bm25.ScoredDoc {
	if len(allowed) == 0 {
		// Seed not found — return empty rather than an unfiltered
		// result. Matches the §5.4 contract: a misspelled seed is a
		// signal the Agent should fix the prompt, not silently fall
		// back to all hunks.
		return nil
	}
	out := make([]bm25.ScoredDoc, 0, len(scored))
	for _, s := range scored {
		hit := false
		if sha, ok := commitDocSHA(s.ID); ok {
			for _, n := range c.changedBySHA[sha] {
				hit = hit || allowed[n.ID]
			}
		}
		for id := range c.hunkModifyIDs[s.ID] {
			if allowed[id] {
				hit = true
				break
			}
		}
		if hit {
			out = append(out, s)
		}
	}
	return out
}

// groupByCommit takes the BM25-ranked documents (hunks and commits),
// groups them under their commit, decorates each hunk with its `modifies`
// metadata, orders the commits by their best score (then recency, then SHA,
// so equal scores — an issue_id browse — read newest first and paging is
// stable), and stops emitting once the cumulative patch_text size exceeds
// budget_tokens. A commit found by its own document brings all its hunks;
// one outside the hunk window brings the declarations it changed instead.
//
// Token estimation uses the standard ~4-chars-per-token heuristic.
func groupByCommit(scored []bm25.ScoredDoc, c *hunkCorpus, k, budgetTokens, offset int, store persist.StoreReader) []Hit {
	const charsPerToken = 4
	if len(scored) == 0 {
		return nil
	}
	if offset < 0 {
		offset = 0
	}
	best := map[string]float64{}
	kinds := map[string]map[string]bool{}
	matchedHunks := map[string][]string{}
	order := []string{}
	for _, s := range scored {
		sha := docSHA(c, s.ID)
		if _, ok := c.commitBySHA[sha]; !ok || sha == "" {
			continue
		}
		if _, seen := best[sha]; !seen {
			order = append(order, sha)
			best[sha] = s.Score
		} else if s.Score > best[sha] {
			best[sha] = s.Score
		}
		if kinds[sha] == nil {
			kinds[sha] = map[string]bool{}
		}
		kinds[sha][docKind(s.ID)] = true
		if _, own := commitDocSHA(s.ID); !own {
			matchedHunks[sha] = append(matchedHunks[sha], s.ID)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if best[order[i]] != best[order[j]] {
			return best[order[i]] > best[order[j]]
		}
		ti := commitTimestamp(c.commitBySHA[order[i]].Signature)
		tj := commitTimestamp(c.commitBySHA[order[j]].Signature)
		if ti != tj {
			return ti > tj
		}
		return order[i] < order[j]
	})
	if offset >= len(order) {
		return nil
	}
	order = order[offset:]

	out := make([]Hit, 0, min(k, len(order)))
	usedTokens := 0
	for _, sha := range order {
		if len(out) >= k {
			break
		}
		hit := buildHit(sha, matchedHunks[sha], c, store)
		hit.Score = best[sha]
		hit.Matched = slices.Sorted(maps.Keys(kinds[sha]))
		commitTokens := 0
		for _, h := range hit.Hunks {
			commitTokens += len(h.PatchText) / charsPerToken
		}
		if usedTokens+commitTokens > budgetTokens && len(out) > 0 {
			// Keep the first commit even if it alone exceeds the budget,
			// so the Agent always gets some response on a successful query.
			break
		}
		out = append(out, hit)
		usedTokens += commitTokens
	}
	return out
}

// buildHit assembles one commit's hit: the hunks that matched, or all its
// hunks when only its own document matched, or the declarations it changed
// when the graph holds none of its hunks.
func buildHit(sha string, hunkIDs []string, c *hunkCorpus, store persist.StoreReader) Hit {
	commitNode := c.commitBySHA[sha]
	hit := Hit{
		Commit: CommitInfo{
			SHA:        sha,
			Subject:    stripCommitTimestamp(commitNode.Signature),
			AuthorTime: commitTimestamp(commitNode.Signature),
			IssueIDs:   sortedIssueIDs(c.issuesBySHA[sha]),
		},
		InHunkWindow: len(c.hunksBySHA[sha]) > 0,
		SearchedBy:   searchedMessage,
		Hunks:        []HunkRow{},
	}
	if hit.InHunkWindow {
		hit.SearchedBy = searchedAll
	}
	if len(hunkIDs) == 0 {
		hunkIDs = c.hunksBySHA[sha]
	}
	for _, id := range hunkIDs {
		h, ok := c.hunkByID[id]
		if !ok {
			continue
		}
		patch := ""
		if blob, err := store.GetBlob(h.ID); err == nil {
			if dec, gErr := gunzipIfNeeded(blob); gErr == nil {
				patch = string(dec)
			}
		}
		hit.Hunks = append(hit.Hunks, HunkRow{
			ID: h.ID, FilePath: h.FilePath, StartLine: h.StartLine, EndLine: h.EndLine,
			PatchText: patch, Modifies: decorateModifies(store, c.modifiesOf[h.ID]),
		})
	}
	if !hit.InHunkWindow {
		changed := c.changedBySHA[sha]
		hit.ChangedTotal = len(changed)
		hit.Changed = decorateModifies(store, changed[:min(len(changed), changedListMax)])
	}
	return hit
}

// searchedCorpus describes what the corpus holds.
func searchedCorpus(c *hunkCorpus, h *persist.HistoryRecord) Searched {
	s := Searched{Commits: len(c.commitBySHA)}
	for sha := range c.hunksBySHA {
		commit, ok := c.commitBySHA[sha]
		if !ok {
			continue
		}
		s.HunkCommits++
		t := commitTimestamp(commit.Signature)
		if s.HunkOldest == "" || t < s.HunkOldestTime {
			s.HunkOldest, s.HunkOldestTime = sha, t
		}
		s.HunkNewestTime = max(s.HunkNewestTime, t)
	}
	s.Summary = fmt.Sprintf("ranked the patches of the %d newest commits whose hunks the graph holds (from %s) and, for every one of the %d recorded commits, its subject, body and changed symbol names; older commits match by their message only",
		s.HunkCommits, time.Unix(s.HunkOldestTime, 0).UTC().Format(time.RFC3339), s.Commits)
	if h != nil {
		s.Walked, s.Unrecorded, s.RecordedRule = h.Commits, max(0, h.Commits-h.Recorded), h.RecordedRule
		s.Summary += fmt.Sprintf("; %d of the %d commits the build walked are not recorded and cannot be found (%s)", s.Unrecorded, s.Walked, h.RecordedRule)
	}
	return s
}

// sortedIssueIDs flattens the per-SHA issue set into a deterministic
// slice. Returns nil for empty/missing sets so JSON serialisation
// elides the IssueIDs field via its `omitempty` tag.
func sortedIssueIDs(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// decorateModifies converts the modifies-target Node slice into the
// JSON-friendly ModifiesInfo shape. Returns nil rather than [] for
// hunks with no modifies edges so JSON serialization elides the field
// entirely (matches the `omitempty` tag on the struct).
func decorateModifies(store persist.StoreReader, targets []types.Node) []ModifiesInfo {
	if len(targets) == 0 {
		return nil
	}
	out := make([]ModifiesInfo, 0, len(targets))
	for _, t := range targets {
		out = append(out, ModifiesInfo{
			Qname:     persist.PrintedName(store, t),
			Type:      string(t.Type),
			FilePath:  t.FilePath,
			StartLine: t.StartLine,
			EndLine:   t.EndLine,
		})
	}
	return out
}
