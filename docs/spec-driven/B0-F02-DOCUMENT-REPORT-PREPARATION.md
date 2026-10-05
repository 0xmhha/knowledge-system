# F-02 장문 꼬리·부모 재조립 보고 준비

2026-10-05. B0-06/B1-04의 **개발 진단 준비 완료**, 공식 F-02 및 B0/B1 평가는 미완료다. [작업리스트](./EXECUTION-WORKLIST.md), [현황판](./EXECUTION-STATUS.md), [원장](../../system/eval/b0-knowledge-system/f02-document-preparation-m2max-2026-10-05/summary.json)을 함께 따른다.

`scripts/b0-audit-f02-document.py`는 고정 F-02-DEV 정의와 published SQLite metadata를 읽기 전용으로 대조한다. DB/manifest seal·미봉인 WAL 거부·고정 source bytes/SHA·같은 commit·6144 byte cap·연속 ordinal/줄·정규 child/parent ID·heading path를 검사한다. 모든 child를 순서대로 연결해 203줄 원문 전체와 같은지 확인한다. 최종 문장을 포함한 마지막 child를 별도로 식별한다. 현재 짧은 원문 줄의 단일 heading 개발 fixture 계약이며, oversized 단일 줄·다중 heading/최종 변형 검증을 대신하지 않는다.

보관 raw CKV 결과를 기존 source-bound reader로 재생하고 full exact vector inventory와 저장 child 집합·parent/ordinal/heading metadata를 연결한다. 공개 CKV query의 parent citation과 density view를 원문으로 검증한다. `full`, `signature+N`, `signature_only`에 따른 실제 문장 노출을 인용 유효성과 구분한다. CKS SDK는 요청/설정/바이너리 전후 SHA·고정 DEV 질의·phase/iteration·measurement ID·planned/failed/missing 분모·v2 source/body/integrity를 대조한다. 별도 공개 Go EvidencePackV2.Verify도 실제 보관 응답을 검사했다. CKS v2에 parent-ID 필드가 있다고 주장하지 않는다.

새 최신 바이너리와 승인 BGE-M3 digest/1024차원/Ollama 0.35.1로 새 임시 strict 개발 데이터셋을 만들었다. 소스 commit은 `08c39f18e8a46a982f2461ecd217847e9c56ab70`이다. `CKV_REQUIRE_COMPLETE_EMBEDDINGS=1`의 실제 build는 source 1파일·doc 5청크·truncated 0이다. 자식 bytes는 19/6060/6060/6060/2091이며 합계 20,290이다. 문서 전용 fixture의 canonical coverage gate는 disabled이며 코드 조인 밀도 합격으로 세지 않는다.

영어·한국어 raw 질의 각각 K10에서 마지막 203줄의 child가 1위다. K10이 저장된 다섯 child를 모두 반환하는 소형 사례이므로 일반 검색 품질·독립 표본 2개로 해석하지 않는다. 두 언어 각각 full budget10000·default4000·minimum20의 CKV 응답을 보관했다. default는 estimated3709 tokens이고 꼬리 본문을 유지한다. minimum은 모든 후보의 signature를 유지하여 estimated105 tokens이며 꼬리 문장은 축약되지만 child/parent 인용은 남는다. 더 줄일 수 없는 signature floor를 metadata에 드러낸 관측이며 엄격한 hard token bound를 보증하지 않는다.

CKS v2 두 응답의 인용/본문은 각각 8개이고 꼬리 source body는 각각 1개다. 원문 SHA·좌표·공개 Go Verify가 통과했다. 요청별 raw/text 검색 K10 두 회와 별도 invariant/convention pass K6를 footprint measurement ID로 연결했다. 두 SDK 성공 안의 best-effort backend nonreturned 14회도 보존했다. warmup/latency/cold는 실행하지 않았다. DB·manifest·graph·binary/config/request SHA 및 selected model metadata가 같다.

helper의 budget1 요청은 공개 minimum20 계약으로 거부됐다. 이를 보존하고 지원되는 minimum20을 추가 호출했다. generated YAML이 optional recall_k를 생략한다는 가정 오류와 모든 CKV pass를 K10이라고 검사한 helper 오류도 보존했다. root retrieval K10을 명시하고 별도 knowledge K6를 유지했으며 제품 코드를 바꾸지 않았다. Python F-02 11개·raw 15개 실패 오라클과 관련 Go 장문/부모 회귀를 통과했다. dropped/gapped/duplicate child·위조 text/hash/parent·stale/foreign SDK·잘못된 request/MID·실패/누락 분모·density 축약을 검사한다.

재생은 보관 source/manifest와 SDK 원문을 사용하며, DB를 다시 읽는 전체 감사에는 임시 published dataset이 필요하다. 새 출력만 0600으로 만들고 기존 보고서는 거부한다.

```sh
python3 scripts/b0-audit-f02-document.py \
  --vector-dir /private/tmp/ks-f02-preparation-20261005/dataset/f02-dev/vector \
  --source-root /private/tmp/ks-b0-dynamic-dev-20261003/F-02-DEV/repo \
  --probe /private/tmp/ks-f02-preparation-20261005/raw-0.json \
  --probe /private/tmp/ks-f02-preparation-20261005/raw-1.json \
  --out /private/tmp/f02-document-audit-new.json
python3 -m unittest discover -s scripts -p test_b0_f02_document.py
```

사람 입력 5개와 개발 fixture 정의는 변경하지 않았으며 FINAL 질의·공식 corpus build/quality·사람 답변 판정은 실행하지 않았다. 다음 독립 작업은 F-04/F-05 과거/현재 snapshot 및 교차 프로젝트 오라클 보고 연결이다. 실제 B0-01/02/03/05·STV2-01 결정 후 입력 동결·새 strict 공식 B0-07 빌드를 우선한다.
