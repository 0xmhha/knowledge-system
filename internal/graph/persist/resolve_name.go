package persist

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// NameClass says what a node matched by name is. The classes are ordered: a
// bare name resolves within the first class that holds a match, so a
// function-local "tx" does not make the package-level "tx" ambiguous.
type NameClass int

// The classes of a node matched by name, in the order a bare name prefers.
const (
	// NameDeclared is a declaration in a file of the build.
	NameDeclared NameClass = iota
	// NamePromoted is the copy of a method under a type that embeds the
	// method's receiver: no declaration of its own, the declared method's
	// file and lines.
	NamePromoted
	// NameLocal is a name known only inside one function.
	NameLocal
	// NameExternal is a stub for code outside the build.
	NameExternal
	// NameOther is a node no symbol name refers to (statement, import,
	// history rows, a blank declaration).
	NameOther
)

// nameClasses lists the classes in preference order.
var nameClasses = []NameClass{NameDeclared, NamePromoted, NameLocal, NameExternal, NameOther}

// promotedDocPrefix starts the doc comment the Go extractor gives the copy of
// a promoted method; the declared method's qualified name follows it.
const promotedDocPrefix = "promoted from "

// otherCaseLimit bounds the names of another case listed for a name that
// matched nothing.
const otherCaseLimit = 20

// BlankName is the name of a declaration that binds nothing (`var _ = x`).
// No symbol name refers to it, so it is no answer to a name query.
const BlankName = "_"

// The keys a tool adds to its answer when the caller named the copy of a
// promoted method and was answered for the method it copies.
const (
	SeedKeyPromotedFrom = "promoted_from"
	SeedKeyNote         = "seed_note"
)

// PromotedFrom reports whether n is the copy of a promoted method and, if so,
// the qualified name of the method it copies.
func PromotedFrom(n types.Node) (string, bool) {
	if n.Type != types.NodeMethod || n.CanonicalID != "" || !strings.HasPrefix(n.DocComment, promotedDocPrefix) {
		return "", false
	}
	return strings.TrimPrefix(n.DocComment, promotedDocPrefix), true
}

// DeclSite is where a method is declared. The copy of a promoted method has
// the site of the method it copies, which is how the two are matched.
type DeclSite struct {
	File       string
	Start, End int
	Name       string
}

// SiteOf returns the declaration site of n.
func SiteOf(n types.Node) DeclSite {
	return DeclSite{File: n.FilePath, Start: n.StartLine, End: n.EndLine, Name: n.Name}
}

// EmbeddingType returns the qualified name of the type a promoted copy is
// listed under: its qualified name without the method name.
func EmbeddingType(promotedCopy types.Node) string {
	return strings.TrimSuffix(promotedCopy.QualifiedName, "."+promotedCopy.Name)
}

// PromotedTo returns, by the id of each method among methods, the types that
// get that method through an embedded field (qualified names, sorted). A
// promoted method is one declaration; the types are an attribute of it.
func PromotedTo(r StoreReader, methods []types.Node) (map[string][]string, error) {
	bySite := map[DeclSite][]string{} // ids of the methods declared at a site
	names := map[string]bool{}
	for _, n := range methods {
		if _, isCopy := PromotedFrom(n); n.Type == types.NodeMethod && !isCopy {
			bySite[SiteOf(n)] = append(bySite[SiteOf(n)], n.ID)
			names[n.Name] = true
		}
	}
	out := map[string][]string{}
	for name := range names {
		same, err := r.FindSymbol(name, false, FindSymbolOptions{Kinds: []types.NodeType{types.NodeMethod}, Limit: FindSymbolNoLimit})
		if err != nil {
			return nil, fmt.Errorf("promoted copies of %q: %w", name, err)
		}
		for _, n := range same {
			if _, isCopy := PromotedFrom(n); !isCopy {
				continue
			}
			for _, id := range bySite[SiteOf(n)] {
				out[id] = append(out[id], EmbeddingType(n))
			}
		}
	}
	for id := range out {
		sort.Strings(out[id])
	}
	return out, nil
}

