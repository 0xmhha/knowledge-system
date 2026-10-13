# 원래 리팩토링 목적 기준의 남은 작업리스트

**2026-10-13 현재: 기존17/18·온톨로지4/4 완료. 전체 잔여1개는 N16 보류(미충족)다.** [O04 최종 판정](./ONTOLOGY-FINAL-VERDICT-2026-10-13.md)은 품질inconclusive·독립승인0·기본off다. PR #98의 과거 main 머지는 완료됐지만 이번 feat/setup-tool 브랜치는 push·[일반 PR #99 생성](https://github.com/0xmhha/knowledge-system/pull/99)을 완료했다. 최초 공개 HEAD `82dbb684`에서 필수 CI는 Go/린터 버전 불일치로 실패했고 권고용 취약점 검사도 실패했다. 별도 전달 잔여는 CI 수정·재검증, 취약점 검토, PR 리뷰이며 이번 main merge는 미진행이다. 태그·공개 배포는 보류한다.

**별도 전달 작업 진행:** [CI 수정·검증](./CI-TOOLCHAIN-VALIDATION-2026-10-13.md). Go1.26.9 고정·정리 린트 오류2개 수정과 로컬 검사는 통과, 전체 race·수정 후 GitHub CI는 확인 중. 미호출 모듈 취약점2개를 기록했으며 PR 리뷰·N16 보류·main merge 미진행·배포 보류를 유지한다.

