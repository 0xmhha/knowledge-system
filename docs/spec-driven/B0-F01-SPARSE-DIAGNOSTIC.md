# F-01 실제 희소 검색 개발 진단

2026-10-05. [원장](../../system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/summary.json)은 미승인 F-01-DEV 합성 입력의 진단이다. 공식 F-01 verdict와 품질·사람 판정은 null이다. 원문을 점수에 맞춰 바꾸거나 FINAL을 실행하지 않았다.

고정 commit `3433883f3d811dff182b92ada250fae491a6faa2`, tree `7e3ceb35b1055b0e85024e37d6e0b09e3a5c2516`의 독립 clone을 새 strict BGE-M3로 빌드했다. 원문 61개는 원래 Git blob과 바이트가 같고 Go 60파일→122청크·truncated0이다. 승인 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 1024차원이다. 실제 canonical 60/60 정렬은 기록하지만 configured canonical coverage gate disabled를 전체 gate 통과로 해석하지 않는다.

질문은 동결 DEV 문구 `Where is the gas refund route implemented?`, K5, language=go/path=src/*.go/Function/symbol/exclude_tests다. 직접 저장 벡터 probe는 다음 결과를 반환했다.

| 검사 | 실제 결과 |
|---|---|
| 무필터 top5 | refund01 header/function와 distractor39/07/55; 대상 파일5개 중4개 누락 |
| 전체 eligible inventory | src/refund00.go–04.go의 Function5개, 모두 lines3–4 |
| 정상 필터 검색 | complete, Function5개, 독립 같은 저장 벡터 exact 순위/ID/줄/거리 일치 |
| candidate budget2 | incomplete/candidate_limit, 반환0개, coarse SQL candidate_count60 |
| 원문/DB/모델 | probe 전후 DB·manifest·graph SHA 및 source/선택 모델 metadata 동일 |
| 독립 reader | diagnostic_controls_pass, invariant violations0 |

SQL의 coarse 후보60과 최종 eligible5는 서로 다르다. 후보 상한은 60후보를 대상으로 적용되며 eligible5로 바꿔 기록하지 않는다. exact full inventory와 최종 필터를 따로 보존한다. 무필터에는 같은 파일의 다른 raw chunk 슬롯이 있으며 인용 dedup 점수로 대체하지 않는다. 이 실행의 지표는 @5이고 @10 또는 독립 표본5개가 아니다. 독립 질의 unit1의 추론은 inconclusive다.

[원시 probe](../../system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/probe-stdout.txt), [개발 진단 보고](../../system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/diagnostic-report.json), [보관 소스 재생](../../system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/archive-replay.json), [실행 명령/strict 환경](../../system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/commands.json)을 보존한다. probe/reader 소스 SHA와 실제 바이너리 SHA를 기록했으며 큰 DB/모델/바이너리는 Git에 넣지 않았다. 기존 역사적 Alpha K1의 fixture_not_qualified를 소급 변경하지 않는다.

```sh
python3 scripts/b0-summarize-vector-probe.py \
  --probe system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/probe-stdout.txt \
  --manifest system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/dataset/vector/manifest.json \
  --controls system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/controls.json \
  --source-root system/eval/b0-knowledge-system/f01-sparse-preparation-m2max-2026-10-05/source \
  --out /private/tmp/new-f01-archive-replay.json
```

새 출력만 허용하며 재생은 DB나 모델을 호출하지 않는다. 기존 reader 15개 회귀와 보관 재생·최종 조립 검사로 확인했다. 첫 시험 glob은 0개를 발견했으므로 실패 진단 원자료로 보존하고 정확한 test_b0_vector_summary.py로 15개를 실행했다. 이번 문서/원자료 추가는 제품 Go 수정이 아니므로 이미 통과한 query/계약 시험을 반복하지 않았다. B0-01/02/03/05·STV2-01 승인, 공식 새 전체 기준선/8경로/사람 판정 및 최종 단계는 남는다.
