# 리팩토링 날짜별 진행 기록 — 2026-10-13 정리

역사 기록이다. 아래의 “현재”, “최신”, 완료 수와 다음 작업은 각 기록 시점의 상태이며 실행 지시가 아니다. 최신 상태는 [현재 인수인계](./CURRENT-HANDOFF.md), [작업리스트](./REFACTORING-REMAINING-WORKLIST.md), [진행 원장](./REFACTORING-PROGRESS.json)을 따른다. 원문 포착 커밋은 d5784a20이며, 문서 앞부분의 기록을 내용 변경 없이 같은 디렉터리로 옮겨 상대 링크를 유지했다.

## 작업리스트의 이전 머리말


2026-10-13: **온톨로지4/4·O04-A–D 완료(품질inconclusive·기본off)**. [최종 판정·원자료](./ONTOLOGY-FINAL-VERDICT-2026-10-13.md): 사전동결4387b888·실제BGE-M3/heldWBFT702응답/351쌍·실패/누락/pending0·재채점일치. Recall1.0→1.0, expectedRR0.6583→0.6458, warm p95비1.0855이나3문항1.25초과·전체성능미수용. 독립자격미충족/독립승인0/새FINAL0을명시하고원명세의조건·한계판정/증거결합을완료했다. 기존17/18·전체잔여1개 N16(사용자결정으로공개키fingerprint 미충족유지). 운영활성0·공개배포보류 유지.

2026-10-11 최신(Claude Code): **N18 완료(사용자 승인)**. [판정](./N18-RELEASE-VERDICT-DRAFT.md) — 선택 기능 기본값 유지, 로컬 개인 운영 허용, merge·공개 배포 보류. N16은 공개키 fingerprint 조건을 사용자 결정으로 미충족 유지. **현재 17/18 완료, 남은 1개:** N16 신뢰·권한.

2026-10-11 최신(Claude Code): **N08·N09 종료(사용자 결정)**. N08: FINAL2 개선 근거 없음 → 사용 타입 선언 삽입을 opt-in(`retrieval.type_neighbors`, 기본 off)으로 전환(`16332767`), 기본 인용은 이전 binary와 7/7 동일. N09: guard 기계 동작 확인, 무답2단위 inconclusive로 종료. **현재 16/18 완료, 남은 2개:** N16 신뢰·권한(서명 키), N18 기본값·출시 판정.

2026-10-11 최신(Claude Code): **FINAL2 실행·채점, N10 완료**. [결과·판정](./FINAL2-RESULT.md) — A(후보/참조 각 DEV1,200·FINAL2,400행)와 B(DEV240·FINAL960행)를 오류0으로 실행했다. 후보 FINAL warm p95비 1.039–1.109(참조 1.334–1.873)로 N10-C 충족. N08은 recall 동일·MRR −0.032(타입 선언 삽입이 gold 위로 들어간 5문항)로 개선 근거 없음. N09는 무답 on arm 240/240 `declared_unknown`·대조 잘못된 기권0이나 무답2단위라 inconclusive. **현재 14/18 완료, 남은 4개:** N08 검색 품질, N09 기권, N16 신뢰·권한(서명 키), N18 출시 판정.

2026-10-10 최신(Claude Code): **FINAL2 초안 검토 요청**. [초안](./FINAL2-DRAFT-REVIEW.md) — A: 기존 corpus에서 새 DEV5·FINAL10 문항, 이전/새 binary 비교(runner `1d25abf8`, 채점 `ec27582f`), N10 p95 판정. B: knowledge 미지정 사실 2건과 무답2·대조2 문항. 입력 검사 근거 오류·관측 중복 0, 검토·동결 대기. **현재 13/18 완료, 남은 5개:** N08, N09, N10, N16, N18.

2026-10-10 최신(Claude Code): **N09·N08 제품 변경**. [N09 미지정 사실 기권](./N09-DECLARED-UNKNOWN.md)(`22e45ff7`), [N08 사용 타입 선언](./N08-TYPE-NEIGHBORS.md)(`f2f463db`, `7077741f`): DEV 개발 측정 Recall@10 0.762→0.833·MRR 0.544→0.563·p95비 0.98–1.05. **현재 13/18 완료, 남은 5개:** N08·N09·N10(저장소 unknowns 자료 검토·새 corpus·새 독립 FINAL), N16 서명 키, N18 출시 판정.

