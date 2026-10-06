# 최초 리팩토링 목적과 현재 코드 대조

2026-10-06 · 감사 대상 `feat/spec-driven-knowledge-system`의 `04d3bfdf55f488ef6192e218579b694e3cf58024`. 이번 요청은 최초 목적·작업리스트를 추적하고 남은 범위를 정리하는 작업이다. 제품 수정·새 평가·운영 배포는 수행하지 않았다. [새 잔여 작업리스트](./REFACTORING-REMAINING-WORKLIST.md)가 이 감사의 후속 목록이다.

**시험 preview의 30/30 작업은 승인 범위에서 종료됐다. 그러나 검색 품질, 실제 파일럿 지식/수용 판정, 운영 출시까지 달성했다는 뜻은 아니다.** 이번 코드 대조에서는 기존 평가 제한 외에도 쓰기 경로·잠금·내구성·보관본 정리·자원 설정에 설계 대비 잔여가 확인됐다. 과거 완료 기록을 삭제하거나 기존 preview 목표를 다시 열지 않고 새 목록으로 분리한다.

**study 추가 탐색 보완:** [목적 원문·6월 마스터 계획과 현재 코드 대조](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md)를 추가했다. 상위 시니어 개발자 모방 목표의 WI10개를 복구했으며 제품 후속18개만으로 전체 상위 목표의 완료율을 표시하지 않는다. 9월에 참조한 원문3개의 미확보 상태는 유지한다.

## 1. 시작 문서와 출처 확인

| 자료 | 확인 결과 | 이 감사에서의 역할 |
|---|---|---|
| 최초 제품 실행 기록 | `41fa8817`(2026-09-29)의 [EXECUTION 원본](../../system/eval/b0-knowledge-system/original-goal-reaudit-m2max-2026-10-06/initial-execution-2026-09-29.md) 확보. 기준 `main`은 `1ded9b3e47bc2e09062dba329c2f918423746fa4` | 이번 spec-driven 리팩토링의 시작 시점과 W0/W1·원본 자료 위치 확인 |
| 최초 v2 순서 변경 | `c0654ed6`(2026-09-30)의 [DELIVERY-PLAN-V2 원본](../../system/eval/b0-knowledge-system/original-goal-reaudit-m2max-2026-10-06/initial-delivery-plan-v2-2026-09-30.md) 확보 | 기존 FR01–10/INV01–07/NFR01–05/S01–09 유지, A→B→C 순서와 구조/품질 분리 |
| 상세 목적/설계 | `a0a11c74`(2026-09-30)의 [END-TO-END-DESIGN 원본](../../system/eval/b0-knowledge-system/original-goal-reaudit-m2max-2026-10-06/initial-end-to-end-design-2026-09-30.md), [현행 설계](./END-TO-END-DESIGN.md) | 코드·문서·의미·스펙·설치를 공통 좌표와 근거로 연결하는 목표 계약 |
| 현재 추적 자료 | [A0 요구사항 추적](./A0-TRACE-BASELINE.md), [PDF 개선 추적](./PDF-IMPROVEMENT-TRACE.md), [현행 WBS v2](./DELIVERY-PLAN-V2.md), [누적 실행 기록](./EXECUTION.md) | 원 요구사항 ID와 W0–W5 및 D/A/B/C의 연결. 과거 상태 문구는 당시 기록으로 읽음 |
| 원 요구사항·원 WBS·설계 동기 | 당시 참조한 `study/docs/reviews/knowledge-system/`의 `spec-driven-requirements.md`, `spec-driven-wbs.md`, `ckv-ckg-ontology-installation-proposal.md`는 현재 로컬에서 미확보 | 원문을 직접 확인했다고 주장하지 않음. N-01로 복구/완전성 재확인 |
| 더 오래된 CKV 계획 | [2026-05-29 CKV 리팩토링 계획](../vector/archive/plan-2026-05-29-ckv-refactor.md)은 2026-07-19에 실행된 계획으로 보관 표시 | 이번 9월 spec-driven 계획과 같은 작업리스트로 합산하지 않음. 이전 도메인 검색 도구 확장의 배경 |

원본 3개는 `/Users/kevin/work` 파일 검색(숨김/ignore 제외 해제, Git 내부·node_modules 제외), 현재 제품 저장소의 보유 Git 참조, `/Users/kevin/work/github/0xmhha/study`의 로컬 트리·보유 참조 이력에서 찾지 못했다. 원격 fetch는 수행하지 않았다. [원본 탐색 기록](../../system/eval/b0-knowledge-system/original-goal-reaudit-m2max-2026-10-06/original-source-discovery.json)을 보존했다. 따라서 아래는 **확보한 자료로 확인 가능한 잔여 목록**이며 v0.1 원문 전체의 누락 없음까지 인증한 목록은 아니다.

