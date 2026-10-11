# N11 검토 대기 코퍼스·색인·실행 신원 연결

2026-10-08 · **원문/색인 범위 보완과 무답 확인 자료 보호 검증 완료 / N11 전체 미완료**.

## 목적과 발견한 차이

평가 질문의 근거가 실제 보관·색인 범위에 있어야 검색 실패를 해석할 수 있다. 입력 승인·독립성·오염 검토는 별도이며 색인 존재가 gold의 정확성을 보증하지 않는다. 이전 [N12 partial lock 후보](./N12-PROPOSED-LOCK-PREPARATION.md)를 N11 평가용으로 그대로 승인하지 않았다.

기존 N11 책의 DEV7+FINAL후보9, 총16참조를 대조했다. 인용 대상14개는 이전 코퍼스에 보관돼 있었으나 그중9문항이 참조하는6Go파일은 CKG/CKV 선택 범위에 없었다. resources.go/gc.go/semantic readonly.go/trace.go/knowledgepack lock.go/instances.go가 해당한다. ABS2개의 summary.json/C1-OPERATIONS-REVIEW.md는 이전 보관 범위에도 없었다. 책의 c8d86be7과 이전 후보의32d7e264도 다른 commit이다. 원문이 같다고 source commit 불일치를 무시하지 않았다.

## 코퍼스 설계와 실제 연결

기존 독립 준비 코퍼스의1,416파일을 그대로 두고, ABS 검토 확인 자료2개만 c8d86be7의 정확한 바이트로 추가해 별도 준비 Git19803801을 만들었다. 원본 저장소의 전체 이력/평가 raw/질문책은 입력 코퍼스에 복사하지 않았다. 승인 Go29와 기존 원문·policy/ADR 초안·lock은 유지했다. capture 보호와 임계치도 변경하지 않았다.

새 filelist는 기존58개에6Go+확인 자료2개를 추가한66개다. 실제 CKG59Go와 CKV63Go/Markdown 입력은 각 지원 형식의 선택 집합과 SHA가 정확히 일치한다. ontology/spec YAML2개와 ABS summary JSON1개는 보관하지만 vectorized로 세지 않는다. JSON은 원문 기록을 확인하는 자료이며 답변 인용 gold가 아니다.

질문/후보답/원문span/제안군·제외2항목은 그대로 보존하고 새 책의 source_commit만 새 코퍼스에 재결합했다. 원래 책·프로토콜·이미 승인한 입력과 FINAL 원자료는 바꾸지 않았다. 사람이 이전 시점에 한 판정을 이 새 범위/책/설정의 승인으로 자동 확장하지 않는다.

| 새 held 후보 | 값 |
|---|---|
| project/source | knowledge-system / committed |
| source_commit | `19803801bdde0d283b262f6c2d0f7d5656923135` |
| snapshot_id | `b1204cf14904afa85250340a6420106fb5662e615acae28efdf0c2fdbbd71222` |
| dataset_id | `ee6f91bef76a9d3867983f0f968986919b2b1241dea7d7725bda27dee6808218` |
| 보관 | repo1,418+knowledge origin1 =1,419항목 |
| CKG | 59Go, 10,830nodes/25,179edges, parse/unresolved0 |
| CKV | 63입력/785chunks, 667/667canonical code symbols, floor1.0 |
| 원문 대조 | positive14+ABS basis2 =16/16 file/spanSHA 일치 |
| 실제 색인 연결 | positive14/14가 겹치는 CKG canonical·동일canonical CKV chunk 존재 |

겹치는 심벌/청크의 확인은 전체 gold 줄이 모두 색인됐다는 뜻이나 실제 검색 recall 결과가 아니다. 범위·원문 SHA·canonical ID·chunk ID와 겹침을 [참조별 원자료](../../system/eval/b0-knowledge-system/refactoring-n11-held-corpus-2026-10-08/corpus-coverage.json)에 기록했다. ABS basis2는 positive evidence=[]/strict_zero_citations=true를 유지한다. 자료에 값이 없다는 확인만으로 실제 운영 전체에 값/담당자가 없다고 승인하지 않는다.

## 무답 확인 자료 검사의 RED→GREEN

