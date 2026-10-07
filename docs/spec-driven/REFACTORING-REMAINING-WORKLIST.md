# 원래 리팩토링 목적 기준의 남은 작업리스트

2026-10-06 · 기준 코드 `04d3bfdf` · [원래 목적/코드 대조](./REFACTORING-ORIGINAL-GOALS-REAUDIT.md). 이전 [시험 preview30항목](./EXECUTION-WORKLIST.md)은 사용자 선택 범위에서 종료됐다. 이 문서는 원래 목적과 코드 사이의 잔여를 관리하는 **새 후속 목록**이다. 이전 HC·입력 승인은 다시 요구하지 않는다.

후속 목록 **18개 중 N-01–07 및 N-13 완료, 잔여10개(N-08–12, N-14–18)**. 제품 구현을 새로 완료한 항목은 N-02–07/N-13 7개다. N-15–18의 실제 운영 범위는 이전에 제외됐으며 목록 작성만으로 수행·배포가 승인된 것은 아니다. `study/main` 동기화로 초기 원문3개와 아키텍처/분석을 확보하고 [전체 초기 ID·v2 대응](./REFACTORING-ORIGIN-AND-V2-TRACE.md)을 확인했다. 이 목록의 크기를 원래 전체 목표의 완료율로 환산하지 않는다.

[study 보완 감사](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md)에서 더 오래된 목적 원문과 2026-06-26 마스터 계획을 확보했다. **상위 WI10개는 별도 추적 대상**이며 이 제품의18개와 합산하지 않는다. 상위 범위 전체의 완료 판정에는 coding-agent/ChainBench/별도 학습 저장소의 현행 코드·실행 증거 대조도 필요하다.

[후속 실행 명세·완료 판정 절차](./REFACTORING-EXECUTION-SPEC.md)에 목적·설계·수용 조건별 검증을 기록한다. N-02–07/N-13의 수용 조건별 실행 증거를 대조하고 완료했다. [진행 원장](./REFACTORING-PROGRESS.json)과 `python3 scripts/check-refactoring-progress.py`는 완료 체크/커밋 증거/현행 소스/잔여수 일치를 자동 검사한다.

## 상태와 증거 규칙

- **코드 차이 확인**: 현재 코드가 설계와 다름. 새 장애 주입/회귀 재현 전이다.
- **실패 실측**: 보관된 DEV/FINAL 원자료에 실패/회귀가 있다. 이미 관측한 FINAL은 새 독립 holdout으로 쓰지 않는다.
- **내용/환경/운영 대기**: 도구가 있다고 실제 사람 판정·native 실행·운영 사실을 채우지 않는다.
- 완료는 해당 행의 수용 조건·코드/입력/실행/원자료·제한을 연결했을 때 기록한다. 새 수치 목표/프로토콜은 실험 결과를 보기 전에 고정한다. 기존 안전/좌표/비밀 혼입0·기권 오인용 증가0·Recall/MRR 변화−0.02/중요군−0.05·p95비1.25·소표본 규칙은 임의 변경하지 않는다.

## 남은 전체 목록

