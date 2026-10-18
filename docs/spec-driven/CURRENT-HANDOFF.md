# 단계59 — WI-3 계측 시 출처 식별 기능 보존 / 2026-10-19

[단계59 보고서](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/REPORT.txt), [명세와 검증 근거](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/acceptance-review.json), [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/WORKLIST.json)이 최신 진입점이다. **AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처는 요구하지 않는다.

`logging.measure_backend_calls`를 켜면 래퍼가 선택적 CitationResolver를 숨겨 실제 선언 위치 입력이 거부되고 후보 심볼이 사라졌다. 수정 전 어댑터/실제 MCP handler 회귀가 실패했고, 수정 후 관련332개가 통과했다. 래퍼는 공급자가 지원할 때만 그 기능을 보존하고 문맥·인용·반환값·오류를 그대로 전달한다. 기존 계측 scope 안에서는 입력 hash/길이와 안전한 오류 종류만 기록하며 원문 경로·심볼·오류 문구를 기록하지 않는다. 지원하지 않는 공급자에 기능을 만들어 붙이지 않는다.

전체 빌드·일반/race 각4123통과/기존18skip/실패0, fmt·vet·고정 lint2.14.0·경계·문서134개 검증을 새로 수행했다. 소유 복사본의30e43a34 +4Go파일 작업 트리 변경을1738native/5승인 입력과 결속했고 lint/docs 설정2개도 실행 전후 동일함을 확인했다. 최종 커밋 clean build로 이름을 바꿔 기록하지 않는다. [native 원자료](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/native/receipts.json)를 따른다.

실제 이전/새 CKS stdio 비교에서 선언 위치·심볼·보존 인용 확장3경로는 새 계측 on/off 결과가 경과 시간만 제외하고 일치하며, off의 이전/새 결과도 같다. 첫14회 시도의 주석 줄257 오류를 보존했고 선언 줄258·주석 줄을 분리한 이전16회/새16회를 추가했다. 총46호출·소유 세션6개 EOF0, 기존 source/graph/vector 전후 동일이다. 잘못된 commit·dataset·hash, 주석/없는 위치는 구조화 결과 없이 거부한다. 5000ms 상한의 기능 회귀이며 기본1초·warm 지연 수용이나 새 독립 표본으로 세지 않는다.

출처 결속 마감 완료: 기능·검증 근거를 서명 커밋 `d692db85`에 저장하고, 그 실제 커밋의 source-binding을 현재 소스 참조로 연결한 뒤 완료 주장 guard를 통과했다. 기존 작업 상태는 모두 보존했다. [결속 마감 기록](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1-finalization.json)을 따른다. native 빌드 모드는30e43a34 +4파일 작업 트리 검증이며, 그 전체 입력 hash가 현재 커밋과 같다는 뜻이다. 자동 label_suspects 계측 scope와 unanchored traversal 정규화는 이번에 추가하지 않았다.

다음: WI-3의 주석 포함 인용/정확한 선언 identity와 최종 검증 예산 계약. 남은 전체: WI-1의53발생 지점/3블록·타입/값/메모리 의미, WI-2의 독립 의미/build profile/지원/설치역할, WI-3 traversal/예산/startup/전체 호출 관측, WI-4/V1/V2 실제경로·오라클, WI-6 nonseed/쌍/warm, WI-SP overlay/ACL, WI-C 필수항목/전달, 새 독립 DEV/FINAL·완전 정답·매칭·충분성·F01~08, 정식9개, 온톨로지 효과 평가, 별도 선택 백로그12개다. 실제 LLM 답변 평가는 모델·인증·상한 결정 후 진행한다.

현재 Python 회귀96개(resolver14·공통 수용59·문서20·notice3), 문서134개·생성91파일 동일 재생·Python8파일 구문·공백 검사와 미승인 gold/미동결 실행 거부를 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi3-resolver-fix-v1/validation/SUMMARY.json)은 새 native4123/race4123·기존 skip18·실제46호출/EOF6과 커밋 후 출처 결속 필요를 명시한다.

이하 단계58 이전은 당시 기록이다.

# 단계58 — WI-1 select 수신 정의 보정 / 2026-10-19

[단계58 보고서](./operational-preparation/source-review-supplement/followups/wi1-select-review-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi1-select-review-v1/WORKLIST.json)이 최신 진입점이다. **AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처를 요구하지 않는다. 익명 역할 ID는 검토 여부와 근거를 구분하기 위한 선택지이며, 개인정보 제출 대기가 아니다.

원래30함수·1,654쌍·소스·과거 정답/실제 응답은 보존하고 별도 보완 reference만 생성했다. 고정 CFG가 select 진입의 전체 수신 대입과 선택 분기의 LHS를 모두 담아 원래 관측기가 정의 위치를 미확정으로 남겼다. 새 관측기는 해당 CommClause의 SelectCaseBody에 있는 정확한 AST identifier만 연결한다. 6함수의16정의 위치를 보정하여37쌍 기록이 달라졌다: 구조 unknown→도달32, 도달→비도달2, 도달 유지·이유 변경3. SRC-027의 이전 반복→다음 guard/log 연결2개는 새 수신 정의가 차단한다.

8개 완전한 합성 문맥은 타입 검사에 성공했고, 기존 변수 대입·선언·shadow·send/default·다중 case·closure·comma-ok를 구분했다. CFG에 없는 두 번째 수신 LHS는 unknown으로 남긴다. 실제 컴파일한 단일 합성 witness는 두 번의 수신 전 값3/8, 보정 후5/8을 관찰했다. 프로젝트 전체 타입·모든 스케줄·새 독립 표본의 증거로 확대하지 않는다.

원래30개 실제 MCP 응답을 **오프라인 재매칭**했을 때 구조 차이는0개다. 새 native 호출이나 엔진 수정은 없다. 발생 지점53개와 빈 블록3개는 여전히 매칭 미확정이고, 소비자의 일부 정의 위치가 수신 분기와 다르므로 매칭을 느슨하게 하지 않았다. 값 의미의 unknown1427/positive212/negative15는 그대로이며, 구조 관계만 positive1459/negative195다. 독립 정답·정확도 점수로 사용하지 않는다. 유지 Go 소스가 같아738c4a51의 최신 native 결속·일반/race 각4107통과/18skip을 재사용한다.

다음: WI-3 계측 래퍼의 선택적 CitationResolver 보존과 unanchored traversal 계약을 점검한다. 남은 전체: WI-1~3 의미/부분응답/실제설치경로, WI-4/V1/V2 실제경로·오라클, WI-6 nonseed/쌍/warm, WI-SP overlay/ACL, WI-C 필수항목/전달, 새 독립 DEV/FINAL·완전 정답·매칭·표본 충분성·F01~08, 정식9개, 온톨로지 효과 평가, 별도 선택 백로그12개다. 실제 LLM 답변 평가는 모델·인증·비용 상한 결정 후 진행한다.

현재 회귀91개(WI1 select10·공통 수용58·문서20·notice3), 문서134개·생성15파일 동일 재생·Python7파일 구문·보완 Go helper fmt·공백 검사와 미승인 gold/미동결 실행 거부를 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi1-select-review-v1/validation/SUMMARY.json)은 새 소비자 호출0과 기존 source/native 결과 재사용을 명시한다.

이하 단계57 이전은 당시 기록이다.

# 현재 인수인계

## 2026-10-19 단계57: WI2 호출 계약·별칭·opaque helper·index 범위 점검

[단계57 보고서](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/WORKLIST.json)이 최신 진입점이다. **AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처는 요구하지 않는다.

| 점검 범위 | 확인한 내용과 한계 |
|---|---|
| 차이6건 | 지역 selector 가림2·renamed import2·opaque helper2로 분류. v1은 명시적으로 syntax_only_unverified/not_proven/lexical_may_reach이며 실제 stdlib target/helper 효과 검증이 아님. 원래 의미 negative/positive/unknown과4반환 차이·2미반환 AI양성 유지 |
| 컴파일러 보완 | 원래6F 본문 bytes를 그대로 둔 전체 파일 문맥8개는 타입 검사 통과. 지역 method와 stdlib 별칭 확인. 두 opaque 사례는 F hash가 같아도 helper가 identity/constant일 수 있음을 별도 문맥으로 기록. 본문만으로 helper 효과를 확정하지 않음 |
| 원래 prefix | 선택6개의 기존 prefix는 standalone 프로그램으로 타입 검사 오류가 있음. 새 Fake methods/helper body/import-use prefix는 보완 증거이며 원래 입력이나 retained blob의 묵시적 대체가 아님 |
| 심볼9개/입력17개 | 기록된 graph manifest에 없는4파일: build/ci.go·build/update-license.go·crypto/signify/signify_fuzz.go·metrics/disk_linux.go. 현재16개 MatchFile 관측은 none/gofuzz tag·Linux 파일 선택과 연결. 당시 정확한 GOOS/GOARCH/tags는 미기록이라 배타적 역사 원인은unknown |
| 실제 MCP8회 | 동일 입력·pin·상한의 제어6개 후보 수1/1/0/1/1/0 유지. 과거 최종 출처 확인 deadline2개는 이번 동일1초 예산에서partial analysis/matched_bounded empty rules 반환. 과거 실패 보존·코드 수정/안전/독립 표본·점수로 승격하지 않음 |
| 보존·native | 원래108입력과 정답 분모 유지. 기존 project·controlled graph 전후 동일, 소유 stdio2개 EOF0. 관리 Go 코드 변경0이며738c4a51의1738native/5승인 입력 결속·일반/race4107검증 재사용. 외부/유료LLM·설치역할·운영/게시0 |

[컴파일러 문맥](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/compiler-scope-review.json), [manifest/profile 범위](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/index-scope-review.json), [실제 계약 관측](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/actual-contract-review.json)을 따른다. 현재 규칙팩의 계약 일치와 실제 의미 정확도를 분리한다. 새 pack·지원범위 확대·임계치 변경·정답 수정은 수행하지 않았다.

다음은 **WI1 select 수신 범위의 reference 반례·WI3 traversal/계측 optional resolver 검토**다. 남은 전체는 이 부채와 WI1 값종류/53occurrence/3block, WI2 독립 의미·미기록 build profile·지원 범위·설치 역할, WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 독립필수항목/실제전달, 새독립DEV/FINAL·완전gold·matching/충분성·F01~08, 정식9개·온톨로지 효과다. 실제 LLM 답변 효과는 모델/인증/비용 상한 확정 후다. 선택12개는 별도 backlog다.

