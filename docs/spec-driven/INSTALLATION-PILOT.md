# Knowledge System 설치 시범 절차

현재 산출물은 **빌드한 호스트에서 검증하는 개발 프리뷰**다. 운영 배포 전에 대상 OS/CPU에서 네이티브 라이브러리, 모델, 라이선스와 전체 WBS Gate 5를 확인한다. `mock` 임베더는 구조 스모크 전용이며 자연어 검색 품질을 검증하지 않는다.

## 1. 호스트 패키지

저장소에서 다음을 실행한다.

```sh
python3 scripts/package-host.py --out-dir /tmp/knowledge-system-dist
```

출력 JSON의 `sha256`으로 압축 파일을 확인하고 해제한다. 압축 파일에는 `ckg`, `ckv`, `cks`, `LICENSE`, `INSTALLATION.md`, 소독 규칙, `modules.txt`, `manifest.json`이 들어 있다. `manifest.json`에는 바이너리별 SHA-256·크기, 빌드 커밋, 호스트, Go 버전, 감지한 네이티브 의존성이 기록된다. 작업 트리가 더러울 때 프리뷰가 필요하면 `--allow-dirty`를 명시하며, 매니페스트가 `dirty: true`로 표시된다. 배포 가능 판정은 아니다.

## 2. 프로젝트별 초기화

압축을 푼 디렉터리에서 **다른 Git 저장소**에도 동일한 절차를 적용한다. 데이터셋은 소스 트리 밖에 둔다.

```sh
./cks doctor --src /path/to/project
./cks init --src /path/to/project \
  --dataset /path/to/project-dataset \
  --config-out /path/to/project-setup.yaml \
  --embedder ollama --model-name YOUR_EMBEDDING_MODEL
./cks setup --config /path/to/project-setup.yaml --version auto
./cks doctor --src /path/to/project --dataset /path/to/project-dataset --strict
```

`init`은 기존 설정을 덮어쓰지 않는다. 실제 임베더를 명시해야 하며 Ollama 모델은 별도로 준비해야 한다. `doctor`는 Go, TypeScript, JavaScript, Solidity, Markdown 파일 수와 graph-only Proto 수를 미리 보고한다. 심볼릭 링크와 의심스러운 비밀 경로를 개수로만 보고하며 내용을 읽거나 출력하지 않는다. 이 진단은 프로젝트의 `.ckvignore`와 파일 목록 정책이 제대로 적용됐는지 대체하지 않는다.

Go 소스가 없는 모듈이나 지원하지 않는 Python 코드와 Markdown만 있는 저장소에서도 문서 검색은 가능하지만 코드 AST 그래프와 CKV 코드 심볼의 조인은 없다. `doctor`는 `shared_code_files=0`, `status=degraded`로 이 범위를 표시한다. TypeScript 저장소는 `shared_code_files`를 양수로 보고한다. 세 유형의 독립 저장소에서 `init → setup → doctor → ckv query → CKS MCP context.get_for_task`를 실행하는 구조 스모크는 `scripts/wbs-install-smoke.sh`에 있다. 각 프로젝트의 MCP 서버를 새로 띄워 다시 조회했을 때도 인용 커밋이 그대로였고, TypeScript 코드 질문은 `main.ts`를 인용했다. 이 테스트는 mock 임베더와 현재 macOS 호스트만 다룬다.

`--version auto`는 커밋 기반 버전 이름을 사용한다. 현재 모든 `setup` 빌드는 추적·미추적 파일이 남아 있거나 Git이 무시하지만 CKV/CKG가 읽을 수 있는 소스 파일이 있으면 거부한다. 명시적 버전 이름을 줘도 예외가 아니다. 버전 후보는 활성화 직전에 빌드 결과의 커밋과 소스 청결성을 다시 검사한다. 작업 트리 스냅샷 모드는 아직 구현 중이다. 설정 파일을 소스 저장소 안에 만들었다면 커밋하거나 무시 정책을 정한 뒤 빌드한다.

검토 완료된 CKS 온톨로지·명세를 CKV 자연어 코퍼스에도 넣으려면 먼저 활성 의미 투영을 `cks semantic export-text ... --out /path/to/new-corpus`로 내보내고, 다음 재색인에 `--semantic-corpus /path/to/new-corpus`를 지정한다. 출력 디렉터리는 소스 저장소 밖에 둔다. 후보 그래프와 코퍼스의 저장소·커밋·파일 해시가 맞아야 CKV가 이를 임베딩한다. `--version auto`는 이 코퍼스 매니페스트 해시도 버전 이름에 반영한다. 코퍼스 출처가 달라지면 같은 소스 커밋이라도 새 데이터셋으로 만든다.

## 3. 서비스와 롤백

데이터셋이 승격된 다음 CKS 런타임 설정을 생성한다. 배포용 소독 규칙 경로를 명시한다.

```sh
./cks mcp gen-config --dataset-dir /path/to/project-dataset/current \
  --source-root /path/to/project \
  --sanitize-rules /path/to/unpacked/policies/sanitization_rules.yaml \
  --embed-model YOUR_EMBEDDING_MODEL --out /path/to/cks.yaml
./cks mcp --config /path/to/cks.yaml
```

롤백은 이전에 검증된 버전으로 `current`를 되돌린다. 실행 중인 MCP 프로세스는 이미 연 데이터셋 핸들을 유지하므로 롤백·승격 후 재시작해야 한다.

커밋된 패치에 대해 승격 전 테스트 게이트를 걸려면 `setup --version <new-version> --gate-test-bin go --gate-test-arg test --gate-test-arg ./...`처럼 실행한다. 인자는 각각 전달하며 셸을 거치지 않는다. 빌드한 후보의 `test-gate.json`에는 커밋, 그래프 다이제스트, 명령·출력 해시, 종료 코드와 실행 환경이 남는다. 테스트 실패 또는 실행 전후 소스 변경은 `current`를 바꾸지 않는다. 이는 선택한 **명령의 성공**을 확인하는 게이트이며, 개별 수용 기준의 의미적 충족은 검토자가 판정해야 한다.

```sh
./cks setup --config /path/to/project-setup.yaml --rollback PREVIOUS_VERSION
```

이 절차는 프로젝트별 수동 스모크의 출발점이다. Windows/Linux 빌드·실행, 깨끗한 호스트의 네이티브 의존성, 실제 임베딩 모델의 품질·지연, 작업 트리 스냅샷은 Gate 5에 남아 있다. 현재 호스트의 구조 스모크는 세 프로젝트를 독립 데이터셋으로 설치해 각 인용의 커밋을 검사했다. 또 다른 스모크에서는 실패 후보가 `current`를 바꾸지 않고, 실행 중인 MCP 프로세스가 교체 전 커밋의 인용을 계속 내보내며, 새 프로세스는 교체 후 커밋을 인용하고, 롤백 후 새 프로세스는 이전 커밋을 다시 인용하는 것을 확인했다.