2026-10-10 최신(Claude Code): **N10 요청 단위 archive 검증 공유**. [변경·DEV 측정](./N10-REQUEST-VERIFICATION.md) — 요청당 전체 검증 3–8회를 2회로 줄였다(`37302b35`). DEV 개발 측정 p95비 0.92–0.98(DEV3 1.20–1.73), 품질 동일. N10-C는 관측 뒤 변경이라 새 독립 FINAL이 필요하다. **현재 13/18 완료, 남은 5개:** N08 검색 품질, N09 기권, N10 비용·지연(새 FINAL), N16 신뢰·권한(서명 키), N18 출시 판정.

2026-10-10 최신(Claude Code): **N11 완료(inconclusive/disabled/출시 보류)**. [DEV3·FINAL 결과](./N11-FINAL-RESULT.md) — 깨끗한 DEV3(1,680행)와 독립 FINAL(2,400행)을 오류 0으로 실행·채점했다. 8arm 품질 동일, FINAL warm p95비 1.37–1.97(N10-C 미충족), ABS off-basis 인용 100%(N09 미충족), N08 개선 근거 없음. **현재 13/18 완료, 남은 5개:** N08 검색 품질, N09 기권, N10 비용·지연(p95), N16 신뢰·권한(서명 키), N18 출시 판정. 다음 N10 projection 지연 개선 또는 N08 후보 수정(새 DEV·새 독립 FINAL 필요).

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


## 실행 명세의 이전 머리말


2026-10-11 최신 단계: **N18 완료(사용자 승인)**. [판정](./N18-RELEASE-VERDICT-DRAFT.md) — 선택 기능 기본값 유지, 로컬 개인 운영 허용, merge·공개 배포 보류. N16은 사용자 결정으로 공개키 fingerprint 미충족 유지. 전체17/18·남은1개: N16.

2026-10-11 최신 단계: **N08·N09 종료(사용자 결정)**. N08은 FINAL2 개선 근거가 없어 사용 타입 선언 삽입을 opt-in(`retrieval.type_neighbors`, 기본 off)으로 돌렸고, 기본 인용은 이전 binary와 7/7 같다. N09는 guard 기계 동작을 확인했지만 무답 2단위라 inconclusive로 종료했다. 전체16/18·남은2개: N16, N18.

2026-10-11 최신 단계: **FINAL2 실행·채점, N10 완료**. [결과·판정](./FINAL2-RESULT.md) — 후보 FINAL warm p95비 1.039–1.109로 N10-C 충족, 검색 차이는 모두 N08 삽입. N08 MRR −0.032·개선 근거 없음, N09 guard 동작·inconclusive. 전체14/18·남은4개.

2026-10-10 최신 단계: **FINAL2 초안 검토 요청(미동결·미실행)**. [초안](./FINAL2-DRAFT-REVIEW.md) — 소규모: N10은 p95비로 판정 가능, N08·N09는 소표본으로 inconclusive 예정. runner 참조 binary 실행·채점기 binary 비교 추가. 전체13/18·남은5개.

2026-10-10 최신 단계: **N09 미지정 사실 기권·N08 사용 타입 선언 추가(제품 변경)**. [N09](./N09-DECLARED-UNKNOWN.md) — 검토된 unknowns 레코드에 일치하면 근거 줄만 인용·`declared_unknown`, 혼합 인용 거부(시험6·변이6). [N08](./N08-TYPE-NEIGHBORS.md) — uses_type 선언+doc comment 최대2개 삽입, DEV 개발 측정 Recall@10 0.762→0.833·MRR 0.544→0.563, p95비 0.98–1.05. 저장소 unknowns 자료 검토·새 corpus·새 독립 FINAL(이전/새 binary 비교 포함)이 남는다. 전체13/18·남은5개.

2026-10-10 최신 단계: **N10 요청 단위 archive 검증 공유**. [변경·DEV 측정](./N10-REQUEST-VERIFICATION.md) — 요청당 전체 검증 3–8회→2회, 시험3·변이4 검출·17패키지 race 통과, DEV 개발 측정 p95비 0.92–0.98·품질 DEV3와 동일. 종료 시 강제 종료 9건(측정 행 실패0)과 N10-C의 새 독립 FINAL 필요가 남는다. 전체13/18·남은5개: N08, N09, N10, N16, N18.

2026-10-10 최신 단계: **N11 완료(inconclusive/disabled/출시 보류)**. [DEV3·FINAL 결과](./N11-FINAL-RESULT.md) — 깨끗한 DEV3와 독립 FINAL(2,400행, 오류0)을 실행·채점했다. 8arm 품질 지표 동일(FINAL Recall@10 1.0·MRR 0.587), warm p95비 1.37–1.97로 N10-C 미충족, ABS off-basis 인용 100%로 N09 미충족, N08 개선 근거 없음. 전체13/18·남은5개: N08, N09, N10, N16, N18.