이번 회귀111개(WI2 scope31·공통 수용57·문서20·notice3), 문서134개·생성39파일 동일 재생·Python8파일 구문·공백 검사와 미승인 gold/미동결 실행 거부를 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi2-contract-audit-v1/validation/SUMMARY.json)에 Go helper/입력/출력 hash·버전, 원래108입력·실패 보존과 새 독립 표본0을 기록한다. native/승인 입력 hash가738c4a51의 결속과 같아 새 전체 Go 실행은 하지 않았다.

이하 단계56 이전은 당시 기록이다.

## 2026-10-19 단계56: WI3 오류 사유 구분·새 native/실제 MCP 검증

[단계56 보고서](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1/WORKLIST.json)이 최신 진입점이다. **AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처는 요구하지 않으며, D04미정은 실제 검토·승인 대기다.

| 구현·검증 | 결과와 한계 |
|---|---|
| 실패 분류 | typed/wrapped 미해석 심볼은 error/seed_unresolved, 다른 불명 백엔드 실패는 backend_error, 예산 소진은 budget_exhausted. 원래 null·partial·출처·이후 성공에도 남는 실패 상태와 내부 오류 내용 비노출 유지 |
| 새 native 검증 | 전체 빌드·일반/race 각각4107pass/18skip/0fail, fmt·vet·고정 golangci-lint2.14.0(0issues)·경계·CLI 문서 통과. 이전에 통과했던4097개 테스트 누락없음. 이전 native 결과를 현재 결과로 바꾸지 않음 |
| 새 source/binary | 소유 복제본3869a548+정확히2개Go 변경, native/의존 입력1738(Go1282)·승인5 hash 결속. working-tree 빌드이며 현재 HEAD의 clean binary로 주장하지 않음. 시험 candidate와 전체 native에서 만든 binary bytes 동일 |
| 순차 MCP | native 프로세스 종료 후 원래20입력·검색1초 예산 그대로20partial. 후보68개의 출처 신원·순서는20건 모두 이전과 동일. 신호별51error는 이제seed_unresolved, 신호별9observed·8not_requested 유지. 의미 검색 품질 점수는null |
| 제어7건 | 미해석 단독·미해석/정상 양순서의 error/null/partial, 정상 단독의observed, 다른project·잘못된cap·stalecommit 거부를 실제 경로에서 확인 |
| 모든 시도 | 첫 initialize20초 제한은 검색0/미도달20/EOF0. 다음120초 초기 준비 한도 시도는 native 동시 실행 중20호출에서11tool_error/9partial. 순차 실행과 별도 보존하며 앞선 오류를 삭제하거나1초 예산을 늘리지 않음 |
| 모델·자원 | 실제 BGE-M3/native1024 digest와 Ollama0.40.4·graph/source/vector 전후 동일성·EOF0 확인. 초기화와 동시 부하의 배타적 원인·production SLA를 입증하지 않음. 시작 anchor embedding 물리 호출수미측정, 외부/유료LLM·설치역할·운영적용·게시0 |

[현재20건](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1/private/current-cases.json)과 원자료는 실제 관측만 기록한다. aggregate reason은 완전한 per-call 실패 이력이나 유일 원인 증명이 아니다. 기존20결과·독립 정답·표본을 변경하지 않았다. 새 전체 source binding을 커밋 후 별도 결속하여 완료 검사를 통과했다.

다음은 **WI2 syntax-only 호출 계약/alias·WI1 select 수신 범위의 reference 반례·WI3 traversal/계측 wrapper 경계 검토**다. 남은 전체는 이 부채와 값 종류/지원 밖/실제 설치역할·독립 의미, WI4/V1/V2 실제경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 독립필수항목/실제전달, 새독립DEV/FINAL·완전gold·matching/충분성·F01~08, 정식9개와 온톨로지 효과다. 실제 LLM 답변 효과는 모델/인증/비용 상한 확정 후다. 선택12개는 별도 backlog다.

추가·공통 회귀115개(오류 사유36·공통 수용56·문서20·notice3), 새 일반/race 각4107pass/18skip/0fail과 전체 빌드/fmt/vet/lint, 문서134개·원자료를 포함한 생성72파일 동일 재생·Python8파일 구문을 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1/validation/SUMMARY.json)은 초기화 실패·동시 실행20건·순차20건을 전부 유지하며 독립 점수를 만들지 않는다. 변경된 Go 소스의 완료 검사도 새 결속 파일 커밋 후 갱신했다.

새 native source 결속을 구현·증거 커밋738c4a51의 source-binding.json에 연결했고, 커밋된 자료를 읽는 완료 검사도 통과했다. [결속 완료 기록](./operational-preparation/source-review-supplement/followups/wi3-reason-fix-v1-finalization.json)에 서명 확인·1738개 입력·과거 binding 보존과 정식0/9 범위를 기록한다. 이 결속은 새 native 검증의 추적성이며 독립 의미 승인이나 전체 운영 수용이 아니다.

이하 단계55 이전은 당시 기록이다.

## 2026-10-19 단계55: WI1~3 검토 부채와 실제 graph provider 진단

[단계55 보고서](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/WORKLIST.json)이 최신 진입점이다. AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false다. 실명/연락처를 요구하지 않으며 검토 승인만 대기한다.

| 점검 대상 | 실제 확인과 남은 범위 |
|---|---|
| WI3 원래 결과 | 원래20건 모두partial·후보68개 보존. 17건의 시작 심볼26개가 미해석. 조합기는 하나라도 미해석이면 분석 대상3개 모두error로 두며 뒤의 성공이 그 오류를 지우지 않음 |
| 오류 분모 | impact51+concurrency51=102개는 공유되는 출력 label이며 독립 장애102회가 아님. 나머지 RQ-003/004/019는 신호별observed9개. 신호별8개는not_requested. 과거 각 호출의 typed error trace가 없어 배타적 원인/과거 장애0을 단정하지 않음 |
| 현재 실제 provider | 같은 원래binary/retained 데이터에서 관측된 서로 다른40심볼×2도구=80회 모두구조화 응답. 별도6회 탐색 중 미등록심볼2회 명시 거부. 소유 stdio2개 exit0,80회 실행 전후 graph/source/vector inventory 원래 자료와 동일 |
| 결과 보존 | 두 CKS의 시작 시 모델목록 신원 조회는 sandbox 거부로 실패했고 CKV는 degraded였다. 실제 graph 전용 검증이며 전체 retrieval/모델 준비 성공이 아니다. 새 semantic/embedding 추론·LLM·설치에이전트 실행0. 직접 호출 성공으로 원래partial/error를 성공으로 교체하지 않음. 범위 밖/매칭불가66개는unknown 유지. legacy 직접응답이 retained 출처/품질 정답을 대체하지 않음 |
| WI1 부채 | 58개: 암묵 occurrence53·도달불가 빈 block3·select loop 수명 lexical차이2. 값 종류·메모리/타입 의미·독립 검토는 미해결 |
| WI2 부채 | source unknown13·미확인심볼9·분석불가2·반환된 source차이4·미반환 AI양성2. stdlib이 아닌 local/import 대상에 대한 잘못된 매칭과 alias flow를 우선 수정하며 정답/매칭을 결과에 맞춰 완화하지 않음 |

[진단별 원래 사례](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/private/original-case-diagnostic.json), [현재 직접 호출](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/private/current-provider-diagnostic.json), [WI1~3 검토 부채](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/REVIEW-LEDGER.json)을 따른다. 모든 미확정 의미·독립 검토와 설치 역할은 미해결로 유지한다. native 코드 수정은 아직 하지 않았고 소스1738(Go1282)/승인5 동일성으로 이전 전체 native 검증을 재사용한다.

다음은 **WI3의 seed_unresolved·실제 backend_error·budget 구분 구현과 공유 시작 심볼 회귀, 새 native/MCP 검증**이다. 이후 WI2 호출대상/alias·WI1 select 범위/unknown·WI3 unanchored traversal·설치 역할/독립 의미, WI4/V1/V2 실제 경로/oracle·WI6 nonseed/쌍/warm·WI-SP overlay/ACL·WI-C 필수항목/실제전달, 새 독립 DEV/FINAL·완전gold·matching/충분성·F01~08, 정식9개와 온톨로지 효과가 남아 있다. 실제 LLM 답변 효과는 모델/인증/비용 상한 확정 후다. 선택12개는 별도 backlog다.

현재 회귀111개(진단33·공통 수용55·문서20·notice3), 문서134개·생성26파일 동일 재생·Python6파일 구문·공백 검사와 미승인 gold/미동결 실행 거부를 확인했다. 기존 원자료와 pinned source별 tool hash를 유지하고 과거 packet을 다시 seal하지 않았다. [검증 요약](./operational-preparation/source-review-supplement/followups/wi123-debt-diagnostic-v1/validation/SUMMARY.json)에 현재graph80회/탐색6회와 모델목록 조회 거부2회·CKV degraded 한계를 분리한다. native 구현은 아직 미수정이며 이전 전체 검증 재사용이다.

이하 단계54 이전은 당시 기록이다.

## 2026-10-19 단계54: WI-C DEV10·필수 항목/우선순위·현재 소비자 전달 계약 AI 초안

