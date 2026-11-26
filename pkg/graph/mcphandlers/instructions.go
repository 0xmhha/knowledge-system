package mcphandlers

import "strings"

// Instructions is the server-level guide an MCP client hands its model next
// to the tool list (the initialize result's "instructions"). Tool
// descriptions say what one tool does; this says which tool answers which
// question, what the graph holds and what it cannot see, so a model picks the
// graph for the questions it answers better than a text search.
//
// Tool names follow the namespace rule in force when it is called, so call
// it after SetNamespaceRoot.
func Instructions() string {
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
	)
	manual := searchCodeManual
	if searchCodeReasons {
		manual += "\n" + searchCodeReasonsManual
	}
	if searchCodeNameAsBoth {
		manual += "\n" + searchCodeNameAsBothManual
	}
	return r.Replace(instructionsTemplate + "\n\n" + strings.ReplaceAll(manual, "^", "`"))
}

// searchCodeReasonsManual explains the reasons argument and the related key;
// it is sent only when the server has the option on.
const searchCodeReasonsManual = `- Reasons: give {search_code} reasons, the words for why you search (the behaviour or mechanism the problem is about), next to the pattern. The result then has ^related^ after ^hits^: declarations next to the hits in the graph whose source or name holds a reason word, each with ^qname^, ^type^, ^file^, ^start_line^, ^end_line^, ^signature^, ^canonical_id^, ^reason_matched^ (the reason words it holds) and ^via^ (the hits it is next to: ^hit^, ^edge^ and ^dir^, "to" when the hit reaches it, "from" when it reaches the hit). The hits keep their order; related lists more reason words first. A related declaration may not match the pattern at all: that is how code the pattern missed gets in.`

// searchCodeNameAsBothManual says what in=name returns with the option on.
const searchCodeNameAsBothManual = `- in=name also returns body matches: name matches come first, then declarations whose body matches.`

const instructionsTemplate = `This server is a code graph of one repository: every function, method, type, field, variable and constant of the files the build compiles (Go, Solidity), and the edges between them — calls, interface dispatch, reads and writes of fields and package-level values, type uses, goroutines, channels and locks, and the merged PRs that changed each symbol. Every result carries file and line; read the code there to confirm.

Pick the tool by the question:
- A problem or task in plain words: start with {search_code}. Turn the problem into the names and code you expect and join them with | ("Flush|Writer|bufSize"), or search code shapes as you would grep ("func \\(.*\\) \\w*Render\\w*\\(", "sql\\.Open\\("). It answers per function with the matched lines, the calls and value uses on them, and what an interface or function-value call may run. Then follow {find_callers} and {find_references}.
- A wide map around several names, when one search is not enough: {investigate}. For each word it returns what the word names, every declaration that holds it with its calls, reads and writes and the conditions each write runs under, who writes and who reads each value, and what two or more words reach within three hops. What does not fit is listed with a handle; {investigate_open} returns it, and the card of any declaration.
- A symbol by name, or by part of its name: {find_symbol} (exact=false for a bare name), {search_text}, or {search_code} with in=name.
- Who calls a function, what it calls: {find_callers}, {find_callees}.
- Who sets or reads a field, a package-level variable, a constant or a Solidity state variable: {find_references}. Use it when the bug is a wrong, stale or missing value: the fix is usually where the value is written.
- Data races, deadlocks, goroutines, channels, locks: {concurrency_impact}.
- What breaks if a symbol changes: {impact_of_change}.
- When and why a symbol changed: {change_history}.
- How a value flows inside one function: {value_dependencies}.
- Everything around one symbol: {get_subgraph}.

What the graph does not see:
- {search_text} and {get_context_for_task} match symbol names, qualified names, signatures and doc comments only, and {get_context_for_task} often returns loosely related code. To find code by what it does or uses, search the bodies with {search_code}, then follow {find_callers} and {find_references}.
- Only the build's files: packages the build does not compile, and code outside the repository, have no nodes.
- A call through an interface or a function value is linked to the code it may run as far as the value flow shows; implementations allowed by the type alone are marked "by type". Code reached only through reflection may be missing.

Names: a qname is "<package>.<Name>" or "<package>.<Type>.<Member>" (for example render.Template.Width); every name a result prints opens exactly that node, so a node whose qualified name other declarations carry is printed in the site form or by its canonical id; a canonical id names its node even when other declarations carry it as their qualified name (the standard library's errors.New), and the answer then lists them under also_named; a bare name works, in the case you write it, and returns candidates when it is ambiguous; a qualified name that several declarations carry (a local name a function declares twice, the v1 and v2 copies of a contract, init functions, blank names) is ambiguous too, and its candidates name each declaration as "<qname>@<file>:<line>", with "#<n>" after it for the n-th name of one declaration ("var _, _ = f()"), and an External stub by its canonical id (the mutex of such a local as "<qname>#mutex@<file>:<line>"; a variable's name names the variable, "<name>#mutex" its mutex): {find_symbol}, {find_callers}, {find_callees}, {find_references}, {get_subgraph}, {impact_of_change}, {concurrency_impact}, {change_history} and {investigate_open} (card:<name>, which answers a local with the card of the declaration that holds it, after a line "local: <name> is declared at <file>:<line> in <owner>") take that form as a name, while {value_dependencies} takes only a node_id or canonical_id, and the seed_qname of {evidence_for_intent} only a qualified name; a canonical_id from an earlier result names one node exactly.`

