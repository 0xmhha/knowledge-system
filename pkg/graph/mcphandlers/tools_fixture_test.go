package mcphandlers

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// The files of the tool fixture.
const (
	fxStateFile  = "core/state/statedb.go"
	fxLoggerFile = "eth/tracers/logger/logger_test.go"
	fxEventFile  = "event/subscription.go"
	fxNamesFile  = "accounts/abi/abi.go"
	fxTypesFile  = "core/types/hashes.go"
	fxSlotFile   = "systemcontracts/stateutil.go"
	fxSolFile    = "contracts/Gov.sol"
	fxNodeFile   = "p2p/enode/urlv4_test.go"
)

// fxPrepareSrc is state.StateDB.Prepare, lines 10 to 14 of fxStateFile: a
// call that starts on line 12 and names its callee on line 13, which ends in
// two spaces.
const fxPrepareSrc = "func (s *StateDB) Prepare(rules int) {\n" +
	"\ts.logs = rules\n" +
	"\treturn wrap(rules).\n" +
	"\t\tReset(rules)  \n" +
	"}"

// fxTestPackSrc is abi.TestPack, lines 20 to 27 of fxNamesFile.
const fxTestPackSrc = "func TestPack() {\n" + // 20
	"\tPack := 1\n" + // 21
	"\tvar count int\n" + // 22
	"\ttype row struct {\n" + // 23
	"\t\tSize int\n" + // 24
	"\t}\n" + // 25
	"\tconst limit = 2\n" + // 26
	"}" // 27

// fxSlotSrc is systemcontracts.Slot, lines 1 to 3 of fxSlotFile.
const fxSlotSrc = "func Slot(base int, key interface{ Bytes() []byte }) int {\n" +
	"\treturn base\n" +
	"}"

// fxSubscriptionSrc is event.Subscription, lines 1 to 4 of fxEventFile: text
// follows each interface method on its line.
const fxSubscriptionSrc = "type Subscription interface {\n" +
	"\tErr() <-chan error // returns the error channel\n" +
	"\tUnsubscribe()      // cancels sending of events\n" +
	"}"

// fxParseNodeTestsSrc is enode.parseNodeTests, lines 30 to 34 of
// fxNodeFile: a local var of a function literal in its initialiser.
const fxParseNodeTestsSrc = "var parseNodeTests = func() []string {\n" + // 30
	"\t// the URL under test\n" + // 31
	"\tvar rawURL = \"enode://x\"\n" + // 32
	"\treturn []string{rawURL}\n" + // 33
	"}()" // 34

// fxGovSrc is the contract Gov, lines 1 to 22 of fxSolFile.
const fxGovSrc = "contract Gov {\n" + // 1
	"    error NotAMember();\n" + // 2
	"    event Voted(address who);\n" + // 3
	"    uint256 public quorum;\n" + // 4
	"    mapping(address => bool) members;\n" + // 5
	"    struct Member {\n" + // 6
	"        bool isActive;\n" + // 7
	"    }\n" + // 8
	"    enum Status {\n" + // 9
	"        Voting, // still open for votes\n" + // 10
	"        Executed // done\n" + // 11
	"    }\n" + // 12
	"    modifier onlyMember() {\n" + // 13
	"        if (!members[msg.sender]) revert NotAMember();\n" + // 14
	"        _;\n" + // 15
	"    }\n" + // 16
	"    function vote(uint256 id) external onlyMember {\n" + // 17
	"        quorum = id;\n" + // 18
	"        members[msg.sender] = true;\n" + // 19
	"        emit Voted(msg.sender);\n" + // 20
	"    }\n" + // 21
	"}" // 22

// fxNode is one node of the tool fixture and its source.
type fxNode struct {
	id, qname, canonical, file string
	kind                       types.NodeType
	start, end                 int
	src, lang, subKind, doc    string
	// at is the byte offset of the node in its file; zero for the nodes
	// whose place in their line the test does not depend on.
	at int
}

// fxLines returns lines from to to (1-based, inclusive) of src.
func fxLines(src string, from, to int) string {
	return strings.Join(strings.Split(src, "\n")[from-1:to], "\n")
}

