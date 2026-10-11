# N14 exact 시험 보고서와 검토 경로의 일치 검증

2026-10-08 · **보고서 검사 보완 / N14 실제 파일럿 수용은 미완료**.

[수정 전 재현](../../system/eval/b0-knowledge-system/refactoring-n14-run-proof-2026-10-08/red-summary.json) · [회귀 결과](../../system/eval/b0-knowledge-system/refactoring-n14-run-proof-2026-10-08/race-tests-summary.json) · [실제 held 후보의 실행 거부](../../system/eval/b0-knowledge-system/refactoring-n14-run-proof-2026-10-08/held-candidate-refusals.json).

## 목적과 문제

시험 실행 pass, 검토된 trace, 사람의 criterion approved는 별도 증거다. N14의 목적은 이 증거를 동일 명세·원문·후보에 결합하여 실제 변경을 수용하는 것이다. 그 과정에서 무관한 실행 보고서를 정확 시험의 증거로 가져오면 완료 판정의 근거가 깨진다.

`patch.validateRun`은 기존에 snapshot/criterion/성공 플래그/비어 있지 않은 hash·관찰 플래그와 verified CHECKED_BY 하나를 확인했다. 그러나 requirement의 전체 reviewed path, 보고서의 TESTED_BY 전체 목록, 잘못 추가된 CHECKED_BY ID, observed test name과 exact command hash를 대조하지 않았다. `RecordRun`, approved `Decide`, `Promote`가 이 함수를 사용한다.

수정 전 합성 fixture에서 정상 fully linked 경로를 확인한 뒤 **15개 불일치 변형이 잘못 통과**하는 것을 재현했다. 이 fixture의 reviewer는 시험용 문자열이며 실제 사람 판정이 아니다. 최초 fixture는 extractor 종류를 잘못 적어 구조 검사부터 실패했다. 그 작성 오류는 별도 원자료로 보존하고 교정한 뒤 실제 RED를 재현했으므로 제품 실패15개에 섞지 않는다. 기존 foreign CHECKED_BY-only 거부1개도 유지한다.

15개는 `validateRun` 직접 호출 수준의 재현이며 실제 공개 경로에서15번 승격한 결과가 아니다. 특히 unreviewed requirement 변형은 앞선 `CandidateProjection`의 구조 검사에서도 이미 거부된다. 이번 보완의 핵심은 구조상 유효한 projection에서도 보고서의 실제 경로/시험/명령과 맞지 않는 입력을 이 검사에서 거부하는 것이다.

## 보완한 계약과 설계

1. 생산 `Projection.Trace`를 다시 평가하여 해당 requirement가 fully linked이고, 지정 criterion·test canonical이 그 경로에 있는지 확인한다. 독립 CHECKED_BY만 존재하는 시험은 허용하지 않는다.
2. 보고서의 TESTED_BY/ CHECKED_BY ID 집합이 해당 검토 경로의 **전체 집합과 정확히 일치**해야 한다. 없는 ID·추가 ID·중복·부분 집합은 거부하고 순서 차이는 허용한다.
3. executable은 생산 exact runner의 `go`, 시험 이름은 canonical target의 suffix와 일치해야 한다. 동일 target의 source anchor가 모호하거나 `_test.go` 원문이 없으면 거부한다.
4. 원문의 package 경로와 시험 이름으로 생산 runner 계약의 `go test -json -count=1 -run ^TestName$ package` argv를 재구성하고 JSON argv의 SHA-256이 보고서와 일치해야 한다. OutputSHA256도 SHA-256 형식인지 확인한다.

기존 snapshot/종료/명령 성공/관찰된 exact pass/CheckedBy-only·legacy 배제 조건은 유지한다. 기존 command-only 시험의 정상 fixture를 fully linked 구조로 보강했다. 보고서 import와 approved 판정·승격 모두 동일 보완 검사를 재사용한다.

이것은 보고서와 현재 검토 trace/명령의 일치 검사다. 외부 작성자가 self-consistent JSON을 만들 수 없다는 인증 기능이나 raw 출력의 독립 서명 검증을 추가한 것이 아니다. exact runner의 실제 실행 및 사람의 Given/When/Then 판단은 여전히 별도이며 이 수정으로 새 criterion을 approved로 기록하지 않는다.

## 검증과 실제 후보의 상태

- 수정 전15개 잘못된 수용을 RED로 보관하고, 수정 후 기존 거부 포함16변형을 모두 거부했다. 정상 fixture와 여러 proof ID의 순서 차이·nested package 명령을 허용하고 일부 TESTED_BY만 남기면 거부한다.
- 직접 관련 patch/semantic 두 패키지 race 회귀 **46시험·31하위시험** 통과. patchcli는 컴파일 대상이며 `[no test files]`다. 관련3패키지 vet와 boundary/format 검사도 통과했다.
- 전체 Go 회귀 `-count=1`: **117패키지·2329시험·864하위시험**, 실패0. skip16/no-test12는 별도 기록한다. 이 실행은 Darwin ARM64이며 새 native Linux 실행은 아니다. [실행 요약](../../system/eval/b0-knowledge-system/refactoring-n14-run-proof-2026-10-08/full-go-tests-summary.json)과 압축 JSONL의 원문 SHA를 보관했다.
- 기존 실제 held projection의 4criteria/5test targets에 generic/exact 실행을 각각 요청했다. **10개 모두 `no fully reviewed code/test path`로 명령 시작 전에 거부**됐으며 제어용 child canary가 생성되지 않았다. 실제 linked report0/사람 decision0/patch registry0/승격0이다.
- 원 semantic store를 read-only로 사용했다. 원 source/index/의미 저장소·신원 등 권위9파일의 전후 SHA는 동일하다. 3명세는 spec_unapproved/paths=[]이고 semantic current도 없다. N12 snapshot/source 내용에 이번 수정이 포함됐다고 주장하지 않는다. 수정은 별도 현행 runtime 소스에만 적용했다.

모델 HTTP·새 DEV/FINAL matrix dispatch·새 독립 FINAL·실제 사람 활동 시간 측정은 없다. actual human review time=null이며 기존 N11 중요군 승인0, N12의 미작성 개념5경로·명세 범위 초안 미적용 상태를 유지한다.

## 완료 체크와 다음 작업

- [x] 실행 보고서 일치의 누락 재현과 회귀 fixture 교정 이력 보존.
- [x] 전체 reviewed path/proof IDs/canonical test/명령 hash 검사 구현.
- [x] 정상·불일치·multiple proof/nested package 회귀, 관련 race/vet/boundary/format 검사.
- [x] 실제 proposed 후보의10실행 요청 거부·child/report/current0 및 원 권위9파일 불변 확인.
- [ ] 실제 명세/관계 검토 결과로 새 source-backed 후보·trace 구성.
- [ ] 실제 해당 후보에서 exact linked run을 실행·기록하고 사람이4criteria를 판정.
- [ ] 실제 patch/승격·반려·rollback 및 원문 재생 수용.

**N14-A–E는 미체크, 전체8/18·요청5/15·남은10개다.** 다음은 N12 명세 범위·관계 판정과 N11 gold/독립성 판정 반영이다. 이후 수정한 runtime/source binding으로 실제 N14 후보를 준비한다. 남은 전체: N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N12 파일럿 검토, N14 변경 수용, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
