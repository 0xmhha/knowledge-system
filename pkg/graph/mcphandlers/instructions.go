package mcphandlers

import "strings"

// maxGuideChars bounds the server guide. A client measured on 2026-11-27
// (Claude Code 2.1.332, headless runs) hands its model only the first 4,096
// characters of the server instructions and a "[truncated]" marker; what
// follows never reached an agent. The guide holds what an agent needs
// before its first call; every other rule is in the description of the
// tool it governs, or in the answer that it explains (a legend key).
const maxGuideChars = 4096

// maxToolTextChars bounds a tool's description and each of its parameter
// descriptions. The same client delivers tool descriptions whole (measured
// up to 2,277 characters); no longer one was measured, so no description
// relies on more than the guide's measured cut.
const maxToolTextChars = 4096

// guideText replaces the {tool} placeholders of a guide or description text
// with the tool names in force (the namespace rule), and ^ with a backtick,
// which a raw string cannot hold.
func guideText(s string) string {
	r := strings.NewReplacer(
		"{find_symbol}", ToolName("find_symbol"),
		"{search_text}", ToolName("search_text"),
		"{search_code}", ToolName("search_code"),
		"{search_test_code}", ToolName("search_test_code"),
		"{investigate}", ToolName("investigate"),
		"{investigate_open}", ToolName("investigate_open"),
		"{find_callers}", ToolName("find_callers"),
		"{find_callees}", ToolName("find_callees"),
		"{find_references}", ToolName("find_references"),
		"{get_subgraph}", ToolName("get_subgraph"),
		"{get_context_for_task}", ToolName("get_context_for_task"),
		"{impact_of_change}", ToolName("impact_of_change"),
		"{concurrency_impact}", ToolName("concurrency_impact"),
		"{change_history}", ToolName("change_history"),
		"{value_dependencies}", ToolName("value_dependencies"),
		"{evidence_for_intent}", ToolName("evidence_for_intent"),
		"{execution_flow}", ToolName("execution_flow"),
		"^", "`",
		"{caret}", "^",
	)
	return r.Replace(s)
}

// Instructions is the server-level guide an MCP client hands its model next
// to the tool list (the initialize result's "instructions"): what the server
// is, which tool answers which question, the name forms, and where the
// details are. It stays within maxGuideChars.
//
// Tool names follow the namespace rule in force when it is called, so call
// it after SetNamespaceRoot.
func Instructions() string { return guideText(instructionsTemplate) }

// searchCodeReasonsManual explains the reasons argument and the related key
// (the reasons parameter's description); only with the option on.
const searchCodeReasonsManual = `words for why you search: the behaviour or mechanism the problem is about, as words or identifier parts ("retry", "expire", "cache"). The result then has ^related^ after ^hits^: declarations next to the hits in the graph (callers, callees, values read or written) whose source or name holds a reason word, each with ^qname^, ^type^, ^file^, ^start_line^, ^end_line^, ^signature^, ^canonical_id^, ^reason_matched^ (the reason words it holds) and ^via^ (the hits it is next to: ^hit^, ^edge^ and ^dir^, "to" when the hit reaches it, "from" when it reaches the hit). The hits keep their order; related lists more reason words first. A related declaration may not match the pattern at all: that is how code the pattern missed gets in.`

// searchCodeNameAsBothManual says what in=name returns with the option on
// (the in parameter's description).
const searchCodeNameAsBothManual = ` With this server, in=name also returns body matches: name matches come first, then declarations whose body matches.`