| ID | 우선순위·선행 | 상태·현재 근거 | 해야 할 작업 | 완료 조건 |
|---|---|---|---|---|
| N-01 | 완료 · 2026-10-06 | study/main `1c350c1` 원문3개·초기architecture/analysis 확보 | 초기 작성/보관 시점·SHA와 시작 배경 기록. FR10/INV7/NFR5/S9·WBS29를 v2/현행 감사와 대조 | [복구·대응 원장](../../system/eval/b0-knowledge-system/original-source-recovery-2026-10-06/recovered-source-and-trace.json). 기존 탐색 증거 보존. PDF 원본 재검증·상위WI 외부 구현 감사는 별도 |
| N-02 | **완료 · 2026-10-06** | [실행 명세·검증](./REFACTORING-EXECUTION-SPEC.md), 공개 MCP RED→GREEN/CLI 7사례 | pinned v2에서 활성 DB를 직접 수정하지 않게 거부/후보 빌드 경로로 통일. legacy 유지보수 동작과 migration 경계 명시 | 한 엔진 실패/혼입/취소에도 이전current·신원·원문 보존; 실제 공개 MCP와 versioned setup 경로의 동일 후보/승격 증거. [새 DEV 원자료](../../system/eval/b0-knowledge-system/refactoring-n02-2026-10-06/cli-results.json); 동시 writer/내구성은 N-03/04에 유지 |
| N-03 | **완료 · 2026-10-06** | [공통 OS 잠금 검증](./REFACTORING-EXECUTION-SPEC.md), legacy live PID·8process 경합/SIGKILL·CLI 충돌 | v2 빌드/승격/rollback의 OS 잠금 계약 정리·구현. 살아 있는 긴 빌드의 age reclaim·동시 회수 경합을 재현 | 긴 live holder/동시 요청/owner crash에서 두 writer 없음; PID·나이로 살아 있는 잠금 탈취 없음; legacy 호환 정책 기록. [DEV 원자료](../../system/eval/b0-knowledge-system/refactoring-n03-2026-10-06/cli-results.json); 구 writer 중단 후 전환·local flock 경계 |
| N-04 | **완료 · 2026-10-06** | [내구성 DEV 증거](../../system/eval/b0-knowledge-system/refactoring-n04-2026-10-06/manifest.json), WAL·linked RED→GREEN·8+5 실패 경계·7 SIGKILL·CLI 7+2 | DB checkpoint/close·source/manifest/candidate sync·current 전환/부모 sync 순서와 crash recovery 계약 보강 | 전환 단계별 실패/강제 종료 주입으로 이전 또는 완성된 새candidate만 관측. 전원 차단 검증/한계는 플랫폼별 구분 |
| N-05 | **완료 · 2026-10-06** | [DEV 검증](../../system/eval/b0-knowledge-system/refactoring-n05-2026-10-06/cli-results.json), 공개 CLI 12사례·live MCP reader·stale/취소/2 process crash | 활성·rollback·실행 중 pinned reader·검토 대기 candidate 참조를 보호하는 GC dry-run/실행과 용량/보관 정책·status 보고 | 회수 계획 결정적·참조 중 blob/이력/의미 원문 삭제0; 경합/취소/복구 시험; 실제 삭제 전에 검토 가능한 dry-run |
| N-06 | **완료 · 2026-10-06** | [새 DEV 공개CLI12사례](../../system/eval/b0-knowledge-system/refactoring-n06-2026-10-06/cli-results.json), 상한/외부origin·deadline/취소·공개DTO·race 검증 | 파일 수/단일/총 바이트 상한을 setup/config로 전달, build deadline·용량 예상/여유 공간·공개 resource 상태 구현 또는 계약 수정 검토 | 기본100000/32MiB/4GiB·계약2h와 설정값 결합, 초과/timeout에서 candidate 실패/이전current 보존, 민감 path 비노출 |
| N-07 | **완료 · 2026-10-06** | [새 DEV RED→GREEN](../../system/eval/b0-knowledge-system/refactoring-n07-2026-10-06/manifest.json), public RPC partial/typed error·13패키지 race | 32000바이트/12인용 경계 안의 보관 근거 선택/부분 상태 및 안전한 typed budget 오류를 설계하고 독립 DEV 재현 후 수정 | 본문/좌표/해시 훼손 없이 상한 준수; 오류·부분·정상 분모 유지; source_missing/mismatch 구분·권한 누출0; 원 FINAL 불변 |
| N-08 | P1 · N-11, 필요 시N-12 | 실패 실측: FINAL 정적 양성Recall1/6·8arm 동일 | CODE/WHY/POLICY/TRACE 기대 근거가 후보/조립에서 빠지는 원인 분리. 새 DEV에서 primary recall·청크·lexical/graph 조합 개선 | 질문/gold hardcode 없음; 원문/프로젝트/모델 신원·기본 안전 유지; 사전 승인 품질/회귀·중요군 수용 기준으로 개선 확인 |
| N-09 | P1 · N-11 | 실패 실측: ABS01/02 strict no-citation fail | 무답의 검색 근거 상태와 downstream 답변 보류 계약을 명확히 하고 부적합 인용/무근거 운영 추정을 차단 | 승인된 무답/답가능 대조 사례에서 guard 검증; 사람 기권/기계 무인용 각각 집계; 생성 답변 평가가 없으면null |
| N-10 | P1 · N-11, N-07/08 영향 | 실패 실측/진단: 일부p95비>1.25·neighbors 오류; [typed 세부 진단](./N10-NEIGHBOR-DIAGNOSTICS.md) 검증, 전체 미완료 | model/backend/원문 검증/의미 투영 비용 분해, 비반환 neighbors 오류 원인·영향 확인, 범위 내 비용/오류 개선 | 같은pack·같은ontology·static/dynamic/pooled·warm/cold·성공/오류 분모 유지; 실제K/MID/호출/크기와 회귀1.25 검증; 운영 부하는별도 |
| N-11 | **P1 · 개발 튜닝 전에 준비** | 현FINAL 소그룹n<10·이미 관측 | 새 DEV 버전/질문과 결과 사전 비공개 독립 FINAL, 중요군 표본·평가 단위·정답·임계치·회전/비용 조건을 사전 검토·동결 | 충분한 독립 중요군으로 판정하거나 여전히inconclusive/disabled 명시. 반복/언어/상태/HC 문장 표본 부풀림0·이전FINAL 튜닝/holdout 재사용0 |
| N-12 | P1 · 실제 내용 검토 | 사람 판정 대기: [실제 검토 자료](./N12-ACTUAL-PILOT-REVIEW.md);27원문span/6테스트 및 [정책·ADR 보완](./N12-POLICY-ADR-REVIEW.md)의6주장/12관계·4거부 제어 확인, proposed 유지 | 실제 파일럿 concept/term/claim/정책/ADR/요구·기준과 코드 앵커를 검토. 대표 추출 오차/관계 오용/검토 시간·비용 기록 | 실제 사람 승인/수정/기권·근거 원문SHA/권위/범위/시점과 새lock/snapshot 결합; HC/SF 승인 재사용으로 운영 사실 승인하지 않음 |
| N-13 | 완료 · 2026-10-06 | [N13 DEV manifest](../../system/eval/b0-knowledge-system/refactoring-n13-2026-10-06/manifest.json) · CLI11회/기존 파일80개 변경0/관련5패키지 race | 조직 catalog를 명시 선택형 pack 자료로 분리; 생성기의 허위 앵커 검사 완료 문구와 구 CLI 명령 교정 | 무팩/구팩·8매핑/필터/앵커/빈queue; pin/의존/type/출처/경로 검증; 기존 출력 보호·상태 자동 승격0 검증 |
| N-14 | P2 · N-12 | 구조 구현·실제 의미 수용 증거 필요 | 실제 파일럿 ADR→requirement→criterion→code→test 연결, 변경patch→실제run→사람 수용→새색인/승격·거부/rollback 사례 검증 | 테스트pass/linked/verified/accepted 각각 기록, 무관test·정책/구현불일치·stale로 허위승격0; 실제 검토 계획/수용 원장 |
| N-15 | P2 · N-02–14 중 변경 영향 | 환경/외부 소비자 미검증; 이전preview 제외 | 대상 지원을 확정하고 native LinuxAMD64·실운영 규모/모델·외부 CLI/MCP/daemon 및 실제 프로젝트 설치/upgrade 검증 | OS/CPU/binary/source/모델 pin과 native/에뮬 구분; 실제지원/preview/미지원 명시. coding-agent 총비용은 외부별도측정 |
| N-16 | P2 · 운영 출시 범위 결정 | 운영키·법무 미승인; test-preview 계약만 | OP01–06 역할/권한·scope/schema·public key/독립 신뢰·보관/교체/폐기·native/전이 고지 적합성 검토 | 실제 권한자/공개키fingerprint/신뢰경로·결정시각/근거, 운영계약 회귀, 정확고지범위·담당 적합성 판정; 개인키 본문은 저장하지 않음 |
| N-17 | P2 · N-15/16 | OP07–08·실운영복구 미실행; 이전preview 제외 | 실제 backup/failover/복구 담당·RTO/RPO/재시작/키 복구를 정하고 실훈련. network agent/전원 정책의호스트상태 확인; AC-only 정책 범위 검토 | 최신 운영후보/데이터로 복원·이전current/인용/키 신뢰 재생; 실제 실행·한계/담당 기록. 원격운영 상태를 로컬code만으로 완료하지 않음 |
| N-18 | **P3 · N-08–17/변경 회귀** | 품질fail·소그룹inconclusive·disabled·release withheld | 새 독립평가·원래회귀·사람비용/내용·대상지원·운영결정으로 기본값/출시 판정. 릴리스 문서/리뷰/merge/publish 준비 | 합격 근거 없으면disabled/보류 유지. 실제 merge/push/배포는 구체 산출물과 별도권한으로 실행; preview종료 승인을 운영권한으로 확장하지 않음 |

