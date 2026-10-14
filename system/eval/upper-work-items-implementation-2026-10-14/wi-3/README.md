# Bounded suspect signals: controlled implementation evidence

CKS source commit: `63a7411742426839244948743dd02b902df350aa`.
Coding-agent source commit: `b9290af1982b55693a5f49bbc9fad5dc2cbb76af`.

This packet records implementation verification, not independent production
acceptance or a confirmed cause. It preserves:

- Whole-module build, standard/race tests (119 tested packages), fmt/vet/lint,
  engine-boundary and documentation checks; 96 plugin tests and bench checks.
- Real owned Go source and Git bundle, real SQLite graph, actual stdio MCP
  request/result receipts, two consumer rounds and explicit stop/unknowns.
- Final committed-source Claude Code Streamable HTTP call with the exact
  `mcp__plugin_coding-agent_cks__cks_context_label_suspects` host name. It confirms
  normalization and read-only execution, not full installed plugin-role flow.
- Provider/consumer input conformance for all29 registered tools, null/source/
  revision/budget/ambiguity/policy/error/round/hash/handoff negative controls.
- Recoverable foreign repository patch, original commit object and incremental
  bundle with prerequisite `068d3c3f2340a89317ca1cbb11f5bf1d30f620b1`.

To reproduce, build `cmd/graph` and the `internal/system/mcp` test binary at the
source commit, restore the coding-agent commit, and run `bench/suspects/smoke.py`
with `--ckg`, `--server`, `--consumer`, and a fresh `--out`. The fixture is owned
and synthetic; no existing nodes are stopped. The HTTP fixture was terminated
with SIGTERM and returned PASS; the stdio fixture exited after EOF.

The code-specific source hashes and unchanged approved-input hashes refresh
only current source binding. Historical acceptance packets remain unchanged.
Symbol/site/text and pinned archive contracts have meaningful unit controls;
the real Go integration used graph seeds, with a fake vector backend that was
not queried. A real model-backed text discovery and production investigation,
independent acceptance, and full installed bugfix-role execution remain separate.
Security/dataflow provider connections are deliberately explicit unsupported
nulls here and will be implemented by their responsible later work items.