이 문서는 현재 작업 표를 유지한다. [현재 인수인계](./CURRENT-HANDOFF.md)에서 읽기 순서를 확인하고, 이전 단계의 “최신/현재” 기록은 [날짜별 이력](./REFACTORING-STATUS-HISTORY-2026-10-13.md)에서 읽는다. 초기 목적과 상위 WI 범위는 [원래 목표 감사](./REFACTORING-ORIGINAL-GOALS-REAUDIT.md), [study 보완 감사](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md)를 따른다. 기존 preview와 상위 WI를 이번 완료율에 합산하지 않는다.

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
| N-08 | **완료**(개선 근거 없음·기본 비활성) · 2026-10-11 | [FINAL2](./FINAL2-RESULT.md): recall 동일, MRR −0.032(삽입 선언이 gold 위 5문항, 개선0). 사용자 결정으로 삽입을 opt-in(`retrieval.type_neighbors`, 기본 off)으로 전환, 기본 인용은 이전 binary와 7/7 동일. 품질 개선은 주장하지 않음 | CODE/WHY/POLICY/TRACE 기대 근거가 후보/조립에서 빠지는 원인 분리. 새 DEV에서 primary recall·청크·lexical/graph 조합 개선 | 질문/gold hardcode 없음; 원문/프로젝트/모델 신원·기본 안전 유지; 사전 승인 품질/회귀·중요군 수용 기준으로 개선 확인 |
| N-09 | **완료**(기계 동작 확인·inconclusive) · 2026-10-11 | [FINAL2](./FINAL2-RESULT.md): 무답 on arm 240/240 `declared_unknown`·근거 외 인용0, 대조·DEV 잘못된 기권0/600. 무답2단위 inconclusive, 답변 수준 기권 null. 사용자 결정으로 종료, 기본 활성·출시 근거 아님 | 무답의 검색 근거 상태와 downstream 답변 보류 계약을 명확히 하고 부적합 인용/무근거 운영 추정을 차단 | 승인된 무답/답가능 대조 사례에서 guard 검증; 사람 기권/기계 무인용 각각 집계; 생성 답변 평가가 없으면null |
| N-10 | **완료** · 2026-10-11 | [FINAL2](./FINAL2-RESULT.md): 후보 FINAL warm p95비 1.039–1.109(참조 1.334–1.873), 오류0. 검색 차이는 모두 N08 삽입이며 N10 자체 회귀 없음. A·B·D는 [N11 FINAL](./N11-FINAL-RESULT.md) | model/backend/원문 검증/의미 투영 비용 분해, 비반환 neighbors 오류 원인·영향 확인, 범위 내 비용/오류 개선 | 같은pack·같은ontology·static/dynamic/pooled·warm/cold·성공/오류 분모 유지; 실제K/MID/호출/크기와 회귀1.25 검증; 운영 부하는별도 |
| N-11 | **완료 · 2026-10-10(inconclusive/disabled)** | [DEV3·FINAL 결과](./N11-FINAL-RESULT.md): 동결 입력·DEV3 1,680행·FINAL 2,400행 오류0, 모든 중요군 단위10 미만 | 새 DEV 버전/질문과 결과 사전 비공개 독립 FINAL, 중요군 표본·평가 단위·정답·임계치·회전/비용 조건을 사전 검토·동결 | 충분한 독립 중요군으로 판정하거나 여전히inconclusive/disabled 명시. 반복/언어/상태/HC 문장 표본 부풀림0·이전FINAL 튜닝/holdout 재사용0 |
| N-12 | **완료 · 2026-10-09** | [결합·활성화 기록](./N12-REVIEWED-ACTIVATION.md): AI 사전검토 65건을 사용자가 일괄 채택하고 AI 검토 라벨을 지정했다. 독립적인 사람의 항목별 내용 검토는 없었고, 사람 활동 시간도 미측정이다 | 실제 파일럿 concept/term/claim/정책/ADR/요구·기준과 코드 앵커를 검토. 대표 추출 오차/관계 오용/검토 시간·비용 기록 | 사용자 일괄 채택과 지정 라벨로 완료 처리한 범위를 명시한다. 원문SHA/권위/범위/시점 및 새 lock/snapshot 결합 근거를 보존한다. 독립 사람 검토 완료로 확대 해석하지 않는다 |
| N-13 | 완료 · 2026-10-06 | [N13 DEV manifest](../../system/eval/b0-knowledge-system/refactoring-n13-2026-10-06/manifest.json) · CLI11회/기존 파일80개 변경0/관련5패키지 race | 조직 catalog를 명시 선택형 pack 자료로 분리; 생성기의 허위 앵커 검사 완료 문구와 구 CLI 명령 교정 | 무팩/구팩·8매핑/필터/앵커/빈queue; pin/의존/type/출처/경로 검증; 기존 출력 보호·상태 자동 승격0 검증 |
| N-14 | **완료 · 2026-10-10** | [수용 기록](./N14-CHANGE-ACCEPTANCE.md): 실제 변경 7c57a388 hold 후보·patch·linked run9/10·기준4 사용자 승인·승격·rollback·보관본 재생 | 실제 파일럿 ADR→requirement→criterion→code→test 연결, 변경patch→실제run→사람 수용→새색인/승격·거부/rollback 사례 검증 | 테스트pass/linked/verified/accepted 각각 기록, 무관test·정책/구현불일치·stale로 허위승격0; 실제 검토 계획/수용 원장 |
| N-15 | **완료 · 2026-10-10(로컬 개인 운영)** | [수용 기록](./N15-N17-LOCAL-OPERATION-ACCEPTANCE.md): go-stablenet 실제 색인·MCP 소비자·submodule 수정·지원/비용/제한 사용자 수용 | 대상 지원을 확정하고 native LinuxAMD64·실운영 규모/모델·외부 CLI/MCP/daemon 및 실제 프로젝트 설치/upgrade 검증 | OS/CPU/binary/source/모델 pin과 native/에뮬 구분; 실제지원/preview/미지원 명시. coding-agent 총비용은 외부별도측정 |
| N-16 | **보류(미충족)** · 사용자 결정 2026-10-12 | 권한자=사용자·신뢰=commit/binary SHA·재배포 없음 기록; 공개키 fingerprint 조건 미충족. 보류 중에는 완료나 공개 배포 가능으로 표시하지 않음 | OP01–06 역할/권한·scope/schema·public key/독립 신뢰·보관/교체/폐기·native/전이 고지 적합성 검토 | 재개 시 실제 권한자/공개키 fingerprint/신뢰경로·결정시각/근거, 운영계약 회귀, 정확한 고지 범위·담당 적합성 판정; 개인키 본문은 저장하지 않음 |
| N-17 | **완료 · 2026-10-10(로컬 개인 운영)** | [수용 기록](./N15-N17-LOCAL-OPERATION-ACCEPTANCE.md): 백업·손실 모의·복구 첫 질의35초·RTO1h/RPO24h 사용자 수용 | 실제 backup/failover/복구 담당·RTO/RPO/재시작/키 복구를 정하고 실훈련. network agent/전원 정책의호스트상태 확인; AC-only 정책 범위 검토 | 최신 운영후보/데이터로 복원·이전current/인용/키 신뢰 재생; 실제 실행·한계/담당 기록. 원격운영 상태를 로컬code만으로 완료하지 않음 |
| N-18 | **완료** · 2026-10-11(사용자 승인) | [판정](./N18-RELEASE-VERDICT-DRAFT.md): 선택 기능 기본값 유지(ontology off·knowledge opt-in·타입 선언 off), 로컬 개인 운영 허용. 2026-10-12 별도 지시로 PR #98 `main` 머지 완료; 태그·공개 배포 보류. 간헐 실패는 `396242f0`에서 수정 | 새 독립평가·원래회귀·사람비용/내용·대상지원·운영결정으로 기본값/출시 판정. 릴리스 문서/리뷰/merge/publish 준비 | 합격 근거 없으면 disabled/보류 유지. 태그·공개 배포는 별도 결정이 필요하며, preview 종료 승인을 운영권한으로 확장하지 않음 |

