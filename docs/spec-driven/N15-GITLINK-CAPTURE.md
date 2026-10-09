# N15 submodule이 있는 저장소의 committed 캡처

2026-10-10 · **제품 수정·시험 완료 / N15-D 진행 중 / N15 미완료**. 전체10/18, 남은8개.

## 1. 왜 바꿨는가

사용자는 knowledge-system을 로컬 개인 운영으로 쓰기로 하고, 실제 소비 프로젝트로 go-stablenet(`wemade/go-stablenet`, HEAD `740526d0`)을 정했다([범위 결정](../../system/eval/b0-knowledge-system/refactoring-n15-n17-operating-scope-2026-10-10/scope-decision.json)). 첫 운영 색인은 10.7초 만에 `operation_failed`로 끝났고, 모델 호출은 0이었다. 같은 인자로 setup을 직접 실행해 원래 오류를 확인했다. 원인은 추적 항목 `systemcontracts/solidity/openzeppelin/contracts`가 일반 파일이 아니라는 것이었다([원인](../../system/eval/b0-knowledge-system/refactoring-n15-gitlink-capture-2026-10-10/real-repo-cause.txt)). go-stablenet에는 초기화되지 않은 submodule(gitlink, mode 160000)이 4개 있다.

## 2. 무엇이 잘못되었는가

committed 캡처는 `git ls-files --cached`의 모든 항목을 일반 파일로 읽었다. gitlink는 superproject commit 안의 파일이 아니라 다른 저장소 commit을 가리키는 포인터다. 그래서 submodule이 하나라도 있는 저장소는 통째로 거부되었다. filelist로 범위를 좁혀도 신원 계산이 모든 추적 항목을 읽으므로 피할 수 없었다.

## 3. 무엇을 바꿨는가

- `git ls-files -z --cached --stage`로 항목의 mode와 객체 ID를 함께 읽는다. mode 160000은 파일 목록에서 빼고, 경로와 고정 commit을 `CapturedGitlink`로 모은다. stage가 0이 아닌 항목(충돌 상태)은 거부한다.
- gitlink 목록은 캡처 manifest의 `gitlinks`에 남는다. 목록 digest는 소스 신원의 선택 필드 `gitlink_digest`에 들어간다. 이 필드가 있을 때만 snapshot ID를 `cks.snapshot.v6`로 계산한다. submodule이 없는 저장소는 필드가 비어 있으므로 기존 v5 snapshot ID가 그대로다.
- 보관 원문 검증은 manifest의 gitlink 목록을 다시 해시해 신원과 대조하고, 정렬이 어긋나거나 경로가 위험하거나 commit 형식이 틀리면 거부한다. 캡처 직후 사후 검증은 live 저장소의 gitlink가 바뀌었으면 거부한다.
- submodule 내용(openzeppelin, ethereum/tests 등)은 색인하지 않는다. superproject commit에 그 바이트가 없기 때문이다.

## 4. 어떻게 확인했는가

- RED: 수정 전 코드에서 gitlink 하나가 있는 저장소의 committed 신원 계산이 실패했다([기록](../../system/eval/b0-knowledge-system/refactoring-n15-gitlink-capture-2026-10-10/gitlink-red-before-fix.txt)).
- GREEN: `TestCommittedCaptureExcludesGitlinksAndBindsTheirCommits`가 다음을 확인한다. 캡처와 빌드 전 신원 일치, 파일 목록에서 gitlink 제외와 기록, 보관 원문 검증 통과, 복원 checkout이 깨끗함(gate 통과), manifest의 gitlink commit 위조 거부, submodule commit을 옮기면 파일 digest는 같고 gitlink digest만 바뀜, 이동 뒤 사후 검증 거부. `TestSnapshotIdentityWithoutGitlinksKeepsV5`는 submodule 없는 저장소의 v5 ID가 그대로임을 확인한다.
- 관련 9패키지 race(setup, setupcli, doctorcli, patch, semantic, evidencev2, mcp, evalcli, graph buildpipe) 통과([출력](../../system/eval/b0-knowledge-system/refactoring-n15-gitlink-capture-2026-10-10/gitlink-related-race.txt)), vet·gofmt 통과.

## 5. 한계

- 이전 release(985e22d8, d4fd6e3e)는 go-stablenet을 색인할 수 없다. 새 release가 만든 v6 신원은 새 필드를 모르는 이전 binary가 읽지 못할 수 있으므로, 이전 release로 binary rollback하는 경로는 이 데이터셋에서 지원하지 않는다.
- N11 동결 실행 바인딩의 binary(`e53c308e`)는 별도 파일이며 이 변경과 무관하다. 평가 corpus에는 submodule이 없으므로 신원도 바뀌지 않는다.

**남은 전체8개:** N08 검색 품질, N09 기권, N10 비용·지연, N11 독립 평가, N15 규모·이식성, N16 신뢰·권한, N17 실제 복구, N18 기본값·출시 판정.
