# 외부 패치 후보와 기준별 판정

상태: A5.3의 **명시 경로 부분 구현** (2026-10-01). 이 절차는 코드를 자동 수정하지 않는다. `TestExecution.pass`와 `CriterionDecision.approved`를 분리하는 [종단 간 설계](./END-TO-END-DESIGN.md)의 검증 경로다.

## 커밋된 Git 후보의 순서

1. 현재 데이터셋 버전을 기준으로 소스 변경을 별도 커밋한다. `cks setup --project-id ID --source-mode committed --version NEXT --hold-for-review --gate-test-bin go --gate-test-arg test --gate-test-arg ./...`로 새 CKG/CKV 후보를 만든다. 일반 `current`는 바뀌지 않는다.
2. `cks patch --dataset DATA --patch-id PATCH register --version NEXT`가 두 보관본의 `(origin_id,path,SHA-256)` 차이를 기록한다. 같은 `patch_id`로 다른 변경 바이트를 등록할 수 없다.
3. `cks semantic build --version-dir DATA/NEXT ... --extract-only`로 의미 투영 초안을 만들고, 검토된 개념→코드→테스트 및 `CHECKED_BY` 연결을 근거와 함께 채운다. `cks semantic promote --version-dir DATA/NEXT --input PROJECTION ...`로 **저장만** 하고 아직 활성화하지 않는다.
4. `cks semantic test --project-id ID --dataset-id DATASET_ID --version-dir DATA/NEXT ... --criterion-id CRITERION --go-test-exact --out RUN.json`을 실행한다. `cks patch --dataset DATA --patch-id PATCH record-run --semantic-store STORE --report RUN.json`으로 성공 보고서를 불변 기록한다.
5. 검토자가 Given/When/Then과 실제 코드·테스트 근거를 읽고 `cks patch --dataset DATA --patch-id PATCH decide --semantic-store STORE --decision-id DECISION --criterion-id CRITERION --reviewer REVIEWER --outcome approved|rejected --reason TEXT`로 기준별 판정을 남긴다. `approved`에는 같은 스냅샷에서 성공한 테스트 실행 기록이 선행돼야 한다.
6. 모든 요구사항의 추적 상태가 `linked`, 모든 기준에 성공 실행과 `approved`가 있고 상충 판정이 없을 때만 `cks patch --dataset DATA --patch-id PATCH promote --semantic-store STORE`가 `current`를 바꾼다. 테스트 성공 직후에는 `unconfirmed`이고, 일반 `setup --rollback NEXT`로 보류를 우회할 수 없다. 검토 후 승격된 버전은 이후 일반 롤백 대상으로 사용할 수 있다.

`scripts/wbs-patch-smoke.sh`가 실제 Go AST/CKV/CKG 후보, 정확 테스트 실행, 사람 판정 부재 시 거부, 승격과 왕복 롤백을 mock 임베더로 재생한다. 이 스모크는 조직 정책의 진위나 자연어 품질을 판정하지 않는다. 현재 정확 테스트 실행은 커밋된 Git 소스의 Go 테스트에 맞춰져 있고, 다른 프레임워크 및 working-tree/비Git 기준별 실행은 남은 A 작업이다. 로컬 `reviewer`는 감사용 ID이며 외부 신원 인증이 아니다.
