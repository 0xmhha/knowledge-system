# 온톨로지 단일 프로젝트 개발 관찰

2026-10-15 기준, 현재 승인 정의로는 이번 자연어 질문의 검색 개선을 관찰하지 못했다. 고정 질문12개 ×4모드 ×8호출의384회를 모두 기록했고 실패·누락은0이다. 활성 모드288회는 모두 `no_match`였으며 off와 비교한 검색 순위 변경도0이다. 기본 off를 유지한다. 이 결과는 독립 품질 수용이나 OE-01 전체 완료가 아니다.

명세는 [OE-01](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md#oe-01--온톨로지-효과-평가--p0-필수-단일-프로젝트부터-시작), 진행 상태는 [전용 원장](./KNOWLEDGE-ROADMAP-PROGRESS.json), 원자료는 [manifest](../../system/eval/operational-preparation-2026-10-15/ontology-effect-dev/manifest.json)와 [재계산 요약](../../system/eval/operational-preparation-2026-10-15/ontology-effect-dev/summary.json)을 따른다.

## 자료와 비교 조건

- source commit `baca2869118774c5f6219932dc76e197a47e6ecd`, project `p-3fa6368dd3f6757c6b6a86121f0a9b7d`.
- dataset `4894eaf2f1f14b049d8a262f04327f92e7eacf73a5b4cd0401f9f9df30eb9ff4`, snapshot `99ad6ae09c82fb2cfa2c70b405bde55f5caf112116562249463903aadff43bb4`.
- 앞선 정책 평가의 `0bf2f4d1`과 Git 차이는 승인 의미 문서2개 추가뿐이다. 기준 manifest의 비시험 Go882개는 원문 blob까지 동일함을 확인했다. 기존 retained-corpus dataset `474440d4…`의 평가로 합산하지 않는다.
- 원래 평가 버전을 APFS 별도 복제하고, 보관된 승인 ontology와 같은 바이트로 semantic DB를 새로 구축했다. 검토된 개념4개, 검토된 구현 관계0개다. 의미 DB 구축의 `activated=false`, 원래 current와 검토 제한을 보존했다.
- 로컬 `bge-m3:latest`, digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 1024차원이다. 유료 LLM이나 외부 모델 API를 호출하지 않았다.
- off/concept_text/relations/combined 외 구성은 동일하다. 질문별 모드 순서를 교대했다. 각 질문·모드에 준비1회, 검색2회, warm 지연5회다. cold/실제 LLM 답변/정답 승인/운영 전환은 이번 범위에 없다.
- 공개 질문에 정답을 전달하지 않았다. 실행 전 별도로 고정한 AI 원문 위치 대조표는 비완전한 진단 기준이며 사람 승인 정답이 아니다. 긍정 질문8개는 기존 WBFT4개 주제에 겹치며, 제어 질문4개는 답변 평가에서 미채점이다.

## 관찰 결과

| 모드 | 호출/실패 | 개념 매칭 | 위치 hit@10 | 위치 RR@10 | warm p95 |
|---|---|---|---|---|---|
| off | 96/0 | 0 | 0.625 | 0.2073 | 497.1ms |
| concept_text | 96/0 | 0 | 0.625 | 0.2073 | 552.2ms |
| relations | 96/0 | 0 | 0.625 | 0.2073 | 556.1ms |
| combined | 96/0 | 0 | 0.625 | 0.2073 | 761.7ms |

위치 지표는 긍정 질문의2회 검색 평균을4개 주제로 집계했다.5/8질문에서 지정한 원문 위치와 겹치는 인용을 상위10개 안에서 찾았다. D02의 정족수 변형, D06의 반복 투표, D08의 모니터링 동기화 위치는 이 대조표 기준 상위10개에 없었다. 이는 해당 위치의 회수 진단이며 precision/recall이나 전체 관련 근거·생성 답변의 정확도를 뜻하지 않는다. 제어 질문의 검색 결과가 있었다는 사실로 답변 기권 성공/실패를 판단하지 않는다.

각 활성 모드에서 off와 짝지은 검색24개 전부 인용 순서가 같았다. 본 시험 인용·본문 각각3,616개를 원래 retained manifest/blob의 프로젝트·버전·파일·줄 범위·해시와 대조했고 응답 무결성 해시도 검증했다. source blob 검증은 추가 진단까지 포함해53개다.48개 본 시험 세션과4개 추가 진단 세션은 EOF로 정상 종료했다.

warm은 모드당60개 관찰이다. p95의 off 대비 비는 concept_text1.111, relations1.119, combined1.532였다. 같은 머신의 작은 표본이며 시간대·캐시·공유 자원의 영향이 있어 인과적 성능 차이나 통계적 회귀 판정으로 확대하지 않는다. 실제 embedding 서버 요청 수·토큰/금액 비용은 계측하지 않았다.

## 별도 정확 용어 진단

본 결과를 본 뒤 등록된 선호 용어 `쿼럼`을 고정하고 모드별 준비1회·검색1회, 총8회를 추가했다. 이 진단은384회 분모나 품질 지표에 합치지 않는다. [별도 protocol](../../system/eval/operational-preparation-2026-10-15/ontology-effect-dev/mechanism-probe/protocol.json)에 사후 선택임을 명시했다.

off 외3개 모드는 `wbft-quorum`1개를 매칭했다. concept_text/combined는 정의를 사용하는 추가 검색1회도 수행했다. 최종 boost는 모두0이며 적용된 구현 관계도0이다. 따라서 저장소·매칭·정의 검색 연결은 동작하지만, 이 진단도 검색 개선의 근거가 되지 않는다. `active` 응답은 기능 실행 상태이며 품질/최종 순위 변경 판정이 아니다.

[현행 matcher](../../internal/system/semantic/matcher.go)는 등록된 용어를 질의 문자열에서 찾는다. 예컨대 질문의 `정족수`나 `라운드를 바꾸면`은 등록된 `쿼럼`·`라운드 변경`과 자동으로 같은 개념이 되지 않는다. 본12개 질문을 실행 뒤 변경하거나 새 별칭을 승인된 정의에 끼워 넣지 않았다. 관계 모드의 상태 조회는 `no_reviewed_implementations`를 보고하며, 정확 용어 요청에서 개념 매칭이 active여도 구현 연결이 새로 생기는 것은 아니다.

## 복제본 초기 오류와 보존

최초 version 디렉터리만 복제하면서 부모 reader lease가 빠져 MCP 초기 연결이 종료됐다. 이때 검색은0회였고 의미 DB 구축만 성공했다. 표준 복구 명령은 검토 대기 후보의 활성화를 거부했다. 검토 표식은 유지하고 부모의 읽기 lease 파일을 독립 파일로 복원한 뒤, 활성화 없이 읽기 시험을 진행했다. 초기 오류와 거부·복원 경위는 [초기 시도 기록](../../system/eval/operational-preparation-2026-10-15/ontology-effect-dev/initial-attempt.json)에 남겼다. 이후52개 평가 세션의 정상 EOF 종료와 구분한다.

원래 평가 버전 및 복제본의 graph/vector DB·manifest·identity·retained source manifest·reader/review 표식8개가 전후 같은 해시임을 확인했다. semantic DB도 전후 동일하다. 큰 graph/vector DB는 임시 시험 경로에 유지하고, 작은 semantic DB·입력 좌표·사용한 원문·원시 응답은 Git 증거에 보관했다. gzip 파일은 원시 응답을 수정하지 않고 압축했으며 압축 전/후 해시를 모두 기록했다.

## 단계별 체크와 다음 작업

- [x] 승인 정의와 보관 원문·DB 좌표 정합, 별도 후보 준비.
- [x] 실행 전 질문·위치 대조표·순서·분모 고정 및4모드 readiness 확인.
- [x] 본 시험384회, 별도 정확 용어 진단8회와 세션 종료 기록.
- [x] 원문/응답 검증, 위치·순위·지연 재계산, 원자료 보존.
- [ ] 독립 정답·관련 근거 전체 집합, 한영·다의어 범위 확대와 실제 답변 효과 검증.
- [ ] 자연어 개념 해석·검토된 구현 연결·최종 후보에 도움이 되는 정의 검색을 보완하고 새 입력으로 재평가.
- [ ] KM-05/06 이후 다중 프로젝트 개념 매핑·오인·비교 효과 평가.

위의 완료 체크는 이 개발 관찰의 하위 실행만 뜻한다. OE-01.1–03은 넓은 평가 명세의 일부를 수행한 상태로 남기고, OE-01.4는 미실행이다. 새7개 기능 완료0/7과 기존 독립 수용0/9를 유지한다.

다음 구현은 KM-01의 scope/key 조사와 collection 격리 계약이다. 이후 KM-02 구축·갱신 도구, KM-03 지식 매니저, KM-04 지속 갱신, KM-05 공통 개념·프로젝트 매핑, KM-06 비교 MCP 순서로 이어간다. 위 온톨로지 결손은 KM-05와 후속 OE-01 평가 입력에 연결한다. 정책 생성기의 잘못된 참조 재생성 보완도 별도 잔여다. 운영·독립 수용9개와 선택 후속12개는 [전체 작업표](../REMAINING-WORK.md)에 유지한다.

재검증: `python3 scripts/check-ontology-effect-dev.py`, `python3 -m unittest discover -s scripts -p test_ontology_effect_dev.py`. 검사기는 누락·중복 호출, 바뀐 요청/구성, 타 프로젝트 인용, 원문 변조, 가짜 지표·승인·완료 승격을 거부한다.

검증 결과: 새 관찰 증거 검사와 허위 완료 방지10개, 문서 계약20개가 통과했다. 문서123개와 기존 source/input/bundle 결속2,069개를 확인했고 기존 구현·운영 수용 원장을 완료로 승격하지 않았다.
