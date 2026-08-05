GO ?= go
GOLANGCI_LINT ?= golangci-lint

BIN_DIR := bin
LOG_DIR := logs

# cks-mcp build stamp — injected into main.builderVersion and surfaced via
# cks_ops_health.builder_version, so a running MCP can be matched to the source
# it was built from. Derived from the git tag: on a tagged commit this is the
# tag (e.g. v0.1.0); off-tag it appends -<n>-g<sha>; "-dirty" flags an
# uncommitted build.
CKS_COMMIT  := $(shell git describe --tags --always --dirty --abbrev=8 2>/dev/null || echo 0.1.0-dev)
CKS_VERSION := cks-mcp/$(CKS_COMMIT)
CKS_LDFLAGS := -X main.builderVersion=$(CKS_VERSION)

# Dogfood eval (cks indexing cks itself) — used to produce the retrieval
# baseline in eval/reports/. Override these to point at a different
# target repo for cross-repo eval runs.
CKG            ?= ckg
CKG_DATA       ?= .ckg-data
# Dogfood source = the consolidated module root. system/ itself holds no Go
# files since the consolidation, so "." would index an empty corpus. The graph
# side narrows to the system engine via the main-package closure (CKG_MAINS);
# the vector side has no closure flag, so it indexes the module root — dogfood
# is wire-up verification, not a gate, so the wider corpus is acceptable.
CKG_SRC        ?= ..
CKG_MAINS      ?= ./cmd/system-mcp,./cmd/system/agent,./cmd/system/eval
CKG_LANG       ?= go
CKV            ?= ckv
CKV_DATA       ?= .ckv-data
CKV_SRC        ?= ..
# CKV_EMBEDDER is the ckv build backend (mock | bgeonnx | ollama).
# CKV_MODEL is the embedding MODEL NAME — required when the backend is
# ollama, and written into the fused-server config's embed_model (the
# cks-side ckv adapter embeds queries through ollama with that model).
# The two used to share one variable, which made the ollama path
# unconstructable: the backend name leaked into embed_model and the
# server's connectivity check asked ollama for a model named "mock".
CKV_EMBEDDER   ?= mock
CKV_MODEL      ?=
CKV_CFG_MODEL   = $(if $(CKV_MODEL),$(CKV_MODEL),$(CKV_EMBEDDER))
# USE_CKV controls whether dogfood-eval wires ckv as a real backend.
# Default 0 — but NOTE: since the composer retrieval fix, get_for_task
# fails closed without a ready ckv leg, so USE_CKV=0 runs only exercise
# the wire-up of the non-semantic tools; every get_for_task scenario
# reports "service unavailable". For meaningful recall numbers run:
#   make dogfood-eval USE_CKV=1 CKV_EMBEDDER=ollama CKV_MODEL=bge-m3 \
#        CKG=/path/to/ckg CKV=/path/to/ckv
# (needs `ollama serve` + the model pulled; embedding CKV_SRC takes
# minutes-to-hours depending on corpus size — narrow CKV_SRC to bound it).
USE_CKV        ?= 0
DOGFOOD_CONFIG ?= cks-dogfood.yaml
EVAL_SCENARIOS ?= eval/scenarios
EVAL_REPORT    ?= eval/reports/baseline-dogfood.json

.PHONY: all build build-bins test test-short race vet lint fmt fmt-check tidy clean help \
        log-clear log-tail \
        ckg-index ckg-index-clean ckv-index ckv-index-clean \
        dogfood-config dogfood-eval dogfood-eval-summary

all: build