## 실행 순서와 우선순위의 이유

1. **N-02→N-03→N-04**: 새 검색 실험 전에 활성 데이터/동시 writer/복구 기반을 보호한다. N-05/06은 이 기반 위에 둔다.
2. **N-07**, **N-11 준비**: 이미 실측된 본문 예산 오류를 새 DEV 실패 시험으로 연결한다. 새 품질 목표·분할·표본은 결과를 보기 전에 검토한다.
3. **N-12/13→N-08/09/10→N-14**: 실제 지식과 원문 검색을 구분해 품질/기권/비용을 개선하고 실제 기준 수용 경로를 검증한다. 사실 승인 대기 중 구조·시험 준비는 진행 가능하다.
4. **N-15→N-16/17→N-18**: 운영까지 요구하는 경우 환경·키/신뢰/적합성·복구를 충족한 뒤 기본값/출시를 판정한다. 실패면 승인된 범위의 보고/보류로 종료할 수 있으나 운영 출시 완료라고 표시하지 않는다.
5. **N-01 완료**: 원본 복구와 전체 ID 대응에서 새 필수ID 누락은 찾지 못했다. 원래 목적의 품질·검토·비용·설치 조건은 기존 잔여에 유지하고 확인된 코드 차이의 재현을 이어간다.

## 기존 승인과 범위 유지

