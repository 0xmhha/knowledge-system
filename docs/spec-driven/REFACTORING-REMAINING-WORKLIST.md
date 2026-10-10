# 원래 리팩토링 목적 기준의 남은 작업리스트

2026-10-10 최신(Claude Code): **N15·N17 완료(로컬 개인 운영 범위), N16 미완료**. [수용 기록](./N15-N17-LOCAL-OPERATION-ACCEPTANCE.md) — go-stablenet 실제 색인(29.5분·embedding 20,958·1.2GB), submodule 캡처 결함 수정, MCP 질의 5건 11–18초, 백업·손실·복구 첫 질의 35초를 사용자가 수용했다. N16은 서명 키 미사용으로 공개키 fingerprint 조건이 남는다. **현재 12/18 완료, 남은 6개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N16 신뢰·권한, N18 출시 판정. 다음 N11 DEV3·FINAL(한가한 시간).

2026-10-10 최신(Claude Code): **N14 완료**. [수용 기록](./N14-CHANGE-ACCEPTANCE.md) — 실제 변경 7c57a388을 hold 후보·불변 patch attempt·결과 snapshot linked run(9/10, F3-T10 거부)·잘못된 run 4종 거부로 검증하고, 기준 4개를 사용자 판정(표기 opus, agent 초안 이유 채택)으로 승인해 소유 dataset에서 승격했다. 반려 대조는 승격 거부, rollback은 해제된 두 버전 사이에서 확인, 보관본 단독 재생 통과. **현재 10/18 완료, 남은 8개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 출시 판정. 다음 N11 DEV3·FINAL(한가한 시간).

2026-10-09 최신(Claude Code): **N11 입력 동결**. [동결](./N11-INPUT-FREEZE.md) — 철회5 제거·대체 DEV2 추가, DEV7/FINAL10을 “opus5.5 검토”(사용자 지정 AI 라벨)로 승인·동결, 입력 검사 input_ready=true·sampling inconclusive. 기존 N11 corpus는 제외 경로를 포함해 새 corpus·실행 바인딩 필요, FINAL 실행은 비가역이라 사용자 결정 대기. 9/18 완료·남은 9개.

2026-10-09 최신(Claude Code): **N12 완료**. [결합·활성화·linked run](./N12-REVIEWED-ACTIVATION.md) — 사용자가 사전검토 65건을 채택하고 검토자를 “opus5.5 검토”로 지정(AI 검토 라벨, 독립 사람 내용 검토 아님). 검토된 ontology v2/spec v3를 선택해 새 capture `0b9085e7`·snapshot `813b80a9`·dataset `daf9608b`에 결합, 관계43·개념23·요구3·주장6 verified, 세 요구 trace linked, 소유 held store 활성화. 실제 semantic linked run 10건 중 9건 통과·1건(F3-T10) 검토 경로 없음으로 거부, 보관본 archive-only 재생 일치. criterion accepted0은 N14로 넘긴다. **현재 9/18 완료, 요청6/15, 남은 9개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 출시 판정. 다음 N11.

2026-10-09 최신 하위 단계(Claude Code): **N11 후보 책 수정 반영·기권 인용 규칙**. [수정 반영](./N11-CANDIDATE-REVISION.md) — 사용자 채택·DEV 교체·평가 문서 corpus 제외·ABS 인용 허용을 기록(검토자 이름 없음). 개정 책 DEV5/FINAL10, FINAL 군 CODE4·WHY1·POLICY1·TRACE2·ABS1(모두10 미만). 입력 검사가 protocol 규칙과 근거 원문이 있을 때만 인용 허용 기권을 받도록 RED→GREEN, 스크립트 시험26 통과, 개정 초안 검사 exit2·바이트 오류0. 승인0·동결0. 전체8/18·요청5/15·남은10개 유지.

2026-10-09 최신 하위 단계(Claude Code): **N12 core·기능·요구·기준 수정 반영 제안**. [수정 반영](./N12-CORE-REVISION.md) — 사용자 30건 채택(검토자 이름 없음)을 ontology-pilot-v2/functional-v2/spec-pilot-v3 제안 파일로 반영, 원본·PILOT-SCOPE 불변. 생산 parser·core 검증 통과, 한국어 용어 충돌 해소를 matcher로 확인. verified0. 전체8/18·요청5/15·남은10개 유지.

2026-10-09 최신 하위 단계(Claude Code): **N11 후보20문항 기술 사전검토**. [사전검토](./N11-CANDIDATE-PREREVIEW.md) — accept7·조건부1·modify5·증거보강4·reject3, span SHA 전부 일치. 후보 정답이 N11/N12 문서와 질문 책에 그대로 있어 corpus 제외 규칙 필요(치명), DEV-06/FINAL-03·DEV-07/FINAL-14 사실 공유, WHY3 reject, ABS 인용0 규칙 충돌. 채택해도 모든 군10개 미만. 사람 판정 아님. 전체8/18·요청5/15·남은10개 유지. 다음 검토자 이름과 N11/N12 판정. 남은 전체 N08/09/10/11/12/14/15/16/17/18.

2026-10-09 최신 하위 단계(Claude Code): **N12 core20·기능3·요구3·기준4 기술 사전검토**. [사전검토](./N12-CORE-PREREVIEW.md) — 개념 accept13·modify10, 요구·기준 accept2·modify5. criterion-decision의 needs-review는 제품 미지원, ac-filtered-completeness는 취소를 cancelled로 반환, ontology 기준은 집합 보존 명시 필요, 한국어 용어 부분 문자열 충돌2쌍을 코드에서 재확인. 사람 판정 아님·원본 YAML 불변. 전체8/18·요청5/15·남은10개 유지. 다음 검토자 이름과 30건 판정, N11 후보 사전검토. 남은 전체 N08/09/10/11/12/14/15/16/17/18.

