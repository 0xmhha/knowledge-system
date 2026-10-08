# B1-05 안전 제어 감사 준비

2026-10-04 · **도구 준비 검증, 공식 B1-05 판정 대기**. 현재 단계와 남은 전체 작업은 [진행 현황판](./EXECUTION-STATUS.md), 완료 조건은 [작업리스트](./EXECUTION-WORKLIST.md)를 따른다.

[평가용 Go 도구](../../cmd/b1-evidence-audit/main.go)는 보관 SDK 원응답의 structured EvidencePackV2를 실제 공개 `evidencev2.Verify`로 재생한다. 원문 SHA·실행 바이너리 SHA·Go 버전·SDK measurement ID·각 행의 성공/오류를 기록한다. archive·모델·네트워크·데이터셋을 열지 않는다. 바깥 JSON integrity뿐 아니라 공개 계약의 내부 semantic 인용 registry, 상태·reviewed endpoint·conflict/required behavior/trace 투영과 complete/body 조건을 검사한다. 초기화·SDK/transport 오류는 기록하며 원래 private 진단 문자열을 새 보고서에 복사하지 않는다.

[제어 감사기](../../scripts/b1-audit-safety-controls.py)는 해당 Go 결과의 원응답 SHA와 모든 행의 sequence/arm/phase/request/iteration/measurement ID를 대조한다. 기존 matrix의 원문/설정 SHA·회전·조회 인자·source/citation/body/integrity·전후 불변성 검사도 재사용한다. 내부 semantic citation은 검증된 상위 registry의 정확한 인용과 같아야 한다. source bytes와 `.cks/knowledge/knowledge.lock.json`의 선언을 읽고 source control SHA·project/lock을 대조한다. 원자료/설정/source가 감사 중 바뀌면 거부한다.

## 검사하는 합성 계약

기대 상태는 최초 [fixture recipe](../../scripts/wbs-ontology-relations-smoke.py)의 `verify_knowledge` 계약이다. 정답을 새로운 공식 결과에 맞춰 바꾸지 않는다. source에 선언된 synthetic policy와 lock을 사용하며 `fixture-reviewer`를 실제 운영 사실 승인으로 기록하지 않는다.

| 요청 scope | 허용 state | 적용 policy ID | required_behavior | 추가 조건 |
|---|---|---|---|---|
| alpha | complete / partial | public | public | 자체 source 인용, implementation_link_unverified |
| wrong-scope | unknown | 없음 | 없음 | 범위 밖 정책 source/claim 미노출 |
| proposed | unknown | 없음 | 없음 | 미검토 정책 자동 current 승격 없음 |
| expired | stale | 없음 | 없음 | 만료 정책 적용/본문 노출 없음 |
| restricted | restricted | 없음 | 없음 | 접근 제한 정책 ID/source/본문 미노출 |
| conflict | conflict | conflict-a / conflict-b | 없음 | 지정 endpoints/reason, conflicting_policies_require_review |

모든 pack-off에서 knowledge/coding overlay와 knowledge source를 내보내지 않는다. 합성 proposed/expired/restricted statement marker가 SDK text·body·semantic·metadata 등 응답 어디에도 없는지 검사한다. 일반 비밀 탐지나 표현을 바꾼 추론 누출까지 보증하는 검사로 확대하지 않는다.

현재 fixture에는 reviewed implementation/rationale/constraint/decision/relation/trace가 없으므로 이를 만들어낸 응답을 거부한다. 같은 mode의 pack-on은 off의 기본 인용과 본문을 모두 보존해야 한다. 의미 tuple이 없는 이 fixture에서는 같은 pack 축의 네 mode가 인용·본문 **집합**을 보존해야 한다. 실제 reviewed tuple이 점수/후보에 기여하는 모든 production 사례에 동일 집합 조건을 강제하는 도구가 아니다.

## 실제 보관 원자료 재생

