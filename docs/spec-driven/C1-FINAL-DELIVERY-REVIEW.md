# 최종 시험 preview 지원·제한·전체 종료

2026-10-06 · **사용자 승인 범위 전체30/30 완료·미완료0**. 사용자 원문: **“HC-01–11 해석 승인. 시험 preview 범위로 종료.”** [승인 원장](../../system/eval/b0-knowledge-system/human-approved-preview-closure-m2max-2026-10-06/human-approval.json)에 승인 전 표 SHA·로컬 검토자 `chat-user`·기록 시각·실제 DEV/FINAL 응답/measurement ID를 연결했다. 정확한 메시지 수신 시각·검토 시간·비용은 미측정이다.

이번 deliverable은 기존 평가 결과·시험 서명 preview 패키지·검증/복구 증거·지원 제한 문서다. [FINAL 평가](./C1-APPROVED-FINAL-REPORT.md), [최신 패키지](./C1-FINAL-PACKAGE-VALIDATION.md), [사람/paired 종료](./B1-HUMAN-AND-PAIRED-CLOSURE.md), [운영 검토표](./C1-OPERATIONS-REVIEW.md)가 근거다. **품질 fail·중요 그룹 inconclusive·ontology disabled·운영 출시 보류**를 유지했다.

## 지원 증거와 미확인 범위

| 대상 | 확인한 최신 실행 | 한계·미확인 |
|---|---|---|
| Darwin ARM64/M2 Max | 추출된 시험 서명 패키지, 두 검증기, 세 mock 설치/업데이트/롤백, BGE guard10, 전체 strict 데이터 비용60+RSS2, 실패/손상/구 v1 복구 | 공유 호스트·단일 DEV 질문의 진단 비용. 운영 backup/failover·담당자/실제 key·법적 적합성 승인 없음 |
| Linux ARM64/Docker VM | Go 없는 Debian에서 서명·세 mock 프로젝트 설치, 실제 고정 BGE로 세 소형 프로젝트 install/update/rollback, SDK6 | CPU 제한 컨테이너의 소형 합성 입력·180초 초기화 deadline. 네이티브 운영 호스트/대규모 실제 비용·기본 deadline 지원 보증 없음 |
| Linux AMD64/ARM64 에뮬레이션 | 최신 패키지 시험 서명·독립 검증·세 mock 프로젝트 설치/업데이트/롤백 | 네이티브 AMD64·실제 모델/운영 비용 환경 없음. 해당 증거는 preview에 한정 |

Go 없는 런타임의 empty-Go/Python은 문서 기반 degraded가 정상이며 TypeScript는 분석 가능 범위다. Go 소스 분석을 지원하려면 Go 분석 도구 체인을 별도로 제공해야 한다. mock은 모델 검색 품질을 판정하지 않는다. 플랫폼별 Go 도구 체인·합성 source commit·binary SHA는 원장에 각각 남겼다.

## 알려진 제한과 최종 기술 판정

- 승인 FINAL 4,800회 중 POLICY-03 정상 팩을 반환하지 못한 240회가 있다. 양성 정적 6문항 중 기대 근거를 찾은 문항은 1개, 양성 Recall 0.1667/MRR 0.0833이다. 실패를 제외한 성공률로 바꾸지 않는다.
- 두 기권 문항의 strict no-citation 조건은 실패했다. HC-09/10의 사람 답변 보류 판단은 사용자가 승인했다. 제품은 LLM 생성 답변을 반환하지 않는다.
- 중요 질문군의 독립 최종 표본은 각각 10개 미만이다. 승인 규칙에 따라 inconclusive이며 기본 활성/출시 근거로 쓰지 않는다. 반복·언어·상태 변형으로 독립 표본을 늘리지 않는다.
- 지연은 정적/동적/pooled 및 같은 팩 축별 비교를 모두 보존했다. 일부 dynamic p95 비율이 1.25를 넘는다. 가장 유리한 pooled 수치만 골라 통과시키지 않는다. 사람 검토 시간·비용은 미측정이다.
- 정상 팩의 source/좌표/해시 검증과 선택적 backend 오류를 구분한다. 최신 대규모60의 neighbors 오류360·별도RSS2의12·Linux 실제SDK6의6을 기록했다.
- 32,000바이트 본문 상한은 유지했다. FINAL 결과에 맞춰 상한·질문·gold·K·제품 소스를 수정하지 않았다. 개선하려면 새 개발 버전과 결과를 보기 전에 검토한 별도 최종 입력이 필요하다.
- 시험 scope의 Ed25519 검증기는 운영 키 권한·유효 기간·온라인 폐기·운영 출시 승인을 증명하지 않는다. 시험 키를 운영 키로 승격하지 않았다. 원문 고지 수집/SHA 검증은 법무 적합성 판정과 별개다.