## 2. 원래 목적과 현재 달성 범위

| 원래 목적 | 현재 코드/실행 증거 | 목적 달성 상태 |
|---|---|---|
| 코드 구조·벡터·문서·의미를 같은 프로젝트/소스/데이터셋으로 연결 | `internal/setup/identity.go`, `capture.go`, `source_read.go`; 세 엔진 신원·보관 원문·작업 트리/비Git·구버전 검사 | 기본 경로 구조 구현/검증 완료. 모든 유지보수 쓰기 경로의 통일·내구성은 남음 |
| 필터·장문·기권·스냅샷 정확성과 검색 품질 개선 | `internal/vector/store/sqlitevec/store.go`, `internal/system/evidencev2/pack.go`; DEV3840/FINAL4800·raw exact·장문/상태 검사 | 구조 개선과 실측 완료. 낮은 회수율·기권 guard·본문 예산 실패는 미해결 |
| 출처를 가진 의미·공통20개·조직 팩으로 안전한 “왜” 제공 | `internal/system/semantic`, `knowledgepack`, `evidencev2/knowledge*`; 네 mode/팩8arm·검토/권한 경계 | 선택형 구조 구현. 실제 파일럿20개 개념·3개 요구사항은 proposed; 실제 내용 승인과 효용은 미완료 |
| 요구사항→기준→코드→테스트→패치/사람 수용 연결 | `internal/system/patch/patch.go`, `review.go`, `semantic/test_run.go`; 실행 성공과 기준 승인 분리 | 장치와 합성 회귀 구현. 실제 파일럿의 내용·수용 경로 및 실제 변경 성공 사례는 별도 증거 필요 |
| 임의 프로젝트에 설치·업그레이드·복구 가능한 배포물 | `cmd/cks/setupcli`, `packagecli`, 최신3대상 preview·mock/실모델·복구 원장 | 시험 preview 검증 완료. native AMD64·실운영 비용/복구·키/신뢰·적합성·운영 출시는 미완료/미승인 |
| 측정 결과로 리팩토링한 뒤 기본값/출시 결정 | [DEV 보고](./B0-B1-APPROVED-DEVELOPMENT-REPORT.md), [FINAL 보고](./C1-APPROVED-FINAL-REPORT.md) | FIX22–28 수정·평가·판정 완료. 품질 fail/소표본 inconclusive로 disabled·출시 보류. 품질 개선 목표 달성으로 읽지 않음 |

## 3. 보존된 W0–W5와 코드의 세부 대조

원본 WBS 3개 문서는 미확보다. 아래 **29개 ID**는 현재 `EXECUTION.md`에 보존된 W0–W5 행과 그룹 행을 펼친 관찰 단위다. 원본 v0.1 WBS의 총 작업 수라고 단정하지 않는다. “구조 완료”는 내용 승인·품질 합격·운영 완료와 별개다.