2026-10-09 최신 하위 단계(Claude Code): **N12 사전검토 수정10건 반영 제안**. [개정 제안](./N12-REVISED-PROPOSAL.md) — 사용자 “남은 작업별 수정을 진행해줘”를 35건 권고 채택으로 해석해 기록(확신도 Mid, 검토자 이름 없음). F3-T08/T11 대상을 code anchor gate로 바꾸고 새 관계14개(F3-R01–14)를 추가했다. 대상16개를 go/ast로 확인, 관계에 등장하는 시험11개/하위시험31 race pass. 모두 proposed·verified0, held 후보·worksheet 불변. 전체8/18·요청5/15·남은10개 유지. 다음 검토자 이름, core20·요구·기준 판정, 이후 새 capture 결합. 남은 전체 N08/09/10/11/12/14/15/16/17/18.

2026-10-09 최신 하위 단계(Claude Code): **N12 외부 snapshot 거부 gate 타입 구분**. [변경·검증](./N12-GATE-ATTRIBUTION.md) — `ErrEvidenceSnapshotMismatch`/`ErrCodeAnchorSnapshotMismatch` 추가, 기존 오류 문자열 바이트 유지. 선언만 한 상태 behavioral RED→연결 후 GREEN, semantic 의존8패키지 race251시험/264하위시험 pass. 통합 시험이 사례별 거부 gate를 특정한다. 사람 판정0·후보36353d32 불변, 전체8/18·요청5/15·남은10개 유지. 다음 검토자 이름·판정 확인과 독립 준비 작업. 남은 전체 N08/09/10/11/12/14/15/16/17/18.

2026-10-09 최신 하위 단계(Claude Code): **N12 관계 기술 사전검토·기준 누락 시험3개 보강**. [사전검토](./N12-CRITERION-GAP-PREREVIEW.md) — F3-T08/T11 대상 부정확(좌표 함수가 아니라 code anchor gate가 거부), F3-T10은 code anchor Given 없음, T03 순서·T07 filtered 취소·T15/T17 미검증 관계+CKG 인용 누락을 확인했다. 순서 단언·filtered 진행 중 취소·미검증 관계 baseline 시험을 추가해 관련3패키지 race132시험/102하위시험 pass, 변이3중 상태변환 제거 검출. 사람 판정 아님·reviewer null·개별 승인0, 최신 후보36353d32 불변. 전체8/18·요청5/15·남은10개 유지. 다음 실제 검토자·대상별 판정과 N11 부족분. 남은 전체 N08/09/10/11/12/14/15/16/17/18.

2026-10-09 최신 하위 단계: **N12 새 승격 통합 시험의 동일 source/index/관계 결합 및 archive-only 재생 검증 완료**. [개별 검토 자료](./N12-ANCHOR-BOUND-TRACE-REVIEW.md) — source1,438/선택71·CKG63/CKV66/747chunks625canonical, 실제canonical18·evidence81·신규proposed17(총29), race8시험29하위시험/변조5거부, 원본 source/index/store 없이 archive1,457권위파일 재생·bytes 동일. 전체Go DRIFT992 유지. 좌표 함수의 부분 역할과 실제CKG+mockCKV 통합 제어를 구별했다. human/reviewed path/linked run/FINAL/actual activation0, 전체8/18·요청5/15·남은10개 유지. 다음 실제 검토자·개별 내용 판정 및 N11 gold/독립성·부족 표본·runtime 동결. 남은 전체 N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 출시 판정. 아래는 이전 단계 이력이다.

2026-10-08 최신 하위 단계: **N12 actual foreign-code-anchor 승격·active 보존 통합 회귀 보강**. [새 시험·범위](./N12-CODE-ANCHOR-PROMOTION-REGRESSION.md) — 공개CKG2source/2DB·canonical stable/commit 다름,5사례8쓰기+2activation 거부/old current·저장 자료·reopen 보존. semantic race42시험20하위시험/vet/boundary/format 통과. 최초 fixture YAML 작성 오류는 제품RED와 별도 보존했다. 기존 held66artifact·승인input5 불변, 새 시험은 이전55d8/43ba/f839 pilot 후보에 아직 미포함이다. 전체8/18·남은10개 유지;다음 새 source/index/canonical 관계 결합과 N11 실제 gold·독립성/검토자 판정. 남은 N08/09/10/11/12/14/15/16/17/18. 사람 승인·새FINAL·실모델·운영 활성화0.

2026-10-08 최신 하위 단계: **N12 기능 version2의 새 실제 source/CKG·CKV/관계15 준비 검증**. [증거·기준별 누락](./N12-FUNCTIONAL-V2-TRACE-PREPARATION.md) — source1,437/CKG62·CKV65/742chunks·621canonical,15proposed 관계/11canonical/74span/27문서,related race7시험24하위시험·5거부·원 source/dataset/store 없이 archive 재생 통과. 전체Go DRIFT992는 유지했다. 개별 판정0/spec_unapproved3/활성0/새FINAL0, 전체8/18·요청5/15·남은10개 유지. 다음 N12 actual foreign-code-anchor 승격·active 보존 통합 시험 보강과 N11 gold·독립성/실제 검토자 판정. 남은 전체 N08/09/10/11/12/14/15/16/17/18. 이전 후보·승인input5는 보존하며 새 관계를 독립 FINAL로 세지 않는다.

2026-10-08 최신 하위 단계: **N11/N12 권고 범위2개 선택 적용**. [적용·원문 검증](./N11-N12-SCOPE-ADOPTION.md) — POLICY/TRACE 코드 계약과 실제 조직/검토 경로 구분, 새 metadata 초안7행;N12 core20보존+functional3·요구3/v2/criteria4·30source span/30digest 거부·SQLite reopen/bundle 독립 재생 확인. scope2선택은 개별 content 승인과 별도이며 reviewer name미입력/개별gold·관계·criterion0/spec_unapproved3/current0이다. 기존 pool·held 후보·승인input5는 보존했다. 전체8/18·남은10개 유지;다음 개별검토·부족 독립군 보강·새 전체 source/index 결합. 남은 N08/09/10/11/12/14/15/16/17/18. Claude/model/새FINAL0.

