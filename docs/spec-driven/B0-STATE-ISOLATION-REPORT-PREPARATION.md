# F-04/F-05 상태·프로젝트 격리 보고 준비

2026-10-05. 범위는 미승인 합성 DEV 입력의 구조 진단이다. [진단 원장](../../system/eval/b0-knowledge-system/state-isolation-preparation-m2max-2026-10-05/summary.json), [재생 보고](../../system/eval/b0-knowledge-system/state-isolation-preparation-m2max-2026-10-05/diagnostic-report.json), [작업리스트](./EXECUTION-WORKLIST.md)를 함께 읽는다. 공식 품질 지표·사람 verdict는 null이며 최종 입력은 실행하지 않았다.

F-04 old/new는 같은 프로젝트의 순차 커밋을 독립 clone으로 유지했고 F-05 A/B는 같은 상대 경로를 갖는 서로 다른 프로젝트다. 고정 source SHA·commit/tree와 새 strict BGE-M3 빌드를 연결했다. 모델 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 1024차원, 실제 raw/text K10·knowledge K6다. 각 상태의 identity/manifest·청크 메타데이터·소스·질문 전용 입력·설정·CLI/SDK·backend 시도를 보관했다. 원래/수정 바이너리 4개씩 총 8개 정상 SDK 응답의 28인용/28본문은 자기 source bytes/범위 SHA·좌표와 같고 공개 Go Verify가 유효하다. 선언된 code/policy source body 범위는 모두 반환됐다. 내부 best-effort nonreturned 8시도는 SDK 성공과 별도로 보존한다. 네 상태를 독립 품질 표본 4개로 합산하지 않는다.

F-05의 refund 10/20 Markdown은 원시 합성 문서다. semantic=null이며 reviewed policy pack을 켜거나 조직 정책을 승인한 진단이 아니다. 실제 B0-05 검토와 팩 축은 남는다. 같은 `main.go` 경로라도 A/B 인용의 project/dataset/snapshot/commit/file SHA가 다르고 반환 body에 상대 프로젝트 marker가 없다. marker 검사는 작성된 문자열 범위이며 일반 비밀 누출 탐지를 보증하지 않는다.

## FIX-18: 오래된 CKV 조회를 fresh로 표시하던 오류

old 인덱스를 new 소스로 조회하면 실제 세 인용은 stale_citation=true인데 metadata.fresh=true였다. 수정 전 Git 기반 회귀는 커밋 변경과 0 hit의 두 경우에서 실패했다. 정상 Search의 요약을 확인된 source/index HEAD 불일치 또는 stale 인용에서 false로 수정했다. Git HEAD 미확인/보관 소스와 dry run의 best-effort 호환은 유지하며 true를 검증 보증으로 설명하지 않는다. [공개 CKV 설명](../vector/mcp-tools.md)에 의미를 기록했다.

같은 실모델·old DB/new source를 수정 바이너리로 다시 조회했다. fresh true→false 외에는 trace ID를 제외한 전체 응답(인용·본문·점수·순위·warnings)이 같았다. query 전체/race/vet와 공개 API/MCP·CKS client·CLI/v2 계약 회귀를 통과했다. 네 상태의 replay 전후 payload·source·config/request SHA와 선택 모델 metadata는 같다. SDK에는 별도 fresh 필드를 새로 만들지 않았다. 이번 개발 진단 수정은 공식 C0 종료나 전체 B0/B1 안전 합격이 아니다.

## 혼합 설정 오라클의 실제 결과와 미충족 범위

서로 다른 native version 디렉터리의 graph/vector를 설정한 두 오류에서 CLI는 operation_failed, 보존된 SDK는 IsError=true/reindex_required다. 초기 helper가 오류 시 보고서 미생성을 가정하여 중단한 사실을 남기고 미완료 B 상태만 재개했다. 보존된 partial SDK 원문을 검사하여 정상 SDK 성공으로 세지 않는다.

이 설정은 유효한 native pinned version 배치가 아니므로 초안의 pinned 혼합 좌표 snapshot_mismatch 오라클 통과를 증명하지 않는다. gold를 reindex_required로 고치지 않았다. 기존 공개 EvidencePackV2.Build의 snapshot_mismatch 회귀와 이번 설정 거부는 서로 다른 범위다. 승인 후 공식 수집에서 정확한 pinned 혼합 좌표/정렬 오라클을 실행해야 한다.

## 재생

```sh
python3 scripts/b0-audit-state-isolation.py \
  --bundle system/eval/b0-knowledge-system/state-isolation-preparation-m2max-2026-10-05/bundle.json \
  --out /private/tmp/new-state-isolation-report.json
python3 -m unittest discover -s scripts -p test_b0_state_isolation.py -v
```

새 출력만 허용한다. 보고서는 fixture DEV 분할·원문/신원·입력/바이너리 SHA·measurement/K·본문/인용·실패/누락 분모를 검사한다. 12개 회귀는 outer hash를 재봉인한 foreign 좌표/인용/본문, gold 전달, final 입력, source/config 변조, 실제 K 변경, 누락/오류와 stale 요약을 포함한다. Python helper 초기 문법/소스 dict 가정, Go package 경로 및 감사 CLI flag 실수도 제품 실패와 구분한다. 완료 범위는 FIX-18과 진단 재생 준비이며 28개 공식 미완료 작업은 그대로다.
