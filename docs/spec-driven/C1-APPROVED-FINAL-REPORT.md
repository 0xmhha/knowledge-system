# 승인 FINAL 결과와 출시 보류 판정

2026-10-06 · C1-01/02/03. **4,800회 수집 완료, 실행 상태 failed_preserved. 품질 판정 fail, 중요 질문군 inconclusive, 온톨로지 기본값 disabled 유지.** 수집·감사 작업의 완료를 평가 합격이나 운영 출시 승인으로 합산하지 않는다.

동결 CKS SHA `3b80c36d77b5c052894058ec59b12d64f2e5f9e5800aa11314be9e362189d0e6`, BGE-M3 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, Ollama0.35.1, 승인 K10/F01 K5·8회전 arm·warmup2/검색5/warm20/cold3을 유지했다. 정적8문항1,920회와 독립 동적6가족/8상태2,880회다. 5개 승인 입력·전체 native 저장소·1096제품 소스·바이너리 SHA가 그대로다. 모델을 언로드하지 않았다. 공유 M2 Max 환경이며 물리 자원을 독점하지 않는다.

[전체 캡처·해시](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/final-after-development-freeze/artifact-manifest.json), [감사·direct·v1 원자료](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/final-oracles/artifact-manifest.json), [최종 집계](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/final-after-development-freeze/final-summary-corrected.json), [같은 축 비교](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/final-after-development-freeze/same-axis-comparisons.json)를 연결한다. gzip은 무손실·mtime0이며 원본/압축 SHA를 함께 기록했다. 최초 집계의 정적 source 검증 분모 오류도 보존했고 corrected 집계가 최종 근거다.

| 항목 | 실제 결과 | 판정 범위 |
|---|---|---|
| 전체 SDK/공개 Go | 4,800회, valid_contract4,560 / 도구 오류240 | 정상 팩의 출처·범위·본문·integrity 유효. 오류 envelope를 유효 팩으로 세지 않음 |
| 동적 source/가족 | 2,880회, 위반0·계측 누락0 | F02 꼬리·F03 proposed 비승격/기본 후보·F04 상태·F05 프로젝트·F06 acceptance/test trace 비승격 |
| 정적 source | 1,680개 정상 팩 검증 / 240개 오류 | B0-POLICY-03 모든8arm·모든30회 오류. 누락 슬롯0 |
| 양성 정적6문항 | POLICY-02만 기대 인용 회수. Recall1/6, 평균 기대 역순위0.5/6 | CODE02/03, WHY01/02 miss; POLICY03 v2 error는0점. 모든8arm 동일 |
| 정적 v1 | 8×5 실제 평가·현재 snapshot. 양성1 pass/5 miss | v2 오류와 구분. 생성 답변·정책 집행/수용 승인을 판정하지 않음 |
| 두 기권 문항 | v1 인용0개 guard 둘 다 fail. v2 각arm10회 중0 pass | 빈 양성 gold를 Recall/MRR 성공으로 합산하지 않음. 사람의 답변 기권은 별도 |
| F05 A/B·F06 | 모든8arm 기대 인용 Recall1/MRR0.75 | 두 기대 인용 역순위의 평균. 사람의 정책/답변 승인이 아님 |
| raw20 | F01 filtered/exact@5 자격 충족. F02 en/ko만 순서 불일치 | 같은 top5 집합·거리, noise 동점 내 순서 차이. 원래 exact=false 유지 |
| F04 direct CKV | old index/new source에서 fresh=false·old 인용 stale | 실제 FINAL 상태, 구버전 인용을 최신으로 승격하지 않음 |
| v1 SDK40 | measurement ID/backend/원문 결합·실패0 | primary K10과 auxiliary K6, 추가 K10 호출도 원자료 유지. paired 지연 밖 |

## POLICY-03의 실제 거부 원인

[진단 원자료](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/final-oracles/approved-final-oracles/static-v1-sdk.json.gz)와 별도 `final-policy03-diagnostic` 기록을 보관했다. 선택된10인용을 실제 Go `ReadRetainedLineSpans`로 읽었고 모든 출처가 검증됐다. 원문33,104바이트에서 실제 `evidencev2.Build`가 **budget_exceeded: retained evidence exceeds body limit**를 반환했다. 현재 public v2는 이를 generic `v2_evidence_failed`로 매핑한다. source/identity 혼입이나 파일 손실로 확인된 실패가 아니다.

