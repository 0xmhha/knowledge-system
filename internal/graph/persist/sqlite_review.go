package persist

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// reviewCodeEdgeTypes are the code relations kept in code_edges. Structural
// (contains, defines from files), temporal (changed_in, blame, has_hunk,
// modifies) and cross-file plumbing edges are left out: a review walk over
// them reaches commits and, through them, most of the graph.
var reviewCodeEdgeTypes = []string{
	"calls", "invokes", "implements", "extends", "references",
	"uses_type", "instantiates", "writes_field",
	"spawns", "sends_to", "recvs_from", "acquires_lock", "releases_lock",
	"accessed_under_lock", "timeout_path", "cancellation_path",
}

// RebuildReviewIndex implements StoreWriter. It runs in one transaction so a
// reader never sees a partial index.
func (s *sqliteStore) RebuildReviewIndex(testFiles []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("review index: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE nodes SET is_test = 0 WHERE is_test <> 0`); err != nil {
		return fmt.Errorf("review index: reset is_test: %w", err)
	}
	for start := 0; start < len(testFiles); start += queryEdgesChunk {
		chunk := testFiles[start:min(start+queryEdgesChunk, len(testFiles))]
		if _, err := tx.Exec(`UPDATE nodes SET is_test = 1 WHERE file_path IN (`+placeholders(len(chunk))+`)`, anys(chunk)...); err != nil {
			return fmt.Errorf("review index: mark tests: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM code_edges`); err != nil {
		return fmt.Errorf("review index: clear code_edges: %w", err)
	}
	// defines is kept only for type membership (Struct/Interface -> Method,
	// Struct -> Field); File -> symbol defines are file plumbing.
	if _, err := tx.Exec(`INSERT INTO code_edges (src, dst, type, line)
		SELECT e.src, e.dst, e.type, COALESCE(e.line, 0)
		FROM edges e JOIN nodes s ON s.id = e.src JOIN nodes d ON d.id = e.dst
		WHERE s.is_test = 0 AND d.is_test = 0
		  AND (e.type IN (`+placeholders(len(reviewCodeEdgeTypes))+`)
		       OR (e.type = 'defines' AND s.type IN ('Struct', 'Interface')))`, anys(reviewCodeEdgeTypes)...); err != nil {
		return fmt.Errorf("review index: fill code_edges: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM node_commits`); err != nil {
		return fmt.Errorf("review index: clear node_commits: %w", err)
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO node_commits (node_id, commit_id)
		SELECT m.dst, h.src FROM edges m
		JOIN edges h ON h.dst = m.src AND h.type = 'has_hunk'
		WHERE m.type = 'modifies'`); err != nil {
		return fmt.Errorf("review index: fill node_commits: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("review index: commit: %w", err)
	}
	return nil
}

// Review implements StoreReader. It reads only the review index (code_edges,
// node_commits, nodes.is_test) plus the seed's own contains edges and blob,
// so its cost depends on the function's neighbourhood, not the graph size.
func (s *sqliteStore) Review(qname string, opts ReviewOptions) (Review, error) {
	opts = opts.withDefaults()
	if ok, err := s.hasReviewIndex(); err != nil {
		return Review{}, err
	} else if !ok {
		return Review{}, ErrNoReviewIndex
	}
	var r Review
	var seedStart int
	err := s.db.QueryRow(`SELECT id, qualified_name, type, file_path, start_line, end_line, start_byte, COALESCE(signature, '')
		FROM nodes WHERE qualified_name = ? AND type IN ('Function', 'Method') AND is_test = 0
		ORDER BY file_path, start_line LIMIT 1`, qname).Scan(
		&r.Seed.ID, &r.Seed.QualifiedName, &r.Seed.Type, &r.Seed.FilePath, &r.Seed.StartLine, &r.Seed.EndLine, &seedStart, &r.Signature)
	if errors.Is(err, sql.ErrNoRows) {
		return Review{}, ErrReviewSeedNotFound
	}
	if err != nil {
		return Review{}, fmt.Errorf("review seed: %w", err)
	}
	seed := r.Seed.ID
	out := func(typ string) ([]ReviewSymbol, error) {
		return s.reviewNeighbours(`c.src = ? AND c.type = ?`, `c.dst`, opts.MaxItems, seed, typ)
	}
	for _, f := range []struct {
		typ string
		dst *[]ReviewSymbol
	}{
		{"uses_type", &r.UsesTypes}, {"instantiates", &r.Instantiates}, {"writes_field", &r.WritesFields},
		{"spawns", &r.Goroutines}, {"sends_to", &r.SendsTo}, {"recvs_from", &r.RecvsFrom},
		{"acquires_lock", &r.AcquiresLocks}, {"releases_lock", &r.ReleasesLocks},
		{"timeout_path", &r.TimeoutPaths}, {"cancellation_path", &r.CancellationPaths},
	} {
		if *f.dst, err = out(f.typ); err != nil {
			return Review{}, err
		}
	}
	if err := s.reviewLogic(&r, seedStart, opts.MaxItems); err != nil {
		return Review{}, err
	}
	if err := s.reviewCallers(&r, opts); err != nil {
		return Review{}, err
	}
	if err := s.reviewHistory(&r, opts.MaxItems); err != nil {
		return Review{}, err
	}
	r.TestCallers, err = s.reviewSymbols(`SELECT DISTINCT n.id, n.qualified_name, n.type, n.file_path, n.start_line, n.end_line
		FROM edges e JOIN nodes n ON n.id = e.src
		WHERE e.dst = ? AND e.type IN ('calls', 'invokes') AND n.is_test = 1
		ORDER BY n.qualified_name`, seed)
	if err != nil {
		return Review{}, err
	}
	r.TestCallersTotal = len(r.TestCallers)
	r.TestCallers = r.TestCallers[:min(len(r.TestCallers), opts.MaxItems)]
	return r, nil
}

func (s *sqliteStore) hasReviewIndex() (bool, error) {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name IN ('code_edges', 'node_commits')`).Scan(&n); err != nil {
		return false, fmt.Errorf("review index check: %w", err)
	}
	if n != 2 {
		return false, nil
	}
	return columnExists(s.db, "nodes", "is_test")
}

func (s *sqliteStore) reviewSymbols(query string, args ...any) ([]ReviewSymbol, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("review query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ReviewSymbol
	for rows.Next() {
		var x ReviewSymbol
		if err := rows.Scan(&x.ID, &x.QualifiedName, &x.Type, &x.FilePath, &x.StartLine, &x.EndLine); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// reviewNeighbours returns the distinct nodes at column end of the code
// edges matching where, capped at limit.
func (s *sqliteStore) reviewNeighbours(where, end string, limit int, args ...any) ([]ReviewSymbol, error) {
	return s.reviewSymbols(`SELECT n.id, n.qualified_name, n.type, n.file_path, n.start_line, n.end_line
		FROM (SELECT DISTINCT `+end+` AS id FROM code_edges c WHERE `+where+`) x JOIN nodes n ON n.id = x.id
		ORDER BY n.qualified_name LIMIT `+fmt.Sprint(limit), args...)
}

// reviewContainsQuery lists the seed's statements. "+e.type" keeps SQLite
// from choosing idx_edges_type, which would scan every contains edge in the
// graph (67 ms on this repository's graph) instead of the seed's own.
const reviewContainsQuery = `SELECT n.type, n.start_line, n.start_byte, n.end_byte FROM edges e JOIN nodes n ON n.id = e.dst
		WHERE e.src = ? AND +e.type = 'contains' ORDER BY n.start_byte`

// reviewLogic fills control counts, return count, call-site text in source
// order (sliced from the seed's stored source) and project callees by line.
func (s *sqliteStore) reviewLogic(r *Review, seedStart, limit int) error {
	src, _ := s.GetBlob(r.Seed.ID)
	rows, err := s.db.Query(reviewContainsQuery, r.Seed.ID)
	if err != nil {
		return fmt.Errorf("review logic: %w", err)
	}
	defer func() { _ = rows.Close() }()
	r.Control = map[string]int{}
	for rows.Next() {
		var typ string
		var line, a, b int
		if err := rows.Scan(&typ, &line, &a, &b); err != nil {
			return err
		}
		switch typ {
		case "CallSite":
			r.CallSitesTotal++
			if len(r.CallSites) < limit {
				text := ""
				if lo, hi := a-seedStart, b-seedStart; lo >= 0 && hi <= len(src) && lo < hi {
					text, _, _ = strings.Cut(string(src[lo:hi]), "\n")
				}
				r.CallSites = append(r.CallSites, ReviewCallSite{Line: line, Text: text})
			}
		case "ReturnStmt":
			r.ReturnStatements++
		case "IfStmt", "LoopStmt", "SwitchStmt":
			r.Control[typ]++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	r.Callees, err = s.reviewSymbols(`SELECT n.id, n.qualified_name, n.type, n.file_path, n.start_line, n.end_line
		FROM code_edges c JOIN nodes n ON n.id = c.dst
		WHERE c.src = ? AND c.type IN ('calls', 'invokes') GROUP BY n.id ORDER BY min(c.line), n.qualified_name LIMIT `+fmt.Sprint(limit), r.Seed.ID)
	if err != nil {
		return err
	}
	// Implementations behind interface methods the function calls.
	r.InterfaceImplementers = map[string][]string{}
	impl, err := s.db.Query(`SELECT m.qualified_name, impl.qualified_name
		FROM code_edges c
		JOIN nodes m ON m.id = c.dst
		JOIN code_edges d ON d.dst = m.id AND d.type = 'defines'
		JOIN nodes i ON i.id = d.src AND +i.type = 'Interface'
		JOIN code_edges im ON im.dst = i.id AND im.type = 'implements'
		JOIN nodes impl ON impl.id = im.src
		WHERE c.src = ? AND c.type IN ('calls', 'invokes')
		GROUP BY m.qualified_name, impl.qualified_name ORDER BY 1, 2`, r.Seed.ID)
	if err != nil {
		return fmt.Errorf("review implementers: %w", err)
	}
	defer func() { _ = impl.Close() }()
	for impl.Next() {
		var m, x string
		if err := impl.Scan(&m, &x); err != nil {
			return err
		}
		if len(r.InterfaceImplementers[m]) < limit {
			r.InterfaceImplementers[m] = append(r.InterfaceImplementers[m], x)
		}
	}
	return impl.Err()
}

// reviewCallerBudget bounds the reverse walk: a widely used function has
// thousands of transitive callers, while a review shows 40 callers and 12
// paths (walking them all cost up to 250 ms on the go-stablenet graph).
const reviewCallerBudget = 2000

// reviewCallersQuery returns one walk level's callers for n frontier IDs.
// "+n.type" keeps SQLite from choosing idx_nodes_type for long IN lists,
// which read every function and method on each level.
func reviewCallersQuery(n int) string {
	return `SELECT c.dst, c.src FROM code_edges c JOIN nodes n ON n.id = c.src
		WHERE c.dst IN (` + placeholders(n) + `) AND c.type IN ('calls', 'invokes') AND +n.type IN ('Function', 'Method')
		ORDER BY c.dst, n.qualified_name`
}

// reviewCallers walks production callers back toward entry points. A
// concrete method also starts from the interface methods it implements,
// because interface-dispatched callers reach those, not the method.
func (s *sqliteStore) reviewCallers(r *Review, opts ReviewOptions) error {
	var err error
	if owner, method, ok := cutLast(r.Seed.QualifiedName); ok && r.Seed.Type == "Method" {
		r.ViaInterfaces, err = s.reviewSymbols(`SELECT DISTINCT im.id, im.qualified_name, im.type, im.file_path, im.start_line, im.end_line
			FROM nodes o
			JOIN code_edges c ON c.src = o.id AND c.type = 'implements'
			JOIN code_edges d ON d.src = c.dst AND d.type = 'defines'
			JOIN nodes im ON im.id = d.dst AND im.qualified_name = (SELECT qualified_name FROM nodes WHERE id = c.dst) || '.' || ?
			WHERE o.qualified_name = ? AND o.is_test = 0`, method, owner)
		if err != nil {
			return err
		}
	}
	starts := []string{r.Seed.ID}
	for _, v := range r.ViaInterfaces {
		starts = append(starts, v.ID)
	}
	prev := map[string]string{}
	for _, st := range starts {
		prev[st] = ""
	}
	isStart := func(id string) bool { return slices.Contains(starts, id) }
	frontier := starts
	var roots []string
	for depth := 0; depth < opts.CallerDepth && len(frontier) > 0; depth++ {
		callersOf := map[string][]string{}
		for start := 0; start < len(frontier); start += queryEdgesChunk {
			chunk := frontier[start:min(start+queryEdgesChunk, len(frontier))]
			rows, err := s.db.Query(reviewCallersQuery(len(chunk)), anys(chunk)...)
			if err != nil {
				return fmt.Errorf("review callers: %w", err)
			}
			for rows.Next() {
				var dst, src string
				if err := rows.Scan(&dst, &src); err != nil {
					_ = rows.Close()
					return err
				}
				callersOf[dst] = append(callersOf[dst], src)
			}
			_ = rows.Close()
		}
		var next []string
		for _, n := range frontier {
			if len(callersOf[n]) == 0 && !isStart(n) {
				roots = append(roots, n)
			}
			for _, c := range callersOf[n] {
				if _, seen := prev[c]; !seen {
					prev[c] = n
					next = append(next, c)
				}
			}
		}
		frontier = next
		if len(prev) > reviewCallerBudget {
			r.CallerWalkTruncated = len(frontier) > 0
			break
		}
	}
	roots = append(roots, frontier...) // the walk's depth limit is its edge
	r.EntryPoints = len(roots)
	direct := map[string]bool{}
	for id, via := range prev {
		if isStart(via) {
			direct[id] = true
		}
	}
	ids := make([]string, 0, len(direct))
	for id := range direct {
		ids = append(ids, id)
	}
	names, err := s.NodesByIDs(ids)
	if err != nil {
		return err
	}
	qn := map[string]string{}
	for _, n := range names {
		qn[n.ID] = n.QualifiedName
		r.Callers = append(r.Callers, ReviewSymbol{ID: n.ID, QualifiedName: n.QualifiedName, Type: string(n.Type), FilePath: n.FilePath, StartLine: n.StartLine, EndLine: n.EndLine})
	}
	sort.Slice(r.Callers, func(i, j int) bool { return r.Callers[i].QualifiedName < r.Callers[j].QualifiedName })
	r.Callers = r.Callers[:min(len(r.Callers), opts.MaxItems)]
	// Paths need the names of every node on them.
	var onPath []string
	for _, root := range roots[:min(len(roots), opts.MaxPaths)] {
		for x := root; x != ""; x = prev[x] {
			onPath = append(onPath, x)
		}
	}
	pathNodes, err := s.NodesByIDs(onPath)
	if err != nil {
		return err
	}
	for _, n := range pathNodes {
		qn[n.ID] = n.QualifiedName
	}
	for _, root := range roots[:min(len(roots), opts.MaxPaths)] {
		var p []string
		for x := root; x != ""; x = prev[x] {
			p = append(p, qn[x])
		}
		r.CallerPaths = append(r.CallerPaths, p)
	}
	return nil
}

// reviewHistory lists the commits that changed the function, newest first.
// A Commit node's signature is "<unix time>: <subject>".
func (s *sqliteStore) reviewHistory(r *Review, limit int) error {
	rows, err := s.db.Query(`SELECT c.qualified_name, COALESCE(c.signature, '') FROM node_commits x JOIN nodes c ON c.id = x.commit_id WHERE x.node_id = ?`, r.Seed.ID)
	if err != nil {
		return fmt.Errorf("review history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var q, sig string
		if err := rows.Scan(&q, &sig); err != nil {
			return err
		}
		c := ReviewCommit{SHA: strings.TrimPrefix(q, "commit:")}
		ts, subject, _ := strings.Cut(sig, ": ")
		c.Time, _ = strconv.ParseInt(ts, 10, 64)
		c.Subject = subject
		r.Commits = append(r.Commits, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	sort.Slice(r.Commits, func(i, j int) bool { return r.Commits[i].Time > r.Commits[j].Time })
	r.CommitsTotal = len(r.Commits)
	r.Commits = r.Commits[:min(len(r.Commits), limit)]
	return nil
}

// cutLast splits "a.b.c" into "a.b" and "c".
func cutLast(q string) (string, string, bool) {
	i := strings.LastIndex(q, ".")
	if i <= 0 {
		return "", "", false
	}
	return q[:i], q[i+1:], true
}
