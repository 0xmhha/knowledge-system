# 3-repo 통합 잔여 갭 반영 설계 (2026-07-28, rev.5 — 수렴)

변경이력(rev.4 → rev.5) — 수렴 라운드, 실결함 2건만 교정: §1.3 sed에 `pkg/`
치환 누락(인용 경로 실측 internal/ 15 + pkg/ 4 — 누락 시 자체 수용 기준
위배), §1.0 수도 코드 주석의 전략 혼재(TempDir-밖-이동 서술 제거, retry 단일
전략). 그 외 전 항목 재검증 통과 — 검토·기각한 변경은 문서 말미 §8에 기록.


Status: Tier-3 설계 문서 (구현 전 리뷰용). rev.2 — 자체 비판 리뷰 반영.
rev.3 — 목적 부합성 재검토 반영. rev.4 — rev.3 비판 재검토 반영(신규 갭 G6
발견 포함). 구현 완료 후 결과 기록으로 갱신하거나
`docs/dev/2026-07-27-legacy-ops-go-coverage-audit.md`의 disposition에 병합한다.

변경이력(rev.3 → rev.4) — rev.3 산출물에 대한 비판 재검토 결과:

- **신규 갭 G6 발견 — system dogfood-eval 이중 사망**(§1.3): (a)
  `system/Makefile`의 `CKG_SRC ?= .`가 가리키는 `system/`에는 Go 파일이
  **0개**(실측) — "cks indexing cks itself"가 빈 그래프를 인덱싱; (b) eval
  시나리오의 `expected_citations`가 구 cks 레이아웃 경로
  (`internal/composer/...` — 현재는 `internal/system/composer/...`)를 기대 —
  코퍼스가 있어도 전부 miss. G2와 동근(自기준 좌표가 통합에서 죽음)이나 4-way
  검토·rev.1~3 모두 놓쳤다.
- **§1.2 `--no-cache` → `rm -rf` 관례로 교체**: A3 캐시는 out 디렉터리의
  manifest 기반 diff(`buildpipe.DiffManifest`)이므로 `rm -rf eval/.ckg-data`가
  cold build를 보장하며 `graphExists` 스킵도 함께 해소한다. repo의 기존 관례
  (`ckg-index`/`ckv-index`/`eval-ckv-mirror`/`eval-llm-smoke` 모두 rm -rf
  선행)와도 일치 — 플래그보다 관례를 따른다. `--fail-on-parse-errors`는 유지.
- **§1.2 변경 3의 "한 줄로 정정" 주장 정정**: 패키지 패턴은 `go test`/`go
  vet`에만 유효. vector `fmt`의 `goimports -w .`, vector/system의 bare
  `golangci-lint run`(cwd 기준 → 패키지 0개로 no-op), system `gofmt -s -w .`는
  각각 도구별 인자(디렉터리/경로)가 필요 — 도구별 정정으로 재서술.
- **§6 B 수용 기준 단위 오류 수정**: "include 패턴 수 ≈ 구 473파일"은 단위
  불일치(include=패키지 수, 실측 34개) — build 로그의 `detected files go=N`
  으로 교체.
- **§4.3 activate.sh**: bare `return 1`은 비-source 실행에서 깨짐 — 구 원본의
  `return 1 2>/dev/null || exit 1` 패턴 유지.

변경이력(rev.2 → rev.3) — 목적 부합성 재검토에서 나온 교정 3건:

- **§1.2 `--no-cache`/`--fail-on-parse-errors` 근거 재서술**: rev.2는 이를
  "구 index-project.sh 기본값의 복원"이라 했으나 부정확 — 구 `make eval`에는
  두 플래그가 **없었다**. 이는 승계가 아닌 **의도적 행동 강화**이며, 실제
  근거는 (a) `graphExists` 재실행 스킵(`cmd/graph/build.go:48-52` — 플래그
  없이는 두 번째 `make eval`이 빌드를 건너뛰고 **stale 그래프를 검증**),
  (b) repo가 이미 겪은 stale eval 아티팩트 사건(`graph/Makefile`
  eval-llm-smoke 주석의 `eval/.synthetic-data` 환각 체인). 서술만 교정,
  설계는 유지.
- **§1.2 변경 3 재설계**: graph test 타깃의 "루트 위임"은 구 타깃의 **엔진
  스코프 테스트**라는 목적을 상실(루트=3엔진 전체) → 엔진 패키지 목록으로
  교체. 아울러 동일한 no-op 결함이 **vector/·system/ Makefile에도 존재**함을
  실측 확인(`go list ./...` → "matched no packages")하고 같은 방식으로 포함
  — rev.2는 이 원칙을 graph에만 불완전 적용했다.
- **§6 D 수정**: 훅 스모크 파일의 패키지명 오류(`package graphcmd` —
  `cmd/graph`는 `package main`).

변경이력(rev.1 → rev.2):

- **§1 self-index 방식 교체**: 루트 `.ckgignore` + `--src=..` 안(rev.1)은 두
  결함이 있어 폐기 — (a) `.ckgignore` dir 패턴은 임의 깊이 세그먼트 매칭이라
  (`internal/graph/detect/ckgignore.go:49-69`) 승계한 `build/` 패턴이 실소스
  `internal/vector/build/`를 조용히 제외, (b) self 범위가 "graph 엔진"에서
  "3엔진 전체"로 바뀌어 vector/system 변경이 graph validate 게이트를 트립.
  → **`--files-from-main` 클로저**로 교체(§1.2).
- **G0 신설**: `make test-race`를 CI 게이트로 승격하기 전에 flaky
  `TestServeCmd_PortInUseWithOpen`(WAL 정리 경합, 로컬 2회 연속 재현) 선행
  수정(§1.0).
- **§4 보강**: `cks-health.sh` 삭제의 참조 갱신(문서 3곳+setup-all.sh),
  `run/` gitignore 추가, build-dataset.sh 래퍼에 `--graph-bin/--vector-bin`.
- **§5 보강**: 루트 Makefile `fmt`/`fmt-check`의 stale 제외 패턴
  (`*/web/*/node_modules/*` → `*/node_modules/*`).
