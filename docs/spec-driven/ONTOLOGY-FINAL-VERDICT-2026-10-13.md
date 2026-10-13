# 온톨로지 O04 최종 판정 — 2026-10-13

**O04-A–D 완료. 온톨로지 보완 O01–04의 4/4 완료. 기존 작업은 17/18이며 전체 잔여는 사용자 결정으로 미충족 유지한 N16 하나다.** 검색 품질 개선·독립 품질 수용·기본 활성화·WBFT 기준 충족을 선언하는 완료가 아니다. 최종 품질 판정은 inconclusive, 기본 off/opt-in과 운영 활성0·공개 배포 보류를 유지한다.

## 수용 조건과 완료의 의미

원명세 O04-C는 controlled 기능 설명과 실제 도메인 효과 판정을 분리하고, 실제 자료/독립 평가 조건·부족 표본/활동시간·quality 한계를 기록하도록 요구한다. O04-D는 그 판정과 구현/시험/원자료·사전조건·현재 및 커밋 SHA의 결합을 요구한다. 품질이 개선됐다는 결론이나 미충족 독립성을 충족했다고 표시하는 것이 완료 요건은 아니다.

사용자의 “나머지 004도 모두 진행. 완료까지 계속 진행하고 모두 완료후 보고할것.”에 따라 기술 검토·동결·실측·최종 판정까지 수행했다. 이전 문서에서 독립 입력을 준비하며 대기하던 절차는 이번 직접 기술 검토 및 독립 자격 **미충족 판정**으로 정리한다. 독립 FINAL의 자격 요건을 완화하거나 사람 검토를 만들어내지 않았다. 사람의 독립 내용 검토0·승인 독립 단위0·새 FINAL0을 그대로 기록한다. 독립 효과 입증을 이후 추진하려면 별도 검토·충분한 사실군과 새 입력을 마련해야 하며, 이번 작업 완료를 그 근거로 사용할 수 없다.

| 조건 | 실제 근거·판정 |
|---|---|
| O04-A | bounded query 설명·off/no_match/active/zero 관계 구분·v2 해시/상한·기존 controlled 시험과 이번 실제 설명 수집 |
| O04-B | 동일 binary/dataset/snapshot/source/model/질문·설정의 사전 동결과 실제702응답/351쌍; 지연·인용/본문·실패 분모·요청별 backend 검증 |
| O04-C | 과거 노출 제외·직접 source/gold 검토·독립 조건 미충족·부족 표본·실제 위치 지표/비용/활동시간 및 quality 한계 판정 |
| O04-D | 불변 동결 입력과 전체 원자료·재채점/완료 검사·구현/시험 참조·현행 및 커밋 SHA 결합·명세/작업리스트/진행 원장 |

## 동결과 실제 실행

4387b888에서 질문·gold·프로토콜·채점기·capture driver·원문 사본을 실측 전에 커밋했다. 기존 이력6,376파일 SHA 결합으로 노출4문항 및 설계 맥락1문항을 제외한 13문항을 사용했다. 원문7파일의 기존18후보/19gold 검증을 보존하며 이번 검색13문항의 원문13범위와 별도 trace2문항을 직접 대조했다. IC-04 답은 round=0일 때 상한 비교 반복이 없다는 경계를 보완했고, 모든 overflow 검출을 주장하지 않는다. 검토자는 Codex direct source inspection이며 사람 독립 검토가 아니다.

같은 실제 BGE-M3:latest digest790764642607…16bab/native1024, held WBFT source baca2869118774c5f6219932dc76e197a47e6ecd와 O03 dataset/snapshot을 유지했다. binary5e47f44f…da53은 좌표 JSON 해석 결과 재사용 후보이며 기존 기준 binary와 운영 설치는 그대로다. off=baseline, on=concept_text이며 raw-query-first는 동일하다. 질문마다 off/on 첫 순서를 번갈아 배치했다. arm별 warmup2·검색5·warm20·cold0으로 13×2×27=702응답이다. 품질 위치 채점은 검색130회, warm 지연은 arm당260회다. 반복을 독립 표본으로 세지 않는다.

실제 수집 활동시간은 약 376.2초다(첫 arm 시작–마지막 arm 종료, 사전 준비/비교/재채점 시간 제외). v2 좌표·무결성·인용 전체 파일/행 SHA와 본문 SHA를 검증했고, 다른 프로세스의 CLI compare-responses로351쌍을 대조했다. 오류·누락·pending·HTTP실패·요청 scope 미결합은 모두0이다. 별도 trace2도 missing_code/acceptance_evaluated=false를 정확히 반환했다. 702개 raw pack의 검증·채점을 다시 계산해 최초 결과와 모든 점수 필드가 같음을 확인했다.

## 실제 영향과 비용

