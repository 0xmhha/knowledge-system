# Claude Code 시작 프롬프트

아래 본문을 동일 저장소에서 시작한 새 Claude Code 대화에 전달한다. 원자료 전체를 처음부터 붙여넣을 필요는 없다. 필요한 경우 실제 파일 접근/전송 권한은 해당 도구와 사용자 설정을 따른다.

```text
이 저장소에서 Codex가 진행하던 knowledge-system spec-driven 리팩토링을 이어서 맡아줘.

우선 docs/spec-driven/CLAUDE-CODE-SESSION-HANDOFF-2026-10-09.md와
CLAUDE-CODE-HANDOFF-SNAPSHOT-2026-10-09.json을 읽고, 인계 문서의 읽기 순서대로
현재 원장·실행 명세·작업리스트·최신 N12/N11 검토 자료를 확인해줘.
적용 가능한 AGENTS.md/CLAUDE.md도 먼저 확인해줘.

처음에는 branch/HEAD/git status와 python3 scripts/check-refactoring-progress.py를 확인하고,
전체8/18 완료·남은10개, 승인된 범위2개와 아직 미승인인 개별 내용/검토자,
최신 N12 candidate와 이전 immutable 후보를 정확히 구분하여 이해한 내용을 보고해줘.
이전 CLAUDE-CODE-REMAINING-HANDOFF.md는 최신 범위 선택·색인 결합 전의 위임 이력이다.

그다음 남은 N08/09/10/11/12/14/15/16/17/18을 수용 조건에 따라 순차 진행해줘.
진행 가능한 독립 작업은 매번 '다음 진행'을 기다리지 말고 계속해줘.
실제 사람/운영 사실이 필요한 곳은 구체적인 검토 자료를 갖춰 이 대화에서 필요한 결정만 요청해줘.
현재 우선은 N12의 기능3/요구3·기준4/F3-T01–17 및 core20/주장6/기존12관계의
실제 내용 판정·검토자와 N11 독립 gold/군·오염/기권·충분한 FINAL/runtime 동결이다.
범위 선택을 개별 content 승인으로, fixture author를 실제 사람 검토자로 해석하지 마.

세부 작업마다 목적·설계·실제 검증·미해결 조건을 문서에 반영하고,
현재 단계·완료 수·다음 작업·남은 전체를 한국어 로그로 보여줘.
완료 조건이 모두 충족되지 않은 task checkbox는 체크하지 마.
기존 FINAL·승인5입력·held 후보·사용자 .claude/와 logs/는 임의 변경하지 마.
동일 시험 반복을 실제 파일럿/독립 표본/사람 수용으로 세지 마.
제품 코드·입력 변경 시 새 evidence manifest/current·committed SHA 바인딩을 만들고
원장과 progress checker를 함께 갱신해줘. 관련 시험은 의미 있는 변경에 맞춰 수행해줘.
현재 브랜치를 유지하고 작업 보존과 외부 push/PR/merge/배포의 권한을 구분해줘.
```
