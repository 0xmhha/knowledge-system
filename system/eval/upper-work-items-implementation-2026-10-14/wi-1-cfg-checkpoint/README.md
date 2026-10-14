# Go CFG kernel checkpoint

This is a partial implementation checkpoint, not WI-1 completion. The retained
function blob/hash and coordinates feed x/tools CFG successors/branch joins,
loops, returns and unreachable blocks with source/AST/block/node/edge/time caps.
Def-use/data/control dependence remain not_computed; output remains partial.
No new graph MCP is registered and no absent edge is claimed safe.

The source checkpoint passes full Go test/build, related valueflow/Go-parser/MCP
race, vet, repository-pinned lint v2.14.0, formatting, engine boundaries and
build-based docs checks. The first branch-label test failure is retained; last
condition-expression binding fixed it. Synthetic return endpoints stay inside
the original retained bytes. Tests include real graph successors (not only AST
counts), loop back-edge/unreachable code, source slices, unsupported/effect
uncertainty, budgets/cancellation and invalid input. Five meaningful Go tests.

This source binding records the current primary inventory after adding two Go
files. Existing WI-3 public wiring/source and approved inputs are unchanged;
previous execution evidence and independent-acceptance limits remain historical.
It does not invent a public execution for this unregistered kernel. Next work:
lexical reaching definitions/kill/join/loop, data/control dependence, public
source-bound graph queries, actual indexing/MCP execution and final evidence.
