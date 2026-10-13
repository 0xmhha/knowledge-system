# CKV Embedder Integration Guide

> **대상**: ckv를 *consumer*로 사용하려는 코드 — cks 같은 downstream Go 패키지,
> 별도 CLI 도구, 통합 테스트.
> **다른 문서**: `d1-installation-guide.md` 는 ckv 자체를 빌드·실행하려는 *개발자*
> 시점. 이 문서는 *integrator* 시점.
> **기준**: 현재 cmd/vector/embedder.go, pkg/vector/ckv, pkg/vector/embed/ollama.

ckv는 두 가지 통합 형태를 제공한다:

| 형태 | 어떻게 | 언제 |
|---|---|---|
| **in-process (pkg/vector/ckv)** | Go import로 `Engine`을 직접 생성 | 같은 process 안에서 검색 — cks composer 같은 *직접 통합*. 추천. |
| **subprocess (`ckv mcp`)** | binary 를 stdio MCP server로 spawn | 언어가 다르거나 process 격리가 필요한 케이스 |

이번 가이드는 두 형태 모두 다루되 **in-process를 우선**으로 설명한다.

---

## 1. Quickstart — mock embedder

> *Why mock 먼저*: 시스템 의존성 (ONNX runtime, tokenizers) 없이 통합 자체를
> 검증할 수 있다. 의미 있는 recall은 안 나오지만 *흐름* 검증에는 충분하다.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/0xmhha/knowledge-system/pkg/vector/ckv"
)

func main() {
    // 1) index를 미리 build: `ckv build --src=. --out=.ckv-data --embedder=mock`
    //    (외부 consumer는 내부 패키지를 import하지 말고 CLI 또는 공개 API를 사용)

    // 2) Open + query
    engine, err := ckv.Open(".ckv-data", ckv.OpenOptions{
        Embedder: ckv.MockEmbedder(),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer func() { _ = engine.Close() }()

    // (recommended) prepay the embedder's cold-start cost so the
    // first user-facing query doesn't see a multi-second spike.
    // No-op for mock; meaningful for bgeonnx (ONNX session + CoreML compile).
    if err := engine.Warmup(context.Background()); err != nil {
        log.Printf("ckv warmup failed: %v (continuing — first query may be slow)", err)
    }

    resp, err := engine.SemanticSearch(
        context.Background(),
        "tokenizer encoding pipeline",
        ckv.SearchOptions{K: 5, Threshold: -1},
    )
    if err != nil {
        log.Fatal(err)
    }
    for _, hit := range resp.Hits {
        fmt.Printf("%s:%d-%d  score=%.3f  %s\n",
            hit.Citation.File, hit.Citation.StartLine, hit.Citation.EndLine,
            hit.Score.Normalized, hit.Symbol)
    }
}
```

핵심 호출 3개:
- `ckv.Open(path, OpenOptions{Embedder})` — manifest 검증 + store open
- `engine.SemanticSearch(ctx, intent, SearchOptions)` — embed → retrieve → snippet
- `engine.Close()` — idempotent

`SearchOptions` 의 zero value는 `K=10, Threshold=0.4, BudgetTokens=4000` 으로
복원된다. `Threshold=-1` 은 score gate 비활성.

---

## 2. Ollama consumer integration

The ordinary CLI defaults to Ollama/BGE-M3. The public Go adapter is [pkg/vector/embed/ollama](../../pkg/vector/embed/ollama); Open pins the model tag/digest and probes its dimension. Query-time identity must match the built index. Explicitly select the same model, dimension and prefix policy for build and query. A mock index cannot be meaningfully queried with BGE-M3.

```go
adapter, err := ollama.Open(ollama.Options{ModelName: "bge-m3"})
if err != nil { return err }
engine, err := ckv.Open(indexPath, ckv.OpenOptions{Embedder: adapter})
if err != nil { return err }
defer func() { _ = engine.Close() }()
```

Import `github.com/0xmhha/knowledge-system/pkg/vector/embed/ollama` and the public ckv package shown above. This fragment assumes an error-returning function and a prebuilt indexPath. Ollama manages the model process; callers must keep the endpoint available. A configured real backend failure must not be treated as a successful mock fallback.

## 3. Optional ONNX and CoreML

`bgeonnx` is an internal package available to this module. External modules cannot bypass Go internal visibility by vendoring it. There is no public native factory; use the CLI subprocess or implement the public [Embedder interface](../../pkg/vector/types/embed.go) with a supported adapter.

ONNX uses `-tags bgeonnx`, matching native libraries and model files. See [native preparation](./d1-installation-guide.md). The direct `internal/vector/embed/coreml` adapter uses darwin/tokenizers gates but is not wired into ordinary CLI embedder selection. ONNX's optional CoreML execution provider is a different path.

## 4. Subprocess MCP

```bash
ckv mcp --out=.ckv-data --embedder=mock
```

For an Ollama index, use `--embedder=ollama --model-name=bge-m3` instead. MCP runs over stdio; [the 19-tool reference](./mcp-tools.md) defines names and arguments. Response wrapper schema is currently 1.1; storage schema is a separate contract. Consumers must reject unsupported major schemas rather than guess that an incompatible parser is safe. In-process callers avoid subprocess lifecycle overhead; no old machine timing is a current speed guarantee.

## 5. Runtime settings and verification

Ollama uses CKV_OLLAMA_ENDPOINT when Endpoint is unset, with a default per-request timeout of 60 seconds. Optional ONNX environment settings are defined in [session_impl.go](../../internal/vector/embed/bgeonnx/session_impl.go); CoreML tuning and ORT thread choices need measurements on the target model and machine. The memory guard is defined in [build/memory.go](../../internal/vector/build/memory.go); it does not establish large-corpus throughput acceptance.

Tests should distinguish package/CLI wiring from semantic quality and native compatibility. Mock consumer smoke tests prove neither real-model recall nor ONNX/CoreML operation. [Evaluation status](../spec-driven/EVALUATION-STATUS.md) records frozen observations and remaining limits.
