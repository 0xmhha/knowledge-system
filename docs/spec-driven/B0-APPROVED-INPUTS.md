# B0 승인 입력과 새 기준선

2026-10-06 · 입력 동결 검증 완료 · 정적 strict 감사·DEV 의미 투영 적용 완료 / 공식 DEV 평가 진행. 공식 품질/출시 판정은 아직 없다.

사용자 원문: “① Go 소스 29개 포함. ② 프로토콜 승인. ③ F-01–06 승인. ④ SF-03/05-A/05-B/06을 테스트 사례 범위에서 승인. ⑤ STV2 설정 승인.” 로컬 검토자 ID는 chat-user다. 인증된 실명·운영 검토 권한이라는 주장은 하지 않는다.

[전후 SHA 원장](../../system/eval/b0-knowledge-system/approved-inputs-m2max-2026-10-06/approval-ledger.json)과 [누적 사람 결정](../../system/eval/b0-knowledge-system/human-review-m2max-2026-10-03.json)에 원문·timezone 포함 시각·입력 bytes를 연결했다. 이전 “검토 후 결정”은 decision_history에 보존한다. 정적 gold 12개 bytes와 BGE-M3 선택은 같다.

| 결정 | 동결 및 확인 | 아직 남은 실행 |
|---|---|---|
| 소스29 포함 | 고정 commit/tree의 정확한29 파일·SHA; 실제 discovery 1,569→1,598 차이 일치 | 정적13,820 청크·벡터·CKG Go992 포함 감사 통과; 나머지 동적 데이터셋 남음 |
| 프로토콜 | 개발4/최종8·검색5·warm20/2 warmup·cold3·8arms·기존 임계치·작은 FINAL 질문군 inconclusive | 실제 runtime/환경·질의/지연/분모/반복 원장 |
| F-01–06 | DEV/FINAL12 변형 reviewer/time/source SHA·분할·오라클 선언 승인 | 독립 입력 빌드·exact/parent/freshness/isolation/trace 오라클 실행 |
| SF 네 사례 | 기존 검토 자료 SHA와 승인 ID·테스트 범위 연결; SF-03 공통 개념은 proposed 유지 | DEV4 새 커밋·정책 리뷰3·잠금·canonical/원문 검증 완료; 최종 변형·평가 남음 |
| STV2 | 날짜2026-10-01·subsystem knowledge-system·정수K10·정적12 ID4/8, 현재 프로토콜/승인 SHA 대조 | 실제 runtime YAML·원요청·계측 K/config 결합 |

[프로토콜 입력 checker](../../system/eval/b0-knowledge-system/approved-inputs-m2max-2026-10-06/protocol-readiness.json)는 approved_input_definition, [STV2 checker](../../system/eval/b0-knowledge-system/approved-inputs-m2max-2026-10-06/static-v2-readiness.json)는 approved_scope_definition이다. 두 도구의 official_execution_ready=false는 입력 정의 검증 범위의 한계이며 현재 사람 승인 대기라는 뜻이 아니다.

## FIX-22: 정확한 소스 예외

기본 build/ 제외가 실제 internal/vector/build/ Go 소스29를 누락했다. 새 CKV build의 `--build-sources`는 schema_version=1과 paths 배열 JSON을 받는다. 정확한 상대 .go 파일 경로만 build/ 기본 제외를 통과시키며 glob·상위/절대/비정규 경로·중복 파일·unknown 필드·trailing JSON을 거부한다. CKV 외부 allowlist와 build_roots는 계속 별도 필터로 적용한다.

CKS setup의 `--vector-build-sources`와 설정 vector_build_sources가 이를 전달한다. 입력 JSON bytes는 configured input digest와 dataset recipe에 포함하고 CKV manifest의 build_sources에 경로를 남긴다. source inventory는 파일 bytes를 별도로 결합한다. 정확한 목록을 바꾸면 dataset 신원이 달라진다.

기존 명시적 .ckvignore/extra 규칙·다른 기본 제외·비밀·링크·oversize·binary 필터가 우선한다. 기본 옵션 동작은 유지한다. 수정 전 실제 시험은 engine.go 누락으로 exit1, 수정 후 관련 Go6 패키지·수집/설정 race·B0 Python72 시험 통과. 승인 이후에도 pending 오라클을 시험하기 위해 Python 한 곳을 명시적 임시 draft 입력으로 바꿨다. 최초 비에스컬레이션 실행의 localhost bind 제한과 실패 로그도 보존한다.

