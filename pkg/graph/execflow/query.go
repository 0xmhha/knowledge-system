package execflow

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Direction is which way the flow is read from the seed.
type Direction string

const (
	// Forward reads what happens after the seed starts, in execution order.
	Forward Direction = "forward"
	// Backward reads what leads to the seed: its callers, the goroutines
	// that run it and the messages whose arrival runs it.
	Backward Direction = "backward"
)

// Link is a kind of link the flow follows.
type Link string

const (
	// LinkCalls follows function calls, interface calls and calls through
	// function values.
	LinkCalls Link = "calls"
	// LinkMessages follows channel messages: a send to the receives it can
	// wake and the code each runs.
	LinkMessages Link = "messages"
	// LinkGoroutines follows goroutine starts.
	LinkGoroutines Link = "goroutines"
)

// AllLinks are the links followed by default, in their canonical order.
var AllLinks = []Link{LinkCalls, LinkMessages, LinkGoroutines}

// Tests says whether test code is part of the flow.
type Tests string

const (
	// TestsExclude leaves test code out (the default): it is not opened and
	// not listed among alternatives, and the summary counts it.
	TestsExclude Tests = "exclude"
	// TestsInclude keeps test code, after production code among
	// alternatives.
	TestsInclude Tests = "include"
)

// Defaults and bounds of a Query.
const (
	// DefaultDepth is how many hops (a call, a goroutine start, a message)
	// the flow opens beyond the seed's own steps.
	DefaultDepth = 3
	// MaxDepth bounds Depth.
	MaxDepth = 6
	// DefaultBudget is how many steps an answer shows.
	DefaultBudget = 25
	// MaxBudget bounds Budget.
	MaxBudget = 200
	// DefaultBranches is how many alternatives a branch point shows.
	DefaultBranches = 3
	// MaxBranches bounds Branches: as many as an answer can show, so a cut
	// branch point can be listed whole.
	MaxBranches = MaxBudget
	// DefaultToolName names the tool in the continuation calls an answer
	// prints.
	DefaultToolName = "execution_flow"
)

// ErrBadQuery is returned, wrapped, for a query the tool cannot run.
var ErrBadQuery = errors.New("bad execution flow query")

// Query is one execution flow question.
type Query struct {
	// Seed names the symbol to start from, in any name form the other
	// tools take.
	Seed string
	// Direction is Forward (default) or Backward.
	Direction Direction
	// Follow are the links to follow; empty follows AllLinks.
	Follow []Link
	// Focus, when set, names a field or variable: only the steps that read
	// it, write it or take a lock that guards it are kept, with the steps
	// that lead to them. Forward only.
	Focus string
	// Via, when set, names a node the answer keeps only the paths through;
	// From names a node backward answers show the paths from, to the seed,
	// shortest first. Both take any name form the other tools take.
	Via, From string
	// Depth, Budget and Branches bound the answer (0: the default; with
	// From the default depth is MaxDepth; Depth OwnStepsOnly shows the
	// seed's own steps and opens none). A value above its bound is capped,
	// and the answer says so.
	Depth, Budget, Branches int
	// Tests is TestsExclude (default) or TestsInclude.
	Tests Tests
	// ToolName names the tool in continuation calls (default
	// DefaultToolName); a server under a namespace passes its own.
	ToolName string
	// CallersTool, CalleesTool and ReferencesTool name the tools an answer
	// points to for what it leaves out (default find_callers, find_callees
	// and find_references).
	CallersTool, CalleesTool, ReferencesTool string
	// capped says which bounds were capped (set by normalized).
	capped []string
}

// OwnStepsOnly as Query.Depth opens nothing: only the seed's own steps.
const OwnStepsOnly = -1

// Default names of the tools continuation calls point to.
const (
	defaultCallersTool    = "find_callers"
	defaultCalleesTool    = "find_callees"
	defaultReferencesTool = "find_references"
)

// normalized returns q with defaults filled in and bounds applied, or an
// error wrapping ErrBadQuery.
func (q Query) normalized() (Query, error) {
	q.Seed = strings.TrimSpace(q.Seed)
	if q.Seed == "" {
		return q, fmt.Errorf("%w: no seed name", ErrBadQuery)
	}
	switch q.Direction {
	case "":
		q.Direction = Forward
	case Forward, Backward:
	default:
		return q, fmt.Errorf("%w: direction %q (want %s or %s)", ErrBadQuery, q.Direction, Forward, Backward)
	}
	if len(q.Follow) == 0 {
		q.Follow = AllLinks
	}
	for _, l := range q.Follow {
		if !slices.Contains(AllLinks, l) {
			return q, fmt.Errorf("%w: follow %q (want %s)", ErrBadQuery, l, joinLinks(AllLinks))
		}
	}
	switch q.Tests {
	case "":
		q.Tests = TestsExclude
	case TestsExclude, TestsInclude:
	default:
		return q, fmt.Errorf("%w: tests %q (want %s or %s)", ErrBadQuery, q.Tests, TestsExclude, TestsInclude)
	}
	q.Via, q.From = strings.TrimSpace(q.Via), strings.TrimSpace(q.From)
	if q.From != "" && q.Direction != Backward {
		return q, fmt.Errorf("%w: from is read backward (direction=%s); use via forward", ErrBadQuery, Backward)
	}
	depthDefault := DefaultDepth
	if q.From != "" {
		depthDefault = MaxDepth
	}
	if q.Depth > MaxDepth {
		q.capped = append(q.capped, fmt.Sprintf("depth %d capped at %d", q.Depth, MaxDepth))
	}
	if q.Budget > MaxBudget {
		q.capped = append(q.capped, fmt.Sprintf("budget %d capped at %d", q.Budget, MaxBudget))
	}
	if q.Branches > MaxBranches {
		q.capped = append(q.capped, fmt.Sprintf("branches %d capped at %d", q.Branches, MaxBranches))
	}
	ownOnly := q.Depth == OwnStepsOnly
	q.Depth = bounded(q.Depth, depthDefault, MaxDepth)
	if ownOnly {
		q.Depth = 0
	}
	q.Budget = bounded(q.Budget, DefaultBudget, MaxBudget)
	q.Branches = bounded(q.Branches, DefaultBranches, MaxBranches)
	if q.ToolName == "" {
		q.ToolName = DefaultToolName
	}
	if q.CallersTool == "" {
		q.CallersTool = defaultCallersTool
	}
	if q.CalleesTool == "" {
		q.CalleesTool = defaultCalleesTool
	}
	if q.ReferencesTool == "" {
		q.ReferencesTool = defaultReferencesTool
	}
	q.Focus = strings.TrimSpace(q.Focus)
	return q, nil
}

// follows reports whether the query follows link l.
func (q Query) follows(l Link) bool { return slices.Contains(q.Follow, l) }

// bounded is v, or def when v is not positive, at most limit.
func bounded(v, def, limit int) int {
	if v <= 0 {
		return def
	}
	return min(v, limit)
}

// ParseLinks splits a comma-separated list of links ("calls,messages");
// empty gives nil (all links).
func ParseLinks(s string) ([]Link, error) {
	var out []Link
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		l := Link(part)
		if !slices.Contains(AllLinks, l) {
			return nil, fmt.Errorf("%w: follow %q (want %s)", ErrBadQuery, part, joinLinks(AllLinks))
		}
		if !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out, nil
}

// joinLinks joins links with commas, in their order.
func joinLinks(ls []Link) string {
	parts := make([]string, len(ls))
	for i, l := range ls {
		parts[i] = string(l)
	}
	return strings.Join(parts, ",")
}