| 지표 | off | on |
|---|---:|---:|
| 제안10군 평균 Recall@10 | 1.000000 | 1.000000 |
| 제안10군 평균 Precision@10 | 0.147778 | 0.147778 |
| 평균 expected reciprocal rank@10 | 0.658333 | 0.645833 |
| 전체 warm p95(ms) | 444.818 | 482.868 |
| 요청 scope /api/embed 호출 | 1434 | 1548 |
| 요청 scope /api/tags 호출 | 2868 | 3096 |

온톨로지는 4문항의108개 on응답에서 active, 나머지9문항의243개에서 no_match였다. 공개 인용 구성·순서·본문이 변한 것은 IC-17/18의54쌍이며 나머지297쌍은 같다. IC-17/18은 같은 요청 입구 군으로 묶었고, 근거 순위 지표 하락을 개선으로 설명하지 않는다. 매칭된 검토 개념 정의로 추가 검색/후보 score가 바뀌어도 기존 CKV+CKG 검색을 대체하거나 소스 사실을 검증하는 것은 아니다.

전체 warm p95 비율 1.085541은 사전 descriptive 예산1.25 이내이나, 문항별 IC-04/17/18은 각각 1.2817, 1.3236, 1.2988로1.25를 넘었다. 전체 분위수만으로 이 초과를 숨기지 않는다. shared host·warm·비무작위 순서와 문항별20회라는 조건, 이전 no_match 비용 이상치를 함께 남긴다. **전체 성능 수용 false**다. 모델 호출은 실제 요청 scope 기준이며 scope 밖 constructor probe는 미계측이다. 시작/종료 model tags 검사는 별도이고 위 측정에 합산하지 않는다. 과거 DEV·비용·FINAL 결과와 합치거나 소급해서 threshold를 바꾸지 않았다.

## 독립성과 품질 판정

제안10군은 IC-02/03 validator-membership, IC-11/16 큐 입구·상태 처리, IC-17/18 요청 검증·전송 입구를 보수적으로 합친 수다. 작성자는 이전 DEV를 봤고 같은 승인 개념군을 사용한다. 행 노출 없음·새 문장·10군 제안 수는 의미적/통계적 독립성을 증명하지 않는다. 빈 역사 파일3개와 채팅/외부/삭제 이력 미복구 한계도 유지한다. 실제 승인 독립단위0<바닥10으로 독립 자격 not_established, quality inconclusive다.

Recall은 위치 교집합 관측이며, 함수 범위가 gold 행과 겹치는 hit가 후보 답의 사실 정확성을 증명하지 않는다. 생성 답변 품질과 사람의 기권 점수는 평가하지 않았다. 실제 의미 store의 검토된 코드/시험 관계0이며 요구 trace의 missing_code와 acceptance_evaluated=false도 그대로다. WBFT가 기준을 통과했다는 판정을 하지 않는다.

## 구현과 재현 근거

[동결 입력·프로토콜·기술 검토](../../system/eval/b0-knowledge-system/ontology-o04-completion-2026-10-13/question-book-frozen.json), [실측 요약](../../system/eval/b0-knowledge-system/ontology-o04-completion-2026-10-13/result-summary.json), [최종 판정](../../system/eval/b0-knowledge-system/ontology-o04-completion-2026-10-13/final-verdict.json), [재채점](../../system/eval/b0-knowledge-system/ontology-o04-completion-2026-10-13/replay-score-check.json), [종료 보존 검사](../../system/eval/b0-knowledge-system/ontology-o04-completion-2026-10-13/post-run-preservation.json). 전체 capture/요청/config/명령/event/rows/result는 같은 폴더의 capture 아래 압축 보존했다. 기존 설명 기능·race247시험/249하위시험과 비용 진단 자료는 당시 커밋 참조를 유지한다. 이번 Go source1166파일의 SHA는 불변이며 Go 시험을 새로 실행했다고 기록하지 않는다.

입력·채점·이력·종료 판정80시험 pass, 진행guard11시험 및 docs 검사를 결합한다. 종료 검사기는 실행/분모/쌍/trace/backend/보존 누락, 관측과 다른 cost 판정, 독립 단위/사람 검토/품질 성공/기본 활성/관계 수용을 만들어내는 변경을 거부한다. 기존 B0 승인 입력5개의 SHA와 수용 임계치는 그대로다. 운영 원문 HEAD/current/config와 held graph/vector/semantic DB SHA, 실행 binary와 model digest를 종료 후 다시 확인했다. push/PR/merge/deploy·운영 활성은 수행하지 않았다.

## 다음 작업과 남은 전체 작업

**현재 O04의 명세 작업은 남아 있지 않다.** 전체 잔여는 N16 하나로, 공개키 fingerprint/독립 신뢰 조건을 미충족 유지한다는 기존 사용자 결정을 따른다. 이번 작업으로 키를 만들거나 그 조건을 완료로 체크하지 않는다. 향후 독립 품질 입증·추가 비용 개선·새 corpus·운영 전환은 별도의 후속 범위이며 이번 완료에 포함됐다고 선언하지 않는다.
