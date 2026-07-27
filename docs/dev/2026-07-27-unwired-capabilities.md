# Unwired capabilities — decision (2026-07-27)

Status: Tier-3 decision record. Three packages were flagged during the
dead-code audit (B1) as reachable from nothing. Rigorous inspection showed they
are **built-but-not-yet-wired intended features**, not dead code — deleting them
would lose deliberate work, and wiring them is separate feature work (behavior
change) that needs product intent/design. **Decision: keep all three; document
status + wiring path here; do not delete, do not wire in this pass.**

## The three

| Package | What it is | Evidence it is intended | Wiring path (if adopted) |
|---|---|---|---|
| `internal/vector/embed/coreml` | Native CoreML embedder (Apple ANE) behind `//go:build darwin && tokenizers`; Objective-C bridge + tokenizer; documented as `ckv build --embedder=coreml`. | Real native code + build tags + a documented CLI surface. Zero cost when the `tokenizers` tag is off. | Add `"coreml"` to the `ckv build --embedder` dispatch under the `tokenizers` build tag; decide whether CoreML is a supported (vs experimental) backend. |
| `internal/vector/embed/cache` | Hot-path LRU cache wrapping a `types.Embedder` so repeated intents skip the model. | Clear purpose doc; a natural optimization for multi-hop intents / re-embedding. | Wrap the embedder where the build/query path constructs it; add a size knob. Small, self-contained. |
| `internal/system/{observe,auditlog}` | Append-only, hash-chained, tamper-evident audit log + `observe` glue tying it to the footprint stream. | Config already carries `logging.audit_dir` (written by `system-mcp gen-config`, currently consumed by nothing) — the feature was planned. | Emit audit records in the composer at the capability gate + sanitize hits; open the log at `audit_dir`. A real security feature — design it (what is audited, retention) before wiring. |

## Why not delete, why not wire now

- **Not delete.** Each is deliberate, documented, and (for coreml) substantial
  native work; the audit's "unreachable" verdict was about *current wiring*, not
  intent. `graph_digest`/behavior are unaffected by their presence.
- **Not wire now.** Wiring each is a behavior change needing a product decision:
  is CoreML a supported embedder? do we want an embedding cache's memory
  trade-off on by default? what exactly does the audit log capture and how long
  is it kept? Those are separate, intent-driven features — not cleanup.

## Follow-up

If/when adopted, each wiring is an independent change (see the table). Until
then they stay in-tree, compiled (coreml only under its build tag), and tracked
here so they are not mistaken for dead code in a future sweep.