| ID | 목적/현재 코드의 핵심 | 확인한 상태 | 잔여 연결 |
|---|---|---|---|
| W0.1 | 실모델 기준선·평가: `internal/system/eval`, `cmd/cks/evalcli` | BGE 고정 DEV/FINAL 실측 완료, 결과 fail/불확실 | N-08–11 |
| W0.2 | 현재 File 감사: `internal/graph/persist/sqlite_reader.go:81` | 역사 Hunk 제외·실제 strict 범위 감사 증거 있음 | 새 수정 뒤 회귀 유지, 신규 전면 재구현 없음 |
| W0.3 | 계층 정렬·다중 프로젝트: `internal/setup/input_reconcile.go`, `identity.go` | strict1598/13820·프로젝트/상태 격리 구조 검증 | N-02–04, N-15 |
| W0.4 | 무인용 평가: `internal/system/eval/scenario.go`, `runner.go` | 평가 guard 구현; 실제 두 기권 문항 guard 실패 | N-09, N-11 |
| W1.1 | 필터 계약: `pkg/vector/types/search.go`, sqlitevec store | 필터 exact/incomplete 계약 구현·raw 오라클 | N-10–11 실부하 |
| W1.2 | 희소/큰 후보 검색: sqlitevec `SearchResult` | 후보 상한/취소/incomplete·F01 자격 검증 | N-08, N-10–11 |
| W1.3 | 검색 지연 | 실측 있음; dynamic 일부 p95비>1.25 | N-10–11 |
| W1.4 | 장문 청크: `internal/vector/chunk`, `parse/markdown` | 꼬리·부모/자식·원문 범위 보존 검증 | N-07, N-11 |
| W1.5 | 문맥 조립: `internal/system/composer/budget`, evidencev2 | 선택 본문/보관 원문 구현; FINAL33104>32000 오류 | N-07 |
| W1.6 | 검색/근거/기권 상태와 품질 | 상태 보고·실패 분모 보존 구현; 품질 미달 | N-08–11 |
| W1.7 | 실제 무답/충돌·후속 주장 판정 | HC11개 해석과 사람 기권2개 승인; 생성 답변을 평가한 것은 아님 | N-09, N-12, N-14 |
| W2.1 | 출처 기반 의미: `semantic/model.go`, `validate.go`, `store.go` | 불변 투영·보관된 외부 문서 출처 구조 구현 | N-12 실제 지식 |
| W2.2 | 문서/CKV/코드 앵커: `extract_markdown.go`, `chunk_links.go`, `ckg_anchor.go` | 정확 줄/해시·canonical 연결 구조 구현 | N-12, N-14 |
| W2.3 | claim/관계 검토: `semantic/review.go` | 검토 장치 구현·합성/HC 기록 있음; 실제 추출 대표성/오차/비용 미확정 | N-12 |
| W2.4 | 세 계층·외부 원문·mutable 신원 | 신원/보관/canonical 연결 구조 구현 | N-02–06, N-15 |
| W3.1 | 공통 온톨로지 내용 | 설계20개 승인; `ontology-pilot.yaml` 개별20개는 proposed | N-12 |
| W3.2 | 타입/관계·팩: `semantic/core_ontology.go`, `knowledgepack` | 20개/관계·팩 잠금/권한/검토 구조 구현 | N-12–13 |
| W3.3 | 의미 검색/텍스트 투영: `store_terms.go`, `text_corpus.go` | 결정적 투영·lookup 구현; 전체 투영 재검증 비용의 실부하 효용 미확정 | N-10–12 |
| W3.4 | 선택형 runtime: `cmd/cks/mcpcli/ontology.go` | 네 mode/팩8arm 구현. 기본 상위 K 안 boost, opt-in | N-08, N-10–12 효용 |
| W3.5 | 실제 온톨로지 품질·비용 | 실측 완료; 일반 개선 증명 없음·소그룹inconclusive | N-08–12, N-18 |
| W4.1 | 스펙 입력: `semantic/extract_spec.go` | 입력/검증 구현; 실제 `spec-pilot.yaml` 요구사항3개 proposed | N-12, N-14 |
| W4.2 | reviewed trace: `semantic/trace.go`, `trace_anchor.go` | 구조·stale/충돌/승인 분리 구현. F06 완전 경로 없음 | N-14 |
| W4.3 | 변경 계획/팩 주석: `semantic/plan.go`, `pack.go` | 읽기 전용 계획·unconfirmed·근거 범위 유지 구현 | N-14–15 실제 적용/소비자 |
| W4.4 | 정확 테스트·외부 패치: `semantic/test_run.go`, `internal/system/patch` | 패치 신원·observed Go test·사람 기준 승인·승격 경계 구현 | N-14 실제 의미 수용 |
| W5.1 | 패키지·서명·고지: `scripts/package-host.py`, `cmd/cks/packagecli/verify.go` | 세 대상 시험 서명 preview 검증 | N-15–18 |
| W5.2 | init/doctor/status: `cmd/cks/doctorcli` | 비덮어쓰기·역량/신원 상태 구현. 보관 용량/GC 상태 없음 | N-05–06, N-15 |
| W5.3 | 임의 프로젝트/작업 트리 신원: setup capture | committed/working-tree/snapshot-only 구조 구현 | N-02–06 |
| W5.4 | 업데이트/rollback/pin/복구 | mock/보관본·구v1·실모델 일부 증거 확보; ops.index는 직접 쓰기 | N-02–04, N-17 |
| W5.5 | 다중 프로젝트 설치와 출시 | 시험 설치/복구 완료. 실제 운영 대상/배포 적합성은 별도 | N-15–18 |

## 4. 이번 코드 대조로 확인한 설계 차이

[정적 감사 원장](../../system/eval/b0-knowledge-system/original-goal-reaudit-m2max-2026-10-06/code-and-source-audit.json)에 파일 SHA·관측 줄·최초 문서 snapshot을 연결했다. 아래 신규 설계 차이는 코드 읽기로 확인한 것이며 장애를 새로 주입해 재현했다는 뜻은 아니다.

