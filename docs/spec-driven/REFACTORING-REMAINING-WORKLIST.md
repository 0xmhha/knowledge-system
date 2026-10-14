# 현재 리팩토링 작업표

기존 **17/18**·온톨로지 **4/4** 완료. 전체 필수 잔여 **1개 N16**은 미충족·보류다. 최신 남은 전체 범위는 [남은 작업](../REMAINING-WORK.md), 수용 조건은 [실행 명세](./REFACTORING-EXECUTION-SPEC.md), 판정 한계는 [평가 상태](./EVALUATION-STATUS.md)를 따른다.

| ID | 상태 | 목적과 수용 범위 | 근거 |
|---|---|---|---|
| N-01 | 완료 | 초기 작성/보관 시점·SHA와 시작 배경 기록. FR10/INV7/NFR5/S9·WBS29를 v2/현행 감사와 대조 | 원장 `N-01`의 기록된 revision/manifest |
| N-02 | 완료 | pinned v2에서 활성 DB를 직접 수정하지 않게 거부/후보 빌드 경로로 통일. legacy 유지보수 동작과 migration 경계 명시 | 원장 `N-02`의 기록된 revision/manifest |
| N-03 | 완료 | v2 빌드/승격/rollback의 OS 잠금 계약 정리·구현. 살아 있는 긴 빌드의 age reclaim·동시 회수 경합을 재현 | 원장 `N-03`의 기록된 revision/manifest |
| N-04 | 완료 | DB checkpoint/close·source/manifest/candidate sync·current 전환/부모 sync 순서와 crash recovery 계약 보강 | 원장 `N-04`의 기록된 revision/manifest |
| N-05 | 완료 | 활성·rollback·실행 중 pinned reader·검토 대기 candidate 참조를 보호하는 GC dry-run/실행과 용량/보관 정책·status 보고 | 원장 `N-05`의 기록된 revision/manifest |
| N-06 | 완료 | 파일 수/단일/총 바이트 상한을 setup/config로 전달, build deadline·용량 예상/여유 공간·공개 resource 상태 구현 또는 계약 수정 검토 | 원장 `N-06`의 기록된 revision/manifest |
| N-07 | 완료 | 32000바이트/12인용 경계 안의 보관 근거 선택/부분 상태 및 안전한 typed budget 오류를 설계하고 독립 DEV 재현 후 수정 | 원장 `N-07`의 기록된 revision/manifest |
| N-08 | 완료 · 개선 미입증/inconclusive | CODE/WHY/POLICY/TRACE 기대 근거가 후보/조립에서 빠지는 원인 분리. 새 DEV에서 primary recall·청크·lexical/graph 조합 개선 | 원장 `N-08`의 기록된 revision/manifest |
| N-09 | 완료 · 개선 미입증/inconclusive | 무답의 검색 근거 상태와 downstream 답변 보류 계약을 명확히 하고 부적합 인용/무근거 운영 추정을 차단 | 원장 `N-09`의 기록된 revision/manifest |
| N-10 | 완료 | model/backend/원문 검증/의미 투영 비용 분해, 비반환 neighbors 오류 원인·영향 확인, 범위 내 비용/오류 개선 | 원장 `N-10`의 기록된 revision/manifest |
| N-11 | 완료 · 개선 미입증/inconclusive | 새 DEV 버전/질문과 결과 사전 비공개 독립 FINAL, 중요군 표본·평가 단위·정답·임계치·회전/비용 조건을 사전 검토·동결 | 원장 `N-11`의 기록된 revision/manifest |
| N-12 | 완료 | 실제 파일럿 concept/term/claim/정책/ADR/요구·기준과 코드 앵커를 검토. 대표 추출 오차/관계 오용/검토 시간·비용 기록; AI 사전검토65건의 사용자 일괄 채택이며 독립 항목별 사람 검토 아님 | 원장 `N-12`의 기록된 revision/manifest |
| N-13 | 완료 | 조직 catalog를 명시 선택형 pack 자료로 분리; 생성기의 허위 앵커 검사 완료 문구와 구 CLI 명령 교정 | 원장 `N-13`의 기록된 revision/manifest |
| N-14 | 완료 | 실제 파일럿 ADR→requirement→criterion→code→test 연결, 변경patch→실제run→사람 수용→새색인/승격·거부/rollback 사례 검증 | 원장 `N-14`의 기록된 revision/manifest |
| N-15 | 완료 · 로컬 개인 운영 범위 | 대상 지원을 확정하고 native LinuxAMD64·실운영 규모/모델·외부 CLI/MCP/daemon 및 실제 프로젝트 설치/upgrade 검증 | 원장 `N-15`의 기록된 revision/manifest |
| N-16 | 보류(미충족) | OP01–06 역할/권한·scope/schema·public key/독립 신뢰·보관/교체/폐기·native/전이 고지 적합성 검토 | 원장 `N-16`의 기록된 revision/manifest |
| N-17 | 완료 · 로컬 개인 운영 범위 | 실제 backup/failover/복구 담당·RTO/RPO/재시작/키 복구를 정하고 실훈련. network agent/전원 정책의호스트상태 확인; AC-only 정책 범위 검토 | 원장 `N-17`의 기록된 revision/manifest |
| N-18 | 완료 | 새 독립평가·원래회귀·사람비용/내용·대상지원·운영결정으로 기본값/출시 판정. 릴리스 문서/리뷰/merge/publish 준비 | 원장 `N-18`의 기록된 revision/manifest |
| O-01 | 완료 | MCP 온톨로지 준비 상태와 사용 가능 이유 노출 | 원장 `O-01`의 기록된 revision/manifest |
| O-02 | 완료 | O01 기반 개념·용어·관계·요구 추적 읽기 MCP | 원장 `O-02`의 기록된 revision/manifest |
| O-03 | 완료 | O02 기반 프로젝트 ontology/spec/store/mode 설치 연결 | 원장 `O-03`의 기록된 revision/manifest |
| O-04 | 완료 | 온톨로지 영향 설명과 적용 전후 대조 | 원장 `O-04`의 기록된 revision/manifest |

현재 저장소 반영은 PR #102 스쿼시 머지·main a214fbb1 동기화까지 확인했다. PR head e77e59b8의 CI5/5는 통과했다. 기존 리팩토링·온톨로지 원장의 완료 수와 상위 WI 선택9개의 구현9/9·독립 수용 미판정은 별도다. 태그·공개 배포는 보류하며 키 생성/기능 활성화 권한으로 확대하지 않는다. 상위 WI10개 및 새로운 연구/외부 운영은 이 완료 수에 더하지 않는다.
