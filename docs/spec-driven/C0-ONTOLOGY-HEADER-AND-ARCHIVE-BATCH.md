# 승인 개발 평가의 순위 회귀와 보관 원문 검증 비용

2026-10-06 · FIX-26/27 · 개발 입력만 사용. 최종 질문은 수정에 사용하지 않았다.

승인된 F-05-A/B 및 F-06의 8-arm 원응답에서 baseline MRR 0.75가 세 온톨로지 모드 모두 0.666667로 낮아졌다. 같은 canonical 함수의 `symbol`과 이를 포함하는 `file_header`가 각각 가산되어 정책 문서가 2위에서 3위로 밀렸다. Recall은 유지됐지만 MRR 차이 −0.083333은 승인 회귀 기준 −0.02보다 나쁘다. 소표본의 inconclusive 규칙으로 이 관측 회귀를 감추지 않는다.

FIX-26은 원래 상위 후보 안에 같은 canonical ID·커밋·파일의 겹치는 `symbol`/`function_split`이 있을 때 헤더의 **온톨로지 추가 점수만** 제외한다. 기본 후보·기본 점수·본문은 제거하지 않는다. 헤더만 검색된 경우, 본문이 cap 밖에 있는 경우, 다른 함수/파일/커밋/겹치지 않는 줄은 기존 동작을 유지한다. 분할 함수 조각은 서로 억제하지 않는다. 관계·concept_text·combined에 동일 조건을 적용하며 text-only 경로가 구현 관계를 조회하지 않도록 유지한다. 질문·정답·픽스처·K·임계치·모델·인덱스 빌더를 바꾸지 않는다.

수정 전 세 모드 모두 정책 순위가 밀리는 시험을 재현했다. 수정 후 관련 4개 패키지 시험·race·vet가 통과했다. 실제 F-05/06의 전체 승인 반복 재평가로 해소 여부를 확인해야 한다. 테스트 통과를 공식 품질 합격으로 세지 않는다.

정적 v2 경로에서는 본문마다 전체 dataset/source 검증을 반복했다. 하나의 span 읽기가 1,598파일뿐 아니라 Git 복구 자료까지 다시 확인하므로 8–11본문 응답에서 반복 비용이 컸다. FIX-27은 요청-local span batch의 시작·종료에 전체 검증을 수행하고, 각각의 선택 blob을 다시 읽어 해시·크기·UTF-8·원문 줄·origin·tuple을 확인한다. 요청 간 캐시는 두지 않는다. 인용되지 않은 보관 파일 손상도 다음 batch에서 거부하는 시험을 포함한다. sanitization·32,000바이트/12인용 상한은 유지한다.

실제 SDK 개발 4응답은 수정 전후 pack 바이트 구조가 동일했고 오류가 없었다. 단일 진단 호출은 CODE 10.24→2.99초, WHY 9.99→4.55초, POLICY 7.66→2.09초, TRACE 10.36→3.90초였다. 이 값은 서로 다른 시점의 단일 진단이며 공식 warm p95 개선 판정이 아니다. 수정 전 정적 matrix는 종료 응답을 포함한 **112/960행**을 partial로 보존했다. 당시 동적 2,880행은 완전 캡처지만 새 batch/runtime의 지연 근거로 재사용하지 않는다.

원자료는 [수정 전 개발 캡처 manifest](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/development-before-fix26-27/artifact-manifest.json), [수정/바이너리 결합](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/fix26-27/runtime-binding.json), [실제 SDK 전후 비교](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/fix26-27/v2-batch-smoke-comparison.json)에 있다. 큰 원자료는 lossless gzip이며 manifest가 압축 전후 SHA를 각각 기록한다. 입력 승인·사람의 답변/정책 판정·운영 출시 승인은 별개다.
