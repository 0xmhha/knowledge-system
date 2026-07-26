# 세션 핸드오프 — upstream 통합·동등성 검증 (2026-07-24)

다른 머신/세션에서 전체 컨텍스트를 이어받기 위한 기록. 이 폴더의
`generalization-upstream-structure-review-2026-07-23.md`(이하 **리뷰 문서**)가
설계·결정의 원본이고, 본 문서는 실행 상태와 재개 절차를 담는다.

---

## 1. 배경·목표 (요약 — 상세는 리뷰 문서 §1~§7)

- **upstream** = 이 리포(`github.com/0xmhha/knowledge-system`): 구 3-repo
  (`code-knowledge-graph`(ckg) · `code-knowledge-vector`(ckv) ·
  `code-knowledge-system`(cks))를 이력 보존 통합한 **일반화·확장** 프로젝트.
- **downstream** = `stablenet-knowledge-mcp`(stable-net org): stablenet
  **특화** 배포 리포. 통합 과정에서 ckg/ckv/cks 네이밍을 전면 치환해
  가독성이 훼손되어 **홀드** 중.
- **운영 모델 (§8.8 #4 확정)**: 두 리포는 목적이 다르므로 각각 유지.
  upstream 정리가 완료되고 **3-repo 버전과 동등한 동작이 검증되면**,
  upstream → downstream **코드 이식**으로 동기화한다. 이식 절차 =
  `docs/downstream-sync.md` (finalized 2026-07-27 — §4 UPDATE 참조).
- 이식을 기계적으로 만드는 3장치: **L1 주입**(툴 네임스페이스/바이너리명
  — `pkg/mcp`, `make build-mcp NAMESPACE=...`), **projects/ 팩**(프로젝트
  데이터 분리, 경계는 `scripts/check-boundaries.sh`가 강제), **동등성 검증
  도구**(digest·테이블 해시·eval 픽스처).

## 2. 완료된 작업 (P0~P5 + 동등성, 총 28커밋, 전부 로컬 — push는 사용자 직접)

- P0: 3-repo 이력 그래프트(1,033커밋) + 단일 모듈 + bm25 SSOT
- P1: cmd/·internal/ 루트 재배치(Go internal 규칙 대응 — internal/<engine>),
  docs 이동, 경계 검사, 루트 CI(미push라 미검증)
- P2: graph manifest 엔진 정체성(dual-write), system wire 중립화
  (`graph`/`vector` 키·`AlignmentSources`), TRAVERSAL-DEPTH 문서
- P3: `pkg/mcp` 네임스페이스 주입(explicit>env>ldflags>기본값),
  `cmd/graph-mcp`·`cmd/vector-mcp` 신설 — stablenet 툴명이 코드 diff 0으로 재현
- P4: code/enrich digest 분리, `internal/setup`(typed plan+spawn Runner+
  비동기 Jobs), `cmd/knowledge-setup --progress=json --config`,
  `ops.setup`/`ops.setup_status`
- P5: `projects/stablenet` 팩(정책·domain-knowledge·eval·scripts·validate)
- 의존성: govulncheck 클린(`make vuln`), x/text CVE 해소
- **동등성 검증(§8.16) — 이식 전제 조건 충족**:
  - graph: digest 3자 일치(`65d74ed74f57a940…`, 참조 데이터셋↔구 도구↔신 도구)
    + 전 테이블 정렬 덤프 해시 일치(rowid·additive manifest 키 제외)
  - 기능: `eval-retrieval` 8픽스처 출력 JSON 심층 일치
  - vector: manifest 전 항목·**chunks 테이블 전 컬럼 해시 일치**(19,343행,
    canonical_id 16,085), 임베딩 98.95% 비트 일치(불일치 1.05%는 ollama
    동시 배칭 지터 — 입력 동일성은 chunks 해시로 증명)
  - topic_tree 결정화(`db6c291`): map 순회 누출 3곳 수정, 245K 코퍼스
    이중 빌드 해시 일치 증명

## 3. 백그라운드 작업 — 상태·재실행·체크 절차

### ③ D — cks-refactor-1 실모델 데이터셋 (유일하게 실행 중)

- **무엇**: `knowledge-setup`으로 graph(정책 enrichment 포함)+vector(ollama
  bge-m3) 데이터셋 생성. 산출: `~/Work/github/knowledge-data/cks-refactor-1/`
- **왜**: 사용자 요청 실사용 DB + setup 파이프라인/팩 config의 실전 검증 +
  C(fused 스모크)의 입력
- **실행 형태**: nohup 분리 프로세스(세션 죽어도 계속). 로그:
  `~/Work/github/knowledge-data/cks-refactor-1-build.log` (JSON 이벤트/라인)
- **재실행(죽었을 때)**:
  ```sh
  K=~/Work/github/knowledge-system
  rm -rf ~/Work/github/knowledge-data/cks-refactor-1
  nohup $K/bin/knowledge-setup --config $K/projects/stablenet/setup.yaml \
    --src ~/Work/github/cks-seminar/test/cks-refactor-1 \
    --out ~/Work/github/knowledge-data/cks-refactor-1 \
    --graph-bin $K/bin/graph-fixed --vector-bin $K/bin/vector-new \
    --progress=json > ~/Work/github/knowledge-data/cks-refactor-1-build.log 2>&1 &
  ```
  (바이너리 없으면: `go build -o bin/graph-fixed ./cmd/graph`,
  `go build -o bin/vector-new ./cmd/vector`, `go build -o bin/knowledge-setup
  ./cmd/knowledge-setup`. Ollama 필요: bge-m3 모델)
- **완료 체크**: 로그 마지막에 `verify-align` `done` 이벤트 + 종료 라인.
  이후:
  1. `graph/manifest.json`: graph_digest가 `65d74ed7…`인지 — **주의:
     policy enrichment 포함 빌드이므로 digest 분리(P4)가 맞다면 동일해야
     정상**(enrichment는 code digest에서 제외됨). enrich_digest가 비어있지
     **않아야** 함(governed_by 주입 증거)
  2. `vector/manifest.json`: chunk_count(참조 19,890 근처 — docs_roots
     없이 코드만이면 ~19,343 근처), embedding identity=bge-m3/1024,
     `sources.ckg.graph_digest` == graph digest
  3. 결과에 따라: 정상 → **C(fused 스모크)** 진행 / verify-align 실패 →
     로그의 warning/error로 원인(핀 불일치=재정렬 필요) 진단
- **알려진 한계**: setup.yaml에 docs_roots(domain corpus) 미지원 — 참조
  데이터셋과 달리 문서 청크 미포함. 후속: domain-export로 corpus 생성 후
  `ckv build --docs` 경로 추가(§8.15 잔여와 함께)

### ① topic_tree 결정화 이중 빌드 — **완료·커밋됨** (`db6c291`)

재검증 필요 시: 임의 소스에 대해 `bin/graph-fixed build` 2회 → 
`SELECT parent_id,child_id,resolution,topic_label FROM topic_tree ORDER BY
…` 덤프 해시 비교. graph_digest는 수정 전후 불변이어야 함.

### ② vector 동등성 — **완료** (§8.16 기록)

재검증 필요 시(무경합 환경에서 임베딩 100% 일치 확인용):
```sh
# 구 도구 (3-repo)
~/Work/github/code-knowledge-vector/bin/ckv build --src <SRC> --out <OUT-old> --ckg <graph-dir>
# 신 도구
$K/bin/vector-new build --src <SRC> --out <OUT-new> --ckg <같은 graph-dir>
# 대조: manifest 필드, chunks 전 컬럼 정렬 해시, chunk_vec_vector_chunks00 blob 해시
```
**반드시 순차 실행**(동시 실행 시 ollama 배칭 지터로 임베딩 ~1% 불일치 —
이번 세션에서 확인됨. 도구 문제 아님).

## 4. 순차 대기 (다음 할 일, 순서대로)

> **UPDATE 2026-07-27 — 1~4 전부 완료.** `go-stablenet@0bf2f4d1b`로 D 재생성·
> 검수(graph_digest `65d74ed7…` 재현), C fused 스모크 + 구 cks-mcp 응답 형태
> 대조 통과, E 런북 DRAFT 해제, §8.16 동등성 종결. 진행 중 발견한
> `enrich_digest` 미표면화 버그 수정(reproduce-first, `pipeline.go` +
> `enrich_digest_surface_test.go`). 상세는 리뷰 문서 §8.16 "종결 재검증
> (2026-07-27)". 남은 것은 이식 시점 결정(§6)뿐.

1. **D 완료 검수** (위 체크 절차) 
2. **C — fused 서버 스모크**: cks-refactor-1 데이터셋으로 system-mcp 기동
   ```sh
   # config 생성이 필요 — 구 cks의 gen-cks-config.sh 참고(경로: system/scripts/)
   # 확인: ops.health(backends 키가 graph/vector), alignment(OK=true,
   # src_commit 단일화·Sources 중첩 뷰), context.get_for_task 1건 질의
   ```
   3-repo cks-mcp와 응답 형태 비교(신규 additive 필드 외 동일해야).
3. **E — 런북 DRAFT 해제**: `docs/downstream-sync.md`에 vector 동등성
   결과·순차-빌드 주의(ollama 지터) 반영, DRAFT 표기 제거.
4. **동등성 종합 선언**: 리뷰 문서 §8.16 종결 + 이식 착수 가능 상태 명시.

## 5. 백로그 (우선순위 낮음/결정 대기)

| 항목 | 비고 |
|---|---|
| origin push | **사용자가 직접** 하기로 함. 28커밋 로컬-only, CI는 push 후 첫 검증 |
| system 10개 cmd 단일 CLI 통폐합 | §8.8 #3, UX 변경이라 결정 필요 |
| 엔진 CLI 자체 `--progress=json` | 현재 setup 레이어가 출력 라인 래핑 |
| setup.yaml docs_roots 지원 | D 한계 해소(domain corpus 임베드) |
| 운영 스크립트 stablenet 기본값 정리 | system/scripts/cks-*.sh 등 |
| vector Makefile GSN 타깃·dataset-toolkit 정리 | 팩 이동 여부 판단 |
| TestServeCmd_PortInUseWithOpen 플레이크 | full-suite에서만 발생, 단독 8/8 통과(부하 포함) — 테스트 간 상호작용 |

## 6. 이후 마일스톤

1. **downstream 이식**: `docs/downstream-sync.md` 절차대로. 핵심 원칙 —
   소스 리브랜딩 금지(빌드 주입만), projects 팩 별도, 이식 후 동등성 체크
   재실행. 시점은 사용자 결정.
2. **구 3-repo 아카이브**: 이식 완료·안정화 후.
3. projects 팩 추가(wbft, wemix 등) — §8.3 메커니즘 재사용.

## 7. 주요 경로 레퍼런스

| 무엇 | 경로 |
|---|---|
| upstream 리포 | `~/Work/github/knowledge-system` (main, 28커밋 로컬) |
| 구 3-repo | `~/Work/github/code-knowledge-{graph,vector,system}` |
| downstream | `~/Work/github/stablenet-knowledge-mcp` (홀드) |
| 리뷰 문서 원본 | `~/Work/github/generalization-upstream-structure-review-2026-07-23.md` (+이 폴더에 사본) |
| 검증 소스 | `~/Work/github/cks-seminar/test/cks-refactor-1` (clean, `0bf2f4d1b`) |
| 참조 데이터셋 | `~/Work/github/knowledge-data/devtest-cks-5@0bf2f4d1-65d74ed7` |
| 생성 중 데이터셋 | `~/Work/github/knowledge-data/cks-refactor-1` (+ `-build.log`) |
| 동등성 산출물 | ⚠️ 세션 스크래치(`/private/tmp/claude-501/...`) — **머신·세션 소멸성**. 증거 수치는 리뷰 문서 §8.16에 기록됨; 재현은 본 문서 §3 커맨드로 |
| 테이블 해시 스크립트 | 이 폴더 `evidence/dbhash.sh` |
| eval 대조 결과 | 이 폴더 `evidence/eval-{old,new}.json` (심층 동일) |

## 8. 재개 시 첫 확인 목록

1. `git -C ~/Work/github/knowledge-system log --oneline | head` — 28커밋
   (`db6c291`이 HEAD 근처인지)
2. `make build test lint` — 그린인지 (플레이크 1건 §5 참조)
3. D 로그 tail — 완료/실패/중단 판별 → §3 절차
4. 리뷰 문서 §8.16 — 동등성 최신 상태 원본