| 캡처 | SDK 응답 | 내부 semantic 인용 참조 | off/on 비교 | 선언된 제어 위반 |
|---|---:|---:|---:|---:|
| matrix-capture BGE-M3 | 288 | 1,320 | 144 | 0 |
| matrix-capture mock | 288 | 600 | 144 | 0 |
| environment-ledger BGE-M3 | 24 | 132 | 12 | 0 |
| environment-ledger mock | 24 | 72 | 12 | 0 |
| 합계 | 624 | 2,124 | 312 | 0 |

네 묶음은 서로 다른 모델/K/바이너리/환경의 기존 진단이다. 위 합계는 감사한 증거의 수이며 독립 질문 표본이나 하나의 공식 paired 실험 수가 아니다. 새 모델·MCP·공식 최종 질문은 실행하지 않았다. 보관 원자료/설정/로그/source 114개와 verifier 바이너리의 전후 SHA가 같다. 이전 원문의 best-effort 내부 backend 실패도 그대로 남긴다.

공개 Go 검증기에서 outer integrity를 다시 계산한 **내부 다른 프로젝트 인용**, **complete의 본문 누락**, **충돌 정책의 required_behavior 승격**을 각각 실제 거부했다. 입력과 typed 오류를 원장에 보존했다. Go helper의 SDK 정상/실패/누락/잘못된 JSON·진단 비복사 두 시험과 기존 evidencev2/contract 패키지, 새 command vet를 통과했다. Python 10개 회귀는 marker 누출, 미검토/만료/제한 승격, conflict endpoint, off overlay, 꾸며낸 implementation, 기본 본문 탈락, 다른 measurement ID·lock·source·uncertainty를 검사한다. CLI의 기존 출력과 잘못된 원자료 hash도 거부한다.

첫 Go 단위 fixture가 필수 `integrity_hash_algo`를 누락해 실패했으며 fixture를 보완해 재실행했다. 이 새 시험 자료 오류를 제품 결함이나 공식 C0 실패로 세지 않는다. 제품 core Go 검증기를 변경하지 않았다.

[원장](../../system/eval/b0-knowledge-system/safety-audit-preparation-m2max-2026-10-04/summary.json)은 원문/구현/binary SHA·Go 검증 결과·제어 출력·변조·CLI 거부·회귀에 연결된다. `quality_metrics=null`, 사람 verdict null, `official_B1_05_verdict=pending`이다.

## 재현 명령

```sh
go build -o /private/tmp/b1-evidence-audit ./cmd/b1-evidence-audit
/private/tmp/b1-evidence-audit \
  --input system/eval/b0-knowledge-system/matrix-capture-m2max-2026-10-04/real/rows.jsonl \
  --output /private/tmp/b1-contract-new.json
python3 scripts/b1-audit-safety-controls.py \
  --capture-dir system/eval/b0-knowledge-system/matrix-capture-m2max-2026-10-04/real \
  --controls system/eval/b0-knowledge-system/safety-audit-preparation-m2max-2026-10-04/matrix-capture-real-controls.json \
  --contract-audit /private/tmp/b1-contract-new.json \
  --source-root system/eval/b0-knowledge-system/b1-pack-matrix-m2max-2026-10-04/real/source \
  --output /private/tmp/b1-safety-new.json
```

기존 출력은 덮어쓰지 않는다. Go 계약/제어 위반은 보고서를 남기고 exit 2, 잘못된 입력/hash/기존 출력은 exit 1이다. source bytes 검사는 Go 구조 검증과 별도다.

다음 독립 작업은 raw CKV 후보 순위와 F-01 exact/budget 판정 보고 연결 준비다. 공식 B0 입력/프로토콜/동적/의미 사실·STV2 결정 뒤 source/pack/semantic/scope/K 동결·새 strict 빌드와 F-04 과거 상태/F-05 별도 프로젝트/F-06 구현·정책 충돌의 공식 오라클을 실행한다. 실제 비밀/권한·주장/답변/정책의 사람 판정과 최종 C1 운영 범위는 남아 있다.
