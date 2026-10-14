# Bounded retained Go dependence verification

Source commit337300b0 implements the original WI-1 Go CFG, typed lexical def-use,
local data and normal-exit control dependence through the shared graph API and
read-only public MCP. Independent expected tests cover kills, joins, shadow
scopes, parallel assignment, loop fixed points/self control, named returns,
constants, reference/aggregate/generic distinctions, fields/closures/non-exiting
regions, retained positions, limits, stale/mutating source and runtime schema.
The requirement-audit.json maps each original contract to evidence and limits.

The final exact clean-source binary (buildinfo+SHA) compiles/race-tests the owned
Go fixture, indexes it, then exercises31 requests through namespaced stdio MCP:
15 checks,11 registered tools, EOF exit0. Both same-name packages remain separate.
Deleting the live first source does not change the retained original result.
Original fixture source is in its bundle and original-source text; the post-query
source directory intentionally differs. Do not relabel it as original source.

Full primary Go test/build, related valueflow/parser/MCP/system race and repository
lint (fmt, vet, pinned2.14.0, boundaries, build-based docs) passed. CI runs the real
index/MCP fixture. Source inventory SHA is current; approved inputs are unchanged.
Old source bindings and early failures are retained. The deprecated AST binding
was replaced with isolated Go type bindings. The negative output-schema control
caught required:null causing SDK validation skip; the corrected schema validates
real success/not-found/budget responses and rejects the injected wrong status type.

This is bounded isolated Go syntax/type binding plus normal-exit analysis. It is
not package-wide alias, exception, field-memory, dynamic-dispatch or whole-program
vulnerability proof. Source/AST and cooperative deadlines bound work; no hard
real-time/whole-SQLite-memory guarantee. Constants(block-1) are not runtime CFG
statements. Missing/unknown/partial edges are not safety. Production and independent
whole-goal human acceptance remain not_recorded. No push/PR/merge was performed.
