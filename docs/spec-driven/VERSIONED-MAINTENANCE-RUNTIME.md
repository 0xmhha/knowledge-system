# KM-02.2 공개 버전 DB 구축·상태·취소

관리 플러그인이 작업 접수를 DB 구축 완료로 오인하지 않도록, 등록된 프로젝트의 불변 후보 구축과 실제 활성화를 구분한다. [요청·상태 계약](./VERSIONED-MAINTENANCE-CONTRACT.md)의 공개 실행 연결이며, 영속 작업/재시작 복구는 KM-02.3에서 별도로 구현한다. 전체 상태는 [시험표](./KNOWLEDGE-ROADMAP-TEST-MATRIX.md)와 [원장](./KNOWLEDGE-ROADMAP-PROGRESS.json)을 따른다.

## 등록과 공개 도구

운영자가 CKS 설정에 다음과 같이 profile을 등록한다. 설정 경로와 실행 파일은 호스트가 소유하며 도구 caller가 덮어쓰지 못한다. 경로는 절대 경로이고 source/output은 서로 겹치거나 symlink를 포함할 수 없다. 다른 profile의 output과도 겹치지 않는다.

```yaml
maintenance:
  profiles:
    - id: example-default
      project_id: example
      setup_config: /workspace/profiles/example/setup.yaml
      allowed_activation: [hold_for_review, activate_after_gate]
      timeout_seconds: 3600
```

등록한 setup 설정에는 `project_id`, `source_mode`, `src`, `out`, graph/vector/cks 실행 파일과 embedding 설정을 둔다. strict YAML은 오타·중복 key·여러 문서를 거부한다. source는 committed/working-tree/snapshot-only를 지원하며 vector 생략은 허용하지 않는다. source/config/실행 파일 바이트·입력·실제 embedding identity를 digest로 결속한다. `.cks/knowledge`는 기존 검토·lock을 검증한다. domain/semantic 입력은 이미 렌더링된 자료를 사용하며 inspect가 문서를 생성하거나 수정하지 않는다.

| 공개 이름 | 의미 | readOnlyHint |
|---|---|---|
| `cks.dataset.inspect_profile` | project/profile의 source/input/model/base pin·허용 활성화·현재 capability 조회 | true |
| `cks.dataset.build` | inspect의 pin과 새 version/key를 받아 비동기 구축 시작 | false |
| `cks.dataset.status` | project/profile/job ID가 모두 일치하는 작업 상태 조회 | true |
| `cks.dataset.cancel` | 해당 작업 취소 요청·실제 종료 상태는 status로 확인 | false |

미등록 기본 서버는 기존33도구를 유지한다. maintenance 등록 시4개가 추가되며 collection2개를 함께 등록하면39개다. JSON input/output schema를 노출하고 중복·unknown·case alias·후행 JSON과64KiB 초과를 거부한다. 도구 인자에는 경로·shell·endpoint·skip-vector가 없다. 공개 오류는 code/retryable/phase로 제한하며 내부 경로/명령의 오류 원문은 반환하지 않는다.

## 구축·활성화와 보호 경계

inspect는 읽기 작업이며 output을 생성하지 않는다. build는 immutable source capture → 기존 CKG/CKV 구축 → 원문/입력/신원/구성된 graph·vector·knowledge·semantic gate → hold 또는 실제 current 전환을 수행한다. 전역 기본 프로젝트로 fallback하지 않는다. output에 별도 project owner를 기록하고, 기존 버전/실패 후보도 다른 프로젝트의 소유로 바꾸지 못한다.

동일 dataset의 기존 mutation lock 안에서 최초 pin을 재확인하고, publication 직전 기존 pointer lock 안에서 source/config/model/base를 다시 확인한다. 취소와 publication은 같은 job 경계에서 순서를 정한다. 먼저 취소가 접수되면 publication을 하지 않는다. publication 이후 취소는 `too_late`이며 자동 rollback하지 않는다. 실제 전환 후에만 `succeeded`·`activated=true`와 정확한 publication receipt를 반환한다. 후보 생성 완료만으로 성공을 표시하지 않는다.

`held_for_review`는 모든 구성된 gate를 통과했어도 `activated=false`다. 검토 승인이나 current 전환을 만들어내지 않는다. `cancel_requested`는 종료 확인 전 상태이며 실제 하위 프로세스 종료와 정리 후 `canceled`가 된다. 종료 snapshot은 불변이다. 기존 reader pin·rollback/GC 규칙을 그대로 사용한다.

## 이번 단계의 지원 범위

작업 registry와 동일 key replay는 **프로세스 수명 동안**만 유지한다. 응답 capability는 `job_persistence=process_lifetime`, `serving_adoption=not_observed`, `input_preparation=existing_reviewed_or_rendered_inputs`다. profile 최대32개·프로세스당 작업 기록 최대128개로 제한하며 초과 시 거부한다. 실행 중 작업은 서버 종료 시 취소·정리하고 기다린다. 재시작 후 job을 복원하거나 성공으로 추정하지 않는다. durable journal·cross-process replay·publication 장애 복구는 KM-02.3의 완료 기준이다.

등록한 profile의 현재 접근 범위를 먼저 확인한 뒤 scoped key를 재조회한다. 같은 key/요청이면 source/current/version이 바뀌어도 같은 작업을 돌려주며, 같은 key의 다른 요청은 `idempotency_conflict`다. 새 요청은 source·profile·입력·embedding·base·기존 version을 모두 검사한다. 작업 제한 시간은 caller deadline과 profile 상한 중 허용된 범위이며 기존 하위 프로세스의 취소 경로를 사용한다.

activation receipt는 과거 전환의 증거이며 현재 모든 reader가 새 버전을 사용한다는 증거가 아니다. 별도 read 연결과 반환 좌표 확인이 필요하다. 이 단계의 실제 조회 시험은 이전 v1과 새 v3를 각각 재연결해 retained 원문/인용을 확인했다.

## 검증과 원자료

재현 진입점은 `python3 scripts/verify-versioned-maintenance.py --out /private/tmp/<new-owned-directory>`다. 실제 CKG/CKV와 stdio MCP, 두 소유 프로젝트, deterministic mock embedding을 사용한다. 유료 모델 호출·독립 수용·검색 품질 판단은 포함하지 않는다.

최종 공개 시험28항목은 기본33도구 호환·4도구 opt-in/schema·inspect 무변경·stale profile/source/input/model·경로 주입/타 프로젝트 거부·첫 구축/게시·replay/conflict·타 작업 조회/취소 거부·게시 후 취소·두 프로젝트 held·이전 DB 보존·stale base·두 번째 게시·실행 중 writer 충돌/취소·종료 snapshot·재연결 범위·이전/새 버전 실제 조회·다섯 연결 EOF를 확인한다. polling 횟수는 실행 시간에 따라 달라지므로 고정 시험 수와 합치지 않는다.

[검증 packet](../../system/eval/knowledge-roadmap-2026-10-15/km-02-public-maintenance/manifest.json)에 원시 wire·상태 관찰·fixture DB/원문·전체 Go/race 시험 목록·build/lint 결과를 보존한다. `check-maintenance-runtime-evidence.py`와 `test_maintenance_runtime_evidence.py`는 누락·거짓 게시·취소 성공 위조·상태/원문 변조·skip 승격 등 허위 완료를 검사한다. 과거 KM-02.1 설계 packet은 변경하지 않으며 `verify_committed_maintenance_design.py`로 당시7034ce85 snapshot에서 검증한다.