const instructionsTemplate = `This server is a code graph of one repository: every function, method, type, field, variable and constant of the files the build compiles (Go, Solidity), and the edges between them: calls, interface dispatch, reads and writes of values, type uses, goroutines, channels, locks, sync primitives, and the merged PRs that changed each symbol. Every result carries file and line; read the code there to confirm. How to read an answer is in its tool's description; an answer with graph facts that need it carries a legend.

Pick the tool by the question:
- A problem or task in plain words: start with {search_code}, a grep over the code bodies: join the names and code shapes you expect with | ("Flush|Writer|bufSize", "sql\\.Open\\("). Test code: {search_test_code}. Then follow {find_callers} and {find_references}.
- A wide map around several words, when one search is not enough: {investigate}; what it lists by handle, and the card of any declaration: {investigate_open}.
- A symbol by name or part of its name: {find_symbol} (exact=false for a bare name), {search_text} (names, signatures and doc comments only, not bodies), or {search_code} with in=name.
- Who calls a function, what it calls: {find_callers}, {find_callees}.
- What runs this code: {find_callers}: its callers, the goroutines that run it, the channel receives whose message runs it with what wakes them, and the calls that run a literal it is written in.
- What a send or a close causes: {find_callees}, under triggers.
- What happens, in execution order, from a function, across its calls, the goroutines it starts and the messages it sends, with the locks held at each step: {execution_flow}; what leads to a function (callers, goroutine starts, senders whose message runs it): {execution_flow} with direction=backward. One call answers with a short numbered narrative of source lines instead of separate lists; a step that was not opened ends with the exact call that continues there, and focus=<field> keeps only the steps that read, write or guard that field.
- Who sets or reads a field, a package-level variable, a constant or a Solidity state variable: {find_references}. Use it when the bug is a wrong, stale or missing value: the fix is usually where the value is written.
- Data races, deadlocks, goroutines, channels, locks: {concurrency_impact}.
- What breaks if a symbol changes: {impact_of_change}.
- When and why a symbol or file changed: {change_history} lists every commit that changed its lines (with or without a pull request number); past changes like the one you are making: {evidence_for_intent}.
- How a value flows inside one function: {value_dependencies}.
- Everything around one symbol: {get_subgraph}. {get_context_for_task} matches a task against names and doc comments only and is often loosely related; prefer {search_code}.

What the graph does not see: packages the build does not compile and code outside the repository have no nodes (calls into them end at External stubs); a call through an interface or a function value is linked as far as the value flow shows, and implementations the type alone allows are marked "by type"; code reached only through reflection may be missing.

Names: a qname is "<package>.<Name>" or "<package>.<Type>.<Member>" (render.Template.Width); a canonical_id from an earlier result names one node exactly. A bare name works, in the case you write it, and an ambiguous name returns candidates, each named so that it opens one node ("<qname>@<file>:<line>"). Every name a result prints opens exactly that node. The full name rule is in {find_symbol}'s description.`

// nameRule is the full rule for names (find_symbol's description).
const nameRule = ` Names, in every tool: a qname is "<package>.<Name>" or "<package>.<Type>.<Member>" (for example render.Template.Width); every name a result prints opens exactly that node, so a node whose qualified name other declarations carry is printed in the site form or by its canonical id; a canonical id names its node even when other declarations carry it as their qualified name (the standard library's errors.New), and the answer then lists them under also_named; a bare name works, in the case you write it, and returns candidates when it is ambiguous; a qualified name that several declarations carry (a local name a function declares twice, the v1 and v2 copies of a contract, init functions, blank names) is ambiguous too, and its candidates name each declaration as "<qname>@<file>:<line>", with "#<n>" after it for the n-th name of one declaration ("var _, _ = f()"), and an External stub by its canonical id (the mutex of such a local as "<qname>#mutex@<file>:<line>"; a variable's name names the variable, "<name>#mutex" its mutex): {find_symbol}, {find_callers}, {find_callees}, {find_references}, {get_subgraph}, {impact_of_change}, {concurrency_impact}, {change_history} and {investigate_open} (card:<name>, which answers a local with the card of the declaration that holds it, after a line "local: <name> is declared at <file>:<line> in <owner>") take that form as a name, while {value_dependencies} takes only a node_id or canonical_id, and the seed_qname of {evidence_for_intent} only a qualified name; a canonical_id from an earlier result names one node exactly.`