2026-10-08 최신 하위 단계: **N09 CKV 평가 보조 함수의 빈 snippet 오판 수정**. [수정·검증](./N09-EMPTY-SNIPPET-VERIFICATION.md) — 수정 전4내용없음 사례 Verified=true/집계 오류 RED, 수정 후 empty_snippet 진단·관련2패키지 race103시험4하위시험 pass/실BGE smoke1skip·vet/boundary/format 통과. 임시Git 서명·Go cache 환경 실패는 별도 보존하고 명령 범위의 fixture 서명 설정만 조정했다. 실제 기권/인용0/새FINAL/사람 판정0, 전체8/18·남은10개 유지. 다음 N11 gold·독립성/범위·충분한 표본 보강과 N12 실제 관계 판정 후 동결/평가. 남은 전체 N08/09/10/11/12/14/15/16/17/18. Claude 실행/외부 전송0, Codex 직접 수행.

2026-10-08 최신 하위 단계: **Codex 직접 N11 추가 원문8범위·응답 두 표면 조사**. [조사와 보류](./N11-SOURCE-PROBE-INVESTIGATION.md) — source8+기존 제외 기준1, 고정 Markdown259개와27gzip/11,632행을 대조했다. response.content23,262문자열을 추가 검사했고 기존CAND05의480노출을 재확인했다. 6범위는 이전 설계·튜닝으로 격리,2개는 의미 중복 검토가 필요한 비실행 초안으로 보류했다. 기존DEV7/FINAL후보13/제안군8/제외6·승인0, 전체8/18·남은10개 유지. 다음 실제 범위/개별 gold·독립성·관계 판정과 더 넓은 미사용 사실군 확보. 남은 전체 N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 출시 판정. Claude 실행/외부 전송0이며 사용자 요청에 따라 직접 실행으로 전환했다.

2026-10-08 최신 하위 단계: **N11 과거 응답 본문 노출 대조·범위 결정 안내**. [추가 감사](./N11-RESPONSE-EXPOSURE-AUDIT.md) — 26source refs/27gzip/11,632행·71,542body·graph neighbor0을 확인하고 이미 제외한 CAND05의104행이480응답에서 원문과 동일 바이트임을 재확인했다. 나머지 문자열 일치0을 독립성 승인으로 쓰지 않는다. [다음 범위 선택](./N11-N12-SCOPE-DECISIONS.md)을 요청했으며 책/gold/protocol/승인/새FINAL0은 유지한다. **전체8/18·요청5/15·남은10개**. 다음 범위·검토자 응답과 문항/관계별 판정, 부족 독립 표본 보강이다. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 이전 단계의 증거는 각 커밋에서 보존한다.

2026-10-08 최신 하위 단계: **N14 exact 실행 보고서/검토 경로 일치 검사 보완**. [수정과 검증](./N14-EXACT-RUN-PROOF-VALIDATION.md) — 수정 전15불일치 수용 RED, 수정 후16거부·multiple proof/nested package 정상 사례와 관련 race/vet 통과. 기존 실제 held 후보의 generic/exact10요청은 명령 시작 전에 거부됐으며 linked report/사람 decision/current0, 원 권위9파일 불변이다. **전체8/18·요청5/15·남은10개 유지**. 다음 N12 명세 범위·관계 및 N11 gold/독립성 판정 반영 후 실제 N14 후보/시험/수용이다. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 아래 기록은 이전 단계 증거를 보존한다.

2026-10-08 최신 하위 단계: **N12 누락 개념 경로5개 범위 대조·실제 retained matcher→stage2→V2 검증3사례 완료**. [범위 검토와 명세 초안](./N12-CONCEPT-PATH-SCOPE-REVIEW.md) — source18참조/관련5패키지17시험·32하위시험/변조6거부/권위9파일 불변. 기존 미작성5개·reviewed 경로0은 유지하며 기능 개념3개·requirement version2 초안은 미적용이다. **전체8/18·요청5/15·잔여10개**. 다음 명세 범위 선택 및 N11 gold/독립성·N12 개념/관계 판정. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정. 아래 기록은 이전 단계의 증거를 보존한다.

2026-10-08 다음 단계 검증: **N12 원문 기반 proposed 구현·시험·기준 관계13개 준비**. [검토 자료](./N12-PROPOSED-TRACE-RELATIONS.md) — 부정확한 테스트 연결1개 제외·미작성 개념 경로5개 유지, 생산API/SQLite/두archive 재생·9거부 제어 통과. **N11 pool13문항/8제안군과 승인0은 그대로이며 관계13개를 FINAL 표본으로 세지 않는다.** 전체8/18·요청5/15·잔여 N08/09/10/11/12/14/15/16/17/18 10개. 다음 실제 관계·명세 범위 판정 및 미작성 경로/독립 평가 입력 보강이다.

2026-10-08 현재 단계: **N11 독립 후보 보강·실제 판정 대기**. [새 검토 문서](./N11-INDEPENDENT-POOL-EXPANSION.md)의 검토 대상FINAL13/8제안군, DEV7, 제외6을 준비했다. 중요군별 승인0·전체8/18·요청5/15·잔여10개를 유지한다. **다음:** 후보 gold/군/독립성·N12 실제 내용 판정과 부족 범위 보강. **남은 전체:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 실제 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 복구, N18 최종 판정. 아래 날짜별 기록은 이전 준비 이력을 함께 보존한다.

2026-10-08 N11 검토 대기 코퍼스 연결: [코퍼스·색인·실행 신원 준비](./N11-HELD-CORPUS-PREPARATION.md) — 이전 N12 색인의6Go 누락/ABS2보관 누락을 실제 대조하고 별도 Git19803801/snapshotb1204cf1/datasetee6f91be의 source1,419/선택66·CKG59/CKV63/785chunks/667canonical을 결합했다. 원문16/16·positive14/14 실제 canonical/chunk·ABS2 uncited basis, 무답 basis 검사 누락 RED11→GREEN25와 실제 DEV/FINAL runner child/output/registry0, 원본 source/index/semantic DB 없이 archive 재생 통과. 새 embedding HTTP71/색인785/probe3, 전체Go DRIFT992 유지. scope/gold/군·독립성/사람 판정/동결은 미승인·FINAL0, N11-A–D unchecked·전체8/18·요청5/15·잔여10개 유지. 다음 N11 실제 판정/독립 FINAL 보강과 N12 판정이다.

