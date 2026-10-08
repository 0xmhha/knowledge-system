# Neighbors 입력 인수 해시 결합 검증

2026-10-05 · FIX-21 · B0-06 계측 보완. [요약](../../system/eval/b0-knowledge-system/neighbor-binding-m2max-2026-10-05/summary.json), [실제 입력 20개](../../system/eval/b0-knowledge-system/neighbor-binding-m2max-2026-10-05/source-cases.json), [실제 Go 재생](../../system/eval/b0-knowledge-system/neighbor-binding-m2max-2026-10-05/direct-go-replay.json), [39개 원자료 해시](../../system/eval/b0-knowledge-system/neighbor-binding-m2max-2026-10-05/evidence-manifest.json)를 기록했다. 공식 품질·출시 판정은 null이다.

이전 [선택 인용 8개 재생](./B0-NEIGHBOR-REPLAY-DIAGNOSTIC.md)은 neighbors 원장의 입력 SHA/바이트가 비어 있어 실제 확장 seed와 직접 결합하지 못했다. 이제 계측 scope가 있는 호출에서 `contract.Citation`을 Go `encoding/json.Marshal`로 직렬화하고 기존 `input_sha256`·`input_bytes`에 SHA/길이만 기록한다. 파일명·줄·commit 원문을 backend 이벤트에 추가하지 않는다. scope가 없으면 직렬화하지 않는다. 원래 인수/옵션 전달과 반환·오류 의미는 유지한다.

바이트 형식은 `file,start_line,end_line,commit_hash` 필드 순서의 compact JSON, UTF-8과 Go 기본 HTML·줄 구분자 이스케이프다. `input_bytes`는 이 인수 표현의 바이트 수이며 원문 파일·네트워크 크기가 아니다. 이 v1 citation 해시는 project/dataset 전체 좌표를 포함하지 않으며 봉인된 설정·dataset identity·원응답 좌표가 별도로 결합한다. 과거 SHA 없는 로그를 소급 복원한 것으로 표현하지 않는다.

수정 전 회귀 시험은 exit1이었다. 다른 줄/commit이 빈 입력으로 합쳐졌고 수정 후 구분된다. 경로 비노출·원래 인수/옵션 전달을 확인했으며 backendmeasure/composer/stage3/mcp/eval/evalcli 시험, backendmeasure/mcp race, 관련 vet와 cks 빌드를 통과했다. 시험 실패 원문과 실제 종료 코드를 보존한다. 조립 보조 도구의 `rows`/`records` 필드 착오는 별도 실패 기록 후 수정했으며 제품·SDK 실패에 합산하지 않는다.

같은 BGE-M3 digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, 기존 부분 13,575청크/102,112 graph node, DEV B0-CODE-01 문의와 동일한 K/범위에서 baseline·knowledge 각 계측 on/off를 실행했다. 계획4/관측4·SDK 실패/누락0, 인용32/본문32의 보관 소스·좌표·integrity와 공개 Go Verify 4행을 확인했다. 계측 on/off의 전체 SDK 응답은 동일하고 FIX-20에서 보관한 응답과 기본 근거·좌표도 같다. 이 반복은 독립 질문 표본이나 공식 지연 시험이 아니다. 새 실행 바이너리 SHA는 `4f4843f9a744b0be8be51aade1f270a19721e9bc374cb031eed3bc0493dc4f77`이다.

계측 on의 neighbors 20개(각 요청10)는 봉인된 vector/graph 인용 후보의 Go JSON SHA·길이와 모두 결합됐다. 측정 ID·ordinal·실제 옵션으로 직접 `ckgclient.Real.Neighbors`를 재생해 결과/개수가 20개 모두 일치했다. 문서 입력12개는 symbol 후보0·qname 미해결·`no node at`, 코드 입력8개는 오류 없이 반환했다. 문서에 있는 Git hunk/file/path 구조 노드를 코드 관계로 승격하지 않았다. 이 원인 확인은 **이번 새 입력 20개**에 한정하며 과거360오류나 FIX-20의 미결합12개를 소급 판정하지 않는다.

원본1,940파일·보관 source1,960파일·조회 사본1,938파일의 내용 봉인은 직접 Go 재생 후에도 같다. 조회 사본의 SHM/검증된 빈 WAL만 별도 처리하며 원본 봉인 전체 파일은 그대로 확인한다. 실제 승인 입력5개도 HEAD와 동일하다. 바이너리/요청/설정/코드와 모델 tags/version 전후를 연결했다. 임시 진단 Go 시험은 `.go.txt`로 보관 후 코드 트리에서 제거했다. DB/모델/바이너리/개인 키는 증거 디렉터리에 넣지 않았다.

현재 단계는 B0-06 계측 보완 검증 완료·공식 입력 연결 대기다. 다음 최우선은 B0-01/02/03/05·STV2 실제 결정 → source/pack/semantic/scope/K 동결 → B0-07 새 strict 빌드다. FIX-19 preview는 FIX-20/21 최종 제품 검증 패키지가 아니므로 최종 수정 동결 후 다시 빌드한다. 완료2/30·진행11·대기17·미완료28개·게이트0/4를 유지한다. 남은 전체 작업은 B0-01–09, B1-03–08, C0-01–06, C1-01–07, STV2-01·OP-01–08·native amd64 및 최종 고지/복구/지원/출시이며 [현황판](./EXECUTION-STATUS.md)을 따른다.
