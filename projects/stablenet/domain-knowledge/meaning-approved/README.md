# Stablenet meaning — 사용자 승인본

2026-10-13. 현재 대화의 **“모두 추천 제안으로 승인”**을 근거로 C01–C04/R01–R04와 기준5개, R04의 read-under-write-lock 해석을 채택했다. [승인 기록](./approval.json), [ontology](./ontology.yaml), [spec](./spec.yaml). 검토자 개인 이름은 지정되지 않아 `current conversation user approval`을 기록했다. 과거 entry의 mhha 이름을 새 승인자로 옮기지 않았다.

[원래 proposed 검토 자료](../meaning-draft/README.md)는 이전 상태·출처/한계의 기록으로 보존한다. 승인본은 실제 운영 자료의 stable project ID에 맞춰 envelope를 정렬하고 status/reviewed_by만 바꿨다. 정의·alias·scope·Given/When/Then 내용은 승인 당시 초안과 같다.

verified는 이 정의와 요구를 사람이 채택했다는 뜻이다. 코드 관계 후보16개, 실제 code/test 연결·criterion 실행·검색 품질·출시·N16 조건은 승인된 것으로 표시하지 않았다. 그러므로 구현 관계가 없는 요구 trace의 정상 상태는 `missing_code`다.

실제 데이터 연결은 원본 source/current/config를 보존하는 별도 Git 복사본과 held 후보에서 진행한다. 복사본의 parent는 원래 code commit0bf2f4d1이며 승인 문서2개만 추가한다. WBFT core/validator/messages의 명시된 의존 범위501파일을 선택하고 실제 BGE-M3를 사용한다. 전체 프로젝트 배포/독립 FINAL 평가로 확대하지 않는다. 캡처·인덱스·semantic tuple·MCP 검증이 끝나기 전에는 운영 연결 완료로 체크하지 않는다.

다음은 held source/index/semantic와 MCP status/조회 검증, 이후 실제 모델의 동일 조건 off/on 관측과 품질 한계 판정이다. 전체 잔여 N16/O03/O04의3개·기존17/18·온톨로지2/4는 데이터 연결 검증 전 상태다.

실제 held WBFT source/index/semantic·MCP 연결 검증이 완료됐다. source baca2869/Go494의 explicit scope이며 full audit762는 scope 밖 DRIFT로 남겼다. 승인 concept4/requirement4·missing_code4·변조/옛 dataset 거부·운영 보존 검증. 구현 관계/criterion 실행/품질 완료로 확대하지 않는다. 다음 O04 독립 평가 준비, 전체 잔여 N16/O04의2개다. [연결과 관측 기록](../../../../docs/spec-driven/ONTOLOGY-MCP-IMPLEMENTATION.md).