// toolStore is a small graph with the shapes the query tools got wrong on
// go-stablenet: a method promoted into two test types, names that differ in
// case, function-local names, an External stub, an interface whose methods
// have trailing comments, a call that spans two lines, a type used by a
// package-level variable, and a Solidity contract with an error, an event, a
// modifier, a mapping, a state variable, a struct and an enum.
func toolStore(t *testing.T) persist.Store {
	t.Helper()
	s, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	promoted := "promoted from state.StateDB.Prepare"
	decls := []fxNode{
		// The promoted copies come first in the file, as they do in the
		// stored graph when their id sorts first.
		{id: "pc00000000000001", kind: types.NodeMethod, qname: "logger.dummyStatedb.Prepare", file: fxStateFile, start: 10, end: 14, src: fxPrepareSrc, doc: promoted},
		{id: "pc00000000000002", kind: types.NodeMethod, qname: "js.dummyStatedb.Prepare", file: fxStateFile, start: 10, end: 14, src: fxPrepareSrc, doc: promoted},
		{id: "st00000000000001", kind: types.NodeStruct, qname: "state.StateDB", canonical: "ex/state.StateDB", file: fxStateFile, start: 1, end: 3, src: "type StateDB struct {\n\tlogs int\n}"},
		{id: "st00000000000002", kind: types.NodeField, qname: "state.StateDB.logs", canonical: "ex/state.StateDB.logs", file: fxStateFile, start: 2, end: 2, src: "logs int"},
		{id: "st00000000000003", kind: types.NodeMethod, qname: "state.StateDB.Prepare", canonical: "ex/state.(*StateDB).Prepare", file: fxStateFile, start: 10, end: 14, src: fxPrepareSrc},
		{id: "st00000000000004", kind: types.NodeCallSite, qname: "state.StateDB.Prepare#CallSite@70-100", file: fxStateFile, start: 12, end: 13, src: "wrap(rules).\n\t\tReset(rules)"},
		{id: "st00000000000005", kind: types.NodeFunction, qname: "state.wrap", canonical: "ex/state.wrap", file: fxStateFile, start: 20, end: 20, src: "func wrap(n int) *StateDB { return nil }"},
		{id: "st00000000000006", kind: types.NodeMethod, qname: "state.StateDB.Reset", canonical: "ex/state.(*StateDB).Reset", file: fxStateFile, start: 30, end: 30, src: "func (s *StateDB) Reset(n int) {}"},
		{id: "lg00000000000001", kind: types.NodeStruct, qname: "logger.dummyStatedb", canonical: "ex/logger.dummyStatedb", file: fxLoggerFile, start: 5, end: 7, src: "type dummyStatedb struct {\n\tstate.StateDB\n}"},
		{id: "lg00000000000002", kind: types.NodeFunction, qname: "logger.TestPrepare", canonical: "ex/logger.TestPrepare", file: fxLoggerFile, start: 10, end: 12, src: "func TestPrepare(t *testing.T) {\n\tnew(dummyStatedb).Prepare(1)\n}"},

		{id: "ev00000000000001", kind: types.NodeInterface, qname: "event.Subscription", canonical: "ex/event.Subscription", file: fxEventFile, start: 1, end: 4, src: fxSubscriptionSrc},
		{id: "ev00000000000002", kind: types.NodeMethod, qname: "event.Subscription.Err", canonical: "ex/event.Subscription.Err", file: fxEventFile, start: 2, end: 2, src: "Err() <-chan error"},
		{id: "ev00000000000003", kind: types.NodeMethod, qname: "event.Subscription.Unsubscribe", canonical: "ex/event.Subscription.Unsubscribe", file: fxEventFile, start: 3, end: 3, src: "Unsubscribe()"},

		{id: "nm00000000000001", kind: types.NodeMethod, qname: "abi.ABI.Pack", canonical: "ex/abi.(ABI).Pack", file: fxNamesFile, start: 1, end: 1, src: "func (abi ABI) Pack() {}"},
		{id: "nm00000000000002", kind: types.NodeMethod, qname: "abi.Arguments.pack", canonical: "ex/abi.(Arguments).pack", file: fxNamesFile, start: 2, end: 2, src: "func (a Arguments) pack() {}"},
		// abi.TestPack declares, inside its body: a := name, a var, a type
		// with a field (stored from its name, as the graph has it today), a
		// const (stored from its keyword, as the extractor will have it) and
		// a goroutine.
		{id: "nm00000000000003", kind: types.NodeFunction, qname: "abi.TestPack", canonical: "ex/abi.TestPack", file: fxNamesFile, start: 20, end: 27, src: fxTestPackSrc},
		{id: "nm00000000000004", kind: types.NodeLocalVariable, qname: "abi.TestPack.Pack", file: fxNamesFile, start: 21, end: 21, src: "Pack := 1"},
		{id: "nm00000000000005", kind: types.NodeVariable, qname: "abi.TestPack.count", file: fxNamesFile, start: 22, end: 22, src: "count int"},
		{id: "nm00000000000011", kind: types.NodeStruct, qname: "abi.TestPack.row", file: fxNamesFile, start: 23, end: 25, src: "row struct {\n\t\tSize int\n\t}"},
		{id: "nm00000000000012", kind: types.NodeField, qname: "abi.TestPack.row.Size", file: fxNamesFile, start: 24, end: 24, src: "Size int"},
		{id: "nm00000000000013", kind: types.NodeConstant, qname: "abi.TestPack.limit", file: fxNamesFile, start: 26, end: 26, src: "const limit = 2"},
		{id: "nm00000000000014", kind: types.NodeGoroutine, qname: "abi.TestPack#Goroutine@77", file: fxNamesFile, start: 22, end: 22},
		{id: "nm00000000000006", kind: types.NodeVariable, qname: "abi.count", canonical: "ex/abi.count", file: fxNamesFile, start: 8, end: 8, src: "var count = 3"},
		{id: "nm00000000000007", kind: types.NodeMethod, qname: "coll1.Set.Size", canonical: "ex/coll1.(*Set).Size", file: fxNamesFile, start: 10, end: 10, src: "func (s *Set) Size() int { return 0 }"},
		{id: "nm00000000000008", kind: types.NodeMethod, qname: "coll2.Other.Size", canonical: "ex/coll2.(*Other).Size", file: fxNamesFile, start: 11, end: 11, src: "func (o *Other) Size() int { return 0 }"},
		{id: "nm00000000000009", kind: types.NodeFunction, qname: "coll1.use", canonical: "ex/coll1.use", file: fxNamesFile, start: 12, end: 12, src: "func use(s *Set) int { return s.Size() }"},
		{id: "nm00000000000010", kind: types.NodeExternal, qname: "strings.Pack", file: types.ExternalFilePath, start: 1, end: 1},

		// A function whose signature holds an inline interface: its method is
		// a declaration that starts in the middle of the function's line.
		{id: "sl00000000000001", kind: types.NodeFunction, qname: "systemcontracts.Slot", canonical: "ex/systemcontracts.Slot", file: fxSlotFile, start: 1, end: 3, src: fxSlotSrc},
		{id: "sl00000000000002", kind: types.NodeMethod, qname: "systemcontracts.Slot.interface@30.Bytes", file: fxSlotFile, start: 1, end: 1, src: "Bytes() []byte"},

		// Two names in one declaration, stored as the graph has them today
		// (each from its own name) and as two nodes with one text (each from
		// the keyword).
		{id: "ty00000000000005", kind: types.NodeVariable, qname: "types.forkA", canonical: "ex/types.forkA", file: fxTypesFile, start: 8, end: 8, src: "forkA, forkB = split()"},
		{id: "ty00000000000006", kind: types.NodeVariable, qname: "types.forkB", canonical: "ex/types.forkB", file: fxTypesFile, start: 8, end: 8, src: "forkB = split()"},
		{id: "ty00000000000007", kind: types.NodeVariable, qname: "types.twinA", canonical: "ex/types.twinA", file: fxTypesFile, start: 9, end: 9, src: "var twinA, twinB = split()"},
		{id: "ty00000000000008", kind: types.NodeVariable, qname: "types.twinB", canonical: "ex/types.twinB", file: fxTypesFile, start: 9, end: 9, src: "var twinA, twinB = split()"},

		// A blank declaration, a constant stored with its trailing comment,
		// and two declarations side by side on line 13.
		{id: "ty00000000000009", kind: types.NodeVariable, qname: "types._", file: fxTypesFile, start: 11, end: 11, src: "var _ Hasher = (*Hash)(nil)"},
		{id: "ty00000000000010", kind: types.NodeConstant, qname: "types.maxSkeleton", canonical: "ex/types.maxSkeleton", file: fxTypesFile, start: 12, end: 12, src: "const maxSkeleton = 128 // number of header fetches"},
		// A named and a blank name in one declaration, both with its text.
		{id: "ty00000000000013", kind: types.NodeVariable, qname: "types.total", canonical: "ex/types.total", file: fxTypesFile, start: 14, end: 14, src: "var total, _ = split()"},
		{id: "ty00000000000014", kind: types.NodeVariable, qname: "types._", file: fxTypesFile, start: 14, end: 14, src: "var total, _ = split()"},
		// A struct with a comment on its first line, and a string that
		// holds the comment delimiters.
		{id: "ty00000000000015", kind: types.NodeStruct, qname: "types.Holder", canonical: "ex/types.Holder", file: fxTypesFile, start: 16, end: 18, src: "type Holder struct { // holds the fetch window\n\tsize int\n}"},
		{id: "ty00000000000016", kind: types.NodeConstant, qname: "types.slashes", canonical: "ex/types.slashes", file: fxTypesFile, start: 20, end: 20, src: `const slashes = "a // b" /* c */ // two slashes`},
		// A spec of a `var (` group, lines 22 to 24: the group's opening
		// and closing lines are no declaration's text.
		{id: "ty00000000000017", kind: types.NodeVariable, qname: "types.grouped", canonical: "ex/types.grouped", file: fxTypesFile, start: 23, end: 23, src: "grouped = 1"},
		// A package-level variable whose initialiser calls a function
		// literal that declares a variable, lines 30 to 34 at byte 600.
		{id: "en00000000000001", kind: types.NodeVariable, qname: "enode.parseNodeTests", canonical: "ex/enode.parseNodeTests", file: fxNodeFile, start: 30, end: 34, at: 600, src: fxParseNodeTestsSrc},
		{id: "en00000000000002", kind: types.NodeVariable, qname: "enode.parseNodeTests.rawURL", file: fxNodeFile, start: 32, end: 32, at: 640, src: "var rawURL = \"enode://x\""},
		{id: "ty00000000000011", kind: types.NodeStruct, qname: "types.pairA", canonical: "ex/types.pairA", file: fxTypesFile, start: 13, end: 13, at: 300, src: "type pairA struct{}"},
		{id: "ty00000000000012", kind: types.NodeStruct, qname: "types.pairB", canonical: "ex/types.pairB", file: fxTypesFile, start: 13, end: 13, at: 321, src: "type pairB struct{}"},

		{id: "ty00000000000001", kind: types.NodeTypeAlias, subKind: "defined", qname: "common.Hash", canonical: "ex/common.Hash", file: fxTypesFile, start: 1, end: 1, src: "type Hash [32]byte"},
		{id: "ty00000000000002", kind: types.NodeVariable, qname: "types.EmptyRootHash", canonical: "ex/types.EmptyRootHash", file: fxTypesFile, start: 3, end: 3, src: "var EmptyRootHash = common.Hash{}"},
		{id: "ty00000000000003", kind: types.NodeConstant, qname: "types.zeroHash", canonical: "ex/types.zeroHash", file: fxTypesFile, start: 4, end: 4, src: "const zeroHash = common.Hash{}"},
		{id: "ty00000000000004", kind: types.NodeFunction, qname: "types.rootOf", canonical: "ex/types.rootOf", file: fxTypesFile, start: 6, end: 6, src: "func rootOf() common.Hash { return EmptyRootHash }"},

		{id: "so00000000000001", kind: types.NodeContract, lang: "sol", qname: "Gov", canonical: fxSolFile + ":Gov", file: fxSolFile, start: 1, end: 22, src: fxGovSrc},
		{id: "so00000000000002", kind: types.NodeError, lang: "sol", qname: "Gov.NotAMember", canonical: fxSolFile + ":Gov.NotAMember", file: fxSolFile, start: 2, end: 2, src: "error NotAMember();"},
		{id: "so00000000000003", kind: types.NodeEvent, lang: "sol", qname: "Gov.Voted", canonical: fxSolFile + ":Gov.Voted", file: fxSolFile, start: 3, end: 3, src: "event Voted(address who);"},
		{id: "so00000000000004", kind: types.NodeField, lang: "sol", subKind: "storage_public", qname: "Gov.quorum", canonical: fxSolFile + ":Gov.quorum", file: fxSolFile, start: 4, end: 4, src: "uint256 public quorum;"},
		{id: "so00000000000005", kind: types.NodeMapping, lang: "sol", qname: "Gov.members", canonical: fxSolFile + ":Gov.members", file: fxSolFile, start: 5, end: 5, src: "mapping(address => bool) members;"},
		{id: "so00000000000006", kind: types.NodeStruct, lang: "sol", qname: "Gov.Member", canonical: fxSolFile + ":Gov.Member", file: fxSolFile, start: 6, end: 8, src: fxLines(fxGovSrc, 6, 8)},
		{id: "so00000000000007", kind: types.NodeField, lang: "sol", subKind: "struct_member", qname: "Gov.Member.isActive", canonical: fxSolFile + ":Gov.Member.isActive", file: fxSolFile, start: 7, end: 7, src: "bool isActive;"},
		{id: "so00000000000008", kind: types.NodeEnum, lang: "sol", qname: "Gov.Status", canonical: fxSolFile + ":Gov.Status", file: fxSolFile, start: 9, end: 12, src: fxLines(fxGovSrc, 9, 12)},
		{id: "so00000000000009", kind: types.NodeConstant, lang: "sol", subKind: "enum_value", qname: "Gov.Status.Voting", canonical: fxSolFile + ":Gov.Status.Voting", file: fxSolFile, start: 10, end: 10, src: "Voting"},
		{id: "so00000000000010", kind: types.NodeConstant, lang: "sol", subKind: "enum_value", qname: "Gov.Status.Executed", canonical: fxSolFile + ":Gov.Status.Executed", file: fxSolFile, start: 11, end: 11, src: "Executed"},
		{id: "so00000000000011", kind: types.NodeModifier, lang: "sol", qname: "Gov.onlyMember", canonical: fxSolFile + ":Gov.onlyMember", file: fxSolFile, start: 13, end: 16, src: fxLines(fxGovSrc, 13, 16)},
		{id: "so00000000000012", kind: types.NodeFunction, lang: "sol", qname: "Gov.vote", canonical: fxSolFile + ":Gov.vote(uint256)", file: fxSolFile, start: 17, end: 21, src: fxLines(fxGovSrc, 17, 21)},
	}
	var nodes []types.Node
	blobs := map[string][]byte{}
	files := map[string]string{}
	for _, d := range decls {
		lang := d.lang
		if lang == "" {
			lang = "go"
		}
		name := d.qname[strings.LastIndex(d.qname, ".")+1:]
		nodes = append(nodes, types.Node{ID: d.id, Type: d.kind, Name: name, QualifiedName: d.qname, CanonicalID: d.canonical,
			FilePath: d.file, StartLine: d.start, EndLine: d.end, StartByte: d.at, EndByte: d.at + max(len(d.src), 1), Language: lang, SubKind: d.subKind,
			DocComment: d.doc, Confidence: types.ConfExtracted})
		if d.src != "" {
			blobs[d.id] = []byte(d.src)
		}
		if d.file != types.ExternalFilePath {
			files[d.file] = lang
		}
	}
	i := 0
	for f, lang := range files {
		i++
		nodes = append(nodes, types.Node{ID: "fi0000000000000" + string(rune('0'+i)), Type: types.NodeFile, Name: f,
			QualifiedName: "file:" + f, FilePath: f, StartLine: 1, EndLine: 1, EndByte: 1, Language: lang, Confidence: types.ConfExtracted})
	}
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertBlobs(blobs); err != nil {
		t.Fatal(err)
	}
	edge := func(src, dst string, kind types.EdgeType, file string, line int) types.Edge {
		return types.Edge{Src: src, Dst: dst, Type: kind, FilePath: file, Line: line, Count: 1, Confidence: types.ConfExtracted}
	}
	edges := []types.Edge{
		edge("lg00000000000001", "pc00000000000001", types.EdgeDefines, "", 0),
		edge("st00000000000001", "st00000000000003", types.EdgeDefines, "", 0),
		edge("st00000000000003", "st00000000000002", types.EdgeWritesField, fxStateFile, 11),
		edge("st00000000000003", "st00000000000005", types.EdgeCalls, "", 12),
		edge("st00000000000003", "st00000000000006", types.EdgeCalls, "", 12),
		edge("st00000000000003", "st00000000000004", types.EdgeContains, fxStateFile, 0),
		edge("st00000000000004", "st00000000000006", types.EdgeSiteCalls, fxStateFile, 12),
		edge("lg00000000000002", "st00000000000003", types.EdgeCalls, "", 11),
		edge("nm00000000000009", "nm00000000000007", types.EdgeCalls, "", 12),

		edge("ty00000000000002", "ty00000000000001", types.EdgeUsesType, fxTypesFile, 3),
		edge("ty00000000000003", "ty00000000000001", types.EdgeUsesType, fxTypesFile, 4),
		edge("ty00000000000004", "ty00000000000001", types.EdgeUsesType, fxTypesFile, 6),

		edge("so00000000000012", "so00000000000011", types.EdgeHasModifier, fxSolFile, 17),
		edge("so00000000000012", "so00000000000003", types.EdgeEmitsEvent, fxSolFile, 20),
		edge("so00000000000011", "so00000000000002", types.EdgeRevertsWith, fxSolFile, 14),
		edge("so00000000000011", "so00000000000005", types.EdgeReadsMapping, fxSolFile, 14),
		edge("so00000000000012", "so00000000000005", types.EdgeWritesMapping, fxSolFile, 19),
		edge("so00000000000012", "so00000000000004", types.EdgeWritesField, fxSolFile, 18),
	}
	if err := s.InsertEdges(edges); err != nil {
		t.Fatal(err)
	}
	if err := s.RebuildFTS(); err != nil {
		t.Fatal(err)
	}
	return s
}
