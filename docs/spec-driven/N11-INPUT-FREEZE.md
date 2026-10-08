# N11 평가 입력 동결

2026-10-09 · **입력 동결·입력 검사 통과(검토자: 사용자 지정 AI 검토 라벨) / 실행 바인딩·FINAL 실행 전 / N11 전체 미완료**. 전체9/18, 요청6/15, 남은9개.

## 동결한 내용

사용자가 채택한 [개정 책](./N11-CANDIDATE-REVISION.md)을 검토자 “opus5.5 검토”(사용자가 지정한 AI 검토 라벨, 독립 사람 내용 검토 아님)로 승인하고 동결했다([생성 스크립트](../../system/eval/b0-knowledge-system/refactoring-n11-input-freeze-2026-10-09/build-freeze.py)).

- 철회 5문항(DEV-06/07, FINAL-04/13/15)은 책에서 뺐다. 철회한 FINAL은 활성 CODE 문항과 같은 군이라 제외 catalog에 넣지 않고 `withdrawn_before_freeze`로 기록했다.
- 대체 DEV 2문항: DEV-08(POLICY code_contract, 캡처 기본 상한, capture.go:50-54), DEV-09(WHY, 변경 잠금 소유권 근거, reindex.go:60-63·87-91의 설계 주석). 둘 다 FINAL 군과 겹치지 않는다.
- 결과: DEV7·FINAL10문항, FINAL 군 CODE4·WHY1·POLICY1·TRACE2·ABS1. [표본 부족 규칙](./N11-SHORTFALL-RULE.md)에 따라 모든 중요군이 inconclusive다.
- [검토 기록](../../system/eval/b0-knowledge-system/refactoring-n11-input-freeze-2026-10-09/review-record.json)은 protocol·질문 책의 정확한 바이트 SHA와 검토·동결 시각에 묶인다.

## 확인 결과

실제 입력 검사(`--require-ready`) exit0, `input_ready=true`, `sampling_verdict=inconclusive`, 거부 사유0([결과](../../system/eval/b0-knowledge-system/refactoring-n11-input-freeze-2026-10-09/input-check.json)). 이 검사는 입력 형식과 기록 일관성만 확인한다. `execution_ready`는 여전히 false다.

## 남은 조건

1. **평가 corpus 새로 구성.** 기존 N11 held corpus(`ee6f91be`)는 이번에 제외하기로 한 `system/eval/**` 파일을 포함하므로 실행 바인딩으로 쓸 수 없다. 제외 규칙을 적용한 새 coordinated dataset을 만들고 protocol의 `execution_binding`(dataset/config/binary SHA)과 `request_scope`를 넣은 뒤 다시 동결해야 한다.
2. **FINAL 실행은 되돌릴 수 없다.** runner는 FINAL 군을 자식 실행 전에 영구 예약한다. 실행하면 이 10문항은 다시 독립 holdout으로 쓸 수 없다. 모든 군이 inconclusive인 상태에서 실행할지는 사용자 결정이 필요하다.
3. N11-D는 이 입력이 N08/09/10 평가에 실제로 쓰여야 완료된다.

- [ ] N11-A–D 전체(입력 동결은 완료, 실행 바인딩·실행 미완료).

**남은 전체9개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
