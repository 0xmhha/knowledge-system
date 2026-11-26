package solidity

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// fileIndex answers which file declares a node and which files a file can
// see through its import directives.
type fileIndex struct {
	// byNode maps a node id to the file that declares it.
	byNode map[string]string
	// imports maps a file to the files its import directives name, in
	// source order.
	imports map[string][]string
	// reach caches, per file, the distance of every file it can see: 0 for
	// itself, 1 for a file it imports, 2 for an import of an import.
	reach map[string]map[string]int
	// aliases maps a file to the names its import directives introduce.
	aliases map[string]map[string]importAlias
}

// importAlias is a name an import directive introduces in the importing
// file: `import { Thing as T2 } from "./x.sol"` makes T2 stand for Thing of
// x.sol, and `import "./x.sol" as X` makes X stand for the file (name is
// then empty, and `X.Thing` is Thing of x.sol).
type importAlias struct {
	file string
	name string
}

// newFileIndex returns an index with no nodes and no imports.
func newFileIndex() *fileIndex {
	return &fileIndex{
		byNode:  map[string]string{},
		imports: map[string][]string{},
		reach:   map[string]map[string]int{},
		aliases: map[string]map[string]importAlias{},
	}
}

// of returns the file that declares the node, or "" for an unknown id.
func (f *fileIndex) of(id string) string { return f.byNode[id] }

// addImport records that file imports target.
func (f *fileIndex) addImport(file, target string) {
	f.imports[file] = append(f.imports[file], target)
	f.reach = map[string]map[string]int{}
}

// addAlias records that alias, written in file, stands for name of target,
// or for the whole target file when name is empty.
func (f *fileIndex) addAlias(file, alias, target, name string) {
	if f.aliases[file] == nil {
		f.aliases[file] = map[string]importAlias{}
	}
	f.aliases[file][alias] = importAlias{file: target, name: name}
}

// Words of an import directive.
const (
	keywordAs      = "as"
	importWildcard = "*"
)

// importAliasesOf returns the names an import directive introduces, read
// from its text: alias → imported name for `{ Name as Alias }`, and alias →
// "" for a whole-file alias (`import "./x.sol" as X`, `import * as X from
// "./x.sol"`).
func importAliasesOf(directive string) map[string]string {
	out := map[string]string{}
	outside := directive
	if open, end := strings.IndexByte(directive, '{'), strings.IndexByte(directive, '}'); open >= 0 && end > open {
		for _, item := range strings.Split(directive[open+1:end], ",") {
			if words := strings.Fields(item); len(words) == 3 && words[1] == keywordAs {
				out[words[2]] = words[0]
			}
		}
		outside = directive[:open] + " " + directive[end+1:]
	}
	words := strings.Fields(outside)
	for i := 1; i+1 < len(words); i++ {
		if words[i] != keywordAs {
			continue
		}
		if prev := words[i-1]; prev == importWildcard || strings.HasSuffix(prev, `"`) || strings.HasSuffix(prev, "'") {
			out[words[i+1]] = ""
		}
	}
	return out
}

// distances returns the distance of every file visible from file.
func (f *fileIndex) distances(file string) map[string]int {
	if d, ok := f.reach[file]; ok {
		return d
	}
	d := map[string]int{file: 0}
	queue := []string{file}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range f.imports[cur] {
			if _, seen := d[next]; seen {
				continue
			}
			d[next] = d[cur] + 1
			queue = append(queue, next)
		}
	}
	f.reach[file] = d
	return d
}

// nearest returns the candidates declared in the file closest to srcFile:
// srcFile itself, else the files it imports, else their imports. It returns
// nil when no candidate is in a file visible from srcFile. Order is kept.
func (f *fileIndex) nearest(ids []string, srcFile string) []string {
	if srcFile == "" {
		return nil
	}
	dist := f.distances(srcFile)
	best := -1
	var out []string
	for _, id := range ids {
		d, visible := dist[f.byNode[id]]
		if !visible {
			continue
		}
		switch {
		case best < 0 || d < best:
			best, out = d, []string{id}
		case d == best:
			out = append(out, id)
		}
	}
	return out
}