[단계54 보고서](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **요청한 AI 초안6/6 준비·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**다. 실명·연락처는 요구하지 않는다. 익명 역할 ID와 실제 검토/승인·근거 hash·독립성을 구분한다. D04미정은 개인정보 제출 대기가 아니라 검토·승인 대기다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| 기존 DEV·원문 | 노출된10함수/8모듈. 원래 Git/file/function/span과 소스 관찰45개+호출자/문맥19개를 확인 |
| 유한 AI 필수 목록 | 소스 관찰64개와 추가 확인 질문20개. 질문의 답은unknown이며 실제 측정/계약을 지어내지 않음. 선택 실험10개 미실행. 완전한 제품 요구·독립 gold가 아님 |
| 이름/우선순위 | 원래 getStartEnd 질문을 보존하며 실제 calculateBlockRange로 정규화. 요구·도달성·영향에 따른 조건부 검토만 작성, 확정 우선순위와 omission 점수null |
| 채택 소비자 대조 | coding-agent d1ebafb8의 Git/파일 proof와 채택 hash 확인. Analyzer5도구 노출·Planner high/medium/low 최대32는 이미 해결됐으며 과거 공백을 재등록하지 않음 |
| 보고서/그림 | typed JSON·근거 JSON·Markdown·Mermaid2개의 정확한5개 산출물. sequence는 지역 CFG flowchart, modules는 파일 소속이며 runtime trace/호출 그래프가 아님. 형식/바이트 검증은 렌더링·의미 품질 승인이 아님 |
| 인수인계/역할 | Analyzer→Planner→Orchestrator, 실제 CLI/hook/state/hash 전달 계약을 기록. 현재 소스 계약·합성 fixture 검증과 실제 설치된 모델 역할 실행을 구분 |

[AI 초안](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/private/AI-gold.json.gz), [현재 소비자 계약](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/consumer-contract.json), [남은 검토](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/REVIEW-LEDGER.json)을 따른다. 같은 기준 ID나6절·우선순위 enum만으로 필수 내용/중요도 정확성을 채점하지 않는다. structural snapshot은 identity_only_unverified_snapshot 한계를 유지한다. 실제 프로젝트 dual-tool·설치 역할·렌더링0, 품질 점수null이다.

다음은 **WI1~3 미확정 의미·partial·공급자 오류·범위 밖·설치 소비자 경로 부채 점검**이다. 남은 전체는 해당 부채, WI4/V1/V2 실제경로/oracle·WI6 nonseed gold/실제쌍/warm·WI-SP 독립 의미/실제overlay/ACL·WI-C 독립필수항목/우선순위/실제보고서/역할, 새 독립 DEV/FINAL·완전 gold·matching/충분성·F01~08, 정식9개(0/9), 온톨로지 효과다. 실제 LLM 답변 효과는 모델·인증·비용 상한 확정 후 수행한다. 선택12개는 별도 backlog다.

현재 회귀136개(WI-C37·공통 수용54·문서20·notice3·비교22), 3bdf444f 기준 과거 WI-SP36개, 문서134개·생성36파일 동일 재생·Python6파일 구문·공백 검사를 통과했다. 초기 검증 중 발생한 fixture cache와 변경 후 이전 tool hash 불일치를 보존하고, 소유 cache 제거·새 미동결 packet 결속 후 전체를 재검증했다. 정식 gold/미동결 실행 거부를 확인했다. [검증 요약](./operational-preparation/source-review-supplement/followups/wic-dev-gold-v1/validation/SUMMARY.json)에 범위를 기록한다. 소스1738(Go1282)/승인 입력5 동일성으로 이전 native 전체 검증을 재사용한다.

이하 단계53 이전 내용은 당시 기록이다.

## 2026-10-19 단계53: WI-SP DEV12·수동 overlay/출처·거부 경계 AI 초안

[단계53 보고서](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **D04미정·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

검토자 개인정보 정책: 실명·이메일·연락처는 요구하지 않는다. 새 승인 기록은 `reviewer_id`에 gold-reviewer / acceptance-reviewer 같은 익명 역할 ID를 사용할 수 있다. 기존 name 필드는 과거 식별자 형식의 호환성으로만 유지한다. 역할 ID는 실제 검토나 승인을 대신하지 않으며, 근거 hash·승인 결정·역할 분리/독립성만 별도로 검증한다. D04미정은 실명 제출 대기가 아니라 실제 검토·승인 상태를 뜻한다. [익명 역할 정책](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/reviewer-identity-policy.json)을 따른다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| 기존 DEV·원문 | 원래12심볼/5파일·함수 전체 body·Git/file/span 확인. 승인 평가용 policy42개와 기존 미적용 AI security proposal4개를 완전 목록으로 보존 |
| 유한 관계 AI 정답 | 12×46=552개 명시 selector 단위 전수 분류: 직접 지정9·지정없음543. 모든 semantic applicability는unknown. 문자열/본문 언급이 governs/matches 관계를 만들지 않음 |
| 두 overlay 조건 | policy-only 예상5관계/상대 미태그8심볼. security제안까지 정확히 별도 ingest한 조건의 예상9관계/상대 미태그4심볼. 현재 retained 응답·security ingest 결과가 아님 |
| typed/legacy·출처 | 실제 enum Policy/SecurityPattern과 typed operator description/hint/severity·known 빈 문자열을 유지. legacy 혼합 설명에서 severity/hint 추측금지. 원래 YAML hash/row/version/reviewer/time/approval는 runtime unknown |
| 실제 경계 공백 | query wire에 project/dataset 명시필드가 없어 실제 routed store/session/canonical·source blob/commit/snapshot 결속 필요. 전체 파일 SHA를 retained 함수 blob SHA로 넣지 않음. YAML 작성 근거/graph metadata SHA는 검토자 인증이 아님 |
| 거부 제안16개 | strict decode/selector/cap·pin·private·위조관계·snapshot_changed·peer ACL/forwarded spoof·빈/미발견/부분/자동출처·다른 저장소를 분리. ACL은 per-symbol/project 권한이나 검토자 인증을 증명하지 않음 |

[정답 초안](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/private/AI-gold.json.gz), [전체 overlay·YAML 근거](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/private/overlay-catalog.json), [거부/경계 제안](./operational-preparation/source-review-supplement/followups/wisp-dev-gold-v1/private/controls.json)을 따른다. 수동low는 운영자 검토 제안값이며 취약점 판정/정책 준수 정답이 아니다. no_annotations는 알려진 심볼/완전 페이지의 선택 overlay 범위이며 not_found/unavailable/unsupported/partial과 다르고 안전을 뜻하지 않는다. actual automatic rule query는 별도 opt-in이다.

새 graph ingest·수동/자동 MCP·listener·모델/대상 실행0·품질 점수null이다. 동일 정책/파일/계약의12심볼을 독립12가족으로 승인하지 않는다. 100% annotation/refusal 기준은 실제 경로·독립 정답·분모 동결 후 평가한다. 소스1738(Go1282)/승인 입력5 동일성으로 이전 전체 Go 검증을 재사용한다. 단계52 자료는fc4186b9에서 원형 보존한다.

다음은 **WI-C 필수/선택·우선순위·보고서/그림·인수인계/설치 역할 AI 초안**이다. 남은 AI1개는 WI-C다. 남은 전체는 WI-C, WI1~3 부채와 WI4/V1/V2 경로/oracle·WI6 nonseed gold/쌍/warm·WI-SP 의미/ingest/store-session/수동자동/ACL/역할, D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08, 정식9개(0/9), 온톨로지 독립 효과다. 선택12개는 별도 backlog다.

현재 회귀159개(WI-SP36·공통 수용53·익명 검토자 GoldAndSplit25·문서20·notice3·비교22),fc4186b9 원래 소스의 과거 회귀35개, 문서134개·정책/전체 보고서를 포함한 생성24파일 동일 재생·Python compile/공백 검사를 통과했다. 실명 없이 reviewer_id로 형식 검증하되 실제 승인/hash/독립성을 요구하며, 미승인 gold·미동결 실행은 거부한다. 원문·데이터·운영 설정 보존, 소스1738(Go1282)/승인 입력5 동일성으로 이전 native 전체 검증을 재사용한다.

이하 단계52 이전 항목은 당시의 기록이다.

## 2026-10-19 단계52: WI-6 DEV20·관련성/잡음 AI 초안·시작 함수 제외 매칭

[단계52 보고서](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **D04미정·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| 기존 DEV | 원래20질문/선택20함수/14파일·4intent 유지. 운영 source 후보19개+개발 stress source-only1개. stress main·노드/채굴/거래 실행 금지 |
| 완전 범위 AI 정답 | 400질문/함수 단위: relevant25(시작함수20+명시적 문맥5)·local noise360·unknown15. 원래 질문의 지역 구현/명시 source 문맥 범위이며 전체 저장소·전이/runtime 관련성 정답이 아님 |
| 명시 문맥 | Prune↔RecoverPruning·Init↔newLimbo의 실제 caller/callee와 WriteBadBlock→ReadAllBadBlocks의 저장 데이터 소비 근거를 구분. 동일파일/intent/topic만으로 관련성을 추가하지 않음 |
| seed·분모 | 두 arm에서 같은 시작함수를 primary 점수 universe에서 제외하고 포함 민감도는 별도 보고하는 제안. nonseed 관련 정답이 있는5개와 없는15개 구분. 빈 정답/빈 반환은 recall0/1·개선으로 채우지 않고 undefined/inconclusive |
| 출처·매칭 | 원래 Git0bf2f4d1의14파일 commit/tree/blob과 전체 함수/body·40anchor 해시 검증. project/dataset/commit/source mode/snapshot·정확한 함수/witness 매칭 제안. 중복·unknown·범위밖 자리를 보존하며 상한 뒤 후보로 보충하지 않음 |
| 쌍·warm 제안 | 같은 retained seed/snapshot과 arm별 상한/순서·전수 실패/partial 슬롯 필요. 새 미동결 제안: warmup3쌍+측정30쌍/운영 case, 단일 동시성·AB/BA 교대·같은 소유 stdio 재사용, client end-to-end monotonic 시간 |
| 지연 의미 | 각 arm nearest-rank p95의 비율. paired ratio들의 p95와 구분. baseline비양수/부족/환경변경은 inconclusive. 다른 평가 warm20을 자동 적용하지 않음. 실제 paired/warm 원자료는 아직0 |

[정답 초안](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/private/AI-gold.json.gz)과 [함수·근거 목록](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/private/location-catalog.json), [검토 원장](./operational-preparation/source-review-supplement/followups/wi6-dev-gold-v1/REVIEW-LEDGER.json)을 따른다. seed만 정답에 포함된15개는 더 넓은 nonseed helper·독립 의미 정답이 없으면 확장 효과를 판정할 수 없다. unknown/범위밖을 잡음으로 계산하지 않는다.

새 MCP/모델/대상/노드 실행0·점수null이며 실제 retained seed와 두 요청·전체 설치 역할은 미검증이다. 코드·관계 확장을 argument-return 전이/curated join이나 incoming call 확장으로 일반화하지 않는다. 소스1738개(Go1282개)·승인 입력5개 동일성으로 이전 전체 Go 검증을 재사용한다. 단계51 자료는875cfc29에서 원형 보존한다.

다음은 **WI-SP 수동/자동 policy/security·미태그·거부 경계의 AI 초안**이다. 남은 AI2개는 WI-SP/WI-C다. 남은 전체는 이2개, WI1~3 검토 부채와 WI4/V1/V2 실제 경로·oracle/WI6 nonseed gold·실제쌍·warm, D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08, 정식9개(0/9), 온톨로지 독립 효과다. 선택12개는 별도 backlog로 유지한다.

현재 회귀129개(WI6 35·공통 수용49·문서20·notice3·비교22),875cfc29 원래 소스의 과거 회귀41개, 문서134개·생성45파일 동일 재생·원래 Git14파일/40anchor·Python compile 검사를 통과했다. 원문 signed Git commit의 GPG folded blank header에 대해 최초 staged 공백 검사exit2를 기록하고, 그 원본 commit artifact 한 경로에만 -whitespace를 적용해 재검사exit0으로 확인했다. 원문 바이트와 코드 fmt 규칙은 보존하며, 미승인 gold·미동결 실행은 거부한다. 소스1738(Go1282)/승인 입력5 동일성으로 이전 native 전체 검증을 재사용한다.

이하 단계51 이전 항목은 당시의 기록이다.

## 2026-10-19 단계51: WI-V1/WI-V2 공유 DEV 티켓·oracle·후보 AI 초안

[단계51 보고서](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **D04미정·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| 공유 DEV | 이미 노출된 티켓1개를 V1/V2가 공유. 스케줄5개·후보/대조군 역할5개이며 실제 method 바이트는4종. ORIGINAL=REINTRO를 독립 결함/표본으로 세지 않음 |
| 완전 범위 AI 대조 | 요구사항12개 × 후보/스케줄25조합=300단위 전수 분류. conditional source fit89·source conflict26·runtime unknown125·not applicable60. 실제 API/실행 관측 결과가 아님 |
| 후보 정답 초안 | 명시한 유한 메서드 조건에서 A/B 적합, 원본/재도입/조기반환 부적합. 실제 API·제품 수용 membership는5개 모두unknown, 독립 acceptable set은null·선택 후보없음 |
| 실제 경로·원문 | 원래 Git5파일·API anchor13개, source/file/method witness·제안 전체 파일 overlay/diff SHA 작성. 실제 적용 commit/tree/binary/driver는 미정 |
| oracle 의미 | 완전한 통제 event trace를 대상으로 삭제→ack→반환·요청1개·반환후 channel 미사용·정상알림·소유정리를 확인하는 초안. RPC eth_unsubscribe true는 내부 ack/반환 대체 불가 |
| 독립성·선택 경계 | 별도 wait graph도 같은 AI/가설이며 독립 executable oracle이 아님. 실제API·정답/후보 독립 검토·필수 gate unknown은DEFERRED. all-bad 무선택·tie 허용·성능/공정성 점수 미생성 |

[요구사항과 원문](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/private/requirements.json), [25조합 정답 초안](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/private/AI-matrix.json), [후보 집합 초안](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1/private/AI-membership.json)을 검토할 수 있다. 초안의 conditional_source_fit은 코드/유한 전제에 대한 AI 판정이며 실제 callback 도달성·종료 공정성·완전 제품 계약·독립 승인에 해당하지 않는다. 미도달/도구/timeout/오류는 symptom RED나 정상 GREEN으로 바꾸지 않는다.

새 API/대상/MCP/모델/노드 실행0, 품질 점수null이다. 소스1738개(Go1282개)·승인 입력5개 동일성으로 이전 전체 Go 검증을 재사용한다. 단계50 packet과 도구는20f470f2에서 원형 보존한다.

다음은 **WI-6 기본/확장 DEV 쌍과 범위 내 완전 AI 관련성·잡음 정답**이다. 남은 AI3개는 WI-6/WI-SP/WI-C다. 남은 전체 필수 범위는 이3개, WI1~3 검토 부채·WI4/V1/V2 실제 경로/독립 oracle, D04·새 DEV/FINAL·완전 독립 gold·matching/충분성·F01~08, 정식9개(0/9), 온톨로지 독립 효과다. 별도 선택12개는 기존 backlog로 유지한다.

현재 회귀134개(공유 티켓41·공통 수용48·문서20·notice3·비교22),20f470f2 원래 소스의 과거 회귀43개, 문서134개·생성23파일 동일 재생·Python compile/공백 검사를 통과했다. 미승인 gold·미동결 실행은 예상대로 거부한다. 소스1738(Go1282)/승인 입력5 동일성으로 이전 native 전체 검증을 재사용하며 새 Go 전체 시험으로 기록하지 않는다.

검사 기록 보완: 최초 staged 공백 검사에서 원문 unified diff의 문맥 접두사(공백+Go tab/빈 문맥행) 때문에 exit2가 발생했다. 결과 확인 전에 커밋한 실행 순서 오류를 기록하며, 원문 packet 33파일은 3caf64d6과 바이트 동일하게 보존했다. 후속 .gitattributes는 해당 diff 자료 경로에만 -whitespace를 적용한다. git show --check 재검증은exit0이며 코드의 fmt 규칙에는 예외를 추가하지 않는다. [검사 보완 기록](./operational-preparation/source-review-supplement/followups/ticket-dev-gold-v1-format-review.json)을 따른다.

이하 단계50 이전 항목은 당시의 기록이다.

## 2026-10-19 단계50: WI-4 DEV21·조건부 결과 AI 초안

[단계50 보고서](./operational-preparation/source-review-supplement/followups/wi4-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi4-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. 사용자가 요청한 AI 초안6개·WI1~3 부채·독립 수용 준비·정식9개·온톨로지 효과를 순서대로 진행한다. **D04미정·독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| WI-4 DEV | 기존5개 메커니즘 제안·21스케줄의 순서를 유지. 실제 진입·채널/버퍼·잠금·동시 준비·원문 버전을 전제조건으로 보강 |
| 결과 AI 초안 | 명시한 유한 vocabulary150단위 전수 분류: legal22·조건부 symptom_candidate11·unknown33·infra/unreached84. 각 결과의 source span·hash·사유와 빈 집합 사유 작성. 모든 실행 trace/제품 계약의 완전 gold는 아님 |
| 두 의미 축 | Go/원문에서 허용되는 동작과 제품 계약 위반을 구분. WORK-both의 send/exit 모두 합법, backlog의 즉시 종료·선택 빈도·공정성을 가정하지 않음 |
| 미확정/경계 | UDP 합성 deadline과 실제 monotonic 경로·타이머 firing을 분리. Gossip 재시도 억제를 결함으로 단정하지 않음. pool은 실제 adds/구독·버퍼/Stats 순서를 요구. 구독 ack·반환·구조 수신의 관측 순서 구분 |
| 매칭·분모 | 정확한 family/schedule/source·binary/driver/environment·seed/arm/order·진입 trace 요구. 결과 ID만으로 실행·독립 oracle 승인 불가. 계획·증상·unknown·오류·미도달·누락·cleanup 오류 보존 |
| 동결 전 부채 | 반복21개가 독립21가족이 아님. 독립5가족·arm당 계획100·baseline증상10·비율0.50·family paired 불확실성·600초 관찰·설치 역할은 미동결/미실행. 무재발·baseline0은 inconclusive |

unknown을 기존3집합 formal 작성 양식의 legal/infra에 숨겨 넣지 않고 **별도 네 번째 AI 집합**으로 유지한다. 정식 양식·제품 계약의 독립 검토가 남는다. [검토 원장](./operational-preparation/source-review-supplement/followups/wi4-dev-gold-v1/REVIEW-LEDGER.json)에 실제 도달성·독립 oracle·동결 arm/오라클·정리/모드9·instrumentation·설치 역할 부채를 기록한다. 원래 Git chain과 파일/좌표/hash, 과거 go1.26.9 stdlib 원문을 검증하며 대상 toolchain 채택으로 승격하지 않는다.

새 대상 실행·MCP·모델·노드·품질 점수는0이다. 유지보수 소스1738개(Go1282개)·승인 입력5개의 동일성을 확인해 이전 전체 native 검증을 재사용하며 새 Go 전체 시험으로 기록하지 않는다. 이전 단계49 증거/도구는d95b3e91 소스에 결속해 보존한다.

다음은 **WI-V1/WI-V2 공유 DEV 티켓의 완전 AI oracle·수용 가능 후보 집합**이다. 남은 AI5개는 WI-V1/V2/6/SP/C다. WI1~3 부채와 WI4 실제 도달성/독립 oracle·수용 준비(D04·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08)·정식9개(0/9)·필수 온톨로지 독립 효과는 미완료다. 별도 선택 후속12개는 기존 backlog이며 이번 요청의 필수 범위와 구분한다.

현재 회귀135개(WI4 43·공통 수용47·문서20·notice3·비교22), d95b3e91 원래 소스의 과거 회귀37개, 문서134개·생성30파일 동일 재생·Python compile/공백 검사를 통과했다. 미승인 gold·미동결 실행은 예상대로 거부한다. 결속 소스1738개(Go1282개)·승인 입력5개 동일성으로 이전 native 전체 검증을 재사용하며 새 전체 Go 시험으로 기록하지 않는다.

이하 단계49 이전 항목은 당시의 기록이다.

## 2026-10-19 단계49: WI-3 source unknown·지원 문맥·실제 증상 검색 매칭

D04는 **미정**이다. [단계49 보고서](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/WORKLIST.json)이 최신 진입점이다. **독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다. 단계48 원본400단위·도구는8553fe97 근거로 그대로 보존한다.

| 점검·검증 범위 | 관찰과 한계 |
|---|---|
| source unknown | 원래400단위 유지. runReorg의 명시적 예약→승격→알림 연결을 근거로 RQ-007/008 상호2조합만 별도 AI 지원 문맥으로 보완. 새 관련25·비관련364·unknown11이며 runtime 원인/독립 승인이 아님 |
| 실제 증상 입력 | 원래20문안만 실제 CKS stdio로 순서대로 요청. symbol/file/expand 정답 힌트 없음.1000ms·후보10·seed3·enriched3·call32·depth2 유지, primary 재시도0 |
| 실제 응답 |20개 모두 partial. 후보68개·출처657곳을 보존. symbol_identity_unavailable·unanchored_edge와 일부 call/edge/candidate/seed 상한, provider error·unsupported를 숨기거나 성공 응답으로 변경하지 않음 |
| 원문·프로젝트 경계 | 후보/중첩 신호657곳의 legacy/V2 일치, project/dataset/source snapshot/commit/mode/origin·파일/내용 SHA 확인.46파일을 retained archive에서 보존하고 원래 Git0bf2f4d1 원문과 대조 |
| 엄격/보완 매칭 | 단계48 엄격 제안 매칭0은 그대로 기록. 문서 주석이 붙은 전체 함수2범위를 Go AST의 Doc+FuncDecl 소유·좌표·해시로 확인해 새 보완 제안에서2개 연결. 임의 포함/이름/canonical 단독 매칭은 불허 |
| 범위 밖·분모 | 나머지66개는 선택20함수 범위 밖·미라벨 unknown. 원래 후보 자리/순서 보존, 중복 제거 후11위 보충 없음. 실제 검색 출력으로 의미 gold를 만들거나 precision/recall을 계산하지 않음 |
| 모델·거부·종료 | 로컬 BGE-M3 기존 digest/native1024·Ollama0.40.4 확인. 잘못된 commit·후보 상한21·다른 프로젝트 확장 citation을 실제 거부. EOF0·retained 전후 hash 동일 |
| 실행 수용 경계 | 실제 CKS 연결·출처·매칭 관찰이며 설치 agent의 조사·원인 판정/독립 수용이 아님. 유료/외부 모델·설치 역할 실행0. 같은20문항의 반복 자료를 독립 가족으로 세지 않음 |

[검토 부채 원장](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/REVIEW-LEDGER.json)에 source unknown11개, 모든 partial/stop reason, 범위 밖66자리와 provider·설치 역할·독립 의미/gold/평가 계약의 부채를 유지한다. 문서 주석 보완은 **관찰 후 제안한 DEV adapter**다. 기존 엄격 계약의 결과를 소급해 변경하지 않으며 정식 matching 승인은 미정이다. 직접 동작20개와 지원 문맥5개의 AI 관련성은 실제 원인이나 의미 정답 승인 수가 아니다.

유지보수 Go1738개·승인 입력5개의 동일성을 확인해 이전 native 전체 검증을 재사용한다. 별도 AST 관측기의 build/vet/fmt는 새로 검증하며 전체 Go 시험을 새로 수행한 것으로 기록하지 않는다. 운영 current·전역 설정·원문 DB·외부 게시를 변경하지 않았다.

다음은 **WI-4 DEV 입력과 범위 내 완전한 AI 결과 정답 초안**이다. 남은 AI 작성6개는 WI-4/V1/V2/6/SP/C다. 남은 전체는 이6개 작성과 WI1·WI2·WI3 검토 부채, D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08 동결, 정식9개 수용(0/9), 필수 온톨로지 독립 효과, 별도 선택 후속12개다. [전체 작업표](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/WORKLIST.json)에 모두 미완료로 유지한다.

현재 회귀128개(WI3 37·공통 수용46·문서20·notice3·비교22), 원래8553fe97 소스의 과거 회귀34개, 문서134개·생성31파일 동일 재생·Git46파일 대조·출처657곳 검증을 통과했다. 보조 AST 관측기 build/vet/fmt 통과, 미승인 gold·미동결 실행은 예상대로 거부했다. [검증 기록](./operational-preparation/source-review-supplement/followups/wi3-consumer-matching-v1/validation-summary.json)을 따른다.

이하 단계48 이전 항목은 당시의 기록이다.

## 2026-10-19 단계48: WI-3 DEV20·선택 함수 범위 내 완전한 AI 정답 초안

D04는 **미정**이다. [단계48 보고서](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. **독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지하며 단계47 근거와 도구는5b72156d의 과거 검증으로 보존한다.

| 작성·검증 범위 | 결과와 한계 |
|---|---|
| DEV 입력 | 기존 증상 전용20문안·7유형 유지. 공개 요청에는 정답 위치·함수명·가족/정답 경로를 넣지 않음 |
| 완전 universe | 선택20함수/17파일 ×20질문, 총400단위 전수 AI 분류. 관련23·비관련364·unknown13. 전체 저장소의 완전 정답은 아님 |
| 의미 근거 |20함수 전체 body를 검토하고 원래40개 소스 사실 근거·새 정확한 witness 좌표·해시·개별 판정 사유를 결속. 직접 동작20개와 지원 문맥3개를 구분 |
| unknown | 같은 가족/파일/용어를 근거로 관련 위치를 자동 승인하지 않음. 전이·comparator·caller/consumer·runtime 계약이 미확정인13단위는 유지 |
| 정답 내용 | 질문별20위치의 관련/비관련/unknown 집합과 빈 집합 사유, 원문 commit/file/function SHA·UTF8 위치, 범위 밖 unknown 정책을 작성 |
| 가족·노출 | 기존15그룹 제안·190쌍 중복·840건 과거 질문 대조 유지.20카드는 독립20가족이 아니며 노출된 DEV. 독립 가족 승인0·새 FINAL0 |
| 매칭 제안 | project/dataset/commit/source mode/graph snapshot과 파일·함수 SHA, 정확한 전체 함수 또는 명시 witness span을 요구. 이름/canonical 단독·임의 겹침은 unknown |
| top-10 제안 | label_suspects 최종 후보의 원래 앞10자리를 사용, 확인된 같은 함수만 중복 제거하고 뒤 순위로 보충하지 않음. unknown/오류/범위 밖 자리와 원래 계획 분모 유지 |
| 실행 경계 | 실제 증상 검색·새 MCP·모델·설치 역할 실행0, 품질 점수null. 과거 pilot pin과 wire 형식은 새 검색/정식 baseline 승인 근거가 아님 |

[검토 가능한 각 질문의 정답](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/private/RQ-001-REVIEW.txt)과 [전수 AI 정답](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/private/AI-gold.json)을 작성했다. 비관련은 이20함수의 명시적 local 동작 검토 범위이며 안전/무관한 전체 저장소를 증명하지 않는다. 범위 밖 위치는 미라벨·unknown으로 유지한다. 입력·정답의 경로 분리는 OS 격리나 봉인된 FINAL이 아니므로 실제 평가 host에는 public만 제공해야 한다.

실제 retained 검색 응답을 검증한 adapter·표본 충분성·독립 의미 gold·원인 계약·case/family macro·baseline/빈 분모·F01~08 동결은 남아 있다. 이번 완료는 bounded AI DEV 작성과 오프라인 매칭 제안의 검증이다. 유지보수 Go1738개·승인 입력5개의 동일성을 확인해 이전 native 전체 검증을 재사용하며 새 전체 Go 시험으로 기록하지 않는다.

다음은 **WI-3 unknown·지원 문맥·실제 top-10 소비자 매칭 보완 후 WI-4 DEV/AI 초안**이다. 남은 AI 작성6개는 WI-4/V1/V2/6/SP/C다. 남은 전체는 위WI3 보완과6개 작성, WI1·WI2 검토 부채, D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08 동결, 정식9개 수용(0/9), 필수 온톨로지 독립 효과, 별도 선택 후속12개다. [전체 작업표](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/WORKLIST.json)에 미완료로 유지한다.

현재 회귀124개(WI3 34·공통 수용45·문서20·notice3·비교22), 원래5b72156d 소스의 과거 회귀38개, 문서134개·생성51파일 동일 재생·원래 Git17파일 대조를 통과했다. 미승인 gold·미동결 실행은 예상대로 거부했다. [검증 기록](./operational-preparation/source-review-supplement/followups/wi3-dev-gold-v1/validation-summary.json)을 따른다.

이하 단계47 이전 항목은 당시의 기록이다.

## 2026-10-18 단계47: WI-2 unknown·호출 문맥·실제 소비자 매칭 보완

D04는 **미정**이다. [단계47 보고서](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/WORKLIST.json)이 최신 진입점이다. 단계46 원본108개 AI 초안과 당시 도구를0993202d 소스에 결속해 보존했다. **독립 승인0·FINAL0·정식0/9·freeze null·execution_allowed=false**를 유지한다.

| 보완·검증 항목 | 결과와 한계 |
|---|---|
| unknown 의미 | 원래538단위를 보존. 읽기 전용 scalar 매개변수 캡처를 쓰기·주소 노출과 분리해 CALL-046의 cmd/args 2단위만 보완. 새 AI 분류 positive20·negative505·unknown13. 기존15 unknown은 원본에 유지 |
| 호출 대상 | 선언 import·동명 객체·별칭·정확한 result/binding을 구분. 패키지 전체 타입 확인·링크·공격자 제어·실행 가능성을 증명한 것이 아님 |
| 호출자14개 | 원래 Git 파일/호출 좌표·해시·import/가림을 확인. literal13개·identifier1개를 해당 site의 문맥으로만 기록. wrapper 전체 안전·전이 flow·독립 표본으로 승격하지 않음 |
| 실제 소비자 | 프로젝트51함수를 탐색해42개 실제 CKS rule 조회, 통제40개도 CKS stdio 조회. 원래68호출 질문은 함수 응답을 중복하지 않고 source-result/sink 단위로 연결 |
| 미도달·상한 | 심볼 미도달9함수/20단위, primary budget_exhausted2함수/16단위는 그대로 unknown. 나머지80함수의 규칙 계산은 matched_bounded이며 기본 분석 partial 의미 한계와 구분 |
| 실제 매칭 | 반환22후보 전부 선언 단위에 연결, 미연결0. 소스 AI 초안 기준 positive18·negative2·unknown2; negative/unknown4개와 미반환 alias positive2개는 검토 원장에 보존. 품질 점수 아님 |
| 조건·좌표 | 실제 조건8개를 원문 노드·CFG 분기 ordinal로 연결. 형제 클로저는 깊이가 같아도 owner 좌표로 분리. 통제 원문 조각은 유지하며 같은 길이의 고유 package prefix와 원래/실제 파일 SHA를 명시 |
| 경계·보존 | 두 CKS 경로 각각3종 잘못된 pin·pack/상한 오류·incomplete 전달 검증. 최종4세션 EOF0, 원래 retained graph/sources/vector 전후 hash 동일. 초기 harness 오류·별도42조회도 진단 근거로 보존 |
| 소스 통제 | 읽기/쓰기·주소 노출·참조형·가림·형제 클로저·한글 식별자7개. 실제 보조 관측기 build/vet/fmt 검증. 불투명 helper·간접/복합 sink·bare return·클로저 실행은 unknown 유지 |

[검토 부채 원장](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/REVIEW-LEDGER.json)은 소스 unknown13개, 미도달9함수, 상한2함수, 반환 후보의 소스 초안 차이4개와 미반환 positive2개를 별도 축으로 기록한다. 중복되는 축을 독립 사례 수로 합산하지 않는다. 빈 후보는 안전이나 음성 정답이 아니며, 실제 소비자 출력을 소스 의미 정답으로 복사하지 않았다. 정답 초안과 공통 x/tools CFG의 한계, caller 전이·대상 정체·조건 의미의 독립 검토가 남는다.

이번 완료 범위는 **AI DEV 보완과 실제 소비자 매칭 관찰**이다. 정식 gold 사용은 거부한다. 유지보수 Go1738개·승인 입력5개의 동일성을 확인해 이전 전체 native build/일반/race/fmt/vet/lint 근거를 재사용하며, 새 전체 native 시험으로 기록하지 않는다. 운영 current·전역 설정·외부 게시를 변경하지 않았다.

다음은 **WI-3 DEV 입력과 범위 내 완전한 AI 정답 초안**이다. 남은 AI 작성7개는 WI-3/4/V1/V2/6/SP/C다. 남은 전체는 이7개 작성, WI1·WI2 검토 부채와 D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08 동결, 정식9개 수용(0/9), 필수 온톨로지 독립 효과 평가, 선택 후속12개다. [전체 작업표](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/WORKLIST.json)에 모두 미완료로 유지한다.

현재 범위 회귀127개(WI2 38·공통 수용44·문서20·notice3·비교22), 원래0993202d 소스의 과거 회귀33개, 소스 통제7개, 문서134개, 생성73파일 동일 재생·원래 Git34파일 대조를 통과했다. 미승인 gold·미동결 실행은 예상대로 거부한다. 상세 검증 기록은 [validation-summary](./operational-preparation/source-review-supplement/followups/wi2-consumer-matching-v1/validation-summary.json)에 유지한다.

이하 단계46 이전 항목은 당시의 기록이다.

## 2026-10-18 단계46: WI-2 DEV 입력과 범위 내 완전한 AI 정답 초안

D04는 **미정**이다. [단계46 보고서](./operational-preparation/source-review-supplement/followups/wi2-dev-gold-v1/REPORT.txt)와 [현재 단계·다음·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi2-dev-gold-v1/WORKLIST.json)이 최신 진입점이다. DEV 초안의 작성·기계 검증은 독립 승인·정식 수용과 구분하며 **정식0/9·freeze null·execution_allowed=false**를 유지한다. 단계45 자료와 도구는5d84449e 소스에 결속해 과거 근거로 보존한다.

| 항목 | 이번 결과와 범위 |
|---|---|
| 통제 DEV | 원래 함수 조각40개 바이트 보존, 별도의 명시적 import/type/bodyless helper 문맥 가정.40개/49단위, AI positive18·negative23·unknown8 |
| 프로젝트 DEV | 원래68호출/51함수/34파일, commit0bf2f4d1·file/function/call SHA·UTF8 좌표에 결속.68개/489단위, AI positive1·negative481·unknown7 |
| 완전 universe | 명령 이름 slot × 모든 named parameter 또는 no-source sentinel; 변환 호출 × 같은 closure 깊이의 모든 반환 slot·bare/void/no-return sentinel.538단위 전부 분류 |
| 정답 내용 | rule/source/sink/binding·callee/import/shadow·RHS/CFG 의존 witness·조건 ordinal·구체 사유, 전수 answer sets와 빈 집합의 AI 사유 |
| 반환 identity | 하나의 bare return이 여러 named result를 반환하면 같은 좌표라도 sink binding으로 분리.여러 결과를 하나로 접지 않음 |
| 문맥/한계 | alias·동명 객체·opaque helper·복합/간접 sink·closure·주소/range·부분 타입을 별도로 기록.소비자 계약 분류와 소스 검토 분류를 분리 |
| 별도 호출자 | 기존14개 참조를 source-only 문맥으로 유지.108입력이나 독립 표본에 합산하지 않고 runtime/attacker control은 미확정 |
| 승인/실행 | source-only AI 제안; candidate engine·새 MCP·외부 모델 호출0.독립 gold0·FINAL0·실제 retained 소비자 매칭/동결 미실행 |

negative는 **선언한 규칙의 source–sink 단위**에서 조건이 없다는 의미다. 변환 오류 변수가 `_`가 아니어도 오류를 검사했다는 뜻이 아니며, 다른 명령 인자·다른 선언·다른 반환 slot을 합치지 않는다. positive도 lexical may-reach 후보 검토 조건이며 취약성·공격자 제어·실행 가능성을 증명하지 않는다. unknown을 빈 탐지/음성 정답으로 바꾸지 않았다. 통제용 prefix는 원본에 없던 **AI DEV 문맥 가정**이며 실제 retained 좌표 adapter·대상 정체의 독립 검토가 남는다.

공개 DEV 입력과 비공개 정답 경로를 분리했다. 이 packet 전체를 평가 host에 제공하지 않으며 Git에 노출된 자료를 새 FINAL로 재명명하지 않는다. source AST/부분 go-types/CFG 참조 계산은 후보 분석기의 출력으로 정답을 복사하지 않는다. 동일 x/tools CFG 구성기를 공유하는 공통 오류 한계가 있다. `check-wi2-dev-gold.py --require-formal-gold`는 정식 gold 사용을 거부한다. 전체 native Go 검증은 소스 결속을 확인해 이전 근거를 재사용하며, 보조 관측기 검증과 구분한다.

다음은 **WI-2 unknown·callee/호출자 문맥·실제 소비자 매칭 검토 보완 후 WI-3 DEV/AI 초안**이다. 남은 AI 작성7개는 WI-3/4/V1/V2/6/SP/C다. WI1 검토 부채, D04 역할/독립성·새 DEV/FINAL 가족·완전 독립 gold·matching/충분성·F01~08 동결, 정식9개 수용, 필수 온톨로지 독립 효과, 선택 후속12개는 [전체 작업표](./operational-preparation/source-review-supplement/followups/wi2-dev-gold-v1/WORKLIST.json)에 미완료로 유지한다.


## 2026-10-18 단계45: WI-1 unknown 의미·값 종류·합성 좌표·실제 소비자 매칭

D04는 **미정**이며 독립 승인0·정식 수용0/9·freeze null·execution_allowed=false를 유지한다. [보완 보고서](./operational-preparation/source-review-supplement/followups/wi1-consumer-matching-v1/REPORT.txt)와 [현재 단계·남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi1-consumer-matching-v1/WORKLIST.json)이 최신 진입점이다. 단계44 원본 초안과 manifest는 그대로 보존하고 당시 검사는4529b6d1 소스에 결속한다.

| 보완 항목 | 검증 결과와 한계 |
|---|---|
| unknown 의미 | lexical 경로 유무, 값 종류, 구체 값, memory/alias/range/capture, 좌표 매칭, 분석기의 partial 상태를 분리. unknown을 음성·빈 집합·안전으로 바꾸지 않음 |
| 값 종류 | 같은 선언의 부분 파일/고립 함수 타입 문자열·오류·분류를 대조;412선언 중73개 분류 차이. 모든 구체 실행 값은 unknown |
| 보수적 의미 초안 | 기존 unknown1004는 원본 보존. 새 값 의미 축은1654쌍 중unknown1427·positive212·negative15. reference/aggregate는 내용·별칭 증명이 아니므로 구조 경로와 구분 |
| 실제 원문·소비자 | 원래 go-stablenet30함수, 원문 commit/함수 SHA/graph snapshot pin으로 실제 CKS stdio 호출. 파일·줄의 중복 후보는 정규 심볼 ID로 구분 |
| 합성 좌표 | 별도 소유 인덱스14사례; entry와 선언 좌표, Rbrace 합성 반환, bare return, UTF8, shadow, read/write phase, block -1 상수를 구분. 프로젝트30 표본에 합산하지 않음 |
| 노드·선언·블록 | 노드·선언 미연결0. predecessor/successor·분기 순서로 빈 블록을 대조, 미연결3개는 unreachable 빈 블록. live branch–block12968쌍 매칭 누락/구조 차이0 |
| 남은 검토 부채 | occurrence phase53개(range 암묵 정의37·소스 CFG 소유 중복16), 매칭된1553쌍 중 lexical 차이2개, 역방향 미연결 소비자 def-use32개. [검토 원장](./operational-preparation/source-review-supplement/followups/wi1-consumer-matching-v1/REVIEW-LEDGER.json)에 보존; 품질 점수 아님 |
| 상한·종료·보존 | 실제 호출1회가 최종 원문 검증 상한으로 budget_exhausted; 응답 보존 후 동일 pin/상한 재확인에서 partial 반환.4세션 정상EOF0, 잘못된3종 pin 거부, retained 파일 전후 hash 동일 |

단계45는 **AI DEV 보완과 실제 매칭 관찰**의 완료다. 의미 정답/독립성·정밀도/재현율·실행 수용의 승인이 아니다. 공유 CFG 구성기의 공통 오류 한계, memory·호출 효과·실제 경로 가능성의 한계가 남는다. 소비자 응답을 정답으로 복사하지 않았고 구조 차이2개도 초안 정답으로 자동 승격하지 않았다. `check-wi1-consumer-matching.py --require-formal-gold`는 정식 정답 사용을 거부한다. 새로운 전체 native Go 검증으로 포장하지 않고, 소스1738개·승인 입력5개의 동일성을 확인해 이전 native 검증을 재사용한다.

다음은 **WI-2 DEV 입력·완전 AI 정답 초안**이다. 나머지 DEV/AI 작성8개는 WI-2/3/4/V1/V2/6/SP/C이며, WI1 검토 부채·D04 역할/독립성·새 미노출 FINAL·독립 완전 gold·matching/충분성·F01~08 동결은 별도 미완료다. 정식9개 실행·수용과 필수 온톨로지 독립 효과 평가, 선택 후속12개도 [전체 작업표](./operational-preparation/source-review-supplement/followups/wi1-consumer-matching-v1/WORKLIST.json)에 유지한다.


## 최신 상태 — 2026-10-18 / WI-1 DEV·AI 정답 초안

**단계44: 기존 프로젝트30함수의 DEV 입력과 범위 내 완전한 AI 초안30개를 작성했다.** 원문25파일에 결속한 지역 binding412개·occurrence1721개를 다루고, 콜백 함수형의 signature4개는 별도 격리했다. 정의–사용쌍1654개(positive556/negative94/unknown1004), 별도 shadow 음성410쌍, 구조 제어 branch–block쌍12968개(positive737/negative12231)를 전수 분류했다. 이는 후보 분석기의 품질 점수나 독립 정답 승인 수가 아니다.

public 입력/원문과 private AI 정답을 분리했다. 정답은 후보 valueflow 출력 없이 원문 관측 및 새 참조 알고리즘으로 작성하고 정의–사용/제어 각각 두 방식으로 대조한다. CFG 생성기는 후보와 같은 x/tools0.48.0이므로 공통 오류 가능성이 있다. 파일 단위 부분 타입 관측과 후보의 고립 함수 타입 문맥도 같다고 가정하지 않는다. 주소·range·capture·상수·미해결 타입·비종료와 실제 consumer 매칭 한계를 보존하며, 지원 밖 의미까지 완전한 운영 정답을 만들었다고 판정하지 않는다.

각 함수의 `private/SRC-xxx-REVIEW.txt`에는 전체 use별 정의 집합과 unknown 사유·블록 매칭·제어 양성/unknown·음성 complement를 사람이 읽을 수 있게 작성했다. [SRC-007 검토 예시](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/private/SRC-007-REVIEW.txt)를 볼 수 있다.

[현재 보고서](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/REPORT.txt), [초안 현황](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/SUMMARY.json), [현재 단계와 남은 전체 작업](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/WORKLIST.json), [검증 기록](./operational-preparation/source-review-supplement/followups/wi1-dev-gold-v1/validation-summary.json)을 따른다. 다음은 WI1 의미 unknown/값 종류·합성 좌표와 실제 소비자 매칭 보완, 이후 WI2 DEV/AI 초안이다. 나머지8개 WI 초안·새 DEV/FINAL 독립 가족/완전 의미 gold·F01~08 동결, 정식9개 수용, 필수 온톨로지 효과 수용, 별도 후속12개가 남는다.

D04미정·독립 승인0·FINAL0·정식0/9·freeze null/execution_allowed=false를 유지한다. 이전 단계43 packet과 도구 source pin은0f28a9a7의 과거 검증으로 보존하고, 현재 준비 검사는 새 WI1 DEV 종류를 사용한다. 유지보수 Go·승인 입력·원문/DB/운영 current·전역 설정은 변경하지 않는다.

현재 범위 회귀112개·원래0f28a9a7 소스의 과거 결속 회귀49개, 수기 source 통제14개·원래 Git25파일 대조·payload101파일 동일 재생·문서134개·미승인/미동결 거부 검증을 통과했다. 새 보조 관측기는 build/vet/fmt 통과이며 유지보수 Go 전체 시험을 새로 실행한 것으로 기록하지 않는다.

## 이전 준비 — 2026-10-18 / 독립 수용 동결 준비

**단계43의 준비 작업 FP01~04를 완료했다.** 기존 후보206행을 원본 SHA/행 좌표·노출 이력과 연결하고,9개 WI와 온톨로지의 완전 정답·매칭 작성 양식10개, 표본 충분성 원장, F01~08 준비표와 FINAL 접근·봉인 절차를 작성했다.206행은 독립 표본206개가 아니다. 모두 노출된 DEV 작성 자료이고, 새 FINAL·승인 gold는0개다. WI3의20입력은 제안 가족15개, WI6은 운영19개+stress1개, V1/V2는 공유 개발 티켓1개다.

검사기는 정답의 null/빈 집합·범위 누락·중복 분류·출처·검토 hash·DEV/FINAL 가족 중복·프로젝트별 위치 매칭을 구분한다. 온톨로지는 DEV4모드 후 FINAL primary1모드 사전 선택, 검토된 구현-개념 연결, 독립 의미 정답, 동일 pin/cap과 혼입·인용·지연 gate를 준비했다. 검색 개선을 실제 LLM 답변 품질로 판정하지 않는다. 검사 통과는 의미 정답의 정확성이나 사람 검토를 증명하지 않는다.

[현재 보고서](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/REPORT.txt), [현재 단계와 남은 전체 작업](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/WORKLIST.json), [F01~08 준비표](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/freeze-readiness.json), [검증 기록](./operational-preparation/source-review-supplement/followups/independent-freeze-v1/validation-summary.json)을 따른다. 다음은 D04 역할·독립성 지정과 새 가족/입력·완전 gold의 작성·검토다. D04미정, 정식0/9, freeze null/execution_allowed=false를 유지한다. 로컬 로드맵7/7·28/28, 격리 준비22/22, 비운영 pilot4/4는 완료 상태를 유지한다. 선택 후속12개는 별도 backlog다.

이번 변경은 Python/문서이며, 결속 소스1738개와 승인 입력5개의 hash가 이전 검증과 동일하다. Go 전체 시험을 새로 수행한 것으로 기록하지 않는다. 원문·데이터·운영 current·전역 설정·이전 증거는 보존했다.

현재 회귀133개·원래 버전9de07a82의 과거 준비 회귀36개, 문서134개·완료 보호·미동결 실행 거부 검증을 통과했다. 현재 준비 참조와 과거 host 준비 참조를 분리해 원래 증거의 소스 범위를 보존한다.

## 이전 검증 — 2026-10-18 / SN-P04

비운영 pilot **4/4 완료**. 실제 go-wemix1350376a의 새 BGE-M3/native1024 held DB와 go-stablenet0bf2f4d1의 보관 DB에서 같은6진단 입력·검색·v2·6개념 비교·출처/혼입 경계를 검증했다. 기존 go-wemix 자료는 mock64이며 새 BGE 자료와 구분한다. 서로 바뀐 실제 graph/vector 설정2개는 기동 거부, 최종정상5+초기/보완전진단6세션은 EOF0·모든 소유 프로세스 회수·강제종료0이다. corpus/정책 범위가 다르며 매핑은 proposed라 품질 우열·운영망 합의·온톨로지 독립 효과를 승인한 결과가 아니다.

새 소스38dd5c54(Go1738개)에서 일반/race 각4097pass/18skip, build/fmt/vet/lint/boundaries/docs/build-bins 및 별도 비교/격리 race232개가 새로 통과했다. 이전 결과 재사용으로 대신하지 않았다. [최신 보고서](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/REPORT.txt)와 [현재 작업리스트](./operational-preparation/source-review-supplement/followups/stablenet-comparison-v1/WORKLIST.json)가 최신 상태다.

다음은 독립 DEV/FINAL·완전 gold·matching/충분성·F01~08 동결 준비다. 남은 전체는 해당 준비, 정식 WI9개(0/9), 필수 온톨로지 독립 효과 평가, 별도 후속12개다. D04미정·freeze null/execution_allowed=false, 로컬 로드맵7/7·28/28·준비22/22 및 기존 제한 온톨로지 연구완료/inconclusive를 유지한다. 운영 current·전역 설정·외부 게시를 변경하지 않았다.

이하 이전 시점의 구현/연구 기록은 당시 범위이며 현재 상태는 위 최신 보고서·원장을 우선한다.

운영·독립 수용 준비 갱신(2026-10-16): source3a530a25 격리 빌드·CKS33개 스키마 일치/ChainBench81개 metadata·현재 Claude Code2.1.301의 로컬 scripted5개를 새로 검증했다. 실제 모델/노드0, 정식9개 수용0/9·F-01–08 pending이다.9개 baseline/표본/위치 매칭/분모 제안과 역할·입력·recipe의 남은 결정을 [현재 protocol](./OPERATIONAL-ACCEPTANCE-PROTOCOL.md)에 정리했다. 다음은 검토자·범위·독립 입력/gold 검토다.


2026-10-16 기준 지식 매니저·프로젝트 비교 로드맵은 **28/28 세부 작업·7/7 기능 완료**다. 사용자가 멈추라고 하기 전까지 남은 작업을 순서대로 구현·검증하고 단계별 서명 로컬 커밋으로 보존한다. 각 완료 단계마다 현재 단계·검증 결과·다음 작업·남은 전체 범위를 화면 로그와 문서에 기록한다. 세부 상태는 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md), [명세](./KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md), [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.

## 현재 구현과 다음 단계

| 기능 | 상태 | 확인한 범위 |
|---|---|---|
| KM-01 프로젝트 격리 | 4/4 완료 | 실제4DB·동명/포크/버전·인용·공개 읽기/쓰기 거부·rollback/GC |
| KM-02 버전 DB 운영 | 4/4 완료 | 공개 build/status/cancel·영속 job/재시작·유지/실패 matrix |
| KM-03 Claude 매니저 | 4/4 완료 | 등록·문서 없는 수집/초안·검토/회수·실제 설치 CLI8회/68도구; 로컬 시험 응답기 |
| KM-04 지속 갱신 | 4/4 완료 | 실제 periodic/change2프로젝트·20항목·221호출·2EOF0·삭제/no-op·실패/연결 retry·타 DB/인용 보존 |
| KM-05 공통 개념/연결 | 4/4 완료 | 공통 어휘·원문 연결·직접 매핑 검토/회수·적용/지원·L1/L2/network/산업 차이 |
| KM-06 비교 근거 MCP | 4/4 완료 | 실제 go-stablenet/go-wemix4243파일·6개념40셀·22항목19호출·3EOF0·오매핑/범위/예산/원본 보존 |
| OE-01 온톨로지 효과 | 4/4 완료 | 512회 함수 과제+실제2프로젝트 매핑 입력 효과18항목/27호출·원문8셀/양측후보3개; off/inconclusive |

OE-01.3 완료: 고정512회 원자료에서 유한한 완전 함수 찾기 과제의 precision/recall·순위·의도한 ontology 개념/범위 진단·warm 지연·실제 로컬 API 호출/서버 token 수를 재계산했다.5사실군 평균 recall은 off54.44%→concept_text/combined56.67%, precision12.87%→13.54%, MRR0.4612→0.4862다. 개선은 EX-08 영어 메시지 처리 질문1개에서 prepare만 회수하던 결과에 commit이 추가된 것으로 한정된다. 나머지13개 채점 질문의 지표는 같았다. hit@10은68.33%로 동일하다. 일반 행동/정책의 semantic relevance나 답변 정확도로 확대하지 않는다.

각 arm의 warm80개 p95는 off583.1ms, concept_text736.2ms(1.263배), relations607.4ms(1.042배), combined697.4ms(1.196배)다. capture batch embedding API는 off/relations1584개·31536서버 prompt tokens, concept_text/combined1664개·56112tokens다. 준비/모델 확인도 포함한 도구 실행 batch 비용이며8호출×16질문의512개 독립 사실이 아니다. 토큰은 서버 보고값이고 금액/전력/하드웨어 비용은 추정하지 않았다. 유료/LLM 답변0이다. 활성 모드는 의도 개념13질문 중9개를 매칭했고, scope 의도 진단3질문 중 외부 go-wemix 대상 EX-16에도 WBFT quorum을 매칭했다. relations는 검토된 구현 연결0·rank 변화0이다. 정의 검색/combined의 최종 인용 순서는32짝 중18짝에서 달랐지만 함수 회수 이득은1질문뿐이다. 작은 source-derived 개발 표본과 지연 관찰로 기본 off·quality=inconclusive를 유지한다.

전체28/28·기능7/7의 문서화된 로컬 로드맵을 완료했다. 별도 정책 생성기의 명시적 소스 대조 재생성 보완도 완료했다. 다음 별도 단계는 운영·독립 수용 범위를 동결하는 일이다. 운영·독립 수용9개와 선택 후속12개는 별도 범위로 유지하며 미실행/미승인 사항을 완료로 바꾸지 않는다.

## 검증과 현재 제한

실제 비교MCP22항목·19도구호출·3EOF0, 원문/범위 hash와 본문·프로젝트/모드별 인용·양쪽 후보·기본 미검토 제외·live 소스 삭제 후 동일 응답·한쪽 archive 누락/오매핑·범위/예산 거부·원본/DB 보존을 검증했다. 새 Go 변경이 없음을1273개 Go와 모든 결속 입력의 hash로 확인해 직전 전체Go4054(4036통과18skip)·race1028(1026통과2skip,7패키지)·빌드/fmt/vet/lint 결과를 재사용했다. 새 실제 corpus/MCP 시험과 과거 Go 실행을 구분한다. 유료 모델 호출0이며 ontology 효과/독립 품질 수용은 아니다. [검증 자료](../../system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/manifest.json)를 따른다.

매니저 plugin0.6.0은 reviewed 기본18도구와 명시 updates6도구를 제공한다. 실제 Claude Code2.1.300과의 갱신 연결2회/36도구도 로컬 시험 응답기로 검증했다. 실제 모델 호출/비용0이며 agent 자율성·답변 품질은 측정하지 않았다. 개인 머신에 서비스나 OS scheduler를 설치하지 않았고 명시 foreground worker가 실행 중인 동안만 주기/변경 polling이 동작한다.

원래 독립 수용의 단계0·담당자·실제 모델/인증/비용 지정은 로컬 개발을 막는 요건이 아니다. 유료 모델·독립 수용·외부 게시에는 해당 실행 범위가 별도로 필요하다. 이번 지속 작업은 단계별 로컬 커밋 범위다. 푸시/PR·전역 설치·운영 current 변경을 완료로 추정하지 않는다.

## 보존할 결정과 품질 관찰

정책 참조 수정7건은 별도 평가 overlay로 사용자 채택 승인을 기록했다. [승인 영수증](../../system/eval/operational-preparation-2026-10-15/adopted-policy-user-approval/receipt.json)과 [적용 검증](./POLICY-OVERLAY-ADOPTION.md)을 따른다. 원본 정책·원래 승인 입력은 보존한다. 기존 무소스 sync의 동작을 보존하면서 명시적 --code-root의 선언 대조/오파일 거부와 재생성을 보완했다.

[OE-01 단일 프로젝트 관찰](./ONTOLOGY-EFFECT-DEV-OBSERVATION.md)은384회 실패/누락0·인용3,616개 원문 일치, 활성288회 no_match·순위 차이0이었다. 별도 정확 용어8회도 최종 boost0이다. 기본 off·quality=inconclusive를 유지한다. KM-04.4에서 단독 합성 대문자 식별자 질의가 비고 같은 자료가 자연어 질의에 나타난 사례도 OE-01 질의 해석/회수율 후보로 보존했다. 새 기능 구현이나 계획 승인을 독립 정답·품질 승인으로 바꾸지 않는다.

이전 단계의 상세 기록은 각 명세의 checkpoint 절과 서명 커밋·동결 증거 packet에 남긴다. 현재 상태는 이 문서 첫 표와 전용 원장을 우선한다.

## OE-01.4 실제 다중 프로젝트 매핑 효과 평가

동일한 실제 go-stablenet/go-wemix 보관 DB와6개 개념·40개 프로젝트/축 셀을 대상으로, binding 입력만 뺀 기준선과 고정된 제안 매핑7개를 비교했다. 양쪽 모두 include_proposed=true이며 기본 검토 필터는 별도 확인했다. 프로토콜을 호출 전에 저장하고 순서를 교대해2회 반복했다. 24개 행렬+describe2개+기본 필터1개=도구27호출, 검증18항목, 두 연결 EOF0이다. 반복80셀을 독립 사실80개로 세지 않고 arm별 고유40셀로 보고한다.

| 관측값 | 매핑 없는 기준선 | 제안 매핑 입력 |
|---|---:|---:|
| 고유 프로젝트/축 셀 | 40 | 40 |
| 원문 근거가 있는 셀 | 0 (0%) | 8 (20%) |
| 선언된 exact/similar/not_applicable 셀 | 0 | 7 (17.5%, 모두 similar) |
| unknown | 0 | 18 |
| not_collected | 40 | 15 |
| 검증한 셀 인용 | 0 | 9 |
| 양쪽 근거가 있는 비교 후보 | 0 | 3 |
| 검사한 프로젝트 그룹 / 혼입 | 12 / 0 | 12 / 0 |

9개 인용의 프로젝트·버전·source mode·원문/범위 hash·본문을 검증했다. 근거0개인 기준선의 source accuracy는 측정 불가(null)다. 처리군의 원문 일치100%는 hash/body 무결성 범위이며 의미 정확도100%가 아니다. 두 arm은 동일한 scope 격리 장치를 사용하므로 혼입0을 매핑만의 효과로 돌리지 않는다. finality 근거1셀은 unknown을 유지하며, staking/정책 미수집을 기능 부재로 단정하지 않는다. 기본 reviewed-only 요청에는 미검토 매핑이 나타나지 않는다. DB/source/config 전후 byte는 동일하다.

매핑은 평가자가 원문을 보고 선별한 제안 입력이다. 따라서 이 결과는 **연결 주석을 추가하면 비교 가능한 원문을 구조적으로 모을 수 있다**는 제한된 관찰이며, 온톨로지 어휘 자체의 인과 효과·독립 holdout·자연어 의도 해석·LLM 답변 품질·실제 네트워크 성능 평가가 아니다. 기존 단일 프로젝트 분석과 함께 기본 off·quality=inconclusive를 유지한다. 사람의 매핑/정답 승인과 정식 독립 수용은 기록하지 않았다.

[원자료와 검증](../../system/eval/knowledge-roadmap-2026-10-16/oe-04-mapping-effect/manifest.json)에 protocol·원시 요청/응답·소스·재계산/반례 시험을 보존한다. 로드맵 **28/28·기능7/7의 로컬 구현/명시된 개발 검증을 완료**했다. 정책 생성기 보완, 운영·독립 수용9개, 선택 후속12개는 별도 범위다. 유료 모델·운영 활성화·외부 게시 승인을 이 완료로 대신하지 않는다.

## 소스 대조 정책 생성기 보완 완료 (2026-10-16)

명시적 `cks domain sync --code-root <source>`는 폴더 이름 대신 실제 Go 선언을 읽어 package·receiver·type/field/const/var/interface 이름을 해당 anchor 파일에서 대조한다. receiver 없는 이름은 해당 파일에서 유일할 때만 보완하고, 오파일/모호한 이름/외부·symlink 경로/손상된 소스는 두 출력을 쓰기 전에 거부한다. symbol이 없는 파일/행 전용 anchor는 새 정책 연결로 만들지 않는다. 기존 무소스 sync는 이전 결과를 보존한다. 이 경로는 명시적 옵션이며 기존 setup/운영 profile을 자동으로 전환하지 않는다.

고정 소스0bf2f4d1에서 원래 ErrInvalidFeePayer 파일 참조는 거부되고 두 기존 출력은 보존됐다. 소유 평가 사본의 해당 anchor 파일만 core/txpool/errors.go로 수정한 후, 승인 overlay와 같은42개 정책·146개 연결을 재생성했다. 생성기는 참조를 결정적으로 정렬하므로 승인 overlay의 기존 목록 순서와 byte는 일부 다르지만, 모든 정책 필드와 정렬한 governs 목록은 같다. 원본 entry·정책·승인 overlay와 원본 소스 HEAD/status를 보존했다. 원래 entry의 잘못된 파일 위치를 조용히 찾아 옮기거나 원본 설명/검토 승인을 변경하지 않았다.

새 생성 결과로 실제 graph를 구축하고146개 연결을 고정 AST 위치와 대조했다. 수정7개에 대해 공개 CKS 일반·소스/snapshot pin·canonical ID 조회21회, 잘못된 pin2개·assembler 혼입1개 거부/빈 결과까지 총24회 도구 조회와26쌍 원시 요청/응답을 검증했다. CKS는 EOF0으로 종료했다.843개 파일이 graph에 선택됐고 parse error0이다. source corpus는882개 비시험 Git Go blob이며 대상 프로젝트 코드를 실행하거나 실제 LLM/유료 모델을 호출하지 않았다.

함수뿐 아니라 실제 정책의 구조체 필드·인터페이스도 확인했다. 초기 진단의 필드 누락과 정책 연결을 요청하지 않는 loc anchor 처리, CLI 오류 정제 정책에 대한 평가 도구의 가정을 수정한 이력은 최종 통과와 구분해 보존한다. 원자료 재계산/반례·새 전체 Go/race·빌드/fmt/vet/lint·문서 검증은 [증거](../../system/eval/knowledge-roadmap-2026-10-16/policy-source-regeneration/manifest.json)를 따른다. 이는 source-name 연결 정합성의 개발 검증이며 정책 의미의 사람/독립 수용이나 운영 채택이 아니다.

현재 로컬 로드맵28/28·기능7/7과 별도 생성기 보완을 완료했다. 남은 별도 범위는 운영·독립 수용9개와 선택 후속12개다. 다음은 독립 평가를 실제로 수행할 경우 해당 입력·검토/실행 범위·필요한 실제 모델 조건을 고정하는 단계이며, 로컬 검증을 그 승인/수용으로 대신하지 않는다.