help:
	@echo "Build:"
	@echo "  build         go build ./..."
	@echo "  build-bins    build cks-mcp, cks-agent, cks-eval, cks-glossary-gen,"
	@echo "                cks-inventory-check, cks-entry-verify, cks-anchor-refresh into ./bin/"
	@echo ""
	@echo "Quality:"
	@echo "  test          go test -race ./..."
	@echo "  test-short    go test -short ./..."
	@echo "  vet           go vet ./..."
	@echo "  lint          golangci-lint run"
	@echo "  fmt           gofmt -s -w ."
	@echo "  fmt-check     fail if files need gofmt"
	@echo "  tidy          go mod tidy"
	@echo ""
	@echo "Logs (dev cycle: collect -> inspect -> clear -> rerun):"
	@echo "  log-clear     remove ./logs/* (footprint + audit)"
	@echo "  log-tail      show tail of each .jsonl log, jq-prettified if available"
	@echo "                (auditlog.Verify() is exposed as a library; a CLI flag"
	@echo "                 lands with cks-eval in Phase E)"
	@echo ""
	@echo "Dogfood eval (cks indexing cks itself):"
	@echo "  ckg-index           build a fresh ckg index from \$$(CKG_SRC) into \$$(CKG_DATA)"
	@echo "  ckg-index-clean     remove \$$(CKG_DATA)"
	@echo "  ckv-index           build a fresh ckv index from \$$(CKV_SRC) into \$$(CKV_DATA)"
	@echo "  ckv-index-clean     remove \$$(CKV_DATA)"
	@echo "  dogfood-config      generate \$$(DOGFOOD_CONFIG) (cks.yaml; ckv block iff USE_CKV=1)"
	@echo "  dogfood-eval        ckg-only WIRE-UP smoke (default; get_for_task scenarios"
	@echo "                      all error — composer requires ckv). Recall/MRR numbers:"
	@echo "  dogfood-eval        USE_CKV=1 CKV_EMBEDDER=ollama CKV_MODEL=<model> (real embed)"
	@echo "  dogfood-eval-summary  print one-line metric summary from \$$(EVAL_REPORT)"
	@echo "                  overrides: CKG=/path/to/ckg, CKV=/path/to/ckv, CKV_EMBEDDER, ..."
	@echo ""
	@echo "  clean         remove ./bin and go build cache"

build:
	$(GO) build ./...

build-bins:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(CKS_LDFLAGS)" -o $(BIN_DIR)/cks-mcp ../cmd/system-mcp
	$(GO) build -o $(BIN_DIR)/cks-agent           ../cmd/system/agent
	$(GO) build -o $(BIN_DIR)/cks-eval            ../cmd/system/eval
	$(GO) build -o $(BIN_DIR)/cks-glossary-gen    ../cmd/system/glossary-gen
	$(GO) build -o $(BIN_DIR)/cks-inventory-check ../cmd/system/inventory-check
	$(GO) build -o $(BIN_DIR)/cks-entry-verify    ../cmd/system/entry-verify
	$(GO) build -o $(BIN_DIR)/cks-anchor-refresh  ../cmd/system/anchor-refresh

# Engine-scoped package set. system/ itself holds no Go packages since the
# consolidation (engine source lives at ../internal/system etc.), so ./...
# here would silently match nothing. internal/setup is boundaries-wise its
# own layer but is knowledge-setup's implementation, so it belongs to this
# engine's test scope.
SYSTEM_PKGS = ../cmd/system/... ../cmd/system-mcp/... ../cmd/knowledge-setup/... \
              ../internal/system/... ../internal/setup/... ../pkg/system/...
# The same set as directories, for tools that take dirs, not package patterns.
SYSTEM_DIRS = ../cmd/system ../cmd/system-mcp ../cmd/knowledge-setup \
              ../internal/system ../internal/setup ../pkg/system

test:
	$(GO) test -race $(SYSTEM_PKGS)

test-short:
	$(GO) test -short $(SYSTEM_PKGS)

race: test

vet:
	$(GO) vet $(SYSTEM_PKGS)

lint:
	$(GOLANGCI_LINT) run $(SYSTEM_PKGS)

fmt:
	gofmt -s -w $(SYSTEM_DIRS)

fmt-check:
	@diff=$$(gofmt -s -l $(SYSTEM_DIRS)); \
	if [ -n "$$diff" ]; then \
		echo "gofmt needs to format:"; echo "$$diff"; exit 1; \
	fi

tidy:
	$(GO) mod tidy

clean: ckg-index-clean ckv-index-clean
	rm -rf $(BIN_DIR) $(DOGFOOD_CONFIG)
	@echo "cleaned $(BIN_DIR), $(CKG_DATA), $(CKV_DATA), $(DOGFOOD_CONFIG)"
	@echo "(go build cache untouched — run 'go clean -cache' manually)"

log-clear:
	@rm -rf $(LOG_DIR)
	@mkdir -p $(LOG_DIR)/footprint $(LOG_DIR)/audit
	@echo "cleared $(LOG_DIR)/{footprint,audit}"