// pick chooses, among nodes that share a name, the one a reference written
// in srcFile means: the one declared in srcFile, else the one in the nearest
// imported file. When none is visible through imports (an import the build
// could not follow, such as a remapped path) the only candidate is taken, and
// with several candidates the first one, which is what every name lookup did
// before imports were recorded. ids must not be empty.
//
// Every name-keyed lookup of a contract, library, interface or library
// function goes through here. Before, each lookup preferred the same file and
// otherwise took the first node with that name, so `GovMinter is GovBase`
// and `Lib.f` could bind to a same-named declaration of another folder.
func (f *fileIndex) pick(ids []string, srcFile string) string {
	if near := f.nearest(ids, srcFile); len(near) > 0 {
		return near[0]
	}
	return ids[0]
}

// confidence is EXTRACTED for a relation inside one file and INFERRED for one
// that crosses files, the rule every Solidity edge follows.
func (f *fileIndex) confidence(srcID, dstID string) types.Confidence {
	src, dst := f.byNode[srcID], f.byNode[dstID]
	if src != "" && dst != "" && src != dst {
		return types.ConfInferred
	}
	return types.ConfExtracted
}

// relativeImportTarget returns the file a relative import path (`./x.sol`,
// `../x.sol`) names, seen from importer, or "" for any other path.
func relativeImportTarget(importer, importPath string) string {
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		return path.Join(path.Dir(filepath.ToSlash(importer)), importPath)
	}
	return ""
}

// resolveImportPath returns the file an import directive of importer names,
// or "" when no file of the graph matches. A relative path is joined with
// the importer's directory. Any other path is matched exactly, then as the
// only file that ends with it, because a remapping (`@lib/x.sol`) cannot be
// followed without the build settings.
func resolveImportPath(importer, importPath string, files map[string]bool) string {
	if target := relativeImportTarget(importer, importPath); target != "" {
		if files[target] {
			return target
		}
		return ""
	}
	if files[importPath] {
		return importPath
	}
	match := ""
	for file := range files {
		if strings.HasSuffix(file, "/"+importPath) {
			if match != "" {
				return ""
			}
			match = file
		}
	}
	return match
}

// scopeIndex resolves a name the way the compiler does for the contract that
// uses it: the contract itself, then its bases in linearisation order, then
// the file level (the file, then the files it imports). It never returns a
// member of a contract that is not a base.
//
// Before this index every name was looked up in one global table and the
// first node won. On the system contracts that sent 21 of 70 has_modifier
// edges and 14 of 64 emits_event edges to another contract's declaration
// (`GovMinter.whenNotPaused` to `GovMasterMinter`'s modifier) and 3 of 8
// writes_mapping edges from v2 code to the v1 mapping.
type scopeIndex struct {
	files *fileIndex
	// node maps a node id to its node.
	node map[string]*types.Node
	// owner maps a member to the contract, library or interface that
	// defines it.
	owner map[string]string
	// members maps a container to its members by name, in declaration order.
	members map[string]map[string][]string
	// fileLevel maps a file to the declarations it makes outside any
	// container, containers included, by name.
	fileLevel map[string]map[string][]string
	// topLevel maps a name to every file-level declaration of that name in
	// the graph. It serves a file whose import could not be followed.
	topLevel map[string][]string
	// mro maps a container to its linearisation, itself first.
	mro map[string][]string
	// children maps a struct to its members and an enum to its values, by
	// name; parent maps each of those back to its struct or enum.
	children map[string]map[string]string
	parent   map[string]string
	// structs maps a struct name to every struct of that name.
	structs map[string][]string
}