// callableKinds are the kinds whose body other declarations can be inside.
var callableKinds = map[types.NodeType]bool{
	types.NodeFunction: true, types.NodeMethod: true, types.NodeModifier: true, types.NodeConstructor: true,
}

// IsCallable reports whether n is a function, method, modifier or constructor
// declaration (not the copy of a promoted method): a node whose body other
// declarations can be inside.
func IsCallable(n types.Node) bool {
	_, isCopy := PromotedFrom(n)
	return callableKinds[n.Type] && !isCopy
}

// IsEncloser reports whether other declarations can be declared inside n: a
// function, method, modifier or constructor (IsCallable), whose body holds
// them, or a variable, whose initialiser can hold a function literal with
// declarations of its own (`var tests = func() []T { var r T; ... }()`).
// InFunctionBody takes the enclosers of a file.
func IsEncloser(n types.Node) bool {
	return IsCallable(n) || n.Type == types.NodeVariable
}

// InFunctionBody reports whether n is declared inside a function: a name made
// by := or a parameter, or a declaration (var, const, type, a member of such
// a type, a goroutine, a channel, a mutex) that lies within the span of one of
// enclosers, the functions, methods and variables of n's file (IsEncloser). It
// is the one rule the name resolver, find_symbol, search_text and the code
// search share for "known only inside one function".
//
// A declaration inside a package-level variable's initialiser is inside the
// function literal that declares it, a function without a node of its own:
// it is part of the variable that owns the initialiser, as a local of a
// function is part of the function. enode.parseNodeTests.r, a `var r` in a
// function literal of the initialiser of enode.parseNodeTests, was a search
// hit and a find_symbol declaration of its own. A field or method within a
// variable's span is left out of this: it is a member of a type written in
// the variable's declaration (`var Config struct{ Port int }`), which code
// outside reaches as Config.Port.
//
// Within is by lines and, when both nodes carry them, by byte offsets too, so
// that a declaration on the line of a one-line function is not taken for part
// of it unless it is. A node with the span of an encloser is not inside
// it: it is the same declaration under another node (a promoted copy of a
// method, another name of `var a, b = f()`, the Mutex node of a variable).
//
// A node with a canonical id is never inside a function: the type checker
// names package-level objects and their members only.
func InFunctionBody(n types.Node, enclosers []types.Node) bool {
	switch n.Type {
	case types.NodeLocalVariable, types.NodeParameter:
		return true
	case types.NodeFile, types.NodePackage, types.NodeExternal:
		return false
	}
	if n.CanonicalID != "" {
		return false
	}
	for _, f := range enclosers {
		if f.ID == n.ID || f.FilePath != n.FilePath {
			continue
		}
		if f.Type == types.NodeVariable && (n.Type == types.NodeField || n.Type == types.NodeMethod) {
			continue
		}
		if within(n, f) && !sameSpan(n, f) {
			return true
		}
	}
	return false
}

// hasBytes reports whether n carries the byte offsets of its span.
func hasBytes(n types.Node) bool { return n.EndByte > 0 }

// within reports whether n lies within the span of f (see InFunctionBody):
// within its lines and, when both carry them, its bytes.
func within(n, f types.Node) bool {
	lines := n.StartLine >= f.StartLine && n.EndLine <= f.EndLine
	if !lines || !hasBytes(n) || !hasBytes(f) {
		return lines
	}
	return n.StartByte >= f.StartByte && n.EndByte <= f.EndByte
}

// sameSpan reports whether n and f span the same text (see InFunctionBody):
// the same lines and, when both carry them, the same bytes.
func sameSpan(n, f types.Node) bool {
	lines := n.StartLine == f.StartLine && n.EndLine == f.EndLine
	if !lines || !hasBytes(n) || !hasBytes(f) {
		return lines
	}
	return n.StartByte == f.StartByte && n.EndByte == f.EndByte
}

