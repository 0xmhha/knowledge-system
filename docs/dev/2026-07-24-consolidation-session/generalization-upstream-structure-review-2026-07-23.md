# 검토: 일반화 프로젝트를 기준(upstream)으로 전용 프로젝트를 파생·동기화하는 구조

- **작성일**: 2026-07-23
- **대상**:
  - 일반화(upstream): `code-knowledge-graph`(ckg) · `code-knowledge-vector`(ckv) · `code-knowledge-system`(cks) — 3개 독립 리포
  - 전용(downstream): `stablenet-knowledge-mcp` — 위 3개를 단일 모듈로 통합한 stablenet 전용 리포
- **목적**: 공통 기능을 계속 동기화할 수 있도록 일반화 프로젝트 구조를 어떻게 바꿀지 검토
- **근거 자료**: 3쌍 전수 코드 대조(모듈 경로·브랜딩 정규화 diff). graph 쌍 상세는
  [ckg-vs-snkm-graph-comparison-2026-07-23.md](ckg-vs-snkm-graph-comparison-2026-07-23.md) 참조.

## 1. 현재 상태 진단 (정량)

| 쌍 | 공통 | 바이트 동일 | 리네임만* | 실질 변경 | A(일반화)에만 | B(전용)에만 |
|---|---|---|---|---|---|---|
| ckg ↔ graph/ | 632 | 547 | 40 | 45 | 179 (대부분 빌드 아티팩트) | 2 |
| ckv ↔ vector/ | 273 | 120 | 127 | 26 | 27 (archive·bm25·viewer data) | 0 |
| cks ↔ system/ | 286 | 155 | 65 | 66 | 147 (domain corpus 43 + cmd/cks-* + docs) | 25 (cmd/stablenet-knowledge-*) |

\* 모듈 경로·브랜딩 문자열 정규화 후 동일

