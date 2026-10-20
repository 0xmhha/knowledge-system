서버 종료 수정 후 검증: 동일42질문·378호출 세션2회에서756팩 검증 실패0·질의/종료 오류0. Close11.1/10.9ms, 별도 EOF·대기/실행 SIGTERM 모두 exit0·강제 종료없음·reader lease 해제. 이전 인용/본문756쌍 변경0. 기존 두 종료 실패를 보존했고 material commit d0b7ad2a의 SHA1839개와 진행 guard PASS를 확인했다. [진행 작업표](./GO-STABLENET-DATASET-COMPARISON.md).

# 현재 평가 결과와 판정 한계

2026-10-15 추가 범위: 사용자가 온톨로지 효과 평가를 우선 필수 검증으로 채택했다. [OE-01 명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md#oe-01--온톨로지-효과-평가--p0-필수-단일-프로젝트부터-시작)와 [전용 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)에 단일 프로젝트 off/on 및 향후 다중 프로젝트 비교의 품질·근거·오인·지연 측정을 등록했다. [새 로컬 개발 관찰](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)은384회와 별도 정확 용어8회를 수행했다. 실패/누락0·인용3,616개를 검증했으나 활성288회 no_match·순위 변화0으로 개선을 관찰하지 못했다. 확대 범위·독립 품질·실제 답변·다중 프로젝트 평가는 남고 품질 inconclusive·기본 off를 유지한다. 아래 과거 수치/판정과 기본 off는 이 요청만으로 변경하지 않는다.

개발 단계의 완료는 시험·관측·부족분과 판정을 기록했다는 뜻이다. 품질 개선, 사람의 독립 검토, 기준 충족, 운영 활성화 또는 공개 배포가 자동 승인됐다는 뜻이 아니다.

## 온톨로지 관측

보관된 WBFT off/on 실측은13질문, arm별 warmup2·검색5·warm20·cold0으로702응답/351쌍이다. 같은 소스·데이터셋·스냅샷·실제 BGE-M3 digest/native1024·질문·설정을 사용했다. 반복을 독립 표본으로 세지 않는다. [요약](../../system/eval/b0-knowledge-system/ontology-o04-completion-2026-10-13/result-summary.json)과 [판정](../../system/eval/b0-knowledge-system/ontology-o04-completion-2026-10-13/final-verdict.json)에 당시 binary·모델·입력·원자료 좌표가 있다.

| 관측 | off | on |
|---|---:|---:|
| 제안10군 평균 Recall@10 | 1.000000 | 1.000000 |
| Precision@10 | 0.147778 | 0.147778 |
| expected reciprocal rank@10 | 0.658333 | 0.645833 |
| 전체 warm p95(ms) | 444.818 | 482.868 |
| 요청 scope embed 호출 | 1434 | 1548 |
| 요청 scope tags 호출 | 2868 | 3096 |

오류·누락0, 재채점 일치, trace2개는 missing_code/acceptance_evaluated=false다. 공개 인용·본문은54쌍에서 변했고297쌍은 동일했다. 전체 p95 비율1.085541이지만 IC04/17/18은1.2817/1.3236/1.2988로1.25를 넘었다. **전체 성능 수용은 false**다. warm/shared-host·비무작위 순서·질문별20회·활동시간376.2초·scope 밖 constructor probe 미계측을 유지한다.

승인 독립 단위0·사람의 독립 내용 검토0·새 FINAL0이다. 제안10군이나 행 교집합은 독립성 또는 사실 답변의 정확성을 증명하지 않는다. 품질은 **inconclusive**, 온톨로지 기본값은 **off**다. 실제 store의 검토된 코드/시험 관계0을 연결 완료로 재해석하지 않는다.

## 기존 수용 범위

- 타입 선언 삽입은 품질 개선 근거가 없어 `retrieval.type_neighbors` 기본 off다. 이전 FINAL에서 순위 하락을 관측했다.
- 미지정 사실 guard의 기계 동작과 사람의 답변 기권은 별도다. 생성 답변 평가는 없고 답변 수준 기권은 null이다.
- 실제 파일럿65건은 AI 사전검토를 사용자가 일괄 채택한 범위다. 독립적인 사람의 항목별 내용 검토 완료로 확대하지 않는다.
- 정확한 테스트 실행·기준별 판정·보류 후보·승격·rollback은 실제 파일럿 범위의 증거다. 다른 프로젝트의 모든 기준을 통과했다는 뜻은 아니다.
- 로컬 개인 운영은 사용자가 수용했다. native Linux CI의 mock 설치/복구 계약과 실제 BGE 규모·외부 조직 운영 수용을 구분한다.

각 증거는 [원장](./REFACTORING-PROGRESS.json)의 기록된 revision에서 확인한다. 현재 코드의 CI 통과는 과거 모델 평가를 다시 실행했다는 뜻이 아니다. 새 corpus나 독립 품질 입증은 새 입력·source/model pin·사전 기준·검토가 필요한 별도 작업이다.
