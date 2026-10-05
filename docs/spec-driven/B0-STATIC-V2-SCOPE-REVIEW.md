# B0 정적 v2 조회 범위·K 검토 제안

2026-10-04 · **2026-10-06 설정 승인·해시 결합 검증 완료 / 실제 runtime·공식 실행 진행**. [작업리스트](./EXECUTION-WORKLIST.md)의 B0-02/06에서 남은 정적 v2 날짜/subsystem·실제 K 결합을 위한 입력 제안이다. 정적 gold와 기존 프로토콜·동적 입력·승인 원문을 변경하지 않았다.

## 승인된 제안

검토 ID는 **STV2-01**이며 사용자 명시적 승인을 [승인 기록](./B0-APPROVED-INPUTS.md)에 연결했다. 정적 12문항의 ID·개발/최종 분할만 사용하는 [초안](../../system/eval/b0-knowledge-system/static-v2-scopes-m2max-draft.json)을 준비했다. 최종 프롬프트나 후보 답을 이 파일에 복사하지 않았다.

| 필드 | 제안 | 근거와 의미 |
|---|---|---|
| knowledge_as_of | 2026-10-01 | 고정 코퍼스 커밋 71cb71cd의 실제 committer time은 2026-10-01T20:53:13+09:00. 그 시간대의 달력 날짜를 조회 파라미터로 제안 |
| knowledge_subsystem | knowledge-system | 승인된 질문집의 corpus_project 이름을 명시적 subsystem으로 제안. 기존 정책이 이 scope에 해당한다는 사실 승인은 아님 |
| retrieval.recall_k | 정수 10 | 기존 프로토콜의 retrieval_k=10. 생략 시 기본 K20이므로 승인 후 실제 runtime YAML에 이 값을 명시하고 계측값과 대조해야 함 |
| include_knowledge | 요청 입력에서 생략 | 실제 matrix가 off/on 축을 소유하고 각 arm에 설정. 요청 파일이 이를 먼저 지정하면 CLI가 거부 |
| 개발/최종 분할 | 기존 4/8 유지 | 질문 ID를 같은 프로토콜 분할에 연결. 최종 ID를 개발 요청으로 이동하지 않음 |

조회 날짜는 커밋 시각 자체를 재현하는 timestamp가 아니다. 정책의 날짜 비교에 사용할 제안이며, 그날 정책이 승인됐거나 효력이 있었다는 판정은 아니다. 검토자가 다른 날짜/subsystem을 원하면 실제 팩·정책의 효력/범위 근거와 함께 수정하고 입력을 새 해시로 동결한다. 현재 checker는 위 제안 계약을 검사하므로 다른 계약을 선택하면 checker와 회귀 사례도 같이 갱신한다.

## 확인한 추가 선행 조건

실제 고정 커밋의 소스 루트 `.cks` 트리 목록은 비어 있다. 따라서 이 커밋만으로 검토된 팩·활성 의미 투영의 존재를 주장하지 않는다. 외부 팩/의미 저장소를 사용한다면 source-bound 출처·실제 검토·잠금·dataset identity를 별도로 기록해야 한다. 팩 on이 unavailable 폴백인 실행은 팩 효과의 품질 증거로 합산하지 않는다. 이 제안으로 static B1의 semantic/pack inventory를 임의로 생성하거나 승인하지 않았다.

승인 전에는 프로토콜/동적 사례의 기존 보류가 남으며, STV2-01도 draft다. 메타데이터만 approved로 바꿔도 실제 human-review의 scope SHA 연결 없이는 승인으로 인정하지 않는다. 승인된 범위 정의가 있더라도 새 strict 데이터셋·팩/semantic 신원·실제 K/config·모델/환경/바이너리 검증이 남으므로 official_execution_ready와 v2_matrix_ready는 false다.

## 검증과 재현

```sh
python3 scripts/b0-check-static-v2-scopes.py \
  --output /private/tmp/b0-static-v2-scope-new.json
python3 -m unittest discover -s scripts -p test_b0_static_v2_scopes.py
```

검사기는 입력의 SHA·corpus commit/tree·실제 Git committer time·12개 ID 유일성/완전성·4/8 분할·날짜 형식/제안 근거·subsystem·정수 K10을 검사한다. row에 prompt/gold 필드를 끼워 넣으면 거부한다. 출력은 새 0600 JSON 보고서 하나이며 기존 입력·출력을 덮어쓰지 않는다. 승인 대기 상태는 보고서를 남긴 뒤 exit 2다. 최종 요청 exporter나 모델 호출 도구가 아니다.

새 validator의 첫 회귀에서 Python이 ISO week date와 10.0을 각각 날짜/정수 10과 같게 처리하는 두 허점을 확인했다. MCP의 YYYY-MM-DD와 실제 Go config 정수 계약에 맞춰 엄격하게 검사하도록 고쳤다. 실패 전/후 로그는 [범위 제안 원장](../../system/eval/b0-knowledge-system/static-v2-scope-preparation-m2max-2026-10-04/summary.json)에 보존한다. 기존 정적 입력 준비 검증과의 호환도 확인한다. 이 새 보조 도구의 검증 결과를 공식 품질 점수로 기록하지 않는다.


2026-10-04 FIX-16 재감사: scope 해시가 같은 `pending` 사람 결정이 `scope_review_record_bound`를 만족하는 실패를 재현 후 수정했다. 이제 `static_v2_query_scopes` 기록의 실제 `status=approved`, 검토자·시간대 있는 `reviewed_at`도 필요하다. 원문 scope와 실제 사람 기록은 바꾸지 않았으며 [비교 보고/승인 경계 자료](./B1-PAIRED-REPORT-PREPARATION.md)에 수정 전·후 증거를 연결했다.