**양쪽 라인이 모두 활발함**: upstream 3-repo는 2026-07-19~23까지 커밋 진행 중
(ckg `ddb2bff`, ckv `e2b9c1b`, cks `8132e5b`), downstream은 07-22 통합(`1894d3e`) 직후
자체 수정(`31b806b`, #6) 진행. 구조를 잡지 않으면 발산은 가속된다.

## 2. 발산의 5가지 성격 (구조 설계의 근거)

- **A. 기계적 변환** (~230파일): 모듈 경로, 주석 속 cks/ckg/ckv → stablenet-knowledge.
  정규화하면 소멸 — 규칙으로 기술 가능한 변환.
- **B. 배포 정체성이 코드에 박힘**: MCP 툴 네임스페이스(`stablenet_knowledge.context.*`),
  MCP 서버명, `cmd/cks-*` → `cmd/stablenet-knowledge-*` 바이너리명, health/hit의 백엔드 키명.
  전부 코드 수정으로 처리됨 — diff의 최대 원인.
- **C. 일반 개선이 downstream에만 착지** ⚠️:
  - graph: manifest `Engine`/`BuilderVersion` dual-write + `EffectiveBuilderVersion()`
  - system: `AlignmentSources` 리팩터(deprecated 키 유예 포함), health `backendStat`
    서술적 엔진 키, `HitSource` = "graph"/"vector" 중립화
  - graph: `docs/TRAVERSAL-DEPTH.md` (depth=2 근거의 Tier-2 승격)
  - bm25 루트 공유화(graph/vector 공용)
  stablenet 전용이 아니라 일반화 프로젝트가 가져가야 할 개선인데 upstream에 없음 —
  동기화 부재의 직접 증거.
- **D. 진짜 프로젝트 전용**: go-stablenet domain corpus(43파일), dataset 빌드 스크립트,
  eval ground-truth, 도메인 문서.
- **E. 이력물**: docs/archive, eval 리포트 — 의도적 미이관(문제 아님).

## 3. 핵심 문제 3가지

1. **형태 불일치**: upstream은 3-repo(각자 go.mod, cks가 ckg/ckv v0.1.0을 모듈 의존으로 pin),
   downstream은 단일 모듈 모노레포. 같은 파일이라도 git 이력이 단절돼 merge 기반 동기화가
   불가능하고, 매번 수동 전수 대조·포팅이 된다.
2. **브랜딩·정체성이 코드 축**: B류가 config가 아니라 코드 diff — 전용 프로젝트를 만들 때마다
   수백 파일 diff가 생기고, 이후 모든 upstream 변경이 충돌 후보가 된다.
3. **역류 경로 부재**: C류 개선이 downstream에 갇힘. 다음 전용 프로젝트는 이 개선들 없이
   시작하게 된다.

## 4. 권장 구조: 2단계

### Phase 1 — upstream 모노레포화 + 발산점 파라미터화 (핵심)

일반화 3-repo를 **downstream과 동일한 형태의 단일 모노레포로 통합** (예: `code-knowledge-mcp`):

```
code-knowledge-mcp/            ← 일반화 upstream (신설)
├── go.mod                     단일 모듈
├── pkg/bm25/                  공유 코어 (downstream이 이미 검증한 배치)
├── graph/   (구 ckg)
├── vector/  (구 ckv)
└── system/  (구 cks)
```

downstream이 이미 이 형태를 검증했으므로 통합 방법 자체가 레퍼런스로 존재한다. 이후
**stablenet-knowledge-mcp를 이 모노레포의 fork로 재기반(rebase)** — 특성화된 delta(B+D류)만
얹으면 되고, 이후 동기화는 `git merge upstream/main`이 된다.

동시에 B류를 **주입점(injection point)으로 승격**:

| 발산점 | 현재 (downstream 방식) | 개선안 |
|---|---|---|
| MCP 툴 네임스페이스 | `nsTool` + 하드코딩 상수 | upstream이 `nsTool` 메커니즘 채택, 값은 config/ldflags (`-X mcphandlers.ToolNamespace=...`) |
| MCP 서버명·인스턴스명 | 코드 수정 | system config에 이미 `Name` 존재 — graph/vector에도 동일 패턴 |
| 바이너리명 | `cmd/` 디렉터리 리네임(25파일) | 중립 cmd 유지 + Makefile `BINARY_PREFIX` (심볼릭 명명) |
| manifest 정체성 | downstream 전용 | 일반 기능으로 upstream 백포트 (아래) |
| 브랜딩 주석 | 일괄 sed | upstream을 중립 용어(graph/vector/system 엔진)로 통일 → 리네임 diff 자체가 소멸 |

이러면 **전용 프로젝트의 코드 diff는 이론상 0에 수렴**하고, 남는 것은 config 값 +
D류(도메인 데이터)뿐이다.

#### Phase 1에서 함께 할 백포트 (downstream → upstream)

1. `Engine`/`BuilderVersion` dual-write + `EffectiveBuilderVersion()` (graph manifest —
   back-compat 설계가 이미 모범적)
2. `AlignmentSources` 리팩터 + deprecated 키 유예 (system)
3. health `backendStat` 서술적 엔진 키, `HitSource` = "graph"/"vector" (system)
4. `TRAVERSAL-DEPTH.md` Tier-2 승격 (graph)
5. bm25 루트 공유화 (모노레포화와 동시 달성)

### Phase 2 (선택, 안정화 후) — overlay 모델로 전환

Phase 1이 정착해 "전용 diff = config + 데이터"가 검증되면, 전용 프로젝트를 fork가 아닌
**얇은 overlay 리포**로 전환할 수 있다: upstream을 Go 모듈 의존성으로 pin하고(cks가 이미
ckg/ckv v0.1.0을 pin하던 방식의 확장), 전용 리포에는 config·domain pack·dataset·cmd 래퍼만
남기는 형태. 단, downstream이 `internal/`을 건드릴 일이 정말 없어졌을 때만 유효 —
Phase 1 이후 판단.

## 5. 대안 비교

| 방안 | 동기화 비용 | 단점 |
|---|---|---|
| 현상 유지 + 수동 포팅 | 매번 전수 대조 | 이번 검토가 그 비용의 증명 |
| 3-repo 유지 + git subtree/patch 동기화 | 중간 | 형태 불일치가 남아 변환 규칙(경로 재배치) 계속 유지보수 |
| **모노레포 upstream + fork/merge (권장)** | `git merge` | 초기 1회 통합·재기반 비용 |
| 즉시 library-only overlay | 최소 | 지금은 downstream이 internal을 수정 중이라 시기상조 |

## 6. 네이밍 / 레이어링 결정 (2026-07-24 논의 반영)

구조 검토 후속으로 "외부 노출명 vs 내부 통일명", "공통부 라이브러리화 + 인터페이스",
"코드네임 폐기" 세 가지를 검토했다. 세 가지는 **하나의 스킴**으로 수렴한다.

### 6.1 확인된 코드 사실 (결정의 근거)

- **MCP 툴 프리픽스는 컴파일 상수**: upstream `internal/mcp/flow.go` 등에
  `const ToolNameGetFlow = "cks.context.get_flow"`, downstream은 같은 자리를
  `stablenet_knowledge.context.*`로 치환. → 외부 정체성이 코드에 하드코딩됨.
- **인터페이스 seam은 이미 존재**: `internal/ckgclient/interface.go`,
  `internal/ckvclient/interface.go`의 `type Client interface` + Dummy/Real 구현,
  공유 데이터 타입은 `pkg/contract`.
- **서버 표시명은 이미 config로 외부화**: `internal/config/config.go`의 `Name`
  (MCP 핸드셰이크 이름 + ops.health에 echo). 툴 프리픽스만 외부화가 안 됨.
- **내부 백엔드 키는 downstream이 이미 중립화**: health `backendStat` 주석 키와
  `pkg/contract/hit.go`의 `HitSource`가 cks/ckg/ckv → 서술적 `graph`/`vector`로 변경.
  → "외부는 배포별, 내부는 중립 통일"이라는 방향을 downstream이 이미 부분 실천 중.

### 6.2 결정 — 이름 레이어 3분할

| 레이어 | 무엇 | 값 | 주입 방식 |
|---|---|---|---|
| **L1 외부 정체성** (MCP wire) | 툴 네임스페이스 프리픽스, MCP 서버명, health `name`/`description` | 배포별 (`stablenet-knowledge` 등) | **config / ldflags 주입** — 코드는 상수를 갖지 않음 |
| **L2 내부 엔진 역할** | 백엔드 키, `HitSource`, 로그·주석 어휘 | 통일 중립 (`graph` / `vector` / `system`) | 코드 고정, 프로젝트 불문 동일 |
| **L3 모듈/패키지 경로** | import path, 디렉터리 | 통일 | 모노레포 단일 모듈 (§4 Phase 1) |

여러 MCP 서버가 동시에 뜰 때 구분되어야 하는 것은 **L1뿐**이다. L2/L3까지 배포명을
섞으면 그것이 곧 §2-B(배포 정체성이 코드에 박힘) 발산의 원인이 된다.

**교정 1건**: L1 툴 프리픽스를 상수 → 주입값으로. `config.Name`이 이미 그렇게 동작하므로
같은 패턴의 확장이며, 이것만으로 전용 프로젝트별 툴명 코드 diff가 소멸한다.

### 6.3 결정 — 공통부 라이브러리화는 "신규 설계"가 아니라 "기존 seam 승격"

커스텀 영역을 두 종류로 구분해 과설계를 피한다.

- **코드 인터페이스가 필요한 커스텀** (진짜 seam): 엔진 교체·구현 차이.
  → 이미 있는 `ckgclient.Client` / `ckvclient.Client` + `pkg/contract`를
  **`internal/` → `pkg/`로 승격**해 공식 확장점으로 만든다. (Go의 internal 규칙상
  외부 소비를 하려면 필수) L1 주입도 이 경계에서 받는다.
- **데이터/설정 커스텀** (인터페이스 불필요): domain corpus(43파일), dataset 스크립트,
  eval ground-truth, config 값. → 기존 `config` + domain-pack 계약
  (ADR-0001 domain-pack-contract)으로 흡수. 새 인터페이스로 감싸지 않는다.

결과 구조: **라이브러리 core(L2/L3 통일 코드) + 얇은 주입면(L1 config/ldflags +
Client 인터페이스 + domain-pack 데이터)**. 전용 프로젝트 = core 소비 + 주입값·데이터 제공.
이것이 §4 Phase 2(overlay 모델)의 전제조건이며, 6.3을 제대로 하면 Phase 2가 자연스럽게 열린다.

선행 작업: downstream이 아직 `internal/`을 수정 중이므로, **무엇을 `pkg/` 계약으로
승격할지 확정**하는 것이 6.3의 첫 스텝.

### 6.4 결정 — 코드네임 폐기, 단 새 명칭 도입은 하지 않음

3글자 코드네임(ckg/ckv/cks)은 불투명하므로 폐기한다. 다만 `knowledge-graph` /
`knowledge-vector` 같은 **새 어휘를 도입하면 세 번째 명명 규칙**이 되어 downstream이
방금 통일한 것을 다시 깬다 (downstream 현행: 디렉터리 `graph/`·`vector/`·`system/`,
내부 키 `"graph"`/`"vector"`).

**결정**: L2 내부 역할명은 **`graph` / `vector` / `system`** (downstream 현행 유지,
재작업 0). 서술형(`knowledge-graph` 등)이 필요한 자리는 L1(제품 문맥·공개 doc·프리픽스
주입값)뿐이며 거기서는 자유롭게 쓴다.

→ 이로써 3번 항목은 "새 통일"이 아니라 **"upstream을 downstream 어휘에 정렬하는 백포트"**가
되고, §4의 C류(역류) 백포트와 한 번에 처리된다.

### 6.5 종합

> **L1(외부, 배포별·주입) / L2(내부 역할, 중립 통일 = graph·vector·system) / L3(모듈 경로 통일)**
> 로 이름 레이어를 분리하고, L1은 config/ldflags로, 커스텀 데이터는 config + domain-pack으로,
> 엔진 교체만 `pkg/`로 승격한 Client 인터페이스로 주입한다.

세 논점이 모순 없이 한 구조로 수렴하며, 전용 프로젝트의 **코드 diff는 거의 0**이 된다.

## 7. 운영 규율 (구조가 아니라 규칙으로 지킬 것)

- **일반 개선은 upstream-first**: 전용 리포에서 먼저 필요해졌더라도 C류는 upstream PR →
  downstream merge 순서로. (이번 manifest/alignment 건 같은 역류 방지)
- **downstream 자체 커밋은 overlay 영역으로 제한**: config, `domains/`, dataset,
  eval ground-truth, 전용 docs.
- **drift budget CI**: 정규화-diff 스크립트를 CI 체크로 상설화 — "정규화 후 diff가 허용 목록
  밖 파일에서 발생하면 실패". 발산이 규칙을 벗어나는 순간 감지.

## 8. 통합 리포 target 구조와 실행 플랜 (2026-07-24 확정)

11개 항목의 통합 방안을 비판적 검토 → 항목별 사용자 의견 → 추가 코드 검증을 거쳐
확정한 내용. **이 절이 §4의 Phase 1/2 계획을 구체화·일부 supersede한다** (특히
"fork 모델"은 전환기 조치로 격하되고, projects/ 기반 overlay가 기본 모델이 됨).

### 8.1 확정 사항 요약

| # | 항목 | 결정 |
|---|---|---|
| 1·2 | 통합 repo 신설 + SSOT | 확정. SSOT 대상은 "이미 같은 코드"(bm25, contract 타입, manifest 좌표 규약, canonical_id 정렬 타입)만. "비슷해 보이는 것"은 올리지 않음 — 공용 pkg의 잡동사니화 방지 |
| 3 | 용어 | **`graph` / `vector` / `system`** (knowledge- 프리픽스는 코드에서 제외 — Go 명명 규칙·stutter·재작업 0). L1(바이너리명·MCP 서버명·제품 문서)에서만 `knowledge-graph` 등 서술형 사용 |
| 4 | 프로젝트 확장 | **`projects/`** 폴더 (`x/` 아님). in-tree(`projects/<name>`)와 out-of-tree(사설 리포가 upstream 모듈 의존 + 자기 projects/ 구현) **양쪽 동시 지원** — §8.3 |
| 5·8 | MCP | 공통 코어를 **`pkg/mcp`**로(네임스페이스 주입·부트스트랩·health/alignment 타입), `cmd/` 하위에서 조립해 graph 전용·vector 전용·fused MCP 서버를 각각 생성. 툴 핸들러는 엔진 소유 유지 |
| 6·7 | cmd/·docs/ 통합 | 루트 `cmd/`·`docs/` 하위 graph·vector·system·mcp 분류. docs 루트에 cross-engine 조율 문서 자리 신설(현재 vector/docs의 조율 문서를 graph 주석이 참조하는 문제 해소). system 10개 cmd의 단일 cobra 통폐합은 CLI UX 변경이므로 별도 결정 |
| 9 | knowledge-setup | Go typed-plan 모듈로 bash 스크립트 대체. **실행 백엔드는 spawn(subprocess) 기본** — §8.4 |
| 10 | migration | `setup / update / migrate / enrich` 4개 동사 분리. enrich-핀 문제는 digest 분리로 해결 — §8.5 |
| 11 | DB 모듈 | `pkg/db` 공유 인프라 + 엔진별 인터페이스 + DB별 handler — §8.6 |

### 8.2 Target 레이아웃

```
<upstream-repo>/                  # 단일 go.mod
├── cmd/
│   ├── graph/ vector/ system/    # 엔진 CLI (바이너리명은 Makefile BINARY_PREFIX = L1)
│   ├── graph-mcp/ vector-mcp/ system-mcp/   # pkg/mcp 조립 (§8.1 #5·8)
│   └── eval-gate/
├── pkg/                          # cross-engine 공유 (SSOT + 계약 + 주입면)
│   ├── bm25/  contract/  mcp/  db/  setup/
├── graph/   internal/… pkg/…     # 엔진 구현 (엔진 전용 pkg는 엔진 아래 유지)
├── vector/  internal/… pkg/…
├── system/  internal/…
├── projects/
│   └── stablenet/                # config·domain corpus·policies·dataset 정의·eval GT
├── docs/    graph/ vector/ system/ mcp/ + (루트: cross-engine 조율)
└── Makefile                      # BINARY_PREFIX 등 L1 주입
```

### 8.3 projects/ — 이동 판정 기준과 주입 메커니즘

**전제 확인 (audit 결과)**: 3-repo의 Go 코드에는 stablenet 특화 로직이 사실상 없고,
특화물은 전부 데이터·스크립트·eval에 있다. 따라서 hook/interface가 커버할 범위는
현재 작으며, 대부분 **데이터 주입(런타임 경로 해석)** 으로 해결된다.

**이동 판정 기준 — "프로젝트가 소유한 데이터인가, 범용 로직인가"**:

| 대상 | 판정 |
|---|---|
| `generated/domain-corpus/go-stablenet`(43) · `generated/policies` · `stablenet-filelist` | → `projects/stablenet/` |
| `eval/stablenet/*` ground-truth (graph·system) | → `projects/stablenet/eval/` |
| `build-stablenet-dataset.sh` · `build-vector-stablenet.sh` | **분해**: 범용 오케스트레이션 → setup 모듈, stablenet 파라미터 → `projects/stablenet/config` |
| synthetic corpus · ckv-mirror 등 엔진 테스트 픽스처 | 엔진 잔류 (프로젝트 소유물 아님) |

파일명에 stablenet이 있다고 통째로 옮기면 범용 로직이 프로젝트 폴더에 갇혀 다음
프로젝트에서 복붙이 재발한다 — 이 분해가 핵심.

**주입 메커니즘**: 빌드타임 매직(build tag, `init()` 부수효과) 비권장. 우선순위는
① 데이터/설정 주입(런타임 경로 해석 — 현 domain-pack 방식) → ② 코드가 정말 필요할
때만 명시적 등록(`projects/<name>`이 `pkg/` 인터페이스 구현, `cmd/`에서 명시 import로
조립). 이 방식이면 in-tree와 out-of-tree가 **동일 메커니즘**으로 동작한다.
의존 방향 규칙: **projects/ → core 허용, core → projects/ 금지** (CI로 강제).

### 8.4 setup 실행 모델 — spawn 기본 (2026-07-24 입장 수정)

코드 확인 결과 시스템의 build/index 경로는 **이미 spawn 방식**이다
(`system/internal/mcp/ops_index.go:46` `exec.CommandContext`로 `ckg build
--policy-file …` 실행). query 경로만 in-process(`ckgclient.NewReal`, ckv도 G1에서
subprocess→in-process 전환).

- setup 모듈은 `StepRunner` 인터페이스 뒤에 실행 백엔드를 둔다.
  **기본 = Subprocess(spawn)**: 프로세스 격리(빌드 OOM/크래시가 MCP 서버를 죽이지
  않음), kill 기반 취소. InProcess는 테스트·임베드용 옵션.
- 이때 필수가 되는 것은 pkg 빌드 진입점이 아니라 **안정된 기계-판독 CLI 계약**
  (플래그·종료코드·`--progress=json` 류 진행 출력) — setup 모듈의 파싱 대상이므로
  버전 계약으로 관리. pkg 진입점("graph/pkg/ 아래 빌드 API")은 "필요해지면"으로 격하.
  (배경: Go internal 규칙상 루트의 setup 모듈은 `graph/internal/buildpipe`를 import
  불가 — in-process 옵션을 원할 때만 pkg 승격이 필요)
- MCP 경유 setup은 장시간 작업 → **비동기 job 모델**(시작 툴 + 상태 조회 툴)로 설계,
  기존 `ops.index`를 흡수·확장(병렬 구현 금지).

### 8.5 enrich와 좌표 핀 — graph 암묵지 주입의 실체와 digest 분리

**graph에도 암묵지 주입 요소가 이미 2개 존재한다** (순수 코드+git 시스템이 아님):

1. `--policy-file` (P1 #4): cks-domain-sync가 뽑은 policy.yaml → `NodePolicyRule` +
   `governed_by` 엣지 ("이 코드는 어떤 정책의 지배를 받나")
2. `SecurityPatternFile` (P1 #5): YAML qname 매칭 → `NodeSecurityPattern` +
   `has_security_pattern` 엣지

단, 주입의 본체는 vector가 맞다(corpus·glossary·invariants·conventions·flow).

**확정된 설계 문제**: `ComputeGraphDigest`(graph/internal/buildpipe/graph_digest.go)는
temporal(Commit/Hunk)만 제외하므로 **policy/security 노드가 digest에 포함**된다 →
policy enrich만 해도 좌표 핀이 바뀌어 vector 재정렬 캐스케이드 발생. vector 정렬은
코드 심볼 canonical_id로 조인하므로 이 재정렬은 불필요한 낭비.

**결정**: 핀을 **`(code_digest, enrich_digest)` 쌍으로 분리** — code_digest는
enrichment 계열을 temporal과 동급으로 제외, vector 재정렬 트리거는 code_digest
변경만. enrich_digest는 "주입물 변경" 신호용. → upstream 설계 변경 항목.

migration 동사 4분리 (트리거·의미론이 다름):

| 동사 | 트리거 | 기존 구현 | 핀 영향 |
|---|---|---|---|
| update | 소스 코드 변경 | graph incremental cache, vector reindex+재정렬(P2a) | code_digest 변경 → 재정렬 필수 |
| migrate | 엔진 스키마 진화 | graph sqlite_migrate.go (멱등 ALTER) | 없음 |
| enrich | 암묵지 주입 | --policy-file, SecurityPatternFile, domain-sync | enrich_digest만 (분리 후) |
| setup | 최초 구축 | bash 스크립트 (→ setup 모듈로 대체) | 신규 생성 |

### 8.6 pkg/db 구조

"엔진 통합 단일 인터페이스"가 아니라 **공유 인프라 + 엔진별 인터페이스 + DB별
handler**:

```
pkg/db/
├── (공유 인프라) 커넥션 수명주기 · 락/트랜잭션 규율 · WAL ·
│   백업/복구 훅 · health · blue-green 데이터셋 스왑
├── graphstore/                   ← graph 기존 Store/StoreReader 승격 (재설계 금지)
│   ├── sqlite/  postgres/        ← handler (이미 2백엔드 검증됨)
├── vectorstore/                  ← 신규 얇은 인터페이스 (현 사용 메서드 최소집합)
│   ├── sqlitevec/  (pgvector/ 향후 — graph=postgres와 단일 DB 서버 배포 가능성)
```

- 라이브니스·복구는 현재 DB 내부가 아니라 **blue-green versioned dataset 레이아웃**
  (`@<ver>`, alignment.go DatasetVersion)이 파일 수준에서 담당 — 이 스왑·핀 검증
  메커니즘이 공유 db 모듈의 1순위 공통 관심사.
- 경계 2건 사수: graph persist는 인터페이스 **승격만**(재설계 X); system은 자체 DB가
  없으므로 이 모듈의 소비자가 아님(`ckgclient`/`ckvclient` seam 유지).
- vectorstore 인터페이스는 두 번째 백엔드 요구 전까지 얇게 유지(YAGNI —
  sqlite-vec 형태 베끼기 방지).

### 8.7 수정된 실행 플랜

1. **P0**: repo 신설 + 3-repo subtree graft(이력 보존) — 레이아웃은 우선 downstream과
   동일한 `graph/`·`vector/`·`system/`
2. **P1**: 순수 `git mv` 커밋으로 cmd/·docs/ 재배치 (로직 변경 0, 링크 체커 동반)
3. **P2**: L2 중립화(어휘 graph/vector/system) + downstream C류 백포트
   (manifest·alignment·bm25) — 개명 커밋과 로직 커밋 분리
4. **P3**: `pkg/mcp` 코어 추출 + L1 주입(네임스페이스 config/ldflags) +
   cmd/{graph,vector,system}-mcp 조립
5. **P4**: setup 모듈 (spawn StepRunner + CLI 계약 + 비동기 job, ops.index 흡수) —
   bash 대체. code/enrich digest 분리 포함
6. **P5**: `projects/` 메커니즘 + stablenet 자료 이동(§8.3 판정표) + downstream
   재기반 또는 폐지 결정
- 병렬 가능: `pkg/db` 정리(vectorstore seam), system cmd 통폐합 결정

### 8.8 남은 결정

1. ~~stablenet 도메인 데이터의 in-tree 가능 여부~~ → **확정: in-tree** (2026-07-24).
   근거: 3-repo·go-stablenet 모두 공개 리포. generated/ 데이터는 공개 코드·개발자
   docs·README에서 LLM이 생성한 파생물("generated"의 유래). eval은 기능 동작
   검증용. **SecurityPattern 실검토 결과**: 프로덕션 인스턴스 부재(스키마+로더+
   교과서적 픽스처만), policies 실내용은 공개 코드 파일:라인 인용 불변식 서술 —
   민감물 없음 확인. 원칙: **in-tree에는 공개 불가 데이터가 절대 들어가서는 안
   되며, 존재할 수 없다.**
   추가 발견: `generated/`는 git 미추적 산출물(domain-export가
   docs/domain-knowledge에서 생성) → P5 이동 대상은 산출물이 아닌 **소스**.
2. ~~통합 repo 이름~~ → **확정: `github.com/0xmhha/knowledge-system`** (2026-07-24)
3. **system 10개 cmd의 단일 cobra 통폐합 여부** (CLI UX 변경) — 미결
4. ~~downstream 최종 처분~~ → **확정: 두 리포 유지 (4-A 모델)** (2026-07-24).
   upstream = 일반화·기능 확장 프로젝트, downstream = stablenet 특화 프로젝트로
   **목적이 명확히 다름**. downstream은 통합 과정의 전면 네이밍 치환(ckg/ckv/cks
   → stablenet-knowledge)으로 가독성·확장성이 훼손되어 **홀드** 상태. upstream
   정리가 완료되고 3-repo 버전과 동등한 동작이 전 테스트에서 검증되면,
   **upstream → downstream 코드 이식으로 동기화**한다. 이후에도 upstream의 버그
   수정·유용 기능을 downstream에 적용하기 쉽게 하는 것이 이번 통합·모듈화·
   projects 분리 작업의 목적.
   4-A의 알려진 리스크(발산 재발)는 현 구조가 완화: L1 주입으로 브랜딩 diff
   소멸(P3), projects/ 분리로 delta가 데이터+설정으로 수렴(P5), 정규화-diff
   스크립트가 이식 검증 도구 — 이식이 전수 대조가 아닌 기계적 적용이 됨.

### 8.9 P0 완료 기록 (2026-07-24)

`~/Work/github/knowledge-system`에 P0 완료. main 직접 커밋(사용자 지시로
git-guard 예외 승인), 커밋 8개:

1. 스캐폴딩 (README·AGPL-3.0 LICENSE·.gitignore — cks는 LICENSE가 없었음)
2. ~ 4. 3-repo 이력 그래프트 (`graph/`·`vector/`·`system/`, **총 1,033 커밋 보존**;
   `src-graph`/`src-vector`/`src-system` 리모트 유지 — 구 리포 후속 커밋을
   `git fetch src-X && git merge -X subtree=X src-X/main`으로 계속 수용 가능)
5. 단일 모듈 통합 (root go.mod — downstream의 검증된 의존성 병합 세트 채택,
   import 경로 재작성, per-engine go.mod 제거)
6. bm25 공유 코어 승격 (`pkg/bm25`; vector 포크 5파일 삭제 + rerank/keyword/explain
   재배선 — downstream 검증 패턴 그대로)
7. vector discover 자기-테스트 수정 (go.mod 앵커 → 프로젝트 디렉터리 앵커;
   downstream 동일 수정 이식)
8. 루트 Makefile + gofmt 정리

검증: `go build ./...` · `go vet ./...` · `go test ./...`(전 엔진) · `make lint`
모두 통과.

P0에서 의도적으로 남긴 것(후속 단계 범위): docs/Makefile 등 비-코드 파일 28곳의
구 모듈 경로 문자열(P2 어휘 정리와 함께), cmd/·docs/ 재배치(P1), per-engine
`.github`/`.claude` 정리(P1), downstream의 `ckg_path→graph_path` 등 L2 키
중립화(P2 백포트).

### 8.10 의존성 보안 백로그 (2026-07-24 기록, 본격 감사는 리팩토링 완료 후 별도 작업)

전제: 통합으로 신규 유입된 외부 의존성 0개(39개 전부 구 3-repo 사용분) —
감사 유예가 현재 노출을 늘리지 않음. sqlite 사용처는 로컬 생성 데이터
(자기 소스 파싱 산출물) 위주로 신뢰경계 노출이 낮음.

**기록된 발견**:

1. **sqlite-vec-go-bindings 내장 sqlite 3.47.2에 알려진 CVE** (수동 검토 발견).
   ⚠️ govulncheck가 잡지 못함 — Go vulndb는 C amalgamation 내장 코드 CVE에
   사각지대. 자동 도구 의존 불가, 수동 추적 필요.
2. **GO-2026-5970** `golang.org/x/text` v0.29.0 → fix v0.39.0 (무한루프).
   govulncheck 확인, 도달 경로는 Postgres opt-in(`persist.OpenPostgresCold`)뿐.
   버전 범프로 즉시 해결 가능.
3. **sqlite 3벌 중복**: `modernc.org/sqlite`(순수 Go, graph) ·
   `mattn/go-sqlite3`(cgo, vector) · `sqlite-vec-go-bindings`(자체 amalgamation
   3.47.2, vector). 버전·패치 주기 제각각 — 별도 감사 작업의 실질 과제는
   39개 개별 검토보다 이 3벌의 정합·축소.
4. P0 템플릿 유입 패치 범프 2건 (구 repo 버전과 다름): `mattn/go-sqlite3`
   v1.14.44→48, `yalue/onnxruntime_go` v1.30.1→1.31.0 — 원복 또는 명시적
   범프 커밋으로 분리 결정 대기.

**상설화**: `govulncheck`를 `make vuln` 타깃 + CI 게이트로 추가 (일회성 감사
대신 상시 검사).

→ (2026-07-24 처리) #2 x/text v0.39.0 범프 완료 — govulncheck "No vulnerabilities
found". #4 원복 완료. `make vuln` 타깃 + CI advisory job 추가. #1·#3은 별도 감사
작업으로 유지.

### 8.11 P1 완료 기록 (2026-07-24)

커밋: 의존성 정리(`91e0d50`) · vuln 타깃(`f1f01d2`) · **레이아웃 재구성(`9441b7d`)**
· docs 이동(`f03b654`) · 루트 CI(`82141b0`).

**중요 설계 변경 — Go internal 규칙과의 충돌 해소** (§8.2 레이아웃 정정):
루트 `cmd/`가 `<engine>/internal`을 import할 수 없음(Go internal 가시성은 부모
디렉터리 스코프). 사용자 결정으로 **엔진 internal을 `internal/<engine>`으로
재배치** — 모듈 외부 비공개성은 유지되나 엔진 간 격리는 컴파일러 강제에서
**`scripts/check-boundaries.sh` (make lint 내장) 강제로 전환**.

최종 레이아웃:

```
knowledge-system/
├── cmd/  graph/ vector/ eval-gate/ system-mcp/ system/{agent,domain-sync,…9개}
├── internal/  graph/ vector/ system/     ← 엔진 구현 (경계는 lint로 강제)
├── pkg/  bm25/                           ← 공유 (SSOT)
├── graph/  pkg/ web/ eval/ testdata/ policies/ scripts/   ← 엔진 잔여물
├── vector/ pkg/ eval/ testdata/ policy/ scripts/
├── system/ pkg/ docs/(domain-knowledge 런타임 소비로 보류) dataset-toolkit/ …
├── docs/   graph/ vector/                ← system/docs는 P5(projects/)와 함께
└── .github/workflows/ci.yml             ← build+lint+test / vuln(advisory); 미push 미검증
```

바이너리 출력명은 전부 불변(bin/ckg 등 — L1 리네임은 P3). 검증: build·vet·
전체 테스트·lint(boundaries 포함) 통과. 테스트 상대경로 픽스처 ~10곳 수정.

다음: P2 (L2 어휘 중립화 + downstream C류 백포트 + 비-코드 파일 구 모듈 경로
28곳 정리).

### 8.12 P2 완료 기록 (2026-07-24)

커밋 4개 (로직/개명 분리 규율 준수):

1. `fa8769d` **graph manifest 엔진 정체성 백포트** — `Engine`("graph")·
   `BuilderVersion` dual-write + `EffectiveBuilderVersion()`/`WithGraphBuilderIdentity()`,
   read-side 양방향 미러링, cache 판정 전환, JSON export 헤더, 테스트 2건. §8.1 계획 그대로.
2. `2262a88` **system wire 표면 중립화** — `HitSource` "ckg"/"ckv"→"graph"/"vector",
   dummy Backend 라벨, health backend 맵 키, ops.index 응답 키, vector `ckg_path`→
   `graph_path`, eval report `cks_version`→`builder_version`. **AlignmentSources
   중첩 뷰 + 단일 `src_commit`(합의 시만) + 구 suffix 키 deprecated dual-write** 이식.
   config yaml 키(ckg:/ckv:)와 로그 필드는 downstream과 동일하게 유지.
3. `e5f469b` TRAVERSAL-DEPTH.md 승격 + 툴 설명 참조 갱신 + DOC-MAP 등재.
4. `7fff301` living docs 모듈 경로 정리 — dated handoff/coordination 문서는
   pre-consolidation 이력 기록으로 의도적 제외.

**P2에서 하지 않은 것 (의도적)**:
- MCP 툴 프리픽스(`cks.context.*` 등) — P3 L1 주입에서 처리 (두 번 리네임 방지)
- 코드 주석 산문의 코드네임(ckg/ckv/cks) 일괄 치환 — 기계 치환은 문장을 깨뜨리고
  (downstream의 "syntax stablenet-knowledge consumers" 사례), L2 역할 어휘는 이미
  디렉터리·wire 키에서 중립화됨. 코드네임은 역사적 명칭으로 유지, 루트 README
  glossary가 매핑 담당.

검증: 전체 테스트 ALL PASS · lint(boundaries 포함) OK.

다음: P3 — `pkg/mcp` 코어 추출 + L1 주입(네임스페이스 config/ldflags) +
cmd/{graph,vector}-mcp 조립.

### 8.13 P3 완료 기록 (2026-07-24)

커밋 4개: graph(`3c94f2f`) · vector(`e8f610f`) · system(`1bf1351`) ·
빌드/경계(`67b72ec`).

**L1 주입 설계 (구현됨)**:
- `pkg/mcp` — 네임스페이스 루트 해석 단일 규칙:
  **explicit(flag/config) > env `KNOWLEDGE_MCP_NAMESPACE` > ldflags `-X pkg/mcp.BuildRoot` > 엔진 기본값**
- 엔진 기본값이 하위호환을 보존: graph = ""(bare 이름), vector/system = "cks"
  (기존 wire 그대로). 루트 주입 시 `<root>.context.<name>` / `<root>.ops.<name>`
  으로 통일 — downstream의 `stablenet_knowledge.context.*`가 **코드 diff 0으로
  재현됨** (스모크로 실증: bare/flag/env/ldflags 4모드 확인).
- system의 `ToolName*` 상수들은 주입 가능한 var로 전환, 리포 내 클라이언트
  (eval runner·agent)도 리터럴 대신 파생값 참조 — 루트 변경 시 end-to-end 일관.

**신규 바이너리**: `cmd/graph-mcp`(stdio, graph 8-tool), `cmd/vector-mcp`
(stdio/HTTP, embedder 플래그 명시). §8.1 #5·8의 "graph 전용/vector 전용 MCP"
충족. `make build-mcp NAMESPACE=<root>`로 3종 빌드+스탬핑.

검증: 전체 테스트 통과(1건 flaky — `TestServeCmd_PortInUseWithOpen` 포트 타이밍,
단독 재실행 2회 통과, P3 무관) · lint/boundaries OK · graph-mcp 라이브 스모크.

다음: P4 — knowledge-setup 모듈 (spawn StepRunner + 기계-판독 CLI 계약 +
비동기 job, ops.index 흡수, code/enrich digest 분리 포함).

### 8.14 P4 완료 기록 (2026-07-24)

커밋 3개: digest 분리(`361c943`) · setup 모듈+CLI(`0ae7e32`) ·
비동기 MCP surface(`83f1814`).

1. **code/enrich digest 분리** (§8.5 결정 구현): `ComputeGraphDigest`가
   enrichment(Policy/SecurityPattern + governed_by/has_security_pattern)를
   temporal과 동급으로 제외 — **enrich가 좌표 핀을 더는 무효화하지 않음**.
   신규 `ComputeEnrichDigest` + manifest `enrich_digest` 필드(additive).
   ⚠️ 마이그레이션 노트: 기존에 enrichment 포함으로 빌드된 그래프는 다음
   리빌드에서 graph_digest가 1회 변경 → vector 재정렬 1회 발생.
2. **`internal/setup`**: typed Plan(graph-build → vector-build --ckg →
   verify-align) + `Runner` 인터페이스(기본 `SubprocessRunner`: spawn·스트리밍
   출력·kill 취소) + `Event` 스트림 + manifest 직독 정렬 게이트(누락 좌표=경고,
   불일치=실패) + in-memory 비동기 `Jobs` 레지스트리. **엔진 internal 무import
   — CLI 플래그가 계약**이며 boundary 검사로 강제.
3. **`cmd/knowledge-setup`**: `--progress=text|json`(json = 라인당 이벤트 1개
   — 기계-판독 계약). **E2E 라이브 검증**: synthetic corpus로 graph+vector
   빌드 → verify-align이 실제 좌표 핀 매치로 통과.
4. **`ops.setup` / `ops.setup_status`** (fused server): 시작→job_id 즉시 반환 +
   상태 폴링(이벤트 tail). **ops.index는 병렬 exec 경로를 버리고 setup의
   subprocess 기계를 공유**(흡수). 스키마 SSoT 픽스처(agent-mcp.schema.json)에
   툴 2종 등재.

의도적 보류: bash 스크립트 삭제는 P5(projects/ 이동)에서 stablenet 파라미터
분해와 함께. 엔진 CLI 자체의 `--progress=json`은 후속(현재는 setup 레이어가
출력 라인을 이벤트로 래핑).

Known-flaky: `TestServeCmd_PortInUseWithOpen` — 실포트 점유 테스트라 풀 스위트
병렬 부하에서 간헐 실패(단독 통과). 수정 후보로 기록.

다음: P5 — `projects/` 메커니즘 + stablenet 자료 이동(§8.3 판정표) +
downstream 처분 결정(§8.8 #1·#4 선결).

### 8.15 P5 완료 기록 (2026-07-24)

선결 결정 확정(§8.8 #1 in-tree, #4 두 리포 유지·이식 동기화) 후 커밋 2개:
팩 분리(`b967c42`) · setup config(`f7c5794`).

**projects/stablenet 팩 구성**:
- `policies/graph.yaml`(구 graph/policies/stablenet) · `policies/vector.yaml`
  (구 vector/policy/stablenet.yaml) · `domain-knowledge/`(구 system/docs/
  domain-knowledge/projects/go-stablenet, 76파일 — **소스**; generated/는
  미추적 산출물이라 이동 대상 아님) · `eval/graph`·`eval/graph-keyword`
  (구 graph/eval/stablenet*, 65파일) · `scripts/`(dataset 빌드 2종) ·
  `validate/`(팩 데이터를 엔진 로더로 검증하는 Go 테스트 — 구
  internal/vector/policy/verify_test.go에서 이전, exported API 경유로 재작성) ·
  `setup.yaml` · README(팩 규칙 명문화)
- **의존 방향 강제 확장**: 팩→엔진 import 허용 / 엔진→팩 금지. boundary
  검사에 "엔진 코드의 `"projects/<name>` 문자열 리터럴 금지" 규칙 추가
  (generic `"projects/"` 마커는 허용).
- **core 탈-stablenet**: domain-sync `-entries` 기본값 제거(명시 필수),
  도구 usage 예시 projects/ 규약으로, worksheet 경로 축약기 신·구 레이아웃
  모두 인식.
- **`knowledge-setup --config`**: 팩 setup.yaml에서 파라미터 로드(상대경로는
  파일 기준 해석, 명시 플래그 우선). 스모크: 팩 정책 경로가 플랜에 주입됨
  확인.

잔여(후속): system 운영 스크립트들(cks-mcpd 등)의 stablenet 기본값,
vector Makefile GSN 타깃, dataset-toolkit — 기본값/타깃 수준이라 팩 이동
필수는 아니며 점진 정리. docs 산문 내 구 경로 언급.

검증: 전체 테스트 통과(known-flaky 1건 제외) · lint/boundaries OK.

**남은 플랜**: P0~P5 완료. 다음 마일스톤은 §8.8 #4의 이식 동기화 —
① upstream 전 기능 검증(3-repo 동등성: eval 하네스·실데이터 빌드 재현),
② downstream 이식 절차 수립(정규화-diff를 검증 도구로), ③ 구 3-repo 아카이브
시점 결정. 미결: §8.8 #3(system cmd 통폐합), §8.13의 엔진 CLI --progress=json.

### 8.16 동등성 검증 기록 (2026-07-24 입증; 2026-07-27 종결)

대상: `cks-seminar/test/cks-refactor-1` (clean, `0bf2f4d1b` — 참조 데이터셋
`knowledge-data/devtest-cks-5@0bf2f4d1-65d74ed7`과 동일 커밋).
비교 축: 참조 데이터셋(07-14, 3-repo 도구) ↔ 구 3-repo 도구 현재 빌드 ↔
신 upstream 도구 현재 빌드.

**Graph 엔진 — 동등성 입증 완료**:

1. `graph_digest` **3자 바이트 일치** (`65d74ed74f57a940…`), nodes 244,930 /
   edges 1,963,805 / pkg_tree 228,815 / languages / files 1,323 /
   parse_errors 0 전부 일치.
2. **DB 전 테이블 정렬 덤프 해시 감사** (digest 미포함 영역까지):
   nodes(전 컬럼: signature·doc_comment·파생 메트릭 포함)·blobs·node_prs·
   pending_refs·pkg_tree·FTS 내용 완전 일치. edges는 rowid(삽입 순서
   대리키) 제외 전 컬럼 일치. manifest 차이는 additive 신규 키 3개
   (`builder_version`/`engine`/`enrich_digest`)뿐 — P2·P4 의도분.
3. **발견 — topic_tree 비결정성 (upstream 회귀 아님, 3-repo부터 잠재)**:
   같은 신규 도구 2회 빌드가 서로 다름(127,498 vs 127,405행; old 도구
   127,404행). graph_digest는 2회차에도 동일 — 파생 데이터 제외 설계 덕에
   좌표 핀 무영향.

   **개선 후보 (upstream 백로그)**: `internal/graph/cluster/leiden.go` —
   Leiden은 `rand.New(rand.NewSource(opts.Seed))`로 시드 고정이지만,
   **Go map 순회 순서가 tie-breaking에 누출**되어 시드가 무력화됨.
   확인된 누출 지점: `leiden.go:80` `for k, w := range g.edgeWts`
   (aggregation 순서), `leiden.go:121` `for c, w := range toComm`
   (동점 커뮤니티 선택). **수정 방향**: range-over-map 지점에서 키를 정렬해
   순회(또는 동점 시 최소 커뮤니티 id 선택 규칙 고정) → seed 재현성 복원.
   수정 시 topic_tree가 결정적이 되므로 동등성 감사에서 해시 비교 가능해짐.
   (`cluster/naming.go`·`topic_tree.go`의 유사 패턴도 함께 점검할 것)

**Vector 엔진 — 진행 중**: 구/신 도구로 mock embedder(결정적 feature-hash)
빌드 중, 동일 graph(`eq/old-graph`)에 정렬 — 유일 변수 = 도구. 완료 후
chunk_count·languages·embedding identity·`sources.ckg` 핀·canonical_id
커버리지·DB 정렬 덤프 해시 대조 예정. 불일치 시 graph와 동일하게 "도구 차이
vs 비결정성"을 동일 도구 2회 빌드로 판별.

**기능 동등성 (keyword retrieval) — 입증 완료** (2026-07-24): 구 도구+구
그래프 vs 신 도구+신 그래프로 `eval-retrieval`(pack fixtures 8종) 실행 →
**출력 JSON 심층 일치**(경로 필드 제외 전체 — 픽스처별 R/P/F1·extra-hit 심볼
목록·실패 픽스처까지 동일: SNK06 precision 0.4, 동일 extra 9심볼). 참조
results.json(8/8)과의 차이는 도구가 아니라 **입력 그래프 차이**(참조는
knowledge-data/pr-77-2 = filelist 빌드 183K 노드; eq 빌드는 전체 트리 245K
노드로 test.* 심볼 포함 → SNK06 precision 하락) — 동등성과 무관.

**topic_tree 결정화 — 완료** (`db6c291`): 누출은 예상보다 3곳(leiden
localMove 동점 tie-break / splitProblemCommunities ID 배분 /
reLeidenSubgraph 반환 순서) + 인접 정렬 위생 1곳. 동점 유발 대칭 그래프
50회 반복 락킹 테스트 추가. **전체 코퍼스(245K 노드) 이중 빌드 증명 통과**:
topic_tree 127,155행 해시 완전 일치(수정 전엔 매 실행 상이), graph_digest
불변(`65d74ed7…`).

**Vector 엔진 동등성 — 입증 완료** (2026-07-24): 구/신 도구, 동일 소스,
동일 graph 정렬, 실모델(ollama bge-m3 — ckv 기본 embedder가 mock이 아니라
ollama였음). 결과:
- manifest 전 항목 일치: chunk_count 19,343 · languages · embedding
  identity(bge-m3/1024/ollama) · `sources.ckg` 핀(graph_digest `65d74ed7…`
  ·src_commit)
- **chunks 테이블 전 컬럼(21개, text·canonical_id 포함) 정렬 해시 완전
  일치** — canonical_id 스탬핑 16,085개 동일
- 임베딩 벡터: 19,456개 중 **98.95% 비트 단위 동일**, 불일치 204개(1.05%)는
  동시 추론 배칭 지터(ollama 서버에 빌드 3개 동시 요청 상황) — 입력 동일성이
  chunks 해시로 증명되므로 서빙측 비결정. 도구 동등성 성립. (선택: 무경합
  순차 재빌드로 100% 재확인 가능)

**동등성 검증 종합 — 이식 전제 조건 충족**: graph(구조·전 테이블) ✓ ·
기능(retrieval eval 심층 일치) ✓ · vector(청킹·정렬·canonical_id·모델
identity) ✓ · topic_tree 결정화로 잔여 비교 불가 항목 해소 ✓.

**종결 재검증 (2026-07-27) — `go-stablenet@0bf2f4d1b`, 신 도구 단독**:

- **데이터셋 재생성**: `knowledge-setup --config projects/stablenet/setup.yaml`
  로 graph(정책 enrichment) + vector(ollama bge-m3) 빌드
  (`knowledge-data/go-stablenet@0bf2f4d1b/`). graph_digest **`65d74ed7…`
  재현**, vector chunk_count **19,343**(코드 전용 — setup.yaml docs_roots
  미지원), `sources.ckg` 핀 일치. 참조 수치와 일치.
- **Fused 레이어(C) — 응답 형태 대조 통과**: 신 `system-mcp`(신 데이터셋) ↔
  구 `code-knowledge-system/bin/cks-mcp`(`test-data/go-stablenet`,
  `d7cff3df`) 동일 prompt. tools/list·ops.health·get_for_task 구조가
  **의도된 리네임 1건(backends `ckg/ckv`→`graph/vector`, P2) + additive
  필드/툴(`ops.setup`/`setup_status`, health `name`/`description`,
  `graph_digest`, alignment `sources`/`dataset_version`, pack
  `graph_neighbors`)**을 제외하고 동일 — 미설명 드리프트 없음. alignment.ok=
  true, `graph_digest_actual==expected`. (구 데이터셋은 커밋·docs 코퍼스가
  달라 값이 아닌 **형태** 대조 — 값 동등성은 graph_digest로 별도 증명됨.)
- **`enrich_digest` 잠재 버그 발견·수정**: cold build에서 enrichment 노드/
  엣지가 store에만 주입되고 in-memory 그래프엔 미반영되어
  `ComputeEnrichDigest(g...)`가 항상 `""` → manifest `omitempty`로 키 누락.
  **위 590행이 `enrich_digest`를 "additive 신규 키"로 적었으나 실제로는 이
  버그로 부재였음**(참조·3-repo 빌드도 동일 코드라 마찬가지 — "설계상 존재
  의도, 구현상 미표면화"). 수정: `internal/graph/buildpipe/pipeline.go`가
  skeleton 직후 주입된 overlay로 `EnrichDigest` 재계산(g 미변형 →
  Stats/Files/graph_digest 불변). 재빌드로 실증: `enrich_digest`
  `82a44b15…` non-empty, `graph_digest` `65d74ed7…` 불변, stats 불변.
  회귀 테스트 `enrich_digest_surface_test.go` 추가. **좌표 핀(graph_digest)
  동등성에는 영향 없음** — enrichment는 code digest에서 제외되므로 이 버그는
  overlay 관측 핀에만 국한.
- **incremental enrichment 침식 — 확정·수정**: incremental 경로가
  `loadPolicy`/`loadSecurityPatterns`를 재실행하지 않아, governed 심볼이 담긴
  파일을 수정하면 그 governed_by 엣지가 dirty-node 삭제의 FK CASCADE로 사라진
  뒤 재생성되지 않고, manifest enrich_digest도 "" 로 리셋됨(재현 테스트로
  cold governed_by=1→incremental=0 관측). 3-repo와 동일 설계 계보의 **선입
  버그**(upstream 회귀 아님). 수정: `incremental.go`가 persist 이후 overlay를
  edge-by-type + node-by-file로 정리하고 현재 policy/security YAML로 재적용,
  EnrichDigest 재계산(cold 미러, g 미변형). 회귀 테스트
  `enrich_incremental_test.go` 추가 — 수정 후 파일 수정에도 governed_by·
  enrich_digest 보존 확인.
- **잔여(비차단)**: 신 데이터셋은 docs 코퍼스 미포함(§ setup.yaml docs_roots
  백로그) — get_for_task가 도메인 flow 문서를 인용하지 못함.

**이식 착수 가능 상태**: graph·기능·vector·fused 4개 레이어 동등성 확인,
좌표 핀 바이트 재현. `docs/downstream-sync.md`(DRAFT 해제) 절차로 이식 가능.

## 다음 단계 후보

1. §8.7 P0 착수: repo 신설 + subtree graft 계획(3개 이력 보존), CI 통합
2. 백포트 PR 목록화 (§4 5건 + §6.4 어휘 정렬 + §8.5 digest 분리, 각각 독립 PR)
3. §8.8 결정 4건 확정 (특히 #1 in-tree 가능 여부가 P5 설계를 좌우)
4. §6.2 L1 주입 전환: 툴 네임스페이스 프리픽스를 상수 → config/ldflags
5. setup 모듈 CLI 계약 초안 (`--progress=json`·종료코드·플래그 안정화 목록)