// newScopeIndex builds the index from nodes alone. Membership is read from
// the qualified name: `Container.member` in a file belongs to the container
// of that name in the same file (a file cannot declare two), and a name
// without a container is a file-level declaration. Pass 1 names every member
// that way (memberQName) and records the same fact as a defines edge, but an
// incremental build hands Resolve the nodes of unchanged files without their
// edges, so the index cannot rest on edges. Struct members and enum values
// belong to their struct or enum and are not in the index.
func newScopeIndex(nodes []types.Node, files *fileIndex) *scopeIndex {
	s := &scopeIndex{
		files:     files,
		node:      make(map[string]*types.Node, len(nodes)),
		owner:     map[string]string{},
		members:   map[string]map[string][]string{},
		fileLevel: map[string]map[string][]string{},
		topLevel:  map[string][]string{},
		mro:       map[string][]string{},
		children:  map[string]map[string]string{},
		parent:    map[string]string{},
		structs:   map[string][]string{},
	}
	containerAt := map[string]string{}
	typeAt := map[string]string{}
	for i := range nodes {
		n := &nodes[i]
		s.node[n.ID] = n
		switch {
		case isContainerType(n.Type):
			containerAt[n.FilePath+refSep+n.Name] = n.ID
		case n.Type == types.NodeStruct:
			typeAt[n.FilePath+refSep+n.QualifiedName] = n.ID
			s.structs[n.Name] = append(s.structs[n.Name], n.ID)
		case n.Type == types.NodeEnum:
			typeAt[n.FilePath+refSep+n.QualifiedName] = n.ID
		}
	}
	for i := range nodes {
		n := &nodes[i]
		switch {
		case n.Type == types.NodeFile, n.Type == types.NodeImport:
			continue
		case n.SubKind == subKindStructMember, n.SubKind == subKindEnumValue:
			// A struct member or an enum value belongs to the struct or
			// enum its qualified name is under.
			if parentID, found := typeAt[n.FilePath+refSep+parentQName(n.QualifiedName)]; found {
				s.parent[n.ID] = parentID
				if s.children[parentID] == nil {
					s.children[parentID] = map[string]string{}
				}
				s.children[parentID][n.Name] = n.ID
			}
			continue
		}
		containerName, _, qualified := strings.Cut(n.QualifiedName, qnameSep)
		if !qualified {
			if s.fileLevel[n.FilePath] == nil {
				s.fileLevel[n.FilePath] = map[string][]string{}
			}
			s.fileLevel[n.FilePath][n.Name] = append(s.fileLevel[n.FilePath][n.Name], n.ID)
			s.topLevel[n.Name] = append(s.topLevel[n.Name], n.ID)
			continue
		}
		containerID, found := containerAt[n.FilePath+refSep+containerName]
		if !found {
			continue
		}
		s.owner[n.ID] = containerID
		if s.members[containerID] == nil {
			s.members[containerID] = map[string][]string{}
		}
		s.members[containerID][n.Name] = append(s.members[containerID][n.Name], n.ID)
	}
	return s
}

// basesOf returns the names of the bases a contract or interface lists in
// its `is` clause, in source order, read from its declaration header
// (`contract C is A, lib.B(1)` gives A and B). The header is on the node, so
// the bases of a contract in an unchanged file are known on an incremental
// build, when its inheritance references are not handed to Resolve.
func basesOf(n *types.Node) []string {
	words := strings.Fields(n.Signature)
	start := -1
	for i, w := range words {
		if w == keywordIs && i > 0 {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}
	var bases []string
	for _, part := range splitTopLevel(strings.Join(words[start:], " ")) {
		if open := strings.IndexByte(part, '('); open >= 0 {
			part = part[:open]
		}
		part = strings.TrimSpace(part)
		if dot := strings.LastIndex(part, qnameSep); dot >= 0 {
			part = part[dot+1:]
		}
		if part != "" {
			bases = append(bases, part)
		}
	}
	return bases
}

// keywordIs starts the list of bases in a contract header.
const keywordIs = "is"

// splitTopLevel splits text at the commas that are outside any parentheses,
// brackets or braces.
func splitTopLevel(text string) []string {
	var parts []string
	depth, from := 0, 0
	for i, r := range text {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, text[from:i])
				from = i + 1
			}
		}
	}
	return append(parts, text[from:])
}

