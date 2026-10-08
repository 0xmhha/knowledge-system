# N12 core·기능 개념·요구·기준 수정 반영(제안)

2026-10-09 · **사용자 채택 기록·새 버전 제안 파일 작성·생산 parser 검증 / verified0·N12 전체 미완료**. 전체8/18, 요청5/15, 남은10개.

## 배경

[core 사전검토](./N12-CORE-PREREVIEW.md) 30건 권고를 사용자가 “권고대로 채택”했다. 검토자 이름 질문에는 이름 입력을 골랐지만 이름 텍스트가 오지 않아 reviewer는 null이다([결정 기록](../../system/eval/b0-knowledge-system/refactoring-n12-core-revision-2026-10-09/decision.json)). 그래서 수정은 새 버전 **제안** 파일로만 반영했다. 원본 `ontology-pilot.yaml`, `ontology-pilot-functional.yaml`, `spec-pilot-v2.yaml`, `PILOT-SCOPE.json`은 held 후보의 입력이므로 그대로 두었다.

## 반영 파일

- [ontology-pilot-v2.yaml](./ontology-pilot-v2.yaml): core20 중 7개 수정(code-symbol, claim, evidence-span, semantic-assertion, acceptance-criterion, patch-attempt, criterion-decision). criterion-decision에서 제품이 지원하지 않는 needs-review를 지우고, 한국어 용어를 "기준별 수용 판정"으로 바꿨다.
- [ontology-pilot-functional-v2.yaml](./ontology-pilot-functional-v2.yaml): 위 core v2와 기능 개념3의 수정(정확 탐색 조건, cancelled 상태, 어휘 매칭, 집합 보존, 한국어 용어 "스냅샷 결합 관계 검증"·"기준선").
- [spec-pilot-v3.yaml](./spec-pilot-v3.yaml): 요구3을 version 3으로 올리고 기준4의 Given/When/Then을 제품 동작에 맞췄다. 취소는 cancelled, promotion은 Put/Activate 네 경로와 typed mismatch 오류, ontology는 집합 보존으로 명시했다.

## 확인 방법과 결과

모듈 안에 임시 driver를 만들어 생산 `ExtractOntology`/`ExtractSpec`/`ValidateCoreOntology`/`Projection.Validate`로 검증하고 driver를 지웠다([driver](../../system/eval/b0-knowledge-system/refactoring-n12-core-revision-2026-10-09/validate-proposal.go.txt), [결과](../../system/eval/b0-knowledge-system/refactoring-n12-core-revision-2026-10-09/validation.json)). core v2는 20개 ID·kind가 원본과 같고 core 검증을 통과했다. functional v2의 앞 20개는 core v2와 같다. 요구3·기준4가 함께 검증되고, 모두 proposed·ReviewedBy 비어 있음이다. snapshot/dataset ID는 세 파일 바이트의 digest로 만든 source-only 값이며 coordinated 신원이 아니다.

생산 matcher로 한국어 충돌을 확인했다. 이전에는 "수용 기준 판정" 질의가 두 개념을 함께 매칭했고, 수정 후에는 새 용어 "기준별 수용 판정"이 criterion-decision 하나만 매칭한다.

## 남은 조건

- [x] 30건 채택 기록·새 버전 제안 파일·생산 parser 검증.
- [ ] 검토자 이름. 이름 뒤에 제안 파일을 선택(PILOT-SCOPE 갱신)하고 새 capture/CKG/CKV/projection에 결합(N12-B).
- [ ] 실제 검토 활동 시간(N12-C, 미측정 null), 현행 결합·N14 계획(N12-D).

**남은 전체10개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
