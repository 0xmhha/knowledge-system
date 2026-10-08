# 문서 인용의 코드 그래프 확장 재생

2026-10-05. [직접 Go 결과](../../system/eval/b0-knowledge-system/neighbor-replay-m2max-2026-10-05/direct-go-replay.json), [요약](../../system/eval/b0-knowledge-system/neighbor-replay-m2max-2026-10-05/summary.json), [해시](../../system/eval/b0-knowledge-system/neighbor-replay-m2max-2026-10-05/evidence-manifest.json)을 기록했다. [FIX-20](./B0-FTS-KEYWORD-DIAGNOSTIC.md) SDK의 선택된 기본 인용8개를 실제 `ckgclient.Real.Neighbors`에 직접 재생했다. 모델/SDK/FINAL을 추가로 실행하지 않았다.

문서6개는 자기 파일의 후보가 없거나 Git history Hunk 등 structural pseudo node뿐이었다. 실제 `matchQname`은 `file:`/`hunk:`/`import:` 및 path-shaped 노드를 코드 symbol로 취급하지 않으며 여섯 사례 모두 qname이 비어 `no node at`을 반환했다. 코드 제어2개는 오류 없이 조회됐다. 문서 검색 본문·인용이 유효해도 그 문서에 코드 symbol의 호출/정의 관계가 있는 것은 아니다. 임의의 Git hunk/file node로 대체하거나 구현 관계를 꾸미지 않았다.

계획8/관측8·시험 통과, adapter/helper/입력 source capture SHA를 결합했다. 원본·사본 source/DB/manifest와 사람 입력5개도 그대로다. 실제 코드 영역에 잠시 만든 진단 테스트는 원문 `.go.txt`로 보관한 뒤 제거했고 제품 코드는 변경하지 않았다. 일반 CI 시험을 추가하거나 수정했다고 주장하지 않는다.

이 직접 재생은 선택된 문서6/코드2개만 확인한다. 기존 backend neighbors 계측은 source citation 인수를 기록하지 않으므로 직접8호출을 과거360오류나 FIX-20 SDK12오류의 모든 seed에 일대일 결합할 수 없다. SDK의 `failed_seeds=6`과 정합적이지만 전체 원인이 확정됐다는 판정은 하지 않는다. 실제 오류·미결합 범위를 보존하며 source 인수를 결합한 공식 평가와 혼입 오라클은 남는다. quality_metrics/공식 verdict는 null이다.

후속 [FIX-21 계측 보완](./B0-NEIGHBOR-BINDING-DIAGNOSTIC.md)은 새 실행의 실제 seed20개를 SHA/바이트·옵션·결과로 직접 결합했다. 위 과거8개와 미결합 오류의 제한은 그대로 유지한다.

다음 최우선은 B0-01/02/03/05·STV2의 실제 결정→source/팩/semantic/scope/K 동결→B0-07 새 strict 빌드다. 완료2/30·진행11·대기17·미완료28개·게이트0/4를 유지한다. 남은 전체 작업은 B0-01–09, B1-03–08, C0-01–06, C1-01–07과 STV2-01·OP-01–08·native amd64·최종 고지/복구/지원/출시이며 [현황판](./EXECUTION-STATUS.md)을 따른다.