검사기는 positive evidence의 원문SHA/줄을 확인하지만 선언한 `abstention_basis_sources`를 확인하지 않았다. 검토 SHA까지 일관되게 갱신한 합성 control의 잘못된 fileSHA/spanSHA/range/bool좌표/missing-path/unsafe-path/empty/null/object/string 10개가 input_ready=true로 통과했다. runner도 잘못된 확인 자료를 넣은 control에서 예약·출력 이후 가짜 child callback에 진입했다. 실제 CKS/model 질의·사람 승인·신규 독립 표본이 아니다.

같은 원문 검증 함수를 positive gold와 **선언된 ABS 확인 자료**에 적용했다. 비어 있거나 잘못된 catalog·파일/span 불일치를 거부하고 해당 문항을 유효한 독립군 분모에 넣지 않는다. 텍스트 basis만 있는 기존 형식의 호환은 유지한다. 제공된 원문 참조의 일관성을 확인하며 사람의 무답/권위·범위·시점 판단이나 검토자의 실체를 인증하지 않는다.

25개 입력/runner 시험 통과(추가3시험; 10개 변조 subcase와 실제 runner의 child/output/reservation0 포함). valid basis의 ABS evidence는 여전히 빈 배열이고, 기존 cluster/제외/동결/모델·임계치/FINAL예약·OS flock/SIGKILL 제어를 유지했다. 제품 Go 코드/바이너리는 변경0; Python checker/시험3개만 변경했다. 이 변경에 전체 Go 회귀를 새로 실행했다고 표시하지 않는다.

## 검증한 세부 단계

- [x] 이전 후보의 색인6Go 누락/ABS2보관 누락/sourcecommit 불일치 확인; 원래 원문16ref와 보완2파일 SHA 대조.
- [x] 생산 setup의 committed·strict complete embeddings·canonical floor1.0·hold-for-review 통과; source1,419·선택66·실제 CKG59/CKV63 범위/SHA 대조.
- [x] production retained API·read-only 실제 CKG/CKV DB로 원문16/16, positive14/14 canonical/chunk 겹침 확인. fileSHA/spanSHA/missing-file/range4종 보관 근거 제어 거부.
- [x] ABS basis checker RED10+runner1→GREEN25. 원문 참조를 답변 인용으로 전환하지 않고 파라미터/과거 제외·예약 보호 유지.
- [x] 생산 semantic CLI의 proposed v4 추출/저장·27문서 chunk 연결과 config API의 K10 확인. semantic current 없음, 사람verified/precision0/null.
- [x] dataset/source/model/config/binary SHA의 **실행 신원 초안** 준비. 실제 checker는 source/basis 오류0이며 미승인 입력/inconclusive를 반환. DEV와 FINAL runner 모두 exit2·child/output/registry/lock 생성0. request_scope와 동결은 미판정/null, execution_allowed=false.
- [x] 원본 임시 소스·색인·semantic DB를 숨긴 독립 archive 재생: archive의 질문책·원문16·색인14연결·semantic DB/current 부재·v4 source/CKV/lock 검증 통과. 새 모델 호출0, 원본 복원.

CKS/CKG/CKV는 이전 실제 빌드3e204ee3의 binary SHA와 모두 같음을 확인해 재사용했다. 새 Go 바이너리 빌드나 binary 출처의 사람 수용이라고 쓰지 않는다. sanitizer 원문과 baseline protocol/model/arm/임계치·승인 입력5개도 유지했다.

## 계측·경고·준비 오류

| 단계 | relay HTTP | embedding HTTP | 시작 probe 입력 | 색인 입력 | exit |
|---|---:|---:|---:|---:|---:|
| 처음 YAML 초안 오류 |5|1|1|0|1|
| 수정한 held 빌드 |216|70|2|785|0|
| 이번 합계 |221|71|3|785|—|

relay 관측 HTTP221 외에 실행 전후 metadata GET4회를 별도 기록해 알려진 모델 HTTP 전체는225회다. metadata GET은 embedding/질의로 세지 않는다. 실제 embedding 입력 합계788개 성공/오류0·truncate:false다. 승인 BGE-M3 digest790764642607…16bab/native/target1024·Ollama0.35.1/runtime8192/8192를 유지했고 metadata 전후 동일했다. 초기 실패의 probe1도 비용 분모에 포함했다. 과거 N12 index/model 관측은 별도 기록이며 새 DEV/FINAL 검색·질의 비용/p95 또는 사람 활동 시간으로 사용하지 않는다.