32,000바이트 상한을 올리거나 질의·gold·K·제품 코드를 바꿔 FINAL 점수를 복구하지 않았다. 이 사례는 현재 후보의 예산 경계와 질의 처리 제한으로 공개하며 완전한 사용 가능성/품질 합격을 주장하지 않는다. 이후 예산 선택 동작을 개선하면 새로운 개발 버전과 사전 검토된 독립 최종 입력이 필요하다. 이 FINAL을 새 버전의 독립 holdout이라고 부르지 않는다.

진단1회에서 baseline_off 원장 경로를 재사용해9개 로그를 추가한 작업 실수를 별도로 기록했다. 전체 추가 후 파일·진단 tail·기존 prefix를 모두 보관하고, 실제 캡처240개 measurement ID와 정확히 일치하는 prefix를 복원했다. 원본 report/rows는 그대로다. `footprint-restoration-note.json`에 세 SHA와 무손실 결합 증명을 남겼다. 해당 진단을 공식 warm/cold에 합산하지 않는다.

## 비용·같은 축 비교

실패 호출은 검색 분모에서0점으로 남기고, warm/cold 통계는 성공 조건부로 기록한다. 정적 각arm warm160개 중140개 성공,20개 오류; cold24개 중21개 성공,3개 오류다. 동적 각arm warm240개, 전체 성공 warm380개다. 실패20개의 빠른 거부 시간을 성공 p95에 끼워 넣지 않는다. 사람 검토 시간·비용은 미측정이다.

| 기준: baseline_off | 정적 p95비 | 동적 p95비 | pooled p95비 |
|---|---:|---:|---:|
| baseline_on | 1.069 | 1.238 | 1.066 |
| concept_text_off | 1.016 | 1.151 | 0.998 |
| concept_text_on | 1.064 | 1.533 | 1.068 |
| relations_off | 1.011 | 1.018 | 0.995 |
| relations_on | 1.075 | 1.311 | 1.063 |
| combined_off | 1.005 | 1.168 | 0.995 |
| combined_on | 1.073 | 1.511 | 1.072 |

같은 pack_on 축의 ontology 비교는 dynamic concept_text/relations/combined p95비1.238/1.059/1.220이다. 같은 ontology 축의 knowledge_on/off 비교는 baseline/concept_text/relations/combined에서1.238/1.331/1.288/1.293이다. 원래 baseline_off 비교와 모두 보존하며 어떤 축이나 pooled 결과 하나를 사후 선택해 포괄적인 비용 합격을 만들지 않는다. 기존 한계1.25와 population 차이를 그대로 표시한다. 원시 p50/p95·cold·backend·크기는 집계/개별 보고서에 있다.

## 최종 결정과 미완료 범위

- C1-01: 승인 조건의 독립 재평가·실패 포함 원자료/감사 완료. 240개 실패를 성공으로 바꾸지 않음.
- C1-02: 실제 실행 오류와 엄격 인용 guard 실패가 남아 **품질 fail / 출시 보류**. 중요 질문군 code2/why2/policy2/기권2, 동적 양성4가족은 소표본 inconclusive. 반복을 독립 표본으로 세지 않음.
- C1-03: 사용자가 승인한 n<10 규칙과 실패/불확실성에 따라 **disabled 유지**. 기본 활성·운영 출시를 승인하지 않음. 구현 기본값도 opt-in이다.
- B1-04/07/08: 사람의 출력 주장·정책·기권 판정/비교 종료 남음. B1-06은 [비용 보고](./B1-COST-REPORT.md)에 실측과 미측정/null을 연결해 완료했다. [검토표](./B1-APPROVED-OUTPUT-CLAIM-REVIEW.md)의 입력 승인과 출력 승인을 분리한다. 실제 FINAL 기권 질문의 운영 담당자/키 소유자는 근거 없이 지정하지 않는다.
- C1-04/06: [최종 동결 패키지](./C1-FINAL-PACKAGE-VALIDATION.md)의 최신3대상 preview·실모델·전체 strict 비용·복구를 완료했다. C1-07은 [지원/known limits·출시보류 보고](./C1-FINAL-DELIVERY-REVIEW.md)를 명시적 잔여 포함 조건으로 완료했다. C1-05의 운영 서명·신뢰/적합성/담당자·scope 결정은 남는다.

현재 작업 범위는 계속 [작업리스트](./EXECUTION-WORKLIST.md)를 따른다. 실패 판정을 완료한 것은 제품 품질을 통과시킨다는 뜻이 아니다.