// NodesInFunctions returns, by node id, which of nodes are declared inside a
// function body (InFunctionBody). It reads the enclosers of a file only for
// nodes the kind and canonical id do not decide.
func NodesInFunctions(r StoreReader, nodes []types.Node) (map[string]bool, error) {
	out := map[string]bool{}
	enclosers := map[string][]types.Node{}
	for _, n := range nodes {
		if n.CanonicalID == "" && n.FilePath != "" && n.Type.IsSymbol() {
			if _, loaded := enclosers[n.FilePath]; !loaded {
				all, err := r.NodesByFilePath(n.FilePath)
				if err != nil {
					return nil, fmt.Errorf("functions of %s: %w", n.FilePath, err)
				}
				fs := []types.Node{}
				for _, f := range all {
					if IsEncloser(f) {
						fs = append(fs, f)
					}
				}
				enclosers[n.FilePath] = fs
			}
		}
		if InFunctionBody(n, enclosers[n.FilePath]) {
			out[n.ID] = true
		}
	}
	return out, nil
}

// ClassifyName returns the class of a node matched by name. inFunction is
// InFunctionBody for the node. It is the one rule the name resolver,
// find_symbol and search_text share for "is this a declaration of the build".
func ClassifyName(n types.Node, inFunction bool) NameClass {
	switch {
	case n.Type == types.NodeExternal:
		return NameExternal
	case inFunction:
		return NameLocal
	case !n.Type.IsSymbol(), n.Name == BlankName:
		return NameOther
	}
	if _, ok := PromotedFrom(n); ok {
		return NamePromoted
	}
	return NameDeclared
}

// ClassifyNames returns the class of each of nodes by id.
func ClassifyNames(r StoreReader, nodes []types.Node) (map[string]NameClass, error) {
	inFunc, err := NodesInFunctions(r, nodes)
	if err != nil {
		return nil, err
	}
	out := make(map[string]NameClass, len(nodes))
	for _, n := range nodes {
		out[n.ID] = ClassifyName(n, inFunc[n.ID])
	}
	return out, nil
}

// Declarations returns the nodes of nodes that are declarations of the
// build an agent asks about by name (ClassifyName: NameDeclared or
// NamePromoted), in order: not statement nodes, goroutines or names local to
// a function, imports or External stubs.
func Declarations(r StoreReader, nodes []types.Node) ([]types.Node, error) {
	classes, err := ClassifyNames(r, nodes)
	if err != nil {
		return nil, err
	}
	out := nodes[:0:0]
	for _, n := range nodes {
		if c := classes[n.ID]; c == NameDeclared || c == NamePromoted {
			out = append(out, n)
		}
	}
	return out, nil
}

// NameResolution is what a caller-supplied symbol name resolved to: one
// qualified name and its nodes, or the candidates of an ambiguous bare name,
// or nothing.
type NameResolution struct {
	// QName is the qualified name the input resolved to; empty when it did
	// not resolve.
	QName string
	// Nodes are the nodes resolved to: one for a canonical id, every node
	// carrying QName otherwise.
	Nodes []types.Node
	// ByID is true when Nodes are exact nodes (a canonical id's node, the
	// declaration of a promoted method), so a traversal starts from their
	// ids and not from every node that shares QName.
	ByID bool
	// PromotedAs is the qualified name the input resolved to when that was
	// the copy of a promoted method; QName and Nodes are then the method it
	// copies.
	PromotedAs string
	// Candidates are the distinct qualified names a bare name matched when
	// it matched more than one, sorted.
	Candidates []string
	// OtherCase lists qualified names that differ from the input only in
	// case, when the input matched nothing.
	OtherCase []string
	// AlsoNamed lists, by printed name, the other declarations whose
	// qualified name is the input when the input resolved as the canonical
	// id of one node (the standard library's errors.New has the canonical
	// id "errors.New", which External stubs of other packages carry as
	// their qualified name). The canonical id wins; these are named so the
	// caller can choose another.
	AlsoNamed []string
}

// Found reports whether the input resolved to one qualified name.
func (r NameResolution) Found() bool { return r.QName != "" }