## 실행 순서와 우선순위의 이유

1. **N-02→N-03→N-04**: 새 검색 실험 전에 활성 데이터/동시 writer/복구 기반을 보호한다. N-05/06은 이 기반 위에 둔다.
2. **N-07**, **N-11 준비**: 이미 실측된 본문 예산 오류를 새 DEV 실패 시험으로 연결한다. 새 품질 목표·분할·표본은 결과를 보기 전에 검토한다.
3. **N-12/13→N-08/09/10→N-14**: 실제 지식과 원문 검색을 구분해 품질/기권/비용을 개선하고 실제 기준 수용 경로를 검증한다. 사실 승인 대기 중 구조·시험 준비는 진행 가능하다.
4. **N-15→N-16/17→N-18**: 운영까지 요구하는 경우 환경·키/신뢰/적합성·복구를 충족한 뒤 기본값/출시를 판정한다. 실패면 승인된 범위의 보고/보류로 종료할 수 있으나 운영 출시 완료라고 표시하지 않는다.
5. **N-01 완료**: 원본 복구와 전체 ID 대응에서 새 필수ID 누락은 찾지 못했다. 원래 목적의 품질·검토·비용·설치 조건은 기존 잔여에 유지하고 확인된 코드 차이의 재현을 이어간다.

## 기존 승인과 범위 유지

정적12/BGE·Go29/프로토콜/F01–06/SF 테스트 사례/STV2·HC01–11 해석 승인과 시험 preview 종료 결정은 유효하다. 새 목록 때문에 그 승인을 취소하거나 같은 질문을 다시 요청하지 않는다. 새 실제 파일럿 사실, 새 사전평가 입력/목표, 실제 운영 역할/키/지원/법적 적합성은 별도 대상이다.

GraphRAG 커뮤니티, RDF/OWL/SHACL, 물리 컬렉션 분할, 긴 함수 overlap 조정은 [PDF 추적표](./PDF-IMPROVEMENT-TRACE.md)의 조건부 연구 항목이다. 필요성/수용 조건 없이 필수 미완료로 추가하지 않았다. 이전 May CKV 계획이나 다른 저장소의 전체 coding-agent/ChainBench 작업도 이번 목록에 자동 편입하지 않았다.

**기존 N현재 상태:** 17/18 완료·N16 미충족 유지. 최신 진행 원장과 사용자 종료 범위가 기준이다. 온톨로지 추가 O작업은 아래 표로 별도 관리한다.