// searchCodeReading explains how to read a search_code result, key by key
// (part of search_code's description; search_test_code's points to it).
const searchCodeReading = ` Reading a result (JSON). ^hits^: one entry per matching declaration, in the order above; the order is not relevance to your question, so scan the whole list. In each hit: ^qname^; ^type^ (Function, Method, Struct, ...); ^file^; ^start_line^ and ^end_line^ of the declaration; ^signature^; ^canonical_id^, an exact handle for the other tools; ^test^, true in a test file; ^name_match^, the pattern matched the qualified name or the declaration's first line without its comments (what in=name searches); ^declared_with^, the other names of a multi-name declaration (^var a, b = f()^): they share its lines, and a match there is counted once and listed under the name it touches, else under the first name; ^promoted_to^, on a method, the types that get it through an embedded field (T.Method for one of them is this declaration, listed once under its own name); ^matched^, the distinct texts matched, lowercased; ^matches^, how many times the pattern matched; ^lines^, every matched line, each with ^line^ (its number), ^text^ (the line from where the declaration's part of it starts, the indentation in front of a nested declaration included, with context lines when asked for) and ^uses^ (what the graph resolved on that line: "calls X", "reads X", "writes X", "calls a function value that may run A, B", "calls I.M (may run A, B; 2 more by type)", in Solidity also "emits E", "reverts with E", "applies modifier M"; a call over several lines is listed on its first line and on the line that names the callee; a call into code outside the build is named by its package name, "calls strings.Split", or by its import path when two packages in the result share a name); ^callers^, ^callees^ and ^references^ with their totals, when asked for with expand (see expand). At the top: ^pattern^; ^total_declarations^, ^total_files^ and ^total_matches^, the size of the whole result; ^note^ and ^without_filters^, why a result is empty; ^error^, a pattern that could not be used or a part that does not exist; ^ids_not_matched^, ids you asked for that the pattern did not match; ^packages^, once each, the packages outside the build that ^uses^ names: ^name^, ^path^ (import path), ^module^ ("std" for the standard library) and ^version^ (absent for std). A result too large for one response comes as an index (see ids) or, when the server has a result folder, as a ^file^ only: ^path^ (a JSON file holding the full result), ^declarations^, ^bytes^, ^lines^ and ^read_me^; read that file, in parts with offset and limit when it is too large, before you draw any conclusion.`

// searchCodePatternRule says what text the pattern is matched against (the
// pattern parameter's description).
const searchCodePatternRule = `RE2 regular expression; | separates alternatives. With fixed=true, a literal string. What is searched: the stored text of every declaration, line by line: functions, methods (interface methods included), types, constants and package-level variables (Solidity: contracts, interfaces, libraries, functions, modifiers, events, errors, state variables, structs, enums and enum values); a blank declaration (^var _ I = (*T)(nil)^) too, which in=name cannot return. A declaration's text runs from its first token (the keyword, or a spec's name in a ^var (^, ^const (^ or ^type (^ group) through its last token and the plain comment after it on that line (in Solidity the comma after an enum value may stand between; a NatSpec comment there belongs to the next declaration). Each match is reported once, under the declaration that owns its first character. A declaration inside another one (an interface method, a type or constant in a function, a Solidity member of its contract) owns its lines with the indentation in front and what follows on its last line; other text in front of it on its first line belongs to the enclosing declaration. A variable declared in a function or in a function literal of a package-level variable's initialiser, and a struct field, are part of their function, variable or struct, not hits of their own. ^{caret}^ and ^$^ apply to the line as stored: inside a function, type or contract the whole source line; for a top-level declaration its own text, so ^{caret}^ matches at its first token even when indented, and no match runs from one top-level declaration into another on the same line. Not searched: the package clause, imports, comments above or between declarations, the opening line of a ^var (^, ^const (^ or ^type (^ group and its closing ^)^ line, and Solidity pragma and import directives.`

// searchCodeIdsRule explains the index and how to ask for code (the ids
// parameter's description).
const searchCodeIdsRule = `qname (or canonical_id) of declarations to return in full: their matched lines with code, uses and expansion. Use it after a result that came as an index: its first key is then ^index^: ^declarations^ (how many matched), ^with_lines^ (rows given with their matched lines), ^names_only^ (rows after those, by name, file and lines only), ^not_listed^ (declarations that did not fit even by name; the top-level ^not_listed^ counts them per file) and ^read_me^. Each index row is short: ^qname^, ^type^, ^file^, ^span^ ("158-262"), ^matched^, ^matches^, ^test^, ^name_match^, ^at^ (the matched line numbers) and ^uses^ (line -> what the graph resolved there); ^canonical_id^ only when two rows share a qname. Call again with the same pattern and ids=[...]; several at once. Choose from the whole index, not only its first rows.`

// searchCodePageRule explains parts (the page parameter's description).
const searchCodePageRule = `part to return (default 1). A result asked for with ids that is still too large comes in parts; its first key is then ^page^: ^number^, ^of^, ^declarations^ ("1-12 of 30"), ^next^ and ^get_next^. Call again with the same arguments and page=^next^ for the rest.`
