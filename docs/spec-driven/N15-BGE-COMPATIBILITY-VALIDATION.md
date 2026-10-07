# N15 실제 BGE-M3의 작은 설치 호환성 검증

2026-10-07 · **기술 세부 검증 통과 / N15 전체 미완료**. [원자료 요약](../../system/eval/b0-knowledge-system/refactoring-n15-bge-compatibility-2026-10-07/summary.json)을 기준으로 한다. 요청15개 중5개, 전체8/18 완료이며 잔여10개는 유지한다.

기존 mock 설치 시험만으로 실제 embedding runtime의 모델 신원·차원·재시작 호환성을 확인할 수 없어, clean `b006e776` Darwin ARM64 패키지를 새 소유 fixture3개에 연결했다. 패키지 SHA256은 `2315c29dfa4e0008746da1eecfc11bf3ef886b8684b5125dd17bb103f4a496dd`다. `b006e776`부터 전체 race를 실행한 `d08cb384`까지 runtime/code roots와 go.mod/go.sum의 차이는0이다. 원래 개발/FINAL 자료와 사용자 프로젝트는 사용하지 않았다.

Ollama 0.35.1의 `bge-m3:latest`, digest `7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab`, native1024차원을 사용했다. 시험 전후 해당 모델 메타데이터가 동일했고, 각 프로젝트의 원본/갱신 데이터6개 vector manifest에서 provider/model digest/native dimension과 project/snapshot/dataset/source commit을 대조했다. 무관한 설치 모델의 목록은 커밋할 증거에서 제외했다.

| 시험 | 실제 관측 |
|---|---|
| empty Go module | 문서 전용 설치·MCP 재시작·다음 데이터 버전 생성·이전 identity rollback 통과; degraded는 지원 코드가 없는 fixture의 예상 상태 |
| TypeScript | shared code1개, ready; node_modules 제외·코드 인용·설치/재시작/갱신/rollback 통과 |
| Python 문서 fixture | Python 코드 지원을 주장하지 않음; 문서 전용 degraded·설치/재시작/갱신/rollback 통과 |
| CKV CLI | 원본/rollback6응답의 인용 commit·파일·줄 범위와 보관한 Git 원문을 대조 |
| MCP | 서비스 가능·pinned commit·인용 파일을 compact probe7응답에서 확인; 재시작 결과 동일 |
| 전체 race | `make test-race`, exit0, 시험 패키지117개 통과·시험 없는 패키지12개 |
| 도메인 자료 | `make check-domain-artifacts`, exit0, tracked 생성 자료 변경0 |

MCP7개와 CKV6개는 호환성 제어13응답이다. 검색 품질의 독립 표본이나 비용/호출 수 분모로 사용하지 않는다. MCP 원자료는 compact probe 출력이며 raw v2/JCS·전체 응답 의미를 새로 인증했다고 주장하지 않는다. source 변형·fixture 질문은 사람 승인 gold나 새 FINAL로 배정하지 않았고 새 FINAL dispatch0이다.

원 설치 스크립트의 mock 설정만 로컬 어댑터에서 Ollama/model로 바꿨다. 원 스크립트·pin probe·패키지 helper와 어댑터 바이트를 SHA에 연결한다. 임시 원본 Git/SQLite/바이너리와 전체 Ollama 설치 목록은 로컬에 남긴다. 보관 자료는 driver·출력·설정·메타데이터·fixture의 두 Git 원문 버전이다. 전체 race는 현재 저장소에서 실행했고 패키지는 앞서 clean checkout에서 빌드한 것이다.

도메인 검사 중 sandbox의 전역 Go module stat cache 쓰기 경고3개가 있었지만 빌드와 두 생성 명령은 성공했고 생성 파일 diff는0이었다. 경고를 숨기지 않고 stderr를 보존한다. 해당 경고는 제품 시험 실패나 native Linux 실행 증거가 아니다.

## 현재·다음·남은 전체

N15-D의 **작은 실제 모델 연결 부분만 검증**했다. native Linux AMD64 실행, 실운영 규모, 독립 운영 소비자의 설치/바이너리 upgrade 및 지원 수용이 남아 있으므로 N15-C/D/E는 미체크다. 실제 운영 권한·고지 적합성·backup/RTO/RPO·훈련은 N16/17이며, 품질/기권/비용·출시는 N08/09/10/11/12/14/18에 남는다.

다음은 N11 사전평가/독립 gold·N12 실제 내용 판정 반영 및 native Linux CI 실행이다. 남은 전체: **N08·N09·N10·N11·N12·N14·N15·N16·N17·N18**, 10개. 사람 판정과 실운영 사실을 시험 결과로 대체하지 않는다.

공개 전송 요청 범위는 `d08cb384`까지의80개 커밋으로 고정한다. 이 추가 BGE/전체 race receipt와 후속 로컬 문서 커밋은 그 전송 범위에 포함하지 않으며 공개 승인이 없다. 승인 응답이 오더라도 최신 HEAD를 자동 푸시하지 않고 승인된 정확한 커밋을 사용한다. native CI도 해당 소스 커밋에서 실행하며 추가 로컬 증거를 CI artifact로 보내지 않는다.