2026-10-08 N12 ADR 기술 lock 연결: [검토 대기 ADR의 보관 lock](./N12-PROPOSED-LOCK-PREPARATION.md) — 원소스1,410 불변, proposed ADR2/정책0(운영 owner 미정)/review0의 partial overlay·lock과 별도 held snapshota0457cc1/dataset5a24904f를 생성했다. 생산 knowledge·include-packs CLI/v4 의미27문서·36코드참조/33canonical·7 lock 거부·원 소스/색인/semantic DB 없이 archive 재생 통과. 새 BGE embed HTTP62=probe2+색인60/666입력, 전체Go DRIFT998 유지. 승인 lock/사람 내용·관계·운영 판정은 미완료, N12-A–D unchecked·전체8/18·요청5/15·잔여10개 유지. 다음 N11 실제 gold/독립성 및 N12 판정/정책 담당자·시점 확인.

2026-10-08 N12 기술 연결 검증: [동일 신원의 held 색인·의미 준비](./N12-COORDINATED-PREPARATION.md) — 보관소스1,410파일/선택58의 CKG53Go·CKV56입력·666chunks/566심벌 canonical100%, 문서27/27·코드36/고유33·proposed 저장 roundtrip·12거부 제어·archive-only 재생 통과. 원문project knowledge-system로 수정한 별도 후보이며 첫 mismatch 거부와 전체Go범위 DRIFT998 경고를 보존했다. 실제 BGE embedding HTTP124회(색인입력1332·시작probe4), 새DEV/FINAL0·human0·current/semantic activation0. approved overlay/lock·내용/관계/입력범위 판정은 남아 N12-A–D/전체8/18·요청5/15·잔여10개를 유지한다.

2026-10-08 N12 캡처 입력 준비 검증: [명시 소스 범위의 보관 캡처](./N12-SCOPED-CAPTURE-PREPARATION.md) — 원본667a9af5에서 소스·의존·검토 문서1,410개를 독립 준비 Git 커밋562e37aa로 구성해 committed 캡처/복원·archive-only 재생/신원·69검토span·승인Go29·8거부 제어를 확인했다. snapshot-only는 build31/vendor fixture1 등32개를 제외하므로 전체 입력 보관으로 사용하지 않았다. 제품 정책/소스 변경0, 새 모델/FINAL질의·사람 판정0. 제안 입력 범위·내용/관계 승인 및 새 coordinated CKG/CKV/lock 결합은 미완료, 전체8/18·요청5/15·잔여10개 유지.

2026-10-08 N12 추가 진행: [현행 canonical 연결 준비](./N12-CANONICAL-SOURCE-PREPARATION.md) — 현행 원문69참조/코드canonical36참조(고유33), wrong evidence7거부; 공통SQLite 조회 canonical_id 누락 수정·8경로 회귀와 증분캐시 검증. 전체 root capture의 public.pem 정책 거부는 미완료로 보존. 사람 내용·관계 판정/통합capture·lock·CKV·승격 대기, 전체8/18·요청5/15·잔여10개 유지.

2026-10-08 N11 다음 세부 단계: [후보 범위·좌표 대조](./N11-COVERAGE-CANDIDATE-REVIEW.md)의 작성11개 중 과거관측 동일군2개를 제외해9후보/제안군6개가 남았다. DEV04 근거는 동일본문265–439행으로 재결합했고, 제외 재삽입 방지22시험·실제초안CLI/runner 거부(질의/예약0)를 확인했다. **승인FINAL0·전체8/18·요청5/15·잔여 N08/09/10/11/12/14/15/16/17/18 10개**, 다음 실제gold/범주/독립성·오염·무답/N12 판정과 충분한독립입력 준비다. 아래는 이전 시점 이력이다.

2026-10-08 N14 보강: [실제 변경 검토 자료](./N14-ACTUAL-CHANGE-REVIEW.md)를 준비했다. 머지PR97의7변경 파일/11span과 새 exact5 test run·pass를 대조했으며 제안 연결/승인null을 유지한다. 실제 canonical trace·patch candidate/linked run·사람 수용·승격/반려/rollback은 미완료다. 우선순위와 남은 전체는 [완료 계획](./REFACTORING-COMPLETION-PLAN-2026-10-08.md)을 따른다. **전체8/18·요청5/15·잔여10개**, 다음 N11/N12 실제 판정과 독립 입력/actual candidate 결합.

2026-10-08 최신: [전체 완료 실행 계획](./REFACTORING-COMPLETION-PLAN-2026-10-08.md)을 따른다. PR97 스쿼시 머지·main eeeab2e7 동기화 후 feat/refactoring-followup에서 기존 로컬 증거를 복구했다. 부분 완료 증거를 없애도 checker가 통과하던 빈틈을 실제 복사 ledger로 재현/수정했고8제어를 통과했다. N15-A/B/C·N17-A/B는 커밋 명세/manifest/원자료와 개별 결합한다. **전체8/18·요청5/15·잔여 N08/09/10/11/12/14/15/16/17/18 10개**, 다음 N11/N12 실제 검토와 N14 실제 수용 자료다. 아래는 이전 시점 이력이다.