**상위 목표의 추가 추적 전체:** WI-V1 오라클 타당성, WI-V2 수정안 선택, WI-3 의심 라벨링, WI-4 비결정 버그, WI-1 값 흐름, WI-2 규칙 태깅, WI-C 깊은 코드 리뷰, WI-6 값 흐름 기반 다단 확장, WI-SP 정책/보안패턴 노출, WI-5 별도 사람 검토 학습. 각 상태/보류/담당·제품 N 연결은 [복구한 상위10개 표](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md#3-복구한-상위-작업계획의-전체10개-항목)를 기준으로 한다. 다음 상위 감사는 담당 저장소와 현재 WI-V1/V2/3 증거 확인이다.

N11 [새 DEV7개/프로토콜 초안](./N11-FRESH-EVALUATION-PREPARATION.md)은 미승인/미실행이며 새 FINAL0이다. N12 실제20타입/3요구/4기준 검토 자료를 요청했고 사람 판정·활동 시간 및 운영 환경/담당 입력을 기다린다. 준비 자료를 완료로 체크하지 않았다.

N12 정책·ADR 원문 주장이 준비 범위에서 누락된 것을 재감사로 발견해 [별도 검토 자료](./N12-POLICY-ADR-REVIEW.md)로 보완했다. 원문 커밋ab77ce56,6주장/12관계 proposed, 원문span 대조 및 실제 CLI4개 음성 제어 통과. 기존27개 사람 검토 요청은 보존한다. 새 주장/관계 승인0, 통합 lock/canonical·의미 수용 미완료이므로 N12 완료 체크와 전체 수는 바꾸지 않는다.

N11-C [실행 보호 세부 단계](./N11-EVALUATION-RUNNER-CONTRACT.md)를 구현·검증했다. 승인된 전체 책/runtime pins 검사와 FINAL durable reservation/영구 OS 잠금을 matrix 경로에 연결했다. Python19개·관련2패키지 race·native CLI4제어 통과, 실제 초안 child dispatch0/질의0. 새 사람 판정과 실제 평가 결과가 없으므로 N11 및 전체 완료 수는 그대로다.

N10 진단의 구조 RED→GREEN과 실제 SQLite3제어를 추가했다. 미해결 seed typed 표시를 추가하되 backend_error/모든 시도·composer 결과를 보존하고, 과거 정보가 없으면null이다. Go3패키지 race/vet·Python17개 통과. 비용/p95 새 평가가 없어 완료8/18·요청5/15·잔여10을 유지한다.

N11 [부분 FINAL 후보](./N11-FINAL-CANDIDATE-PREPARATION.md)4문항을2제안 사실군으로 묶고 원문·span SHA에 연결했다. 원 DEV7개와 프로토콜 바이트는 불변, 승인된 독립 FINAL0·실행/예약0이다. 전체 입력 검사 exit2, 미승인/미동결 및 중요군 미충족을 유지한다. 후보 작성은 전체 N11 완료가 아니다.

2026-10-07 N15/N17 [현행 패키지·외부 모듈·복구 세부 단계](./N15-N17-CURRENT-TECHNICAL-VALIDATION.md): clean native Darwin 패키지/3mock 프로젝트/외부 Go 공개 계약7응답/4거부 제어, 새 root 복원 reader lease 결함 수정·9제어·3패키지 race·복구10응답/13payload SHA 통과. native Linux CI는 정의만 존재, 실제 BGE/외부 운영 소비자·고지/권한/RTO/RPO 수용은 미완료다. 전체8/18·요청5/15·잔여10개를 유지한다.

2026-10-07 native CI 사전 검사: 전역 formatter의 동결 평가 원문3개 변경 요구를 관리 코드/fixture 범위로 제한했다. 5제어와 make fmt-check 통과, 원문/Go runtime 소스 변경0. 전체 완료 수와 CI 미실행 상태는 동일하다.

2026-10-07 [BGE-M3 작은 설치 호환성/전체 race](./N15-BGE-COMPATIBILITY-VALIDATION.md): 승인 digest/native1024의3소유 fixture 설치·MCP 재시작·데이터 갱신·rollback,6dataset identity·compact MCP7/CKV6응답과 보관 Git 원문 좌표를 확인했다. d08cb384 전체 race117패키지와 도메인 생성 자료 일치도 통과했다. N15-D의 모델 연결만 검증했으며 native Linux/규모/독립 운영 소비자·사람/운영 수용이 남아 전체8/18·요청5/15·잔여10개를 유지한다. 다음 N11/N12 실제 판정과 native CI다. 공개 전송 요청은 d08cb384의80커밋이며 추가 로컬 receipt/문서 커밋은 제외한다.

2026-10-07 [N11 후보 출처/과거 관측 대조](./N11-CANDIDATE-PROVENANCE-AUDIT.md): 전체 보관 FINAL9파일/4,800행의 SHA를 확인했다. 후보 lock.go 파일의 동일SHA/80행 인용720개를 발견했지만 후보parser55–76행 교집합0이다. 해당3request의 반복을 독립 표본으로 세지 않으며 후보의 독립성·개념적 오염은 미판정이다. 현재4후보/2proposed cluster·승인FINAL0·새질의0·N11 전체 미완료를 유지한다. d08cb384 공개 전송은 자동 승인 검토에서 새 범위 미승인으로 차단됐고 명시 응답 대기다. 전체8/18·요청5/15·잔여10개, 다음 N11/N12 판정 및 native CI를 유지한다.

2026-10-08 N11 후보 보강: [독립 FINAL 후보·범위 부족분](./N11-INDEPENDENT-POOL-EXPANSION.md). 신규CODE/WHY4문항/2제안군 추가, 기존 사실과 겹칠 위험의 신규4문항 격리. 검토 대상FINAL13/8제안군·DEV7, 승인된 중요군 모두0. 과거DEV/FINAL27gzip/11,632행 및 실제20원문 참조 대조, 정보 메타데이터 교정·새runtime 미결합, 실제input checker exit2/source오류0; 모델/검색/빌드0. 전체8/18·요청5/15·잔여10개 N08/09/10/11/12/14/15/16/17/18 유지. 다음 N11/N12 실제 내용·군/독립성 판정과 부족 범위 보강.

2026-10-08 N12 trace 연결 준비: [13개 proposed 관계와 의미 경계 검증](./N12-PROPOSED-TRACE-RELATIONS.md). 구현3/시험5/기준5 관계를 별도 SQLite에 원문·canonical·CKV로 연결하고 재조회/원본 없는 두 archive 재생·9거부 제어를 통과했다. Projection.Validate 시험을 다른 정렬 함수에 연결하는 부정확한1쌍은 제외, broad concept5경로 미작성 유지. 전체25관계 proposed·3요구 spec_unapproved·검토된경로0/current없음/human0/model0. 신규FINAL로 합산하지 않으며 N11 pool13/8·승인0, 전체8/18·요청5/15·잔여10개 유지. 다음 관계/원문 범위 판정·5경로 보강과 N11 독립 입력 확보다.

## 온톨로지 보완 작업리스트 — 2026-10-13

[검토 근거](./ONTOLOGY-MCP-EXPOSURE-REVIEW-2026-10-13.md). O항목은 기존 N항목 종료 기록을 변경하지 않는 추가 작업이다. 아래 완료에는 실행 명세의 모든 O 수용 조건·커밋 원자료 바인딩이 필요하다.

| ID | 상태 | 목적·선행 | 완료 조건 |
|---|---|---|---|
| O-01 | 완료 | MCP 온톨로지 준비 상태와 사용 가능 이유 노출 | off/미설정/누락/stale/준비/예산 구분, 동일 신원·읽기 전용·출력 schema/도구 등록·원자료 검증 |
| O-02 | 완료 | O01 기반 개념·용어·관계·요구 추적 읽기 MCP | 모호 후보/검토 상태/원문/기준·시험 링크·미연결 이유·상한·권한·무결성; 검색 모드와 독립 |
| O-03 | 완료 | O02 기반 프로젝트 ontology/spec/store/mode 설치 연결 | 명시 입력·같은 후보 신원 검증·off 기본·불가 상태 smoke; 실제 도메인 검토 필요한 항목은 대기 |
| O-04 | 완료(품질inconclusive·기본off) | 온톨로지 영향 설명과 적용 전후 대조 | 어떤 개념/관계가 점수·인용/본문/지연에 기여했는지 설명, independent protocol·표본/품질 한계 기록 |

**온톨로지 현재4/4 완료. O04 작업 잔여0개. 전체 잔여1개 N-16(기존 미충족 유지 결정).** 아래는 이전 단계 이력이다.

O03 설치 연결의 기술 A/B 완료, 실제 도메인/대상 자료 C/D 대기. [구현 기록](./ONTOLOGY-MCP-IMPLEMENTATION.md). 다음 실제 도메인 제안·검토 준비와 O04 설명/평가 계약. 전체 잔여 N16/O03/O04의3개.

O04-A 설명 기능 검증 완료, 저장 off/on 비교 도구는 기술 준비이며 O04-B 실제 실행/latency 조건과 C/D 도메인 품질 판정은 미완료. 전체 잔여3개 N16/O03/O04 유지. [검증과 필요 입력](./ONTOLOGY-MCP-IMPLEMENTATION.md#o04-설명과-저장-응답-비교--실제-효과-평가-대기).

O04 후속 완료: no_candidates 설명/비교와 eval capture의 v2 설명 입력 전달 누락 수정, 실제 stdio mock20응답/10쌍·실패/누락/pending0·기존 current/config SHA 보존. 실제 도메인 품질은 미평가, O04-B/C/D 미완료. 운영 source0bf2f4d1에는 추적 ontology/spec0, store 미연결; 기존 pack baselineec6a4e96b와 새 source 정합성 대조가 다음 O03 준비다. 전체 잔여 N16/O03/O04의3개.

O03 후속 기술 준비: [실제 팩 기반 proposed 검토 자료](../../projects/stablenet/domain-knowledge/meaning-draft/README.md) — concept4/requirement4/criteria5, 관계 후보16개 비실행, source span13/queue8, race44/20 pass. 이전 verified/mhha 기록을 새 승인으로 옮기지 않았다. 전체anchor134/selectedsymbol14의 기계 대조와 R04 표현 차이를 기록했다. C/D 내용/대상 수용은 미완료. 전체 잔여 N16/O03/O04의3개, 다음 subset 내용 검토·검토자·code/test 후보 판단이다.

O03 완료: 승인본과 실제501파일(Go494) WBFT held 후보·동일 source/index/semantic tuple·MCP조회/trace·실패 제어 검증. source/current/config 보존·활성0. O04 실제DEV 기계 관측과 source 한계를 기록했지만 품질/gold·독립성 판정은 미완료. 전체 잔여2개 N16/O04. [실제 연결·결과 기록](./ONTOLOGY-MCP-IMPLEMENTATION.md#o03-완료-사용자-승인과-실제-held-wbft-파일럿-결합).

O04-B 실제DEV 대조 완료: 사전 동결·같은 입력/신원과 실패 분모·모델 호출·지연/위치지표를 검증했다. C의 독립 평가 요구는 사용자 “다음 진행”만으로 면제하지 않았으며 D 최종 수용과 전체 O04 미완료를 유지한다. [결과와 한계](./ONTOLOGY-DEV-PILOT-RESULT-2026-10-13.md). 전체 잔여 N16/O04의2개.

O04 후속: [비용과독립입력준비](./ONTOLOGY-COST-AND-INDEPENDENT-INPUTS-2026-10-13.md). 같은파일바이트SHA를재확인하는bounded JSON좌표해석재사용, 동일DEV원자료/출력불변·미수용지연을기록했다. 8후보/7제안군은내용/전체이력·의미독립성미승인이다. C/D unchecked·전체잔여 N16/O04 유지.