- **§6 수정**: D(repo 밖 파일로 훅 검증 — 무효)·E(grep 로직 뒤집힘 + §4.2
  유예 항목과 모순) 재작성.
- **§2 보강**: `eval-gate-graph`에 `fetch-depth: 0`(temporal pass 재현성) +
  실패 시 results 아티팩트 업로드; `needs:` 제거(독립 신호 병렬화).

## 0. 배경 · 범위

3-repo(ckg/ckv/cks) → knowledge-system 통합의 완성도 검토(2026-07-28, 전 파일
byte-diff 4-way 비교) 결과, **Go 코드 레벨 기능 손실은 0건**이며 잔여 갭은 모두
"코드를 둘러싼 자동화 계층"에 있다:

| # | 갭 | 심각도 | 절 |
|---|---|---|---|
| G0 | flaky `TestServeCmd_PortInUseWithOpen` — CI `-race` 게이트의 신뢰 전제 | 선행 | §1.0 |
| G1 | eval 회귀 게이트 2종(vector recall / graph eval-gate)이 CI에서 미배선 | 높음 | §2 |
| G2 | `make -C graph eval`의 self-index가 우연 잔존 Go 파일 **7개**만 인덱싱, 커밋된 baseline(구 473파일)과 완전 비정합 | 높음 | §1 |
| G3 | `.githooks/pre-commit` 미이식 + `graph/Makefile install-hooks`가 훅 전체를 무력화 | 중간 | §3 |
| G4 | 이식된 운영 스크립트가 구 3-repo 사이드카 경로(`../code-knowledge-*`) 참조 / cks.env 체인 잔존 | 중간 | §4 |
| G5 | stale 문서·Makefile 잔재(`graph/CLAUDE.md` CI 서술, 루트 fmt 제외 패턴 등) | 낮음 | §5 |
| G6 | system dogfood-eval 이중 사망: 빈 코퍼스(`CKG_SRC ?= .`) + 시나리오 기대 경로 stale | 중간 | §1.3 |

**비범위(명시적 제외):**

- **뷰어 CI는 복원하지 않는다.** npm build / ESLint / Playwright / npm audit
  어떤 형태로도 CI job을 추가하지 않는다. `tools/viewer`와
  `make -C graph build-full` / `lint-viewer` / `audit`(npm 절반)은 로컬 수동
  실행 전용으로 유지한다.
- **cks.env는 어떤 형태로도 재도입하지 않는다.** env 파일 생성기(shell이든
  Go든)를 새로 만드는 것은 namespace 기반 구조와 중복이다 — §4.1의 매핑이
  근거.

구현 순서: **§1.0(G0) → §1 → §2 → §3 → §4 → §5** (§2의 graph 게이트가 §1의 새
baseline에, §2의 `-race` 게이트가 §1.0에 의존).

---

## 1. G2 — graph self-index eval 정합 회복

### 1.0 G0 (선행) — flaky serve 테스트 수정

`cmd/graph`의 `TestServeCmd_PortInUseWithOpen`이 간헐 실패한다(이 머신에서 2회
연속 재현 후 통과 — flaky):

```
testing.go:1369: TempDir RemoveAll cleanup: unlinkat .../001: directory not empty
```

원인: serve의 port-in-use 에러 경로에서 SQLite 스토어 close 시 WAL
체크포인트가 `graph.db-shm`/`-wal`을 재생성하는 것과 `t.TempDir` 정리
(RemoveAll)의 경합. 테스트 본문·serve.go·server.go는 구 repo와 동일(이식 회귀
아님 — 통합 전부터 잠재하던 경합; 잔존 임시 디렉터리 7/24·7/28자 확인).

수정 방향(구현 시 확정): 테스트 측 수정을 우선한다 — 프로덕션 serve 경로는
구 repo와 byte-동일하므로 건드리지 않는 것이 통합 동등성 유지에 유리.

```go
// cli_extra_test.go — graphDir(t.TempDir)는 그대로 두고, WAL 재생성
// 경합을 흡수하는 t.Cleanup(retry-RemoveAll)만 단다. t.Cleanup은
// TempDir 자동 정리보다 먼저(LIFO) 실행되므로 잔여 -shm/-wal을 여기서
// 흡수하면 TempDir 정리는 빈 디렉터리를 지우게 된다:
t.Cleanup(func() {
    for i := 0; i < 10; i++ {           // WAL 재생성 경합 흡수
        if os.RemoveAll(graphDir) == nil { return }
        time.Sleep(50 * time.Millisecond)
    }
})
```

수용 기준: `go test ./cmd/graph -run TestServeCmd -count=20`이 20회 전부 통과.

### 1.1 문제 (실측)

`graph/Makefile` `eval:` step 1:

```make
./bin/ckg build --src=. --out=eval/.ckg-data --lang=go
```

`graph/` 밑에 남은 Go 파일은 `eval/ckv-mirror/corpus/*.go` 3개 +
`testdata/synthetic/**/*.go` 4개뿐이다(엔진 소스는 `../internal/graph` 등으로
이동). 반면 `graph/eval/baseline/{validate,benchmark,bench-mcp}.json`은 구
repo의 473파일 self-index 산출물 그대로 — **어떤 명령으로도 재현 불가능한
baseline**이다.

단, `cmd/eval-gate`가 실제로 게이트하는 것은 두 가지뿐임에 유의:

- `retrieval.json` — `testdata/synthetic` 고정 코퍼스 기반 → **baseline 유효,
  재생성 불필요**
- `validate.json` — self-index 기반 issue count → **재베이스라인 필요**

(`benchmark.json`/`bench-mcp.json`은 게이트 대상이 아닌 참고 지표이나, 같은
self-index 산출물이므로 함께 재생성한다. **구 baseline 수치와의 직접 비교는
코퍼스가 달라 무의미** — 재베이스라인 시점부터 새 연속성이 시작된다.)

### 1.2 설계: self = graph 엔진의 main-클로저 (`--files-from-main`)