## 4개 잔여 항목의 실제 종료 근거

| ID | 완료 근거 | 유지한 제한 |
|---|---|---|
| B1-04 | DEV/FINAL 질문군 자동 지표와 승인 HC 해석/사람 기권 집계 결합 | 검토 단위는 해석11개. 전체 SDK 생성 답변 오류율 null, strict no-citation fail |
| B1-07 | HC-01–11 원문 그대로 승인·판정자/시각·승인 전 표/실제 응답 SHA 결합 | 운영 사실/criterion acceptance/법무 승인 아님 |
| B1-08 | paired 실패·불확실·같은 축 비용·동결과 실제 사람 판정 결합 | 소표본 inconclusive·실패/지연 경계 유지·FINAL 튜닝 없음 |
| C1-05 | 사용자의 명시적 시험 preview 종료 선택, 시험 신뢰/서명·고지 검토 자료와 운영 제외 확정 | 원래 운영 키/정책 완료조건은 이력에 보존. 실제 운영 조건 충족·키 승격을 주장하지 않음 |

B1-06의 비용 사실 보고는 이미 완료했으며 비용 미측정을0이나 절감으로 바꾸지 않았다. C1-07은 이제 전체 작업 종료 증거와 지원/제한·범위·출시보류를 연결했다. [전체 작업리스트46절](./EXECUTION-WORKLIST.md)과 [현황판](./EXECUTION-STATUS.md)이 현재 기준이다. 과거 pending/26완료 원장은 당시 상태로 보존했다.

## 공식 판정과 종료 범위

[게이트 종료 원장](../../system/eval/b0-knowledge-system/human-approved-preview-closure-m2max-2026-10-06/gate-decisions.json): **B0 pass**는 승인 입력·재현 가능한 실모델 기준선 성립, **C0 pass**는 관측 개발 결함의 수정/회귀/동결이다. **B1 fail·C1 fail**은 비용 회귀·기계 기권 guard/최종 오류·품질 제한을 그대로 드러낸 판정이다. 중요 질문군은 inconclusive다. 공식4/4 판정 완료가4개 품질 합격을 뜻하지 않는다.

사용자가 선택한 시험 preview 범위에서 **남은 전체 작업0, 다음 작업 없음**이다. 실제 운영 OP01–08 역할/키·독립 신뢰/보관/폐기·법적 적합성·실운영 지원/backup/failover, 네이티브 AMD64 실제 모델, Darwin SDK/전체 생성기/전이 native 적합성 등은 **범위 밖·미확인·미승인**이다. 운영 출시를 별도로 요청하면 그 사실/환경부터 검토해야 한다. 이번 결정은 원격 배포·새 Git push 권한을 부여하지 않았다.

현재 품질 개선을 후속으로 수행하려면 새 DEV 버전과 결과를 보기 전에 검토한 별도 독립 FINAL 입력이 필요하다. 현재 FINAL을 새 버전의 독립 holdout으로 재사용하거나 상한·gold·K·임계치를 성공에 맞추지 않는다.