// SeedNote is the sentence a tool puts under SeedKeyNote, next to QName under
// SeedKeyPromotedFrom, when the caller named the copy of a promoted method;
// it is empty otherwise. Every tool that takes a symbol name reports the step
// from the copy to the method with these two keys and this text.
func (r NameResolution) SeedNote() string {
	if r.PromotedAs == "" {
		return ""
	}
	return r.PromotedAs + " is " + r.QName + ", which its type gets through an embedded field; this answer is for " + r.QName
}

// Ambiguous reports whether a bare name matched several qualified names.
func (r NameResolution) Ambiguous() bool { return len(r.Candidates) > 0 }

// ResolveName maps a caller-supplied name to nodes. It is the one resolver
// behind every tool that takes a symbol name, so the same input means the
// same symbol everywhere.
//
// Order:
//  1. a canonical id: globally unique, one node.
//  2. a qualified name: every node carrying it (the same symbol in several
//     languages is one name, not an ambiguity).
//  3. a bare or partly qualified name ("Pack", "ABI.Pack"), in the case the
//     caller wrote: the declarations of the build with that name. One distinct
//     qualified name resolves; several are returned as Candidates. Promoted
//     copies, function-local names and External stubs are considered only
//     when no declaration has the name, class by class (see NameClass).
//
// One qualified name can name several declarations: a function that declares
// a local name twice, in two blocks (`errMu` at lines 978 and 1016 of one
// test), or two init functions that declare the same local. Such a name is
// not resolved by merging them: like a bare name that matches several
// qualified names, it is ambiguous, and its Candidates name each declaration
// as "<qname>@<file>:<line>" (siteName), a form ResolveName accepts and
// resolves to the nodes of that declaration alone. Nodes at one place (the
// LocalVariable and the Mutex node of one `var mu sync.Mutex`) are one
// declaration, and so are nodes with a canonical id or of different
// languages, which name one symbol.
//
// A name that resolves to the copy of a promoted method ("logger.dummy.Prepare"
// for state.StateDB.Prepare) is answered with the method it copies, and
// PromotedAs keeps the name asked for: the copy has no edge but the one from
// its type, so every question about it is a question about that method.
//
// language, when not empty, restricts steps 2 and 3 to that language.
func ResolveName(r StoreReader, name, language string) (NameResolution, error) {
	res, err := resolveName(r, name, language)
	if err != nil || !res.Found() {
		return res, err
	}
	declared, err := declaredMethods(r, res.Nodes)
	if err != nil || len(declared) == 0 {
		return res, err
	}
	return NameResolution{QName: declared[0].QualifiedName, Nodes: declared, ByID: true, PromotedAs: res.QName}, nil
}

// declaredMethods returns the declaration that copies, all promoted copies
// of one method, were copied from: the declared nodes carrying the copied
// qualified name at the copies' file and line. It returns nil when copies
// holds anything else or the declaration is not in the graph.
func declaredMethods(r StoreReader, copies []types.Node) ([]types.Node, error) {
	from := ""
	for _, n := range copies {
		q, promoted := PromotedFrom(n)
		if !promoted || from != "" && q != from {
			return nil, nil
		}
		from = q
	}
	if from == "" {
		return nil, nil
	}
	nodes, err := r.FindSymbol(from, true, FindSymbolOptions{Limit: FindSymbolNoLimit})
	if err != nil {
		return nil, fmt.Errorf("resolve promoted method %q: %w", from, err)
	}
	var declared []types.Node
	for _, n := range nodes {
		if _, isCopy := PromotedFrom(n); !isCopy && SiteOf(n) == SiteOf(copies[0]) {
			declared = append(declared, n)
		}
	}
	return declared, nil
}