1. `END-TO-END-DESIGN`의 v2 쓰기 통일과 달리 `internal/system/mcp/ops_index.go:175` 부근은 활성 인덱스 직접 갱신·이전 버전 없음이라고 명시한다. pinned 경로의 거부/후보 빌더 연결과 legacy 호환 경계를 다시 정해야 한다(N-02).
2. 설계는 빌드/승격/rollback을 OS 잠금으로 직렬화하며 나이 기반 탈취를 v2에서 쓰지 않기로 했다. 실제 `Reindex`는 `.reindex.lock` 독점 생성/PID/6시간 회수, 승격만 `flock`이다(N-03).
3. 설계의 checkpoint·close·파일/디렉터리 sync 후 pointer 전환과 비교하면 승격은 symlink rename 뒤 부모 디렉터리 sync가 없다. identity 임시 파일의 Sync는 있으나 후보 전체와 pointer의 전원 차단 내구성을 증명하지 않는다(N-04).
4. 설계 §2.2/§8의 참조 보호 GC·보관 용량/기간 보고는 현재 CLI/doctor에 없다. 현재 원문을 자동 삭제한다는 뜻은 아니며 안전한 회수 기능이 아직 없다는 뜻이다(N-05).
5. 캡처 기본100000파일/32MiB/4GiB는 구현돼 있지만 setup은 해당 상한 조절을 전달하지 않으며 기본2시간 build deadline도 없다. history bundle 상한만 CLI로 노출된다(N-06).
6. 실제 FINAL 본문 예산 오류의 코드는 여전히32000바이트 초과를 정상 팩 없이 거부하고 MCP는 generic error로 변환한다. 새 DEV에서 bounded 선택/명시 상태를 먼저 설계해야 한다(N-07).
7. 공통 엔진의 `domaincli/worksheet.go`에 StableNet catalog가 여전히 하드코딩돼 있다. 기존 living backlog D-2와 D5의 공통/조직 팩 분리 목적에 연결한다(N-13).

## 5. 품질·검토·운영의 실제 잔여

- FINAL4800 중240개 오류, 정상팩4560. 양성 정적6문항 Recall1/6·MRR0.5/6; 모든8arm 정적 점수 동일. DEV 기대 source의 primary exact 순위는 CODE1085·POLICY47/52·TRACE301/1490였다. 기본 상위K 안 rerank만으로 들어오지 않은 기대 근거를 복구할 수 없으므로 후보 회수/조립 원인을 새 DEV에서 구분해야 한다.
- ABS01/02의 strict no-citation guard 실패는 사람 답변 보류2/2 승인 후에도 남는다. 제품은 EvidencePack을 반환한다. 생성 답변을 만드는 외부 coding-agent의 품질/총비용까지 측정한 것은 아니다.
- 중요 FINAL 질문군의 독립 수는각10미만이다. 반복·언어·상태 변형과 HC11문장으로 표본을 늘리지 않는다. 현재 FINAL은 이미 관측됐으므로 개선 버전의 새 독립 holdout으로 쓸 수 없다.
- FINAL dynamic 일부 비용비1.533/1.311/1.511과 같은 ontology on/off 비1.331/1.288/1.293은 경계1.25를 넘는다. 최신 패키지의 선택적 neighbors 오류360/별도RSS12/Linux6도 설명/개선 대상이다. 일괄적인 backend 성공으로 해석하지 않는다.
- 공통20개 설계 승인, SF 테스트 사례 승인, HC 해석 승인은 각각 유효하다. 실제 파일럿 개념20개/요구사항3개·운영 정책/ADR/criterion의 내용 승인을 대체하지 않는다. 실제 사람 검토 비용은 미측정이다.
- LinuxAMD64는 에뮬레이션이며 native 실모델/운영 환경이 없다. 운영 키·신뢰 경로·기간/폐기·실제 역할·native/전이 고지 적합성·운영 backup/failover/RTO/RPO는 이전 preview 종료에서 제외됐고 이번에도 자동 승인하지 않는다.

## 6. 결론과 다음 단계

구조 리팩토링과 시험 preview는 큰 범위에서 구현/검증됐고 실모델 평가를 수행했다. **원래 목적의 달성 상태는 구조 구현과 품질/내용/운영으로 나눠 평가해야 하며, 전체 달성률을30/30에서 환산할 수 없다.** 이번 감사는 기존 완료를 blanket 취소하지 않고 확인된 설계 차이·미달·미검증을 [N-01–18](./REFACTORING-REMAINING-WORKLIST.md)에 기록한다.

현재 단계는 원래 목적 대조와 잔여 목록 작성 완료다. 다음 구현 우선순위는 N-02 활성 쓰기 경로의 실패 재현/설계이며 N-03/04의 잠금/내구성 검증과 N-07의 bounded evidence를 이어서 진행한다. 원본 복구와 신규 DEV/FINAL 프로토콜 준비는 독립적으로 병행할 수 있다. 이번 요청으로 배포·새 push·파일럿 사실 승인을 만들지 않는다.
