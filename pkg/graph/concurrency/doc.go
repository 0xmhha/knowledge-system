// Package concurrency computes the concurrency blast radius of a symbol:
// every module that affects or is affected by it via the contract's five
// concurrency edge types (spawns, sends_to, recvs_from, acquires_lock,
// accessed_under_lock), BFS in both directions over a store.Reader.
//
// It backs the cks.context.concurrency_impact tool. cks
// calls Analyze in-process; the dev-only ckg MCP server may also expose it
// via pkg/mcphandlers.RegisterConcurrencyImpact.
//
// Direction semantics on each returned Module:
//
//	"affected_by" — forward edge: seed -> ... -> module
//	"affects"     — reverse edge: module -> ... -> seed
//	"both"        — reached in both traversals
//
// Design notes:
//
//   - releases_lock is intentionally excluded. The unlock half of a lock
//     pair adds no "what is affected" signal over acquires_lock.
//
//   - A field that is a mutex is two nodes: the Field and a Mutex node named
//     "<field>#mutex", where the lock edges end. A field seed that has such
//     a node starts from both, so it reaches its lockers and the fields the
//     mutex guards; Result.Mutex names the Mutex node. A package-level or
//     local variable that is a mutex is a Variable or LocalVariable and a
//     Mutex node with the variable's own name, so its name resolves to both.
//     The node names differ and stay (ids and stored graphs depend on them);
//     the seed forms do not: every mutex is seeded by its declaring name, or
//     by that name with "#mutex".
//
//   - The store's neighbourhood walk is single-direction per call, so a
//     FUNCTION seed reaches its own goroutine and the channel it writes,
//     but NOT the peer goroutine across the channel (crossing a channel
//     requires switching edge direction at the Channel node). To recover
//     producer<->consumer or lock-sharing peers, seed a Channel / Mutex /
//     Field node instead. This makes the byzantine-fairness query "what
//     else touches this field/mutex under the same lock" work by seeding
//     the Field/Mutex node and reading "affects".
//
//   - Channel and lock edges are usually INFERRED confidence; they are
//     surfaced (not filtered to EXTRACTED), or concurrency impact would be
//     empty on real Go code.
package concurrency