// resolveName is ResolveName without the step from a promoted copy to the
// method it copies.
func resolveName(r StoreReader, name, language string) (NameResolution, error) {
	if name == "" {
		return NameResolution{}, nil
	}
	if n, found, err := r.FindByCanonicalID(name); err != nil {
		return NameResolution{}, fmt.Errorf("resolve %q by canonical id: %w", name, err)
	} else if found {
		return byCanonicalID(r, name, n)
	}
	opts := FindSymbolOptions{Language: language, Limit: FindSymbolNoLimit}
	exact, err := r.FindSymbol(name, true, opts)
	if err != nil {
		return NameResolution{}, fmt.Errorf("resolve %q by qualified name: %w", name, err)
	}
	if len(exact) > 0 {
		return oneDeclaration(name, exact), nil
	}
	if declaring, ok := strings.CutSuffix(name, MutexSuffix); ok && declaring != "" {
		if res, err := resolveMutex(r, declaring, language); err != nil || res.Found() || res.Ambiguous() {
			return res, err
		}
	}
	if qname, file, line, ordinal, ok := parseSite(name); ok {
		return resolveSite(r, qname, file, line, ordinal, opts)
	}
	matches, err := r.FindSymbol(name, false, opts)
	if err != nil {
		return NameResolution{}, fmt.Errorf("resolve %q by suffix: %w", name, err)
	}
	if len(matches) == 0 {
		return otherCaseNames(r, name, opts)
	}
	classes, err := ClassifyNames(r, matches)
	if err != nil {
		return NameResolution{}, err
	}
	byClass := map[NameClass][]types.Node{}
	for _, n := range matches {
		byClass[classes[n.ID]] = append(byClass[classes[n.ID]], n)
	}
	for _, c := range nameClasses {
		nodes := byClass[c]
		if len(nodes) == 0 {
			continue
		}
		// Copies of one method under several types are one symbol: the
		// method ResolveName answers them with.
		oneMethod := c == NamePromoted && oneSite(nodes)
		if names := distinctQNames(nodes); len(names) > 1 && !oneMethod {
			return NameResolution{Candidates: candidatesOf(r, nodes)}, nil
		}
		// Whether the one qualified name names one declaration is decided
		// over every node that carries it, not over the matches of a class.
		q := nodes[0].QualifiedName
		all, err := r.FindSymbol(q, true, FindSymbolOptions{Limit: FindSymbolNoLimit})
		if err != nil {
			return NameResolution{}, fmt.Errorf("resolve %q: %w", q, err)
		}
		if oneMethod || len(all) == 0 {
			return oneDeclaration(q, nodes), nil
		}
		return oneDeclaration(q, all), nil
	}
	return NameResolution{}, nil
}

// siteMark separates a qualified name from the place of one of its
// declarations in a site name ("<qname>@<file>:<line>"); lineMark separates
// the file from the line.
const (
	siteMark = "@"
	lineMark = ":"
)

// siteName names the declaration of qname that n stands at.
func siteName(qname string, n types.Node) string {
	return qname + siteMark + n.FilePath + lineMark + strconv.Itoa(n.StartLine)
}

// ParseSiteName splits a site name ("<qname>@<file>:<line>", the form
// ResolveName gives the candidates of a qualified name that several
// declarations carry) into its qualified name, file and line.
// A qualified name may hold "@" itself (pkg.T.struct@120.f), so the place is
// what follows the last one and must end in ":<line>".
func ParseSiteName(name string) (qname, file string, line int, ok bool) {
	qname, file, line, _, ok = parseSite(name)
	return qname, file, line, ok
}

// parseSite is ParseSiteName with the place among the names of one
// declaration ("#<n>" at the end, 0 when absent).
func parseSite(name string) (qname, file string, line, ordinal int, ok bool) {
	at := strings.LastIndex(name, siteMark)
	if at <= 0 {
		return "", "", 0, 0, false
	}
	place, ordinal := splitOrdinal(name[at+len(siteMark):])
	colon := strings.LastIndex(place, lineMark)
	if colon <= 0 {
		return "", "", 0, 0, false
	}
	line, err := strconv.Atoi(place[colon+len(lineMark):])
	if err != nil || line <= 0 {
		return "", "", 0, 0, false
	}
	return name[:at], place[:colon], line, ordinal, true
}

