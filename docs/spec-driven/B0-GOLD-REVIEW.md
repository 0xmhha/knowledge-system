# B0 질문·정답 검토표

상태: **검토 요청 초안** (2026-10-02). 이 표는 [`questions.json`](../../system/eval/b0-knowledge-system/questions.json)의 12개 문항을 요약한 것이다. 질문·후보 답의 **정확한 평가 입력 문장**은 JSON을 기준으로 검토한다. 모든 `review_state`는 `draft`이며 이 표 자체가 승인 기록은 아니다. 근거는 현재 작업 브랜치가 아니라 **고정 코퍼스 커밋** `71cb71cd55960833e930269e272f7a4a060be3aa`의 파일·줄을 뜻한다.

| ID | 질문 요약 | 제안된 정답/동작 | 고정 근거 |
|---|---|---|---|
| B0-CODE-01 | CKG와 CKV가 서로 다른 소스 스냅샷을 색인했을 때 어디서 거부하는가? | `VerifyAlignment`이 프로젝트·스냅샷·데이터셋·파일 매니페스트·캡처 정책을 비교한다. `cite` | `internal/setup/verify.go:57–85` |
| B0-CODE-02 | CKV가 정확 후보 상한에 닿으면 어디서 불완전 검색으로 보고하는가? | `SearchDetailed`이 `incomplete/candidate_limit`을 반환한다. `cite` | `internal/vector/store/sqlitevec/store.go:613–654` |
| B0-CODE-03 | v1 데이터셋에 v2 문맥 질의를 할 때 재색인 경계는 어디인가? | `get_for_task_v2`가 보관 버전의 v2 신원을 검사한다. `cite` | `internal/system/mcp/get_for_task_v2.go:33–55` |
| B0-WHY-01 | `CHECKED_BY` 테스트 연결이 왜 사람 수용 승인이 아닌가? | 연결 검토와 `CriterionDecision.approved` 사건이 분리된다. `cite` | `docs/spec-driven/PUBLIC-CONTRACT-V2.md:67` |
| B0-WHY-02 | 코드가 같아도 프로젝트 지식 팩 변경이 왜 새 데이터셋을 만드는가? | 빌드 레시피에 잠금 다이제스트가 포함된다. `cite` | `docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md:40` |
| B0-WHY-03 | 패치 테스트가 성공해도 왜 자동 승격되지 않는가? | 근거 연결과 필수 기준별 사람 승인 결정을 요구한다. `cite` | `internal/system/patch/review.go:205–281` |
| B0-POLICY-01 | 같은 범위·시점의 검토된 프로젝트 정책 두 개가 충돌하면 무엇을 우선하는가? | 자동 우선하지 않고 충돌을 드러낸다. `cite` | `docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md:23`; `internal/system/knowledgepack/instances.go:195–242` |
| B0-POLICY-02 | 산업 팩이 검토된 프로젝트 정책을 덮어쓸 수 있는가? | 산업 팩은 템플릿이며 불일치는 기록하거나 적용 제외한다. `cite` | `docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md:23` |
| B0-POLICY-03 | 접근 권한 없는 비공개 정책의 본문/ID를 코딩 문맥에 넣을 수 있는가? | 본문·ID를 숨기고 불확실 상태를 유지한다. `cite` | `internal/system/knowledgepack/instances.go:203–241` |
| B0-TRACE-01 | ADR 요구가 검증된 코드·테스트 경로라고 주장하기 전 무엇이 일치해야 하는가? | 검토된 추적 링크, 현재 데이터셋 좌표, 보관된 원문 참조, 코드·테스트 앵커가 일치해야 한다. `cite` | `internal/system/evidencev2/knowledge_trace.go:20–115` |
| B0-ABS-01 | 이 조직의 운영 블록체인 가스 환불 정책 승인 담당자는 누구인가? | 승인된 조직 운영 정책·담당자 근거가 없으면 기권한다. `abstain` | **부재 확인 필요** |
| B0-ABS-02 | 실제 배포 서명 개인키와 보유자는 누구인가? | 승인된 운영 키/보유자 근거가 없으면 기권한다. `abstain` | **부재 확인 필요** |

## 검토 시 확인할 것

1. 질문별 문장과 제안 답이 실제 파일·줄의 **의미**와 일치하는지 확인한다. `b0-preflight.py`는 앵커가 존재하고 줄 범위에 들어가는지만 기계 검사한다. 의미 검증은 하지 않는다.
2. `B0-ABS-01/02`는 파일 한 곳이 아닌 고정 코퍼스 전체에서 출처 부재를 확인한다. 실제 비밀이나 개인 신원을 찾아내는 과제로 해석하지 않는다.
3. 정책 질문은 **프로젝트 구현·설계 계약**을 묻는다. 특정 조직의 업무 정책이 실제로 승인됐다는 정답으로 바꾸지 않는다.
4. 코드 위치·이유·정책·기권의 비율과 한국어/영어 표현을 확인한다. 동적 실패 6종은 [`B0-DYNAMIC-FIXTURES.md`](./B0-DYNAMIC-FIXTURES.md)에 별도 초안으로 남아 있다.
5. 승인 또는 수정할 문항 ID를 결정한다. 승인 뒤 각 항목에 `review_state=approved`, 검토자와 시각을 기록하고 질문셋 SHA-256을 다시 고정한다. 모델 후보 `bge-m3:latest`의 확정도 별도 결정이다.

원문 확인 예시:

```sh
git show 71cb71cd55960833e930269e272f7a4a060be3aa:internal/setup/verify.go \
  | nl -ba | sed -n '57,85p'
```

모델 승인과 12개 정답 승인 뒤에도 실제 파일럿 벡터 색인, 완전성 검사, 동적 fixture 실행 및 B1 ablation은 남는다. 현재 품질 지표는 `null`이다.