self-index의 원래 취지 = "**graph 엔진이 자기 자신**(실전 규모 Go 코퍼스)을
인덱싱한 결과를 회귀 추적". 통합 후에도 self의 범위는 "graph 엔진"이어야
게이트 의미론이 보존된다(모듈 전체로 넓히면 vector/system 변경이 graph
게이트를 트립). 엔진 코드가 `../cmd/graph`·`../internal/graph`·`../pkg/graph`
등으로 흩어졌으므로, 정확한 엔진 경계는 **main 패키지 클로저**가 정의한다 —
이를 위해 이미 랜딩한 `--files-from-main`(구 `index-project.sh`의 Go 포트,
`internal/graph/filterlist/generate.go`)을 그대로 쓴다.

**변경 1 — `graph/Makefile` `eval:` step 1 재조준:**

```diff
 eval: build-no-viewer
 	@mkdir -p eval/results/latest
 	@echo "=== ckg eval: step 1/5 — self-index (corpus baselining) ==="
-	./bin/ckg build --src=. --out=eval/.ckg-data --lang=go
+	@# self = graph 엔진 = graph 바이너리들의 module-local 패키지 클로저.
+	@# (--src는 module 루트; 클로저가 include 목록을 정의하므로 루트
+	@#  .ckgignore는 불필요하다. 클로저 패키지 디렉터리의 *_test.go 포함.)
+	@# rm -rf: cold build 보장 + graphExists 재실행 스킵 해소 — repo 관례
+	@# (ckg-index / eval-ckv-mirror / eval-llm-smoke 모두 동일).
+	@rm -rf eval/.ckg-data
+	./bin/ckg build --src=.. \
+	    --files-from-main ./cmd/graph,./cmd/graph-mcp,./cmd/eval-gate \
+	    --out=eval/.ckg-data --lang=go --fail-on-parse-errors
```

- `rm -rf` + `--fail-on-parse-errors`: **구 eval에는 없던 의도적 행동 강화**
  (승계 아님). 근거:
  (a) `rm -rf` 없이는 `graphExists` 스킵(`cmd/graph/build.go:48-52`) 때문에
  두 번째 `make eval`이 빌드를 건너뛰고 **stale 그래프를 검증**한다 — repo가
  eval-llm-smoke에서 이미 겪고 문서화한 stale-아티팩트 버그 계열이며, A3
  캐시도 out 디렉터리의 manifest 기반 diff이므로 rm -rf가 cold build까지
  보장한다(`--no-cache` 플래그 불필요 — rev.4 변경이력).
  (b) `--fail-on-parse-errors`는 파스 실패로 부분 그래프가 된 코퍼스가
  "정상"으로 게이트를 통과하는 것을 차단(구 `index-project.sh`의
  FAIL_ON_PARSE=1과 같은 규율을 eval에도 적용). CI 소요 증가는 §7에서 추적.
- include는 클로저 패키지 디렉터리별 `*.go` glob이므로 해당 디렉터리의 테스트
  파일은 포함된다. 클로저 밖(테스트 전용 패키지, 예: `internal/graph/e2e`)은
  제외 — 구 self-index보다 좁아지는 유일한 지점이며, 수용한다(코퍼스 정의가
  "실행 바이너리를 구성하는 코드"로 더 명확해짐).
- **루트 `.ckgignore`는 만들지 않는다**(rev.1 폐기 사유: 변경이력 참조).
  `graph/.ckgignore`(엔진 자체 인덱싱용)는 그대로.

step 2~4(validate/benchmark/bench-mcp)는 `eval/.ckg-data`를 읽으므로 변경 불요.
step 5(retrieval)는 `testdata/synthetic` 고정 — 변경 불요.

**변경 2 — 재베이스라인 (구현 시 1회 실행):**

```sh
make -C graph eval                    # 새 self-index로 4종 JSON 생성
diff -ur graph/eval/baseline graph/eval/results/latest   # 리뷰(특히 validate issue count)
make -C graph eval-baseline-update    # baseline 교체
git add graph/eval/baseline graph/Makefile
```

수용 기준: `make -C graph eval` 직후 `go run ./cmd/eval-gate
-baseline graph/eval/baseline -latest graph/eval/results/latest`가 PASS.

**변경 3 — 엔진 Makefile의 no-op `test`/`test-race`/`lint` 정정 (3엔진 공통):**
`graph/`·`vector/`·`system/` 어디에도 Go 패키지가 없으므로(`go list ./...` →
"matched no packages" 실측) 세 Makefile의 `./...` 기반 타깃은 전부 **조용히
성공하는 no-op**이다 — "테스트 통과"라는 거짓 신호가 목적 훼손의 핵심.

루트 위임(rev.2 안)은 구 타깃의 **엔진 스코프**라는 목적을 잃으므로(루트 =
3엔진 전체), 엔진 패키지 목록으로 교체한다:

```make
# graph/Makefile — 구 `make test`(당시 repo 전체 = 엔진)의 스코프를 통합
# 레이아웃에서 보존. pkg/bm25는 공유 SSOT지만 graph가 소비자이므로 포함.
GRAPH_PKGS = ../cmd/graph/... ../cmd/graph-mcp/... ../cmd/eval-gate/... \
             ../internal/graph/... ../pkg/graph/... ../pkg/bm25/...
test:
	$(GO) test $(GRAPH_PKGS)
test-race:
	$(GO) test -race -coverprofile=coverage.out $(GRAPH_PKGS)
# lint의 `$(GO) vet ./...`도 동일하게 $(GRAPH_PKGS)로 교체.
```

```make
# vector/Makefile — PKG_LIST := ./... 를 다음으로 교체:
PKG_LIST := ../cmd/vector/... ../cmd/vector-mcp/... \
            ../internal/vector/... ../pkg/vector/... ../pkg/bm25/...
# 도구별 추가 정정 (패키지 패턴이 안 통하는 곳 — rev.4):
#   fmt의 `goimports -w .`  → `goimports -w ../cmd/vector ../cmd/vector-mcp \
#                              ../internal/vector ../pkg/vector` (디렉터리 인자)
#   lint의 `golangci-lint run` → `golangci-lint run $(PKG_LIST)` (bare run은
#                              cwd(vector/) 기준이라 패키지 0개 no-op)
```

```make
# system/Makefile — ./... 기반 test/test-short/vet 타깃을 다음 목록으로 교체:
SYSTEM_PKGS = ../cmd/system/... ../cmd/system-mcp/... ../cmd/knowledge-setup/... \
              ../internal/system/... ../internal/setup/... ../pkg/system/...