// resolveSite resolves a site name: the nodes of qname that stand at line of
// file.
func resolveSite(r StoreReader, qname, file string, line, ordinal int, opts FindSymbolOptions, only ...types.NodeType) (NameResolution, error) {
	nodes, err := r.FindSymbol(qname, true, opts)
	if err != nil {
		return NameResolution{}, fmt.Errorf("resolve %q at %s:%d: %w", qname, file, line, err)
	}
	var at []types.Node
	for _, n := range nodes {
		if n.FilePath == file && n.StartLine == line && (ordinal == 0 || ordinalOf(n) == ordinal) && (len(only) == 0 || n.Type == only[0]) {
			at = append(at, n)
		}
	}
	if len(only) == 0 {
		at = withoutMutexOfValue(at)
	}
	if declaring, ok := strings.CutSuffix(qname, MutexSuffix); ok && len(at) == 0 && len(only) == 0 && declaring != "" {
		// "<variable>#mutex@<file>:<line>": the Mutex node of the
		// variable at that place, the one name of the mutex of a local
		// that its function declares twice.
		return resolveSite(r, declaring, file, line, ordinal, opts, types.NodeMutex)
	}
	if len(at) == 0 {
		return NameResolution{}, nil
	}
	// Several names of one declaration share the place: each is named
	// with its place among them.
	if res := oneDeclaration(qname, at); res.Ambiguous() {
		return res, nil
	}
	return NameResolution{QName: qname, Nodes: at, ByID: true}, nil
}

// byCanonicalID answers a canonical id that names n: n alone, unless other
// declarations carry the same id (the init functions of one package share
// "<path>.init"), when it is ambiguous with their printed names.
func byCanonicalID(r StoreReader, cid string, n types.Node) (NameResolution, error) {
	nodes, err := r.FindSymbol(n.QualifiedName, true, FindSymbolOptions{Limit: FindSymbolNoLimit})
	if err != nil {
		return NameResolution{}, fmt.Errorf("resolve %q: %w", cid, err)
	}
	var same []types.Node
	for _, m := range nodes {
		if m.CanonicalID == cid {
			same = append(same, m)
		}
	}
	if res := oneDeclaration(n.QualifiedName, same); res.Ambiguous() {
		return res, nil
	}
	res := NameResolution{QName: n.QualifiedName, Nodes: []types.Node{n}, ByID: true}
	named, err := r.FindSymbol(cid, true, FindSymbolOptions{Limit: FindSymbolNoLimit})
	if err != nil {
		return NameResolution{}, fmt.Errorf("resolve %q: %w", cid, err)
	}
	for _, m := range named {
		if m.ID != n.ID && declKey(m) != declKey(n) {
			res.AlsoNamed = append(res.AlsoNamed, PrintedName(r, m))
		}
	}
	sort.Strings(res.AlsoNamed)
	return res, nil
}

// MutexSuffix ends the name of a mutex as a seed: the qualified name of the
// Mutex node of a struct field ("pkg.T.mu#mutex"), and the name by which the
// Mutex node of a variable is asked for ("pkg.mu#mutex"; its node carries
// the variable's own name).
const MutexSuffix = "#mutex"

// NamedNodes returns the nodes that the qualified name qname names among
// nodes, all carrying it: without the Mutex node of a variable (see
// withoutMutexOfValue). find_symbol lists these, as the seed tools resolve
// to them.
func NamedNodes(nodes []types.Node) []types.Node { return withoutMutexOfValue(nodes) }

// withoutMutexOfValue leaves out of nodes, all carrying one qualified name,
// the Mutex nodes that stand at the place of a Variable or LocalVariable
// among them: a `var mu sync.Mutex` is a Variable and a Mutex node of one
// name and place, and its name names the variable, a value whose readers and
// writers every tool answers with, as it names a field that is a mutex (the
// field's Mutex node is "<field>#mutex"). With both nodes, find_callers took
// the name for no value and answered with the two nodes alone (9 of 13 mutex
// variables of go-stablenet). The Mutex node is asked for with MutexSuffix.
func withoutMutexOfValue(nodes []types.Node) []types.Node {
	values := map[declPlace]bool{}
	for _, n := range nodes {
		if n.Type == types.NodeVariable || n.Type == types.NodeLocalVariable {
			values[declPlace{language: n.Language, file: n.FilePath, line: n.StartLine}] = true
		}
	}
	if len(values) == 0 {
		return nodes
	}
	out := make([]types.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.Type != types.NodeMutex || !values[declPlace{language: n.Language, file: n.FilePath, line: n.StartLine}] {
			out = append(out, n)
		}
	}
	return out
}