정적12/BGE·Go29/프로토콜/F01–06/SF 테스트 사례/STV2·HC01–11 해석 승인과 시험 preview 종료 결정은 유효하다. 새 목록 때문에 그 승인을 취소하거나 같은 질문을 다시 요청하지 않는다. 새 실제 파일럿 사실, 새 사전평가 입력/목표, 실제 운영 역할/키/지원/법적 적합성은 별도 대상이다.

GraphRAG 커뮤니티, RDF/OWL/SHACL, 물리 컬렉션 분할, 긴 함수 overlap 조정은 [PDF 추적표](./PDF-IMPROVEMENT-TRACE.md)의 조건부 연구 항목이다. 필요성/수용 조건 없이 필수 미완료로 추가하지 않았다. 이전 May CKV 계획이나 다른 저장소의 전체 coding-agent/ChainBench 작업도 이번 목록에 자동 편입하지 않았다.

**현재 단계:** N-01–07 및 N-13 완료(8/18), 요청15개 중5개 완료. **다음 작업:** N-11 새 사전평가 입력·프로토콜과 N-12 실제 내용 검토 판정. **남은 전체:** N-08/09/10/11/12/14/15/16/17/18, 10개. 각 실제 수용 조건이 충족되기 전에는 완료 처리하지 않는다.

