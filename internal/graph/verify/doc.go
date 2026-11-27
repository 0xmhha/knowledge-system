// Package verify checks a built code graph against what the Go compiler
// front end resolves for the same source.
//
// The truth side loads the packages with go/packages and go/types and
// records, for every Go file the graph covers:
//
//   - package-level declarations (funcs, methods, types, consts, vars) with
//     their import-path-qualified canonical id and position;
//   - static calls, whose callee go/types resolves (typeutil.StaticCallee);
//   - interface method calls, through an interface receiver or an interface
//     embedded in a struct, and dynamic calls (function values, closures),
//     which have no static callee and are counted separately;
//   - references from a declaration to a package-level named type, including
//     the signature of a function declared without a Go body and the type and
//     initialiser of a package-level var or const;
//   - the calls and type references that leave the covered files (standard
//     library, dependencies, files of the module the build scope leaves out,
//     the builtin error method), which the graph holds as edges to External
//     nodes.
//
// The graph side reads the nodes and the calls, invokes, uses_type and
// instantiates edges. Each truth declaration is matched to the graph node in
// the same file whose canonical id (without the "@line" duplicate suffix)
// equals it and whose span contains the declaration's name. A truth pair
// (caller, callee) is then looked up as an edge between those two nodes.
//
// The comparison reports, per relation, how many truth pairs the graph has,
// how many it attaches to a different node with the same name that go/types
// does not support either (misresolved), how many it lacks (missing), and how
// many graph edges between known declarations have no truth pair. Numbers are
// measurements, not a pass or fail verdict.
package verify