2026-10-10 최신 단계: **N15·N17 완료, N16 미완료**. [로컬 운영 수용](./N15-N17-LOCAL-OPERATION-ACCEPTANCE.md) — 사용자 범위 결정(로컬 개인 운영, 소비자 go-stablenet)으로 실제 bge-m3 색인(29.5분·20,958 embedding·1.2GB), submodule 캡처 결함 수정(e90fab1f), MCP 질의 5건(11–18초), 백업·손실·복구 훈련(첫 질의 35초)을 실행하고 사용자가 수용했다. N16은 서명 키를 쓰지 않기로 해 공개키 fingerprint 조건 미충족. 전체12/18·남은6개: N08, N09, N10, N11, N16, N18.

2026-10-10 최신 단계: **N14 완료**. [수용 기록](./N14-CHANGE-ACCEPTANCE.md) — 실제 변경 7c57a388(retained archive 검증 batch)을 소유 dataset의 hold 후보(snapshot b4d63460)로 색인·patch 등록, 검토 판정 내용 동일 결합, linked run 9/10, 잘못된 run 4종 거부, 기준4개를 사용자 판정(표기 opus, 초안 이유 채택)으로 승인·승격, 반려 대조 거부, 해제된 버전 사이 rollback, 원본 없이 보관본 재생 통과. 전체10/18·남은8개: N08, N09, N10, N11, N15, N16, N17, N18.

2026-10-09 최신 하위 단계: **N12 새 승격 통합 시험의 동일 source/index/관계 결합 및 archive-only 재생 검증 완료**. [개별 검토 자료](./N12-ANCHOR-BOUND-TRACE-REVIEW.md) — source1,438/선택71·CKG63/CKV66/747chunks625canonical, 실제canonical18·evidence81·신규proposed17(총29), race8시험29하위시험/변조5거부, 원본 source/index/store 없이 archive1,457권위파일 재생·bytes 동일. 전체Go DRIFT992 유지. 좌표 함수의 부분 역할과 실제CKG+mockCKV 통합 제어를 구별했다. human/reviewed path/linked run/FINAL/actual activation0, 전체8/18·요청5/15·남은10개 유지. 다음 실제 검토자·개별 내용 판정 및 N11 gold/독립성·부족 표본·runtime 동결. 남은 전체 N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 출시 판정. 아래는 이전 단계 이력이다.

2026-10-08 최신 하위 단계: **N12 actual foreign-code-anchor 승격·active 보존 통합 회귀 보강**. [새 시험·범위](./N12-CODE-ANCHOR-PROMOTION-REGRESSION.md) — 공개CKG2source/2DB·canonical stable/commit 다름,5사례8쓰기+2activation 거부/old current·저장 자료·reopen 보존. semantic race42시험20하위시험/vet/boundary/format 통과. 최초 fixture YAML 작성 오류는 제품RED와 별도 보존했다. 기존 held66artifact·승인input5 불변, 새 시험은 이전55d8/43ba/f839 pilot 후보에 아직 미포함이다. 전체8/18·남은10개 유지;다음 새 source/index/canonical 관계 결합과 N11 실제 gold·독립성/검토자 판정. 남은 N08/09/10/11/12/14/15/16/17/18. 사람 승인·새FINAL·실모델·운영 활성화0.

2026-10-08 최신 하위 단계: **N12 기능 version2의 새 실제 source/CKG·CKV/관계15 준비 검증**. [증거·기준별 누락](./N12-FUNCTIONAL-V2-TRACE-PREPARATION.md) — source1,437/CKG62·CKV65/742chunks·621canonical,15proposed 관계/11canonical/74span/27문서,related race7시험24하위시험·5거부·원 source/dataset/store 없이 archive 재생 통과. 전체Go DRIFT992는 유지했다. 개별 판정0/spec_unapproved3/활성0/새FINAL0, 전체8/18·요청5/15·남은10개 유지. 다음 N12 actual foreign-code-anchor 승격·active 보존 통합 시험 보강과 N11 gold·독립성/실제 검토자 판정. 남은 전체 N08/09/10/11/12/14/15/16/17/18. 이전 후보·승인input5는 보존하며 새 관계를 독립 FINAL로 세지 않는다.