전체 Go 감사는1,051 대59의 DRIFT/exit1, 선택 밖992 missing/extra0다. 선택 범위의14gold 연결과 전체 저장소 감사 PASS를 혼동하지 않는다.

초기 filelist에 extra globs를 exclude_globs 뒤에 추가한 YAML 오류, 첫 unittest 호출의 모듈 경로 누락, config/archive driver의 OpenStoreReadOnly context 인자 누락은 준비 도구 오류다. 각각 원출력/원인을 보존하고 수정해 정상 검증했다. 첫 import 오류와 합성 RED 제어의 실제11실패를 구분한다. archive 실패에서도 finally가 원본을 복원했다. 제품 실패·사람 반려나 성공 제어 수로 합산하지 않는다.

## 보관·바인딩·남은 수용

[archive](../../system/eval/b0-knowledge-system/refactoring-n11-held-corpus-2026-10-08/held-candidate.tar.gz)는13,880,522bytes/1,434files, SHA256 `1328685a0e91d4fe9befaa57c90637d4eb724ed3b8e9d3ac44fb6c773e8c0fe2`이다. source/history·CKG/CKV·hold/identity에 proposed semantic DB/투영·질문/프로토콜/설정/실행binding을 별도 review-inputs로 보관했다. 질문/gold 책은 캡처·색인된 원문 코퍼스에 들어 있지 않다. logical source root와 config 절대 경로는 원래 좌표를 유지한다. 이것을 독립 운영 소비자 이동 수용으로 계산하지 않는다.

[검증](../../system/eval/b0-knowledge-system/refactoring-n11-held-corpus-2026-10-08/runtime-preflight.json), [모델 계측](../../system/eval/b0-knowledge-system/refactoring-n11-held-corpus-2026-10-08/model-call-accounting.json), [원본 없이 재생](../../system/eval/b0-knowledge-system/refactoring-n11-held-corpus-2026-10-08/archive-originals-hidden.json), [새 질문책](../../system/eval/b0-knowledge-system/refactoring-n11-held-corpus-2026-10-08/candidate-question-book.json), [새 프로토콜 초안](../../system/eval/b0-knowledge-system/refactoring-n11-held-corpus-2026-10-08/candidate-protocol.json).

원장 checker는 새 Python source를 아직 커밋 근거에 결합하지 않은 상태에서 실제 exit1로 거부했다. 검증한 새 소스/문서/원자료 manifest를 커밋하고 active_source_binding에 결합한 뒤 정상 확인한다. 단순히 기존 sourceSHA를 지우거나 checker를 우회하지 않는다.

N11-A–D는 unchecked다. 새 독립 후보를 추가한 것이 아니므로 DEV7/FINAL후보9·제안군6·승인FINAL0 유지, 각 중요군 승인 독립단위0/10이다. 다음은 새 범위/설정·gold/오염·무답/군과 조회 scope의 실제 판정 및 충분한 독립 FINAL 보강·전체 사전 동결이다. 운영/사람/질문 수용을 기술 연결 PASS로 대체하지 않는다.

현재 전체8/18·요청5/15 완료, 남은 전체10개:

| 우선순위 | 작업 | 남은 수용 |
|---|---|---|
| P1 | N11 독립 평가 | gold/군·오염·새 범위/scope 판정, 중요군별 충분한 독립 FINAL·사전 동결 |
| P1 | N12 실제 파일럿 검토 | 내용/관계·권위/범위·담당자/시점, 승인 overlay/lock |
| P2 | N08 검색 품질 | DEV 개선·독립 FINAL 회귀 |
| P2 | N09 무답·기권 | strict인용0·실제 답변 기권 별도 판정 |
| P2 | N10 비용·지연 | 동일8arm 비용·실패 분모·warm/cold p95 |
| P3 | N14 실제 의미 수용 | 실제 patch/run·criterion 사람 판정·승격/반려/rollback |
| P4 | N15 지원 환경·외부 소비자 | 지원 규모/모델·독립 upgrade 수용 |
| P4 | N16 운영 권한·고지 | 역할·키 신뢰/교체/폐기·전이 고지 |
| P4 | N17 운영 복구 | 담당자/RTO/RPO·대상호스트 운영 drill |
| P5 | N18 최종 수용 | 모든 근거/제약·기본값·출시 판정 |
