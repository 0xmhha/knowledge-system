# N11 후보의 과거 관측·출처 대조

2026-10-07 · **직접 출처 대조 완료 / 독립성·오염의 사람 판정 대기 / N11 미완료**. [대조 요약](../../system/eval/b0-knowledge-system/refactoring-n11-provenance-audit-2026-10-07/summary.json)과 [해당 인용720개](../../system/eval/b0-knowledge-system/refactoring-n11-provenance-audit-2026-10-07/matching-citations.jsonl)을 연결한다. 기존 질문·프로토콜·승인 원장·FINAL 원자료는 수정하지 않았고 새 질의/FINAL 예약·dispatch0이다.

이전 자료는 후보4문항을2제안 사실군으로 묶고 과거 pack/읽기 전용 진단과의 중복 여부를 사람에게 남겼다. 실제로 어떤 출처가 이미 반환됐는지 확인하기 위해, 승인 FINAL 보관9파일의4,800행을 다시 읽고 compressed/decompressed SHA를 기존 artifact manifest와 대조했다. 이는 보관 자료의 분석이며 새 독립 평가가 아니다.

known-final-observations의7파일은14개 cluster의 최초 보관 응답을 연결한다. 동일 사실군의 추가 상태인 F04-new/F05-b도 이번 대조에 포함해 전체9파일을 읽었다. 최초 관측 목록을 불완전한 전체 capture 목록으로 해석하지 않았으며 해당 원장은 바꾸지 않았다.

## 확인한 범위

| 후보 | 직접 확인 | 남은 판정 |
|---|---|---|
| CAND01/02: semantic-readonly-open | readonly.go의11–36행/file·span SHA 확인; 보관 FINAL의 해당 파일 직접 인용0 | 과거 DEV/진단·본문 외 의미 참조 및 같은 근본 사실의 오염 여부 |
| CAND03/04: knowledge-manifest-parser | lock.go 동일 file SHA의80행 인용720개; 후보55–76행과 교집합0 | 같은 파일/인접 기능 노출과 parser 사실의 개념적 중복 여부 |

lock.go80행 인용은 B0-WHY-01·B0-POLICY-02·B0-ABS-01의3개 request ID에서 반복됐다. **720회를 독립720단위로 세지 않는다.** 같은 파일을 인용했다는 사실만으로 parser의 엄격한 YAML 처리 사실이 관측됐다고 판정하지 않는다.

형식v2 pack4,560행과 pack 없는 오류240행을 포함해 총4,800행을 유지했다. 후보 파일과 같은 path의 직접 인용720개는 모두 동일 file SHA이며 후보 span과 겹치는 인용은0이다. canonical body의 OpenStoreReadOnly·PRAGMA user_version·ReadManifest·KnownFields 문자열 출현도 각각0이다. 이 값은 검색 품질 재판정이나 오염 부재 증명이 아니다.

기존 승인 B0 질문12개, N11 DEV7개, N12 정책·ADR6개 주장의 source/candidate-code 참조를 별도로 대조했다. 후보 파일과 같은 직접 참조 path는0이다. 문장 의미가 다른 파일에서 재사용됐는지, graph-neighbor/semantic 참조·이전 DEV/다른 진단·사람의 사전 노출 여부는 이번 직접 비교 범위 밖이다. 키워드가 없거나 path가 다르다는 이유로 독립성을 승인하지 않는다.

## 수용 조건과 다음 작업

4개 후보의 source commit f066e67b와 실제 Git file/span을 확인하고 보관한 span을 .txt로 연결했다. parser와 readonly의 CODE/WHY 쌍은 각각 같은 proposed cluster이며 승인된 독립 FINAL은 여전히0이다. CODE/WHY/POLICY/TRACE/ABS의 중요군 조건을 충족하지 않았고 protocol freeze/runtime binding도 없다. N11-A–D는 미체크로 유지한다.

사람은 위 관측 범위와 별도의 과거 DEV/개념적 중복을 검토하여 각 후보의 gold·cluster·독립성·오염을 승인/수정/기권해야 한다. 그 뒤 나머지 실제 POLICY/TRACE/ABS 및 독립 gold 범위를 준비하고 전체 입력을 사전 동결한다. 현재 DEV7개와 N12 기존27개·추가6주장/12관계 판정도 대기다.

**현재 요청5/15·전체8/18 완료. 남은 전체 N08·N09·N10·N11·N12·N14·N15·N16·N17·N18, 10개. 다음은 N11/N12 실제 판정 반영과 native Linux CI 실행이다.** 새 출처 대조를 전체 평가·검색 개선·운영 출시 완료로 바꾸지 않는다.

## 공개 전송 상태

사용자의 “다음 진행”을 앞서 제시한 d08cb384의80커밋 공개 푸시·draft PR·CI 실행 지시로 해석해 전송을 시도했으나, 자동 승인 검토가 실제 실행 전에 거부했다. 검토기는 기존 명시 승인이329becf3에 한정되며9,031개 파일/평가 원자료의 새 공개 전송을 승인하지 않는다고 판단했다. 원격 푸시/PR 생성/CI dispatch는 수행하지 않았다. 해당 구체 범위의 명시적인 승인 질문을 다시 제시했으며 응답을 기다린다. 우회 전송은 수행하지 않는다. 추가 BGE 및 이 출처 대조의 로컬 커밋은 요청한 d08cb384 전송 범위 밖이다.