// resolveMutex resolves "<declaring>#mutex" when no node carries that name:
// the Mutex nodes of the variable declaring names, alone. A declaring name
// that names several declarations is answered with their candidates.
func resolveMutex(r StoreReader, declaring, language string) (NameResolution, error) {
	nodes, err := r.FindSymbol(declaring, true, FindSymbolOptions{Language: language, Limit: FindSymbolNoLimit})
	if err != nil {
		return NameResolution{}, fmt.Errorf("resolve %q%s: %w", declaring, MutexSuffix, err)
	}
	if res := oneDeclaration(declaring, nodes); res.Ambiguous() {
		// Candidates that name the mutexes, not the variables.
		for i, c := range res.Candidates {
			res.Candidates[i] = declaring + MutexSuffix + strings.TrimPrefix(c, declaring)
		}
		return res, nil
	}
	var mutexes []types.Node
	for _, n := range nodes {
		if n.Type == types.NodeMutex {
			mutexes = append(mutexes, n)
		}
	}
	if len(mutexes) == 0 {
		// A field or variable that holds a channel used as a lock names
		// that lock: the Channel node (sub kind lock) its alias comes from.
		locks, err := LockChannelsOf(r, nodes)
		if err != nil || len(locks) == 0 {
			return NameResolution{}, err
		}
		mutexes = locks
	}
	return NameResolution{QName: declaring, Nodes: mutexes, ByID: true}, nil
}