log-tail:
	@found=0; \
	for f in $$(find $(LOG_DIR) -name '*.jsonl' 2>/dev/null); do \
		found=1; \
		echo "===== $$f (last 20) ====="; \
		if command -v jq >/dev/null 2>&1; then \
			tail -n 20 "$$f" | jq -c '.'; \
		else \
			tail -n 20 "$$f"; \
		fi; \
	done; \
	if [ $$found -eq 0 ]; then echo "no logs found in $(LOG_DIR)/"; fi

# ---------------------------------------------------------------------
# Dogfood eval — cks-mcp runs against a ckg index built from cks itself,
# then cks-eval scores the corpus and writes a JSON report.
#
# The whole flow is wire-up only — every artifact is rebuilt every time
# so the resulting baseline always reflects the current working tree.
# Override CKG / CKG_SRC / CKG_DATA / DOGFOOD_CONFIG / EVAL_REPORT to
# retarget against a different repo or a different scenario set.
# ---------------------------------------------------------------------

# ckg-index always rebuilds (.PHONY): cks changes invalidate the index,
# and ckg's own incremental cache is fast enough that a clean rebuild
# is cheap.
ckg-index:
	@command -v $(CKG) >/dev/null 2>&1 || { \
		echo "ckg binary not found on PATH; set CKG=/path/to/ckg"; exit 1; }
	@rm -rf $(CKG_DATA)
	@mkdir -p $(CKG_DATA)
	$(CKG) build --src $(CKG_SRC) --files-from-main $(CKG_MAINS) \
	    --out $(CKG_DATA) --lang $(CKG_LANG)
	@echo "ckg index built: $(CKG_DATA)/graph.db"

ckg-index-clean:
	rm -rf $(CKG_DATA)

ckv-index: ckv-backend-guard
	@command -v $(CKV) >/dev/null 2>&1 || { \
		echo "ckv binary not found on PATH; set CKV=/path/to/ckv"; exit 1; }
	@rm -rf $(CKV_DATA)
	@mkdir -p $(CKV_DATA)
	$(CKV) --embedder=$(CKV_EMBEDDER) build --src $(CKV_SRC) --out $(CKV_DATA) \
		$(if $(CKV_MODEL),--model-name=$(CKV_MODEL),)
	@echo "ckv index built: $(CKV_DATA)/vector.db"

# USE_CKV=1 with a non-ollama backend cannot work end-to-end: the fused
# server's ckv adapter embeds queries via ollama and its connectivity
# check 404s on a model named after the mock/bgeonnx backend. Fail fast
# before an index is built or a config with a bogus embed_model is written.
.PHONY: ckv-backend-guard
ckv-backend-guard:
	@if [ "$(USE_CKV)" = "1" ] && [ "$(CKV_EMBEDDER)" != "ollama" ]; then \
		echo "ERROR: USE_CKV=1 requires CKV_EMBEDDER=ollama CKV_MODEL=<model>"; \
		echo "  (the fused server embeds queries via ollama; '$(CKV_EMBEDDER)' cannot pass its connectivity check)"; \
		exit 1; \
	fi
	@if [ "$(CKV_EMBEDDER)" = "ollama" ] && [ -z "$(CKV_MODEL)" ]; then \
		echo "ERROR: CKV_EMBEDDER=ollama requires CKV_MODEL=<model> (e.g. bge-m3)"; \
		exit 1; \
	fi

ckv-index-clean:
	rm -rf $(CKV_DATA)