**상위 목표의 추가 추적 전체:** WI-V1 오라클 타당성, WI-V2 수정안 선택, WI-3 의심 라벨링, WI-4 비결정 버그, WI-1 값 흐름, WI-2 규칙 태깅, WI-C 깊은 코드 리뷰, WI-6 값 흐름 기반 다단 확장, WI-SP 정책/보안패턴 노출, WI-5 별도 사람 검토 학습. 각 상태/보류/담당·제품 N 연결은 [복구한 상위10개 표](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md#3-복구한-상위-작업계획의-전체10개-항목)를 기준으로 한다. 다음 상위 감사는 담당 저장소와 현재 WI-V1/V2/3 증거 확인이다.

N11 [새 DEV7개/프로토콜 초안](./N11-FRESH-EVALUATION-PREPARATION.md)은 미승인/미실행이며 새 FINAL0이다. N12 실제20타입/3요구/4기준 검토 자료를 요청했고 사람 판정·활동 시간 및 운영 환경/담당 입력을 기다린다. 준비 자료를 완료로 체크하지 않았다.

N12 정책·ADR 원문 주장이 준비 범위에서 누락된 것을 재감사로 발견해 [별도 검토 자료](./N12-POLICY-ADR-REVIEW.md)로 보완했다. 원문 커밋ab77ce56,6주장/12관계 proposed, 원문span 대조 및 실제 CLI4개 음성 제어 통과. 기존27개 사람 검토 요청은 보존한다. 새 주장/관계 승인0, 통합 lock/canonical·의미 수용 미완료이므로 N12 완료 체크와 전체 수는 바꾸지 않는다.

N11-C [실행 보호 세부 단계](./N11-EVALUATION-RUNNER-CONTRACT.md)를 구현·검증했다. 승인된 전체 책/runtime pins 검사와 FINAL durable reservation/영구 OS 잠금을 matrix 경로에 연결했다. Python19개·관련2패키지 race·native CLI4제어 통과, 실제 초안 child dispatch0/질의0. 새 사람 판정과 실제 평가 결과가 없으므로 N11 및 전체 완료 수는 그대로다.

N10 진단의 구조 RED→GREEN과 실제 SQLite3제어를 추가했다. 미해결 seed typed 표시를 추가하되 backend_error/모든 시도·composer 결과를 보존하고, 과거 정보가 없으면null이다. Go3패키지 race/vet·Python17개 통과. 비용/p95 새 평가가 없어 완료8/18·요청5/15·잔여10을 유지한다.

N11 [부분 FINAL 후보](./N11-FINAL-CANDIDATE-PREPARATION.md)4문항을2제안 사실군으로 묶고 원문·span SHA에 연결했다. 원 DEV7개와 프로토콜 바이트는 불변, 승인된 독립 FINAL0·실행/예약0이다. 전체 입력 검사 exit2, 미승인/미동결 및 중요군 미충족을 유지한다. 후보 작성은 전체 N11 완료가 아니다.

2026-10-07 N15/N17 [현행 패키지·외부 모듈·복구 세부 단계](./N15-N17-CURRENT-TECHNICAL-VALIDATION.md): clean native Darwin 패키지/3mock 프로젝트/외부 Go 공개 계약7응답/4거부 제어, 새 root 복원 reader lease 결함 수정·9제어·3패키지 race·복구10응답/13payload SHA 통과. native Linux CI는 정의만 존재, 실제 BGE/외부 운영 소비자·고지/권한/RTO/RPO 수용은 미완료다. 전체8/18·요청5/15·잔여10개를 유지한다.

2026-10-07 native CI 사전 검사: 전역 formatter의 동결 평가 원문3개 변경 요구를 관리 코드/fixture 범위로 제한했다. 5제어와 make fmt-check 통과, 원문/Go runtime 소스 변경0. 전체 완료 수와 CI 미실행 상태는 동일하다.