2026-10-08 현재 PR 상태: [PR97](https://github.com/0xmhha/knowledge-system/pull/97)을 일반 PR(ready for review)로 전환했다. 공개 head d7de0b89와 CI5/5 통과는 유지한다. 전체 미완료/출시 보류를 코드 검토 준비와 묶어 draft를 유지하던 판단을 바로잡았다. merge/출시/새 gold 승인은 없으며 전체8/18·요청5/15·잔여10개와 다음 N11/N12 실제 판정은 동일하다. 아래 draft 생성/유지 기록은 당시 이력이다.

2026-10-07 최신: [PR97/native Linux CI](./N15-NATIVE-LINUX-CI-VALIDATION.md). 사용자 “푸시하고 Pr생성해”로 d08cb384의80커밋 공개 전송·draft PR97 생성 완료. graph/vuln Go 요구 불일치를 공개 source만 수정한 d7de0b89로 보완했고 재실행 CI5/5 success. 실제 Linux AMD64의 설치/재시작/갱신/복구, public consumer7응답/4거부, 복구10응답/13payload SHA와 artifact178파일 SHA를 확인하여 **N15-C 완료**. N15-D/E와 N17-C/D/E 운영 수용은 미완료. 추가 로컬 BGE/출처 receipt 및 본 native 기록은 미전송. **전체8/18·요청5/15·잔여10개 N08/09/10/11/12/14/15/16/17/18. 다음 N11/N12 실제 판정과 독립 입력/런타임 결합.** 아래는 이전 시점 이력이다.

2026-10-06 · 기준 코드 `04d3bfdf` · [원래 목적/코드 대조](./REFACTORING-ORIGINAL-GOALS-REAUDIT.md). 이전 [시험 preview30항목](./EXECUTION-WORKLIST.md)은 사용자 선택 범위에서 종료됐다. 이 문서는 원래 목적과 코드 사이의 잔여를 관리하는 **새 후속 목록**이다. 이전 HC·입력 승인은 다시 요구하지 않는다.

후속 목록 **18개 중 N-01–07 및 N-13 완료, 잔여10개(N-08–12, N-14–18)**. 제품 구현을 새로 완료한 항목은 N-02–07/N-13 7개다. N-15–18의 실제 운영 범위는 이전에 제외됐으며 목록 작성만으로 수행·배포가 승인된 것은 아니다. `study/main` 동기화로 초기 원문3개와 아키텍처/분석을 확보하고 [전체 초기 ID·v2 대응](./REFACTORING-ORIGIN-AND-V2-TRACE.md)을 확인했다. 이 목록의 크기를 원래 전체 목표의 완료율로 환산하지 않는다.

[study 보완 감사](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md)에서 더 오래된 목적 원문과 2026-06-26 마스터 계획을 확보했다. **상위 WI10개는 별도 추적 대상**이며 이 제품의18개와 합산하지 않는다. 상위 범위 전체의 완료 판정에는 coding-agent/ChainBench/별도 학습 저장소의 현행 코드·실행 증거 대조도 필요하다.

[후속 실행 명세·완료 판정 절차](./REFACTORING-EXECUTION-SPEC.md)에 목적·설계·수용 조건별 검증을 기록한다. N-02–07/N-13의 수용 조건별 실행 증거를 대조하고 완료했다. [진행 원장](./REFACTORING-PROGRESS.json)과 `python3 scripts/check-refactoring-progress.py`는 완료 체크/커밋 증거/현행 소스/잔여수 일치를 자동 검사한다.

## 상태와 증거 규칙

- **코드 차이 확인**: 현재 코드가 설계와 다름. 새 장애 주입/회귀 재현 전이다.
- **실패 실측**: 보관된 DEV/FINAL 원자료에 실패/회귀가 있다. 이미 관측한 FINAL은 새 독립 holdout으로 쓰지 않는다.
- **내용/환경/운영 대기**: 도구가 있다고 실제 사람 판정·native 실행·운영 사실을 채우지 않는다.
- 완료는 해당 행의 수용 조건·코드/입력/실행/원자료·제한을 연결했을 때 기록한다. 새 수치 목표/프로토콜은 실험 결과를 보기 전에 고정한다. 기존 안전/좌표/비밀 혼입0·기권 오인용 증가0·Recall/MRR 변화−0.02/중요군−0.05·p95비1.25·소표본 규칙은 임의 변경하지 않는다.

## 남은 전체 목록

| ID | 우선순위·선행 | 상태·현재 근거 | 해야 할 작업 | 완료 조건 |
|---|---|---|---|---|
| N-01 | 완료 · 2026-10-06 | study/main `1c350c1` 원문3개·초기architecture/analysis 확보 | 초기 작성/보관 시점·SHA와 시작 배경 기록. FR10/INV7/NFR5/S9·WBS29를 v2/현행 감사와 대조 | [복구·대응 원장](../../system/eval/b0-knowledge-system/original-source-recovery-2026-10-06/recovered-source-and-trace.json). 기존 탐색 증거 보존. PDF 원본 재검증·상위WI 외부 구현 감사는 별도 |
| N-02 | **완료 · 2026-10-06** | [실행 명세·검증](./REFACTORING-EXECUTION-SPEC.md), 공개 MCP RED→GREEN/CLI 7사례 | pinned v2에서 활성 DB를 직접 수정하지 않게 거부/후보 빌드 경로로 통일. legacy 유지보수 동작과 migration 경계 명시 | 한 엔진 실패/혼입/취소에도 이전current·신원·원문 보존; 실제 공개 MCP와 versioned setup 경로의 동일 후보/승격 증거. [새 DEV 원자료](../../system/eval/b0-knowledge-system/refactoring-n02-2026-10-06/cli-results.json); 동시 writer/내구성은 N-03/04에 유지 |
| N-03 | **완료 · 2026-10-06** | [공통 OS 잠금 검증](./REFACTORING-EXECUTION-SPEC.md), legacy live PID·8process 경합/SIGKILL·CLI 충돌 | v2 빌드/승격/rollback의 OS 잠금 계약 정리·구현. 살아 있는 긴 빌드의 age reclaim·동시 회수 경합을 재현 | 긴 live holder/동시 요청/owner crash에서 두 writer 없음; PID·나이로 살아 있는 잠금 탈취 없음; legacy 호환 정책 기록. [DEV 원자료](../../system/eval/b0-knowledge-system/refactoring-n03-2026-10-06/cli-results.json); 구 writer 중단 후 전환·local flock 경계 |
| N-04 | **완료 · 2026-10-06** | [내구성 DEV 증거](../../system/eval/b0-knowledge-system/refactoring-n04-2026-10-06/manifest.json), WAL·linked RED→GREEN·8+5 실패 경계·7 SIGKILL·CLI 7+2 | DB checkpoint/close·source/manifest/candidate sync·current 전환/부모 sync 순서와 crash recovery 계약 보강 | 전환 단계별 실패/강제 종료 주입으로 이전 또는 완성된 새candidate만 관측. 전원 차단 검증/한계는 플랫폼별 구분 |
| N-05 | **완료 · 2026-10-06** | [DEV 검증](../../system/eval/b0-knowledge-system/refactoring-n05-2026-10-06/cli-results.json), 공개 CLI 12사례·live MCP reader·stale/취소/2 process crash | 활성·rollback·실행 중 pinned reader·검토 대기 candidate 참조를 보호하는 GC dry-run/실행과 용량/보관 정책·status 보고 | 회수 계획 결정적·참조 중 blob/이력/의미 원문 삭제0; 경합/취소/복구 시험; 실제 삭제 전에 검토 가능한 dry-run |
| N-06 | **완료 · 2026-10-06** | [새 DEV 공개CLI12사례](../../system/eval/b0-knowledge-system/refactoring-n06-2026-10-06/cli-results.json), 상한/외부origin·deadline/취소·공개DTO·race 검증 | 파일 수/단일/총 바이트 상한을 setup/config로 전달, build deadline·용량 예상/여유 공간·공개 resource 상태 구현 또는 계약 수정 검토 | 기본100000/32MiB/4GiB·계약2h와 설정값 결합, 초과/timeout에서 candidate 실패/이전current 보존, 민감 path 비노출 |
| N-07 | **완료 · 2026-10-06** | [새 DEV RED→GREEN](../../system/eval/b0-knowledge-system/refactoring-n07-2026-10-06/manifest.json), public RPC partial/typed error·13패키지 race | 32000바이트/12인용 경계 안의 보관 근거 선택/부분 상태 및 안전한 typed budget 오류를 설계하고 독립 DEV 재현 후 수정 | 본문/좌표/해시 훼손 없이 상한 준수; 오류·부분·정상 분모 유지; source_missing/mismatch 구분·권한 누출0; 원 FINAL 불변 |
| N-08 | P1 · N-11, 필요 시N-12 | 실패 실측: FINAL 정적 양성Recall1/6·8arm 동일 | CODE/WHY/POLICY/TRACE 기대 근거가 후보/조립에서 빠지는 원인 분리. 새 DEV에서 primary recall·청크·lexical/graph 조합 개선 | 질문/gold hardcode 없음; 원문/프로젝트/모델 신원·기본 안전 유지; 사전 승인 품질/회귀·중요군 수용 기준으로 개선 확인 |
| N-09 | P1 · N-11 | 실패 실측: ABS01/02 strict no-citation fail | 무답의 검색 근거 상태와 downstream 답변 보류 계약을 명확히 하고 부적합 인용/무근거 운영 추정을 차단 | 승인된 무답/답가능 대조 사례에서 guard 검증; 사람 기권/기계 무인용 각각 집계; 생성 답변 평가가 없으면null |
| N-10 | P1 · N-11, N-07/08 영향 | 실패 실측/진단: 일부p95비>1.25·neighbors 오류; [typed 세부 진단](./N10-NEIGHBOR-DIAGNOSTICS.md) 검증, 전체 미완료 | model/backend/원문 검증/의미 투영 비용 분해, 비반환 neighbors 오류 원인·영향 확인, 범위 내 비용/오류 개선 | 같은pack·같은ontology·static/dynamic/pooled·warm/cold·성공/오류 분모 유지; 실제K/MID/호출/크기와 회귀1.25 검증; 운영 부하는별도 |
| N-11 | **P1 · 개발 튜닝 전에 준비** | 현FINAL 소그룹n<10·이미 관측 | 새 DEV 버전/질문과 결과 사전 비공개 독립 FINAL, 중요군 표본·평가 단위·정답·임계치·회전/비용 조건을 사전 검토·동결 | 충분한 독립 중요군으로 판정하거나 여전히inconclusive/disabled 명시. 반복/언어/상태/HC 문장 표본 부풀림0·이전FINAL 튜닝/holdout 재사용0 |
| N-12 | **완료 · 2026-10-09** | 사람 판정 대기: [실제 검토 자료](./N12-ACTUAL-PILOT-REVIEW.md);27원문span/6테스트 및 [정책·ADR 보완](./N12-POLICY-ADR-REVIEW.md)의6주장/12관계·4거부 제어 확인, proposed 유지 | 실제 파일럿 concept/term/claim/정책/ADR/요구·기준과 코드 앵커를 검토. 대표 추출 오차/관계 오용/검토 시간·비용 기록 | 실제 사람 승인/수정/기권·근거 원문SHA/권위/범위/시점과 새lock/snapshot 결합; HC/SF 승인 재사용으로 운영 사실 승인하지 않음 |
| N-13 | 완료 · 2026-10-06 | [N13 DEV manifest](../../system/eval/b0-knowledge-system/refactoring-n13-2026-10-06/manifest.json) · CLI11회/기존 파일80개 변경0/관련5패키지 race | 조직 catalog를 명시 선택형 pack 자료로 분리; 생성기의 허위 앵커 검사 완료 문구와 구 CLI 명령 교정 | 무팩/구팩·8매핑/필터/앵커/빈queue; pin/의존/type/출처/경로 검증; 기존 출력 보호·상태 자동 승격0 검증 |
| N-14 | **완료 · 2026-10-10** | [수용 기록](./N14-CHANGE-ACCEPTANCE.md): 실제 변경 7c57a388 hold 후보·patch·linked run9/10·기준4 사용자 승인·승격·rollback·보관본 재생 | 실제 파일럿 ADR→requirement→criterion→code→test 연결, 변경patch→실제run→사람 수용→새색인/승격·거부/rollback 사례 검증 | 테스트pass/linked/verified/accepted 각각 기록, 무관test·정책/구현불일치·stale로 허위승격0; 실제 검토 계획/수용 원장 |
| N-15 | **완료 · 2026-10-10(로컬 개인 운영)** | [수용 기록](./N15-N17-LOCAL-OPERATION-ACCEPTANCE.md): go-stablenet 실제 색인·MCP 소비자·submodule 수정·지원/비용/제한 사용자 수용 | 대상 지원을 확정하고 native LinuxAMD64·실운영 규모/모델·외부 CLI/MCP/daemon 및 실제 프로젝트 설치/upgrade 검증 | OS/CPU/binary/source/모델 pin과 native/에뮬 구분; 실제지원/preview/미지원 명시. coding-agent 총비용은 외부별도측정 |
| N-16 | P2 · 로컬 운영 결정 기록, 서명 키 미사용 | 권한자=사용자·신뢰=commit/binary SHA·재배포 없음 기록; 공개키 fingerprint 조건 미충족 | OP01–06 역할/권한·scope/schema·public key/독립 신뢰·보관/교체/폐기·native/전이 고지 적합성 검토 | 실제 권한자/공개키fingerprint/신뢰경로·결정시각/근거, 운영계약 회귀, 정확고지범위·담당 적합성 판정; 개인키 본문은 저장하지 않음 |
| N-17 | **완료 · 2026-10-10(로컬 개인 운영)** | [수용 기록](./N15-N17-LOCAL-OPERATION-ACCEPTANCE.md): 백업·손실 모의·복구 첫 질의35초·RTO1h/RPO24h 사용자 수용 | 실제 backup/failover/복구 담당·RTO/RPO/재시작/키 복구를 정하고 실훈련. network agent/전원 정책의호스트상태 확인; AC-only 정책 범위 검토 | 최신 운영후보/데이터로 복원·이전current/인용/키 신뢰 재생; 실제 실행·한계/담당 기록. 원격운영 상태를 로컬code만으로 완료하지 않음 |
| N-18 | **P3 · N-08–17/변경 회귀** | 품질fail·소그룹inconclusive·disabled·release withheld | 새 독립평가·원래회귀·사람비용/내용·대상지원·운영결정으로 기본값/출시 판정. 릴리스 문서/리뷰/merge/publish 준비 | 합격 근거 없으면disabled/보류 유지. 실제 merge/push/배포는 구체 산출물과 별도권한으로 실행; preview종료 승인을 운영권한으로 확장하지 않음 |

## 실행 순서와 우선순위의 이유

1. **N-02→N-03→N-04**: 새 검색 실험 전에 활성 데이터/동시 writer/복구 기반을 보호한다. N-05/06은 이 기반 위에 둔다.
2. **N-07**, **N-11 준비**: 이미 실측된 본문 예산 오류를 새 DEV 실패 시험으로 연결한다. 새 품질 목표·분할·표본은 결과를 보기 전에 검토한다.
3. **N-12/13→N-08/09/10→N-14**: 실제 지식과 원문 검색을 구분해 품질/기권/비용을 개선하고 실제 기준 수용 경로를 검증한다. 사실 승인 대기 중 구조·시험 준비는 진행 가능하다.
4. **N-15→N-16/17→N-18**: 운영까지 요구하는 경우 환경·키/신뢰/적합성·복구를 충족한 뒤 기본값/출시를 판정한다. 실패면 승인된 범위의 보고/보류로 종료할 수 있으나 운영 출시 완료라고 표시하지 않는다.
5. **N-01 완료**: 원본 복구와 전체 ID 대응에서 새 필수ID 누락은 찾지 못했다. 원래 목적의 품질·검토·비용·설치 조건은 기존 잔여에 유지하고 확인된 코드 차이의 재현을 이어간다.

## 기존 승인과 범위 유지

정적12/BGE·Go29/프로토콜/F01–06/SF 테스트 사례/STV2·HC01–11 해석 승인과 시험 preview 종료 결정은 유효하다. 새 목록 때문에 그 승인을 취소하거나 같은 질문을 다시 요청하지 않는다. 새 실제 파일럿 사실, 새 사전평가 입력/목표, 실제 운영 역할/키/지원/법적 적합성은 별도 대상이다.

GraphRAG 커뮤니티, RDF/OWL/SHACL, 물리 컬렉션 분할, 긴 함수 overlap 조정은 [PDF 추적표](./PDF-IMPROVEMENT-TRACE.md)의 조건부 연구 항목이다. 필요성/수용 조건 없이 필수 미완료로 추가하지 않았다. 이전 May CKV 계획이나 다른 저장소의 전체 coding-agent/ChainBench 작업도 이번 목록에 자동 편입하지 않았다.

**현재 단계:** N-01–07 및 N-13 완료(8/18), 요청15개 중5개 완료. **다음 작업:** N-11 새 사전평가 입력·프로토콜과 N-12 실제 내용 검토 판정. **남은 전체:** N-08/09/10/11/12/14/15/16/17/18, 10개. 각 실제 수용 조건이 충족되기 전에는 완료 처리하지 않는다.

**상위 목표의 추가 추적 전체:** WI-V1 오라클 타당성, WI-V2 수정안 선택, WI-3 의심 라벨링, WI-4 비결정 버그, WI-1 값 흐름, WI-2 규칙 태깅, WI-C 깊은 코드 리뷰, WI-6 값 흐름 기반 다단 확장, WI-SP 정책/보안패턴 노출, WI-5 별도 사람 검토 학습. 각 상태/보류/담당·제품 N 연결은 [복구한 상위10개 표](./STUDY-ORIGINAL-PLAN-FOLLOWUP.md#3-복구한-상위-작업계획의-전체10개-항목)를 기준으로 한다. 다음 상위 감사는 담당 저장소와 현재 WI-V1/V2/3 증거 확인이다.

N11 [새 DEV7개/프로토콜 초안](./N11-FRESH-EVALUATION-PREPARATION.md)은 미승인/미실행이며 새 FINAL0이다. N12 실제20타입/3요구/4기준 검토 자료를 요청했고 사람 판정·활동 시간 및 운영 환경/담당 입력을 기다린다. 준비 자료를 완료로 체크하지 않았다.

N12 정책·ADR 원문 주장이 준비 범위에서 누락된 것을 재감사로 발견해 [별도 검토 자료](./N12-POLICY-ADR-REVIEW.md)로 보완했다. 원문 커밋ab77ce56,6주장/12관계 proposed, 원문span 대조 및 실제 CLI4개 음성 제어 통과. 기존27개 사람 검토 요청은 보존한다. 새 주장/관계 승인0, 통합 lock/canonical·의미 수용 미완료이므로 N12 완료 체크와 전체 수는 바꾸지 않는다.

N11-C [실행 보호 세부 단계](./N11-EVALUATION-RUNNER-CONTRACT.md)를 구현·검증했다. 승인된 전체 책/runtime pins 검사와 FINAL durable reservation/영구 OS 잠금을 matrix 경로에 연결했다. Python19개·관련2패키지 race·native CLI4제어 통과, 실제 초안 child dispatch0/질의0. 새 사람 판정과 실제 평가 결과가 없으므로 N11 및 전체 완료 수는 그대로다.

N10 진단의 구조 RED→GREEN과 실제 SQLite3제어를 추가했다. 미해결 seed typed 표시를 추가하되 backend_error/모든 시도·composer 결과를 보존하고, 과거 정보가 없으면null이다. Go3패키지 race/vet·Python17개 통과. 비용/p95 새 평가가 없어 완료8/18·요청5/15·잔여10을 유지한다.

N11 [부분 FINAL 후보](./N11-FINAL-CANDIDATE-PREPARATION.md)4문항을2제안 사실군으로 묶고 원문·span SHA에 연결했다. 원 DEV7개와 프로토콜 바이트는 불변, 승인된 독립 FINAL0·실행/예약0이다. 전체 입력 검사 exit2, 미승인/미동결 및 중요군 미충족을 유지한다. 후보 작성은 전체 N11 완료가 아니다.

2026-10-07 N15/N17 [현행 패키지·외부 모듈·복구 세부 단계](./N15-N17-CURRENT-TECHNICAL-VALIDATION.md): clean native Darwin 패키지/3mock 프로젝트/외부 Go 공개 계약7응답/4거부 제어, 새 root 복원 reader lease 결함 수정·9제어·3패키지 race·복구10응답/13payload SHA 통과. native Linux CI는 정의만 존재, 실제 BGE/외부 운영 소비자·고지/권한/RTO/RPO 수용은 미완료다. 전체8/18·요청5/15·잔여10개를 유지한다.

2026-10-07 native CI 사전 검사: 전역 formatter의 동결 평가 원문3개 변경 요구를 관리 코드/fixture 범위로 제한했다. 5제어와 make fmt-check 통과, 원문/Go runtime 소스 변경0. 전체 완료 수와 CI 미실행 상태는 동일하다.

2026-10-07 [BGE-M3 작은 설치 호환성/전체 race](./N15-BGE-COMPATIBILITY-VALIDATION.md): 승인 digest/native1024의3소유 fixture 설치·MCP 재시작·데이터 갱신·rollback,6dataset identity·compact MCP7/CKV6응답과 보관 Git 원문 좌표를 확인했다. d08cb384 전체 race117패키지와 도메인 생성 자료 일치도 통과했다. N15-D의 모델 연결만 검증했으며 native Linux/규모/독립 운영 소비자·사람/운영 수용이 남아 전체8/18·요청5/15·잔여10개를 유지한다. 다음 N11/N12 실제 판정과 native CI다. 공개 전송 요청은 d08cb384의80커밋이며 추가 로컬 receipt/문서 커밋은 제외한다.

2026-10-07 [N11 후보 출처/과거 관측 대조](./N11-CANDIDATE-PROVENANCE-AUDIT.md): 전체 보관 FINAL9파일/4,800행의 SHA를 확인했다. 후보 lock.go 파일의 동일SHA/80행 인용720개를 발견했지만 후보parser55–76행 교집합0이다. 해당3request의 반복을 독립 표본으로 세지 않으며 후보의 독립성·개념적 오염은 미판정이다. 현재4후보/2proposed cluster·승인FINAL0·새질의0·N11 전체 미완료를 유지한다. d08cb384 공개 전송은 자동 승인 검토에서 새 범위 미승인으로 차단됐고 명시 응답 대기다. 전체8/18·요청5/15·잔여10개, 다음 N11/N12 판정 및 native CI를 유지한다.

2026-10-08 N11 후보 보강: [독립 FINAL 후보·범위 부족분](./N11-INDEPENDENT-POOL-EXPANSION.md). 신규CODE/WHY4문항/2제안군 추가, 기존 사실과 겹칠 위험의 신규4문항 격리. 검토 대상FINAL13/8제안군·DEV7, 승인된 중요군 모두0. 과거DEV/FINAL27gzip/11,632행 및 실제20원문 참조 대조, 정보 메타데이터 교정·새runtime 미결합, 실제input checker exit2/source오류0; 모델/검색/빌드0. 전체8/18·요청5/15·잔여10개 N08/09/10/11/12/14/15/16/17/18 유지. 다음 N11/N12 실제 내용·군/독립성 판정과 부족 범위 보강.

2026-10-08 N12 trace 연결 준비: [13개 proposed 관계와 의미 경계 검증](./N12-PROPOSED-TRACE-RELATIONS.md). 구현3/시험5/기준5 관계를 별도 SQLite에 원문·canonical·CKV로 연결하고 재조회/원본 없는 두 archive 재생·9거부 제어를 통과했다. Projection.Validate 시험을 다른 정렬 함수에 연결하는 부정확한1쌍은 제외, broad concept5경로 미작성 유지. 전체25관계 proposed·3요구 spec_unapproved·검토된경로0/current없음/human0/model0. 신규FINAL로 합산하지 않으며 N11 pool13/8·승인0, 전체8/18·요청5/15·잔여10개 유지. 다음 관계/원문 범위 판정·5경로 보강과 N11 독립 입력 확보다.
