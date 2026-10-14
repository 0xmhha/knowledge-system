# Opt-in Go rule candidates

Contract version 1; pack `go-local-review@1.0.0`. Automatic analysis is enabled
only by an explicit pack selection. It reads retained function blobs and reuses
one CFG, isolated Go type binding, reaching-definition and control analysis.
It never writes policy/security YAML, graph annotations or reviewer approval.

## Frozen initial rules

| Rule | Source | Sink | Interpretation |
| --- | --- | --- | --- |
| `parameter-command-name` | Named scalar function parameter | Direct identifier first argument of syntactic `exec.Command` or `exec.CommandContext` (argument 1 for Context) | Parameter-derived command-name review candidate; command arguments alone do not match |
| `discarded-conversion-error` | First result of `strconv.Atoi`, `ParseInt`, `ParseUint` when second assignment target is `_` | Direct identifier in an explicit return | Conversion result may reach a return after its error was discarded |

Selector matches are syntax assumptions, not resolved imported functions. Renamed
imports, indirect calls, qualified field/index sinks, closures and alias-mediated
flows are outside initial matching. A same-named local selector can cause a false
positive. Parameters are potential input sources, not proof of attacker control.
No integer-overflow, command-injection or exploitability verdict is produced.

One witness is retained per source/sink pair; alternative path feasibility is not enumerated.
Paths follow lexical reaching definitions, then corresponding assignment RHS
uses; multi-result RHS dependencies are conservative. Branches and loops may
reach without proving runtime feasibility. Control conditions cite original
nodes and labels. Calls on a path remain opaque. Sanitizer semantics are
`not_proven`; apparent sanitizer calls do not suppress candidates. Replacing an
input with a literal kills that lexical flow, without proving whole-program
safety. Manual annotations retain independent provenance.

## Public input and output

Read-only CKG/CKS MCP `rule_candidates` accepts required `rule_pack` (exact ID above),
required `function` (the strict `value_dependencies` identity/pins/limits object),
optional positive `max_candidates` ≤64 and `max_flow_steps` ≤4096. There is no
implicit default pack. Unknown fields, duplicates, nulls, ambiguous identities,
stale pins, fractions and out-of-range bounds are rejected.

The output is the shared value-dependency envelope with optional `analysis.rules`:
pack ID/version/SHA256, engine version, automatic origin, status, stop reason,
actual matching-work steps, limits, candidates and limitations. Each candidate
has a stable ID, rule ID, source/sink locations, path node IDs, control conditions,
certainty and unverified target/sanitizer status. The encompassing analysis
retains immutable function/blob/snapshot pins, node locations and diagnostics.
Unsupported or exhausted base analysis cannot produce a complete rule result;
empty candidates never mean safe. Bounds cover matching work and emitted
candidates, with the same cooperative total query deadline as value analysis.

Implementation verification requires compiled owned Go cases and actual retained
index → namespaced MCP, including positive, kill, command-argument-only,
branch/loop/shadow, sanitizer unknown, selector spoof/rename, discarded/checked
errors, manual annotation coexistence and bounded incomplete results. Precision
and recall denominators must name this fixture scope; independent vulnerability
acceptance is separate.