# (system test는 이미 -race 포함 — 유지.)
# 도구별 추가 정정 (rev.4):
#   fmt/fmt-check의 `gofmt -s -w .` / `gofmt -s -l .` → 디렉터리 인자로:
#     gofmt -s -w ../cmd/system ../cmd/system-mcp ../cmd/knowledge-setup \
#                 ../internal/system ../internal/setup ../pkg/system
#   lint의 `golangci-lint run` → `golangci-lint run $(SYSTEM_PKGS)`
```

(모듈 내 상대경로 패키지 패턴은 `go test`/`go vet`/`golangci-lint run`에서
유효 — graph 목록 34개 패키지 해석 실측 확인. `gofmt`/`goimports`는 패키지
패턴을 모르므로 디렉터리 인자가 필요하다. 전체-모듈 실행이 필요하면 루트
`make test`가 이미 그 역할이다. `internal/setup`은 boundaries 규칙상 독립
계층이지만 knowledge-setup의 구현부이므로 system 스코프에 포함 — 판단 근거
명시.)

### 1.3 G6 — system dogfood-eval 정합 회복 (rev.4 신규)

**문제 (실측).** `system/Makefile`의 dogfood 플로우("cks indexing cks
itself")가 통합 후 이중으로 죽어 있다:

1. **빈 코퍼스**: `CKG_SRC ?= .` / `CKV_SRC ?= .` → `system/` 디렉터리에는 Go
   파일이 **0개**(엔진 소스는 `../internal/system` 등으로 이동). `make -C
   system dogfood-eval`은 빈 그래프를 인덱싱하고 eval을 돌린다 — G2와 동근.
2. **stale 기대 경로**: `system/eval/scenarios/*.yaml`의 `expected_citations`
   가 구 cks 레이아웃(`internal/composer/composer.go`,
   `internal/ckgclient/real.go`, ...)을 기대 — 현재 실경로는
   `internal/system/composer/...` 등. 코퍼스를 고쳐도 전부 miss.
   `system/eval/scenarios-stablenet*`는 대상이 go-stablenet이므로 무관 —
   **cks 자기-인덱싱 시나리오만** 해당.

**설계.**

```diff
 # system/Makefile — dogfood 소스를 system 엔진 클로저로 재조준
-CKG_SRC        ?= .
+CKG_SRC        ?= ..
+# system 엔진 = system-mcp + cmd/system 툴들의 module-local 클로저
+CKG_MAINS      ?= ./cmd/system-mcp,./cmd/system/agent,./cmd/system/eval
-CKV_SRC        ?= .
+CKV_SRC        ?= ..

 ckg-index:
 	@rm -rf $(CKG_DATA)
 	@mkdir -p $(CKG_DATA)
-	$(CKG) build --src $(CKG_SRC) --out $(CKG_DATA) --lang $(CKG_LANG)
+	$(CKG) build --src $(CKG_SRC) --files-from-main $(CKG_MAINS) \
+	    --out $(CKG_DATA) --lang $(CKG_LANG)
```

(`ckv build`에는 files-from-main이 없으므로 `CKV_SRC ?= ..` 모듈 루트 인덱싱
— dogfood는 게이트가 아닌 wire-up 검증이라 코퍼스 과잉은 수용. mock 임베더
기본이라 비용도 무시 가능.)

시나리오 기대 경로는 기계 치환 + 검증:

```sh
# scenarios/*.yaml 한정 (scenarios-stablenet*은 제외). 인용 경로 유형 실측:
# internal/ 15건 + pkg/ 4건 — 두 유형 모두 치환해야 아래 검증이 0줄이 된다
# (구 cks의 internal/* → internal/system/*, pkg/* → pkg/system/* 전량 매핑 확인).
sed -i '' 's|file: internal/|file: internal/system/|' system/eval/scenarios/*.yaml
sed -i '' 's|file: pkg/|file: pkg/system/|'           system/eval/scenarios/*.yaml
# 치환 후 전수 검증: 기대 파일이 실제로 존재하는가
grep -rh 'file: ' system/eval/scenarios/*.yaml | awk '{print $NF}' | sort -u \
  | while read f; do [ -f "$f" ] || echo "MISSING: $f"; done   # 출력 0줄이어야 함
```

수용 기준: `make -C system dogfood-eval`(빌드된 `ckg`/`ckv`를 CKG=/CKV=로
지정)이 시나리오 대다수에서 0이 아닌 recall을 내고, `MISSING:` 검증이 0줄.
CI에는 넣지 않는다(구 repo에서도 수동 플로우 — 승계 범위 유지).

**구현 결과 (2026-07-28)**: 좌표계 복구는 완료·검증 — MISSING 0줄, 코퍼스
315 Go 파일/50 패키지 인덱싱, get_for_task가 composer까지 도달. 단 recall>0
은 **선재 블로커**로 미달성: (a) `USE_CKV=0` 기본 경로는 통합 전 landed된
composer retrieval fix가 ckv-미구성 시 fail-closed라 이미 동작 불가,
(b) `USE_CKV=1` 경로는 `CKV_EMBEDDER` 변수가 ckv build의 백엔드명과 config
`embed_model`(ollama 모델명)을 겸용해 ollama+bge-m3 실경로를 구성할 수 없음
(mock은 cks 쪽 ollama connectivity check에서 404). 수정하려면 dogfood 배관
분리(CKV_MODEL 신설) + 실모델 임베딩(수 시간)이 필요 — 이 설계의 범위(좌표
복구) 밖이므로 **후속 백로그**로 남긴다. 두 블로커 모두 이번 변경으로 새로
생긴 것이 아니라 빈-코퍼스 상태에 가려져 있던 선재 결함이다.

---

## 2. G1 — CI eval 회귀 게이트 복원

### 2.1 vector recall 게이트 (검증 완료)

구 ckv CI의 `eval-gate` job을 신규 경로로 치환한 명령을 **로컬 실행으로 검증
완료**(mock 임베더, 외부 의존성 없음, recall@5 ≥ 0.5 통과, exit 0):

```sh
TMP_OUT=$(mktemp -d)
go run ./cmd/vector build --embedder=mock --src ./vector/testdata/sample --out "$TMP_OUT"
go run ./cmd/vector eval  --embedder=mock --fixture ./vector/testdata/queries.yaml \
    --out "$TMP_OUT" --src ./vector/testdata/sample --min-recall5 0.5 --json
```

### 2.2 `.github/workflows/ci.yml` 변경안 (전체 수도 코드)

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v7
        with: { go-version-file: go.mod }
      - name: Build
        run: make build
      - name: Lint (gofmt drift, vet, engine boundaries)
        run: make lint
      - name: Test (race)                      # ← 변경: 구 ckg CI의 -race 복원
        run: make test-race                    #    (G0 flaky 수정이 선행 조건)

  eval-gate-vector:                            # ← 신규: 구 ckv CI eval-gate 승계
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v7
        with: { go-version-file: go.mod }
      - name: Build index + eval (mock embedder, recall@5 >= 0.5)
        run: |
          TMP_OUT=$(mktemp -d)
          go run ./cmd/vector build --embedder=mock --src ./vector/testdata/sample --out "$TMP_OUT"
          go run ./cmd/vector eval  --embedder=mock --fixture ./vector/testdata/queries.yaml \
            --out "$TMP_OUT" --src ./vector/testdata/sample \
            --min-recall5 0.5 --json
          rm -rf "$TMP_OUT"

  eval-gate-graph:                             # ← 신규: 구 ckg CI eval job 승계
    runs-on: ubuntu-latest                     #    (needs 없음 — test와 독립 신호,
    steps:                                     #     병렬 실행으로 wall-clock 단축)
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0                       # temporal pass(G6)가 git 이력 기반 —
                                               # shallow clone이면 로컬 baseline과
                                               # 코퍼스 이력이 달라진다
      - uses: actions/setup-go@v7
        with: { go-version-file: go.mod }
      - name: make eval (self-index + 4 probes)
        run: make -C graph eval
      - name: eval-gate (retrieval drift <= 0.02, validate issues must not rise)
        run: go run ./cmd/eval-gate -baseline graph/eval/baseline -latest graph/eval/results/latest
      - name: Upload results on failure       # gate FAIL 진단용 — 재현 없이 diff 가능
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: eval-results-latest
          path: graph/eval/results/latest/

  vuln-scan:                                   # 기존 유지 (advisory)
    runs-on: ubuntu-latest
    continue-on-error: true
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v7
        with: { go-version-file: go.mod }
      - name: govulncheck
        run: make vuln
```

뷰어 관련 job(빌드/ESLint/Playwright/npm audit)은 **의도적으로 없다**(§0 비범위).

### 2.3 별도 결정 사항 (이번 구현에서 기본값 유지, 리뷰 시 재론 가능)

- **OS 매트릭스**: 구 ckv(linux amd64/arm64 + macos arm64)·ckg(ubuntu+macos)
  대비 신규는 ubuntu 단일. cgo(sqlite-vec) 특성상 macos 커버리지가 의미는
  있으나, 통합 repo가 의도적으로 축소한 흔적이 있어 **이번에는 ubuntu 단일
  유지**를 기본값으로 한다.
- **vuln-scan 차단 여부**: 구 ckg는 차단형, 신규는 advisory(ci.yml 주석에
  사유 명시 — DB가 repo와 독립적으로 움직임). **advisory 유지**.

---

## 3. G3 — git pre-commit 훅 루트 이관

구 훅은 `git rev-parse --show-toplevel`로 이동 후 `make fmt-check`에 위임하는
구조라 루트 Makefile(`fmt-check` 존재)과 그대로 호환된다. 단 루트
`fmt-check`의 제외 패턴이 stale하므로 §5의 수정이 훅 신뢰성의 전제다.

**변경 1 — `.githooks/pre-commit` 신설** (구 `code-knowledge-graph/.githooks/pre-commit`
원본 그대로 이식 — 주석 포함, `chmod +x` 필수; 요지):

```sh
#!/bin/sh
# Pre-commit hook: gofmt drift check. Opt-in via `make install-hooks`.
set -e
command -v make >/dev/null 2>&1 || { echo "pre-commit: 'make' not on PATH; skipping" >&2; exit 0; }
cd "$(git rev-parse --show-toplevel)"
make fmt-check || {
  echo "pre-commit: gofmt drift detected. Fix: make fmt && git add -u" >&2
  echo "Bypass:   git commit --no-verify" >&2
  exit 1
}
```

**변경 2 — 루트 `Makefile`에 타깃 추가:**

```diff
-.PHONY: all build test test-race vet fmt fmt-check lint tidy clean vuln boundaries build-mcp
+.PHONY: all build test test-race vet fmt fmt-check lint tidy clean vuln boundaries build-mcp install-hooks
 ...
+# install-hooks: opt-in — git이 .githooks/를 보도록 설정 (pre-commit = fmt-check).
+install-hooks:
+	@git config core.hooksPath .githooks
+	@echo "git hooks path set to .githooks (pre-commit will run fmt-check)"
```

**변경 3 — `graph/Makefile:85-92`의 `install-hooks` 타깃 삭제** (현재 존재하지
않는 `graph/.githooks`를 가리켜 **모든 훅을 무력화**하는 결함). `.PHONY`
목록에서도 제거. 필요 시 안내만 남긴다:

```diff
-install-hooks:
-	@git config core.hooksPath .githooks
-	@echo "git hooks path set to .githooks (pre-commit will run fmt-check)"
+# install-hooks는 루트 Makefile로 이동 (make -C .. install-hooks)
```

---

## 4. G4 — cks.env 체인 해체 · 스크립트 정리

### 4.1 판정 근거: 구 cks.env ↔ namespace 기반 신규 구조 매핑

`system/scripts/gen-cks-config.sh`가 생성하던 두 산출물의 대체 경로:

| 구 산출물 / export | 신규 구조 대응 | 근거 위치 |
|---|---|---|
| `cks-stablenet.yaml` (config) | `system-mcp gen-config` (플래그 → 검증된 YAML; sanitize 기본 경로 fail-safe 포함) | `cmd/system-mcp/genconfig.go` |
| `CKS_MCP_URL` | `system-mcp print-mcp-config --config <cfg>` → `.mcp.json` 형태 JSON(HTTP url) 직접 출력 | `cmd/system-mcp/printconfig.go` |
| `CKS_MCP_BIN` | 동일 명령 stdio 모드(`command`+`args`); 멀티 인스턴스는 `system-mcp daemon` + `instances.yaml` | `internal/system/daemon/registry.go` |
| `GO_STABLENET_ROOT` | config `source_root`; 비면 graph manifest에서 자동 유도(audit #14) | `gen-config` / config.Load |
| `CKV_OLLAMA_ENDPOINT` | config `ollama_url` / registry `ollama_url` | 〃 |
| LAN IP 자동감지 | `gen-config --lan` (`internal/system/netutil`) | 〃 |
| dataset↔source_root 일치 검증(Python 블록) | `internal/setup/verify.go` + manifest 유도로 대체(설계상 supersede) | audit #5 |
| 툴 네임스페이스("cks.") | `pkg/mcp.Root`: explicit > `KNOWLEDGE_MCP_NAMESPACE` > ldflags `BuildRoot` > 기본값 | `pkg/mcp/namespace.go` |

→ **cks.env의 모든 정보가 신규 구조에서 표현 가능. env 파일 매체 자체를 폐기
한다.** `2026-07-27-legacy-ops-go-coverage-audit.md`의 "cks.env half still
lacks a Go equivalent" 판정은 `print-mcp-config` 관점에서 **해소된 것으로
재판정**하고 audit 문서를 갱신한다(§5).

### 4.2 스크립트 처분표

| 스크립트 | 처분 | 근거 / 파급 |
|---|---|---|
| `system/scripts/serve-cks-http.sh` | **삭제** | `system-mcp daemon` 완전 대체 (audit "fully superseded") |
| `system/scripts/cks-mcpd.sh` | **삭제** | 〃 |
| `system/scripts/reindex-dataset.sh` | **삭제** | `knowledge-setup --version/--rollback` + `ops.reindex` 완전 대체 |
| `system/scripts/gen-cks-config.sh` | **삭제** | §4.1 매핑 완성 — config 절반=`gen-config`, env 절반=`print-mcp-config` |
| `system/scripts/cks-health.sh` | **삭제 + 참조 갱신** | `cks.ops.health` thin client. **참조 4곳 갱신 필수**: `setup-all.sh`, `system/docs/SETUP.md`, `system/docs/go-stablenet-dataset-build-manual.md`, `system/dataset-toolkit/docs/multi-project-setup.md` |
| `system/activate.sh` | **수정**(§4.3) 후 coding-agent 플러그인 이관 후보 | cks.env 소비자 → print-mcp-config 소비자로 전환 |
| `system/scripts/apply-cc-settings.sh` | **수정**(§4.3) 후 〃 | 〃 |
| `system/scripts/setup-all.sh` | **경로 수정**(§4.4) | 유일하게 Go 대체 없는 원클릭 프리렉 설치 |
| `system/scripts/coding-agent.sh`, `enable-autopilot.sh` | 유지 | 플러그인 글루 (엔진 스코프 밖) |
| `projects/stablenet/scripts/build-dataset.sh` | **thin 래퍼로 교체**(§4.5) | `knowledge-setup`이 Go 대체 경로 |
| `projects/stablenet/scripts/gen-filelist.sh` | **삭제 + 문서 참조 갱신** | `ckg build --files-from-main`이 대체 (audit #1 LANDED); 위 문서 3곳에 언급 잔존 여부 확인 |
| `system/dataset-toolkit/scripts/*` | 유지 — **경로 수정은 명시적 후속**(이번 범위 밖, §6-E 검사 대상에서도 제외) | 특수 데이터셋 준비, audit "kept as-is" |
| `vector/scripts/build-vector-stablenet.sh` | 유지 — **경로 수정은 명시적 후속**(〃) | core는 `vector build`가 커버, DRY_RUN 등 편의만 잔존 |

### 4.3 `activate.sh` / `apply-cc-settings.sh` 수정 수도 코드

원칙: **cks.env를 source하지 않는다.** 단일 입력 = config YAML 경로. env 조립은
`print-mcp-config` 출력(JSON)에서 파생한다.

```sh
# activate.sh (요지 — cks.env 블록을 다음으로 교체)
_CKS_HERE="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
KS_ROOT="$(cd "$_CKS_HERE/.." && pwd)"                    # knowledge-system 루트
SYSTEM_MCP="$KS_ROOT/bin/system-mcp"
CONFIG="${KS_CONFIG:-$KS_ROOT/run/cks.yaml}"              # gen-config 산출물(§4.3.1)

[ -x "$SYSTEM_MCP" ] || { echo "activate: system-mcp not built — make build-mcp"; return 1 2>/dev/null || exit 1; }
[ -f "$CONFIG" ] || {
  echo "activate: config missing — 생성 예:"
  echo "  $SYSTEM_MCP gen-config --out $CONFIG --dataset-dir <dataset> [--lan]"
  return 1 2>/dev/null || exit 1
}

# print-mcp-config의 JSON에서 플러그인 .mcp.json이 요구하는 env를 파생.
_MCP_JSON="$("$SYSTEM_MCP" print-mcp-config --config "$CONFIG")"
export CKS_MCP_URL="$(printf '%s' "$_MCP_JSON" | jq -r '.mcpServers[].url // empty')"
export CKS_MCP_BIN="$SYSTEM_MCP"
# GO_STABLENET_ROOT / CKV_OLLAMA_ENDPOINT는 config가 단일 진실원 —
# 필요 소비자가 남아있는 동안만 config에서 읽어 export(yq); 소비자 제거가
# 최종 목표(§7).
```

§4.3.1 — **`.gitignore`에 `run/` 추가 필수**: 현재 루트 `.gitignore`에 `run/`
항목이 없다(확인됨). config는 머신-로컬(절대경로 포함)이므로 커밋되면 안 된다.

명명 주의: 변수명은 `KS_CONFIG`를 쓴다 — 구 `CKS_CONFIG`는 폐기되어
apply-cc-settings.sh가 stale 키로 **제거**하는 대상이므로 재사용하면 혼동.

```sh
# apply-cc-settings.sh (요지)
# source activate.sh 로 env를 얻는 구조는 유지하되, activate.sh가 이미
# print-mcp-config 기반이므로 이 파일의 변경은 "CKS_* 키의 출처가 바뀐다"는
# 주석 갱신 + stale 키 정리뿐. merge 로직(Python 블록)은 그대로.
```

주의(중복구조 방지): `print-mcp-config`에 "env 파일 출력 모드"를 추가하는 것은
**금지** — env 파생은 소비자(플러그인 글루) 측 책임으로 남긴다. 엔진 repo는
config YAML + JSON 두 형식만 안다.

### 4.4 `setup-all.sh` 경로 수정 수도 코드

```sh
# 변경 전(사망 경로): CKG_REPO=../code-knowledge-graph, CKV_REPO=../code-knowledge-vector
# 변경 후: in-repo 단일 모듈 빌드
KS_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
( cd "$KS_ROOT" &&
  go build -o bin/ckg  ./cmd/graph &&
  go build -o bin/ckv  ./cmd/vector &&
  go build -o bin/knowledge-setup ./cmd/knowledge-setup &&
  make build-mcp )                              # graph-mcp / vector-mcp / system-mcp
# 이후 단계는 gen-cks-config.sh 호출 대신:
#   bin/system-mcp gen-config --out run/cks.yaml --dataset-dir "$DATASET" ...
#   bin/system-mcp print-mcp-config --config run/cks.yaml   # 등록 안내 출력
# cks-health.sh 호출부는 삭제하고 안내로 대체:
#   "health: cks.ops.health MCP 툴 또는 GET /healthz"
```

### 4.5 `projects/stablenet/scripts/build-dataset.sh` 교체 수도 코드

```sh
#!/usr/bin/env bash
# build-dataset.sh — knowledge-setup 위임 래퍼 (구 3-repo 오케스트레이션 폐기).
set -euo pipefail
KS_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
: "${GSN_SRC:?set GSN_SRC=/path/to/go-stablenet}"
: "${OUT:?set OUT=/path/to/knowledge-data/<name>}"
# knowledge-setup의 --graph-bin/--vector-bin 기본값은 "PATH의 ckg/ckv"이므로
# in-repo 빌드 산출물을 명시적으로 전달한다(없으면 빌드 안내 후 종료).
for b in knowledge-setup ckg ckv; do
  [ -x "$KS_ROOT/bin/$b" ] || { echo "bin/$b not built — run system/scripts/setup-all.sh or go build"; exit 1; }
done
exec "$KS_ROOT/bin/knowledge-setup" \
  --config "$KS_ROOT/projects/stablenet/setup.yaml" \
  --src "$GSN_SRC" --out "$OUT" \
  --graph-bin "$KS_ROOT/bin/ckg" --vector-bin "$KS_ROOT/bin/ckv" "$@"
```

(래퍼는 존재 검사 + 옵션 통과만 하고 로직을 갖지 않는다 — 로직이 필요해지면
Go(`internal/setup`)에 넣는 것이 규율.)

---

## 5. G5 — 문서·Makefile 정합

| 대상 | 수정 |
|---|---|
| `graph/CLAUDE.md` | "CI runs go vet/-race/make lint/make audit/eval gate" 서술을 §2.2의 실제 CI로 교체. `make install-hooks` 언급을 루트 타깃(`make -C .. install-hooks`)으로 교체 |
| `docs/dev/2026-07-27-legacy-ops-go-coverage-audit.md` | disposition 갱신: gen-cks-config.sh의 "cks.env half" 잔여 판정 → print-mcp-config로 해소(§4.1); 삭제 스크립트 목록 반영 |
| **루트 `Makefile` `fmt`/`fmt-check`** | 제외 패턴 `*/web/*/node_modules/*` → `*/node_modules/*`. 뷰어가 `web/`→`tools/`로 이동해 현 패턴은 `tools/viewer/node_modules`를 못 잡는다 — npm install 후 vendored `.go`가 생기면(구 repo에서 실제 발생 사례로 명시된 배제 사유) `fmt-check`(=CI lint + §3 훅) 오염. **§3 훅 신뢰성의 전제** |
| `graph/Makefile` `fmt`/`fmt-check` | `-not -path './web/viewer-next/node_modules/*'` — 경로 소멸, 제외절 삭제(무해하나 stale) |
| `graph/Makefile` `eval:` 주석 | "Self-indexes this repo (Go only)" → "Self-indexes the graph engine (main-package closure)" 로 갱신 |
| `system/docs/SETUP.md` 외 2개 문서 | `cks-health.sh`/`gen-filelist.sh` 참조를 신규 경로(`cks.ops.health`·`/healthz`, `--files-from-main`)로 교체(§4.2 파급) |

---

## 6. 수용 기준 · 검증 계획

구현 완료 시 아래가 모두 성립해야 한다 (로컬 재현 명령 포함):

```sh
# G0. flaky 수정 (§1.0)
go test ./cmd/graph -run TestServeCmd -count=20          # 20/20 통과

# A. 루트 게이트 일괄
make build && make lint && make test-race                # 전부 exit 0

# B. graph eval 정합 (§1)
make -C graph eval
go run ./cmd/eval-gate -baseline graph/eval/baseline -latest graph/eval/results/latest
#   → "eval-gate: PASS". self-index 빌드 로그의 `detected files go=N`이
#     엔진 규모(수백 파일)인지 확인 — 7개(현 상태)면 재조준 실패.
#     (include 패턴 수(~34)는 패키지 수라 파일 수와 비교 불가 — rev.4)

# C. vector recall 게이트 (§2.1 — 이미 사전 검증 완료)
#   §2.1의 3줄 그대로 → exit 0

# D. 훅 (§3) — repo 안의 실제 파일로 검증
make install-hooks
printf 'package main\nfunc  bad () {}\n' > cmd/graph/hook_smoke_test.go  # 고의 미포맷 (cmd/graph는 package main)
git add cmd/graph/hook_smoke_test.go
git commit -m tmp && echo "FAIL: hook did not block" || echo "OK: hook blocked"
git restore --staged cmd/graph/hook_smoke_test.go && git checkout -- cmd/graph/hook_smoke_test.go 2>/dev/null; rm -f cmd/graph/hook_smoke_test.go

# E. 스크립트 (§4) — 검사 범위는 §4.2 처분표의 "이번 범위" 항목과 일치시킨다
#    (dataset-toolkit·vector/scripts는 명시적 후속이므로 제외)
bash -n system/scripts/setup-all.sh projects/stablenet/scripts/build-dataset.sh system/activate.sh
! grep -rn 'code-knowledge-\(graph\|vector\|system\)' \
    system/scripts system/activate.sh projects/stablenet/scripts \
  && echo "OK: no stale sibling-repo path in this round's scope"
#   activate.sh: config 있는 환경에서 source 후 $CKS_MCP_URL 이 http://…/mcp 형태인지
grep -qx 'run/' .gitignore                               # §4.3.1

# F. CI (push 후)
#   build-and-test / eval-gate-vector / eval-gate-graph 3 job green, 뷰어 job 부재

# G6. system dogfood (§1.3) — CI 밖, 수동 1회
#   시나리오 기대 경로 MISSING 검증 0줄 + dogfood-eval recall > 0
```

## 7. 리스크 · 열린 결정

- **재베이스라인의 일회성 리뷰 부담(§1.2 변경 2)**: 새 self-index의 validate
  issue count가 0이 아닐 수 있다(클로저 코퍼스는 구 473파일과 다름). diff
  리뷰에서 issue가 발견되면 "코드 수정으로 0 수렴"이 원칙(eval-gate 주석)이나,
  최초 baseline은 현상 스냅샷으로 커밋하고 수렴은 후속 작업으로 분리한다.
- **`make -C graph eval`의 CI 소요 시간**: `--no-cache` 강제로 CI에선 매번 콜드
  빌드(구 기준 ~2분, 클로저 규모 동급이므로 유사 예상 — 구현 시 실측). 5분
  초과 시 `eval-gate-graph`를 main push 전용으로 낮추는 옵션을 재론.
  `fetch-depth: 0`도 대형 이력(1,033커밋 그래프트)에서 checkout 시간을 다소
  늘린다 — 실측 후 temporal 재현성과 트레이드오프 판단.
- **`--files-from-main` 클로저의 커버리지**: 테스트 전용 패키지(예:
  `internal/graph/e2e`)는 self-index 코퍼스에서 빠진다. validate가 이들
  패키지의 스키마 이슈를 못 보는 대가로 게이트 의미론(엔진 스코프)을 얻는
  선택 — 수용. 커버리지가 아쉬우면 mains에 벤치 바이너리를 추가하는 것으로
  확장 가능.
- **OS 매트릭스 / vuln 차단화**: §2.3 — 이번 범위에서는 현상 유지.
- **`GO_STABLENET_ROOT`/`CKV_OLLAMA_ENDPOINT`의 최종 소비자**: coding-agent
  플러그인 `.mcp.json`이 아직 `${VAR}` 플레이스홀더로 요구하는 동안은
  activate.sh가 config에서 파생·export(§4.3). 플러그인 측이 config 직접 소비로
  바뀌면 export 자체를 제거 — 그 시점에 activate.sh/apply-cc-settings.sh를
  coding-agent repo로 이관한다.

---

## 8. 검토했으나 기각한 변경 (재론 방지 기록)

수렴 라운드(rev.5)에서 검토 후 **의도적으로 바꾸지 않기로** 한 항목. 다음
리뷰에서 같은 지적이 나오면 여기 근거를 먼저 확인할 것.

| 항목 | 기각 근거 |
|---|---|
| `graph/Makefile` `fmt`/`fmt-check`를 엔진 디렉터리로 재조준 | 검사 경로는 루트 `make fmt-check`(CI lint + §3 훅)가 이미 전 모듈을 게이트 — 엔진-로컬 fmt는 편의 타깃일 뿐이라 no-op이어도 거짓 신호를 만들지 않는다(test no-op과 달리 안전망 구멍이 아님). §5의 stale 제외절 정리로 충분 |
| `activate.sh`의 jq 의존 추가 | 신규 의존이지만 플러그인 글루 스코프(엔진 밖)이고, 대안(수제 JSON 파싱)이 더 나쁨 — 수용 |
| `eval-gate-graph`의 `needs: build-and-test` 복원(구 CI 형태) | 두 job은 독립 신호 — 병렬이 wall-clock 우위. 실패 시 러너 낭비는 미미 |
| `system/Makefile` dogfood를 CI에 편입 | 구 repo에서도 수동 플로우 — 승계 범위 유지(§1.3). CI 편입은 별도 결정 |
| GRAPH_PKGS에서 `pkg/bm25` 제외(vector와 중복 테스트) | 공유 SSOT의 소비자 양쪽에서 도는 것이 의도 — 중복 실행 비용은 초 단위 |
| G0를 프로덕션(serve의 close 순서) 측에서 수정 | serve 경로는 구 repo와 byte-동일 — 통합 동등성 보존이 우선, 경합 흡수는 테스트 책임으로 충분 |

검증 완료로 확인된 사항(변경 불요): GRAPH_PKGS 상대 패턴 34개 패키지 해석
정상, `cmd/system-mcp` 클로저가 `internal/system` 19개 패키지(시나리오 인용
대상 포함) 포섭, t.Cleanup의 LIFO 실행 순서(TempDir 자동 정리보다 먼저), 구
ckg CI가 setup-node 없이 `make eval`을 돌렸으므로 ts 파서는 CI 자립적.
