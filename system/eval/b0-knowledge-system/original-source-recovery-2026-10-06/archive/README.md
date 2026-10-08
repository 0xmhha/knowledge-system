# Knowledge System 초기 분석·설계 자료

이 디렉터리는 2026-09-27–29에 작성한 PDF 기반 분석, 초기 요구사항·아키텍처·WBS와 재현 도구를 보관한다. 분석 대상은 `knowledge-system`의 고정 커밋 `1ded9b3e47bc2e09062dba329c2f918423746fa4`다. 문서에 적힌 결함·제안·완료 상태는 그 당시의 관측이며 현재 구현 상태를 뜻하지 않는다.

## 문서

| 문서 | 내용 |
|---|---|
| [analysis-review.md](./analysis-review.md) | PDF와 AST·문서 그래프를 바탕으로 한 초기 코드 구조·동작·장단점 분석 |
| [ckv-ckg-ontology-installation-proposal.md](./ckv-ckg-ontology-installation-proposal.md) | PDF 페이지별 짧은 인용, CKV/CKG 개선안, 온톨로지·통합 개발·설치형 CKS 제안 |
| [spec-driven-requirements.md](./spec-driven-requirements.md) | 초기 FR/INV/NFR 요구사항과 수용 시나리오 |
| [spec-driven-architecture.md](./spec-driven-architecture.md) | 초기 기술 설계와 검증 경계 |
| [spec-driven-wbs.md](./spec-driven-wbs.md) | 초기 WBS v0.1; 후속 WBS v2로 재기준선화된 이력 자료 |
| [analysis-metrics.json](./analysis-metrics.json), [ckg-validation.txt](./ckg-validation.txt) | 당시 실제 AST 그래프 측정·검증 결과; 실모델 검색 품질 지표가 아님 |

## 현재 실행 기준

후속 설계와 개발 기록은 별도 저장소 [0xmhha/knowledge-system](https://github.com/0xmhha/knowledge-system)의 `feat/spec-driven-knowledge-system` 브랜치에 있다. 해당 브랜치의 최신 커밋과 게이트 보고서를 확인한다.

- [전체 WBS v2](https://github.com/0xmhha/knowledge-system/blob/feat/spec-driven-knowledge-system/docs/spec-driven/DELIVERY-PLAN-V2.md)
- [전체 단계 상세 설계](https://github.com/0xmhha/knowledge-system/blob/feat/spec-driven-knowledge-system/docs/spec-driven/END-TO-END-DESIGN.md)
- [PDF 개선안 → 요구사항 → WBS → 검증 추적표](https://github.com/0xmhha/knowledge-system/blob/feat/spec-driven-knowledge-system/docs/spec-driven/PDF-IMPROVEMENT-TRACE.md)
- [구현·검증 실행 기록](https://github.com/0xmhha/knowledge-system/blob/feat/spec-driven-knowledge-system/docs/spec-driven/EXECUTION.md)

## 원천과 재현 도구

원본 『데이터베이스 설계와 구축』 PDF와 전체 OCR `database-design-and-build-ocr.md`는 로컬 제공 자료다. 저장소에는 페이지별 짧은 인용과 분석·설계 문서를 보관하며, 전체 책 전사본은 포함하지 않는다. 정확한 문구 확인은 같은 원본 PDF를 별도로 제공받아 수행한다.

생성된 `ckg-index/`와 `knowledge-system-review-graph.sqlite`도 로컬 산출물로 유지한다. [build_go_ast_graph.go](./build_go_ast_graph.go)는 Go 구문 레코드를 만들고, [build_review_graph.py](./build_review_graph.py)는 그 레코드와 Markdown·YAML을 검토용 SQLite 그래프로 결합한다. 전자는 Go 도구 체인, 후자는 Python과 PyYAML이 필요하며 제품 CKG의 타입 분석·스키마를 대신하지 않는다. 입력 저장소는 위 고정 커밋을 사용한다.

```sh
go run build_go_ast_graph.go /path/to/pinned-knowledge-system > /tmp/knowledge-system-go-ast.jsonl
python3 build_review_graph.py /path/to/pinned-knowledge-system \
  /tmp/knowledge-system-go-ast.jsonl /tmp/knowledge-system-review-graph.sqlite
```

[ocr_pdf.swift](./ocr_pdf.swift)는 macOS PDFKit·Vision으로 PDF 페이지를 OCR하고 페이지별 JSON을 출력한다. 자동 OCR 결과의 표·수식·코드는 원본 대조가 필요하다.