// LockChannelsOf returns the channels used as locks (types.ChannelLockSubKind)
// that the fields and variables among nodes hold: the make sites their
// channel aliases (flows_to) come from, and, for a variable (a local one:
// resetBusy), the lock channel made on the line that declares it in its
// file (a local's channel has no alias of its own).
func LockChannelsOf(r StoreReader, nodes []types.Node) ([]types.Node, error) {
	var ids []string
	var vars []types.Node
	for _, n := range nodes {
		switch n.Type {
		case types.NodeField:
			ids = append(ids, n.ID)
		case types.NodeVariable, types.NodeLocalVariable:
			ids = append(ids, n.ID)
			vars = append(vars, n)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	from, _, err := r.NeighborhoodByIDs(ids, 1, true, string(types.EdgeFlowsTo))
	if err != nil {
		return nil, fmt.Errorf("lock channels: %w", err)
	}
	isLock := func(n types.Node) bool { return n.Type == types.NodeChannel && n.SubKind == types.ChannelLockSubKind }
	var out []types.Node
	seen := map[string]bool{}
	for _, n := range from {
		if isLock(n) && !seen[n.ID] {
			seen[n.ID] = true
			out = append(out, n)
		}
	}
	for _, v := range vars {
		inFile, err := r.NodesByFilePath(v.FilePath)
		if err != nil {
			return nil, fmt.Errorf("lock channels of %s: %w", v.QualifiedName, err)
		}
		for _, n := range inFile {
			if isLock(n) && n.StartLine == v.StartLine && !seen[n.ID] {
				seen[n.ID] = true
				out = append(out, n)
			}
		}
	}
	return out, nil
}

// PrintedQName is the qualified name a tool prints for n: the qualified name,
// except for the Mutex node of a variable, which carries the variable's own
// name and is printed "<name>#mutex" (MutexSuffix), the name that resolves to
// it; printed plain, it named the variable. A field's Mutex node carries the
// suffix in its qualified name already.
func PrintedQName(n types.Node) string {
	if n.Type == types.NodeMutex && !strings.HasSuffix(n.QualifiedName, MutexSuffix) {
		return n.QualifiedName + MutexSuffix
	}
	return n.QualifiedName
}

// declPlace is where a declaration stands, for telling the declarations
// that share a qualified name apart (oneDeclaration).
type declPlace struct {
	language, file string
	line           int
}

// oneDeclaration returns the resolution of qname to nodes, all carrying it:
// the nodes when they are one declaration, else the candidates that name
// each declaration (see ResolveName).
func oneDeclaration(qname string, nodes []types.Node) NameResolution {
	printed := printedNames(nodes)
	all := len(nodes)
	named := withoutMutexOfValue(nodes)
	// Fewer nodes than carry the name: they are exact nodes, so a walk
	// starts from them and not from every node of the name.
	byID := len(named) < all
	reps := map[string]types.Node{}
	byLanguage := map[string]int{}
	for _, n := range named {
		if _, isCopy := PromotedFrom(n); isCopy {
			continue
		}
		k := declKey(n)
		if _, seen := reps[k]; !seen {
			reps[k] = n
			byLanguage[n.Language]++
		}
	}
	several := false
	for _, c := range byLanguage {
		several = several || c > 1
	}
	if !several {
		return NameResolution{QName: qname, Nodes: named, ByID: byID}
	}
	var ps []types.Node
	for _, n := range reps {
		if byLanguage[n.Language] > 1 {
			ps = append(ps, n)
		}
	}
	sort.Slice(ps, func(i, j int) bool { return placeLess(ps[i], ps[j]) })
	candidates := make([]string, 0, len(ps))
	seen := map[string]bool{}
	for _, n := range ps {
		if c := printed[n.ID]; !seen[c] {
			seen[c] = true
			candidates = append(candidates, c)
		}
	}
	return NameResolution{Candidates: candidates}
}

// oneSite reports whether nodes all have one declaration site.
func oneSite(nodes []types.Node) bool {
	for _, n := range nodes {
		if SiteOf(n) != SiteOf(nodes[0]) {
			return false
		}
	}
	return true
}

// otherCaseNames answers a name that matched nothing with the declarations
// whose name differs only in case, so a caller who wrote "newserver" learns
// that "NewServer" exists instead of concluding that nothing does.
func otherCaseNames(r StoreReader, name string, opts FindSymbolOptions) (NameResolution, error) {
	opts.IgnoreCase = true
	near, err := r.FindSymbol(name, false, opts)
	if err != nil {
		return NameResolution{}, fmt.Errorf("resolve %q without case: %w", name, err)
	}
	classes, err := ClassifyNames(r, near)
	if err != nil {
		return NameResolution{}, err
	}
	var declared []types.Node
	for _, n := range near {
		if classes[n.ID] == NameDeclared {
			declared = append(declared, n)
		}
	}
	names := distinctQNames(declared)
	if len(names) > otherCaseLimit {
		names = names[:otherCaseLimit]
	}
	return NameResolution{OtherCase: names}, nil
}

// candidatesOf names each declaration among nodes (the matches of a bare
// name) by its printed name, decided over the whole graph (PrintedName): a
// qualified name that other declarations carry is listed in its site form,
// once per declaration. Sorted, each once.
func candidatesOf(r StoreReader, nodes []types.Node) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range nodes {
		if _, isCopy := PromotedFrom(n); isCopy {
			if !seen[n.QualifiedName] {
				seen[n.QualifiedName] = true
				out = append(out, n.QualifiedName)
			}
			continue
		}
		if name := PrintedName(r, n); !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// distinctQNames returns the distinct qualified names of nodes, sorted, or
// nil when there are none.
func distinctQNames(nodes []types.Node) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range nodes {
		if !seen[n.QualifiedName] {
			seen[n.QualifiedName] = true
			out = append(out, n.QualifiedName)
		}
	}
	sort.Strings(out)
	return out
}

// sortNodesByID puts nodes collected in a map in id order, so a traversal
// returns the same list on every call: the neighbourhood walks returned their
// nodes in map order, and a tool that cut such a list (search_code's
// expand, at 10) showed a different part of it each time.
func sortNodesByID(nodes []types.Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}