2026-10-08 최신 하위 단계: **N11/N12 권고 범위2개 선택 적용**. [적용·원문 검증](./N11-N12-SCOPE-ADOPTION.md) — POLICY/TRACE 코드 계약과 실제 조직/검토 경로 구분, 새 metadata 초안7행;N12 core20보존+functional3·요구3/v2/criteria4·30source span/30digest 거부·SQLite reopen/bundle 독립 재생 확인. scope2선택은 개별 content 승인과 별도이며 reviewer name미입력/개별gold·관계·criterion0/spec_unapproved3/current0이다. 기존 pool·held 후보·승인input5는 보존했다. 전체8/18·남은10개 유지;다음 개별검토·부족 독립군 보강·새 전체 source/index 결합. 남은 N08/09/10/11/12/14/15/16/17/18. Claude/model/새FINAL0.

2026-10-08 최신 하위 단계: **N09 CKV 평가 보조 함수의 빈 snippet 오판 수정**. [수정·검증](./N09-EMPTY-SNIPPET-VERIFICATION.md) — 수정 전4내용없음 사례 Verified=true/집계 오류 RED, 수정 후 empty_snippet 진단·관련2패키지 race103시험4하위시험 pass/실BGE smoke1skip·vet/boundary/format 통과. 임시Git 서명·Go cache 환경 실패는 별도 보존하고 명령 범위의 fixture 서명 설정만 조정했다. 실제 기권/인용0/새FINAL/사람 판정0, 전체8/18·남은10개 유지. 다음 N11 gold·독립성/범위·충분한 표본 보강과 N12 실제 관계 판정 후 동결/평가. 남은 전체 N08/09/10/11/12/14/15/16/17/18. Claude 실행/외부 전송0, Codex 직접 수행.

2026-10-08 최신 하위 단계: **Codex 직접 N11 추가 원문8범위·응답 두 표면 조사**. [조사와 보류](./N11-SOURCE-PROBE-INVESTIGATION.md) — source8+기존 제외 기준1, 고정 Markdown259개와27gzip/11,632행을 대조했다. response.content23,262문자열을 추가 검사했고 기존CAND05의480노출을 재확인했다. 6범위는 이전 설계·튜닝으로 격리,2개는 의미 중복 검토가 필요한 비실행 초안으로 보류했다. 기존DEV7/FINAL후보13/제안군8/제외6·승인0, 전체8/18·남은10개 유지. 다음 실제 범위/개별 gold·독립성·관계 판정과 더 넓은 미사용 사실군 확보. 남은 전체 N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 출시 판정. Claude 실행/외부 전송0이며 사용자 요청에 따라 직접 실행으로 전환했다.

2026-10-06 · 개발 시작 코드 `c9c91ebe` · [전체 작업·우선순위](./REFACTORING-REMAINING-WORKLIST.md) · [원래 요구사항/WBS 대응](./REFACTORING-ORIGIN-AND-V2-TRACE.md)

2026-10-08 추가 검증: [N14 exact 보고서 통합 회귀](./N14-EXACT-RUN-ROUNDTRIP.md)의 소유 root/nested CKG 원문·생산 runner→공개 import/재시도·proof 거부·immutable 보존·자동 판정/승격 없음과 race47시험33하위시험 통과. N12 미적용 범위 초안의 필수 질문을 교정하고 생산 파서/구조를 확인했으나 실제 source binding/사람 승인으로 대체하지 않는다. 전체8/18·남은10개, N14-A–E는 미체크다.

2026-10-08 N12 추가 검증: [생산 shared-alias 통합 회귀](./N12-SHARED-ALIAS-INTEGRATION.md)는 소유한 원문/좌표 fixture의 실제 파서·SQLite 재열기·동률 matcher2/9개·상한 fallback·retained 정제/무결성을 연결했다.10질문 제어/변조10거부 및 관련 race116시험104하위시험 통과. 기존 실제 파일럿 미적용·사람 판정/새 FINAL0이며 N12-A–D/N14-A–E와 전체8/18·남은10개를 유지한다.

2026-10-08 N11 추가 출처 대조: [응답 본문 감사](./N11-RESPONSE-EXPOSURE-AUDIT.md)로 기존27gzip/11,632행·71,542body·graph neighbor0 및26source refs를 검증했다. CAND05의104행480본문은 실제fileSHA/행SHA까지 동일하며 기존 제외를 유지한다. 긴 단일 행까지 검사한 나머지 일치0은 독립성 승인으로 사용하지 않는다. 전체8/18·남은10개 및 N11-A–D 미체크, scope/gold/군·독립성 판정과 동결은 남는다.