[실제 수집 목록](../../system/eval/b0-knowledge-system/approved-inputs-m2max-2026-10-06/source-inventory.json)은 기본1,569/승인1,598 파일과 전체 bytes SHA를 기록한다. discovery의 Go1,055는 플랫폼/build tag/별도 module 여부를 적용한 CKG Go992와 분모가 다르다. 실제 청크와 graph 대상의 포함 여부는 신규 빌드 후 감사한다. [독립 코퍼스](../../system/eval/b0-knowledge-system/approved-inputs-m2max-2026-10-06/isolation.json)의 commit/tree는 원래 고정값이며 unreachable commit은0이다.

새 strict 빌드는 별도 임시 루트에서 진행한다. 기존 진단 DB와 source는 보존하며 build/query 지연을 구분한다. SF 승인은 운영 정책·코드 수용·무관 TestHealth의 환급 검증·출시/법무/키 승인이 아니다. 조회 날짜도 정책 효력 승인 날짜가 아니다. FINAL 결과를 개발 튜닝에 사용하지 않는다.

현재 단계·남은 전체 작업·다음 행동은 [작업 현황](./EXECUTION-STATUS.md)과 [작업리스트36절](./EXECUTION-WORKLIST.md)을 따른다.


## 정적 감사와 DEV 적용 결과

새 strict 빌드는1,005.26초/exit0이다. 임베딩 구간 중 mock SF 준비 작업이 있었으므로 통제된 독점 build 비용으로 해석하지 않는다. 모든1598 선택 파일 원문 SHA·13590 실제 parser body/source span·65 parent 재조립·13820 청크/벡터·orphan0·doctor ready·입력/모델 불변을 확인했다. AST 파생 invariant77/convention153은 원문 body와 다른 생성 문장으로 별도 계산한다. alignable symbol8273 중 canonical7947이며 미연결326을 숨기지 않는다.

SF DEV4는 검토 당시 source SHA·supplement commit과 동일한 새 후보를 확인한 뒤 실제 retained source 리뷰3개를 기록했다. 새 커밋/잠금·BGE-M3 dataset/투영이 별도 신원이며 SF-03의20개 개념은 proposed, 나머지 승인된 개념/요구/IMPLEMENTED_BY 관계만 reviewed_by=chat-user를 가진다. 무관 테스트의 수용 관계는0이다. [원자료](../../system/eval/b0-knowledge-system/approved-semantic-m2max-2026-10-06/approved-built-manifest.json)를 따른다.

첫 두 공식 DEV 시도에는 초기 stderr pipe backpressure와 작업자가 잘못 전달한 HTTP 설정이 함께 있었다. 두 원인을 구분하고 실패 원자료를 보존했다. FIX-23의 실제2MiB 회귀 실패·수정 후 정상/실패 초기화·race/vet와 새 runtime SHA를 [입력 증거 폴더](../../system/eval/b0-knowledge-system/approved-inputs-m2max-2026-10-06/runtime-after-stderr-binding.json)에 기록했다. 기존 builder/DB/실패 시도를 보존하고 새 경로로 공식 DEV 평가를 재개했다. 최종/동적 평가·전체 B0/B1 결과와 사람 판정은 아직 남아 있다.

정적 v1 DEV20회 실행은 완료했지만 CODE/POLICY/TRACE miss·WHY pass다. 8-arm 첫 사전 검사는 빈 provider를 서버 기본 Ollama와 다르게 비교하여 rows0/partial이었다. FIX-24는 실제 수정 전 실패 후 서버와 같은 기본 제공자를 적용하고 관련 회귀/race/vet를 통과했다. [최종 runtime 결합](../../system/eval/b0-knowledge-system/approved-inputs-m2max-2026-10-06/runtime-final-binding.json)은 이전 v1 바이너리와 새 matrix 바이너리를 구분한다. 새 8-arm 환경 preflight는 valid=true이며 지연/품질 원자료를 수집 중이다.
