.PHONY: all build test test-race vet fmt fmt-check lint tidy clean vuln

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
	@find . -name '*.go' -not -path '*/web/*/node_modules/*' -print0 | xargs -0 gofmt -w

fmt-check:
	@drift=$$(find . -name '*.go' -not -path '*/web/*/node_modules/*' -print0 | xargs -0 gofmt -l); \
	if [ -n "$$drift" ]; then \
	    echo "gofmt drift detected — run 'make fmt' before commit:"; \
	    echo "$$drift"; \
	    exit 1; \
	fi

lint: fmt-check vet

tidy:
	$(GO) mod tidy

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
