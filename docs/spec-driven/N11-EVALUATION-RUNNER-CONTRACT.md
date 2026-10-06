# N11 새 평가 실행 보호 계약

2026-10-06 · **실행 보호 세부 단계 검증 / N11 전체 미완료**. 실제 새 입력 승인·독립 FINAL·품질/비용 결과는 없다. [기존 입력 검사](./N11-FRESH-EVALUATION-PREPARATION.md)와 [개발 입력 검토](./N11-DEV-INPUT-REVIEW.md)를 유지한다.

## 목적과 경로

입력 검사 보고서를 만들었어도 다른 질문/설정/바이너리로 실행하거나 같은 FINAL을 다시 실행하면 사전 평가 동결이 무효다. `scripts/refactoring-eval-run.py`는 검토 기록·동결된 질문/gold/cluster·과거 관측 목록을 재검사한 뒤 기존 CKS matrix CLI에 고정 입력과 실행 신원을 전달한다. 직접 capture/matrix는 기존처럼 진단 도구이며 이 경로를 거치지 않은 실행이 새 독립 평가라는 승인을 얻지는 않는다.

전체 질문 책의 승인 기록(scope=fresh-evaluation-inputs)과 protocol/questions SHA·시각을 검사한다. 선택한 split의 요청 ID/prompt는 그 책에서 생성하고 정답을 요청 파일에 넣지 않는다. protocol의 execution_allowed=true와 아래 실행 바인딩·scope도 같은 검토 SHA 안에 있어야 한다. 현재 DEV7개 부분 검토만으로 이 전체 입력 조건은 충족되지 않는다. 부분 판정을 전체 FINAL 승인으로 바꾸지 않는다.

- protocol.execution_binding: dataset_id, config_sha256, binary_sha256.
- protocol.request_scope: knowledge_as_of(YYYY-MM-DD), knowledge_subsystem. 8arm의 조회 범위를 함께 고정한다.
- 모델 provider/name/digest/차원/runtime options·retrieval K와 반복/회전은 기존 승인 프로토콜 조건을 유지한다.
- 실제 실행은 `run` 하위 명령에서 protocol/questions/review/baseline/known-observations/registry/config/binary/repo/output 및 DEV 또는 FINAL split을 지정한다. 옵션은 도구의 help로 확인한다.

## 원문·설정·모델 결합

새 `--evaluation-binding` matrix 옵션은 `--environment-ledger`가 필수다. 검토한 dataset ID/원문 commit·config/binary SHA·모델 digest/차원/runtime options·retrieval K를 입력 신원에 대조하며, 불일치하면 출력/서버 세션을 만들기 전에 거부한다. 이 원문 commit은 코퍼스의 commit이다. 바이너리 SHA는 선언한 실행물의 신원을 고정하며 컴파일 출처를 독립 인증하지 않는다. 릴리스/평가 시 실제 build receipt와 소스 바인딩은 별도로 보존한다.

matrix의 기존 pinned v2/보관 원문 검증, 전체 입력 전후 해시 검사, live 모델 식별과 호스트 기록,8arm 순환·직렬 호출을 재사용한다. 설정에서 retrieval.recall_k=0이면 실제 기본 K는20이므로 K10 프로토콜에는 불일치로 거부된다. 제품의 기본 검색 K를 바꾸지 않았다.

질문/프로토콜/검토/기존 관측 자료의 정확 바이트를 private 출력에 복사해 matrix가 잠근다. 요청은 동결한 책에서 생성한다. 원 설정은 frozen SHA와 비교하고, 실행 중 변화/누락은 기존 matrix의 partial/error 분모와 전후 검사에 남는다. standalone input checker의 execution_ready=false를 출시 또는 평가 합격으로 덮어쓰지 않는다.

## FINAL 예약과 실패 경계

registry는 운영자가 계속 보존하는 로컬 JSON 원장이다. `init`는 새 파일만 생성하고 기존 파일을 덮어쓰지 않는다. 실행은 원장이 없거나 손상됐거나 symlink이면 거부한다. 같은 경로의 영구 companion .lock inode를 OS flock으로 잠그며 PID/나이로 탈취하거나 unlink하지 않는다.

FINAL split은 모든 선택 FINAL cluster와 책/프로토콜 SHA를 **자식 실행 전에** FINAL_reserved/reserved_at로 저장한다. temp 파일 fsync→rename→부모 디렉터리 fsync를 수행한다. 예약은 실제 응답 관측이 아니다. 실패/실행 전 취소/부모 SIGKILL 뒤에도 보수적으로 재사용을 거부하고 새 독립 FINAL로 회전한다. 예약을 자동 삭제하거나 first_observed_at로 꾸미지 않는다. DEV split은 FINAL을 예약하지 않는다.

wrapper가 matrix 종료까지 잠금을 보유하고 자식에게 같은 fd를 넘긴다. 부모가 SIGKILL돼도 살아 있는 자식이 잠금을 유지한다. 다른 실행은 잠금 획득에 실패하며 병렬 지연 실험을 시작하지 않는다. 동일 원장·cooperative writer/로컬 OS 경계다. 다른 원장을 만들거나 이력을 삭제한 운영자, network FS, 전체 과거 오염 사실 및 검토자의 실체를 인증하지 않는다. 기존 모든 관측과 수동 오염 검토도 필요하다.

## 검증과 미완료 조건

[runner 제어 원자료](../../system/eval/b0-knowledge-system/refactoring-n11-runner-controls-2026-10-06/runner-control-manifest.json)에 입력11개+runner8개 Python 시험, 실제 separate-process flock 경합·예약 뒤 SIGKILL·부모 SIGKILL 후 자식 잠금 보존, Go binding11사례(정상1/거부10), environment 필수 조건과 관련2패키지 race를 연결했다. 자식 잠금 보존 시험은 Python process 제어다. 실제 native Darwin CKS CLI 옵션 거부와 현재 미승인 초안의 child dispatch0/새질의0을 별도로 검증했다. 실제 BGE 8arm 실행·품질·native Linux·운영 적합성 증거가 아니다.

N11-A/B의 새 gold/독립성·오염 판정·입력 동결, N11-C의 실제 coordinated dataset/model/binary 결합 실행, N11-D의 N08–10 사용 증거는 남는다. 충분한 독립 중요군이 없으면 inconclusive/default disabled/release withheld다. 단지 코드 보호를 구현했다고 N11 전체 완료를 표시하지 않는다.

현재 요청15개 중5완료, 남은 전체 N08/09/10/11/12/14/15/16/17/18 10개. 다음은 실제 DEV/프로토콜 및 N12 내용·관계 판정 반영과 새 독립 FINAL 사전 검토다.