# dogfood-config writes a minimal cks.yaml that points at the local
# indexes. Skips overwrite if the file already exists; delete it first
# to regenerate. The ckv block fires only when USE_CKV=1 (default 0)
# because the ckv subprocess link is currently unstable (see USE_CKV
# doc above). With USE_CKV=0 the cks-mcp ckv adapter stays on the
# in-memory Fake.
dogfood-config: ckv-backend-guard
	@if [ -f $(DOGFOOD_CONFIG) ]; then \
		echo "$(DOGFOOD_CONFIG) already exists (delete to regenerate)"; \
	else \
		ckv_path=""; ckv_bin=""; \
		if [ "$(USE_CKV)" = "1" ]; then \
			ckv_path="./$(CKV_DATA)"; \
			ckv_bin=$$(command -v $(CKV) || echo $(CKV)); \
		fi; \
		printf '%s\n' \
			'version: 1' \
			'backends:' \
			'  ckg:' \
			'    path: ./$(CKG_DATA)/graph.db' \
			'    timeout_ms: 5000' \
			'  ckv:' \
			"    path: \"$$ckv_path\"" \
			"    binary_path: \"$$ckv_bin\"" \
			"    embed_model: $(CKV_CFG_MODEL)" \
			'    timeout_ms: 3000' \
			'listen:' \
			'  http_addr: "127.0.0.1:8080"' \
			'  mcp_stdio: true' \
			'logging:' \
			'  level: info' \
			'  mode: prod' \
			'  footprint_dir: "./logs/footprint"' \
			'  audit_dir: "./logs/audit"' \
			'sanitize:' \
			'  rules_path: "./policies/sanitization_rules.yaml"' \
			'  default_action: "drop"' \
			'  fail_closed_on_unknown_rule: true' \
			> $(DOGFOOD_CONFIG); \
		echo "wrote $(DOGFOOD_CONFIG) (USE_CKV=$(USE_CKV))"; \
	fi

# dogfood-eval-deps switches the ckv-index prerequisite on USE_CKV.
DOGFOOD_DEPS = build-bins ckg-index dogfood-config
ifeq ($(USE_CKV),1)
DOGFOOD_DEPS += ckv-index
endif

dogfood-eval: $(DOGFOOD_DEPS)
	@if [ "$(USE_CKV)" != "1" ]; then \
		echo "!! USE_CKV=0: every get_for_task scenario will ERROR (the composer"; \
		echo "!! fails closed without a ready ckv leg since the retrieval fix) —"; \
		echo "!! this run only smoke-tests the non-semantic tool wire-up."; \
		echo "!! For recall/MRR numbers run:"; \
		echo "!!   make dogfood-eval USE_CKV=1 CKV_EMBEDDER=ollama CKV_MODEL=bge-m3 CKG=... CKV=..."; \
	fi
	@mkdir -p $$(dirname $(EVAL_REPORT))
	./$(BIN_DIR)/cks-eval \
		--scenarios $(EVAL_SCENARIOS) \
		--verify-anchors .. \
		--cks-mcp ./$(BIN_DIR)/cks-mcp \
		--config $(DOGFOOD_CONFIG) \
		--output $(EVAL_REPORT)
	@echo ""
	@$(MAKE) -s dogfood-eval-summary

dogfood-eval-summary:
	@if [ ! -f $(EVAL_REPORT) ]; then \
		echo "no report at $(EVAL_REPORT) — run 'make dogfood-eval' first"; \
		exit 1; \
	fi
	@python3 -c "import json; \
		d = json.load(open('$(EVAL_REPORT)')); \
		print('='*70); \
		print('cks-eval baseline (%d scenarios, schema v%d)' % (len(d['results']), d['schema_version'])); \
		print('='*70); \
		print('%-34s %-22s %5s %5s %5s %5s' % ('scenario', 'intent', 'P', 'R', 'F1', 'MRR')); \
		print('-'*70); \
		[print('%-34s %-22s %5.2f %5.2f %5.2f %5.2f' % (r['name'], r.get('intent','-'), r['metrics']['file_precision'], r['metrics']['file_recall'], r['metrics']['file_f1'], r['metrics'].get('file_mrr',0))) for r in d['results']]; \
		ok = [r for r in d['results'] if not r.get('error')]; \
		avg_r = sum(r['metrics']['file_recall'] for r in ok)/len(ok) if ok else 0; \
		avg_m = sum(r['metrics'].get('file_mrr',0) for r in ok)/len(ok) if ok else 0; \
		print('-'*70); \
		print('avg recall %.3f / avg MRR %.3f (errored runs excluded, %d scenarios)' % (avg_r, avg_m, len(ok))); \
		print(); \
		print('per-intent rollup:'); \
		[print('  %-22s n=%d R=%.2f MRR=%.2f lat=%dms' % (s['intent'], s['count'], s['avg_recall'], s.get('avg_mrr',0), s['avg_latency_ms'])) for s in d.get('intent_summary', [])]"


