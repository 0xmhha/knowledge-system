# FIX-20: 검색 후보를 FTS literal로 전달

2026-10-05. [대규모 DEV 진단](./C1-LARGE-COST-DIAGNOSTIC.md)에서 문서 제목 후보8개의 SHA가 BM25 backend_error와 일치했다. `1-r3-...`, `2026-09-30-...` 같은 하이픈을 bare FTS로 전달하면 SQLite가 일부를 column 문법으로 해석하여 `no such column`을 반환했다. 단일 후보를 literal로 조회하면 오류 없이0개가 반환됐다. source가 graph 검색에 매칭되지 않는 경우와 query 문법 오류는 서로 다른 결과다.

## 수정과 회귀

`ckgclient.KeywordFTSQuery`를 Stage1 rerank 및 Stage2 기본/테스트 경로의 BM25 호출에 적용했다. 일반 ASCII 식별자의 기존 query는 유지하고, punctuation·한글·FTS 연산자는 double quote를 escape한 literal로 전달한다. 원래 후보 문자열은 rerank 결과·FindSymbol·투영에 유지한다. 직접 BM25Search의 명시적 FTS 표현식 및 공개 search_text의 OR 계약은 바꾸지 않았다.

수정 전 두 회귀가 exit1로 실패했다. 문서 후보는 rerank에서 사라졌고, `api.(*Server).Handle`은 기본/테스트 보조 검색에 문법으로 전달됐다. 수정 후 실제 SQLite FTS5에서 하이픈/qualified symbol/quote/연산자/한글이 query 오류를 만들지 않고, `Alpha OR Beta` 및 quote 삽입이 별도 Alpha/Beta row로 검색을 넓히지 않는다. 관련 ckgclient·Stage1/2·composer·MCP·eval·evalcli 시험, ckgclient/Stage1/2 race 및 관련 vet를 통과했다. 신규 helper의 실패 결과와 수정 후 원문은 [증거 원장](../../system/eval/b0-knowledge-system/fts-keyword-fix-m2max-2026-10-05/evidence-manifest.json)에 보존했다.

## 실제 모델에서 확인한 범위

수정 바이너리 SHA `8e1aef454f053229dc5fa3a7a3bbd00f1e7c062274562c198ac98d6ff0ec17e5`와 같은 부분13,575청크·승인 BGE-M3·DEV 문항/scope/K를 사용했다. baseline/knowledge 각1, 계획2 SDK/16인용/16본문이 독립 Python 원문 SHA/줄/좌표/무결성 및 공개 Go Verify를 통과했다. 수정 전 비용 진단과 인용·본문·좌표 집합은 같다. 실제 BM25 query SHA에 literal 후보8개가 결합됐고 호출당 BM25 오류는8→0이다. 일반 K10/별도K6·HTTP200·원본/사본 DB/source·binary/config/request·모델 metadata·사람 입력5개 불변을 확인했다.

neighbors backend_error12(호출당6)는 그대로 보존했다. 아직 전체 원인이나 공식 검색 품질 실패로 판정하지 않았다. 새2호출은 지연 개선/독립2표본·최종 품질로 합산하지 않는다. EOF 등 수정 전 빌드의 전체 범위 제한도 유지한다. 제품 binary와 테스트/회귀/SDK 원자료31개를 결합했고, DB/바이너리/모델/private key는 Git에 넣지 않았다.

이 수정은 관측된 구조 오류의 FIX-20이며 공식 B1 이후 C0-01–06 종료 증거가 아니다. 기존 FIX-19 패키지·복구·대규모 비용은 그 source 범위에서 유효하지만 FIX-20 최종 패키지 검증은 아니다. 최종 품질/수정 동결 후 새 후보를 패키징해야 한다.

현재 완료2/30·진행11·대기17·미완료28개·공식게이트0/4다. 다음 독립 작업은 남은 neighbors의 실제 seed/노드 해석을 확인하는 것이다. 공식 최우선은 B0-01/02/03/05·STV2 결정→입력 동결→새 strict B0-07이다. 남은 전체는 B0-01–09·B1-03–08·C0-01–06·C1-01–07, STV2-01·OP-01–08·native amd64·최종 고지/복구/지원/출시이며 [현황판](./EXECUTION-STATUS.md)을 따른다.