// matchingParen returns the index of the parenthesis that closes the one at
// open, or -1.
func matchingParen(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parameterCount returns the number of parameters in a callable's
// declaration header (`function f(address a, uint256 b) external` gives 2).
// ok is false when the header has no parameter list. Overloads are told
// apart by this count; it is read from the node for the same reason as
// basesOf.
func parameterCount(n *types.Node) (count int, ok bool) {
	open := strings.IndexByte(n.Signature, '(')
	if open < 0 {
		return 0, false
	}
	end := matchingParen(n.Signature, open)
	if end < 0 {
		return 0, false
	}
	list := strings.TrimSpace(n.Signature[open+1 : end])
	if list == "" {
		return 0, true
	}
	return len(splitTopLevel(list)), true
}

// inheritanceParents resolves the bases of every container, as written in
// its header, to container nodes: container → direct bases in source order.
// A base no file of the graph declares is left out.
func (s *scopeIndex) inheritanceParents() map[string][]string {
	parents := map[string][]string{}
	for id, n := range s.node {
		if !isContainerType(n.Type) {
			continue
		}
		for _, base := range basesOf(n) {
			if baseID := s.container(n.FilePath, base); baseID != "" && baseID != id {
				parents[id] = append(parents[id], baseID)
			}
		}
	}
	return parents
}

// linearisation returns the container and its bases in the order names are
// looked up. A container without recorded bases is its own linearisation.
func (s *scopeIndex) linearisation(containerID string) []string {
	if lin := s.mro[containerID]; len(lin) > 0 {
		return lin
	}
	if containerID == "" {
		return nil
	}
	return []string{containerID}
}

// filter keeps the ids whose node type accept allows.
func (s *scopeIndex) filter(ids []string, accept func(types.NodeType) bool) []string {
	var out []string
	for _, id := range ids {
		if n := s.node[id]; n != nil && accept(n.Type) {
			out = append(out, id)
		}
	}
	return out
}

// lookup resolves name as used inside containerID, in a file. It returns
// every declaration of that name at the first level that has one (overloads
// share a level), or nil. containerID is "" for code at file level.
func (s *scopeIndex) lookup(containerID, file, name string, accept func(types.NodeType) bool) []string {
	if ids := s.lookupInherited(s.linearisation(containerID), name, accept); len(ids) > 0 {
		return ids
	}
	return s.lookupFileLevel(file, name, accept)
}

// lookupInherited resolves name among the members of the given containers,
// first container first.
func (s *scopeIndex) lookupInherited(containers []string, name string, accept func(types.NodeType) bool) []string {
	for _, c := range containers {
		if ids := s.filter(s.members[c][name], accept); len(ids) > 0 {
			return ids
		}
	}
	return nil
}

// lookupFileLevel resolves name among file-level declarations: those of the
// file, then of the nearest imported file. A name the file's imports
// introduce as an alias (`import { Thing as T2 }`) is the declaration it
// stands for; before, `contract C is T2` had no base and every reference to
// an inherited member was dropped. When the file's imports lead to no
// declaration of that name and the whole graph has exactly one, that one is
// taken: the import that brings it in could not be followed.
func (s *scopeIndex) lookupFileLevel(file, name string, accept func(types.NodeType) bool) []string {
	if alias, isAlias := s.files.aliases[file][name]; isAlias && alias.name != "" && (alias.file != file || alias.name != name) {
		return s.lookupFileLevel(alias.file, alias.name, accept)
	}
	candidates := s.filter(s.topLevel[name], accept)
	if len(candidates) == 0 {
		return nil
	}
	if near := s.files.nearest(candidates, file); len(near) > 0 {
		return near
	}
	if len(candidates) == 1 {
		return candidates
	}
	return nil
}

// container resolves the name of a contract, library or interface as written
// in file, or "" when none is visible.
func (s *scopeIndex) container(file, name string) string {
	if ids := s.lookupFileLevel(file, name, isContainerType); len(ids) > 0 {
		return ids[0]
	}
	return ""
}

// isContainerType reports whether t is a contract, library or interface.
func isContainerType(t types.NodeType) bool {
	return t == types.NodeContract || t == types.NodeInterface
}

// isStorageType reports whether t is a state variable, a constant or a
// mapping: the kinds a value identifier in a function body can name.
func isStorageType(t types.NodeType) bool {
	return t == types.NodeField || t == types.NodeConstant || t == types.NodeMapping
}

// isStateVariable reports whether n is a state variable that occupies
// storage: a Field that is neither a struct member nor an immutable, or a
// Mapping. Storage layout code takes its inputs from here.
func isStateVariable(n *types.Node) bool {
	switch n.Type {
	case types.NodeMapping:
		return true
	case types.NodeField:
		return n.SubKind != subKindStructMember && n.SubKind != subKindImmutable
	}
	return false
}

// inheritStateVarTypes adds to each container the state variables of its
// bases, nearest base first, so the receiver of `set.add(x)` is found when a
// base declares `set`. A name the container or a nearer base declares wins.
func inheritStateVarTypes(stateVarTypes stateVarTypeMap, mro map[string][]string) {
	own := make(stateVarTypeMap, len(stateVarTypes))
	for cid, vars := range stateVarTypes {
		own[cid] = vars
	}
	for cid, lin := range mro {
		if len(lin) < 2 {
			continue
		}
		merged := map[string]string{}
		for _, ancestor := range lin {
			for name, typeName := range own[ancestor] {
				if _, taken := merged[name]; !taken {
					merged[name] = typeName
				}
			}
		}
		if len(merged) > 0 {
			stateVarTypes[cid] = merged
		}
	}
}

// contextOf returns the container and the file in whose scope a name written
// at a declaration is looked up: the declaration's container (the container
// of its struct or enum for a member or a value), or "" at file level.
func (s *scopeIndex) contextOf(id string) (containerID, file string) {
	if parentID, nested := s.parent[id]; nested {
		id = parentID
	}
	if n := s.node[id]; n != nil {
		file = n.FilePath
	}
	return s.owner[id], file
}

// isTypeDeclaration reports whether t is a kind that can be named as a type:
// a struct, an enum, a contract, a library or an interface.
func isTypeDeclaration(t types.NodeType) bool {
	return t == types.NodeStruct || t == types.NodeEnum || isContainerType(t)
}

// typeDeclaration resolves a type name as written (`Item`, `Shapes.Box`) in
// the scope of a container and a file, or returns "". A qualified name is
// looked up segment by segment: the first in scope, each next one among the
// members of the container before it.
func (s *scopeIndex) typeDeclaration(containerID, file, name string) string {
	segments := strings.Split(strings.TrimSpace(name), qnameSep)
	var ids []string
	if target, isFile := s.fileAlias(file, segments[0]); isFile && len(segments) > 1 {
		// `Space.Thing`, where Space is a whole-file import alias.
		ids = s.lookupFileLevel(target, segments[1], isTypeDeclaration)
		segments = segments[1:]
	} else {
		ids = s.lookup(containerID, file, segments[0], isTypeDeclaration)
	}
	for _, segment := range segments[1:] {
		if len(ids) == 0 || !isContainerType(s.node[ids[0]].Type) {
			return ""
		}
		ids = s.lookupInherited(s.linearisation(ids[0]), segment, isTypeDeclaration)
	}
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// structOf resolves the struct a type name means, or returns "". The name is
// looked up as written in the given scope. A declared type is stored on
// some nodes without its qualifier (`Set` for `SetLib.Set`); when the scope
// has no struct of that name, the only struct of that name in the files the
// scope's file can see is taken.
func (s *scopeIndex) structOf(containerID, file, typeName string) string {
	if id := s.typeDeclaration(containerID, file, typeName); id != "" {
		if s.node[id].Type == types.NodeStruct {
			return id
		}
		return ""
	}
	candidates := s.structs[lastSegment(strings.TrimSpace(typeName))]
	if near := s.files.nearest(candidates, file); len(near) == 1 {
		return near[0]
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

// fileAlias returns the file a whole-file import alias written in file
// stands for (`import "./x.sol" as X`).
func (s *scopeIndex) fileAlias(file, name string) (target string, ok bool) {
	alias, isAlias := s.files.aliases[file][name]
	if !isAlias || alias.name != "" {
		return "", false
	}
	return alias.file, true
}
