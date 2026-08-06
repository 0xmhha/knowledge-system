.PHONY: all build test test-race vet fmt fmt-check lint tidy clean vuln boundaries build-mcp install-hooks build-dataset-bins sync-domain-artifacts check-domain-artifacts

GO ?= go

all: build

# ---------------------------------------------------------------------------
# Whole-module targets. graph/, vector/, system/ share one Go module
# (github.com/0xmhha/knowledge-system), so these operate across all three.
# ---------------------------------------------------------------------------

build:
	$(GO) build ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	@find . -name '*.go' -not -path '*/node_modules/*' -print0 | xargs -0 gofmt -w

fmt-check:
	@drift=$$(find . -name '*.go' -not -path '*/node_modules/*' -print0 | xargs -0 gofmt -l); \
	if [ -n "$$drift" ]; then \
	    echo "gofmt drift detected — run 'make fmt' before commit:"; \
	    echo "$$drift"; \
	    exit 1; \
	fi

lint: fmt-check vet boundaries

# boundaries: cross-engine isolation is convention now that internals live at
# internal/<engine> (see scripts/check-boundaries.sh for the rules).
boundaries:
	@./scripts/check-boundaries.sh

tidy:
	$(GO) mod tidy

# install-hooks: opt-in helper that points git at .githooks/ so the
# pre-commit script (gofmt drift gate, delegates to `make fmt-check`)
# runs locally. Idempotent — re-running is safe. Not auto-installed:
# hooks are per-clone config and some operators use IDE-side commit
# flows that already cover formatting.
install-hooks:
	@git config core.hooksPath .githooks
	@echo "git hooks path set to .githooks (pre-commit will run fmt-check)"

# build-mcp: build the three MCP server binaries. Deployments can stamp a
# tool-namespace root at build time, e.g.:
#   make build-mcp NAMESPACE=stablenet_knowledge
NAMESPACE ?=
NS_LDFLAGS = $(if $(NAMESPACE),-ldflags "-X github.com/0xmhha/knowledge-system/pkg/mcp.BuildRoot=$(NAMESPACE)",)
build-mcp:
	$(GO) build $(NS_LDFLAGS) -o bin/graph-mcp ./cmd/graph-mcp
	$(GO) build $(NS_LDFLAGS) -o bin/vector-mcp ./cmd/vector-mcp
	$(GO) build $(NS_LDFLAGS) -o bin/cks ./cmd/cks

# vuln: scan for known vulnerabilities reachable from this module's code.
# Note: the Go vulnerability DB covers Go-level advisories; CVEs in C code
# embedded by cgo dependencies (e.g. bundled sqlite amalgamations) are not
# visible to this scan and need separate tracking.
vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

clean:
	rm -rf graph/bin vector/bin system/bin

# ---------------------------------------------------------------------------
# Specialized targets (eval, viewer, model-fetch, dataset builds, ...) carry
# engine-specific assumptions (Node/Next.js viewer, ONNX model downloads,
# corpus paths, Ollama) and stay in the per-engine Makefiles. Run them via:
#
#   make -C graph  <target>
#   make -C vector <target>
#   make -C system <target>
#
# Paths inside those Makefiles are relative to their own directory and remain
# valid after the consolidation.
# ---------------------------------------------------------------------------

# build-dataset-bins: the three engine binaries, one per engine. cks hosts
# the dataset orchestrator (`cks setup`) and its whole toolchain; ckg/ckv
# are resolved beside the cks binary at run time, so building all three
# into bin/ is the complete dataset story.
build-dataset-bins:
	@mkdir -p bin
	$(GO) build -o bin/ckg ./cmd/graph
	$(GO) build -o bin/ckv ./cmd/vector
	$(GO) build $(NS_LDFLAGS) -o bin/cks ./cmd/cks

# sync-domain-artifacts: refresh the committed renderings of the domain
# entries. The dataset build derives these per run and uses the fresh copy;
# the committed ones exist so governance and vocabulary changes show up as a
# reviewable diff when an entry changes. Run after editing entries/.
DOMAIN_PACK ?= projects/stablenet
sync-domain-artifacts: build-dataset-bins
	./bin/cks domain sync --project $(DOMAIN_PACK)/domain-knowledge \
	    --ckg-out $(DOMAIN_PACK)/policies/graph.yaml \
	    --ckv-out /dev/null
	./bin/cks domain glossary-gen --project $(DOMAIN_PACK)/domain-knowledge \
	    --out $(DOMAIN_PACK)/domain-knowledge/glossary.yaml

# check-domain-artifacts: fail when the committed renderings no longer match
# the entries they come from. Both files went months out of date because
# nothing regenerated them and nothing noticed.
check-domain-artifacts: sync-domain-artifacts
	@if ! git diff --quiet -- $(DOMAIN_PACK)/policies/graph.yaml \
	    $(DOMAIN_PACK)/domain-knowledge/glossary.yaml; then \
	    echo "domain artifacts are stale — run 'make sync-domain-artifacts' and commit:" >&2; \
	    git --no-pager diff --stat -- $(DOMAIN_PACK)/policies/graph.yaml \
	        $(DOMAIN_PACK)/domain-knowledge/glossary.yaml >&2; \
	    exit 1; \
	fi
	@echo "domain artifacts match their entries"