// searchCodeManual explains how to read a search_code result, key by key, and
// how a result too large for one response comes (an index; details by ids).
// It sits in the server guide, which the client places next to the system
// prompt. Parts were dropped: in agent runs (Claude Code 2.1.326) no agent
// asked for part 2, with or without this guide (runs/...-nocut2,
// runs/...-guidesplit). ^ stands for a backtick, which a raw string cannot
// hold.
const searchCodeManual = `Reading a {search_code} result (JSON):
- {search_code} searches production code only; {search_test_code} searches test code (_test.go files and testdata) with the same arguments and keys.
- What is searched: the stored text of every declaration, line by line: functions, methods (interface methods included), types, constants and package-level variables (Solidity: contracts, interfaces, libraries, functions, modifiers, events, errors, state variables, structs, enums and enum values). A blank declaration (^var _ I = (*T)(nil)^) is searched too; it has no name, so in=name does not return it. A declaration's text runs from its first token through its last token and the plain comment that follows it on that line, in Go and in Solidity (where the comma after an enum value may stand between, and a NatSpec ^///^ or ^/** */^ comment there is left to the next declaration it documents); its first token is the keyword (^func^, or ^type^, ^var^, ^const^ standing alone) or, for a spec in a ^var (^, ^const (^ or ^type (^ group, its name. Each match is reported once, under the declaration that owns its first character. A declaration inside another one (an interface method, a type or constant inside a function, a Solidity function, event or state variable inside its contract) owns its lines together with the indentation in front of it and with what follows it on its last line, up to the next declaration; other text in front of it on its first line belongs to the enclosing declaration. A variable declared inside a function, a variable declared in a function literal of a package-level variable's initialiser, and a struct field (Go or Solidity) are not hits of their own: they are part of their function, of that package-level variable, or of their struct. The anchors for the start and the end of a line apply to the line as far as it is stored: inside a function, type or contract that is the whole source line; for a declaration that is not inside another one it is that declaration's own text, so the start anchor matches at its first token even when it is indented (a spec in a ^var (^ group), and when two such declarations stand on one line no anchor matches between them and no match runs from one into the other. Text outside every declaration is not searched: the package clause, imports, comments above or between declarations, the opening line of a ^var (^, ^const (^ or ^type (^ group and its closing ^)^ line, and, in Solidity, pragma and import directives.
- ^hits^: one entry per matching declaration. Order: a match in the name first; then more matches; then declarations that more of the repository depends on (PageRank). Declarations equal on these keep no meaningful order. The order is not relevance to your question, so scan the whole list.
- In each hit: ^qname^; ^type^ (Function, Method, Struct, ...); ^file^; ^start_line^ and ^end_line^ of the declaration; ^signature^; ^canonical_id^, an exact handle to pass to the other tools; ^test^, true in a test file (search_test_code); ^name_match^, the pattern matched the qualified name or the declaration's first line without its comments (what in=name searches); ^declared_with^, the other names of a multi-name declaration (^var a, b = f()^): they share its lines, and a match there is counted once and listed under the name it touches, else under the first name; ^promoted_to^, on a method, the types that get it through an embedded field (T.Method for one of them is this declaration, which is listed once, under its own name); ^matched^, the distinct texts the pattern matched, lowercased; ^matches^, how many times the pattern matched in the declaration; ^lines^, every matched line, each with ^line^ (the line number), ^text^ (the line from where the declaration's part of it starts, the indentation in front of a nested declaration included, with context lines when asked for) and ^uses^ (what the graph resolved on that line: "calls X", "reads X", "writes X", "calls a function value that may run A, B", "calls I.M (may run A, B; 2 more by type)", and in Solidity also "emits E", "reverts with E", "applies modifier M"; a call that spans several lines is listed on its first line and on the line that names the callee; a call into code outside the build, such as the standard library or a dependency, is named by its package name, as in "calls strings.Split", or by its import path when two packages in the result share a name); ^callers^, ^callees^ and ^references^, the next hop when asked for with expand (at most 10 each: callers and callees the first 10 by qualified name, then file and line; ^callers_total^, ^callees_total^ or ^references_total^ counts them when there are more; the counted set is exactly what {find_callers} or {find_callees} with qname=<the hit's ^canonical_id^, or its ^qname^ when it has none> and depth=1 returns besides the hit itself, and what {find_references} with that qname and limit=<^references_total^> returns).
- At the top: ^pattern^; ^total_declarations^, ^total_files^ and ^total_matches^, the size of the whole result; ^note^ and ^without_filters^, why a result is empty; ^error^, a pattern that could not be used or a part that does not exist; ^ids_not_matched^, ids you asked for that are not among the declarations the pattern matched; ^packages^, once each, the packages outside the build that the ^uses^ name: ^name^ (the package name used in ^uses^), ^path^ (the import path), ^module^ ("std" for the standard library, else the module path) and ^version^ (the module version, absent for std).
- File: when the server has a result folder, a result too large for one response is not sent at all; its only key is ^file^: ^path^ (an indented JSON file holding the full result, with every key above), ^declarations^, ^bytes^, ^lines^ and ^read_me^ (the same in words). Nothing of the result is in the response. Read that file before you draw any conclusion from the search; when the reader says it is too large, read it in parts with offset and limit until the end.
- Index: without a result folder, a result too large for one response comes as an index, and then its first key is ^index^: ^declarations^ (how many matched), ^with_lines^ (how many rows are given in full: with their matched lines, or none for a row matched in its name only), ^names_only^ (rows after those, by name, file and lines only), ^not_listed^ (declarations that did not fit even by name; the top-level ^not_listed^ counts them per file) and ^read_me^ (the same in words). Each row of ^hits^ is short: ^qname^, ^type^, ^file^, ^span^ (start and end line, "158-262"), ^matched^, ^matches^, ^test^, ^name_match^, ^at^ (the matched line numbers) and ^uses^ (matched line number -> what the graph resolved on that line, only lines that have any); ^canonical_id^ appears only when two rows share a qname. To see the code of the declarations you choose, call {search_code} with the same pattern and ids=[their ^qname^ or ^canonical_id^, ...]; you may ask for several at once. Choose from the whole index, not only its first rows.
- Parts: a result asked for with ids that is still too large comes in parts; its first key is then ^page^: ^number^, ^of^, ^declarations^ (such as "1-12 of 30"), ^next^ and ^get_next^. Call again with the same arguments and page=^next^ for the rest.`
